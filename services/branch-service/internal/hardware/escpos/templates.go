package escpos

import (
	"fmt"
	"strings"
)

type ReceiptLine struct {
	Name      string
	Quantity  string
	LineTotal string
	Modifiers []string
}

type ReceiptData struct {
	PaperWidth    PaperWidth
	BranchName    string
	BranchAddress string
	OrderMode     string
	OrderNumber   string
	CheckNumber   int
	TableCode     string
	Timestamp     string
	Lines         []ReceiptLine
	Subtotal      string
	TaxTotal      string
	DiscountTotal string
	GrandTotal    string
	PaymentMethod string
	Tendered      string
	ChangeDue     string
	TrackingQRURL string
	FooterNote    string
	OpenDrawer    bool
}

func RenderCustomerReceipt(r ReceiptData) []byte {
	enc := NewEncoder(r.PaperWidth).Init().CodeTable(0)

	enc.Align(AlignCenter).Bold(true).TextSize(2, 2).Line(r.BranchName)
	enc.TextSize(1, 1).Bold(false)
	if r.BranchAddress != "" {
		enc.Line(r.BranchAddress)
	}
	enc.Feed(1)

	// Inverted Order Mode Banner
	modeBanner := " " + strings.ToUpper(strings.ReplaceAll(r.OrderMode, "_", " ")) + " "
	enc.Invert(true).Bold(true).Line(modeBanner).Invert(false).Bold(false)
	enc.Align(AlignLeft).Rule('-')

	enc.KeyValue("Order: "+r.OrderNumber, r.Timestamp)
	if r.TableCode != "" {
		enc.KeyValue("Table: "+r.TableCode, fmt.Sprintf("Check #%d", max1(r.CheckNumber)))
	}
	enc.Rule('-')

	for _, line := range r.Lines {
		enc.Bold(true).KeyValue(fmt.Sprintf("%sx %s", line.Quantity, line.Name), line.LineTotal).Bold(false)
		for _, mod := range line.Modifiers {
			enc.Line("   + " + mod)
		}
	}

	enc.Rule('-')
	enc.KeyValue("Subtotal", r.Subtotal)
	if r.DiscountTotal != "" && r.DiscountTotal != "0.0000" && r.DiscountTotal != "0.00" {
		enc.KeyValue("Discount", "-"+r.DiscountTotal)
	}
	enc.KeyValue("Tax", r.TaxTotal)
	enc.Bold(true).TextSize(1, 2).KeyValue("TOTAL", r.GrandTotal).TextSize(1, 1).Bold(false)
	enc.Rule('-')

	if r.PaymentMethod != "" {
		enc.KeyValue("Paid ("+strings.ToUpper(r.PaymentMethod)+")", r.Tendered)
		if r.ChangeDue != "" && r.ChangeDue != "0.0000" && r.ChangeDue != "0.00" {
			enc.KeyValue("Change Due", r.ChangeDue)
		}
	}

	if r.TrackingQRURL != "" {
		enc.Feed(1).Align(AlignCenter).QRCode(r.TrackingQRURL, 5).Feed(1)
		enc.Line("Scan for Digital Receipt / Live Tracking")
	}

	if r.FooterNote != "" {
		enc.Align(AlignCenter).Line(r.FooterNote)
	}
	enc.Feed(3).Cut(true)

	if r.OpenDrawer {
		enc.CashDrawerKick(DrawerPin2)
	}
	return enc.Bytes()
}

type KitchenTicketItem struct {
	Quantity  string
	Name      string
	Modifiers []string
	Notes     string
}

type KitchenTicketData struct {
	PaperWidth  PaperWidth
	StationName string
	OrderMode   string
	OrderNumber string
	TableCode   string
	RoundNumber int
	Timestamp   string
	Items       []KitchenTicketItem
}

func RenderKitchenTicket(t KitchenTicketData) []byte {
	enc := NewEncoder(t.PaperWidth).Init().CodeTable(0)

	banner := " *** " + strings.ToUpper(strings.ReplaceAll(t.StationName, "_", " ")) + " *** "
	enc.Align(AlignCenter).Invert(true).Bold(true).TextSize(1, 2).Line(banner).TextSize(1, 1).Invert(false)

	enc.Align(AlignLeft).Rule('=')
	enc.Bold(true).TextSize(1, 2).KeyValue(t.OrderNumber, strings.ToUpper(strings.ReplaceAll(t.OrderMode, "_", " "))).TextSize(1, 1)
	if t.TableCode != "" {
		enc.KeyValue("TABLE: "+t.TableCode, fmt.Sprintf("ROUND #%d", max1(t.RoundNumber)))
	} else {
		enc.KeyValue(fmt.Sprintf("ROUND #%d", max1(t.RoundNumber)), t.Timestamp)
	}
	enc.Bold(false).Rule('=')

	for _, item := range t.Items {
		enc.Bold(true).TextSize(1, 2).Line(fmt.Sprintf("%sx  %s", item.Quantity, item.Name)).TextSize(1, 1)
		for _, mod := range item.Modifiers {
			enc.Line("    >> " + mod)
		}
		enc.Bold(false)
		if item.Notes != "" {
			enc.Line("    NOTE: " + item.Notes)
		}
	}

	enc.Rule('-').Line("Printed: " + t.Timestamp).Feed(3).Cut(true)
	return enc.Bytes()
}

// RenderCashDrawerPulse returns a standalone RJ11 cash drawer kick frame (Pin 2 + Pin 5).
func RenderCashDrawerPulse() []byte {
	return NewEncoder(Paper80mm).
		Init().
		CashDrawerKick(DrawerPin2).
		CashDrawerKick(DrawerPin5).
		Bytes()
}

type ShiftZReportData struct {
	PaperWidth    PaperWidth
	BranchName    string
	ShiftID       string
	ClosedBy      string
	ClosedAt      string
	OpeningFloat  string
	CashSales     string
	CashRefunds   string
	PaidIn        string
	PaidOut       string
	SafeDrops     string
	ExpectedCash  string
	CountedCash   string
	Variance      string
}

func RenderShiftZReport(z ShiftZReportData) []byte {
	enc := NewEncoder(z.PaperWidth).Init().CodeTable(0)
	enc.Align(AlignCenter).Invert(true).Bold(true).TextSize(1, 2).Line(" SHIFT Z-REPORT ").TextSize(1, 1).Invert(false)
	enc.Line(z.BranchName).Bold(false).Align(AlignLeft).Rule('=')
	enc.KeyValue("Shift ID:", z.ShiftID)
	enc.KeyValue("Closed By:", z.ClosedBy)
	enc.KeyValue("Closed At:", z.ClosedAt)
	enc.Rule('-')
	enc.KeyValue("Opening Float", z.OpeningFloat)
	enc.KeyValue("Cash Sales (+)", z.CashSales)
	enc.KeyValue("Cash Refunds (-)", z.CashRefunds)
	enc.KeyValue("Paid In (+)", z.PaidIn)
	enc.KeyValue("Paid Out (-)", z.PaidOut)
	enc.KeyValue("Safe Drops (-)", z.SafeDrops)
	enc.Rule('-')
	enc.Bold(true).KeyValue("Expected Cash", z.ExpectedCash)
	enc.KeyValue("Blind Counted", z.CountedCash)
	enc.TextSize(1, 2).KeyValue("VARIANCE", z.Variance).TextSize(1, 1).Bold(false)
	enc.Rule('=').Feed(3).Cut(true)
	return enc.Bytes()
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
