package procurement

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type ValidationFlag struct {
	Code       string `json:"code"`     // unknown_item | ambiguous_unit | missing_price | unusual_price_change | math_mismatch | duplicate_document | duplicate_invoice_number
	Severity   string `json:"severity"` // error | warning
	LineNumber int    `json:"line_number,omitempty"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

type SupplierAlias struct {
	SupplierSKU  string          `json:"supplier_sku"`
	AliasText    string          `json:"alias_text"`
	IngredientID string          `json:"ingredient_id"`
	ApprovedUnit string          `json:"approved_unit"`
	LastPaid     decimal.Decimal `json:"last_paid_unit_price"`
}

type BillLineValidationInput struct {
	LineNumber        int             `json:"line_number"`
	RawDescription    string          `json:"raw_description"`
	SupplierSKU       string          `json:"supplier_sku"`
	IngredientID      string          `json:"ingredient_id"`
	Unit              string          `json:"unit"`
	ApprovedUnits     []string        `json:"approved_units"`
	Quantity          decimal.Decimal `json:"quantity"`
	UnitPrice         decimal.Decimal `json:"unit_price"`
	LineTax           decimal.Decimal `json:"line_tax"`
	LineAmount        decimal.Decimal `json:"line_amount"`
	IsBonusLine       bool            `json:"is_bonus_line"`
	LastPaidUnitPrice decimal.Decimal `json:"last_paid_unit_price"`
}

type BillValidationInput struct {
	HeaderTax           decimal.Decimal           `json:"header_tax"`
	DeliveryCharges     decimal.Decimal           `json:"delivery_charges"`
	DiscountTotal       decimal.Decimal           `json:"discount_total"`
	GrandTotal          decimal.Decimal           `json:"grand_total"`
	IsDuplicateDoc      bool                      `json:"is_duplicate_document"`
	IsDuplicateInvNo    bool                      `json:"is_duplicate_invoice_number"`
	MaxPriceVariancePct decimal.Decimal           `json:"max_price_variance_pct"`
	Lines               []BillLineValidationInput `json:"lines"`
}

type BillValidationReport struct {
	CanApprove bool             `json:"can_approve"`
	ErrorCount int              `json:"error_count"`
	WarnCount  int              `json:"warn_count"`
	Flags      []ValidationFlag `json:"flags"`
}

type ReviewerEditRecord struct {
	LineNumber int       `json:"line_number,omitempty"`
	Field      string    `json:"field"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	ReviewerID string    `json:"reviewer_id"`
	EditedAt   time.Time `json:"edited_at"`
}

// ResolveSupplierAliases auto-maps incoming lines by exact SupplierSKU or normalized Urdu/English description.
func ResolveSupplierAliases(lines []BillLineValidationInput, aliases []SupplierAlias) []BillLineValidationInput {
	out := make([]BillLineValidationInput, len(lines))
	copy(out, lines)

	for i := range out {
		if out[i].IngredientID != "" {
			continue
		}
		descLower := strings.ToLower(strings.TrimSpace(out[i].RawDescription))
		skuLower := strings.ToLower(strings.TrimSpace(out[i].SupplierSKU))
		for _, a := range aliases {
			if (skuLower != "" && strings.EqualFold(skuLower, strings.TrimSpace(a.SupplierSKU))) ||
				(a.AliasText != "" && strings.Contains(descLower, strings.ToLower(strings.TrimSpace(a.AliasText)))) {
				out[i].IngredientID = a.IngredientID
				if out[i].Unit == "" {
					out[i].Unit = a.ApprovedUnit
				}
				if len(out[i].ApprovedUnits) == 0 && a.ApprovedUnit != "" {
					out[i].ApprovedUnits = []string{a.ApprovedUnit}
				}
				if out[i].LastPaidUnitPrice.IsZero() {
					out[i].LastPaidUnitPrice = a.LastPaid
				}
				break
			}
		}
	}
	return out
}

// ValidateVendorBill evaluates all 6 deterministic anomaly rules using exact shopspring/decimal math.
func ValidateVendorBill(in BillValidationInput) BillValidationReport {
	var flags []ValidationFlag
	tol := decimal.RequireFromString("0.01")
	maxVarPct := in.MaxPriceVariancePct
	if maxVarPct.LessThanOrEqual(decimal.Zero) {
		maxVarPct = decimal.NewFromInt(15)
	}

	if in.IsDuplicateDoc {
		flags = append(flags, ValidationFlag{
			Code: "duplicate_document", Severity: "error", Field: "sha256_fingerprint",
			Message: "Exact duplicate file (SHA-256) already uploaded in this tenant",
		})
	}
	if in.IsDuplicateInvNo {
		flags = append(flags, ValidationFlag{
			Code: "duplicate_invoice_number", Severity: "error", Field: "invoice_number",
			Message: "Normalized supplier invoice number collides with an existing bill",
		})
	}

	lineSum := decimal.Zero
	for _, l := range in.Lines {
		if l.IngredientID == "" {
			flags = append(flags, ValidationFlag{
				Code: "unknown_item", Severity: "error", LineNumber: l.LineNumber, Field: "ingredient_id",
				Message: fmt.Sprintf("Line %d (%s) is not mapped to an ingredient", l.LineNumber, l.RawDescription),
			})
		}

		unitApproved := false
		for _, u := range l.ApprovedUnits {
			if strings.EqualFold(strings.TrimSpace(u), strings.TrimSpace(l.Unit)) {
				unitApproved = true
				break
			}
		}
		if strings.TrimSpace(l.Unit) == "" || (len(l.ApprovedUnits) > 0 && !unitApproved) {
			flags = append(flags, ValidationFlag{
				Code: "ambiguous_unit", Severity: "error", LineNumber: l.LineNumber, Field: "unit",
				Message: fmt.Sprintf("Line %d unit %q is missing or not in approved conversions", l.LineNumber, l.Unit),
			})
		}

		if !l.IsBonusLine && (l.UnitPrice.LessThanOrEqual(decimal.Zero) || l.Quantity.LessThanOrEqual(decimal.Zero)) {
			flags = append(flags, ValidationFlag{
				Code: "missing_price", Severity: "error", LineNumber: l.LineNumber, Field: "unit_price",
				Message: fmt.Sprintf("Line %d must have positive quantity and unit price", l.LineNumber),
			})
		}

		if l.LastPaidUnitPrice.GreaterThan(decimal.Zero) && l.UnitPrice.GreaterThan(decimal.Zero) {
			varPct := l.UnitPrice.Sub(l.LastPaidUnitPrice).Div(l.LastPaidUnitPrice).Mul(decimal.NewFromInt(100)).Abs().Round(2)
			if varPct.GreaterThan(maxVarPct) {
				flags = append(flags, ValidationFlag{
					Code: "unusual_price_change", Severity: "warning", LineNumber: l.LineNumber, Field: "unit_price",
					Message: fmt.Sprintf("Line %d price %s deviates %s%% from last paid %s",
						l.LineNumber, l.UnitPrice.StringFixed(2), varPct.StringFixed(2), l.LastPaidUnitPrice.StringFixed(2)),
				})
			}
		}

		expectedLine := l.Quantity.Mul(l.UnitPrice).Add(l.LineTax).Round(4)
		if expectedLine.Sub(l.LineAmount).Abs().GreaterThan(tol) {
			flags = append(flags, ValidationFlag{
				Code: "math_mismatch", Severity: "error", LineNumber: l.LineNumber, Field: "line_amount",
				Message: fmt.Sprintf("Line %d amount %s != expected %s", l.LineNumber, l.LineAmount.StringFixed(2), expectedLine.StringFixed(2)),
			})
		}
		lineSum = lineSum.Add(l.LineAmount)
	}

	expectedGrand := lineSum.Add(in.HeaderTax).Add(in.DeliveryCharges).Sub(in.DiscountTotal).Round(4)
	if expectedGrand.Sub(in.GrandTotal).Abs().GreaterThan(tol) {
		flags = append(flags, ValidationFlag{
			Code: "math_mismatch", Severity: "error", Field: "grand_total",
			Message: fmt.Sprintf("Header grand total %s != recalculated %s", in.GrandTotal.StringFixed(2), expectedGrand.StringFixed(2)),
		})
	}

	errCount, warnCount := 0, 0
	for _, f := range flags {
		if f.Severity == "error" {
			errCount++
		} else {
			warnCount++
		}
	}
	return BillValidationReport{
		CanApprove: errCount == 0,
		ErrorCount: errCount,
		WarnCount:  warnCount,
		Flags:      flags,
	}
}

func (h *Handler) ValidateVendorBillEndpoint(w http.ResponseWriter, r *http.Request) {
	var in BillValidationInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	report := ValidateVendorBill(in)
	httpx.WriteJSON(w, http.StatusOK, report, nil)
}
