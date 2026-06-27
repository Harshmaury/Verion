# Verion — Master Decision Register

> Single source of truth for all architectural and technical decisions made in the Verion project.
> Every major decision gets an ADR. Every ADR is registered here.

---

## How to Use This Register

- **Before implementing** any major component → write an ADR first
- **Link every ADR** here immediately after writing it
- **Update status** here when an ADR is superseded or deprecated
- **Never delete rows** — mark as Superseded and point to the replacement

---

## ADR Status Legend

| Status | Meaning |
|--------|---------|
| 🟡 Proposed | Under discussion, not yet decided |
| 🟢 Accepted | Decision made, in effect |
| 🔵 Superseded | Replaced by a newer ADR |
| 🔴 Deprecated | No longer applicable |
| ⚪ Experimental | Tentative, subject to change |

---

## Decision Register

| ADR | Title | Status | Date | Phase |
|-----|-------|--------|------|-------|
| [ADR-000](adr/ADR-000-template.md) | ADR Template | 🟢 Accepted | 2026-06-27 | Foundation |
| [ADR-001](adr/ADR-001-technology-stack.md) | Technology Stack Selection | 🟢 Accepted | 2026-06-27 | Phase 0 |
| [ADR-002](adr/ADR-002-identity-data-model.md) | Identity Data Model | 🟢 Accepted | 2026-06-27 | Phase 1 |
| ADR-003 | Cryptographic Primitives & Agility Strategy | 🟡 Proposed | — | Phase 1 |
| ADR-004 | API Protocol Design (Proto-first, grpc-gateway) | 🟡 Proposed | — | Phase 1 |
| ADR-005 | Database Migration Strategy | 🟡 Proposed | — | Phase 1 |
| ADR-006 | Session & Token Design | 🟡 Proposed | — | Phase 2 |
| ADR-007 | WebAuthn / FIDO2 Implementation Strategy | 🟡 Proposed | — | Phase 2 |
| ADR-008 | Trust Score Model & Evidence Schema | 🟡 Proposed | — | Phase 3 |
| ADR-009 | Policy Engine Design (OPA vs custom) | 🟡 Proposed | — | Phase 4 |
| ADR-010 | Federation Protocol Support (OIDC, SAML, DID) | 🟡 Proposed | — | Phase 5 |
| ADR-011 | Observability Stack (Metrics, Tracing, Logging) | 🟡 Proposed | — | Phase 6 |
| ADR-012 | Post-Quantum Cryptography Migration Path | 🟡 Proposed | — | Phase 6 |

---

## Key Decisions at a Glance

### Language & Runtime
- **Go 1.24** — primary language for all core services
- **Python** — reserved for AI/ML components only (behavioral analysis, risk scoring)
- **Rust** — experimental, considered for future performance-critical modules

### API
- **gRPC + Protobuf (proto3)** — all internal service communication
- **grpc-gateway v2** — REST transcoding layer for external clients
- **OpenAPI 3.0** — auto-generated from proto definitions
- Single source of truth: `.proto` files in `api/proto/`

### Data
- **PostgreSQL 16** — primary identity store (ACID, RLS, JSONB)
- **Redis 7** — session cache, trust score cache, pub/sub for invalidation
- **golang-migrate** — database migration management
- **Row-Level Security (RLS)** — tenant isolation enforced at database level

### Identity Model
- **Universal Identity entity** — one model supports human, org, device, service, machine, AI agent
- **Cryptographic key binding** — every identity owns ≥1 key pair; no password-only identities
- **Soft delete only** — identities are never hard deleted
- **Append-only audit log** — every mutation recorded immutably
- **Multi-tenant from day one** — tenant_id on every entity

### Security
- **AES-256-GCM** — symmetric encryption for attributes at rest
- **Ed25519 / ECDSA P-256** — asymmetric signing (preferred over RSA)
- **Argon2id** — key derivation and password hashing
- **TLS 1.3 minimum** — enforced on all connections
- **Private keys never stored in database** — key_ref points to secure external storage

### Infrastructure
- **Docker Compose** — local development
- **Kubernetes** — production deployment target
- **Minikube** — local k8s testing

---

## Planned ADR Topics (Backlog)

These topics will need decisions before implementation reaches them:

- Secure key storage backend (HashiCorp Vault vs AWS KMS vs local HSM emulation)
- Rate limiting strategy for auth endpoints
- Distributed tracing correlation (trace-id propagation across gRPC)
- AI Agent identity lifecycle (how agents are provisioned, attested, rotated)
- Offline authentication model (auth without network connectivity)
- Cross-tenant federation model
- Compliance framework mapping (SOC2, ISO 27001, GDPR)
- Chaos engineering and fault injection strategy

---

## Decision Process

1. Identify a major decision that needs to be made
2. Copy `ADR-000-template.md` → `ADR-XXX-title.md`
3. Fill in Context, Options, Decision, Consequences
4. Add to this register with status `Proposed`
5. Review and finalize → update status to `Accepted`
6. Implement following the ADR
7. If decision changes → write a new ADR, mark old one `Superseded`

---

*Last updated: 2026-06-27*
*Maintained by: Harsh Maury*
