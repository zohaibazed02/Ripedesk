package payments

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type WebhookHandler struct {
	pool     *db.Pool
	provider OnlinePaymentProvider
}

func NewWebhookHandler(pool *db.Pool, provider OnlinePaymentProvider) *WebhookHandler {
	return &WebhookHandler{pool: pool, provider: provider}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "READ_BODY_FAILED", err.Error(), nil)
		return
	}

	sig := r.Header.Get("X-Webhook-Signature")
	if err := h.provider.VerifyWebhookSignature(rawBody, sig); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", err.Error(), nil)
		return
	}

	var ev struct {
		ProviderEventID string `json:"provider_event_id"`
		TenantID        string `json:"tenant_id"`
		BranchID        string `json:"branch_id"`
		OrderID         string `json:"order_id"`
		PaymentID       string `json:"payment_id"`
		EventType       string `json:"event_type"` // payment.authorized | payment.captured
	}
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_WEBHOOK_JSON", err.Error(), nil)
		return
	}

	var duplicate bool
	err = h.pool.WithTenantTx(r.Context(), ev.TenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM processed_commands WHERE command_id = $1)
		`, ev.ProviderEventID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			duplicate = true
			return nil
		}

		payStatus := "authorized"
		if ev.EventType == "payment.captured" {
			payStatus = "captured"
		}
		_, _ = tx.Exec(r.Context(), `UPDATE payments SET status = $2 WHERE id = $1`, ev.PaymentID, payStatus)
		_, _ = tx.Exec(r.Context(), `UPDATE orders SET status = 'open' WHERE id = $1`, ev.OrderID)

		if _, err := tx.Exec(r.Context(), `
			INSERT INTO processed_commands (command_id, tenant_id, branch_id, command_type, result_status)
			VALUES ($1, $2, $3, $4, 'acked')
		`, ev.ProviderEventID, ev.TenantID, ev.BranchID, ev.EventType); err != nil {
			return err
		}

		_, err := tx.Exec(r.Context(), `
			INSERT INTO outbox_events (tenant_id, branch_id, aggregate_type, aggregate_id, event_type, payload)
			VALUES ($1, $2, 'order', $3, 'web_order.paid_awaiting_branch', $4::jsonb)
		`, ev.TenantID, ev.BranchID, ev.OrderID, string(rawBody))
		return err
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "WEBHOOK_PROCESS_FAILED", err.Error(), nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"acknowledged": true,
		"duplicate":    duplicate,
	}, nil)
}
