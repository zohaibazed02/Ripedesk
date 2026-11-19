package procurement

import (
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type GuardrailConfig struct {
	MaxAutoSendBudget   decimal.Decimal
	MaxPriceVariancePct decimal.Decimal
}

type GuardrailResult struct {
	NeedsReview   bool     `json:"needs_review"`
	TargetStatus  string   `json:"target_status"` // pending_review | approved
	ReviewReasons []string `json:"review_reasons"`
}

// EvaluatePOGuardrails checks budget caps, price variance vs last paid price, and supplier bank/detail changes.
func EvaluatePOGuardrails(
	poTotal decimal.Decimal,
	unitPrice decimal.Decimal,
	lastPaidPrice decimal.Decimal,
	supplierDetailsChanged bool,
	cfg GuardrailConfig,
) GuardrailResult {
	var reasons []string

	if cfg.MaxAutoSendBudget.GreaterThan(decimal.Zero) && poTotal.GreaterThan(cfg.MaxAutoSendBudget) {
		reasons = append(reasons, fmt.Sprintf("PO total %s exceeds auto-send budget %s", poTotal.StringFixed(2), cfg.MaxAutoSendBudget.StringFixed(2)))
	}

	if lastPaidPrice.GreaterThan(decimal.Zero) && cfg.MaxPriceVariancePct.GreaterThan(decimal.Zero) {
		varPct := unitPrice.Sub(lastPaidPrice).Div(lastPaidPrice).Mul(decimal.NewFromInt(100)).Abs().Round(2)
		if varPct.GreaterThan(cfg.MaxPriceVariancePct) {
			reasons = append(reasons, fmt.Sprintf("Unit price variance %s%% exceeds guardrail %s%%", varPct.StringFixed(2), cfg.MaxPriceVariancePct.StringFixed(2)))
		}
	}

	if supplierDetailsChanged {
		reasons = append(reasons, "Supplier bank or contact details recently changed")
	}

	if len(reasons) > 0 {
		return GuardrailResult{
			NeedsReview:   true,
			TargetStatus:  "pending_review",
			ReviewReasons: reasons,
		}
	}
	return GuardrailResult{
		NeedsReview:   false,
		TargetStatus:  "approved",
		ReviewReasons: []string{},
	}
}

// CanDispatchPO enforces idempotent outbound supplier dispatch (never double-sends an already issued/sent PO).
func CanDispatchPO(currentStatus string) (shouldSend bool, alreadyDispatched bool) {
	if currentStatus == "issued" || currentStatus == "sent" || currentStatus == "confirmed" {
		return false, true
	}
	return true, false
}

func (h *Handler) ListPurchaseOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	type poRow struct {
		ID          string `json:"id"`
		PONumber    string `json:"po_number"`
		SupplierID  string `json:"supplier_id"`
		Status      string `json:"status"`
		TotalAmount string `json:"total_amount"`
	}
	items := []poRow{}
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, po_number, supplier_id, status, total_amount::text
			FROM purchase_orders
			ORDER BY created_at DESC
			LIMIT 100
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p poRow
			if err := rows.Scan(&p.ID, &p.PONumber, &p.SupplierID, &p.Status, &p.TotalAmount); err != nil {
				return err
			}
			items = append(items, p)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "PO_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) ApprovePurchaseOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	poID := r.PathValue("id")
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `UPDATE purchase_orders SET status = 'issued' WHERE id = $1`, poID)
		return err
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "PO_APPROVE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": poID, "status": "issued"}, nil)
}

func (h *Handler) DispatchPurchaseOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	poID := r.PathValue("id")
	var currentStatus string
	var duplicate bool
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `SELECT status FROM purchase_orders WHERE id = $1 FOR UPDATE`, poID).Scan(&currentStatus); err != nil {
			return err
		}
		shouldSend, already := CanDispatchPO(currentStatus)
		if already {
			duplicate = true
			return nil
		}
		if shouldSend {
			_, err := tx.Exec(r.Context(), `UPDATE purchase_orders SET status = 'issued' WHERE id = $1`, poID)
			return err
		}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "PO_DISPATCH_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":                 poID,
		"status":             "issued",
		"already_dispatched": duplicate,
	}, nil)
}
