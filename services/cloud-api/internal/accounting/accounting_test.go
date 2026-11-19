package accounting_test

import (
	"errors"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/accounting"
	"github.com/shopspring/decimal"
)

func TestAutomatedHospitalityJournalsAreStrictlyBalanced(t *testing.T) {
	// 1. Sale journal with discount + 16% GST + split Cash/Card tender
	// Gross 1000 - Discount 100 = 900 net; Tax 144; Total collected = 1044 (544 Cash + 500 Card)
	saleLines, err := accounting.BuildSaleJournal(accounting.SaleJournalInput{
		GrossSales:    decimal.RequireFromString("1000.00"),
		DiscountTotal: decimal.RequireFromString("100.00"),
		TaxTotal:      decimal.RequireFromString("144.00"),
		CashTender:    decimal.RequireFromString("544.00"),
		CardTender:    decimal.RequireFromString("500.00"),
	})
	if err != nil || len(saleLines) != 5 {
		t.Fatalf("BuildSaleJournal failed: err=%v lines=%d", err, len(saleLines))
	}

	// 2. Refund journal
	if _, err := accounting.BuildRefundJournal(accounting.RefundJournalInput{
		NetRefund:   decimal.RequireFromString("500.00"),
		TaxReversal: decimal.RequireFromString("80.00"),
		CashOut:     decimal.RequireFromString("580.00"),
	}); err != nil {
		t.Fatalf("BuildRefundJournal failed: %v", err)
	}

	// 3. Cash shift variance (both short and over)
	if _, err := accounting.BuildCashShiftVarianceJournal(decimal.RequireFromString("-50.00")); err != nil {
		t.Fatalf("BuildCashShiftVarianceJournal short failed: %v", err)
	}
	if _, err := accounting.BuildCashShiftVarianceJournal(decimal.RequireFromString("25.00")); err != nil {
		t.Fatalf("BuildCashShiftVarianceJournal over failed: %v", err)
	}

	// 4. Card settlement journal
	if _, err := accounting.BuildCardSettlementJournal(accounting.SettlementInput{
		GrossCardAmount: decimal.RequireFromString("10000.00"),
		ProcessingFee:   decimal.RequireFromString("250.00"),
	}); err != nil {
		t.Fatalf("BuildCardSettlementJournal failed: %v", err)
	}

	// 5. Reject unbalanced journal (DR 1000 != CR 999.99)
	err = accounting.ValidateBalancedJournal([]accounting.JournalLineInput{
		{AccountCode: "1010", Debit: decimal.RequireFromString("1000.00"), Credit: decimal.Zero},
		{AccountCode: "4010", Debit: decimal.Zero, Credit: decimal.RequireFromString("999.99")},
	})
	if !errors.Is(err, accounting.ErrUnbalancedJournal) {
		t.Fatalf("expected ErrUnbalancedJournal, got %v", err)
	}
}

func TestDailyReconciliationGuards(t *testing.T) {
	ok := accounting.EvaluateDailyReconciliation(
		decimal.RequireFromString("50000.00"),
		decimal.RequireFromString("50000.00"),
		0,
		true,
	)
	if !ok.IsReconciled {
		t.Fatalf("expected reconciled day, got %+v", ok)
	}

	blockedPending := accounting.EvaluateDailyReconciliation(
		decimal.RequireFromString("50000.00"),
		decimal.RequireFromString("50000.00"),
		3,
		true,
	)
	if blockedPending.IsReconciled || len(blockedPending.BlockReasons) == 0 {
		t.Fatalf("expected daily close blocked when pending_sync_count > 0")
	}
}
