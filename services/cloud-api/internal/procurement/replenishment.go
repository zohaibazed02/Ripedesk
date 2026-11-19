package procurement

import (
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
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

func (h *Handler) RunReplenishment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []ReplenishmentInput `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	grouped := GroupReplenishmentDecisions(in.Items)
	httpx.WriteJSON(w, http.StatusOK, grouped, map[string]int{"po_groups": len(grouped)})
}
