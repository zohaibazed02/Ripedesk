-- 000002_pos_and_orders.up.sql
-- Menu catalog, floor plans, omnichannel orders, rounds, KDS tickets, payments, and cash shifts.
-- All monetary columns use NUMERIC(19,4); all quantity columns use NUMERIC(19,6).

-- 1. Menu Categories
CREATE TABLE categories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name            TEXT NOT NULL,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name)
);

CREATE TRIGGER trg_categories_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 2. Menu Items
CREATE TABLE menu_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID,
    category_id         UUID NOT NULL,
    sku                 TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    base_price          NUMERIC(19,4) NOT NULL CHECK (base_price >= 0),
    tax_rate_percent    NUMERIC(6,3) NOT NULL DEFAULT 0 CHECK (tax_rate_percent >= 0 AND tax_rate_percent <= 100),
    prep_station        TEXT NOT NULL DEFAULT 'hot_kitchen'
                        CHECK (prep_station IN ('barista', 'hot_kitchen', 'cold_prep', 'pastry', 'expediter')),
    is_available        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, sku),
    FOREIGN KEY (tenant_id, category_id) REFERENCES categories(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_menu_items_tenant_branch ON menu_items (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_menu_items_updated_at
    BEFORE UPDATE ON menu_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 3. Modifier Groups & Modifiers
CREATE TABLE modifier_groups (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name                TEXT NOT NULL,
    min_selections      INTEGER NOT NULL DEFAULT 0 CHECK (min_selections >= 0),
    max_selections      INTEGER NOT NULL DEFAULT 1 CHECK (max_selections >= min_selections),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id)
);

CREATE TRIGGER trg_modifier_groups_updated_at
    BEFORE UPDATE ON modifier_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE modifiers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    modifier_group_id   UUID NOT NULL,
    name                TEXT NOT NULL,
    price_delta         NUMERIC(19,4) NOT NULL DEFAULT 0,
    is_available        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, modifier_group_id) REFERENCES modifier_groups(tenant_id, id) ON DELETE CASCADE
);

CREATE TRIGGER trg_modifiers_updated_at
    BEFORE UPDATE ON modifiers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE item_modifier_groups (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    menu_item_id        UUID NOT NULL,
    modifier_group_id   UUID NOT NULL,
    sort_order          INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, menu_item_id, modifier_group_id),
    FOREIGN KEY (tenant_id, menu_item_id) REFERENCES menu_items(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, modifier_group_id) REFERENCES modifier_groups(tenant_id, id) ON DELETE CASCADE
);

-- 4. Floors & Dining Tables
CREATE TABLE floors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id       UUID NOT NULL,
    name            TEXT NOT NULL,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE CASCADE
);

CREATE TRIGGER trg_floors_updated_at
    BEFORE UPDATE ON floors
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE dining_tables (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id       UUID NOT NULL,
    floor_id        UUID NOT NULL,
    table_code      TEXT NOT NULL,
    capacity        INTEGER NOT NULL CHECK (capacity > 0),
    pos_x           INTEGER NOT NULL DEFAULT 0,
    pos_y           INTEGER NOT NULL DEFAULT 0,
    width           INTEGER NOT NULL DEFAULT 80,
    height          INTEGER NOT NULL DEFAULT 80,
    shape           TEXT NOT NULL DEFAULT 'rectangle'
                    CHECK (shape IN ('rectangle', 'circle', 'square', 'booth')),
    status          TEXT NOT NULL DEFAULT 'available'
                    CHECK (status IN ('available', 'occupied', 'reserved', 'dirty')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, branch_id, table_code),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, floor_id) REFERENCES floors(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_dining_tables_tenant_branch ON dining_tables (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_dining_tables_updated_at
    BEFORE UPDATE ON dining_tables
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 5. Cash Shifts
CREATE TABLE cash_shifts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    device_id           UUID,
    opened_by           UUID NOT NULL,
    closed_by           UUID,
    opened_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at           TIMESTAMPTZ,
    opening_float       NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (opening_float >= 0),
    expected_cash       NUMERIC(19,4) NOT NULL DEFAULT 0,
    counted_cash        NUMERIC(19,4),
    variance            NUMERIC(19,4),
    status              TEXT NOT NULL DEFAULT 'open'
                        CHECK (status IN ('open', 'closed', 'audited')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, opened_by) REFERENCES users(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_cash_shifts_tenant_branch_created ON cash_shifts (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_cash_shifts_updated_at
    BEFORE UPDATE ON cash_shifts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 6. Orders (client-generated stable UUID for offline idempotency)
CREATE TABLE orders (
    id                  UUID PRIMARY KEY,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    order_number        TEXT NOT NULL,
    order_mode          TEXT NOT NULL
                        CHECK (order_mode IN ('quick_sale', 'takeaway', 'dine_in', 'delivery', 'qr_order', 'drive_thru')),
    table_id            UUID,
    order_version       INTEGER NOT NULL DEFAULT 1 CHECK (order_version >= 1),
    status              TEXT NOT NULL DEFAULT 'open'
                        CHECK (status IN ('open', 'in_prep', 'ready', 'completed', 'voided', 'refunded')),
    subtotal            NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    tax_total           NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (tax_total >= 0),
    discount_total      NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (discount_total >= 0),
    grand_total         NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (grand_total >= 0),
    sync_status         TEXT NOT NULL DEFAULT 'synced'
                        CHECK (sync_status IN ('synced', 'pending_sync', 'conflict')),
    device_id           UUID,
    created_offline_at  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, branch_id, order_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, table_id) REFERENCES dining_tables(tenant_id, id) ON DELETE SET NULL
);

CREATE INDEX idx_orders_tenant_branch_created ON orders (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 7. Order Rounds, Order Items & Modifiers
CREATE TABLE order_rounds (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    order_id            UUID NOT NULL,
    round_number        INTEGER NOT NULL CHECK (round_number >= 1),
    submitted_by        UUID,
    prep_confirmed_at   TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, order_id, round_number),
    FOREIGN KEY (tenant_id, order_id) REFERENCES orders(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_order_rounds_tenant_branch_created ON order_rounds (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_order_rounds_updated_at
    BEFORE UPDATE ON order_rounds
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE order_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    order_id            UUID NOT NULL,
    order_round_id      UUID NOT NULL,
    menu_item_id        UUID NOT NULL,
    item_name           TEXT NOT NULL,
    quantity            NUMERIC(19,6) NOT NULL CHECK (quantity > 0),
    unit_price          NUMERIC(19,4) NOT NULL CHECK (unit_price >= 0),
    line_total          NUMERIC(19,4) NOT NULL CHECK (line_total >= 0),
    seat_number         INTEGER,
    check_number        INTEGER NOT NULL DEFAULT 1 CHECK (check_number >= 1),
    prep_confirmed_at   TIMESTAMPTZ,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, order_id) REFERENCES orders(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, order_round_id) REFERENCES order_rounds(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, menu_item_id) REFERENCES menu_items(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_order_items_tenant_branch_created ON order_items (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_order_items_updated_at
    BEFORE UPDATE ON order_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE order_item_modifiers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_item_id       UUID NOT NULL,
    modifier_id         UUID NOT NULL,
    modifier_name       TEXT NOT NULL,
    quantity            NUMERIC(19,6) NOT NULL DEFAULT 1 CHECK (quantity > 0),
    price_delta         NUMERIC(19,4) NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, order_item_id) REFERENCES order_items(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, modifier_id) REFERENCES modifiers(tenant_id, id) ON DELETE RESTRICT
);

-- 8. Kitchen Tickets & Ticket Items
CREATE TABLE kitchen_tickets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    order_id            UUID NOT NULL,
    order_round_id      UUID NOT NULL,
    prep_station        TEXT NOT NULL
                        CHECK (prep_station IN ('barista', 'hot_kitchen', 'cold_prep', 'pastry', 'expediter')),
    status              TEXT NOT NULL DEFAULT 'queued'
                        CHECK (status IN ('queued', 'preparing', 'bumped', 'recalled')),
    bumped_at           TIMESTAMPTZ,
    recalled_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, order_id) REFERENCES orders(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, order_round_id) REFERENCES order_rounds(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_kitchen_tickets_tenant_branch_created ON kitchen_tickets (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_kitchen_tickets_updated_at
    BEFORE UPDATE ON kitchen_tickets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE kitchen_ticket_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    kitchen_ticket_id   UUID NOT NULL,
    order_item_id       UUID NOT NULL,
    quantity            NUMERIC(19,6) NOT NULL CHECK (quantity > 0),
    status              TEXT NOT NULL DEFAULT 'queued'
                        CHECK (status IN ('queued', 'preparing', 'bumped', 'recalled')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, kitchen_ticket_id) REFERENCES kitchen_tickets(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, order_item_id) REFERENCES order_items(tenant_id, id) ON DELETE CASCADE
);

-- 9. Payments (client-generated stable UUID for offline idempotency)
CREATE TABLE payments (
    id                  UUID PRIMARY KEY,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    order_id            UUID NOT NULL,
    cash_shift_id       UUID,
    check_number        INTEGER NOT NULL DEFAULT 1 CHECK (check_number >= 1),
    method              TEXT NOT NULL
                        CHECK (method IN ('cash', 'card_terminal', 'online_gateway', 'cod')),
    status              TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'authorized', 'captured', 'ambiguous', 'reversed', 'refunded')),
    amount              NUMERIC(19,4) NOT NULL CHECK (amount >= 0),
    tendered_amount     NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (tendered_amount >= 0),
    change_due          NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (change_due >= 0),
    provider_ref        TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, order_id) REFERENCES orders(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, cash_shift_id) REFERENCES cash_shifts(tenant_id, id) ON DELETE SET NULL
);

CREATE INDEX idx_payments_tenant_branch_created ON payments (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
