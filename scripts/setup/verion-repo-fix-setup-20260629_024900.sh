#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Repository Layer Fix
# Fixes: withTenantConn callback type mismatch
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TS="20260629_024900"
PKG="internal/identity/postgres"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Repository Layer Fix                        ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

echo "[1/3] Replacing fixed files..."
cp "$DROP/verion-db-fix-${TS}.go"             "$PKG/db.go"
echo "      ✓ $PKG/db.go"
cp "$DROP/verion-tenant-repo-fix-${TS}.go"    "$PKG/tenant_repo.go"
echo "      ✓ $PKG/tenant_repo.go"
cp "$DROP/verion-identity-repo-fix-${TS}.go"  "$PKG/identity_repo.go"
echo "      ✓ $PKG/identity_repo.go"
cp "$DROP/verion-audit-key-repo-fix-${TS}.go" "$PKG/audit_key_repo.go"
echo "      ✓ $PKG/audit_key_repo.go"

echo "[2/3] Verifying compilation..."
go build ./internal/...
echo "      ✓ All packages compile cleanly"

echo "[3/3] Committing fix..."
git add internal/
git commit -m "fix(identity): resolve withTenantConn callback type mismatch

All repository callbacks now use *pgxpool.Conn directly instead of
anonymous interface types — consistent with db.go signature."
git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Repository layer fixed and compiling                 ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Next: Phase 1 · Step 5 — Crypto Service"
echo ""
