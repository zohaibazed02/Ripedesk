package storefront

import (
	"errors"
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var ErrBranchOffline = errors.New("branch is currently offline — immediate online ordering is paused")

type CheckoutLinePricing struct {
	UnitPrice  decimal.Decimal
	ModDelta   decimal.Decimal
	Quantity   decimal.Decimal
	TaxRatePct decimal.Decimal
}

type CheckoutTotals struct {
	Subtotal    decimal.Decimal `json:"subtotal"`
	TaxTotal    decimal.Decimal `json:"tax_total"`
	DeliveryFee decimal.Decimal `json:"delivery_fee"`
	GrandTotal  decimal.Decimal `json:"grand_total"`
}

// CheckBranchHeartbeat enforces the strict branch online guard before accepting immediate web orders.
func CheckBranchHeartbeat(heartbeatExpiresAt *time.Time, now time.Time) error {
	online, accepting := ComputeBranchOnlineStatus(heartbeatExpiresAt, now)
	if !online || !accepting {
		return ErrBranchOffline
	}
	return nil
}

// ComputeAuthoritativeCheckoutTotals calculates exact server-side totals using shopspring/decimal.
func ComputeAuthoritativeCheckoutTotals(lines []CheckoutLinePricing, deliveryFee decimal.Decimal) CheckoutTotals {
	hundred := decimal.NewFromInt(100)
	sub := decimal.Zero
	tax := decimal.Zero
	for _, l := range lines {
		lineNet := l.UnitPrice.Add(l.ModDelta).Mul(l.Quantity).Round(4)
		lineTax := lineNet.Mul(l.TaxRatePct).Div(hundred).Round(4)
		sub = sub.Add(lineNet)
		tax = tax.Add(lineTax)
	}
	grand := sub.Add(tax).Add(deliveryFee).Round(4)
	return CheckoutTotals{
		Subtotal:    sub.Round(4),
		TaxTotal:    tax.Round(4),
		DeliveryFee: deliveryFee.Round(4),
		GrandTotal:  grand,
	}
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("tenantSlug")
	branchID := r.PathValue("branchId")

	var tenantID string
	if err := h.pool.QueryRow(r.Context(), `SELECT id FROM tenants WHERE slug = $1 AND status = 'active'`, slug).Scan(&tenantID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "Storefront tenant not found", nil)
		return
	}

	var in struct {
		ClientOrderID string `json:"client_order_id"`
		OrderMode     string `json:"order_mode"` // takeaway | delivery | qr_order
		PaymentMethod string `json:"payment_method"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}

	var heartbeatExpiresAt *time.Time
	err := h.pool.WithTenantTx(r.Context(), tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT heartbeat_expires_at FROM branches WHERE id = $1`, branchID).Scan(&heartbeatExpiresAt)
	})
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "BRANCH_NOT_FOUND", "Branch not found", nil)
		return
	}
	if err := CheckBranchHeartbeat(heartbeatExpiresAt, time.Now().UTC()); err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "branch_offline", err.Error(), nil)
		return
	}

	status := "pending_branch_acceptance"
	if in.PaymentMethod == "card" || in.PaymentMethod == "online_gateway" {
		status = "pending_payment"
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"order_id":       in.ClientOrderID,
		"branch_id":      branchID,
		"status":         status,
		"tracking_token": "trk_" + in.ClientOrderID,
	}, nil)
}
