package sync_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/sqlite"
	branchsync "github.com/hospitality-os/branch-service/internal/sync"
)

type spyBroadcaster struct {
	lastReachable bool
	calls         int
}

func (s *spyBroadcaster) BroadcastSyncStatus(cloudReachable bool, _ int) {
	s.lastReachable = cloudReachable
	s.calls++
}

// TestOutageAndRecoveryReplay simulates an internet outage (HTTP 503) followed by
// recovery (HTTP 200), proving buffered offline orders drain in strict FIFO order
// and update cloud_reachable and pending_outbox_count accurately.
func TestOutageAndRecoveryReplay(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "branch_replay.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	var isOnline atomic.Bool
	isOnline.Store(false) // Start in simulated internet outage
	var receivedOrderIDs []string

	cloudServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isOnline.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Commands []struct {
				EventID     string `json:"event_id"`
				AggregateID string `json:"aggregate_id"`
			} `json:"commands"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		type ack struct {
			EventID string `json:"event_id"`
			Status  string `json:"status"`
		}
		acks := make([]ack, 0, len(req.Commands))
		for _, c := range req.Commands {
			receivedOrderIDs = append(receivedOrderIDs, c.AggregateID)
			acks = append(acks, ack{EventID: c.EventID, Status: "acked"})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"acks": acks},
		})
	}))
	defer cloudServer.Close()

	// Configure branch_state pointing at mock cloud server
	_, err = store.DB().Exec(`
		UPDATE branch_state
		SET tenant_id = 'tn-1', branch_id = 'br-1', device_id = 'dev-1', device_secret = 'sec-1', cloud_api_url = ?
		WHERE id = 1
	`, cloudServer.URL)
	if err != nil {
		t.Fatalf("update branch_state: %v", err)
	}

	// Create 2 offline orders locally while internet is down
	proc := commands.NewProcessor(store, nil)
	ctx := context.Background()
	for _, ordID := range []string{"ord-offline-1", "ord-offline-2"} {
		_, err := proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
			OrderID:         ordID,
			TenantID:        "tn-1",
			BranchID:        "br-1",
			OrderNumber:     "OFF-" + ordID,
			OrderMode:       "quick_sale",
			ExpectedVersion: 0,
			Subtotal:        "500.0000",
			TaxTotal:        "80.0000",
			DiscountTotal:   "0.0000",
			GrandTotal:      "580.0000",
			DeviceID:        "dev-1",
			IdempotencyKey:  "idem-" + ordID,
			Items: []commands.OrderItemInput{
				{
					MenuItemID:  "item-1",
					ItemName:    "Espresso",
					PrepStation: "barista",
					Quantity:    "1",
					UnitPrice:   "500.0000",
					LineTotal:   "500.0000",
				},
			},
		})
		if err != nil {
			t.Fatalf("create offline order %s: %v", ordID, err)
		}
	}

	broadcaster := &spyBroadcaster{}
	engine := branchsync.NewEngine(store, broadcaster, "test-jwt")

	// 1. Attempt sync during outage -> should fail, flip cloud_reachable = false, keep 2 pending
	if _, err := engine.ShipNextBatch(ctx); err == nil {
		t.Fatalf("expected error during simulated outage")
	}
	snap, _ := store.HealthSnapshot(ctx)
	if snap.CloudReachable {
		t.Fatalf("expected cloud_reachable=false during outage")
	}
	if snap.PendingOutboxCount != 2 {
		t.Fatalf("expected 2 pending outbox items during outage, got %d", snap.PendingOutboxCount)
	}
	if broadcaster.lastReachable {
		t.Fatalf("expected broadcaster to report offline status")
	}

	// Clear next_retry_at so test doesn't sleep for backoff timer
	_, _ = store.DB().Exec(`UPDATE local_outbox SET next_retry_at = NULL`)

	// 2. Restore internet -> ShipNextBatch should drain both orders in FIFO order
	isOnline.Store(true)
	shipped, err := engine.ShipNextBatch(ctx)
	if err != nil {
		t.Fatalf("ShipNextBatch after recovery failed: %v", err)
	}
	if shipped != 2 {
		t.Fatalf("expected 2 shipped events, got %d", shipped)
	}
	if len(receivedOrderIDs) != 2 || receivedOrderIDs[0] != "ord-offline-1" || receivedOrderIDs[1] != "ord-offline-2" {
		t.Fatalf("expected strict FIFO order [ord-offline-1, ord-offline-2], got %v", receivedOrderIDs)
	}

	snap, _ = store.HealthSnapshot(ctx)
	if !snap.CloudReachable {
		t.Fatalf("expected cloud_reachable=true after recovery")
	}
	if snap.PendingOutboxCount != 0 {
		t.Fatalf("expected 0 pending outbox items after recovery, got %d", snap.PendingOutboxCount)
	}
}
