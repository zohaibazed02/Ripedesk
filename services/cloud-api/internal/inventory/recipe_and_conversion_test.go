package inventory_test

import (
	"errors"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/shopspring/decimal"
)

func TestMultiStepPackConversionAndUnmappedRejection(t *testing.T) {
	conversions := []inventory.UnitConversion{
		{FromUnit: "case", ToUnit: "L", Multiplier: decimal.NewFromInt(12)},
		{FromUnit: "L", ToUnit: "ml", Multiplier: decimal.NewFromInt(1000)},
	}

	// 1 case = 12 L = 12,000 ml
	got, err := inventory.ConvertToBaseUnit("ml", decimal.NewFromInt(1), "case", conversions)
	if err != nil {
		t.Fatalf("ConvertToBaseUnit failed: %v", err)
	}
	want := decimal.RequireFromString("12000.000000")
	if !got.Equal(want) {
		t.Fatalf("expected %s ml, got %s", want, got)
	}

	// Unmapped unit must be explicitly rejected
	_, err = inventory.ConvertToBaseUnit("ml", decimal.NewFromInt(2), "barrel", conversions)
	if !errors.Is(err, inventory.ErrUnmappedUnit) {
		t.Fatalf("expected ErrUnmappedUnit, got %v", err)
	}
}

func TestLargeOatLattePlusExtraShotExplosionAndCOGS(t *testing.T) {
	ingredients := map[string]inventory.Ingredient{
		"ing-beans": {ID: "ing-beans", Name: "House Espresso Beans", BaseUnit: "g", ValuationCost: decimal.RequireFromString("4.50")},
		"ing-oat":   {ID: "ing-oat", Name: "Barista Oat Milk", BaseUnit: "ml", ValuationCost: decimal.RequireFromString("0.85")},
		"ing-cup":   {ID: "ing-cup", Name: "16oz Paper Cup", BaseUnit: "pcs", ValuationCost: decimal.RequireFromString("22.00")},
		"ing-lid":   {ID: "ing-lid", Name: "16oz Sip Lid", BaseUnit: "pcs", ValuationCost: decimal.RequireFromString("8.50")},
	}

	latteRecipe := &inventory.RecipeVersion{
		VersionNumber: 1,
		YieldQuantity: decimal.NewFromInt(1),
		Lines: []inventory.RecipeLine{
			{IngredientID: "ing-beans", QuantityBaseUnit: decimal.NewFromInt(18)},
			{IngredientID: "ing-oat", QuantityBaseUnit: decimal.NewFromInt(250)},
			{IngredientID: "ing-cup", QuantityBaseUnit: decimal.NewFromInt(1)},
			{IngredientID: "ing-lid", QuantityBaseUnit: decimal.NewFromInt(1)},
		},
	}
	extraShotRecipe := &inventory.RecipeVersion{
		VersionNumber: 1,
		YieldQuantity: decimal.NewFromInt(1),
		Lines: []inventory.RecipeLine{
			{IngredientID: "ing-beans", QuantityBaseUnit: decimal.NewFromInt(18)},
		},
	}

	// Menu price = 950 + 220 = 1170 PKR
	res := inventory.ExplodeOrderLine(
		decimal.NewFromInt(1),
		decimal.RequireFromString("1170.00"),
		latteRecipe,
		[]*inventory.RecipeVersion{extraShotRecipe},
		ingredients,
	)

	// Beans: 36g * 4.50 = 162.00
	// Oat Milk: 250ml * 0.85 = 212.50
	// Cup: 1 * 22.00 = 22.00
	// Lid: 1 * 8.50 = 8.50
	// Total COGS = 405.00
	wantCOGS := decimal.RequireFromString("405.0000")
	if !res.TheoreticalCOGS.Equal(wantCOGS) {
		t.Fatalf("expected COGS %s, got %s", wantCOGS, res.TheoreticalCOGS)
	}
	if res.Consumptions[0].TotalBaseQty.String() != "36" {
		t.Fatalf("expected 36g beans, got %s", res.Consumptions[0].TotalBaseQty)
	}
}
