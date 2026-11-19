package procurement_test

import (
	"testing"

	"github.com/hospitality-os/cloud-api/internal/procurement"
	"github.com/shopspring/decimal"
)

func TestPilotSplitSourcingWithPackSizeAndMOQ(t *testing.T) {
	// Scenario 1: Branch requests 12 L Oat Milk; Warehouse has 8 L; Inbound = 0 L; Pack = 6 L; MOQ = 6 L.
	// Expect: 8 L warehouse transfer + 4 L uncovered rounded up to 6 L PO draft.
	res1 := procurement.EvaluateSplitSourcing(
		decimal.NewFromInt(12),
		decimal.NewFromInt(8),
		decimal.Zero,
		decimal.NewFromInt(6),
		decimal.NewFromInt(6),
	)
	if !res1.WarehouseAllocated.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("expected 8 L warehouse allocated, got %s", res1.WarehouseAllocated)
	}
	if !res1.UncoveredShortage.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("expected 4 L uncovered shortage, got %s", res1.UncoveredShortage)
	}
	if !res1.POOrderQty.Equal(decimal.NewFromInt(6)) {
		t.Fatalf("expected 6 L PO order qty (1x6L pack), got %s", res1.POOrderQty)
	}

	// Scenario 2: Same request with 2 L existing confirmed inbound -> 12 - 8 - 2 = 2 L uncovered -> 6 L MOQ/pack
	res2 := procurement.EvaluateSplitSourcing(
		decimal.NewFromInt(12),
		decimal.NewFromInt(8),
		decimal.NewFromInt(2),
		decimal.NewFromInt(6),
		decimal.NewFromInt(6),
	)
	if !res2.UncoveredShortage.Equal(decimal.NewFromInt(2)) || !res2.POOrderQty.Equal(decimal.NewFromInt(6)) {
		t.Fatalf("unexpected inbound split result: %+v", res2)
	}
}

func TestReplenishmentPlannerAndGrouping(t *testing.T) {
	// avgDaily=5, leadTime=3 => projected=15; safety=5 => target=20; netAvail=6+2-1=7 => shortage=13; pack=6, MOQ=6 => 18
	groups := procurement.GroupReplenishmentDecisions([]procurement.ReplenishmentInput{
		{
			SupplierID:            "sup-1",
			DestinationLocationID: "loc-f7",
			CurrencyCode:          "PKR",
			DeliveryWindow:        "2026-11-21-AM",
			IngredientID:          "ing-oat",
			AvgDailyUsage:         decimal.NewFromInt(5),
			LeadTimeDays:          decimal.NewFromInt(3),
			SafetyStock:           decimal.NewFromInt(5),
			BranchOnHand:          decimal.NewFromInt(6),
			ValidatedOpenInbound:  decimal.NewFromInt(2),
			ExistingReservations:  decimal.NewFromInt(1),
			PackSize:              decimal.NewFromInt(6),
			MinOrderQty:           decimal.NewFromInt(6),
		},
	})
	key := "sup-1|loc-f7|PKR|2026-11-21-AM"
	if len(groups[key]) != 1 || !groups[key][0].RecommendedOrder.Equal(decimal.NewFromInt(18)) {
		t.Fatalf("expected 18 L grouped replenishment, got %+v", groups)
	}
}

func TestBuyerGuardrailsAndDuplicateDispatchProtection(t *testing.T) {
	g := procurement.EvaluatePOGuardrails(
		decimal.NewFromInt(75000),
		decimal.RequireFromString("112.50"),
		decimal.RequireFromString("100.00"),
		false,
		procurement.GuardrailConfig{
			MaxAutoSendBudget:   decimal.NewFromInt(50000),
			MaxPriceVariancePct: decimal.NewFromInt(10),
		},
	)
	if !g.NeedsReview || g.TargetStatus != "pending_review" || len(g.ReviewReasons) != 2 {
		t.Fatalf("expected pending_review with 2 reasons, got %+v", g)
	}

	shouldSend, already := procurement.CanDispatchPO("issued")
	if shouldSend || !already {
		t.Fatalf("expected duplicate dispatch protection for issued PO")
	}
}
