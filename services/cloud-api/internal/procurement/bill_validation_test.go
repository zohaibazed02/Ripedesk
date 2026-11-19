package procurement_test

import (
	"testing"

	"github.com/hospitality-os/cloud-api/internal/procurement"
	"github.com/shopspring/decimal"
)

func TestVendorBillValidationAndUrduAliasMapping(t *testing.T) {
	// 1. Auto-map Urdu/English alias & SKU
	aliases := []procurement.SupplierAlias{
		{
			SupplierSKU:  "ALP-OAT-12L",
			AliasText:    "اوٹ ملک",
			IngredientID: "ing-oat",
			ApprovedUnit: "L",
			LastPaid:     decimal.RequireFromString("850.00"),
		},
	}
	mapped := procurement.ResolveSupplierAliases([]procurement.BillLineValidationInput{
		{
			LineNumber:     1,
			RawDescription: "Barista Oat Milk / اوٹ ملک",
			Unit:           "L",
			Quantity:       decimal.NewFromInt(12),
			UnitPrice:      decimal.RequireFromString("1050.00"), // +23.53% spike -> unusual_price_change warning
			LineAmount:     decimal.RequireFromString("12600.00"),
		},
	}, aliases)

	if mapped[0].IngredientID != "ing-oat" {
		t.Fatalf("expected Urdu alias auto-mapping to ing-oat, got %q", mapped[0].IngredientID)
	}

	report := procurement.ValidateVendorBill(procurement.BillValidationInput{
		GrandTotal: decimal.RequireFromString("12600.00"),
		Lines:      mapped,
	})
	if !report.CanApprove || report.ErrorCount != 0 || report.WarnCount != 1 {
		t.Fatalf("expected CanApprove=true with 1 price warning, got %+v", report)
	}
	if report.Flags[0].Code != "unusual_price_change" {
		t.Fatalf("expected unusual_price_change flag, got %s", report.Flags[0].Code)
	}

	// 2. Blocking errors: unknown_item, ambiguous_unit, missing_price, math_mismatch, duplicate
	badReport := procurement.ValidateVendorBill(procurement.BillValidationInput{
		GrandTotal:       decimal.RequireFromString("500.00"),
		IsDuplicateInvNo: true,
		Lines: []procurement.BillLineValidationInput{
			{
				LineNumber:     1,
				RawDescription: "Mystery Syrup",
				IngredientID:   "",
				Unit:           "jug",
				ApprovedUnits:  []string{"ml", "L"},
				Quantity:       decimal.NewFromInt(2),
				UnitPrice:      decimal.Zero,
				LineAmount:     decimal.RequireFromString("100.00"),
			},
		},
	})
	if badReport.CanApprove || badReport.ErrorCount < 5 {
		t.Fatalf("expected CanApprove=false with >=5 errors, got %+v", badReport)
	}
}
