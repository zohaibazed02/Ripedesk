package inventory

import (
	"time"

	"github.com/shopspring/decimal"
)

type Ingredient struct {
	ID                string           `json:"id"`
	SKU               string           `json:"sku"`
	Name              string           `json:"name"`
	BaseUnit          string           `json:"base_unit"`
	PurchasingUnit    string           `json:"purchasing_unit"`
	ConversionFactor  decimal.Decimal  `json:"conversion_factor"`
	TrackLotExpiry    bool             `json:"track_lot_expiry"`
	LowStockThreshold decimal.Decimal  `json:"low_stock_threshold"`
	ValuationCost     decimal.Decimal  `json:"valuation_cost"`
	IsActive          bool             `json:"is_active"`
	Conversions       []UnitConversion `json:"unit_conversions"`
}

type RecipeLine struct {
	ID               string          `json:"id,omitempty"`
	IngredientID     string          `json:"ingredient_id"`
	IngredientName   string          `json:"ingredient_name,omitempty"`
	BaseUnit         string          `json:"base_unit,omitempty"`
	QuantityBaseUnit decimal.Decimal `json:"quantity_base_unit"`
	WasteFactorPct   decimal.Decimal `json:"waste_factor_pct"`
}

type RecipeVersion struct {
	ID            string          `json:"id"`
	MenuItemID    *string         `json:"menu_item_id,omitempty"`
	ModifierID    *string         `json:"modifier_id,omitempty"`
	VersionNumber int             `json:"version_number"`
	IsActive      bool            `json:"is_active"`
	YieldQuantity decimal.Decimal `json:"yield_quantity"`
	EffectiveFrom time.Time       `json:"effective_from"`
	Lines         []RecipeLine    `json:"recipe_lines"`
}

type IngredientConsumption struct {
	IngredientID   string          `json:"ingredient_id"`
	IngredientName string          `json:"ingredient_name"`
	BaseUnit       string          `json:"base_unit"`
	TotalBaseQty   decimal.Decimal `json:"total_base_qty"`
	UnitCost       decimal.Decimal `json:"unit_cost"`
	LineCost       decimal.Decimal `json:"line_cost"`
}

type OrderLineExplosion struct {
	MenuPrice       decimal.Decimal         `json:"menu_price"`
	TheoreticalCOGS decimal.Decimal         `json:"theoretical_cogs"`
	GrossMarginPct  decimal.Decimal         `json:"gross_margin_pct"`
	FoodCostPct     decimal.Decimal         `json:"food_cost_pct"`
	Consumptions    []IngredientConsumption `json:"consumptions"`
}

// ExplodeOrderLine deterministically aggregates base-unit ingredient consumption
// across a base item recipe + selected modifier recipes, applying waste_factor_pct
// and computing exact theoretical COGS, Food Cost %, and Gross Margin %.
func ExplodeOrderLine(
	quantity decimal.Decimal,
	menuPrice decimal.Decimal,
	itemRecipe *RecipeVersion,
	modifierRecipes []*RecipeVersion,
	ingredients map[string]Ingredient,
) OrderLineExplosion {
	hundred := decimal.NewFromInt(100)
	accum := make(map[string]decimal.Decimal)
	order := []string{}

	addLines := func(rv *RecipeVersion) {
		if rv == nil {
			return
		}
		yield := rv.YieldQuantity
		if yield.LessThanOrEqual(decimal.Zero) {
			yield = decimal.NewFromInt(1)
		}
		for _, line := range rv.Lines {
			// Effective qty = (quantity_base_unit / yield) * order_qty * (1 + waste_pct / 100)
			wasteMult := decimal.NewFromInt(1).Add(line.WasteFactorPct.Div(hundred))
			effQty := line.QuantityBaseUnit.Div(yield).Mul(quantity).Mul(wasteMult)
			if _, exists := accum[line.IngredientID]; !exists {
				order = append(order, line.IngredientID)
			}
			accum[line.IngredientID] = accum[line.IngredientID].Add(effQty)
		}
	}

	addLines(itemRecipe)
	for _, mr := range modifierRecipes {
		addLines(mr)
	}

	consumptions := make([]IngredientConsumption, 0, len(order))
	totalCOGS := decimal.Zero

	for _, ingID := range order {
		ing := ingredients[ingID]
		qty := accum[ingID].Round(6)
		lineCost := qty.Mul(ing.ValuationCost).Round(4)
		totalCOGS = totalCOGS.Add(lineCost)
		consumptions = append(consumptions, IngredientConsumption{
			IngredientID:   ingID,
			IngredientName: ing.Name,
			BaseUnit:       ing.BaseUnit,
			TotalBaseQty:   qty,
			UnitCost:       ing.ValuationCost,
			LineCost:       lineCost,
		})
	}

	totalRevenue := menuPrice.Mul(quantity).Round(4)
	foodCostPct := decimal.Zero
	grossMarginPct := decimal.Zero
	if totalRevenue.GreaterThan(decimal.Zero) {
		foodCostPct = totalCOGS.Div(totalRevenue).Mul(hundred).Round(2)
		grossMarginPct = hundred.Sub(foodCostPct).Round(2)
	}

	return OrderLineExplosion{
		MenuPrice:       totalRevenue,
		TheoreticalCOGS: totalCOGS.Round(4),
		FoodCostPct:     foodCostPct,
		GrossMarginPct:  grossMarginPct,
		Consumptions:    consumptions,
	}
}
