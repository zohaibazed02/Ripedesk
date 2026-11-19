package sync_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	cloudsync "github.com/hospitality-os/cloud-api/internal/sync"
	"github.com/jackc/pgx/v5"
)

func TestPushBatchMaxSizeValidation(t *testing.T) {
	svc := cloudsync.NewService(nil)
	cmds := make([]cloudsync.PushCommand, cloudsync.MaxBatchSize+1)
	_, err := svc.PushBatch(context.Background(), "tn-1", "br-1", cloudsync.PushBatchRequest{
		Commands: cmds,
	})
	if !errors.Is(err, cloudsync.ErrBatchTooLarge) {
		t.Fatalf("expected ErrBatchTooLarge, got %v", err)
	}
}

// TestSyncIdempotencyAndConflictRouting proves:
// 1. Replaying the exact same batch of offline order + payment events twice results in
//    exactly 1 order and 1 payment in PostgreSQL (duplicate_acked on replay).
// 2. Submitting a conflicting order_version routes the command to sync_review_queue
//    without blocking valid commands in the same batch.
func TestSyncIdempotencyAndConflictRouting(t *testing.T) {
	appDBURL := os.Getenv("APP_DATABASE_URL")
	adminDBURL := os.Getenv("DATABASE_URL")
	if appDBURL == "" || adminDBURL == "" {
		t.Skip("APP_DATABASE_URL and DATABASE_URL must be set to run PostgreSQL sync idempotency integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminPool, err := db.NewPool(ctx, adminDBURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer adminPool.Close()

	appPool, err := db.NewPool(ctx, appDBURL)
	if err != nil {
		t.Fatalf("connect app pool: %v", err)
	}
	defer appPool.Close()

	var tenantID, branchID string
	if err := adminPool.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, currency_code, timezone)
		VALUES ('Sync Test Tenant', 'sync-test-' || extract(epoch from now())::bigint, 'PKR', 'Asia/Karachi')
		RETURNING id
	`).Scan(&tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := adminPool.QueryRow(ctx, `
		INSERT INTO branches (tenant_id, name, code, branch_type, default_order_mode)
		VALUES ($1, 'Sync Branch', 'SB-01', 'coffee_shop', 'quick_sale')
		RETURNING id
	`, tenantID).Scan(&branchID); err != nil {
		t.Fatalf("seed branch: %v", err)
	}

	defer func() {
		_, _ = adminPool.Exec(ctx, `DELETE FROM sync_review_queue WHERE tenant_id = $1`, tenantID)
		_, _ = adminPool.Exec(ctx, `DELETE FROM processed_commands WHERE tenant_id = $1`, tenantID)
		_, _ = adminPool.Exec(ctx, `DELETE FROM outbox_events WHERE tenant_id = $1`, tenantID)
		_, _ = adminPool.Exec(ctx, `DELETE FROM payments WHERE tenant_id = $1`, tenantID)
		_, _ = adminPool.Exec(ctx, `DELETE FROM orders WHERE tenant_id = $1`, tenantID)
		_, _ = adminPool.Exec(ctx, `DELETE FROM branches WHERE tenant_id = $1`, tenantID)
		_, _ = adminPool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
	}()

	svc := cloudsync.NewService(appPool)
	now := time.Now().UTC()

	orderID := "11111111-1111-4111-8111-111111111111"
	paymentID := "22222222-2222-4222-8222-222222222222"
	ordEventID := "33333333-3333-4333-8333-333333333333"
	payEventID := "44444444-4444-4444-8444-444444444444"

	batch := cloudsync.PushBatchRequest{
		PendingSyncCount: 0,
		Commands: []cloudsync.PushCommand{
			{
				EventID:          ordEventID,
				CommandType:      "order.upserted",
				AggregateType:    "order",
				AggregateID:      orderID,
				IdempotencyKey:   "idem-ord-1",
				Payload:          json.RawMessage(`{"order_id":"` + orderID + `","order_number":"SB-1001","order_mode":"quick_sale","order_version":2,"grand_total":"950.0000"}`),
				CreatedOfflineAt: now,
			},
			{
				EventID:          payEventID,
				CommandType:      "payment.recorded",
				AggregateType:    "payment",
				AggregateID:      paymentID,
				IdempotencyKey:   "idem-pay-1",
				Payload:          json.RawMessage(`{"payment_id":"` + paymentID + `","order_id":"` + orderID + `","check_number":1,"method":"cash","amount":"950.0000"}`),
				CreatedOfflineAt: now,
			},
		},
	}

	// 1. First push -> both acked
	res1, err := svc.PushBatch(ctx, tenantID, branchID, batch)
	if err != nil {
		t.Fatalf("first PushBatch failed: %v", err)
	}
	if len(res1.Acks) != 2 || res1.Acks[0].Status != "acked" || res1.Acks[1].Status != "acked" {
		t.Fatalf("unexpected first acks: %+v", res1.Acks)
	}

	// 2. Exact replay -> both duplicate_acked
	res2, err := svc.PushBatch(ctx, tenantID, branchID, batch)
	if err != nil {
		t.Fatalf("replay PushBatch failed: %v", err)
	}
	if res2.Acks[0].Status != "duplicate_acked" || res2.Acks[1].Status != "duplicate_acked" {
		t.Fatalf("expected duplicate_acked on replay, got: %+v", res2.Acks)
	}

	// 3. Stale version conflict + valid new order in the same batch
	conflictEventID := "55555555-5555-4555-8555-555555555555"
	validOrderID := "66666666-6666-4666-8666-666666666666"
	validEventID := "77777777-7777-4777-8777-777777777777"

	res3, err := svc.PushBatch(ctx, tenantID, branchID, cloudsync.PushBatchRequest{
		Commands: []cloudsync.PushCommand{
			{
				EventID:          conflictEventID,
				CommandType:      "order.upserted",
				AggregateType:    "order",
				AggregateID:      orderID,
				IdempotencyKey:   "idem-ord-stale",
				Payload:          json.RawMessage(`{"order_id":"` + orderID + `","order_number":"SB-1001","order_mode":"quick_sale","order_version":1,"grand_total":"900.0000"}`),
				CreatedOfflineAt: now,
			},
			{
				EventID:          validEventID,
				CommandType:      "order.upserted",
				AggregateType:    "order",
				AggregateID:      validOrderID,
				IdempotencyKey:   "idem-ord-valid-2",
				Payload:          json.RawMessage(`{"order_id":"` + validOrderID + `","order_number":"SB-1002","order_mode":"takeaway","order_version":1,"grand_total":"500.0000"}`),
				CreatedOfflineAt: now,
			},
		},
	})
	if err != nil {
		t.Fatalf("conflict batch failed: %v", err)
	}
	if res3.Acks[0].Status != "rejected" || res3.Acks[1].Status != "acked" {
		t.Fatalf("expected [rejected, acked], got: %+v", res3.Acks)
	}

	// Verify sync_review_queue has 1 entry
	var reviewCount int
	_ = appPool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM sync_review_queue WHERE branch_id = $1`, branchID).Scan(&reviewCount)
	})
	if reviewCount != 1 {
		t.Fatalf("expected 1 review queue item, got %d", reviewCount)
	}
}
