# Verion — Project Intelligence File

> **READ THIS FIRST** — This file is the single source of truth for any AI or
> developer joining this project. Read it completely before doing anything.
> Then read `SESSION.md` for the latest state.

---

## What is Verion?

Verion is a **Universal Identity Operating System** — a next-generation
identity infrastructure platform designed to operate at planetary scale.

**Core Principle:**
> Trust is not granted; it is continuously established through independently
> verifiable evidence.

Verion treats identity as a continuously evaluated confidence model, not a
binary authentication event. It combines cryptographic verification, contextual
intelligence, distributed trust evaluation, and policy-driven authorization
into a unified platform.

**Scope:** Human identities, organization identities, device identities,
service identities, machine identities, and AI agent identities — all under
one universal model.

---

## Developer Profile

| Field | Value |
|-------|-------|
| Developer | Harsh Maury (solo) |
| Development style | Solo engineering with AI assistance |
| Primary environment | WSL2 Ubuntu on Windows 11 |
| IDE | Visual Studio Code |
| Project root (WSL) | `/home/harsh/workspace/projects/Verion` |
| Git remote | `git@github.com:Harshmaury/Verion.git` |
| Download drop (Windows) | `C:\Users\harsh\Downloads\Verion-drop` |
| WSL drop path | `/mnt/c/Users/harsh/Downloads/Verion-drop` |

---

## AI Team Workflow

Three roles operate on this project:

| Role | Who | Responsibility |
|------|-----|----------------|
| Engineering Lead | Harsh | Runs commands, moves files, final decisions |
| Architect | Claude Account A | Designs, writes SPECs, reviews REPORTs |
| Implementer | Claude Account B | Reads SPECs, writes code, submits REPORTs |

### Implementer Prompt (paste this every time)
```
You are an Implementer engineer. Your only job is to write code.

Read PROJECT.md from the context zip first.
Then read the SPEC file carefully.

DO NOT review. DO NOT discuss. BUILD the code. Every file in the SPEC.
Use timestamped filenames. Submit a REPORT when done.
Produce an install script.

Drop: C:\Users\harsh\Downloads\Verion-drop
Project root (WSL): /home/harsh/workspace/projects/Verion
```

### File naming
All deliverables: `verion-<component>-YYYYMMDD_HHMMSS.<ext>`

### Copy command pattern
```bash
cd /home/harsh/workspace/projects/Verion
cp /mnt/c/Users/harsh/Downloads/Verion-drop/<filename> <target-path>
```

---

## Workflow Documents
```
docs/workflow/
  WORKFLOW.md          ← complete system guide
  SPEC-template.md
  REPORT-template.md
  REVIEW-template.md
  specs/               ← all issued SPECs
```

---

## Known WSL Issues (already fixed)

| Issue | Fix Applied |
|-------|-------------|
| `make build` → `go: Permission denied` | `PATH := /usr/local/go/bin:$(PATH)` in Makefile |
| `docker compose` → unknown flag | Use `docker-compose` (hyphen) not `docker compose` |
| Docker credential error | Set `"credsStore": ""` in `~/.docker/config.json` |
| `git push origin main` → refspec error | `git branch -M main` |

---

## Technology Stack (LOCKED — ADR-001)

| Layer | Technology | Version |
|-------|-----------|---------|
| Language | Go | 1.25 |
| Internal API | gRPC + Protobuf | proto3 |
| External API | REST (net/http) | stdlib |
| Primary Database | PostgreSQL | 16 |
| Cache / Session | Redis | 7 |
| Container (dev) | Docker Compose | v1 standalone |
| Crypto | Ed25519, ECDSA P-256, AES-256-GCM, Argon2id | — |

**Important:** Use `docker-compose` (hyphen) in WSL — plugin not installed.

---

## Go Module

```
module github.com/Harshmaury/verion
go 1.25
```

---

## Project Structure (current state)

```
/home/harsh/workspace/projects/Verion/
├── cmd/verion/main.go               ✓ Full server wiring
├── internal/
│   ├── identity/                    ✓ Domain model + service layer
│   │   ├── model.go                 ✓ All domain structs
│   │   ├── enums.go                 ✓ All typed enums
│   │   ├── errors.go                ✓ Typed sentinel errors
│   │   ├── repository.go            ✓ Repository interfaces
│   │   ├── tenant_service.go        ✓ TenantService
│   │   ├── identity_service.go      ✓ IdentityService
│   │   ├── key_service.go           ✓ KeyService
│   │   ├── svc_helpers.go           ✓ wrapRepoError
│   │   └── postgres/                ✓ PostgreSQL implementations
│   │       ├── db.go                ✓ pgxpool + RLS helpers
│   │       ├── tenant_repo.go       ✓
│   │       ├── identity_repo.go     ✓
│   │       └── audit_key_repo.go    ✓
│   ├── crypto/                      ✓ Crypto service
│   │   ├── service.go               ✓ CryptoService interface + Service
│   │   ├── keys.go                  ✓ Ed25519 + ECDSA key generation
│   │   ├── aes.go                   ✓ AES-256-GCM encrypt/decrypt
│   │   ├── hash.go                  ✓ Argon2id hashing
│   │   └── local/keystore.go        ✓ Dev-only in-memory KeyStore
│   ├── config/                      ✓ Config loader (env vars)
│   ├── transport/
│   │   └── grpc/                    ✓ gRPC handlers
│   │       ├── server.go            ✓
│   │       ├── identity_server.go   ✓
│   │       └── keys_server.go       ✓
│   ├── auth/                        ⏳ Phase 2
│   ├── session/                     ⏳ Phase 2
│   └── trust/                       ⏳ Phase 3
├── api/
│   ├── proto/verion/v1/             ✓ Proto definitions
│   └── gen/go/verion/v1/            ✓ Hand-written pb.go stubs
├── internal/db/migrations/          ✓ 7 migrations (001–007)
├── docs/
│   ├── DECISIONS.md                 ✓ Master decision register
│   ├── adr/                         ✓ ADR-000 through ADR-002
│   └── workflow/                    ✓ SPEC/REPORT/REVIEW system
├── deployments/docker/postgres/     ✓
├── scripts/
│   ├── context-pack.sh              ✓ AI context packaging tool
│   └── setup/                       ✓ Historical setup scripts
├── docker-compose.yml               ✓
├── .env.example                     ✓ (gitignored)
├── go.mod / go.sum                  ✓
├── Makefile                         ✓ WSL PATH fixed
├── README.md                        ✓
├── PROJECT.md                       ✓ THIS FILE
└── SESSION.md                       ✓ Session log
```

---

## Architecture Decision Records

| ADR | Title | Status |
|-----|-------|--------|
| ADR-000 | Template | ✓ Accepted |
| ADR-001 | Technology Stack | ✓ Accepted |
| ADR-002 | Identity Data Model | ✓ Accepted |
| ADR-003 | Cryptographic Primitives | 🟡 Proposed |
| ADR-004 | API Protocol Design | 🟡 Proposed |
| ADR-005 | Database Migration Strategy | 🟡 Proposed |
| ADR-006 | Session & Token Design | 🟡 Proposed — Phase 2 |
| ADR-007 | WebAuthn / FIDO2 Strategy | 🟡 Proposed — Phase 2 |

---

## Identity Data Model (ADR-002)

Core entities in PostgreSQL:
- **tenants** — top-level isolation boundary
- **identities** — universal entity (6 types)
- **identity_keys** — cryptographic key pairs (public only in DB)
- **credentials** — auth mechanisms (passkey, TOTP, API key, mTLS)
- **recovery_methods** — recovery codes, backup keys
- **audit_events** — append-only immutable log

Key rules enforced:
- Private keys NEVER in database — `key_ref` points to KeyStore
- Identities NEVER hard deleted — soft delete only
- Audit log NEVER updated or deleted — append only
- Every table has `tenant_id` — RLS enforced at DB level

---

## Development Phases

| Phase | Name | Status |
|-------|------|--------|
| Phase 0 | Foundation | ✅ COMPLETE |
| Phase 1 | Identity Core | ✅ COMPLETE — tagged `v0.1.0-phase1` |
| Phase 2 | Authentication Engine | 🔄 IN PROGRESS |
| Phase 3 | Trust Evaluation Engine | ⏳ Planned |
| Phase 4 | Authorization & Policy | ⏳ Planned |
| Phase 5 | Federation | ⏳ Planned |
| Phase 6 | Scale & Production Hardening | ⏳ Planned |

---

## Phase 1 — COMPLETE ✅

All 8 steps done. Tagged `v0.1.0-phase1`.

Server starts with:
```bash
export VERION_MASTER_KEY=$(openssl rand -hex 32)
docker-compose up -d postgres
go run ./cmd/verion
# → gRPC server on :50051
```

---

## Phase 2 — Authentication Engine

| Step | Task | Status |
|------|------|--------|
| Step 1 (SPEC-009) | HTTP REST Gateway + middleware | ⏳ NEXT |
| Step 2 (SPEC-010) | WebAuthn passkey registration | ⏳ |
| Step 3 (SPEC-011) | WebAuthn assertion (login) | ⏳ |
| Step 4 (SPEC-012) | JWT token issuance + verification | ⏳ |
| Step 5 (SPEC-013) | Session management (Redis) | ⏳ |
| Step 6 (SPEC-014) | TOTP support | ⏳ |

---

## Local Services

| Service | Address | Credentials |
|---------|---------|-------------|
| PostgreSQL | localhost:5432 | user: verion / pass: verion_dev_secret / db: verion |
| Redis | localhost:6379 | password: verion_redis_secret |
| gRPC server | localhost:50051 | no auth (Phase 2) |
| REST gateway | localhost:8080 | not built yet (SPEC-009) |

Start: `docker-compose up -d`
Stop: `docker-compose down`

---

## Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `VERION_MASTER_KEY` | ✓ | — | 64-char hex AES-256 key |
| `VERION_GRPC_ADDR` | No | `:50051` | gRPC listen address |
| `VERION_DB_HOST` | No | `localhost` | Postgres host |
| `VERION_DB_PORT` | No | `5432` | Postgres port |
| `VERION_DB_NAME` | No | `verion` | Postgres database |
| `VERION_DB_USER` | No | `verion` | Postgres user |
| `VERION_DB_PASSWORD` | No | `verion_dev_secret` | Postgres password |

Generate master key: `openssl rand -hex 32`

---

## Git History

```
a48ca59 feat(wire): wire full stack — Phase 1 complete (SPEC-008)  ← v0.1.0-phase1
366652f feat(grpc): add gRPC transport layer (SPEC-007)
722d48f fix(identity): remove duplicate audit event in CreateIdentity
3203a4b feat(identity): add service layer (SPEC-006)
fe02d31 feat(crypto): add crypto service (SPEC-005)
88e0316 fix(identity): resolve withTenantConn callback type mismatch
c85df50 feat(identity): add Go domain model
9f6129a feat(db): add identity core database migrations
b9b9ee8 chore(meta): add project intelligence system
27695ce docs(adr): add ADR-000, ADR-001, ADR-002
```

---

## Rules for AI Working on This Project

1. Never skip ADRs for major decisions
2. Never hardcode secrets — use env vars
3. Never store private keys in the database — use `key_ref`
4. Never hard delete identities — `status = deactivated`
5. Never write to audit log except appending
6. Always include `tenant_id` in every query
7. Always use timestamped filenames for deliverables
8. Always provide the `cp` command with every file
9. Use `docker-compose` (hyphen) not `docker compose` in WSL
10. Transport layer must never import repository layer directly
11. Service layer is the only entry point from transport to data
12. `ProtoReflect()` returning nil is acceptable until Phase 2 auth work requires JSON transcoding
