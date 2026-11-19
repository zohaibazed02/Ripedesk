package inventory_test

import (
	"testing"

	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/shopspring/decimal"
)

func TestWeightedAvgCostAndProductionBatchRollup(t *testing.T) {
	// 1. Weighted average cost: 1000g @ 4.00 + 1000g @ 5.00 = 2000g @ 4.5000
	avg := inventory.ComputeWeightedAvgCost(
		decimal.NewFromInt(1000),
		decimal.RequireFromString("4.00"),
		decimal.NewFromInt(1000),
		decimal.RequireFromString("5.00"),
	)
	if !avg.Equal(decimal.RequireFromString("4.5000")) {
		t.Fatalf("expected weighted avg 4.5000, got %s", avg)
	}

	// 2. Production batch: Tiramisu tray planned 12 pcs, actual 10 pcs, raw cost 3210 PKR -> 321.0000 PKR/pc
	batch, err := inventory.ComputeProductionBatchCost(
		[]inventory.IngredientConsumption{
			{IngredientID: "ing-beans", LineCost: decimal.RequireFromString("810.00")},
			{IngredientID: "ing-masc", LineCost: decimal.RequireFromString("2400.00")},
		},
		decimal.NewFromInt(12),
		decimal.NewFromInt(10),
	)
	if err != nil {
		t.Fatalf("ComputeProductionBatchCost failed: %v", err)
	}
	if !batch.RolledUpUnitCost.Equal(decimal.RequireFromString("321.0000")) {
		t.Fatalf("expected rolled-up unit cost 321.0000, got %s", batch.RolledUpUnitCost)
	}
	if !batch.YieldVariancePct.Equal(decimal.RequireFromString("-16.67")) {
		t.Fatalf("expected yield variance -16.67%%, got %s", batch.YieldVariancePct)
	}
}

func TestTheoreticalVsActualVariance(t *testing.T) {
	// Theoretical 3,600g beans, Actual 3,960g beans -> +360g (+10.00%) shrinkage @ 4.50 = 1,620.00 PKR loss
	v := inventory.ComputeVarianceLine(
		"ing-beans",
		"House Espresso Beans",
		"g",
		decimal.NewFromInt(3600),
		decimal.NewFromInt(3960),
		decimal.RequireFromString("4.50"),
	)
	if !v.VarianceQty.Equal(decimal.NewFromInt(360)) {
		t.Fatalf("expected variance qty 360, got %s", v.VarianceQty)
	}
	if !v.VariancePct.Equal(decimal.RequireFromString("10.00")) {
		t.Fatalf("expected variance pct 10.00, got %s", v.VariancePct)
	}
	if !v.VarianceValue.Equal(decimal.RequireFromString("1620.0000")) {
		t.Fatalf("expected variance value 1620.0000, got %s", v.VarianceValue)
	}
}
