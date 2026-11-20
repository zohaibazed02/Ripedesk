package procurement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type ReplenishmentInput struct {
	SupplierID            string          `json:"supplier_id"`
	DestinationLocationID string          `json:"destination_location_id"`
	CurrencyCode          string          `json:"currency_code"`
	DeliveryWindow        string          `json:"delivery_window"`
	IngredientID          string          `json:"ingredient_id"`
	AvgDailyUsage         decimal.Decimal `json:"avg_daily_usage"`
	LeadTimeDays          decimal.Decimal `json:"lead_time_days"`
	SafetyStock           decimal.Decimal `json:"safety_stock"`
	BranchOnHand          decimal.Decimal `json:"branch_on_hand"`
	ValidatedOpenInbound  decimal.Decimal `json:"validated_open_inbound"`
	ExistingReservations  decimal.Decimal `json:"existing_reservations"`
	PackSize              decimal.Decimal `json:"pack_size"`
	MinOrderQty           decimal.Decimal `json:"min_order_qty"`
}

type ReplenishmentDecision struct {
	GroupKey         string          `json:"group_key"`
	IngredientID     string          `json:"ingredient_id"`
	ProjectedDemand  decimal.Decimal `json:"projected_demand"`
	TargetStock      decimal.Decimal `json:"target_stock"`
	NetAvailable     decimal.Decimal `json:"net_available"`
	RawShortage      decimal.Decimal `json:"raw_shortage"`
	RecommendedOrder decimal.Decimal `json:"recommended_order_qty"`
}

// EvaluateReplenishmentNeed computes exact Decimal replenishment quantities
// accounting for lead-time demand, safety stock, open inbound, reservations, pack size, and MOQ.
func EvaluateReplenishmentNeed(in ReplenishmentInput) ReplenishmentDecision {
	projected := in.AvgDailyUsage.Mul(in.LeadTimeDays).Round(6)
	target := projected.Add(in.SafetyStock).Round(6)
	netAvail := in.BranchOnHand.Add(in.ValidatedOpenInbound).Sub(in.ExistingReservations).Round(6)

	shortage := target.Sub(netAvail)
	if shortage.IsNegative() {
		shortage = decimal.Zero
	}
	orderQty := RoundUpToPackAndMOQ(shortage, in.PackSize, in.MinOrderQty)

	groupKey := fmt.Sprintf("%s|%s|%s|%s",
		in.SupplierID, in.DestinationLocationID, in.CurrencyCode, in.DeliveryWindow,
	)

	return ReplenishmentDecision{
		GroupKey:         groupKey,
		IngredientID:     in.IngredientID,
		ProjectedDemand:  projected,
		TargetStock:      target,
		NetAvailable:     netAvail,
		RawShortage:      shortage.Round(6),
		RecommendedOrder: orderQty,
	}
}

// GroupReplenishmentDecisions groups positive replenishment lines by
// (supplier_id, destination_location_id, currency_code, delivery_window).
func GroupReplenishmentDecisions(inputs []ReplenishmentInput) map[string][]ReplenishmentDecision {
	grouped := make(map[string][]ReplenishmentDecision)
	for _, in := range inputs {
		d := EvaluateReplenishmentNeed(in)
		if d.RecommendedOrder.GreaterThan(decimal.Zero) {
			grouped[d.GroupKey] = append(grouped[d.GroupKey], d)
		}
	}
	return grouped
}

// PersistedPODraft summarizes a PO draft created or deduplicated during replenishment.
type PersistedPODraft struct {
	POID           string `json:"po_id"`
	GroupKey       string `json:"group_key"`
	IdempotencyKey string `json:"idempotency_key"`
	LineCount      int    `json:"line_count"`
	Created        bool   `json:"created"`
}

// DeterministicReplenishmentKey computes a stable idempotency key per (tenant, groupKey, runDate)
// so concurrent or repeated replenishment runs never create duplicate PO drafts.
func DeterministicReplenishmentKey(tenantID, groupKey, runDate string) string {
	sum := sha256.Sum256([]byte("replenish:" + tenantID + ":" + groupKey + ":" + runDate))
	return "repl-" + hex.EncodeToString(sum[:16])
}

// PersistReplenishmentDraftsTx persists grouped replenishment decisions into purchase_orders
// and purchase_order_lines inside the caller's tenant transaction with idempotency protection.
func PersistReplenishmentDraftsTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	runDate string,
	grouped map[string][]ReplenishmentDecision,
) ([]PersistedPODraft, error) {
	drafts := make([]PersistedPODraft, 0, len(grouped))
	for groupKey, lines := range grouped {
		if len(lines) == 0 {
			continue
		}
		parts := strings.Split(groupKey, "|")
		supplierID, destLocID, currency := "", "", "PKR"
		if len(parts) > 0 {
			supplierID = parts[0]
		}
		if len(parts) > 1 {
			destLocID = parts[1]
		}
		if len(parts) > 2 && parts[2] != "" {
			currency = parts[2]
		}

		idemKey := DeterministicReplenishmentKey(tenantID, groupKey, runDate)
		var poID string
		err := tx.QueryRow(ctx, `
			INSERT INTO purchase_orders (
				tenant_id, supplier_id, destination_location_id, currency_code,
				status, idempotency_key
			)
			VALUES ($1, $2, $3, $4, 'draft', $5)
			ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
			RETURNING id::text
		`, tenantID, supplierID, destLocID, currency, idemKey).Scan(&poID)

		if err == pgx.ErrNoRows {
			// Already created by an earlier or concurrent replenishment run
			_ = tx.QueryRow(ctx, `
				SELECT id::text FROM purchase_orders WHERE tenant_id = $1 AND idempotency_key = $2
			`, tenantID, idemKey).Scan(&poID)
			drafts = append(drafts, PersistedPODraft{
				POID:           poID,
				GroupKey:       groupKey,
				IdempotencyKey: idemKey,
				LineCount:      len(lines),
				Created:        false,
			})
			continue
		}
		if err != nil {
			return nil, err
		}

		for _, line := range lines {
			if _, err := tx.Exec(ctx, `
				INSERT INTO purchase_order_lines (
					tenant_id, purchase_order_id, ingredient_id, ordered_qty_base
				)
				VALUES ($1, $2, $3, $4::numeric)
			`, tenantID, poID, line.IngredientID, line.RecommendedOrder.String()); err != nil {
				return nil, err
			}
		}

		drafts = append(drafts, PersistedPODraft{
			POID:           poID,
			GroupKey:       groupKey,
			IdempotencyKey: idemKey,
			LineCount:      len(lines),
			Created:        true,
		})
	}
	return drafts, nil
}

func (h *Handler) RunReplenishment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in struct {
		RunDate string               `json:"run_date"`
		Items   []ReplenishmentInput `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	if in.RunDate == "" {
		in.RunDate = time.Now().UTC().Format("2006-01-02")
	}
	grouped := GroupReplenishmentDecisions(in.Items)

	var drafts []PersistedPODraft
	if h.pool != nil {
		err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
			var err error
			drafts, err = PersistReplenishmentDraftsTx(r.Context(), tx, claims.TenantID, in.RunDate, grouped)
			return err
		})
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "REPLENISHMENT_PERSIST_FAILED", err.Error(), nil)
			return
		}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"groups": grouped,
		"drafts": drafts,
	}, map[string]int{"po_groups": len(grouped)})
}
