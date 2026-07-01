#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Go Domain Model Setup
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TS="20260628_045648"
PKG_DIR="internal/identity"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Phase 1 · Step 3 · Go Domain Model         ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

echo "[1/4] Creating package directory..."
mkdir -p "$PKG_DIR"
echo "      ✓ $PKG_DIR"

echo "[2/4] Copying Go source files..."
cp "$DROP/verion-enums-${TS}.go"  "$PKG_DIR/enums.go"
echo "      ✓ internal/identity/enums.go"

cp "$DROP/verion-model-${TS}.go"  "$PKG_DIR/model.go"
echo "      ✓ internal/identity/model.go"

cp "$DROP/verion-errors-${TS}.go" "$PKG_DIR/errors.go"
echo "      ✓ internal/identity/errors.go"

echo "[3/4] Verifying Go compilation..."
go build ./internal/identity/...
echo "      ✓ Package compiles cleanly"

echo "[4/4] Committing to Git..."
git add internal/identity/
git commit -m "feat(identity): add Go domain model

- enums.go: typed constants for all 6 identity types, key types,
  credential types, recovery types matching PostgreSQL enums exactly
- model.go: domain structs (Tenant, Identity, IdentityKey, Credential,
  RecoveryMethod, AuditEvent) with helpers and audit event constants
- errors.go: typed sentinel errors and wrapping types for precise
  error handling across service and transport layers"

git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Go domain model installed                            ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Package: internal/identity"
echo "  enums.go   — typed enum constants"
echo "  model.go   — domain structs"
echo "  errors.go  — typed sentinel errors"
echo ""
echo "Next: Phase 1 · Step 4 — Repository Layer (database operations)"
echo ""
