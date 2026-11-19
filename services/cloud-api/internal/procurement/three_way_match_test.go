package procurement_test

import (
	"errors"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/procurement"
	"github.com/shopspring/decimal"
)

func TestCleanThreeWayMatchPartialShipmentAndBalancedJournal(t *testing.T) {
	// PO = 20 L, GRN #1 = 12 L, Invoice #1 = 12 L @ 850.00 + 300.00 delivery + 0.50 tax rounding
	res, err := procurement.EvaluateThreeWayMatch(procurement.ThreeWayMatchInput{
		DeliveryCharges:  decimal.RequireFromString("300.00"),
		InputTax:         decimal.Zero,
		TaxRoundingDelta: decimal.RequireFromString("0.50"),
		Lines: []procurement.ThreeWayLineInput{
			{
				LineNumber:        1,
				IngredientID:      "ing-oat",
				POOrderedQty:      decimal.NewFromInt(20),
				POUnitPrice:       decimal.RequireFromString("850.00"),
				GRNReceivedQty:    decimal.NewFromInt(12),
				InvoicedQty:       decimal.NewFromInt(12),
				InvoicedUnitPrice: decimal.RequireFromString("850.00"),
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateThreeWayMatch failed: %v", err)
	}
	if res.Status != "matched_ready_to_post" {
		t.Fatalf("expected matched_ready_to_post, got %s", res.Status)
	}
	// 12 * 850 + 300 + 0.50 = 10,500.50
	if !res.ApprovedPayableAmount.Equal(decimal.RequireFromString("10500.5000")) {
		t.Fatalf("expected approved payable 10500.5000, got %s", res.ApprovedPayableAmount)
	}
	// Verify strict double-entry balance: SUM(debit) == SUM(credit)
	sumDr, sumCr := decimal.Zero, decimal.Zero
	for _, jl := range res.JournalPreview {
		sumDr = sumDr.Add(jl.Debit)
		sumCr = sumCr.Add(jl.Credit)
	}
	if !sumDr.Equal(sumCr) || !sumDr.Equal(res.ApprovedPayableAmount) {
		t.Fatalf("unbalanced journal: dr=%s cr=%s", sumDr, sumCr)
	}
}

func TestOverBilledQuantityHoldAndDirectPurchaseGuard(t *testing.T) {
	// 1. GRN receives 10 L, Invoice bills 12 L @ 850 -> 8,500 approved, 1,700 held in review
	res, err := procurement.EvaluateThreeWayMatch(procurement.ThreeWayMatchInput{
		Lines: []procurement.ThreeWayLineInput{
			{
				LineNumber:        1,
				IngredientID:      "ing-oat",
				POOrderedQty:      decimal.NewFromInt(12),
				POUnitPrice:       decimal.RequireFromString("850.00"),
				GRNReceivedQty:    decimal.NewFromInt(10),
				InvoicedQty:       decimal.NewFromInt(12),
				InvoicedUnitPrice: decimal.RequireFromString("850.00"),
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateThreeWayMatch failed: %v", err)
	}
	if res.Status != "disputed" ||
		!res.ApprovedPayableAmount.Equal(decimal.RequireFromString("8500.0000")) ||
		!res.HeldInReviewAmount.Equal(decimal.RequireFromString("1700.0000")) {
		t.Fatalf("expected disputed 8500 approved / 1700 held, got %+v", res)
	}

	// 2. Direct Purchase (No-PO) fails without manager auth or GRN
	_, err = procurement.EvaluateThreeWayMatch(procurement.ThreeWayMatchInput{
		IsDirectPurchase: true,
	})
	if !errors.Is(err, procurement.ErrDirectPurchaseUnauthorized) {
		t.Fatalf("expected ErrDirectPurchaseUnauthorized, got %v", err)
	}

	// 3. Deterministic GRN idempotency key prevents duplicate stock receipts on replay
	k1 := procurement.DeterministicGRNIdempotencyKey("grn-100", "line-1")
	k2 := procurement.DeterministicGRNIdempotencyKey("grn-100", "line-1")
	if k1 != k2 || k1 != "receipt:grn-100:line-1" {
		t.Fatalf("unexpected GRN idempotency key: %q", k1)
	}
}
