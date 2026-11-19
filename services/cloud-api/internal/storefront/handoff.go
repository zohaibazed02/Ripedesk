package storefront

import (
	"context"
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/payments"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type PendingWebOrder struct {
	OrderID         string
	PaymentStatus   string // authorized | captured | cod
	ProviderRef     string
	Amount          decimal.Decimal
	PendingSinceUTC time.Time
}

type HandoffReconciliationResult struct {
	OrderID        string `json:"order_id"`
	ActionTaken    string `json:"action_taken"` // still_pending | cancelled_reversed | cancelled_refunded | cancelled_cod
	GatewayRef     string `json:"gateway_ref,omitempty"`
}

// ReconcileStaleWebHandoff evaluates an order stuck in pending_branch_acceptance.
// If SLA timeout has elapsed, it cancels and automatically reverses authorization or refunds captured funds.
func ReconcileStaleWebHandoff(
	ctx context.Context,
	provider payments.OnlinePaymentProvider,
	ord PendingWebOrder,
	now time.Time,
	maxPendingDuration time.Duration,
) (HandoffReconciliationResult, error) {
	if now.Sub(ord.PendingSinceUTC) < maxPendingDuration {
		return HandoffReconciliationResult{
			OrderID:     ord.OrderID,
			ActionTaken: "still_pending",
		}, nil
	}

	switch ord.PaymentStatus {
	case "authorized":
		rev, err := provider.ReverseAuthorization(ctx, ord.ProviderRef, "branch_handoff_timeout")
		if err != nil {
			return HandoffReconciliationResult{}, err
		}
		return HandoffReconciliationResult{
			OrderID:     ord.OrderID,
			ActionTaken: "cancelled_reversed",
			GatewayRef:  rev.ReversalRef,
		}, nil
	case "captured":
		ref, err := provider.RefundCapturedPayment(ctx, ord.ProviderRef, ord.Amount, "branch_handoff_timeout")
		if err != nil {
			return HandoffReconciliationResult{}, err
		}
		return HandoffReconciliationResult{
			OrderID:     ord.OrderID,
			ActionTaken: "cancelled_refunded",
			GatewayRef:  ref.RefundRef,
		}, nil
	default:
		return HandoffReconciliationResult{
			OrderID:     ord.OrderID,
			ActionTaken: "cancelled_cod",
		}, nil
	}
}

func (h *Handler) AcceptWebOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"order_id":           orderID,
		"status":             "branch_accepted",
		"branch_accepted_at": time.Now().UTC().Format(time.RFC3339),
	}, nil)
}

func (h *Handler) RejectWebOrder(provider payments.OnlinePaymentProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID := r.PathValue("orderId")
		var in struct {
			PaymentStatus string          `json:"payment_status"`
			ProviderRef   string          `json:"provider_ref"`
			Amount        decimal.Decimal `json:"amount"`
			Reason        string          `json:"reason"`
		}
		_ = httpx.DecodeJSON(r, &in)

		res, err := ReconcileStaleWebHandoff(r.Context(), provider, PendingWebOrder{
			OrderID:         orderID,
			PaymentStatus:   in.PaymentStatus,
			ProviderRef:     in.ProviderRef,
			Amount:          in.Amount,
			PendingSinceUTC: time.Now().UTC().Add(-1 * time.Hour),
		}, time.Now().UTC(), 0)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "REVERSAL_FAILED", err.Error(), nil)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, res, nil)
	}
}
