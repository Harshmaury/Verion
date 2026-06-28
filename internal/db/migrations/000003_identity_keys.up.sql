-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 003: identity_keys
-- Cryptographic key pairs bound to identities
-- Private keys are NEVER stored here — only public keys and key references
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TYPE key_type AS ENUM (
    'ed25519',
    'ecdsa_p256',
    'ecdsa_p384',
    'rsa_4096'
);

CREATE TYPE key_purpose AS ENUM (
    'signing',
    'encryption',
    'authentication',
    'recovery'
);

CREATE TYPE key_status AS ENUM (
    'active',
    'rotated',
    'revoked',
    'compromised'
);

CREATE TABLE identity_keys (
    id              UUID            PRIMARY KEY DEFAULT uuid_generate_v4(),
    identity_id     UUID            NOT NULL REFERENCES identities(id) ON DELETE RESTRICT,
    tenant_id       UUID            NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,

    -- Key metadata
    key_type        key_type        NOT NULL,
    purpose         key_purpose     NOT NULL,
    algorithm       TEXT            NOT NULL, -- e.g. 'EdDSA', 'ES256', 'RS256'

    -- Public key material (safe to store)
    public_key      BYTEA           NOT NULL, -- DER-encoded public key
    public_key_jwk  JSONB           NOT NULL, -- JWK representation

    -- Private key reference (NEVER the private key itself)
    -- Points to secure external storage: vault path, HSM slot, KMS key ID
    key_ref         TEXT            NOT NULL,

    -- Key fingerprint for fast lookup (SHA-256 of public key, hex-encoded)
    fingerprint     TEXT            NOT NULL,

    -- Lifecycle
    status          key_status      NOT NULL DEFAULT 'active',
    valid_from      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    valid_until     TIMESTAMPTZ     NULL,     -- NULL = no expiry

    -- Rotation tracking
    rotated_at      TIMESTAMPTZ     NULL,
    rotated_to      UUID            NULL REFERENCES identity_keys(id) ON DELETE SET NULL,

    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- An identity can have only one active key per purpose
CREATE UNIQUE INDEX identity_keys_active_purpose_unique
    ON identity_keys (identity_id, purpose)
    WHERE status = 'active';

-- Fast lookups
CREATE INDEX identity_keys_identity_id_idx  ON identity_keys (identity_id);
CREATE INDEX identity_keys_tenant_id_idx    ON identity_keys (tenant_id);
CREATE INDEX identity_keys_fingerprint_idx  ON identity_keys (fingerprint);
CREATE INDEX identity_keys_status_idx       ON identity_keys (status);

COMMENT ON TABLE identity_keys IS 'Cryptographic key pairs bound to identities. Only public keys stored; private keys referenced externally.';
COMMENT ON COLUMN identity_keys.key_ref IS 'Reference to private key in external secure storage (Vault path, HSM slot, KMS key ID). Never contains private key material.';
COMMENT ON COLUMN identity_keys.fingerprint IS 'SHA-256 fingerprint of the public key in hex. Used for fast key lookup and identification.';
