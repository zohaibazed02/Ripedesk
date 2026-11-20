package sync

import (
	"testing"

	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/shopspring/decimal"
)

func TestKDSPrepConfirmation_LargeOatLatteExtraShotIdempotentReplay(t *testing.T) {
	ingredients := map[string]inventory.Ingredient{
		"ing-beans": {ID: "ing-beans", Name: "Espresso Beans", BaseUnit: "g", ValuationCost: decimal.RequireFromString("5.00")},
		"ing-oat":   {ID: "ing-oat", Name: "Oat Milk", BaseUnit: "ml", ValuationCost: decimal.RequireFromString("0.70")},
		"ing-cup":   {ID: "ing-cup", Name: "16oz Cup", BaseUnit: "pc", ValuationCost: decimal.RequireFromString("12.00")},
		"ing-lid":   {ID: "ing-lid", Name: "16oz Lid", BaseUnit: "pc", ValuationCost: decimal.RequireFromString("5.00")},
	}

	baseRecipe := &inventory.RecipeVersion{
		ID:            "rv-latte-v1",
		YieldQuantity: decimal.NewFromInt(1),
		Lines: []inventory.RecipeLine{
			{IngredientID: "ing-beans", QuantityBaseUnit: decimal.NewFromInt(18)},
			{IngredientID: "ing-oat", QuantityBaseUnit: decimal.NewFromInt(250)},
			{IngredientID: "ing-cup", QuantityBaseUnit: decimal.NewFromInt(1)},
			{IngredientID: "ing-lid", QuantityBaseUnit: decimal.NewFromInt(1)},
		},
	}
	extraShotRecipe := &inventory.RecipeVersion{
		ID:            "rv-shot-v1",
		YieldQuantity: decimal.NewFromInt(1),
		Lines: []inventory.RecipeLine{
			{IngredientID: "ing-beans", QuantityBaseUnit: decimal.NewFromInt(18)},
		},
	}

	payload := KDSPrepConfirmedPayload{
		TicketID:        "tk-f7-1001",
		OrderID:         "ord-f7-1001",
		Status:          "preparing",
		StockLocationID: "loc-f7-bar",
		Ingredients:     ingredients,
		Lines: []PrepLineInput{
			{
				MenuItemID:      "item-oat-latte",
				Quantity:        decimal.NewFromInt(1),
				MenuPrice:       decimal.RequireFromString("1170.00"),
				ItemRecipe:      baseRecipe,
				ModifierRecipes: []*inventory.RecipeVersion{extraShotRecipe},
			},
		},
	}

	// Simulate posting into an idempotency-guarded ledger store (mirrors UNIQUE(tenant_id, idempotency_key))
	appliedKeys := make(map[string]bool)
	deductedByIng := make(map[string]decimal.Decimal)

	applyOnce := func() int {
		applied := 0
		for _, m := range BuildPrepDeductionMovements("tn-1", "br-f7", "evt-kds-prep-001", payload) {
			if appliedKeys[m.IdempotencyKey] {
				continue
			}
			appliedKeys[m.IdempotencyKey] = true
			deductedByIng[m.IngredientID] = deductedByIng[m.IngredientID].Add(m.QuantityDelta)
			applied++
		}
		return applied
	}

	first := applyOnce()
	second := applyOnce() // Replay of exact same kds.ticket_updated batch

	if first != 4 {
		t.Fatalf("expected 4 ingredient deductions on first prep confirmation, got %d", first)
	}
	if second != 0 {
		t.Fatalf("expected 0 duplicate deductions on replay, got %d", second)
	}
	if !deductedByIng["ing-beans"].Equal(decimal.NewFromInt(-36)) {
		t.Fatalf("expected -36g beans (18g + 18g extra shot), got %s", deductedByIng["ing-beans"])
	}
	if !deductedByIng["ing-oat"].Equal(decimal.NewFromInt(-250)) {
		t.Fatalf("expected -250ml oat milk, got %s", deductedByIng["ing-oat"])
	}
	if !deductedByIng["ing-cup"].Equal(decimal.NewFromInt(-1)) || !deductedByIng["ing-lid"].Equal(decimal.NewFromInt(-1)) {
		t.Fatalf("expected -1 cup and -1 lid, got cup=%s lid=%s", deductedByIng["ing-cup"], deductedByIng["ing-lid"])
	}
}
