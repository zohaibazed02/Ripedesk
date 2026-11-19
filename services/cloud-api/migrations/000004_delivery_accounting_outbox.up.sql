-- 000004_delivery_accounting_outbox.up.sql
-- Delivery trips & rider GPS, HR rosters & attendance, balanced double-entry accounting,
-- transactional outbox, idempotency, sync review queue, and Row-Level Security (RLS).

-- 1. Delivery Trips & Rider GPS Pings
CREATE TABLE delivery_trips (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id               UUID NOT NULL,
    order_id                UUID NOT NULL,
    rider_user_id           UUID,
    status                  TEXT NOT NULL DEFAULT 'assigned'
                            CHECK (status IN ('assigned', 'picked_up', 'in_transit', 'delivered', 'failed', 'cancelled')),
    cod_expected_amount     NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (cod_expected_amount >= 0),
    cod_collected_amount    NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (cod_collected_amount >= 0),
    pod_photo_url           TEXT,
    tracking_token          TEXT NOT NULL UNIQUE,
    tracking_expires_at     TIMESTAMPTZ NOT NULL,
    delivered_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, order_id) REFERENCES orders(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_delivery_trips_tenant_branch_created ON delivery_trips (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_delivery_trips_updated_at
    BEFORE UPDATE ON delivery_trips
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE rider_gps_pings (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    delivery_trip_id    UUID NOT NULL,
    latitude            NUMERIC(10,7) NOT NULL,
    longitude           NUMERIC(10,7) NOT NULL,
    accuracy_meters     NUMERIC(8,2),
    recorded_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, delivery_trip_id) REFERENCES delivery_trips(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_rider_gps_pings_trip_time ON rider_gps_pings (tenant_id, delivery_trip_id, recorded_at DESC);

-- 2. HR: Staff Rosters & Attendance Logs
CREATE TABLE staff_rosters (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    user_id             UUID NOT NULL,
    shift_start         TIMESTAMPTZ NOT NULL,
    shift_end           TIMESTAMPTZ NOT NULL,
    CHECK (shift_end > shift_start),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_staff_rosters_tenant_branch_created ON staff_rosters (tenant_id, branch_id, created_at DESC);

CREATE TABLE attendance_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    user_id             UUID NOT NULL,
    clock_in_at         TIMESTAMPTZ NOT NULL,
    clock_out_at        TIMESTAMPTZ,
    approved_hours      NUMERIC(8,2),
    exception_flag      TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_attendance_logs_tenant_branch_created ON attendance_logs (tenant_id, branch_id, created_at DESC);

-- 3. Double-Entry Accounting (Balanced SUM(debit) == SUM(credit))
CREATE TABLE chart_of_accounts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    account_code        TEXT NOT NULL,
    name                TEXT NOT NULL,
    account_type        TEXT NOT NULL
                        CHECK (account_type IN ('asset', 'liability', 'equity', 'revenue', 'expense')),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, account_code)
);

CREATE TABLE journal_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID,
    entry_number        TEXT NOT NULL,
    reference_type      TEXT NOT NULL,
    reference_id        UUID,
    entry_date          DATE NOT NULL DEFAULT CURRENT_DATE,
    memo                TEXT,
    status              TEXT NOT NULL DEFAULT 'posted'
                        CHECK (status IN ('draft', 'posted', 'reversed')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, entry_number)
);

CREATE INDEX idx_journal_entries_tenant_branch_created ON journal_entries (tenant_id, branch_id, created_at DESC);

CREATE TABLE journal_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    journal_entry_id    UUID NOT NULL,
    account_id          UUID NOT NULL,
    debit               NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit              NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (credit >= 0),
    CHECK ((debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0)),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, account_id) REFERENCES chart_of_accounts(tenant_id, id) ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION enforce_balanced_journal_entry()
RETURNS TRIGGER AS $$
DECLARE
    v_entry_id UUID;
    v_debit    NUMERIC(19,4);
    v_credit   NUMERIC(19,4);
BEGIN
    v_entry_id := COALESCE(NEW.journal_entry_id, OLD.journal_entry_id);
    SELECT COALESCE(SUM(debit), 0), COALESCE(SUM(credit), 0)
    INTO v_debit, v_credit
    FROM journal_lines
    WHERE journal_entry_id = v_entry_id;

    IF v_debit <> v_credit THEN
        RAISE EXCEPTION 'Journal entry % is unbalanced: debit=% credit=%', v_entry_id, v_debit, v_credit;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_journal_lines_balanced
    AFTER INSERT OR UPDATE OR DELETE ON journal_lines
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION enforce_balanced_journal_entry();

-- 4. Transactional Outbox, Idempotency & Offline Sync Review Queue
CREATE TABLE outbox_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID,
    aggregate_type      TEXT NOT NULL,
    aggregate_id        UUID NOT NULL,
    event_type          TEXT NOT NULL,
    payload             JSONB NOT NULL,
    published_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_events_unpublished ON outbox_events (tenant_id, created_at) WHERE published_at IS NULL;

CREATE TABLE processed_commands (
    command_id          UUID PRIMARY KEY,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    command_type        TEXT NOT NULL,
    result_status       TEXT NOT NULL,
    processed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_processed_commands_tenant_branch ON processed_commands (tenant_id, branch_id, processed_at DESC);

CREATE TABLE sync_review_queue (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    command_id          UUID NOT NULL,
    command_type        TEXT NOT NULL,
    payload             JSONB NOT NULL,
    conflict_reason     TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending_review'
                        CHECK (status IN ('pending_review', 'resolved', 'discarded')),
    resolved_by         UUID,
    resolved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sync_review_queue_tenant_branch_created ON sync_review_queue (tenant_id, branch_id, created_at DESC);

-- 5. Non-Superuser Application Role & Row-Level Security (RLS) Defense-in-Depth
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pos_app_user') THEN
        CREATE ROLE pos_app_user LOGIN PASSWORD 'pos_app_user_dev_secret' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO pos_app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO pos_app_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO pos_app_user;

-- Enable & Force RLS across all tenant-scoped tables
DO $$
DECLARE
    tbl TEXT;
    tenant_tables TEXT[] := ARRAY[
        'branches', 'devices', 'roles', 'users', 'user_branch_assignments',
        'categories', 'menu_items', 'modifier_groups', 'modifiers', 'item_modifier_groups',
        'floors', 'dining_tables', 'cash_shifts', 'orders', 'order_rounds', 'order_items',
        'order_item_modifiers', 'kitchen_tickets', 'kitchen_ticket_items', 'payments',
        'ingredients', 'unit_conversions', 'recipe_versions', 'recipe_lines',
        'stock_locations', 'stock_balances', 'stock_movements',
        'kitchen_requisitions', 'requisition_lines', 'warehouse_transfers', 'transfer_lines',
        'suppliers', 'supplier_items', 'purchase_orders', 'po_lines',
        'goods_receipts', 'goods_receipt_lines', 'vendor_invoices', 'vendor_invoice_lines',
        'vendor_invoice_documents', 'delivery_trips', 'rider_gps_pings',
        'staff_rosters', 'attendance_logs', 'chart_of_accounts', 'journal_entries',
        'journal_lines', 'outbox_events', 'processed_commands', 'sync_review_queue'
    ];
BEGIN
    FOREACH tbl IN ARRAY tenant_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', tbl);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_policy ON %I', tbl);
        EXECUTE format(
            'CREATE POLICY tenant_isolation_policy ON %I USING (tenant_id = current_setting(''app.current_tenant_id'', true)::uuid) WITH CHECK (tenant_id = current_setting(''app.current_tenant_id'', true)::uuid)',
            tbl
        );
    END LOOP;
END
$$;
