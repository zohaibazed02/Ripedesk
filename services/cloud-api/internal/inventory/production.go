package inventory

import (
	"errors"

	"github.com/shopspring/decimal"
)

type ProductionBatchResult struct {
	TotalRawCost     decimal.Decimal `json:"total_raw_cost"`
	ActualYieldQty   decimal.Decimal `json:"actual_yield_qty"`
	RolledUpUnitCost decimal.Decimal `json:"rolled_up_unit_cost"`
	YieldVariancePct decimal.Decimal `json:"yield_variance_pct"`
}

// ComputeProductionBatchCost calculates the rolled-up actual unit cost
// (total_raw_cost / actual_yield_qty) and yield variance % for a kitchen prep batch.
func ComputeProductionBatchCost(
	rawConsumptions []IngredientConsumption,
	plannedYieldQty decimal.Decimal,
	actualYieldQty decimal.Decimal,
) (ProductionBatchResult, error) {
	if actualYieldQty.LessThanOrEqual(decimal.Zero) {
		return ProductionBatchResult{}, errors.New("actual_yield_qty must be > 0")
	}

	totalRawCost := decimal.Zero
	for _, c := range rawConsumptions {
		totalRawCost = totalRawCost.Add(c.LineCost)
	}
	unitCost := totalRawCost.Div(actualYieldQty).Round(4)

	yieldVarPct := decimal.Zero
	if plannedYieldQty.GreaterThan(decimal.Zero) {
		yieldVarPct = actualYieldQty.Sub(plannedYieldQty).
			Div(plannedYieldQty).
			Mul(decimal.NewFromInt(100)).
			Round(2)
	}

	return ProductionBatchResult{
		TotalRawCost:     totalRawCost.Round(4),
		ActualYieldQty:   actualYieldQty.Round(6),
		RolledUpUnitCost: unitCost,
		YieldVariancePct: yieldVarPct,
	}, nil
}

type VarianceLine struct {
	IngredientID   string          `json:"ingredient_id"`
	IngredientName string          `json:"ingredient_name"`
	BaseUnit       string          `json:"base_unit"`
	TheoreticalQty decimal.Decimal `json:"theoretical_qty"`
	ActualQty      decimal.Decimal `json:"actual_qty"`
	VarianceQty    decimal.Decimal `json:"variance_qty"` // actual - theoretical (positive = over-usage/shrinkage)
	VariancePct    decimal.Decimal `json:"variance_pct"`
	UnitCost       decimal.Decimal `json:"unit_cost"`
	VarianceValue  decimal.Decimal `json:"variance_value"`
}

func ComputeVarianceLine(
	ingID, ingName, baseUnit string,
	theoreticalQty, actualQty, unitCost decimal.Decimal,
) VarianceLine {
	varQty := actualQty.Sub(theoreticalQty).Round(6)
	varPct := decimal.Zero
	if theoreticalQty.GreaterThan(decimal.Zero) {
		varPct = varQty.Div(theoreticalQty).Mul(decimal.NewFromInt(100)).Round(2)
	}
	varVal := varQty.Mul(unitCost).Round(4)
	return VarianceLine{
		IngredientID:   ingID,
		IngredientName: ingName,
		BaseUnit:       baseUnit,
		TheoreticalQty: theoreticalQty.Round(6),
		ActualQty:      actualQty.Round(6),
		VarianceQty:    varQty,
		VariancePct:    varPct,
		UnitCost:       unitCost.Round(4),
		VarianceValue:  varVal,
	}
}
