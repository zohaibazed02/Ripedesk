package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type stubReadOnlyDS struct{}

func (s *stubReadOnlyDS) GetMarginDrivers(_ context.Context, _ string, _ []string, _, _ string) ([]MarginDriver, error) {
	return []MarginDriver{
		{
			ItemOrIngredient: "Oat Milk (1L)",
			BranchName:       "F-7 Coffee Bar",
			ImpactPKR:        decimal.RequireFromString("18450.00"),
			ImpactPct:        decimal.RequireFromString("21.40"),
			Category:         "ingredient_cost_spike",
		},
		{
			ItemOrIngredient: "Espresso Beans",
			BranchName:       "G-9 Flagship Dine-In",
			ImpactPKR:        decimal.RequireFromString("9200.00"),
			ImpactPct:        decimal.RequireFromString("10.00"),
			Category:         "stock_variance",
		},
	}, nil
}

func (s *stubReadOnlyDS) GetBranchFreshnessAndGaps(_ context.Context, _ string, _ []string) ([]BranchFreshnessGap, error) {
	now := time.Now().UTC()
	return []BranchFreshnessGap{
		{
			BranchID:          "b-f7",
			BranchName:        "F-7 Coffee Bar",
			LastSyncAt:        now.Add(-1 * time.Minute),
			PendingSyncCount:  0,
			IsStale:           false,
			MissingStockCount: false,
		},
		{
			BranchID:          "b-g9",
			BranchName:        "G-9 Flagship Dine-In",
			LastSyncAt:        now.Add(-9 * time.Minute),
			PendingSyncCount:  3,
			IsStale:           true,
			MissingStockCount: true,
		},
	}, nil
}

func (s *stubReadOnlyDS) GetWasteAndVarianceHotspots(_ context.Context, _ string, _ []string, _, _ string) ([]WasteVarianceHotspot, error) {
	return []WasteVarianceHotspot{
		{
			IngredientName: "Tiramisu Prep",
			BranchName:     "G-9 Flagship Dine-In",
			WastePKR:       decimal.RequireFromString("6400.00"),
			VariancePKR:    decimal.RequireFromString("1800.00"),
			BaseQuantity:   decimal.RequireFromString("12.00"),
			BaseUnit:       "portions",
		},
	}, nil
}

func (s *stubReadOnlyDS) GetReorderSuggestions(_ context.Context, _ string, _ []string) ([]ReorderSuggestion, error) {
	return []ReorderSuggestion{
		{
			IngredientName:      "Oat Milk (1L)",
			BranchName:          "F-7 Coffee Bar",
			CurrentStock:        decimal.RequireFromString("14.00"),
			SafetyStock:         decimal.RequireFromString("30.00"),
			OpenInboundPOQty:    decimal.Zero,
			SuggestedReorderQty: decimal.RequireFromString("36.00"),
			Unit:                "cartons",
			EstimatedCostPKR:    decimal.RequireFromString("25200.00"),
		},
	}, nil
}

func (s *stubReadOnlyDS) GetSalesAndRefundSummary(_ context.Context, _ string, _ []string, _, _ string) (SalesRefundSummary, error) {
	return SalesRefundSummary{
		NetSales:       decimal.RequireFromString("421500.00"),
		Refunds:        decimal.RequireFromString("3300.00"),
		Voids:          decimal.RequireFromString("1200.00"),
		ReconciledToGL: true,
		GLDiffPKR:      decimal.Zero,
	}, nil
}

func TestToolAllowlist_RejectsUnapprovedAndMutatingTools(t *testing.T) {
	reg := NewToolRegistry(&stubReadOnlyDS{})
	execCtx := ToolExecutionContext{
		TenantID:    "t-1",
		ActorUserID: "u-owner",
		ActorRole:   "owner",
		FromDate:    "2026-11-14",
		ToDate:      "2026-11-20",
	}

	for _, forbiddenTool := range []string{"delete_order", "auto_create_po", "exec_sql"} {
		_, err := reg.Execute(context.Background(), execCtx, forbiddenTool)
		if !errors.Is(err, ErrUnapprovedTool) {
			t.Fatalf("expected ErrUnapprovedTool for %q, got %v", forbiddenTool, err)
		}
	}
}

func TestBranchScope_RejectsCrossBranchManagerQuery(t *testing.T) {
	reg := NewToolRegistry(&stubReadOnlyDS{})
	agent := NewAgent(reg, &GroundedAnalyticalExplainer{})

	execCtx := ToolExecutionContext{
		TenantID:           "t-1",
		ActorUserID:        "u-mgr-f7",
		ActorRole:          "branch_manager",
		AllowedBranchIDs:   []string{"b-f7"},
		RequestedBranchIDs: []string{"b-g9"}, // Out of scope!
		FromDate:           "2026-11-14",
		ToDate:             "2026-11-20",
	}

	_, err := agent.Ask(context.Background(), execCtx, "Why did margins drop?")
	if !errors.Is(err, ErrForbiddenBranchScope) {
		t.Fatalf("expected ErrForbiddenBranchScope, got %v", err)
	}
}

func TestAgentAsk_FallingMarginsGroundedResponse(t *testing.T) {
	reg := NewToolRegistry(&stubReadOnlyDS{})
	agent := NewAgent(reg, &GroundedAnalyticalExplainer{})

	execCtx := ToolExecutionContext{
		TenantID:    "t-1",
		ActorUserID: "u-owner",
		ActorRole:   "owner",
		FromDate:    "2026-11-14",
		ToDate:      "2026-11-20",
	}

	resp, err := agent.Ask(context.Background(), execCtx, "Why did margins drop this week?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DateRange.From != "2026-11-14" || resp.DateRange.To != "2026-11-20" {
		t.Fatalf("unexpected date range: %+v", resp.DateRange)
	}
	if len(resp.ContributingFactors) != 2 {
		t.Fatalf("expected 2 contributing factors, got %d", len(resp.ContributingFactors))
	}
	if !resp.ContributingFactors[0].ImpactPKR.Equal(decimal.RequireFromString("18450.00")) {
		t.Fatalf("expected exact decimal 18450.00, got %s", resp.ContributingFactors[0].ImpactPKR)
	}
	if len(resp.DataQualityWarnings) < 2 {
		t.Fatalf("expected stale branch + missing stock count warnings, got %v", resp.DataQualityWarnings)
	}
	if len(resp.HumanReviewSuggestions) == 0 {
		t.Fatalf("expected human review suggestions")
	}
	for _, s := range resp.HumanReviewSuggestions {
		if !s.RequiresHumanApproval {
			t.Fatalf("every suggestion must require human approval: %+v", s)
		}
	}
	if len(resp.UnderlyingReportLinks) < 4 {
		t.Fatalf("expected underlying report links, got %d", len(resp.UnderlyingReportLinks))
	}
	if resp.AuditMetadata.ModelVersion != "grounded-analyst-v1.0" || !resp.AuditMetadata.ReadOnlyEnforced {
		t.Fatalf("unexpected audit metadata: %+v", resp.AuditMetadata)
	}
}
