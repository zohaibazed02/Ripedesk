package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// PrepLineInput represents one menu line confirmed for preparation on a KDS ticket.
type PrepLineInput struct {
	MenuItemID      string                     `json:"menu_item_id"`
	Quantity        decimal.Decimal            `json:"quantity"`
	MenuPrice       decimal.Decimal            `json:"menu_price"`
	ItemRecipe      *inventory.RecipeVersion   `json:"item_recipe,omitempty"`
	ModifierRecipes []*inventory.RecipeVersion `json:"modifier_recipes,omitempty"`
}

// KDSPrepConfirmedPayload is the payload for kds.ticket_updated commands.
type KDSPrepConfirmedPayload struct {
	TicketID        string                          `json:"ticket_id"`
	OrderID         string                          `json:"order_id"`
	Status          string                          `json:"status"`
	StockLocationID string                          `json:"stock_location_id"`
	Lines           []PrepLineInput                 `json:"lines,omitempty"`
	Ingredients     map[string]inventory.Ingredient `json:"ingredients,omitempty"`
}

// BuildPrepDeductionMovements explodes versioned recipes + modifiers for a KDS ticket
// transitioning to "preparing" and returns idempotent prep_consumption movements keyed by commandID.
func BuildPrepDeductionMovements(tenantID, branchID, commandID string, p KDSPrepConfirmedPayload) []inventory.MovementInput {
	if p.Status != "preparing" || len(p.Lines) == 0 {
		return nil
	}

	accum := make(map[string]decimal.Decimal)
	costs := make(map[string]decimal.Decimal)
	order := make([]string, 0)

	for _, line := range p.Lines {
		qty := line.Quantity
		if qty.LessThanOrEqual(decimal.Zero) {
			qty = decimal.NewFromInt(1)
		}
		exp := inventory.ExplodeOrderLine(qty, line.MenuPrice, line.ItemRecipe, line.ModifierRecipes, p.Ingredients)
		for _, c := range exp.Consumptions {
			if _, exists := accum[c.IngredientID]; !exists {
				order = append(order, c.IngredientID)
				costs[c.IngredientID] = c.UnitCost
			}
			accum[c.IngredientID] = accum[c.IngredientID].Add(c.TotalBaseQty)
		}
	}

	movements := make([]inventory.MovementInput, 0, len(order))
	locID := p.StockLocationID
	if locID == "" {
		locID = branchID
	}
	sourceID := p.TicketID
	if sourceID == "" {
		sourceID = commandID
	}

	for _, ingID := range order {
		qty := accum[ingID]
		if qty.LessThanOrEqual(decimal.Zero) {
			continue
		}
		movements = append(movements, inventory.MovementInput{
			TenantID:        tenantID,
			BranchID:        branchID,
			StockLocationID: locID,
			IngredientID:    ingID,
			MovementType:    "prep_consumption",
			QuantityDelta:   qty.Neg(),
			UnitCost:        costs[ingID],
			SourceDocType:   "kds_ticket",
			SourceDocID:     sourceID,
			IdempotencyKey:  fmt.Sprintf("prep:%s:%s", commandID, ingID),
		})
	}
	return movements
}

// ApplyKDSPrepDeductionTx posts idempotent prep_consumption movements inside the caller's tenant tx.
func ApplyKDSPrepDeductionTx(ctx context.Context, tx pgx.Tx, tenantID, branchID, commandID string, rawPayload json.RawMessage) error {
	var p KDSPrepConfirmedPayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return nil
	}
	movements := BuildPrepDeductionMovements(tenantID, branchID, commandID, p)
	for _, m := range movements {
		if _, err := inventory.PostStockMovementTx(ctx, tx, m); err != nil {
			return err
		}
	}
	return nil
}
