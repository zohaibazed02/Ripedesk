package procurement

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var (
	ErrDirectPurchaseUnauthorized = errors.New("direct purchase requires manager authorization and confirmed GRN")
	ErrUnbalancedJournal          = errors.New("generated payable journal is unbalanced")
)

type ThreeWayLineInput struct {
	LineNumber        int             `json:"line_number"`
	IngredientID      string          `json:"ingredient_id"`
	POOrderedQty      decimal.Decimal `json:"po_ordered_qty"`
	POUnitPrice       decimal.Decimal `json:"po_unit_price"`
	GRNReceivedQty    decimal.Decimal `json:"grn_received_qty"`
	InvoicedQty       decimal.Decimal `json:"invoiced_qty"`
	InvoicedUnitPrice decimal.Decimal `json:"invoiced_unit_price"`
	DisputedQty       decimal.Decimal `json:"disputed_qty"`
}

type ThreeWayMatchInput struct {
	IsDirectPurchase         bool                `json:"is_direct_purchase"`
	DirectPurchaseAuthBy     string              `json:"direct_purchase_authorized_by"`
	ConfirmedGRNID           string              `json:"confirmed_grn_id"`
	PriceTolerancePct        decimal.Decimal     `json:"price_tolerance_pct"`
	TaxRoundingTolerance     decimal.Decimal     `json:"tax_rounding_tolerance"`
	DeliveryCharges          decimal.Decimal     `json:"delivery_charges"`
	InputTax                 decimal.Decimal     `json:"input_tax"`
	TaxRoundingDelta         decimal.Decimal     `json:"tax_rounding_delta"`
	SupplierCreditAdjustment decimal.Decimal     `json:"supplier_credit_adjustment"`
	Lines                    []ThreeWayLineInput `json:"lines"`
}

type ThreeWayLineResult struct {
	LineNumber     int             `json:"line_number"`
	Status         string          `json:"status"` // matched | variance_review | disputed
	ApprovedAmount decimal.Decimal `json:"approved_amount"`
	HeldAmount     decimal.Decimal `json:"held_amount"`
	Flags          []string        `json:"flags,omitempty"`
}

type JournalEntryLine struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Debit       decimal.Decimal `json:"debit"`
	Credit      decimal.Decimal `json:"credit"`
}

type ThreeWayMatchResult struct {
	Status                string               `json:"status"` // matched_ready_to_post | variance_review | disputed
	ApprovedPayableAmount decimal.Decimal      `json:"approved_payable_amount"`
	HeldInReviewAmount    decimal.Decimal      `json:"held_in_review_amount"`
	LineResults           []ThreeWayLineResult `json:"line_results"`
	JournalPreview        []JournalEntryLine   `json:"journal_preview"`
}

// BuildBalancedPayableJournal creates a strictly balanced double-entry journal for an approved vendor bill.
func BuildBalancedPayableJournal(
	approvedSubtotal decimal.Decimal,
	deliveryCharges decimal.Decimal,
	inputTax decimal.Decimal,
	taxRoundingDelta decimal.Decimal,
	creditAdjustment decimal.Decimal,
) ([]JournalEntryLine, error) {
	lines := []JournalEntryLine{}
	netGoods := approvedSubtotal.Sub(creditAdjustment).Round(4)
	if netGoods.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalEntryLine{
			AccountCode: "1200", AccountName: "Inventory / GRNI Clearing", Debit: netGoods, Credit: decimal.Zero,
		})
	}
	if deliveryCharges.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalEntryLine{
			AccountCode: "5150", AccountName: "Freight & Delivery Inward", Debit: deliveryCharges.Round(4), Credit: decimal.Zero,
		})
	}
	if inputTax.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalEntryLine{
			AccountCode: "1350", AccountName: "Input Tax Receivable", Debit: inputTax.Round(4), Credit: decimal.Zero,
		})
	}
	if taxRoundingDelta.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalEntryLine{
			AccountCode: "5990", AccountName: "Tax Rounding Variance", Debit: taxRoundingDelta.Round(4), Credit: decimal.Zero,
		})
	}

	totalDebit := decimal.Zero
	for _, l := range lines {
		totalDebit = totalDebit.Add(l.Debit)
	}
	if totalDebit.GreaterThan(decimal.Zero) {
		lines = append(lines, JournalEntryLine{
			AccountCode: "2100", AccountName: "Accounts Payable — Supplier", Debit: decimal.Zero, Credit: totalDebit.Round(4),
		})
	}

	sumDr, sumCr := decimal.Zero, decimal.Zero
	for _, l := range lines {
		sumDr = sumDr.Add(l.Debit)
		sumCr = sumCr.Add(l.Credit)
	}
	if !sumDr.Equal(sumCr) {
		return nil, fmt.Errorf("%w: debit=%s credit=%s", ErrUnbalancedJournal, sumDr, sumCr)
	}
	return lines, nil
}

// EvaluateThreeWayMatch performs deterministic line-by-line PO vs GRN vs Vendor Invoice matching.
func EvaluateThreeWayMatch(in ThreeWayMatchInput) (*ThreeWayMatchResult, error) {
	if in.IsDirectPurchase {
		if in.DirectPurchaseAuthBy == "" || in.ConfirmedGRNID == "" {
			return nil, ErrDirectPurchaseUnauthorized
		}
	}

	priceTol := in.PriceTolerancePct
	if priceTol.LessThanOrEqual(decimal.Zero) {
		priceTol = decimal.NewFromInt(2)
	}

	approvedSubtotal := decimal.Zero
	heldTotal := decimal.Zero
	overallStatus := "matched_ready_to_post"
	lineResults := make([]ThreeWayLineResult, 0, len(in.Lines))

	for _, l := range in.Lines {
		var flags []string
		status := "matched"

		// Quantity match against physically confirmed GRN
		billableQty := l.InvoicedQty.Sub(l.DisputedQty)
		if billableQty.IsNegative() {
			billableQty = decimal.Zero
		}
		approvedQty := billableQty
		heldQty := l.DisputedQty

		if billableQty.GreaterThan(l.GRNReceivedQty) {
			flags = append(flags, "over_billed_qty")
			excess := billableQty.Sub(l.GRNReceivedQty)
			approvedQty = l.GRNReceivedQty
			heldQty = heldQty.Add(excess)
			status = "disputed"
			overallStatus = "disputed"
		} else if !in.IsDirectPurchase && l.POOrderedQty.GreaterThan(l.GRNReceivedQty) {
			flags = append(flags, "partial_shipment_open")
		}

		// Unit price match against PO
		if !in.IsDirectPurchase && l.POUnitPrice.GreaterThan(decimal.Zero) {
			varPct := l.InvoicedUnitPrice.Sub(l.POUnitPrice).Div(l.POUnitPrice).Mul(decimal.NewFromInt(100)).Abs().Round(2)
			if varPct.GreaterThan(priceTol) {
				flags = append(flags, "price_variance")
				if status == "matched" {
					status = "variance_review"
				}
				if overallStatus == "matched_ready_to_post" {
					overallStatus = "variance_review"
				}
			}
		}

		approvedAmt := approvedQty.Mul(l.InvoicedUnitPrice).Round(4)
		heldAmt := heldQty.Mul(l.InvoicedUnitPrice).Round(4)
		approvedSubtotal = approvedSubtotal.Add(approvedAmt)
		heldTotal = heldTotal.Add(heldAmt)

		lineResults = append(lineResults, ThreeWayLineResult{
			LineNumber:     l.LineNumber,
			Status:         status,
			ApprovedAmount: approvedAmt,
			HeldAmount:     heldAmt,
			Flags:          flags,
		})
	}

	journal, err := BuildBalancedPayableJournal(
		approvedSubtotal,
		in.DeliveryCharges,
		in.InputTax,
		in.TaxRoundingDelta,
		in.SupplierCreditAdjustment,
	)
	if err != nil {
		return nil, err
	}

	approvedPayable := decimal.Zero
	for _, jl := range journal {
		approvedPayable = approvedPayable.Add(jl.Credit)
	}

	return &ThreeWayMatchResult{
		Status:                overallStatus,
		ApprovedPayableAmount: approvedPayable.Round(4),
		HeldInReviewAmount:    heldTotal.Round(4),
		LineResults:           lineResults,
		JournalPreview:        journal,
	}, nil
}

func (h *Handler) MatchVendorBill(w http.ResponseWriter, r *http.Request) {
	var in ThreeWayMatchInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	res, err := EvaluateThreeWayMatch(in)
	if err != nil {
		if errors.Is(err, ErrDirectPurchaseUnauthorized) {
			httpx.WriteError(w, http.StatusForbidden, "DIRECT_PURCHASE_GUARD", err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "MATCH_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res, nil)
}

func (h *Handler) PostVendorBillPayable(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	billID := r.PathValue("id")
	var alreadyPosted bool
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(r.Context(), `SELECT status FROM vendor_invoices WHERE id = $1 FOR UPDATE`, billID).Scan(&status); err != nil {
			return err
		}
		if status == "posted" || status == "paid" {
			alreadyPosted = true
			return nil
		}
		_, err := tx.Exec(r.Context(), `UPDATE vendor_invoices SET status = 'posted' WHERE id = $1`, billID)
		return err
	})
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "POST_PAYABLE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":             billID,
		"status":         "posted",
		"already_posted": alreadyPosted,
	}, nil)
}
