-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 007: primary_key_ref + Row Level Security
-- Adds primary_key_id to identities (requires identity_keys to exist first)
-- Enables RLS on all tables for tenant isolation enforcement
-- ─────────────────────────────────────────────────────────────────────────────

-- ── Add primary key reference to identities ───────────────────────────────────
-- Deferred until now because identity_keys didn't exist in migration 002

ALTER TABLE identities
    ADD COLUMN primary_key_id UUID NULL REFERENCES identity_keys(id) ON DELETE SET NULL;

CREATE INDEX identities_primary_key_id_idx ON identities (primary_key_id);

COMMENT ON COLUMN identities.primary_key_id IS 'Reference to the primary signing key for this identity. Set after key creation.';

-- ── Row Level Security ────────────────────────────────────────────────────────
-- Enforces tenant isolation at the database level.
-- Application must set: SET app.current_tenant_id = '<uuid>';
-- This ensures no query can ever read another tenant's data,
-- even if the application layer has a bug.

-- tenants: users can only see their own tenant
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenants_isolation ON tenants
    USING (id = current_setting('app.current_tenant_id', true)::UUID);

-- identities
ALTER TABLE identities ENABLE ROW LEVEL SECURITY;

CREATE POLICY identities_isolation ON identities
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- identity_keys
ALTER TABLE identity_keys ENABLE ROW LEVEL SECURITY;

CREATE POLICY identity_keys_isolation ON identity_keys
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- credentials
ALTER TABLE credentials ENABLE ROW LEVEL SECURITY;

CREATE POLICY credentials_isolation ON credentials
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- recovery_methods
ALTER TABLE recovery_methods ENABLE ROW LEVEL SECURITY;

CREATE POLICY recovery_methods_isolation ON recovery_methods
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- audit_events
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY audit_events_isolation ON audit_events
    USING (tenant_id = current_setting('app.current_tenant_id', true)::UUID);

-- ── Superuser bypass for migrations and admin operations ──────────────────────
-- The verion application user must NOT be superuser in production.
-- Migrations run as superuser and bypass RLS automatically.

COMMENT ON TABLE identities IS 'Universal identity entity. RLS enforced — app must SET app.current_tenant_id before any query.';
