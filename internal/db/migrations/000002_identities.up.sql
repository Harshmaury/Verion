-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 002: identities
-- Universal identity entity supporting all identity types
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TYPE identity_type AS ENUM (
    'human',
    'org',
    'device',
    'service',
    'machine',
    'ai_agent'
);

CREATE TYPE identity_status AS ENUM (
    'pending',
    'active',
    'suspended',
    'deactivated',
    'archived'
);

CREATE TABLE identities (
    id              UUID            PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Tenant isolation (enforced at every query)
    tenant_id       UUID            NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,

    -- Type discriminator
    type            identity_type   NOT NULL,

    -- Display
    display_name    TEXT            NOT NULL,
    handle          CITEXT          NOT NULL,

    -- Status lifecycle
    status          identity_status NOT NULL DEFAULT 'pending',

    -- Flexible encrypted attributes (type-specific data)
    -- Encrypted with AES-256-GCM by internal/crypto service
    attributes      BYTEA           NOT NULL DEFAULT '\x',
    attributes_iv   BYTEA           NOT NULL DEFAULT '\x',

    -- Audit trail
    created_by      UUID            NULL,     -- FK set after table exists
    version         BIGINT          NOT NULL DEFAULT 1,

    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    deactivated_at  TIMESTAMPTZ     NULL
);

-- Handle unique within a tenant
CREATE UNIQUE INDEX identities_tenant_handle_unique ON identities (tenant_id, handle);

-- Fast lookup patterns
CREATE INDEX identities_tenant_id_idx    ON identities (tenant_id);
CREATE INDEX identities_type_idx         ON identities (type);
CREATE INDEX identities_status_idx       ON identities (status);
CREATE INDEX identities_tenant_status_idx ON identities (tenant_id, status);

-- Self-referential FK for created_by (identity that created this identity)
ALTER TABLE identities
    ADD CONSTRAINT identities_created_by_fk
    FOREIGN KEY (created_by) REFERENCES identities(id) ON DELETE SET NULL;

CREATE TRIGGER identities_updated_at
    BEFORE UPDATE ON identities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE identities IS 'Universal identity entity. Supports human, org, device, service, machine, and ai_agent types.';
COMMENT ON COLUMN identities.attributes IS 'AES-256-GCM encrypted JSONB payload containing type-specific identity attributes.';
COMMENT ON COLUMN identities.version IS 'Optimistic locking version counter. Increment on every update.';
