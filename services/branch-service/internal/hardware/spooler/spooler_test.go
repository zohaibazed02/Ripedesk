package spooler_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hospitality-os/branch-service/internal/hardware/escpos"
	"github.com/hospitality-os/branch-service/internal/hardware/spooler"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

func TestEscPosTemplatesAndByteSequences(t *testing.T) {
	receipt := escpos.RenderCustomerReceipt(escpos.ReceiptData{
		PaperWidth:    escpos.Paper80mm,
		BranchName:    "Artisan Roast & Bistro",
		BranchAddress: "F-7 Coffee Bar, Islamabad",
		OrderMode:     "quick_sale",
		OrderNumber:   "F7-1042",
		CheckNumber:   1,
		Timestamp:     "2026-11-19 22:40",
		Lines: []escpos.ReceiptLine{
			{
				Name:      "Large Oat Latte",
				Quantity:  "1",
				LineTotal: "1,170.00",
				Modifiers: []string{"Extra Shot"},
			},
		},
		Subtotal:      "1,170.00",
		TaxTotal:      "187.20",
		GrandTotal:    "1,357.20",
		PaymentMethod: "cash",
		Tendered:      "1,500.00",
		ChangeDue:     "142.80",
		TrackingQRURL: "https://track.artisanroast.pk/ord/F7-1042",
		FooterNote:    "Thank you for visiting!",
		OpenDrawer:    true,
	})

	// Verify ESC @ init bytes
	if !bytes.HasPrefix(receipt, []byte{0x1B, 0x40}) {
		t.Fatalf("receipt missing ESC @ init sequence")
	}
	// Verify QR code GS ( k sequence
	if !bytes.Contains(receipt, []byte{0x1D, 0x28, 0x6B}) {
		t.Fatalf("receipt missing GS ( k QR sequence")
	}
	// Verify partial cut GS V sequence
	if !bytes.Contains(receipt, []byte{0x1D, 0x56, 0x42, 0x00}) {
		t.Fatalf("receipt missing GS V cut sequence")
	}
	// Verify RJ11 cash drawer pulse ESC p 0 25 250
	if !bytes.Contains(receipt, []byte{0x1B, 0x70, 0x00, 0x19, 0xFA}) {
		t.Fatalf("receipt missing ESC p cash drawer kick sequence")
	}

	ticket := escpos.RenderKitchenTicket(escpos.KitchenTicketData{
		PaperWidth:  escpos.Paper80mm,
		StationName: "barista",
		OrderMode:   "dine_in",
		OrderNumber: "G9-208",
		TableCode:   "T-02",
		RoundNumber: 2,
		Timestamp:   "22:41:05",
		Items: []escpos.KitchenTicketItem{
			{
				Quantity:  "2",
				Name:      "Large Oat Latte",
				Modifiers: []string{"Extra Shot", "No Foam"},
				Notes:     "Serve extra hot",
			},
		},
	})
	if !strings.Contains(string(ticket), "BARISTA") || !strings.Contains(string(ticket), "Serve extra hot") {
		t.Fatalf("kitchen ticket missing expected station banner or notes")
	}
}

func TestSpoolerOfflineRetryRecoveryAndIdempotency(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "spooler_test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	sp := spooler.NewSpooler(store)
	mockPrinter := spooler.NewVirtualMockPrinterTransport()
	mockPrinter.SetOffline(true) // Simulate printer cover open / offline

	ctx := context.Background()
	printer, err := sp.RegisterPrinter(ctx, spooler.Printer{
		ID:            "pr-counter-1",
		Name:          "Front Counter 80mm",
		Station:       "receipt_counter",
		TransportType: "virtual_mock",
		Address:       "mock://counter",
		PaperWidthMM:  80,
		IsActive:      true,
	})
	if err != nil {
		t.Fatalf("register printer: %v", err)
	}
	sp.RegisterTransport(printer.ID, mockPrinter)

	frame := escpos.RenderCashDrawerPulse()
	jobID1, err := sp.EnqueueJob(ctx, printer.ID, "cash_drawer_kick", "idem-drawer-1", frame)
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}
	// Duplicate idempotency_key must return same jobID without duplicating
	jobID2, err := sp.EnqueueJob(ctx, printer.ID, "cash_drawer_kick", "idem-drawer-1", frame)
	if err != nil || jobID1 != jobID2 {
		t.Fatalf("idempotency violation: jobID1=%s jobID2=%s err=%v", jobID1, jobID2, err)
	}

	// 1. Process while printer is offline -> transitions to failed_retry
	printed, err := sp.ProcessPending(ctx)
	if err != nil || printed != 0 {
		t.Fatalf("expected 0 printed while offline, got printed=%d err=%v", printed, err)
	}
	jobs, _ := sp.ListJobs(ctx, "failed_retry")
	if len(jobs) != 1 || jobs[0].Attempts != 1 {
		t.Fatalf("expected 1 failed_retry job with attempts=1, got %+v", jobs)
	}

	// 2. Printer recovers -> clear next_retry_at and process -> printed once
	mockPrinter.SetOffline(false)
	_, _ = store.DB().Exec(`UPDATE print_jobs SET next_retry_at = NULL WHERE id = ?`, jobID1)

	printed, err = sp.ProcessPending(ctx)
	if err != nil || printed != 1 {
		t.Fatalf("expected 1 printed after recovery, got printed=%d err=%v", printed, err)
	}
	if len(mockPrinter.Frames) != 1 {
		t.Fatalf("expected exactly 1 captured frame, got %d", len(mockPrinter.Frames))
	}

	// 3. Re-running ProcessPending must NOT double print
	printedAgain, _ := sp.ProcessPending(ctx)
	if printedAgain != 0 || len(mockPrinter.Frames) != 1 {
		t.Fatalf("duplicate print detected: printedAgain=%d frames=%d", printedAgain, len(mockPrinter.Frames))
	}
}
