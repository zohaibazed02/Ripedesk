-- 000003_inventory_and_procurement.up.sql
-- Ingredients, unit conversions, versioned recipes (menu items & modifiers),
-- append-only stock ledger, kitchen requisitions, warehouse transfers, and procurement + OCR invoices.

-- 1. Ingredients & Unit Conversions
CREATE TABLE ingredients (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    sku                     TEXT NOT NULL,
    name                    TEXT NOT NULL,
    base_unit               TEXT NOT NULL CHECK (base_unit IN ('g', 'ml', 'pcs')),
    purchasing_unit         TEXT NOT NULL CHECK (purchasing_unit IN ('kg', 'L', 'box', 'pack', 'g', 'ml', 'pcs')),
    conversion_factor       NUMERIC(19,6) NOT NULL CHECK (conversion_factor > 0),
    track_lot_expiry        BOOLEAN NOT NULL DEFAULT FALSE,
    low_stock_threshold     NUMERIC(19,6) NOT NULL DEFAULT 0 CHECK (low_stock_threshold >= 0),
    valuation_cost          NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (valuation_cost >= 0),
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, sku)
);

CREATE TRIGGER trg_ingredients_updated_at
    BEFORE UPDATE ON ingredients
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE unit_conversions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    ingredient_id       UUID NOT NULL,
    from_unit           TEXT NOT NULL,
    to_unit             TEXT NOT NULL,
    multiplier          NUMERIC(19,6) NOT NULL CHECK (multiplier > 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, ingredient_id, from_unit, to_unit),
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE CASCADE
);

-- 2. Versioned Recipes (linked to menu_items AND modifiers)
CREATE TABLE recipe_versions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    menu_item_id        UUID,
    modifier_id         UUID,
    version_number      INTEGER NOT NULL DEFAULT 1 CHECK (version_number >= 1),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    yield_quantity      NUMERIC(19,6) NOT NULL DEFAULT 1 CHECK (yield_quantity > 0),
    effective_from      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    CHECK (
        (menu_item_id IS NOT NULL AND modifier_id IS NULL) OR
        (menu_item_id IS NULL AND modifier_id IS NOT NULL)
    ),
    FOREIGN KEY (tenant_id, menu_item_id) REFERENCES menu_items(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, modifier_id) REFERENCES modifiers(tenant_id, id) ON DELETE CASCADE
);

CREATE TRIGGER trg_recipe_versions_updated_at
    BEFORE UPDATE ON recipe_versions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE recipe_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    recipe_version_id   UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    quantity_base_unit  NUMERIC(19,6) NOT NULL CHECK (quantity_base_unit > 0),
    waste_factor_pct    NUMERIC(6,3) NOT NULL DEFAULT 0 CHECK (waste_factor_pct >= 0 AND waste_factor_pct < 100),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, recipe_version_id, ingredient_id),
    FOREIGN KEY (tenant_id, recipe_version_id) REFERENCES recipe_versions(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

-- 3. Stock Locations, Balances & Append-Only Stock Movements
CREATE TABLE stock_locations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    name                TEXT NOT NULL,
    location_type       TEXT NOT NULL
                        CHECK (location_type IN ('central_warehouse', 'branch_store', 'kitchen_line', 'barista_bar', 'cold_room')),
    is_default          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, branch_id, name),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT
);

CREATE TRIGGER trg_stock_locations_updated_at
    BEFORE UPDATE ON stock_locations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE stock_balances (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    stock_location_id   UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    on_hand_qty         NUMERIC(19,6) NOT NULL DEFAULT 0,
    reserved_qty        NUMERIC(19,6) NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),
    avg_unit_cost       NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (avg_unit_cost >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, stock_location_id, ingredient_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, stock_location_id) REFERENCES stock_locations(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_stock_balances_tenant_branch ON stock_balances (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_stock_balances_updated_at
    BEFORE UPDATE ON stock_balances
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE stock_movements (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    stock_location_id   UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    movement_type       TEXT NOT NULL
                        CHECK (movement_type IN (
                            'receipt', 'prep_consumption', 'waste', 'count_adjustment',
                            'transfer_out', 'transfer_in', 'production_in', 'production_out'
                        )),
    quantity_delta      NUMERIC(19,6) NOT NULL CHECK (quantity_delta <> 0),
    unit_cost           NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    source_doc_type     TEXT NOT NULL,
    source_doc_id       UUID NOT NULL,
    idempotency_key     TEXT NOT NULL,
    lot_number          TEXT,
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, idempotency_key),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, stock_location_id) REFERENCES stock_locations(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_stock_movements_tenant_branch_created ON stock_movements (tenant_id, branch_id, created_at DESC);

-- 4. Kitchen Requisitions
CREATE TABLE kitchen_requisitions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id               UUID NOT NULL,
    requested_by            UUID NOT NULL,
    approved_by             UUID,
    status                  TEXT NOT NULL DEFAULT 'draft'
                            CHECK (status IN ('draft', 'submitted', 'approved', 'partially_fulfilled', 'fulfilled', 'rejected')),
    needed_by               TIMESTAMPTZ NOT NULL,
    is_recurring_template   BOOLEAN NOT NULL DEFAULT FALSE,
    template_name           TEXT,
    notes                   TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, requested_by) REFERENCES users(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_kitchen_requisitions_tenant_branch_created ON kitchen_requisitions (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_kitchen_requisitions_updated_at
    BEFORE UPDATE ON kitchen_requisitions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE requisition_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    requisition_id      UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    requested_qty       NUMERIC(19,6) NOT NULL CHECK (requested_qty > 0),
    fulfilled_qty       NUMERIC(19,6) NOT NULL DEFAULT 0 CHECK (fulfilled_qty >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, requisition_id) REFERENCES kitchen_requisitions(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

-- 5. Warehouse Transfers
CREATE TABLE warehouse_transfers (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    from_branch_id          UUID NOT NULL,
    to_branch_id            UUID NOT NULL,
    from_location_id        UUID NOT NULL,
    to_location_id          UUID NOT NULL,
    requisition_id          UUID,
    status                  TEXT NOT NULL DEFAULT 'approved'
                            CHECK (status IN ('approved', 'reserved', 'dispatched', 'in_transit', 'received')),
    dispatched_at           TIMESTAMPTZ,
    received_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, from_branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, to_branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, from_location_id) REFERENCES stock_locations(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, to_location_id) REFERENCES stock_locations(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_warehouse_transfers_tenant_created ON warehouse_transfers (tenant_id, to_branch_id, created_at DESC);

CREATE TRIGGER trg_warehouse_transfers_updated_at
    BEFORE UPDATE ON warehouse_transfers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE transfer_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    transfer_id         UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    dispatched_qty      NUMERIC(19,6) NOT NULL CHECK (dispatched_qty > 0),
    received_qty        NUMERIC(19,6) NOT NULL DEFAULT 0 CHECK (received_qty >= 0),
    shortage_qty        NUMERIC(19,6) NOT NULL DEFAULT 0 CHECK (shortage_qty >= 0),
    damaged_qty         NUMERIC(19,6) NOT NULL DEFAULT 0 CHECK (damaged_qty >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, transfer_id) REFERENCES warehouse_transfers(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

-- 6. Procurement: Suppliers, POs, Goods Receipts & Vendor Invoices (with OCR)
CREATE TABLE suppliers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name                TEXT NOT NULL,
    tax_id              TEXT,
    contact_email       TEXT,
    contact_phone       TEXT,
    payment_terms_days  INTEGER NOT NULL DEFAULT 30 CHECK (payment_terms_days >= 0),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id)
);

CREATE TRIGGER trg_suppliers_updated_at
    BEFORE UPDATE ON suppliers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE supplier_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    supplier_id         UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    supplier_sku_alias  TEXT NOT NULL,
    moq_qty             NUMERIC(19,6) NOT NULL DEFAULT 1 CHECK (moq_qty > 0),
    pack_size_qty       NUMERIC(19,6) NOT NULL DEFAULT 1 CHECK (pack_size_qty > 0),
    lead_time_days      INTEGER NOT NULL DEFAULT 2 CHECK (lead_time_days >= 0),
    unit_price          NUMERIC(19,4) NOT NULL CHECK (unit_price >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, supplier_id, ingredient_id),
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE purchase_orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    supplier_id         UUID NOT NULL,
    po_number           TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft', 'issued', 'partially_received', 'received', 'cancelled')),
    total_amount        NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    expected_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, po_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_purchase_orders_tenant_branch_created ON purchase_orders (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_purchase_orders_updated_at
    BEFORE UPDATE ON purchase_orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE po_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    purchase_order_id   UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    ordered_qty         NUMERIC(19,6) NOT NULL CHECK (ordered_qty > 0),
    unit_price          NUMERIC(19,4) NOT NULL CHECK (unit_price >= 0),
    line_total          NUMERIC(19,4) NOT NULL CHECK (line_total >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, purchase_order_id) REFERENCES purchase_orders(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE goods_receipts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id           UUID NOT NULL,
    purchase_order_id   UUID,
    supplier_id         UUID NOT NULL,
    receipt_number      TEXT NOT NULL,
    received_by         UUID NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, receipt_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_goods_receipts_tenant_branch_created ON goods_receipts (tenant_id, branch_id, created_at DESC);

CREATE TABLE goods_receipt_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    goods_receipt_id    UUID NOT NULL,
    ingredient_id       UUID NOT NULL,
    received_qty        NUMERIC(19,6) NOT NULL CHECK (received_qty > 0),
    unit_cost           NUMERIC(19,4) NOT NULL CHECK (unit_cost >= 0),
    lot_number          TEXT,
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, goods_receipt_id) REFERENCES goods_receipts(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_id) REFERENCES ingredients(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE vendor_invoices (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                   UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id                   UUID NOT NULL,
    supplier_id                 UUID NOT NULL,
    purchase_order_id           UUID,
    invoice_number              TEXT NOT NULL,
    normalized_invoice_number   TEXT NOT NULL,
    status                      TEXT NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft', 'pending_review', 'approved', 'posted', 'paid', 'rejected')),
    invoice_date                DATE NOT NULL,
    due_date                    DATE,
    subtotal                    NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    tax_total                   NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (tax_total >= 0),
    grand_total                 NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (grand_total >= 0),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, supplier_id, normalized_invoice_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_vendor_invoices_tenant_branch_created ON vendor_invoices (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_vendor_invoices_updated_at
    BEFORE UPDATE ON vendor_invoices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE vendor_invoice_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    vendor_invoice_id   UUID NOT NULL,
    ingredient_id       UUID,
    description         TEXT NOT NULL,
    quantity            NUMERIC(19,6) NOT NULL CHECK (quantity > 0),
    unit_price          NUMERIC(19,4) NOT NULL CHECK (unit_price >= 0),
    line_total          NUMERIC(19,4) NOT NULL CHECK (line_total >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, vendor_invoice_id) REFERENCES vendor_invoices(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE vendor_invoice_documents (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                   UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    vendor_invoice_id           UUID NOT NULL,
    storage_object_key          TEXT NOT NULL,
    sha256_fingerprint          CHAR(64) NOT NULL,
    ocr_confidence_json         JSONB NOT NULL DEFAULT '{}'::jsonb,
    extraction_model_version    TEXT NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, sha256_fingerprint),
    FOREIGN KEY (tenant_id, vendor_invoice_id) REFERENCES vendor_invoices(tenant_id, id) ON DELETE CASCADE
);
