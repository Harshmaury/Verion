# ADR-002: Identity Data Model

| Field       | Value                        |
|-------------|------------------------------|
| **Status**  | Accepted                     |
| **Date**    | 2026-06-27                   |
| **Author**  | Harsh Maury                  |
| **Supersedes** | —                         |
| **Superseded By** | —                     |

---

## Context

Verion's foundational principle is that identity is a continuously evaluated
confidence model, not a binary state. The identity data model must therefore
represent not just static attributes (name, email) but the full lifecycle of
a digital entity: its cryptographic bindings, trust history, credential
inventory, device relationships, and recovery mechanisms.

The model must support:

- Human identities (individual users)
- Organization identities (tenants, enterprises)
- Device identities (phones, hardware tokens, IoT)
- Service identities (APIs, microservices)
- Machine identities (automated systems)
- AI Agent identities (autonomous software agents)

All identity types share a common core structure with type-specific extensions.

---

## Decision Drivers

- **Cryptographic binding** — Every identity must be bound to at least one cryptographic key pair, not a shared secret
- **Minimal disclosure** — Store only what is necessary; sensitive attributes encrypted at rest
- **Type extensibility** — New identity types must not require schema redesign
- **Auditability** — Every mutation to an identity must be traceable
- **Tenant isolation** — Multi-tenant from day one; no cross-tenant data leakage possible
- **Recovery** — Identity recovery must not weaken cryptographic guarantees
- **Soft delete** — Identities are never hard deleted; they are deactivated and archived

---

## Decision: Core Identity Model

Every entity in Verion is an **Identity**. All identity types share this core:

### Identity (Core Entity)

```
Identity {
  // Primary key
  id              UUID            PRIMARY KEY       -- Globally unique, immutable
  
  // Type discriminator
  type            IdentityType    NOT NULL          -- human | org | device | service | machine | ai_agent

  // Tenant isolation
  tenant_id       UUID            NOT NULL          -- Which tenant owns this identity
  
  // Display
  display_name    TEXT            NOT NULL          -- Human-readable name
  
  // Unique handle within tenant
  handle          TEXT            NOT NULL          -- username / service name / device name
  
  // Status lifecycle
  status          IdentityStatus  NOT NULL          -- active | suspended | deactivated | archived
  
  // Cryptographic root
  primary_key_id  UUID            NOT NULL          -- FK → IdentityKey (primary signing key)
  
  // Flexible attributes (encrypted JSONB)
  attributes      JSONB           NOT NULL DEFAULT '{}'
  
  // Metadata
  created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
  updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
  deactivated_at  TIMESTAMPTZ     NULL
  
  // Audit
  created_by      UUID            NULL              -- Which identity created this one
  version         BIGINT          NOT NULL DEFAULT 1 -- Optimistic locking
}
```

### IdentityType Enum

```
human       -- A human user
org         -- An organization or tenant root identity
device      -- A physical or virtual device
service     -- A software service or API
machine     -- An automated system or job
ai_agent    -- An autonomous AI agent
```

### IdentityStatus Enum

```
active          -- Normal operating state
suspended       -- Temporarily blocked (investigation, policy violation)
deactivated     -- Permanently disabled (user request, offboarding)
archived        -- Soft deleted, data retained for audit
pending         -- Created but not yet verified/activated
```

---

## Decision: Cryptographic Key Model

Every identity owns one or more cryptographic key pairs. Keys are the
foundation of trust — not passwords.

### IdentityKey

```
IdentityKey {
  id              UUID            PRIMARY KEY
  identity_id     UUID            NOT NULL          -- FK → Identity
  
  // Key metadata
  key_type        KeyType         NOT NULL          -- ed25519 | ecdsa_p256 | ecdsa_p384 | rsa_4096
  purpose         KeyPurpose      NOT NULL          -- signing | encryption | authentication | recovery
  
  // Public key material (stored in full)
  public_key      BYTEA           NOT NULL          -- DER-encoded public key
  public_key_jwk  JSONB           NOT NULL          -- JWK representation for OIDC/FIDO2
  
  // Private key: NEVER stored in database
  // Private keys live in: HSM | secure enclave | encrypted vault
  // key_ref points to the external secure storage location
  key_ref         TEXT            NOT NULL          -- Reference to private key in secure storage
  
  // Lifecycle
  status          KeyStatus       NOT NULL          -- active | rotated | revoked | compromised
  algorithm       TEXT            NOT NULL          -- e.g. "EdDSA", "ES256", "RS256"
  
  // Validity window
  valid_from      TIMESTAMPTZ     NOT NULL DEFAULT now()
  valid_until     TIMESTAMPTZ     NULL              -- NULL = no expiry (rotation policy enforced separately)
  
  // Rotation
  rotated_at      TIMESTAMPTZ     NULL
  rotated_to      UUID            NULL              -- FK → IdentityKey (successor key)
  
  // Metadata
  created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
  fingerprint     TEXT            NOT NULL          -- SHA-256 of public key, for quick lookup
}
```

### KeyType Enum

```
ed25519         -- Preferred: fast, small, secure (256-bit)
ecdsa_p256      -- FIDO2/WebAuthn standard (P-256)
ecdsa_p384      -- Higher security margin
rsa_4096        -- Legacy compatibility only
```

### KeyPurpose Enum

```
signing         -- Sign tokens, assertions, identity proofs
encryption      -- Encrypt sensitive attribute data
authentication  -- WebAuthn / FIDO2 credential key
recovery        -- Recovery key (higher ceremony required to use)
```

---

## Decision: Credential Model

Credentials are the mechanisms an identity uses to prove control.
They are separate from keys — a credential references a key.

### Credential

```
Credential {
  id              UUID            PRIMARY KEY
  identity_id     UUID            NOT NULL          -- FK → Identity
  key_id          UUID            NULL              -- FK → IdentityKey (if key-backed)
  
  // Type
  type            CredentialType  NOT NULL
  
  // Credential data (type-specific, encrypted at rest)
  data            BYTEA           NOT NULL          -- Encrypted credential payload
  data_iv         BYTEA           NOT NULL          -- Encryption IV
  
  // WebAuthn specific
  aaguid          UUID            NULL              -- Authenticator AAGUID
  credential_id   BYTEA           NULL              -- WebAuthn credential ID
  sign_count      BIGINT          NOT NULL DEFAULT 0
  
  // Lifecycle
  status          CredentialStatus NOT NULL         -- active | revoked | expired
  last_used_at    TIMESTAMPTZ     NULL
  
  // Metadata
  name            TEXT            NULL              -- User-assigned name e.g. "iPhone 15 Face ID"
  device_info     JSONB           NULL              -- Platform, OS, authenticator info
  created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
  expires_at      TIMESTAMPTZ     NULL
}
```

### CredentialType Enum

```
passkey         -- FIDO2 passkey (WebAuthn)
totp            -- Time-based OTP (TOTP/HOTP)
hardware_token  -- FIDO2 hardware security key
recovery_code   -- Single-use recovery codes
api_key         -- Service-to-service API key
mtls_cert       -- Mutual TLS client certificate
biometric       -- Platform biometric reference
```

---

## Decision: Tenant Model

Verion is multi-tenant from day one. Every identity belongs to exactly one tenant.

### Tenant

```
Tenant {
  id              UUID            PRIMARY KEY
  name            TEXT            NOT NULL
  slug            TEXT            NOT NULL UNIQUE   -- URL-safe identifier
  
  // Tier
  tier            TenantTier      NOT NULL DEFAULT 'standard'
  
  // Settings
  settings        JSONB           NOT NULL DEFAULT '{}'
  
  // Isolation
  data_region     TEXT            NOT NULL DEFAULT 'global'
  
  // Status
  status          TenantStatus    NOT NULL DEFAULT 'active'
  
  created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
  updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
}
```

---

## Decision: Audit Log Model

Every mutation to every identity entity is recorded immutably.

### AuditEvent

```
AuditEvent {
  id              UUID            PRIMARY KEY
  tenant_id       UUID            NOT NULL
  
  // What happened
  event_type      TEXT            NOT NULL          -- identity.created | key.rotated | credential.revoked etc.
  entity_type     TEXT            NOT NULL          -- identity | key | credential | tenant
  entity_id       UUID            NOT NULL
  
  // Who did it
  actor_id        UUID            NULL              -- Identity who performed the action
  actor_type      TEXT            NULL              -- human | service | system
  
  // Context
  ip_address      INET            NULL
  user_agent      TEXT            NULL
  session_id      UUID            NULL
  
  // Change data
  before          JSONB           NULL              -- State before change
  after           JSONB           NULL              -- State after change
  metadata        JSONB           NOT NULL DEFAULT '{}'
  
  // Immutable timestamp
  occurred_at     TIMESTAMPTZ     NOT NULL DEFAULT now()
}
```

Audit events are **append-only**. No UPDATE or DELETE is ever issued against this table.

---

## Decision: Recovery Model

Recovery must maintain equivalent security guarantees to primary authentication.

### RecoveryMethod

```
RecoveryMethod {
  id              UUID            PRIMARY KEY
  identity_id     UUID            NOT NULL
  
  type            RecoveryType    NOT NULL
  
  // Recovery data (heavily encrypted, salted)
  data            BYTEA           NOT NULL
  data_iv         BYTEA           NOT NULL
  
  // Usage control
  used_at         TIMESTAMPTZ     NULL
  is_consumed     BOOLEAN         NOT NULL DEFAULT false  -- One-time use methods
  
  status          RecoveryStatus  NOT NULL DEFAULT 'active'
  created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
  expires_at      TIMESTAMPTZ     NULL
}
```

### RecoveryType Enum

```
recovery_codes      -- Set of single-use codes (Argon2-hashed)
backup_key          -- Cryptographic backup key pair
trusted_contact     -- Social recovery via trusted identities
hardware_backup     -- Secondary hardware security key
```

---

## Entity Relationship Summary

```
Tenant
  └── Identity (many)
        ├── IdentityKey (many)        -- cryptographic keys
        ├── Credential (many)         -- auth mechanisms
        ├── RecoveryMethod (many)     -- recovery options
        └── AuditEvent (many)         -- immutable history
```

---

## Consequences

### Positive
- Universal model supports all identity types without separate tables
- Cryptographic keys are first-class entities — credentials reference keys
- Audit log is structurally immutable
- Multi-tenant isolation enforced at data model level
- No passwords stored — credential types are all phishing-resistant

### Negative / Trade-offs
- JSONB attributes require discipline — untyped fields can accumulate technical debt
- Key reference model assumes external secure key storage — adds operational dependency
- Soft deletes increase query complexity (always filter by status)

### Risks
- JSONB attribute encryption must be consistently applied — one missed field leaks data
- Audit log will grow very large at scale — requires partitioning strategy (Phase 6)

---

## Implementation Notes

- All tables use `UUID` primary keys generated with `uuid_generate_v4()`
- `updated_at` maintained via PostgreSQL trigger, not application code
- Encrypted JSONB uses AES-256-GCM with a per-record IV
- Encryption keys for `attributes` and `data` columns managed by `internal/crypto/`
- Row-level security (RLS) policies enforce tenant isolation at database level
- All queries include `tenant_id` in WHERE clause — enforced by repository layer

---

## References

- [W3C WebAuthn Spec](https://www.w3.org/TR/webauthn-2/)
- [NIST SP 800-63B — Digital Identity Guidelines](https://pages.nist.gov/800-63-3/sp800-63b.html)
- [RFC 7517 — JSON Web Key](https://datatracker.ietf.org/doc/html/rfc7517)
- [Decentralized Identifiers (DIDs) W3C](https://www.w3.org/TR/did-core/)
- [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html)
