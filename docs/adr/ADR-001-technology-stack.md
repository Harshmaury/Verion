# ADR-001: Technology Stack Selection

| Field       | Value                        |
|-------------|------------------------------|
| **Status**  | Accepted                     |
| **Date**    | 2026-06-27                   |
| **Author**  | Harsh Maury                  |
| **Supersedes** | —                         |
| **Superseded By** | —                     |

---

## Context

Verion is a next-generation Identity Operating System designed to operate at
planetary scale. It must support billions of identities, trillions of device
attestations, autonomous AI agent identities, and real-time continuous trust
evaluation — all while maintaining rigorous cryptographic guarantees and
privacy-preserving properties.

The technology stack must support:

- High-throughput, low-latency authentication flows
- Strongly typed, versioned internal service contracts
- Universal external API compatibility
- Horizontally scalable stateless services
- Cryptographically agile security primitives
- Long-term maintainability by a small team with AI assistance

This is a solo engineering project with AI-assisted development. Stack
simplicity, strong tooling, and ecosystem maturity are critical factors.

---

## Decision Drivers

- **Performance** — Authentication and trust evaluation must be sub-100ms at scale
- **Type safety** — Strong typing prevents entire classes of identity bugs
- **Ecosystem maturity** — Libraries for crypto, auth standards, and networking must exist
- **Interoperability** — Must integrate with existing identity ecosystems (OIDC, SAML, FIDO2)
- **Operational simplicity** — Solo developer; avoid unnecessary operational complexity
- **Cryptographic agility** — Stack must support post-quantum primitives as they mature
- **AI-assisted development** — Stack must be well-understood by AI coding assistants

---

## Decision: Primary Language — Go

### Options Considered

**Option A — Go**
- Compiled, statically typed, garbage collected
- Excellent concurrency model (goroutines, channels)
- Native gRPC and protobuf support
- Strong standard library for networking and crypto
- Fast compile times, single binary deployment
- Mature ecosystem for identity (jwt-go, x/crypto, WebAuthn libraries)

**Option B — Rust**
- Maximum performance and memory safety
- No garbage collector (predictable latency)
- Steeper learning curve, slower development velocity
- gRPC ecosystem less mature than Go
- Poor choice for rapid solo development

**Option C — Python**
- Excellent for ML/AI components
- Poor performance for high-throughput auth flows
- Dynamic typing increases risk in security-critical code

**Decision: Go**

Go provides the optimal balance of performance, type safety, developer
productivity, and ecosystem maturity for an identity platform. Rust is
reserved for future performance-critical components if profiling justifies it.
Python is used only for AI/ML components (behavioral analysis, risk scoring).

---

## Decision: API Layer — gRPC + REST Gateway

### Options Considered

**Option A — gRPC Only**
- Maximum performance, strongly typed contracts via Protobuf
- Binary protocol, not human-readable
- Poor browser and third-party compatibility without transcoding
- Excellent for internal service-to-service communication

**Option B — REST Only**
- Universal compatibility
- Weakly typed, JSON schema validation is error-prone
- Higher latency than gRPC for internal calls
- Insufficient for a platform meant to be infrastructure

**Option C — gRPC + REST Gateway (grpc-gateway)** ✓ CHOSEN
- gRPC for all internal service communication
- grpc-gateway generates REST endpoints from Protobuf definitions
- Single source of truth: `.proto` files define both APIs
- External clients (browsers, third-party integrations) use REST
- Internal services use gRPC directly
- OpenAPI spec auto-generated from proto definitions

**Decision: Option C — gRPC + REST Gateway**

This is the industry standard for platform-grade APIs. One proto definition
produces both the gRPC server, the REST gateway, and the OpenAPI documentation.
No duplication, no drift between API layers.

---

## Decision: Primary Database — PostgreSQL

### Options Considered

**Option A — PostgreSQL** ✓ CHOSEN
- ACID compliant — critical for identity data integrity
- JSONB support for flexible identity attributes
- Row-level security for multi-tenant isolation
- Mature ecosystem, excellent Go drivers (pgx)
- UUID primary keys natively supported
- Strong support for encrypted columns and audit trails

**Option B — CockroachDB**
- Distributed SQL, horizontal scaling
- Higher operational complexity
- Premature optimization for current phase

**Option C — MongoDB**
- Flexible schema
- Weaker consistency guarantees — unacceptable for identity data
- No native transaction support across collections

**Decision: PostgreSQL**

Identity data requires the strongest consistency guarantees available.
PostgreSQL's ACID compliance, row-level security, and mature Go ecosystem
make it the only acceptable choice for Phase 1. Distributed SQL (CockroachDB)
is an upgrade path for Phase 6 if horizontal scaling requires it.

---

## Decision: Cache & Session Layer — Redis

### Options Considered

**Option A — Redis** ✓ CHOSEN
- Sub-millisecond reads — critical for session validation on every request
- Native TTL support — perfect for session expiry
- Pub/Sub for real-time session invalidation across nodes
- Sorted sets for trust score time-series
- Mature Go client (go-redis)

**Option B — Memcached**
- Simpler, but no pub/sub, no sorted sets, no persistence
- Insufficient for session intelligence features

**Option C — In-memory (per node)**
- No session sharing across horizontal replicas
- Unacceptable for a distributed identity platform

**Decision: Redis**

Session validation occurs on every authenticated request. Redis provides
the performance, data structures, and pub/sub capabilities required for
Verion's session intelligence layer.

---

## Decision: Container & Orchestration — Docker + Kubernetes

### Options Considered

**Option A — Docker Compose (dev) + Kubernetes (prod)** ✓ CHOSEN
- Docker Compose for local development (simple, fast)
- Kubernetes for production deployment (horizontal scaling, health checks)
- Minikube available locally for k8s testing
- Industry standard for platform infrastructure

**Option B — Docker only**
- Insufficient for production horizontal scaling

**Option C — Bare metal**
- No isolation, no orchestration, not viable

**Decision: Docker Compose for dev, Kubernetes for prod**

Local development uses Docker Compose for simplicity. Production targets
Kubernetes. Minikube enables local k8s testing without cloud dependency.

---

## Decision: Cryptographic Foundation

| Primitive | Library | Rationale |
|-----------|---------|-----------|
| Symmetric encryption | `crypto/aes` (AES-256-GCM) | Go stdlib, FIPS-compliant |
| Asymmetric encryption | `crypto/ecdsa`, `crypto/ed25519` | Modern, fast, small keys |
| Key derivation | `golang.org/x/crypto/argon2` | Memory-hard, phishing-resistant |
| Password hashing | `golang.org/x/crypto/bcrypt` | Battle-tested fallback |
| JWT signing | `ES256` (ECDSA P-256) | Standard, compact tokens |
| WebAuthn / FIDO2 | `github.com/go-webauthn/webauthn` | Mature Go implementation |
| TLS | Go stdlib `crypto/tls` (TLS 1.3) | Enforced minimum version |
| Post-Quantum (future) | `golang.org/x/crypto` (Kyber/Dilithium when stable) | Planned ADR-003 |

---

## Full Stack Summary

| Layer | Technology | Version |
|-------|-----------|---------|
| Language | Go | 1.24 |
| Internal API | gRPC + Protobuf | proto3 |
| External API | REST via grpc-gateway | v2 |
| API Spec | OpenAPI | 3.0 |
| Primary Database | PostgreSQL | 16 |
| Cache / Session | Redis | 7 |
| Container (dev) | Docker Compose | v2 |
| Container (prod) | Kubernetes | 1.29+ |
| Migrations | golang-migrate | latest |
| Config | Viper | latest |
| Logging | Zap (uber-go) | latest |
| Metrics | Prometheus | latest |
| Tracing | OpenTelemetry | latest |
| Testing | Go stdlib + testify | latest |

---

## Consequences

### Positive
- Single language (Go) across all core services reduces cognitive overhead
- Proto-first API design ensures consistency between gRPC and REST
- PostgreSQL + Redis is a proven, battle-tested combination for auth platforms
- Docker Compose makes onboarding and local development trivial

### Negative / Trade-offs
- gRPC adds initial setup complexity (proto compilation, gateway wiring)
- PostgreSQL requires schema migrations discipline from day one
- No polyglot services — Python ML components need a separate service boundary

### Risks
- Post-quantum cryptography standards still evolving — mitigated by crypto agility design
- Redis as single point of failure for sessions — mitigated by Redis Sentinel/Cluster in prod

---

## Implementation Notes

- All `.proto` files live in `api/proto/`
- Proto compilation via `buf` tool (preferred over raw `protoc`)
- Database migrations via `golang-migrate` in `internal/db/migrations/`
- All crypto operations go through `internal/crypto/` — never call crypto primitives directly from business logic
- Configuration via environment variables + Viper, never hardcoded

---

## References

- [gRPC-Gateway](https://github.com/grpc-ecosystem/grpc-gateway)
- [pgx PostgreSQL driver](https://github.com/jackc/pgx)
- [go-redis](https://github.com/redis/go-redis)
- [go-webauthn](https://github.com/go-webauthn/webauthn)
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [uber-go/zap](https://github.com/uber-go/zap)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
