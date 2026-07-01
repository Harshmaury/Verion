#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Repository Layer Setup
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TS="20260629_024407"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Phase 1 · Step 4 · Repository Layer        ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

echo "[1/4] Creating package directories..."
mkdir -p internal/identity/postgres
echo "      ✓ internal/identity/postgres"

echo "[2/4] Copying source files..."

cp "$DROP/verion-repository-${TS}.go"        internal/identity/repository.go
echo "      ✓ internal/identity/repository.go"

cp "$DROP/verion-db-${TS}.go"                internal/identity/postgres/db.go
echo "      ✓ internal/identity/postgres/db.go"

cp "$DROP/verion-tenant-repo-${TS}.go"       internal/identity/postgres/tenant_repo.go
echo "      ✓ internal/identity/postgres/tenant_repo.go"

cp "$DROP/verion-identity-repo-${TS}.go"     internal/identity/postgres/identity_repo.go
echo "      ✓ internal/identity/postgres/identity_repo.go"

cp "$DROP/verion-audit-key-repo-${TS}.go"    internal/identity/postgres/audit_key_repo.go
echo "      ✓ internal/identity/postgres/audit_key_repo.go"

echo "[3/4] Verifying Go compilation..."
go build ./internal/...
echo "      ✓ All packages compile cleanly"

echo "[4/4] Committing to Git..."
git add internal/
git commit -m "feat(identity): add repository layer

- repository.go: interfaces for Tenant, Identity, Key, Credential,
  Recovery, Audit repositories — contracts for all DB operations
- postgres/db.go: pgxpool connection, withTenantConn/withTenantTx
  helpers for automatic RLS enforcement on every query
- postgres/tenant_repo.go: full CRUD for tenants
- postgres/identity_repo.go: full CRUD for identities with optimistic
  locking and atomic create+audit in single transaction
- postgres/audit_key_repo.go: append-only audit log + key lifecycle
  management (rotate, revoke, mark compromised)"

git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Repository layer installed                           ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Packages:"
echo "  internal/identity/repository.go         (interfaces)"
echo "  internal/identity/postgres/db.go         (connection + RLS)"
echo "  internal/identity/postgres/tenant_repo.go"
echo "  internal/identity/postgres/identity_repo.go"
echo "  internal/identity/postgres/audit_key_repo.go"
echo ""
echo "Next: Phase 1 · Step 5 — Crypto Service (key generation)"
echo ""
