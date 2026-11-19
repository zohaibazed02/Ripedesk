package accounting

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

var (
	ErrEmptyJournal        = errors.New("journal entry must contain at least two lines")
	ErrNegativeAmount      = errors.New("journal line debit/credit amounts cannot be negative")
	ErrBothSidesPopulated  = errors.New("journal line cannot have both debit and credit > 0")
	ErrUnbalancedJournal   = errors.New("journal entry is unbalanced: SUM(debit) != SUM(credit)")
)

type JournalLineInput struct {
	AccountCode string          `json:"account_code"`
	Description string          `json:"description,omitempty"`
	Debit       decimal.Decimal `json:"debit"`
	Credit      decimal.Decimal `json:"credit"`
}

func ValidateBalancedJournal(lines []JournalLineInput) error {
	if len(lines) < 2 {
		return ErrEmptyJournal
	}
	sumDr := decimal.Zero
	sumCr := decimal.Zero
	for _, l := range lines {
		if l.Debit.IsNegative() || l.Credit.IsNegative() {
			return ErrNegativeAmount
		}
		if l.Debit.GreaterThan(decimal.Zero) && l.Credit.GreaterThan(decimal.Zero) {
			return ErrBothSidesPopulated
		}
		sumDr = sumDr.Add(l.Debit.Round(4))
		sumCr = sumCr.Add(l.Credit.Round(4))
	}
	if !sumDr.Equal(sumCr) || sumDr.IsZero() {
		return fmt.Errorf("%w: debit=%s credit=%s", ErrUnbalancedJournal, sumDr.StringFixed(4), sumCr.StringFixed(4))
	}
	return nil
}

type SaleJournalInput struct {
	GrossSales    decimal.Decimal
	DeliveryFee   decimal.Decimal
	DiscountTotal decimal.Decimal
	TaxTotal      decimal.Decimal
	CashTender    decimal.Decimal
	CardTender    decimal.Decimal
	CODTender     decimal.Decimal
}

func BuildSaleJournal(in SaleJournalInput) ([]JournalLineInput, error) {
	var lines []JournalLineInput
	addDr := func(acct, desc string, amt decimal.Decimal) {
		if amt.GreaterThan(decimal.Zero) {
			lines = append(lines, JournalLineInput{AccountCode: acct, Description: desc, Debit: amt.Round(4), Credit: decimal.Zero})
		}
	}
	addCr := func(acct, desc string, amt decimal.Decimal) {
		if amt.GreaterThan(decimal.Zero) {
			lines = append(lines, JournalLineInput{AccountCode: acct, Description: desc, Debit: decimal.Zero, Credit: amt.Round(4)})
		}
	}

	addDr(AcctCashOnHand, "POS Cash Tender", in.CashTender)
	addDr(AcctCardClearing, "POS Card Tender", in.CardTender)
	addDr(AcctRiderCOD, "Delivery COD Receivable", in.CODTender)
	addDr(AcctSalesDiscounts, "Order Discounts", in.DiscountTotal)

	addCr(AcctFnBSales, "Food & Beverage Gross Sales", in.GrossSales)
	addCr(AcctDeliveryRevenue, "Delivery Fee Revenue", in.DeliveryFee)
	addCr(AcctSalesTaxPayable, "Sales Tax / GST Payable", in.TaxTotal)

	return lines, ValidateBalancedJournal(lines)
}

type RefundJournalInput struct {
	NetRefund   decimal.Decimal
	TaxReversal decimal.Decimal
	CashOut     decimal.Decimal
	CardOut     decimal.Decimal
}

func BuildRefundJournal(in RefundJournalInput) ([]JournalLineInput, error) {
	var lines []JournalLineInput
	if in.NetRefund.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctSalesRefunds, Description: "Sales Refund", Debit: in.NetRefund.Round(4), Credit: decimal.Zero})
	}
	if in.TaxReversal.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctSalesTaxPayable, Description: "Sales Tax Reversal", Debit: in.TaxReversal.Round(4), Credit: decimal.Zero})
	}
	if in.CashOut.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctCashOnHand, Description: "Cash Refund Out", Debit: decimal.Zero, Credit: in.CashOut.Round(4)})
	}
	if in.CardOut.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctCardClearing, Description: "Card Refund Out", Debit: decimal.Zero, Credit: in.CardOut.Round(4)})
	}
	return lines, ValidateBalancedJournal(lines)
}

// BuildCashShiftVarianceJournal handles both cash short (variance < 0) and cash over (variance > 0).
func BuildCashShiftVarianceJournal(variance decimal.Decimal) ([]JournalLineInput, error) {
	if variance.IsZero() {
		return nil, nil
	}
	absVar := variance.Abs().Round(4)
	var lines []JournalLineInput
	if variance.IsNegative() {
		// Cash Short: DR 5030 Cash Over/Short, CR 1010 Cash on Hand
		lines = []JournalLineInput{
			{AccountCode: AcctCashOverShort, Description: "Cash Shift Shortage", Debit: absVar, Credit: decimal.Zero},
			{AccountCode: AcctCashOnHand, Description: "Register Cash Short", Debit: decimal.Zero, Credit: absVar},
		}
	} else {
		// Cash Over: DR 1010 Cash on Hand, CR 5030 Cash Over/Short
		lines = []JournalLineInput{
			{AccountCode: AcctCashOnHand, Description: "Register Cash Overage", Debit: absVar, Credit: decimal.Zero},
			{AccountCode: AcctCashOverShort, Description: "Cash Shift Overage", Debit: decimal.Zero, Credit: absVar},
		}
	}
	return lines, ValidateBalancedJournal(lines)
}

type SettlementInput struct {
	GrossCardAmount decimal.Decimal
	ProcessingFee   decimal.Decimal
}

func BuildCardSettlementJournal(in SettlementInput) ([]JournalLineInput, error) {
	netBank := in.GrossCardAmount.Sub(in.ProcessingFee).Round(4)
	lines := []JournalLineInput{
		{AccountCode: AcctOperatingBank, Description: "Net Card Settlement Deposit", Debit: netBank, Credit: decimal.Zero},
		{AccountCode: AcctCardFees, Description: "Card Processing Fees", Debit: in.ProcessingFee.Round(4), Credit: decimal.Zero},
		{AccountCode: AcctCardClearing, Description: "Clear Undeposited Card Batch", Debit: decimal.Zero, Credit: in.GrossCardAmount.Round(4)},
	}
	return lines, ValidateBalancedJournal(lines)
}

func BuildCOGSAndWasteJournal(cogsAmount, wasteAmount decimal.Decimal) ([]JournalLineInput, error) {
	var lines []JournalLineInput
	totalInv := decimal.Zero
	if cogsAmount.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctCOGS, Description: "Theoretical Prep COGS", Debit: cogsAmount.Round(4), Credit: decimal.Zero})
		totalInv = totalInv.Add(cogsAmount.Round(4))
	}
	if wasteAmount.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctKitchenWaste, Description: "Kitchen Waste & Spoilage", Debit: wasteAmount.Round(4), Credit: decimal.Zero})
		totalInv = totalInv.Add(wasteAmount.Round(4))
	}
	if totalInv.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalLineInput{AccountCode: AcctInventory, Description: "Ingredient Inventory Credit", Debit: decimal.Zero, Credit: totalInv})
	}
	return lines, ValidateBalancedJournal(lines)
}
