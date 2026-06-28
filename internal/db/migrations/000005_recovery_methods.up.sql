-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 005: recovery_methods
-- Identity recovery mechanisms with equivalent security to primary auth
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TYPE recovery_type AS ENUM (
    'recovery_codes',   -- Set of single-use codes (Argon2id hashed)
    'backup_key',       -- Cryptographic backup key pair
    'trusted_contact',  -- Social recovery via trusted identities
    'hardware_backup'   -- Secondary hardware security key
);

CREATE TYPE recovery_status AS ENUM (
    'active',
    'consumed',
    'revoked',
    'expired'
);

CREATE TABLE recovery_methods (
    id              UUID            PRIMARY KEY DEFAULT uuid_generate_v4(),
    identity_id     UUID            NOT NULL REFERENCES identities(id) ON DELETE RESTRICT,
    tenant_id       UUID            NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,

    type            recovery_type   NOT NULL,

    -- Encrypted recovery data (AES-256-GCM, Argon2id hashed where applicable)
    data            BYTEA           NOT NULL,
    data_iv         BYTEA           NOT NULL,

    -- Usage control
    status          recovery_status NOT NULL DEFAULT 'active',
    used_at         TIMESTAMPTZ     NULL,
    is_consumed     BOOLEAN         NOT NULL DEFAULT false, -- For one-time-use methods

    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ     NULL
);

-- Fast lookups
CREATE INDEX recovery_methods_identity_id_idx ON recovery_methods (identity_id);
CREATE INDEX recovery_methods_tenant_id_idx   ON recovery_methods (tenant_id);
CREATE INDEX recovery_methods_status_idx      ON recovery_methods (status);

-- Only one active backup_key per identity
CREATE UNIQUE INDEX recovery_methods_one_backup_key
    ON recovery_methods (identity_id)
    WHERE type = 'backup_key' AND status = 'active';

COMMENT ON TABLE recovery_methods IS 'Identity recovery mechanisms. Must provide equivalent security guarantees to primary authentication.';
COMMENT ON COLUMN recovery_methods.is_consumed IS 'True for single-use recovery codes after use. Never reusable once consumed.';
