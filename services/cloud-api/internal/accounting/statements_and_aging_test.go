package accounting_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/accounting"
	"github.com/shopspring/decimal"
)

func TestSupplierAgingBucketsAndAPPayment(t *testing.T) {
	asOf := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	report := accounting.ComputeSupplierAging([]accounting.OpenInvoiceInput{
		{InvoiceID: "1", SupplierID: "s1", SupplierName: "Alpine Dairy", DueDate: asOf, OpenBalance: decimal.NewFromInt(1000)},
		{InvoiceID: "2", SupplierID: "s1", SupplierName: "Alpine Dairy", DueDate: asOf.AddDate(0, 0, -15), OpenBalance: decimal.NewFromInt(2000)},
		{InvoiceID: "3", SupplierID: "s1", SupplierName: "Alpine Dairy", DueDate: asOf.AddDate(0, 0, -45), OpenBalance: decimal.NewFromInt(3000)},
		{InvoiceID: "4", SupplierID: "s1", SupplierName: "Alpine Dairy", DueDate: asOf.AddDate(0, 0, -75), OpenBalance: decimal.NewFromInt(4000)},
		{InvoiceID: "5", SupplierID: "s1", SupplierName: "Alpine Dairy", DueDate: asOf.AddDate(0, 0, -110), OpenBalance: decimal.NewFromInt(5000)},
	}, asOf)

	if !report.Totals.Current.Equal(decimal.NewFromInt(1000)) ||
		!report.Totals.Days1To30.Equal(decimal.NewFromInt(2000)) ||
		!report.Totals.Days31To60.Equal(decimal.NewFromInt(3000)) ||
		!report.Totals.Days61To90.Equal(decimal.NewFromInt(4000)) ||
		!report.Totals.Days90Plus.Equal(decimal.NewFromInt(5000)) {
		t.Fatalf("unexpected aging buckets: %+v", report.Totals)
	}

	// Overpayment rejected
	if _, err := accounting.BuildSupplierPaymentJournal(decimal.NewFromInt(1500), decimal.NewFromInt(1000), ""); !errors.Is(err, accounting.ErrOverpaymentNotAllowed) {
		t.Fatalf("expected ErrOverpaymentNotAllowed, got %v", err)
	}
	// Valid payment balanced
	if _, err := accounting.BuildSupplierPaymentJournal(decimal.NewFromInt(1000), decimal.NewFromInt(1000), ""); err != nil {
		t.Fatalf("expected valid AP payment journal, got %v", err)
	}
}

func TestBankReconciliationMatchingAndDiffBlock(t *testing.T) {
	d0 := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	res := accounting.EvaluateBankReconciliation(
		[]accounting.BankLine{
			{ID: "st-1", Date: d0.AddDate(0, 0, 2), Description: "Card Settlement", Amount: decimal.NewFromInt(10000)},
			{ID: "st-2", Date: d0.AddDate(0, 0, 2), Description: "Bank Fee", Amount: decimal.NewFromInt(-250)},
		},
		[]accounting.BankLine{
			{ID: "bk-1", Date: d0, Description: "Card Batch Deposit", Amount: decimal.NewFromInt(10000)},
		},
		decimal.Zero,
		decimal.NewFromInt(9750),
	)
	if len(res.MatchedPairs) != 1 || len(res.UnmatchedStatementLines) != 1 {
		t.Fatalf("expected 1 matched pair and 1 unmatched fee, got %+v", res)
	}
	if res.CanFinalize || !res.UnreconciledDiff.Equal(decimal.NewFromInt(-250)) {
		t.Fatalf("expected CanFinalize=false with -250 diff, got %+v", res)
	}
}

func TestTrialBalancePnLAndBalanceSheetConsistency(t *testing.T) {
	var allLines []accounting.JournalLineInput

	sale, _ := accounting.BuildSaleJournal(accounting.SaleJournalInput{
		GrossSales: decimal.NewFromInt(10000),
		TaxTotal:   decimal.NewFromInt(1600),
		CardTender: decimal.NewFromInt(11600),
	})
	cogs, _ := accounting.BuildCOGSAndWasteJournal(decimal.NewFromInt(2800), decimal.NewFromInt(200))
	settle, _ := accounting.BuildCardSettlementJournal(accounting.SettlementInput{
		GrossCardAmount: decimal.NewFromInt(11600),
		ProcessingFee:   decimal.NewFromInt(300),
	})

	allLines = append(allLines, sale...)
	allLines = append(allLines, cogs...)
	allLines = append(allLines, settle...)

	tb := accounting.ComputeTrialBalance(allLines)
	if !tb.IsBalanced {
		t.Fatalf("Trial Balance not balanced: dr=%s cr=%s", tb.TotalDebits, tb.TotalCredits)
	}

	pnl := accounting.ComputeProfitAndLoss(allLines)
	// Net Revenue = 10000, Direct Costs = 3000 -> Gross Profit = 7000 (70%), OpEx = 300 -> Net Income = 6700
	if !pnl.NetRevenue.Equal(decimal.NewFromInt(10000)) || !pnl.NetIncome.Equal(decimal.NewFromInt(6700)) {
		t.Fatalf("unexpected P&L: %+v", pnl)
	}

	bs := accounting.ComputeBalanceSheet(allLines)
	if !bs.IsEquationBalanced {
		t.Fatalf("Balance Sheet equation failed: Assets=%s Liab=%s Equity=%s",
			bs.TotalAssets, bs.TotalLiabilities, bs.RetainedAndNetEquity)
	}
}
