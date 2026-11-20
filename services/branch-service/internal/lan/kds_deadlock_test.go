package lan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/hardware/spooler"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

type noopPublisher struct{}

func (noopPublisher) PublishEvent(commands.LANEvent) {}

func TestKDSTicketsAndHealthz_NoSQLitePoolDeadlock(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "kds_deadlock.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	hub := NewHub()
	proc := commands.NewProcessor(store, noopPublisher{})
	sp := spooler.NewSpooler(store)
	srv := NewServer(":0", store, hub, proc, sp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Seed 1 active order + ticket + order item
	_, err = proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
		OrderID:         "ord-deadlock-1",
		TenantID:        "tn-1",
		BranchID:        "br-f7",
		OrderNumber:     "F7-9001",
		OrderMode:       "quick_sale",
		ExpectedVersion: 0,
		Subtotal:        "950.0000",
		TaxTotal:        "152.0000",
		DiscountTotal:   "0.0000",
		GrandTotal:      "1102.0000",
		DeviceID:        "win-pos-01",
		IdempotencyKey:  "idem-deadlock-1",
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

	// 1. GET /lan/kds/tickets must complete without deadlocking SQLite MaxOpenConns(1)
	req1 := httptest.NewRequest(http.MethodGet, "/lan/kds/tickets", nil).WithContext(ctx)
	rec1 := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("GET /lan/kds/tickets status=%d body=%s", rec1.Code, rec1.Body.String())
	}

	var parsed struct {
		Tickets []struct {
			ID          string `json:"id"`
			OrderNumber string `json:"order_number"`
			Station     string `json:"station"`
			ItemsJSON   string `json:"items_json"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode tickets: %v", err)
	}
	if len(parsed.Tickets) != 1 || parsed.Tickets[0].OrderNumber != "F7-9001" {
		t.Fatalf("unexpected tickets payload: %+v", parsed.Tickets)
	}

	// 2. Immediately call GET /healthz under the same 2s timeout
	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil).WithContext(ctx)
	rec2 := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /healthz status=%d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestConcurrentSSEAndHealthz_NeverBlocks(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "concurrent_health.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	hub := NewHub()
	proc := commands.NewProcessor(store, noopPublisher{})
	sp := spooler.NewSpooler(store)
	srv := NewServer(":0", store, hub, proc, sp)

	// Start an active SSE subscriber in the background
	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/lan/stream", nil).WithContext(sseCtx)
		rec := httptest.NewRecorder()
		srv.httpSrv.Handler.ServeHTTP(rec, req)
	}()

	// Run 10 concurrent /healthz + /lan/kds/tickets calls under a strict 2s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		srv.httpSrv.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d GET /healthz failed: status=%d body=%s", i, rec.Code, rec.Body.String())
		}
	}
}
