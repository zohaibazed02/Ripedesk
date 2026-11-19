package commands_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

type spyPublisher struct {
	events []commands.LANEvent
}

func (s *spyPublisher) PublishEvent(ev commands.LANEvent) {
	s.events = append(s.events, ev)
}

func TestAtomicOrderOutboxAndVersionConflict(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "branch_test.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer store.Close()

	// Seed a local table
	_, err = store.DB().Exec(`
		INSERT INTO local_tables (id, floor_id, table_code, capacity, status)
		VALUES ('tbl-01', 'fl-01', 'T-01', 4, 'available')
	`)
	if err != nil {
		t.Fatalf("seed local_tables: %v", err)
	}

	pub := &spyPublisher{}
	proc := commands.NewProcessor(store, pub)
	ctx := context.Background()

	// 1. Create order atomically
	res, err := proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
		OrderID:         "ord-001",
		TenantID:        "tn-01",
		BranchID:        "br-01",
		OrderNumber:     "F7-0001",
		OrderMode:       "dine_in",
		TableID:         "tbl-01",
		ExpectedVersion: 0,
		Subtotal:        "950.0000",
		TaxTotal:        "152.0000",
		DiscountTotal:   "0.0000",
		GrandTotal:      "1102.0000",
		DeviceID:        "dev-pos-01",
		IdempotencyKey:  "cmd-ord-001-r1",
		Items: []commands.OrderItemInput{
			{
				MenuItemID:  "item-latte",
				ItemName:    "Large Oat Latte",
				PrepStation: "barista",
				Quantity:    "1",
				UnitPrice:   "950.0000",
				LineTotal:   "950.0000",
				CheckNumber: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrUpdateOrder failed: %v", err)
	}
	if res.OrderVersion != 1 || len(res.TicketIDs) != 1 || res.LANSeqID != 1 {
		t.Fatalf("unexpected order result: %+v", res)
	}
	if len(pub.events) != 1 || pub.events[0].EventType != "order.upserted" {
		t.Fatalf("expected 1 published LAN event, got %d", len(pub.events))
	}

	// Verify atomic commit across local_orders, local_kitchen_tickets, lan_events, local_outbox, local_tables
	var outboxCount int
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM local_outbox WHERE status = 'pending'`).Scan(&outboxCount)
	if outboxCount != 1 {
		t.Fatalf("expected 1 pending outbox row, got %d", outboxCount)
	}
	var tableStatus string
	_ = store.DB().QueryRow(`SELECT status FROM local_tables WHERE id = 'tbl-01'`).Scan(&tableStatus)
	if tableStatus != "occupied" {
		t.Fatalf("expected table status 'occupied', got %q", tableStatus)
	}

	// 2. Concurrent stale expected_version MUST fail with ErrVersionConflict
	_, err = proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
		OrderID:         "ord-001",
		TenantID:        "tn-01",
		BranchID:        "br-01",
		OrderNumber:     "F7-0001",
		OrderMode:       "dine_in",
		TableID:         "tbl-01",
		ExpectedVersion: 0, // stale! current version is 1
		Subtotal:        "1200.0000",
		TaxTotal:        "192.0000",
		DiscountTotal:   "0.0000",
		GrandTotal:      "1392.0000",
		DeviceID:        "dev-pos-02",
		IdempotencyKey:  "cmd-ord-001-stale",
	})
	if !errors.Is(err, commands.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	// 3. Bump ticket & record full payment -> settles table to 'dirty'
	if _, err := proc.BumpOrRecallTicket(ctx, commands.BumpTicketInput{
		TicketID:       res.TicketIDs[0],
		Status:         "bumped",
		IdempotencyKey: "cmd-bump-001",
	}); err != nil {
		t.Fatalf("BumpOrRecallTicket failed: %v", err)
	}

	_, settled, err := proc.RecordPayment(ctx, commands.RecordPaymentInput{
		OrderID:        "ord-001",
		CheckNumber:    1,
		Method:         "cash",
		Amount:         "1102.0000",
		TenderedAmount: "1200.0000",
		ChangeDue:      "98.0000",
		IdempotencyKey: "cmd-pay-001",
	})
	if err != nil {
		t.Fatalf("RecordPayment failed: %v", err)
	}
	if !settled {
		t.Fatalf("expected order to be settled")
	}

	_ = store.DB().QueryRow(`SELECT status FROM local_tables WHERE id = 'tbl-01'`).Scan(&tableStatus)
	if tableStatus != "dirty" {
		t.Fatalf("expected table status 'dirty' after full settlement, got %q", tableStatus)
	}
}
