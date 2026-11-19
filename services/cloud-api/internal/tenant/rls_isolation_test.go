package tenant_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/jackc/pgx/v5"
)

// TestRLSTenantIsolation proves that WithTenantTx enforces PostgreSQL Row-Level Security
// using the non-superuser pos_app_user role: Tenant A cannot read or mutate Tenant B's
// branches, devices, or orders even when queries omit a WHERE tenant_id = ... clause.
func TestRLSTenantIsolation(t *testing.T) {
	appDBURL := os.Getenv("APP_DATABASE_URL")
	adminDBURL := os.Getenv("DATABASE_URL")
	if appDBURL == "" || adminDBURL == "" {
		t.Skip("APP_DATABASE_URL and DATABASE_URL must be set to run PostgreSQL RLS integration test")
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
		t.Fatalf("connect app pool (pos_app_user): %v", err)
	}
	defer appPool.Close()

	// Create two isolated test tenants using admin connection
	var tenantA, tenantB string
	err = adminPool.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, currency_code, timezone)
		VALUES ('RLS Test Tenant A', 'rls-test-a-' || extract(epoch from now())::bigint, 'USD', 'UTC')
		RETURNING id
	`).Scan(&tenantA)
	if err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	err = adminPool.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, currency_code, timezone)
		VALUES ('RLS Test Tenant B', 'rls-test-b-' || extract(epoch from now())::bigint, 'USD', 'UTC')
		RETURNING id
	`).Scan(&tenantB)
	if err != nil {
		t.Fatalf("create tenant B: %v", err)
	}

	defer func() {
		_, _ = adminPool.Exec(ctx, `DELETE FROM orders WHERE tenant_id IN ($1, $2)`, tenantA, tenantB)
		_, _ = adminPool.Exec(ctx, `DELETE FROM devices WHERE tenant_id IN ($1, $2)`, tenantA, tenantB)
		_, _ = adminPool.Exec(ctx, `DELETE FROM branches WHERE tenant_id IN ($1, $2)`, tenantA, tenantB)
		_, _ = adminPool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, tenantA, tenantB)
	}()

	// Seed a branch, device, and order for Tenant B via WithTenantTx(tenantB)
	var branchB, deviceB, orderB string
	err = appPool.WithTenantTx(ctx, tenantB, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO branches (tenant_id, name, code, branch_type, default_order_mode)
			VALUES ($1, 'Tenant B Secret Branch', 'TB-01', 'coffee_shop', 'quick_sale')
			RETURNING id
		`, tenantB).Scan(&branchB); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO devices (tenant_id, branch_id, device_name, device_type, hardware_fingerprint, public_key_hash)
			VALUES ($1, $2, 'Tenant B POS', 'windows_pos', 'fp-tenant-b-01', 'hash-b')
			RETURNING id
		`, tenantB, branchB).Scan(&deviceB); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			INSERT INTO orders (id, tenant_id, branch_id, order_number, order_mode, status, grand_total)
			VALUES (gen_random_uuid(), $1, $2, 'ORD-B-1001', 'quick_sale', 'open', 25.0000)
			RETURNING id
		`, tenantB, branchB).Scan(&orderB)
	})
	if err != nil {
		t.Fatalf("seed tenant B resources: %v", err)
	}

	// Now query as Tenant A WITHOUT any WHERE tenant_id = ... clause
	err = appPool.WithTenantTx(ctx, tenantA, func(tx pgx.Tx) error {
		var branchCount, deviceCount, orderCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM branches`).Scan(&branchCount); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM devices`).Scan(&deviceCount); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&orderCount); err != nil {
			return err
		}
		if branchCount != 0 || deviceCount != 0 || orderCount != 0 {
			t.Errorf("RLS leak detected for Tenant A: branches=%d devices=%d orders=%d (expected 0)", branchCount, deviceCount, orderCount)
		}

		// Attempt cross-tenant UPDATE against Tenant B's branch by ID
		tag, err := tx.Exec(ctx, `UPDATE branches SET name = 'HACKED' WHERE id = $1`, branchB)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			t.Errorf("RLS mutation leak: Tenant A updated %d rows of Tenant B branch %s", tag.RowsAffected(), branchB)
		}

		// Attempt cross-tenant DELETE against Tenant B's order by ID
		tag, err = tx.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderB)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			t.Errorf("RLS delete leak: Tenant A deleted %d rows of Tenant B order %s", tag.RowsAffected(), orderB)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tenant A RLS verification tx failed: %v", err)
	}
}
