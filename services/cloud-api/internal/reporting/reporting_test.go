package reporting

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestComputeOwnerSummary_ExactDecimalAndZeroOrders(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

	// 1. Zero orders -> safe zero-division (no panic, AOV = 0.00)
	zeroRes := ComputeOwnerSummary(OwnerSummaryInput{
		PeriodFrom: "2026-11-20",
		PeriodTo:   "2026-11-20",
		OrderCount: 0,
	}, now)
	if !zeroRes.AverageOrderValue.Equal(decimal.Zero) {
		t.Fatalf("expected 0 AOV for zero orders, got %s", zeroRes.AverageOrderValue)
	}
	if !zeroRes.GLReconciliation.ReconciledToGL {
		t.Fatalf("expected zero-order summary to be reconciled to GL")
	}

	// 2. Non-zero exact decimal calculations
	res := ComputeOwnerSummary(OwnerSummaryInput{
		PeriodFrom:   "2026-11-20",
		PeriodTo:     "2026-11-20",
		GrossSales:   decimal.RequireFromString("100000.00"),
		Discounts:    decimal.RequireFromString("5000.00"),
		Refunds:      decimal.RequireFromString("2500.00"),
		TaxCollected: decimal.RequireFromString("14800.00"),
		OrderCount:   100,
		Collections: CollectionsBreakdown{
			Cash:   decimal.RequireFromString("40000.00"),
			Card:   decimal.RequireFromString("35000.00"),
			Online: decimal.RequireFromString("12500.00"),
			COD:    decimal.RequireFromString("5000.00"),
		},
		FoodCostTotal:      decimal.RequireFromString("27750.00"), // 30.00% of 92,500
		WasteTotal:         decimal.RequireFromString("1850.00"),  // 2.00% of 92,500
		StockVarianceTotal: decimal.RequireFromString("925.00"),   // 1.00% of 92,500
		GLRevenueTotal:     decimal.RequireFromString("92500.00"),
		PrepDurationsSec:   []int{120, 180, 240, 300, 600},
		OverdueTicketCount: 1,
	}, now)

	if !res.NetSales.Equal(decimal.RequireFromString("92500.00")) {
		t.Fatalf("expected NetSales 92500.00, got %s", res.NetSales)
	}
	if !res.AverageOrderValue.Equal(decimal.RequireFromString("925.00")) {
		t.Fatalf("expected AOV 925.00, got %s", res.AverageOrderValue)
	}
	if !res.FoodCostPct.Equal(decimal.RequireFromString("30.00")) {
		t.Fatalf("expected FoodCostPct 30.00, got %s", res.FoodCostPct)
	}
	if !res.GrossMarginPct.Equal(decimal.RequireFromString("70.00")) {
		t.Fatalf("expected GrossMarginPct 70.00, got %s", res.GrossMarginPct)
	}
	if !res.GLReconciliation.ReconciledToGL || !res.GLReconciliation.GLDiff.IsZero() {
		t.Fatalf("expected GL reconciled with 0 diff, got %+v", res.GLReconciliation)
	}
	if !res.KitchenSLA.OnTimeRatePct.Equal(decimal.RequireFromString("80.00")) {
		t.Fatalf("expected OnTimeRatePct 80.00, got %s", res.KitchenSLA.OnTimeRatePct)
	}
}

func TestComputeOwnerSummary_GLMismatchAndStaleness(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

	res := ComputeOwnerSummary(OwnerSummaryInput{
		GrossSales:     decimal.RequireFromString("50000.00"),
		GLRevenueTotal: decimal.RequireFromString("49500.00"),
		OrderCount:     25,
		Branches: []BranchFreshness{
			{
				BranchID:         "b-f7",
				BranchName:       "F-7 Coffee Bar",
				LastSyncAt:       now.Add(-2 * time.Minute),
				PendingSyncCount: 0,
			},
			{
				BranchID:          "b-g9",
				BranchName:        "G-9 Flagship Dine-In",
				LastSyncAt:        now.Add(-8 * time.Minute), // > 5m -> stale
				PendingSyncCount:  3,
				MissingStockCount: true,
			},
		},
	}, now)

	if res.GLReconciliation.ReconciledToGL {
		t.Fatalf("expected GL mismatch, got reconciled=true")
	}
	if !res.GLReconciliation.GLDiff.Equal(decimal.RequireFromString("500.00")) {
		t.Fatalf("expected GL diff 500.00, got %s", res.GLReconciliation.GLDiff)
	}
	if !res.IsStale {
		t.Fatalf("expected IsStale=true when branch sync > 5m and pending > 0")
	}
	if len(res.StaleBranchIDs) != 1 || res.StaleBranchIDs[0] != "b-g9" {
		t.Fatalf("expected stale branch [b-g9], got %v", res.StaleBranchIDs)
	}
	if res.TotalPendingOfflineOrders != 3 {
		t.Fatalf("expected 3 pending offline orders, got %d", res.TotalPendingOfflineOrders)
	}
	if len(res.MissingStockCountBranches) != 1 || res.MissingStockCountBranches[0] != "b-g9" {
		t.Fatalf("expected missing stock count [b-g9], got %v", res.MissingStockCountBranches)
	}
}

func TestComputeBranchComparison_RankingsAndDeltas(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

	comp := ComputeBranchComparison([]BranchComparisonInput{
		{
			BranchID:         "b-f7",
			BranchName:       "F-7 Coffee Bar",
			BranchCode:       "F7",
			BranchType:       "coffee_shop",
			CurrentNetSales:  decimal.RequireFromString("120000.00"),
			PriorNetSales:    decimal.RequireFromString("100000.00"), // +20.00%
			OrderCount:       200,
			FoodCostTotal:    decimal.RequireFromString("30000.00"), // 25% food cost -> 75% margin
			PrepDurationsSec: []int{90, 110, 130},
			LastSyncAt:       now.Add(-1 * time.Minute),
		},
		{
			BranchID:           "b-g9",
			BranchName:         "G-9 Flagship Dine-In",
			BranchCode:         "G9",
			BranchType:         "dine_in",
			CurrentNetSales:    decimal.RequireFromString("250000.00"),
			PriorNetSales:      decimal.RequireFromString("250000.00"), // 0.00%
			OrderCount:         125,
			FoodCostTotal:      decimal.RequireFromString("87500.00"), // 35% food cost -> 65% margin
			PrepDurationsSec:   []int{420, 540, 960},
			OverdueTicketCount: 1,
			LastSyncAt:         now.Add(-2 * time.Minute),
		},
	}, now)

	if !comp.TotalNetSales.Equal(decimal.RequireFromString("370000.00")) {
		t.Fatalf("expected TotalNetSales 370000.00, got %s", comp.TotalNetSales)
	}
	if comp.Branches[0].SalesRank != 2 || comp.Branches[1].SalesRank != 1 {
		t.Fatalf("unexpected sales ranks: F7=%d, G9=%d", comp.Branches[0].SalesRank, comp.Branches[1].SalesRank)
	}
	if comp.Branches[0].MarginRank != 1 || comp.Branches[1].MarginRank != 2 {
		t.Fatalf("unexpected margin ranks: F7=%d, G9=%d", comp.Branches[0].MarginRank, comp.Branches[1].MarginRank)
	}
	if !comp.Branches[0].NetSalesDeltaPct.Equal(decimal.RequireFromString("20.00")) {
		t.Fatalf("expected F7 delta +20.00%%, got %s", comp.Branches[0].NetSalesDeltaPct)
	}
}
