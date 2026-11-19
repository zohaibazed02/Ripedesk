-- 000001_tenant_and_rbac.up.sql
-- Multi-tenant core, branches, enrolled devices, and RBAC with offline PIN support.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Shared updated_at trigger function
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 1. Tenants
CREATE TABLE tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    currency_code   CHAR(3) NOT NULL DEFAULT 'PKR',
    timezone        TEXT NOT NULL DEFAULT 'Asia/Karachi',
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'suspended', 'trial', 'archived')),
    entitlements    JSONB NOT NULL DEFAULT '{
        "kds": true,
        "warehouse": true,
        "ocr_bills": true,
        "rider_tracking": true,
        "ai_agent": true
    }'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 2. Branches
CREATE TABLE branches (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name                    TEXT NOT NULL,
    code                    TEXT NOT NULL,
    branch_type             TEXT NOT NULL
                            CHECK (branch_type IN ('coffee_shop', 'dine_in', 'qsr', 'central_warehouse')),
    default_order_mode      TEXT NOT NULL DEFAULT 'quick_sale'
                            CHECK (default_order_mode IN ('quick_sale', 'takeaway', 'dine_in', 'delivery', 'qr_order', 'drive_thru')),
    heartbeat_expires_at    TIMESTAMPTZ,
    last_sync_at            TIMESTAMPTZ,
    pending_sync_count      INTEGER NOT NULL DEFAULT 0 CHECK (pending_sync_count >= 0),
    oldest_pending_tx_at    TIMESTAMPTZ,
    config                  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, id)
);

CREATE INDEX idx_branches_tenant_created ON branches (tenant_id, created_at DESC);

CREATE TRIGGER trg_branches_updated_at
    BEFORE UPDATE ON branches
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 3. Devices
CREATE TABLE devices (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    branch_id               UUID NOT NULL,
    device_name             TEXT NOT NULL,
    device_type             TEXT NOT NULL
                            CHECK (device_type IN ('windows_pos', 'android_staff', 'kds_browser', 'branch_server')),
    hardware_fingerprint    TEXT NOT NULL,
    public_key_hash         TEXT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'enrolled'
                            CHECK (status IN ('enrolled', 'revoked')),
    isolated_mode_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    last_seen_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, hardware_fingerprint),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_devices_tenant_branch_created ON devices (tenant_id, branch_id, created_at DESC);

CREATE TRIGGER trg_devices_updated_at
    BEFORE UPDATE ON devices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 4. Roles
CREATE TABLE roles (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code                TEXT NOT NULL
                        CHECK (code IN ('owner', 'branch_manager', 'cashier', 'waiter', 'kitchen_lead', 'warehouse_clerk', 'accountant')),
    display_name        TEXT NOT NULL,
    permissions         JSONB NOT NULL DEFAULT '[]'::jsonb,
    can_override        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, id)
);

CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 5. Users
CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    role_id             UUID NOT NULL,
    full_name           TEXT NOT NULL,
    email               TEXT NOT NULL,
    phone               TEXT,
    password_hash       TEXT NOT NULL,
    pin_hash            TEXT NOT NULL,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, email),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_users_tenant_created ON users (tenant_id, created_at DESC);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 6. User Branch Assignments
CREATE TABLE user_branch_assignments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    user_id             UUID NOT NULL,
    branch_id           UUID NOT NULL,
    is_primary          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, user_id, branch_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_user_branch_assignments_lookup ON user_branch_assignments (tenant_id, branch_id, user_id);

CREATE TRIGGER trg_user_branch_assignments_updated_at
    BEFORE UPDATE ON user_branch_assignments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
