# Verion

> **Universal Identity Operating System**

Verion is a next-generation Identity Operating System that provides universal,
cryptographically verifiable, privacy-preserving, and continuously adaptive
identity infrastructure for the digital ecosystem.

---

## Core Principle

> Trust is not granted; it is continuously established through independently verifiable evidence.

---

## Architecture

```
┌────────────────────────────────────────────────────┐
│                  Identity Registry                 │
├────────────────────────────────────────────────────┤
│            Identity Graph & Trust Model            │
├──────────────┬──────────────┬──────────────────────┤
│Authentication│Authorization │ Trust Evaluation     │
├──────────────┼──────────────┼──────────────────────┤
│Session Engine│Device Trust  │ Policy Decision      │
├──────────────┼──────────────┼──────────────────────┤
│Recovery      │Audit         │ Cryptographic Svc    │
├──────────────┴──────────────┴──────────────────────┤
│  APIs · SDKs · Gateways · Federation               │
└────────────────────────────────────────────────────┘
```

---

## Development Phases

| Phase | Scope | Status |
|-------|-------|--------|
| Phase 0 | Foundation — structure, tooling, ADRs | 🔄 In Progress |
| Phase 1 | Identity Core — registry, keys, storage | ⏳ Planned |
| Phase 2 | Authentication — passwordless, FIDO2 | ⏳ Planned |
| Phase 3 | Trust Engine — continuous scoring | ⏳ Planned |
| Phase 4 | Authorization — policy, dynamic access | ⏳ Planned |
| Phase 5 | Federation — OIDC, SAML, DID | ⏳ Planned |
| Phase 6 | Scale — performance, observability | ⏳ Planned |

---

## Getting Started

```bash
# Build
make build

# Run
make run

# Test
make test
```

---

## Technology Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24 |
| API | gRPC + REST (OpenAPI) |
| Database | PostgreSQL |
| Cache | Redis |
| Container | Docker + Kubernetes |
| Crypto | TBD — see ADR-003 |

---

## Architecture Decisions

See [`docs/adr/`](docs/adr/) for all Architecture Decision Records.

---

## License

Proprietary — All rights reserved.
