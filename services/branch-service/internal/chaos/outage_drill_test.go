package chaos_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/hardware/escpos"
	"github.com/hospitality-os/branch-service/internal/hardware/spooler"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

type latencyPublisher struct {
	mu         sync.Mutex
	events     []commands.LANEvent
	delivered  []time.Duration
	lastEmitAt time.Time
}

func (p *latencyPublisher) PublishEvent(ev commands.LANEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.events = append(p.events, ev)
	if !p.lastEmitAt.IsZero() {
		p.delivered = append(p.delivered, now.Sub(p.lastEmitAt))
	}
}

type flakyTransport struct {
	mu          sync.Mutex
	failCount   int
	attempts    int
	sentBytes   [][]byte
}

func (f *flakyTransport) Send(_ context.Context, _ string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	if f.attempts <= f.failCount {
		return errors.New("tcp connection reset mid-job (lost printer ack)")
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	f.sentBytes = append(f.sentBytes, cp)
	return nil
}

func percentileDuration(durs []time.Duration, pct int) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	cp := make([]time.Duration, len(durs))
	copy(cp, durs)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := (pct*len(cp) + 99) / 100
	if idx > 0 {
		idx--
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

// Drill 1: Two-Device Pilot Vertical Slice during an internet outage
func TestDrill1_TwoDevicePilotVerticalSlice(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "drill1.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	pub := &latencyPublisher{}
	proc := commands.NewProcessor(store, pub)
	sp := spooler.NewSpooler(store)
	ctx := context.Background()
	printer, err := sp.RegisterPrinter(ctx, spooler.Printer{
		ID: "pr-f7-1", Name: "F7 Counter", Station: "receipt_counter",
		TransportType: "virtual_mock", Address: "mock://f7", PaperWidthMM: 80, IsActive: true,
	})
	if err != nil {
		t.Fatalf("register printer: %v", err)
	}
	sp.RegisterTransport(printer.ID, spooler.NewVirtualMockPrinterTransport())

	devices := []struct {
		deviceID string
		orderID  string
		orderNum string
		idemKey  string
	}{
		{"win-pos-01", "ord-pilot-a", "F7-1001", "idem-pilot-a"},
		{"android-staff-01", "ord-pilot-b", "F7-1002", "idem-pilot-b"},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(devices))
	for _, d := range devices {
		d := d
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
				OrderID:         d.orderID,
				TenantID:        "tn-pilot",
				BranchID:        "br-f7",
				OrderNumber:     d.orderNum,
				OrderMode:       "quick_sale",
				ExpectedVersion: 0,
				Subtotal:        "1170.0000",
				TaxTotal:        "187.2000",
				DiscountTotal:   "0.0000",
				GrandTotal:      "1357.2000",
				DeviceID:        d.deviceID,
				IdempotencyKey:  d.idemKey,
				Items: []commands.OrderItemInput{
					{
						MenuItemID:  "item-oat-latte",
						ItemName:    "Large Oat Latte + Extra Shot",
						PrepStation: "barista",
						Quantity:    "1",
						UnitPrice:   "1170.0000",
						LineTotal:   "1170.0000",
						CheckNumber: 1,
					},
				},
			})
			if err != nil {
				errs <- err
				return
			}
			if len(res.TicketIDs) != 1 {
				errs <- fmt.Errorf("expected 1 ticket for %s, got %d", d.orderID, len(res.TicketIDs))
				return
			}
			receipt := escpos.RenderCustomerReceipt(escpos.ReceiptData{
				PaperWidth:  escpos.Paper80mm,
				BranchName:  "F-7 Coffee Bar",
				OrderNumber: d.orderNum,
				GrandTotal:  "1,357.20",
			})
			if _, err := sp.EnqueueJob(ctx, printer.ID, "customer_receipt", "prt-"+d.idemKey, receipt); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("vertical slice device error: %v", err)
	}

	var orderCount, ticketCount, outboxCount, printCount int
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM local_orders`).Scan(&orderCount)
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM local_kitchen_tickets`).Scan(&ticketCount)
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM local_outbox WHERE status = 'pending'`).Scan(&outboxCount)
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM print_jobs`).Scan(&printCount)

	if orderCount != 2 || ticketCount != 2 || outboxCount != 2 || printCount != 2 {
		t.Fatalf("expected 2 orders/tickets/outbox/prints, got orders=%d tickets=%d outbox=%d prints=%d",
			orderCount, ticketCount, outboxCount, printCount)
	}
}

// Drill 2: Crash & Branch-Service Restart with 25 Unsynced Orders
func TestDrill2_CrashAndRestartWithUnsyncedOutbox(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "drill2_crash.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	proc := commands.NewProcessor(store, &latencyPublisher{})
	ctx := context.Background()

	for i := 1; i <= 25; i++ {
		_, err := proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
			OrderID:         fmt.Sprintf("ord-crash-%02d", i),
			TenantID:        "tn-pilot",
			BranchID:        "br-f7",
			OrderNumber:     fmt.Sprintf("F7-%04d", 2000+i),
			OrderMode:       "quick_sale",
			ExpectedVersion: 0,
			Subtotal:        "500.0000",
			TaxTotal:        "80.0000",
			DiscountTotal:   "0.0000",
			GrandTotal:      "580.0000",
			DeviceID:        "win-pos-01",
			IdempotencyKey:  fmt.Sprintf("idem-crash-%02d", i),
			Items: []commands.OrderItemInput{
				{
					MenuItemID:  "item-espresso",
					ItemName:    "Double Espresso",
					PrepStation: "barista",
					Quantity:    "1",
					UnitPrice:   "500.0000",
					LineTotal:   "500.0000",
					CheckNumber: 1,
				},
			},
		})
		if err != nil {
			t.Fatalf("order %d failed: %v", i, err)
		}
	}

	// Simulate abrupt crash / restart by closing and reopening the same SQLite DB
	_ = store.Close()

	reopened, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite after crash: %v", err)
	}
	defer reopened.Close()

	var recoveredOrders, pendingOutbox int
	_ = reopened.DB().QueryRow(`SELECT COUNT(*) FROM local_orders`).Scan(&recoveredOrders)
	_ = reopened.DB().QueryRow(`SELECT COUNT(*) FROM local_outbox WHERE status = 'pending'`).Scan(&pendingOutbox)

	if recoveredOrders != 25 || pendingOutbox != 25 {
		t.Fatalf("expected 0 lost orders (25/25), got orders=%d pending_outbox=%d", recoveredOrders, pendingOutbox)
	}
}

// Drill 3: Lost Printer Ack & 3x Duplicate Replay
func TestDrill3_LostPrinterAckAndDuplicateReplay(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "drill3_printer.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	sp := spooler.NewSpooler(store)
	mockPrinter := spooler.NewVirtualMockPrinterTransport()
	mockPrinter.SetOffline(true) // Simulate TCP connection drop / lost printer ack
	ctx := context.Background()

	printer, err := sp.RegisterPrinter(ctx, spooler.Printer{
		ID: "pr-drill3", Name: "Counter 80mm", Station: "receipt_counter",
		TransportType: "virtual_mock", Address: "mock://drill3", PaperWidthMM: 80, IsActive: true,
	})
	if err != nil {
		t.Fatalf("register printer: %v", err)
	}
	sp.RegisterTransport(printer.ID, mockPrinter)

	payload := escpos.RenderCustomerReceipt(escpos.ReceiptData{
		PaperWidth:  escpos.Paper80mm,
		BranchName:  "F-7 Coffee Bar",
		OrderNumber: "F7-3001",
		GrandTotal:  "950.00",
	})

	// Replay same print command 3 times with identical idempotency key
	var jobID string
	for attempt := 1; attempt <= 3; attempt++ {
		id, err := sp.EnqueueJob(ctx, printer.ID, "customer_receipt", "idem-print-3001", payload)
		if err != nil {
			t.Fatalf("enqueue attempt %d: %v", attempt, err)
		}
		jobID = id
	}

	var jobCount int
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM print_jobs`).Scan(&jobCount)
	if jobCount != 1 {
		t.Fatalf("expected 1 deduplicated print job, got %d", jobCount)
	}

	// First process fails (lost ack), then printer recovers and succeeds once
	_, _ = sp.ProcessPending(ctx)
	mockPrinter.SetOffline(false)
	_, _ = store.DB().Exec(`UPDATE print_jobs SET next_retry_at = NULL WHERE id = ?`, jobID)
	printed, err := sp.ProcessPending(ctx)
	if err != nil || printed != 1 || len(mockPrinter.Frames) != 1 {
		t.Fatalf("expected 1 successful print delivery after retry, got printed=%d frames=%d err=%v",
			printed, len(mockPrinter.Frames), err)
	}
}

// Drill 4: Out-of-Order Events & Conflicting Table Edits
func TestDrill4_OutOfOrderAndConflictingTableEdits(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "drill4_conflict.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	_, _ = store.DB().Exec(`INSERT INTO local_tables (id, floor_id, table_code, capacity, status) VALUES ('tbl-09', 'fl-01', 'T-09', 4, 'available')`)
	proc := commands.NewProcessor(store, &latencyPublisher{})
	ctx := context.Background()

	_, err = proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
		OrderID: "ord-t09", TenantID: "tn-pilot", BranchID: "br-g9", OrderNumber: "G9-4001",
		OrderMode: "dine_in", TableID: "tbl-09", ExpectedVersion: 0,
		Subtotal: "1000.0000", TaxTotal: "160.0000", DiscountTotal: "0.0000", GrandTotal: "1160.0000",
		DeviceID: "android-01", IdempotencyKey: "idem-t09-v1",
		Items: []commands.OrderItemInput{{MenuItemID: "m1", ItemName: "Pasta", PrepStation: "kitchen", Quantity: "1", UnitPrice: "1000.0000", LineTotal: "1000.0000", CheckNumber: 1}},
	})
	if err != nil {
		t.Fatalf("initial table order failed: %v", err)
	}

	// Stale version update (expected 0 instead of 1) must be rejected with ErrVersionConflict
	_, err = proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
		OrderID: "ord-t09", TenantID: "tn-pilot", BranchID: "br-g9", OrderNumber: "G9-4001",
		OrderMode: "dine_in", TableID: "tbl-09", ExpectedVersion: 0,
		Subtotal: "2000.0000", TaxTotal: "320.0000", DiscountTotal: "0.0000", GrandTotal: "2320.0000",
		DeviceID: "android-02", IdempotencyKey: "idem-t09-stale",
		Items: []commands.OrderItemInput{{MenuItemID: "m2", ItemName: "Steak", PrepStation: "kitchen", Quantity: "1", UnitPrice: "2000.0000", LineTotal: "2000.0000", CheckNumber: 1}},
	})
	if !errors.Is(err, commands.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

// Drill 5: Local Latency Gate Measurement (< 200ms sale ack p95, < 1000ms KDS delivery p95)
func TestDrill5_LocalLatencyGates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "drill5_latency.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	pub := &latencyPublisher{}
	proc := commands.NewProcessor(store, pub)
	ctx := context.Background()

	const iterations = 40
	saleAckLatencies := make([]time.Duration, 0, iterations)

	for i := 1; i <= iterations; i++ {
		start := time.Now()
		pub.mu.Lock()
		pub.lastEmitAt = start
		pub.mu.Unlock()

		_, err := proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
			OrderID: fmt.Sprintf("ord-lat-%03d", i), TenantID: "tn-pilot", BranchID: "br-f7",
			OrderNumber: fmt.Sprintf("F7-%04d", 5000+i), OrderMode: "quick_sale", ExpectedVersion: 0,
			Subtotal: "650.0000", TaxTotal: "104.0000", DiscountTotal: "0.0000", GrandTotal: "754.0000",
			DeviceID: "win-pos-01", IdempotencyKey: fmt.Sprintf("idem-lat-%03d", i),
			Items: []commands.OrderItemInput{{MenuItemID: "m-latte", ItemName: "Latte", PrepStation: "barista", Quantity: "1", UnitPrice: "650.0000", LineTotal: "650.0000", CheckNumber: 1}},
		})
		if err != nil {
			t.Fatalf("iteration %d failed: %v", i, err)
		}
		saleAckLatencies = append(saleAckLatencies, time.Since(start))
	}

	saleP95 := percentileDuration(saleAckLatencies, 95)
	kdsP95 := percentileDuration(pub.delivered, 95)
	t.Logf("Local Sale Ack p95=%v, Local KDS Delivery p95=%v", saleP95, kdsP95)

	if saleP95 >= 200*time.Millisecond {
		t.Fatalf("GATE FAILED: local_sale_ack_p95=%v >= 200ms", saleP95)
	}
	if kdsP95 >= 1000*time.Millisecond {
		t.Fatalf("GATE FAILED: local_kds_delivery_p95=%v >= 1000ms", kdsP95)
	}
}
