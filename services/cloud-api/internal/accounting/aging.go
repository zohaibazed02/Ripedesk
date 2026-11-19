package accounting

import (
	"errors"
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

var ErrOverpaymentNotAllowed = errors.New("supplier payment exceeds remaining unpaid invoice balance")

type OpenInvoiceInput struct {
	InvoiceID    string          `json:"invoice_id"`
	SupplierID   string          `json:"supplier_id"`
	SupplierName string          `json:"supplier_name"`
	DueDate      time.Time       `json:"due_date"`
	OpenBalance  decimal.Decimal `json:"open_balance"`
}

type AgingBuckets struct {
	Current   decimal.Decimal `json:"current"`
	Days1To30 decimal.Decimal `json:"days_1_30"`
	Days31To60 decimal.Decimal `json:"days_31_60"`
	Days61To90 decimal.Decimal `json:"days_61_90"`
	Days90Plus decimal.Decimal `json:"days_90_plus"`
	Total     decimal.Decimal `json:"total"`
}

type SupplierAgingRow struct {
	SupplierID   string       `json:"supplier_id"`
	SupplierName string       `json:"supplier_name"`
	OverdueCount int          `json:"overdue_count"`
	Buckets      AgingBuckets `json:"buckets"`
}

type SupplierAgingReport struct {
	AsOf      string             `json:"as_of"`
	Suppliers []SupplierAgingRow `json:"suppliers"`
	Totals    AgingBuckets       `json:"totals"`
}

func ComputeSupplierAging(invoices []OpenInvoiceInput, asOf time.Time) SupplierAgingReport {
	bySupplier := map[string]*SupplierAgingRow{}
	order := []string{}
	var totals AgingBuckets

	for _, inv := range invoices {
		if inv.OpenBalance.LessThanOrEqual(decimal.Zero) {
			continue
		}
		row, ok := bySupplier[inv.SupplierID]
		if !ok {
			row = &SupplierAgingRow{SupplierID: inv.SupplierID, SupplierName: inv.SupplierName}
			bySupplier[inv.SupplierID] = row
			order = append(order, inv.SupplierID)
		}

		daysOverdue := int(asOf.UTC().Truncate(24*time.Hour).Sub(inv.DueDate.UTC().Truncate(24*time.Hour)).Hours() / 24)
		amt := inv.OpenBalance.Round(4)

		switch {
		case daysOverdue <= 0:
			row.Buckets.Current = row.Buckets.Current.Add(amt)
			totals.Current = totals.Current.Add(amt)
		case daysOverdue <= 30:
			row.Buckets.Days1To30 = row.Buckets.Days1To30.Add(amt)
			totals.Days1To30 = totals.Days1To30.Add(amt)
			row.OverdueCount++
		case daysOverdue <= 60:
			row.Buckets.Days31To60 = row.Buckets.Days31To60.Add(amt)
			totals.Days31To60 = totals.Days31To60.Add(amt)
			row.OverdueCount++
		case daysOverdue <= 90:
			row.Buckets.Days61To90 = row.Buckets.Days61To90.Add(amt)
			totals.Days61To90 = totals.Days61To90.Add(amt)
			row.OverdueCount++
		default:
			row.Buckets.Days90Plus = row.Buckets.Days90Plus.Add(amt)
			totals.Days90Plus = totals.Days90Plus.Add(amt)
			row.OverdueCount++
		}
		row.Buckets.Total = row.Buckets.Total.Add(amt)
		totals.Total = totals.Total.Add(amt)
	}

	rows := make([]SupplierAgingRow, 0, len(order))
	for _, id := range order {
		rows = append(rows, *bySupplier[id])
	}
	return SupplierAgingReport{
		AsOf:      asOf.UTC().Format("2006-01-02"),
		Suppliers: rows,
		Totals:    totals,
	}
}

func BuildSupplierPaymentJournal(paymentAmount, remainingBalance decimal.Decimal, creditAccountCode string) ([]JournalLineInput, error) {
	if paymentAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrNegativeAmount
	}
	if paymentAmount.GreaterThan(remainingBalance) {
		return nil, ErrOverpaymentNotAllowed
	}
	if creditAccountCode == "" {
		creditAccountCode = AcctOperatingBank
	}
	amt := paymentAmount.Round(4)
	lines := []JournalLineInput{
		{AccountCode: AcctAccountsPayable, Description: "Supplier AP Payment", Debit: amt, Credit: decimal.Zero},
		{AccountCode: creditAccountCode, Description: "Bank / Cash Disbursement", Debit: decimal.Zero, Credit: amt},
	}
	return lines, ValidateBalancedJournal(lines)
}

func (h *Handler) GetSupplierAging(w http.ResponseWriter, r *http.Request) {
	asOf := time.Now().UTC()
	if raw := r.URL.Query().Get("as_of"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			asOf = parsed
		}
	}
	report := ComputeSupplierAging(nil, asOf)
	httpx.WriteJSON(w, http.StatusOK, report, nil)
}

func (h *Handler) PostSupplierPayment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PaymentAmount    decimal.Decimal `json:"payment_amount"`
		RemainingBalance decimal.Decimal `json:"remaining_balance"`
		CreditAccount    string          `json:"credit_account"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	lines, err := BuildSupplierPaymentJournal(in.PaymentAmount, in.RemainingBalance, in.CreditAccount)
	if err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "PAYMENT_INVALID", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"status":  "posted",
		"journal": lines,
	}, nil)
}
