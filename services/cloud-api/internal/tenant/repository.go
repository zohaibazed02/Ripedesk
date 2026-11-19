package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/jackc/pgx/v5"
)

type TenantRecord struct {
	ID           string
	Name         string
	Slug         string
	CurrencyCode string
	Timezone     string
	Entitlements map[string]bool
}

type UserRecord struct {
	ID           string
	TenantID     string
	FullName     string
	Email        string
	PasswordHash string
	PINHash      string
	RoleCode     string
	CanOverride  bool
	IsActive     bool
	BranchIDs    []string
}

type BranchSummary struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id"`
	Name               string          `json:"name"`
	Code               string          `json:"code"`
	BranchType         string          `json:"branch_type"`
	DefaultOrderMode   string          `json:"default_order_mode"`
	HeartbeatExpiresAt *time.Time      `json:"heartbeat_expires_at"`
	LastSyncAt         *time.Time      `json:"last_sync_at"`
	PendingSyncCount   int             `json:"pending_sync_count"`
	OldestPendingTxAt  *time.Time      `json:"oldest_pending_tx_at"`
	IsOnline           bool            `json:"is_online"`
	Config             json.RawMessage `json:"config"`
}

type DeviceRecord struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	BranchID            string     `json:"branch_id"`
	DeviceName          string     `json:"device_name"`
	DeviceType          string     `json:"device_type"`
	HardwareFingerprint string     `json:"hardware_fingerprint"`
	Status              string     `json:"status"`
	IsolatedModeEnabled bool       `json:"isolated_mode_enabled"`
	LastSeenAt          *time.Time `json:"last_seen_at"`
}

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindTenantBySlug(ctx context.Context, slug string) (*TenantRecord, error) {
	var t TenantRecord
	var entRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, currency_code, timezone, entitlements
		FROM tenants
		WHERE slug = $1 AND status = 'active'
	`, slug).Scan(&t.ID, &t.Name, &t.Slug, &t.CurrencyCode, &t.Timezone, &entRaw)
	if err != nil {
		return nil, err
	}
	t.Entitlements = map[string]bool{}
	_ = json.Unmarshal(entRaw, &t.Entitlements)
	return &t, nil
}

func (r *Repository) FindUserByEmail(ctx context.Context, tenantID, email string) (*UserRecord, error) {
	var u UserRecord
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT u.id, u.tenant_id, u.full_name, u.email, u.password_hash, u.pin_hash,
			       ro.code, ro.can_override, u.is_active
			FROM users u
			JOIN roles ro ON ro.tenant_id = u.tenant_id AND ro.id = u.role_id
			WHERE u.email = $1
		`, email).Scan(
			&u.ID, &u.TenantID, &u.FullName, &u.Email, &u.PasswordHash, &u.PINHash,
			&u.RoleCode, &u.CanOverride, &u.IsActive,
		); err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `SELECT branch_id FROM user_branch_assignments WHERE user_id = $1`, u.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var bID string
			if err := rows.Scan(&bID); err != nil {
				return err
			}
			u.BranchIDs = append(u.BranchIDs, bID)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) FindUserByPINHash(ctx context.Context, tenantID, pinHash string) (*UserRecord, error) {
	var u UserRecord
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT u.id, u.tenant_id, u.full_name, u.email, ro.code, ro.can_override, u.is_active
			FROM users u
			JOIN roles ro ON ro.tenant_id = u.tenant_id AND ro.id = u.role_id
			WHERE u.pin_hash = $1 AND u.is_active = TRUE
			LIMIT 1
		`, pinHash).Scan(&u.ID, &u.TenantID, &u.FullName, &u.Email, &u.RoleCode, &u.CanOverride, &u.IsActive)
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) RecordAuditEvent(ctx context.Context, tenantID, branchID, aggregateID, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var bID any = nil
		if branchID != "" {
			bID = branchID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO outbox_events (tenant_id, branch_id, aggregate_type, aggregate_id, event_type, payload)
			VALUES ($1, $2, 'audit', $3, $4, $5::jsonb)
		`, tenantID, bID, aggregateID, eventType, string(raw))
		return err
	})
}

func (r *Repository) EnrollDevice(ctx context.Context, tenantID, branchID, name, deviceType, fingerprint, keyHash string) (*DeviceRecord, json.RawMessage, error) {
	var dev DeviceRecord
	var branchConfig json.RawMessage
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT config FROM branches WHERE id = $1
		`, branchID).Scan(&branchConfig); err != nil {
			return fmt.Errorf("branch lookup: %w", err)
		}

		return tx.QueryRow(ctx, `
			INSERT INTO devices (tenant_id, branch_id, device_name, device_type, hardware_fingerprint, public_key_hash, status, last_seen_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'enrolled', NOW())
			ON CONFLICT (tenant_id, hardware_fingerprint)
			DO UPDATE SET
				branch_id = EXCLUDED.branch_id,
				device_name = EXCLUDED.device_name,
				device_type = EXCLUDED.device_type,
				public_key_hash = EXCLUDED.public_key_hash,
				status = 'enrolled',
				last_seen_at = NOW()
			RETURNING id, tenant_id, branch_id, device_name, device_type, hardware_fingerprint, status, isolated_mode_enabled, last_seen_at
		`, tenantID, branchID, name, deviceType, fingerprint, keyHash).Scan(
			&dev.ID, &dev.TenantID, &dev.BranchID, &dev.DeviceName, &dev.DeviceType,
			&dev.HardwareFingerprint, &dev.Status, &dev.IsolatedModeEnabled, &dev.LastSeenAt,
		)
	})
	if err != nil {
		return nil, nil, err
	}
	return &dev, branchConfig, nil
}

func (r *Repository) ListBranches(ctx context.Context, tenantID string) ([]BranchSummary, error) {
	var branches []BranchSummary
	now := time.Now().UTC()
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, tenant_id, name, code, branch_type, default_order_mode,
			       heartbeat_expires_at, last_sync_at, pending_sync_count, oldest_pending_tx_at, config
			FROM branches
			ORDER BY created_at ASC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b BranchSummary
			if err := rows.Scan(
				&b.ID, &b.TenantID, &b.Name, &b.Code, &b.BranchType, &b.DefaultOrderMode,
				&b.HeartbeatExpiresAt, &b.LastSyncAt, &b.PendingSyncCount, &b.OldestPendingTxAt, &b.Config,
			); err != nil {
				return err
			}
			if b.HeartbeatExpiresAt != nil && b.HeartbeatExpiresAt.After(now) {
				b.IsOnline = true
			}
			branches = append(branches, b)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return branches, nil
}

func (r *Repository) UpdateBranchHeartbeat(
	ctx context.Context,
	tenantID, branchID string,
	ttlSeconds int,
	pendingSyncCount int,
	oldestPendingTxAt *time.Time,
) (*BranchSummary, error) {
	var b BranchSummary
	expiresAt := time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second)
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			UPDATE branches
			SET heartbeat_expires_at = $2,
			    last_sync_at = NOW(),
			    pending_sync_count = $3,
			    oldest_pending_tx_at = $4
			WHERE id = $1
			RETURNING id, tenant_id, name, code, branch_type, default_order_mode,
			          heartbeat_expires_at, last_sync_at, pending_sync_count, oldest_pending_tx_at, config
		`, branchID, expiresAt, pendingSyncCount, oldestPendingTxAt).Scan(
			&b.ID, &b.TenantID, &b.Name, &b.Code, &b.BranchType, &b.DefaultOrderMode,
			&b.HeartbeatExpiresAt, &b.LastSyncAt, &b.PendingSyncCount, &b.OldestPendingTxAt, &b.Config,
		)
	})
	if err != nil {
		return nil, err
	}
	b.IsOnline = true
	return &b, nil
}
