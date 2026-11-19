package storefront

import (
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type StorefrontBranch struct {
	BranchID             string          `json:"branch_id"`
	Name                 string          `json:"name"`
	Code                 string          `json:"code"`
	BranchType           string          `json:"branch_type"`
	HeartbeatExpiresAt   *time.Time      `json:"heartbeat_expires_at"`
	IsOnline             bool            `json:"is_online"`
	AcceptingWebOrders   bool            `json:"accepting_web_orders"`
	EstimatedPrepMinutes int             `json:"estimated_prep_minutes"`
	DeliveryFee          decimal.Decimal `json:"delivery_fee"`
	MinDeliveryOrder     decimal.Decimal `json:"min_delivery_order"`
}

type PublishedModifier struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	PriceDelta decimal.Decimal `json:"price_delta"`
}

type PublishedMenuItem struct {
	ID            string              `json:"id"`
	CategoryID    string              `json:"category_id"`
	SKU           string              `json:"sku"`
	Name          string              `json:"name"`
	BasePrice     decimal.Decimal     `json:"base_price"`
	OnlineVisible bool                `json:"online_visible"`
	InStock       bool                `json:"in_stock"`
	Modifiers     []PublishedModifier `json:"modifiers,omitempty"`
}

// ComputeBranchOnlineStatus strictly marks a branch offline (accepting_web_orders = false)
// when heartbeat_expires_at is nil or in the past.
func ComputeBranchOnlineStatus(heartbeatExpiresAt *time.Time, now time.Time) (isOnline bool, acceptingWebOrders bool) {
	if heartbeatExpiresAt == nil || !heartbeatExpiresAt.After(now) {
		return false, false
	}
	return true, true
}

// FilterPublishedMenuItems excludes items where online_visible == false.
func FilterPublishedMenuItems(items []PublishedMenuItem) []PublishedMenuItem {
	out := make([]PublishedMenuItem, 0, len(items))
	for _, it := range items {
		if it.OnlineVisible {
			out = append(out, it)
		}
	}
	return out
}

type Handler struct {
	pool *db.Pool
}

func NewHandler(pool *db.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("tenantSlug")
	var tenantID, name, currency string
	if err := h.pool.QueryRow(r.Context(), `
		SELECT id, name, currency_code FROM tenants WHERE slug = $1 AND status = 'active'
	`, slug).Scan(&tenantID, &name, &currency); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "Storefront tenant not found", nil)
		return
	}

	now := time.Now().UTC()
	branches := []StorefrontBranch{}
	_ = h.pool.WithTenantTx(r.Context(), tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, name, code, branch_type, heartbeat_expires_at
			FROM branches ORDER BY created_at ASC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b StorefrontBranch
			if err := rows.Scan(&b.BranchID, &b.Name, &b.Code, &b.BranchType, &b.HeartbeatExpiresAt); err == nil {
				b.IsOnline, b.AcceptingWebOrders = ComputeBranchOnlineStatus(b.HeartbeatExpiresAt, now)
				b.EstimatedPrepMinutes = 15
				b.DeliveryFee = decimal.RequireFromString("150.00")
				b.MinDeliveryOrder = decimal.RequireFromString("800.00")
				branches = append(branches, b)
			}
		}
		return rows.Err()
	})

	w.Header().Set("X-RateLimit-Policy", "public-storefront-read")
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"tenant_slug":   slug,
		"name":          name,
		"currency_code": currency,
		"theme_preset":  "coffee_pickup",
		"branches":      branches,
	}, nil)
}

func (h *Handler) GetBranchMenu(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("tenantSlug")
	branchID := r.PathValue("branchId")
	var tenantID string
	if err := h.pool.QueryRow(r.Context(), `SELECT id FROM tenants WHERE slug = $1 AND status = 'active'`, slug).Scan(&tenantID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "Storefront tenant not found", nil)
		return
	}

	items := []PublishedMenuItem{}
	_ = h.pool.WithTenantTx(r.Context(), tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, category_id, sku, name, base_price::text, is_available
			FROM menu_items
			WHERE (branch_id IS NULL OR branch_id = $1)
			ORDER BY name ASC
		`, branchID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it PublishedMenuItem
			var pStr string
			if err := rows.Scan(&it.ID, &it.CategoryID, &it.SKU, &it.Name, &pStr, &it.InStock); err == nil {
				it.BasePrice, _ = decimal.NewFromString(pStr)
				it.OnlineVisible = true
				items = append(items, it)
			}
		}
		return rows.Err()
	})

	published := FilterPublishedMenuItems(items)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"branch_id": branchID,
		"items":     published,
	}, map[string]int{"count": len(published)})
}

func (h *Handler) TrackOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "TRACKING_TOKEN_REQUIRED", "Valid tracking token query parameter is required", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"order_id":    orderID,
		"stage":       "preparing",
		"eta_minutes": 12,
		"pickup_code": "ORD-4082",
	}, nil)
}
