-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 004: credentials
-- Authentication mechanisms bound to identities
-- Each credential type references a key or stores encrypted credential data
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TYPE credential_type AS ENUM (
    'passkey',          -- FIDO2 passkey (WebAuthn)
    'totp',             -- Time-based OTP
    'hardware_token',   -- FIDO2 hardware security key
    'recovery_code',    -- Single-use recovery codes
    'api_key',          -- Service-to-service API key
    'mtls_cert',        -- Mutual TLS client certificate
    'biometric'         -- Platform biometric reference
);

CREATE TYPE credential_status AS ENUM (
    'active',
    'revoked',
    'expired'
);

CREATE TABLE credentials (
    id              UUID                PRIMARY KEY DEFAULT uuid_generate_v4(),
    identity_id     UUID                NOT NULL REFERENCES identities(id) ON DELETE RESTRICT,
    tenant_id       UUID                NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,

    -- Optional key binding (not all credentials are key-backed)
    key_id          UUID                NULL REFERENCES identity_keys(id) ON DELETE SET NULL,

    type            credential_type     NOT NULL,

    -- Encrypted credential payload (AES-256-GCM)
    data            BYTEA               NOT NULL,
    data_iv         BYTEA               NOT NULL,

    -- WebAuthn / FIDO2 specific fields
    aaguid          UUID                NULL,   -- Authenticator AAGUID
    credential_id   BYTEA               NULL,   -- WebAuthn credential ID (from authenticator)
    sign_count      BIGINT              NOT NULL DEFAULT 0,

    -- Lifecycle
    status          credential_status   NOT NULL DEFAULT 'active',
    last_used_at    TIMESTAMPTZ         NULL,

    -- User-facing metadata
    name            TEXT                NULL,   -- e.g. "iPhone 15 Face ID"
    device_info     JSONB               NULL,   -- Platform, OS, authenticator info

    created_at      TIMESTAMPTZ         NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ         NULL
);

-- WebAuthn credential_id must be unique per tenant (replay protection)
CREATE UNIQUE INDEX credentials_webauthn_unique
    ON credentials (tenant_id, credential_id)
    WHERE credential_id IS NOT NULL;

-- Fast lookups
CREATE INDEX credentials_identity_id_idx ON credentials (identity_id);
CREATE INDEX credentials_tenant_id_idx   ON credentials (tenant_id);
CREATE INDEX credentials_type_idx        ON credentials (type);
CREATE INDEX credentials_status_idx      ON credentials (status);

COMMENT ON TABLE credentials IS 'Authentication mechanisms (passkeys, TOTP, hardware tokens, API keys, mTLS certs) bound to identities.';
COMMENT ON COLUMN credentials.data IS 'AES-256-GCM encrypted credential payload. Content varies by credential type.';
COMMENT ON COLUMN credentials.credential_id IS 'WebAuthn credential ID returned by authenticator during registration. Used for assertion lookup.';
COMMENT ON COLUMN credentials.sign_count IS 'WebAuthn signature counter. Must increase monotonically; rollback indicates cloned authenticator.';
