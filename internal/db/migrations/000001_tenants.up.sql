-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 001: tenants
-- Creates the top-level tenant table for multi-tenant isolation
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TYPE tenant_tier AS ENUM (
    'standard',
    'professional',
    'enterprise'
);

CREATE TYPE tenant_status AS ENUM (
    'active',
    'suspended',
    'deactivated'
);

CREATE TABLE tenants (
    id          UUID            PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT            NOT NULL,
    slug        CITEXT          NOT NULL,
    tier        tenant_tier     NOT NULL DEFAULT 'standard',
    status      tenant_status   NOT NULL DEFAULT 'active',
    settings    JSONB           NOT NULL DEFAULT '{}',
    data_region TEXT            NOT NULL DEFAULT 'global',

    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tenants_slug_unique ON tenants (slug);
CREATE INDEX tenants_status_idx ON tenants (status);

-- Shared trigger function for updated_at (used by all tables)
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE tenants IS 'Top-level tenant entities. Every identity belongs to exactly one tenant.';
