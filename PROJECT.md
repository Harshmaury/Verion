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

**Ambition:** Become foundational identity infrastructure the way TCP/IP
standardized communication and TLS standardized transport security.

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

## Workflow Protocol (AI must follow this)

### File Delivery
1. AI produces files with **unique timestamped names**: `verion-<component>-YYYYMMDD_HHMMSS.<ext>`
2. Files are presented for download to `Verion-drop`
3. AI provides exact `cp` command to copy from drop into project
4. Developer runs the command, verifies, then confirms

### Copy Command Pattern
```bash
cd /home/harsh/workspace/projects/Verion
cp /mnt/c/Users/harsh/Downloads/Verion-drop/<filename> <target-path>
```

### Git Convention
```
<type>(<scope>): <description>

Types: feat | fix | docs | chore | refactor | test | arch
```

### Branch Strategy
- `main` — stable only
- `dev` — active development
- `feat/<name>` — feature branches

### Makefile PATH fix (WSL)
The Makefile has this at the top to fix Go PATH in WSL:
```makefile
SHELL := /bin/bash
PATH := /usr/local/go/bin:$(PATH)
```

---

## Technology Stack (LOCKED — see ADR-001)

| Layer | Technology | Version |
|-------|-----------|---------|
| Language | Go | 1.24 |
| Internal API | gRPC + Protobuf | proto3 |
| External API | REST via grpc-gateway | v2 |
| API Spec | OpenAPI | 3.0 |
| Primary Database | PostgreSQL | 16 |
| Cache / Session | Redis | 7 |
| Container (dev) | Docker Compose | v1 (standalone) |
| Container (prod) | Kubernetes | 1.29+ |
| Migrations | golang-migrate | latest |
| Config | Viper | latest |
| Logging | Zap (uber-go) | latest |
| Metrics | Prometheus | latest |
| Tracing | OpenTelemetry | latest |
| Testing | Go stdlib + testify | latest |

**Important WSL note:** Use `docker-compose` (hyphen), NOT `docker compose` (space).
The Docker Compose plugin is not installed — only standalone docker-compose works.

**Important Docker note:** `~/.docker/config.json` must have `"credsStore": ""`
to prevent credential helper errors in WSL.

---

## Go Module

```
module github.com/Harshmaury/verion
go 1.24
```

---

## Project Structure

```
/home/harsh/workspace/projects/Verion/
├── cmd/
│   └── verion/main.go           ✓ EXISTS — entry point
├── internal/
│   ├── identity/                ⏳ Phase 1
│   ├── auth/                    ⏳ Phase 2
│   ├── trust/                   ⏳ Phase 3
│   ├── authz/                   ⏳ Phase 4
│   ├── session/                 ⏳ Phase 2
│   ├── crypto/                  ⏳ Phase 1
│   ├── device/                  ⏳ Phase 5
│   ├── recovery/                ⏳ Phase 1
│   └── audit/                   ⏳ Phase 1
├── pkg/                         ⏳ Phase 3+
├── api/
│   ├── proto/                   ⏳ Phase 1 Step 7
│   └── rest/                    ⏳ Phase 1 Step 7
├── docs/
│   ├── DECISIONS.md             ✓ EXISTS — master decision register
│   └── adr/
│       ├── ADR-000-template.md  ✓ EXISTS
│       ├── ADR-001-technology-stack.md     ✓ EXISTS
│       └── ADR-002-identity-data-model.md  ✓ EXISTS
├── deployments/
│   ├── docker/
│   │   └── postgres/init.sql   ✓ EXISTS
│   └── k8s/                    ⏳ Phase 6
├── scripts/
│   └── context-pack.sh         ✓ EXISTS — context packaging tool
├── docker-compose.yml           ✓ EXISTS
├── .env.example                 ✓ EXISTS
├── .env                         ✓ EXISTS (git-ignored)
├── go.mod                       ✓ EXISTS
├── Makefile                     ✓ EXISTS (WSL PATH fixed)
├── README.md                    ✓ EXISTS
├── .gitignore                   ✓ EXISTS
├── PROJECT.md                   ✓ THIS FILE
└── SESSION.md                   ✓ EXISTS — session log
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
| ADR-006 | Session & Token Design | 🟡 Proposed |
| ADR-007 | WebAuthn / FIDO2 Strategy | 🟡 Proposed |
| ADR-008 | Trust Score Model | 🟡 Proposed |
| ADR-009 | Policy Engine Design | 🟡 Proposed |
| ADR-010 | Federation Protocol Support | 🟡 Proposed |
| ADR-011 | Observability Stack | 🟡 Proposed |
| ADR-012 | Post-Quantum Crypto Path | 🟡 Proposed |

Full details in `docs/DECISIONS.md` and `docs/adr/`.

---

## Identity Data Model Summary (ADR-002)

Core entities:

- **Identity** — Universal entity (human | org | device | service | machine | ai_agent)
- **IdentityKey** — Cryptographic key pairs (Ed25519, ECDSA P-256/P-384)
- **Credential** — Auth mechanisms (passkey, TOTP, hardware token, API key, mTLS)
- **Tenant** — Multi-tenant isolation (every identity belongs to one tenant)
- **AuditEvent** — Append-only immutable audit log
- **RecoveryMethod** — Recovery codes, backup keys, trusted contacts

Key rules:
- Every identity owns ≥1 cryptographic key pair
- Private keys are NEVER stored in the database
- Identities are NEVER hard deleted (soft delete only)
- Audit log is NEVER updated or deleted (append-only)
- Every table has `tenant_id` (enforced at DB level via RLS)

---

## Development Phases

| Phase | Name | Status |
|-------|------|--------|
| Phase 0 | Foundation | ✅ COMPLETE |
| Phase 1 | Identity Core | 🔄 IN PROGRESS |
| Phase 2 | Authentication Engine | ⏳ Planned |
| Phase 3 | Trust Evaluation Engine | ⏳ Planned |
| Phase 4 | Authorization & Policy | ⏳ Planned |
| Phase 5 | Federation | ⏳ Planned |
| Phase 6 | Scale & Production Hardening | ⏳ Planned |

---

## Phase 1 — Identity Core Build Order

| Step | Task | Status |
|------|------|--------|
| Step 1 | Docker Compose (PostgreSQL + Redis) | ✅ COMPLETE |
| Step 2 | Database Migration Schema | ⏳ NEXT |
| Step 3 | Go Domain Model (Identity structs) | ⏳ |
| Step 4 | Repository Layer (DB operations) | ⏳ |
| Step 5 | Crypto Service (key generation) | ⏳ |
| Step 6 | Service Layer (business logic) | ⏳ |
| Step 7 | gRPC Proto definition | ⏳ |
| Step 8 | Wire everything together | ⏳ |

---

## Local Services (when Docker is running)

| Service | Address | Credentials |
|---------|---------|-------------|
| PostgreSQL | localhost:5432 | user: verion / pass: verion_dev_secret / db: verion |
| Redis | localhost:6379 | password: verion_redis_secret |

Start services: `docker-compose up -d`
Stop services: `docker-compose down`
View logs: `docker-compose logs -f`

---

## How to Start a New AI Session

1. Run `./scripts/context-pack.sh` — produces `verion-context-TIMESTAMP.zip`
2. The zip lands in `C:\Users\harsh\Downloads\Verion-drop` automatically
3. Upload the zip to the new AI conversation
4. Say: *"Unzip and read all files. This is the Verion project. Read PROJECT.md
   first, then SESSION.md. Continue from where we left off."*

The AI will have full context within seconds.

---

## Rules for AI Working on This Project

1. **Never skip ADRs** — every major decision needs a decision record
2. **Never hardcode secrets** — always use `.env` and Viper config
3. **Never store private keys in the database** — use `key_ref` to external storage
4. **Never hard delete identities** — set `status = deactivated`
5. **Never write to audit log except appending** — no UPDATE/DELETE on AuditEvent
6. **Always include `tenant_id`** in every query
7. **Always use timestamped filenames** for deliverables
8. **Always provide the `cp` command** with every file
9. **Use `docker-compose`** (hyphen) not `docker compose` (space) in WSL
10. **Update SESSION.md** at the end of every session
