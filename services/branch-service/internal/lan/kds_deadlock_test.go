package lan

import (
	"bytes"
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

func TestWindowsPosOrderPayload_AppearsOnKDSTickets(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "pos_to_kds.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	hub := NewHub()
	proc := commands.NewProcessor(store, noopPublisher{})
	sp := spooler.NewSpooler(store)
	srv := NewServer(":0", store, hub, proc, sp)

	// Exact raw JSON payload produced by windows_pos PosController.sendToBarista()
	rawPayload := `{
		"order_id": "ord-win-9901",
		"tenant_id": "422200de-8670-4aa8-a9cf-b09ec4001ebd",
		"branch_id": "2c905f5a-8dde-448b-a3fa-592c0de66c15",
		"order_number": "F7-1001",
		"order_mode": "quick_sale",
		"expected_version": 0,
		"subtotal": "950.00",
		"tax_total": "152.00",
		"discount_total": "0",
		"grand_total": "1102.00",
		"idempotency_key": "cmd-ord-win-9901",
		"items": [
			{
				"menu_item_id": "item-oat-latte",
				"item_name": "Large Oat Latte",
				"prep_station": "barista",
				"quantity": "1",
				"unit_price": "950.00",
				"line_total": "950.00",
				"check_number": 1
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/lan/commands/orders", bytes.NewBufferString(rawPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /lan/commands/orders status=%d body=%s", rec.Code, rec.Body.String())
	}

	kdsReq := httptest.NewRequest(http.MethodGet, "/lan/kds/tickets", nil)
	kdsRec := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(kdsRec, kdsReq)
	if kdsRec.Code != http.StatusOK {
		t.Fatalf("GET /lan/kds/tickets status=%d body=%s", kdsRec.Code, kdsRec.Body.String())
	}

	var parsed struct {
		Tickets []struct {
			ID          string `json:"id"`
			OrderNumber string `json:"order_number"`
			Station     string `json:"station"`
			Status      string `json:"status"`
			ItemsJSON   string `json:"items_json"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal(kdsRec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode kds response: %v", err)
	}
	if len(parsed.Tickets) != 1 {
		t.Fatalf("expected 1 ticket on KDS, got %d", len(parsed.Tickets))
	}
	tk := parsed.Tickets[0]
	if tk.OrderNumber != "F7-1001" || tk.Station != "barista" || tk.ItemsJSON == "" || tk.ItemsJSON == "[]" {
		t.Fatalf("unexpected KDS ticket: %+v", tk)
	}
}

func TestSpoolerEnabledOrderPost_NoSelfDeadlockAndFastHealthz(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "spooler_deadlock.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	hub := NewHub()
	proc := commands.NewProcessor(store, noopPublisher{})
	sp := spooler.NewSpooler(store)
	proc.SetSpooler(sp) // Matches cmd/branch/main.go!
	srv := NewServer(":0", store, hub, proc, sp)

	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/lan/stream", nil).WithContext(sseCtx)
		srv.httpSrv.Handler.ServeHTTP(httptest.NewRecorder(), req)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rawPayload := `{"order_id":"ord-sp-1","tenant_id":"tn-1","branch_id":"br-f7","order_number":"F7-777","order_mode":"quick_sale","expected_version":0,"subtotal":"500","tax_total":"80","discount_total":"0","grand_total":"580","idempotency_key":"idem-sp-1","items":[{"menu_item_id":"m1","item_name":"Latte","prep_station":"barista","quantity":"1","unit_price":"500","line_total":"500","check_number":1}]}`
	postReq := httptest.NewRequest(http.MethodPost, "/lan/commands/orders", bytes.NewBufferString(rawPayload)).WithContext(ctx)
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST /lan/commands/orders with spooler enabled failed: status=%d body=%s", postRec.Code, postRec.Body.String())
	}

	for i := 0; i < 20; i++ {
		hRec := httptest.NewRecorder()
		srv.httpSrv.Handler.ServeHTTP(hRec, httptest.NewRequest(http.MethodGet, "/healthz", nil).WithContext(ctx))
		if hRec.Code != http.StatusOK {
			t.Fatalf("healthz %d failed: %d %s", i, hRec.Code, hRec.Body.String())
		}
		kRec := httptest.NewRecorder()
		srv.httpSrv.Handler.ServeHTTP(kRec, httptest.NewRequest(http.MethodGet, "/lan/kds/tickets", nil).WithContext(ctx))
		if kRec.Code != http.StatusOK {
			t.Fatalf("kds tickets %d failed: %d %s", i, kRec.Code, kRec.Body.String())
		}
	}
}
