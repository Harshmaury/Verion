#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Database Migration Setup
# Run from: /home/harsh/workspace/projects/Verion
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

DROP="/mnt/c/Users/harsh/Downloads/Verion-drop"
TS="20260628_041509"
MIGRATIONS_DIR="internal/db/migrations"
DB_URL="postgres://verion:verion_dev_secret@localhost:5432/verion?sslmode=disable"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║     VERION — Phase 1 · Step 2 · Database Migrations     ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ── Create migrations directory ───────────────────────────────────────────────
echo "[1/4] Creating migrations directory..."
mkdir -p "$MIGRATIONS_DIR"
echo "      ✓ $MIGRATIONS_DIR"

# ── Copy migration files ──────────────────────────────────────────────────────
echo "[2/4] Copying migration files..."

FILES=(
  "verion-migrate-001-up-${TS}.sql:000001_tenants.up.sql"
  "verion-migrate-001-dn-${TS}.sql:000001_tenants.down.sql"
  "verion-migrate-002-up-${TS}.sql:000002_identities.up.sql"
  "verion-migrate-002-dn-${TS}.sql:000002_identities.down.sql"
  "verion-migrate-003-up-${TS}.sql:000003_identity_keys.up.sql"
  "verion-migrate-003-dn-${TS}.sql:000003_identity_keys.down.sql"
  "verion-migrate-004-up-${TS}.sql:000004_credentials.up.sql"
  "verion-migrate-004-dn-${TS}.sql:000004_credentials.down.sql"
  "verion-migrate-005-up-${TS}.sql:000005_recovery_methods.up.sql"
  "verion-migrate-005-dn-${TS}.sql:000005_recovery_methods.down.sql"
  "verion-migrate-006-up-${TS}.sql:000006_audit_events.up.sql"
  "verion-migrate-006-dn-${TS}.sql:000006_audit_events.down.sql"
  "verion-migrate-007-up-${TS}.sql:000007_primary_key_ref_and_rls.up.sql"
  "verion-migrate-007-dn-${TS}.sql:000007_primary_key_ref_and_rls.down.sql"
)

for entry in "${FILES[@]}"; do
  src="${entry%%:*}"
  dst="${entry##*:}"
  cp "$DROP/$src" "$MIGRATIONS_DIR/$dst"
  echo "      ✓ $dst"
done

# ── Run migrations ────────────────────────────────────────────────────────────
echo ""
echo "[3/4] Running migrations..."
migrate -path "$MIGRATIONS_DIR" -database "$DB_URL" up

echo ""
echo "      Migration status:"
migrate -path "$MIGRATIONS_DIR" -database "$DB_URL" version

# ── Verify tables exist ───────────────────────────────────────────────────────
echo ""
echo "      Verifying tables in PostgreSQL..."
docker-compose exec -T postgres psql -U verion -d verion -c "\dt" 2>/dev/null

# ── Commit ────────────────────────────────────────────────────────────────────
echo ""
echo "[4/4] Committing to Git..."
git add "$MIGRATIONS_DIR"
git commit -m "feat(db): add identity core database migrations

- 001: tenants table with tier/status enums
- 002: identities table (universal entity, 6 types, encrypted attributes)
- 003: identity_keys table (public keys only, private key refs external)
- 004: credentials table (passkey, TOTP, hardware token, API key, mTLS)
- 005: recovery_methods table (codes, backup key, trusted contact)
- 006: audit_events table (append-only, immutable, UPDATE/DELETE blocked)
- 007: primary_key_id ref + Row Level Security on all tables"

git push origin main

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓  Database migrations complete                         ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Tables created:"
echo "  tenants · identities · identity_keys"
echo "  credentials · recovery_methods · audit_events"
echo ""
echo "Next: Phase 1 · Step 3 — Go Domain Model"
echo ""
