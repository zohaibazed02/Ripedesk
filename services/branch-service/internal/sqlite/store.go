package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	path string
	db   *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// SQLite WAL with a single writer connection avoids SQLITE_BUSY lock errors.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=FULL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", p, err)
		}
	}

	s := &Store{path: path, db: db}
	if err := s.bootstrap(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap sqlite schema: %w", err)
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }
func (s *Store) DB() *sql.DB  { return s.db }
func (s *Store) Close() error { return s.db.Close() }

// BeginImmediate starts a SQLite BEGIN IMMEDIATE transaction for atomic local writes.
func (s *Store) BeginImmediate(ctx context.Context) (*sql.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (s *Store) bootstrap() error {
	const schema = `
CREATE TABLE IF NOT EXISTS branch_state (
	id                  INTEGER PRIMARY KEY CHECK (id = 1),
	tenant_id           TEXT NOT NULL DEFAULT '',
	branch_id           TEXT NOT NULL DEFAULT '',
	device_id           TEXT NOT NULL DEFAULT '',
	device_secret       TEXT NOT NULL DEFAULT '',
	cloud_api_url       TEXT NOT NULL DEFAULT '',
	catalog_etag        TEXT NOT NULL DEFAULT '',
	catalog_json        TEXT NOT NULL DEFAULT '{}',
	last_cloud_sync_at  TEXT,
	cloud_reachable     INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO branch_state (id) VALUES (1);

CREATE TABLE IF NOT EXISTS local_tables (
	id              TEXT PRIMARY KEY,
	floor_id        TEXT NOT NULL,
	table_code      TEXT NOT NULL UNIQUE,
	capacity        INTEGER NOT NULL DEFAULT 2,
	pos_x           INTEGER NOT NULL DEFAULT 0,
	pos_y           INTEGER NOT NULL DEFAULT 0,
	shape           TEXT NOT NULL DEFAULT 'rectangle',
	status          TEXT NOT NULL DEFAULT 'available'
	                CHECK (status IN ('available', 'occupied', 'reserved', 'dirty')),
	active_order_id TEXT,
	order_version   INTEGER NOT NULL DEFAULT 0,
	updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS local_orders (
	id                  TEXT PRIMARY KEY,
	tenant_id           TEXT NOT NULL,
	branch_id           TEXT NOT NULL,
	order_number        TEXT NOT NULL,
	order_mode          TEXT NOT NULL,
	table_id            TEXT,
	order_version       INTEGER NOT NULL DEFAULT 1,
	status              TEXT NOT NULL DEFAULT 'open',
	subtotal            TEXT NOT NULL DEFAULT '0.0000',
	tax_total           TEXT NOT NULL DEFAULT '0.0000',
	discount_total      TEXT NOT NULL DEFAULT '0.0000',
	grand_total         TEXT NOT NULL DEFAULT '0.0000',
	device_id           TEXT NOT NULL DEFAULT '',
	created_offline_at  TEXT NOT NULL,
	updated_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_order_rounds (
	id                  TEXT PRIMARY KEY,
	order_id            TEXT NOT NULL REFERENCES local_orders(id) ON DELETE CASCADE,
	round_number        INTEGER NOT NULL,
	prep_confirmed_at   TEXT,
	created_at          TEXT NOT NULL,
	UNIQUE (order_id, round_number)
);

CREATE TABLE IF NOT EXISTS local_order_items (
	id                  TEXT PRIMARY KEY,
	order_id            TEXT NOT NULL REFERENCES local_orders(id) ON DELETE CASCADE,
	order_round_id      TEXT NOT NULL REFERENCES local_order_rounds(id) ON DELETE CASCADE,
	menu_item_id        TEXT NOT NULL,
	item_name           TEXT NOT NULL,
	prep_station        TEXT NOT NULL DEFAULT 'hot_kitchen',
	quantity            TEXT NOT NULL,
	unit_price          TEXT NOT NULL,
	line_total          TEXT NOT NULL,
	seat_number         INTEGER,
	check_number        INTEGER NOT NULL DEFAULT 1,
	prep_confirmed_at   TEXT,
	notes               TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS local_order_item_modifiers (
	id                  TEXT PRIMARY KEY,
	order_item_id       TEXT NOT NULL REFERENCES local_order_items(id) ON DELETE CASCADE,
	modifier_id         TEXT NOT NULL,
	modifier_name       TEXT NOT NULL,
	quantity            TEXT NOT NULL DEFAULT '1',
	price_delta         TEXT NOT NULL DEFAULT '0.0000'
);

CREATE TABLE IF NOT EXISTS local_kitchen_tickets (
	id                  TEXT PRIMARY KEY,
	order_id            TEXT NOT NULL REFERENCES local_orders(id) ON DELETE CASCADE,
	order_round_id      TEXT NOT NULL REFERENCES local_order_rounds(id) ON DELETE CASCADE,
	prep_station        TEXT NOT NULL,
	status              TEXT NOT NULL DEFAULT 'queued'
	                    CHECK (status IN ('queued', 'preparing', 'bumped', 'recalled')),
	bumped_at           TEXT,
	recalled_at         TEXT,
	created_at          TEXT NOT NULL,
	updated_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_kitchen_ticket_items (
	id                  TEXT PRIMARY KEY,
	kitchen_ticket_id   TEXT NOT NULL REFERENCES local_kitchen_tickets(id) ON DELETE CASCADE,
	order_item_id       TEXT NOT NULL REFERENCES local_order_items(id) ON DELETE CASCADE,
	item_name           TEXT NOT NULL,
	quantity            TEXT NOT NULL,
	status              TEXT NOT NULL DEFAULT 'queued'
);

CREATE TABLE IF NOT EXISTS local_cash_shifts (
	id                  TEXT PRIMARY KEY,
	opened_by           TEXT NOT NULL,
	closed_by           TEXT,
	device_id           TEXT NOT NULL DEFAULT '',
	opening_float       TEXT NOT NULL DEFAULT '0.0000',
	expected_cash       TEXT NOT NULL DEFAULT '0.0000',
	counted_cash        TEXT,
	variance            TEXT,
	denominations_json  TEXT NOT NULL DEFAULT '{}',
	status              TEXT NOT NULL DEFAULT 'open',
	opened_at           TEXT NOT NULL,
	closed_at           TEXT
);

CREATE TABLE IF NOT EXISTS local_cash_movements (
	id                  TEXT PRIMARY KEY,
	shift_id            TEXT NOT NULL REFERENCES local_cash_shifts(id) ON DELETE CASCADE,
	movement_type       TEXT NOT NULL CHECK (movement_type IN ('paid_in', 'paid_out', 'safe_drop')),
	amount              TEXT NOT NULL,
	reason              TEXT NOT NULL,
	approved_by         TEXT NOT NULL,
	created_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_audit_logs (
	id                  TEXT PRIMARY KEY,
	tenant_id           TEXT NOT NULL,
	branch_id           TEXT NOT NULL,
	actor_user_id       TEXT NOT NULL,
	approver_user_id    TEXT NOT NULL DEFAULT '',
	action_type         TEXT NOT NULL,
	reason_code         TEXT NOT NULL DEFAULT '',
	target_type         TEXT NOT NULL,
	target_id           TEXT NOT NULL,
	amount              TEXT NOT NULL DEFAULT '0.0000',
	metadata_json       TEXT NOT NULL DEFAULT '{}',
	created_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_payments (
	id                  TEXT PRIMARY KEY,
	order_id            TEXT NOT NULL REFERENCES local_orders(id) ON DELETE CASCADE,
	cash_shift_id       TEXT,
	check_number        INTEGER NOT NULL DEFAULT 1,
	method              TEXT NOT NULL,
	status              TEXT NOT NULL DEFAULT 'captured',
	amount              TEXT NOT NULL,
	tendered_amount     TEXT NOT NULL DEFAULT '0.0000',
	change_due          TEXT NOT NULL DEFAULT '0.0000',
	provider_ref        TEXT NOT NULL DEFAULT '',
	created_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_outbox (
	seq_id              INTEGER PRIMARY KEY AUTOINCREMENT,
	event_id            TEXT NOT NULL UNIQUE,
	command_type        TEXT NOT NULL,
	aggregate_type      TEXT NOT NULL,
	aggregate_id        TEXT NOT NULL,
	payload_json        TEXT NOT NULL,
	idempotency_key     TEXT NOT NULL UNIQUE,
	status              TEXT NOT NULL DEFAULT 'pending'
	                    CHECK (status IN ('pending', 'in_flight', 'acked', 'rejected')),
	retry_count         INTEGER NOT NULL DEFAULT 0,
	next_retry_at       TEXT,
	created_at          TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_local_outbox_status ON local_outbox (status, seq_id);

CREATE TABLE IF NOT EXISTS lan_events (
	seq_id              INTEGER PRIMARY KEY AUTOINCREMENT,
	event_type          TEXT NOT NULL,
	aggregate_id        TEXT NOT NULL,
	payload_json        TEXT NOT NULL,
	created_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS print_printers (
	id                  TEXT PRIMARY KEY,
	name                TEXT NOT NULL,
	station             TEXT NOT NULL,
	transport_type      TEXT NOT NULL
	                    CHECK (transport_type IN ('network_tcp', 'os_raw', 'virtual_mock')),
	address             TEXT NOT NULL,
	paper_width_mm      INTEGER NOT NULL DEFAULT 80 CHECK (paper_width_mm IN (58, 80)),
	is_active           INTEGER NOT NULL DEFAULT 1,
	created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS print_jobs (
	id                  TEXT PRIMARY KEY,
	idempotency_key     TEXT NOT NULL UNIQUE,
	printer_id          TEXT NOT NULL REFERENCES print_printers(id) ON DELETE CASCADE,
	job_type            TEXT NOT NULL
	                    CHECK (job_type IN ('customer_receipt', 'kitchen_ticket', 'cash_drawer_kick', 'test_print')),
	escpos_base64       TEXT NOT NULL,
	status              TEXT NOT NULL DEFAULT 'queued'
	                    CHECK (status IN ('queued', 'printing', 'printed', 'failed_retry', 'dead_letter')),
	attempts            INTEGER NOT NULL DEFAULT 0,
	max_attempts        INTEGER NOT NULL DEFAULT 5,
	last_error          TEXT NOT NULL DEFAULT '',
	next_retry_at       TEXT,
	created_at          TEXT NOT NULL,
	printed_at          TEXT
);

CREATE INDEX IF NOT EXISTS idx_print_jobs_status ON print_jobs (status, created_at);
`
	_, err := s.db.Exec(schema)
	return err
}

type HealthSnapshot struct {
	BranchID           string `json:"branch_id"`
	CloudReachable     bool   `json:"cloud_reachable"`
	PendingOutboxCount int    `json:"pending_outbox_count"`
	OldestPendingAt    string `json:"oldest_pending_at,omitempty"`
	LatestLANSeq       int64  `json:"latest_lan_seq"`
}

func (s *Store) HealthSnapshot(ctx context.Context) (*HealthSnapshot, error) {
	var snap HealthSnapshot
	var reachableInt int
	if err := s.db.QueryRowContext(ctx, `
		SELECT branch_id, cloud_reachable FROM branch_state WHERE id = 1
	`).Scan(&snap.BranchID, &reachableInt); err != nil {
		return nil, err
	}
	snap.CloudReachable = reachableInt == 1

	var oldest sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(created_at) FROM local_outbox WHERE status IN ('pending', 'in_flight')
	`).Scan(&snap.PendingOutboxCount, &oldest); err != nil {
		return nil, err
	}
	if oldest.Valid {
		snap.OldestPendingAt = oldest.String
	}

	var latestSeq sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(seq_id) FROM lan_events`).Scan(&latestSeq); err != nil {
		return nil, err
	}
	if latestSeq.Valid {
		snap.LatestLANSeq = latestSeq.Int64
	}
	return &snap, nil
}

func (s *Store) PendingQueueDepth() int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var count int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM local_outbox WHERE status IN ('pending', 'in_flight')`).Scan(&count)
	return count
}
