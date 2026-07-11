#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Verion — Smoke Test
# Runs against the live server and verifies all core flows work end-to-end.
# Must pass before any REPORT is submitted.
#
# Usage:
#   export VERION_MASTER_KEY=$(openssl rand -hex 32)
#   export VERION_HTTP_ADDR=:8082   # or whichever port is free
#   go run ./cmd/verion &
#   sleep 3
#   ./scripts/smoke-test.sh
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

BASE_URL="${VERION_SMOKE_URL:-http://localhost:8082}"
PASS=0
FAIL=0
TS=$(date +%s)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓ PASS${NC} — $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}✗ FAIL${NC} — $1"; FAIL=$((FAIL+1)); }
info() { echo -e "${YELLOW}→${NC} $1"; }

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║          VERION — Smoke Test                             ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo "  Target: $BASE_URL"
echo ""

# ── 1. Health check ───────────────────────────────────────────────────────────
info "1. Health check"
HEALTH=$(curl -s "$BASE_URL/healthz")
STATUS=$(echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status',''))" 2>/dev/null || echo "")
if [ "$STATUS" = "ok" ]; then
  pass "GET /healthz → {status: ok}"
else
  fail "GET /healthz → expected {status: ok}, got: $HEALTH"
fi

# ── 2. Create tenant ──────────────────────────────────────────────────────────
info "2. Create tenant"
SLUG="smoke-tenant-$TS"
TENANT=$(curl -s -X POST "$BASE_URL/v1/tenants" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Smoke Tenant\",\"slug\":\"$SLUG\",\"tier\":\"standard\",\"data_region\":\"global\"}")
TENANT_ID=$(echo "$TENANT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")
if [ -n "$TENANT_ID" ] && [ "$TENANT_ID" != "null" ]; then
  pass "POST /v1/tenants → id=$TENANT_ID"
else
  fail "POST /v1/tenants → no id in response: $TENANT"
  echo "Cannot continue without tenant. Exiting."
  exit 1
fi

# ── 3. Get tenant ─────────────────────────────────────────────────────────────
info "3. Get tenant"
GOT_TENANT=$(curl -s "$BASE_URL/v1/tenants/$TENANT_ID")
GOT_ID=$(echo "$GOT_TENANT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")
if [ "$GOT_ID" = "$TENANT_ID" ]; then
  pass "GET /v1/tenants/$TENANT_ID → id matches"
else
  fail "GET /v1/tenants/$TENANT_ID → id mismatch: $GOT_TENANT"
fi

# ── 4. Create identity ────────────────────────────────────────────────────────
info "4. Create identity"
HANDLE="smoke-user-$TS"
IDENTITY=$(curl -s -X POST "$BASE_URL/v1/identities" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"type\":\"human\",\"display_name\":\"Smoke User\",\"handle\":\"$HANDLE\"}")
IDENTITY_ID=$(echo "$IDENTITY" | python3 -c "import sys,json; d=json.load(sys.stdin); i=d.get('identity',{}); print(i.get('id',''))" 2>/dev/null || echo "")
IDENTITY_STATUS=$(echo "$IDENTITY" | python3 -c "import sys,json; d=json.load(sys.stdin); i=d.get('identity',{}); print(i.get('status',''))" 2>/dev/null || echo "")
if [ -n "$IDENTITY_ID" ] && [ "$IDENTITY_ID" != "null" ]; then
  pass "POST /v1/identities → id=$IDENTITY_ID"
else
  fail "POST /v1/identities → no id in response: $IDENTITY"
fi

# ── 5. Verify identity status is active ───────────────────────────────────────
info "5. Identity status check"
if [ "$IDENTITY_STATUS" = "active" ]; then
  pass "Identity status = active (not pending)"
else
  fail "Identity status = '$IDENTITY_STATUS' (expected 'active') — UpdateStatus bug?"
fi

# ── 6. Verify primary key was generated ───────────────────────────────────────
info "6. Primary key check"
PRIMARY_KEY_ID=$(echo "$IDENTITY" | python3 -c "import sys,json; d=json.load(sys.stdin); i=d.get('identity',{}); print(i.get('primary_key_id',''))" 2>/dev/null || echo "")
KEYS=$(echo "$IDENTITY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('keys',[])))" 2>/dev/null || echo "0")
if [ -n "$PRIMARY_KEY_ID" ] && [ "$PRIMARY_KEY_ID" != "null" ] && [ "$KEYS" -gt "0" ]; then
  pass "Primary key generated → key_id=$PRIMARY_KEY_ID, keys_count=$KEYS"
else
  fail "Primary key missing → primary_key_id=$PRIMARY_KEY_ID, keys=$KEYS"
fi

# ── 7. Get identity ───────────────────────────────────────────────────────────
info "7. Get identity"
GOT_IDENTITY=$(curl -s "$BASE_URL/v1/identities/$IDENTITY_ID?tenant_id=$TENANT_ID")
GOT_IDENTITY_ID=$(echo "$GOT_IDENTITY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")
if [ "$GOT_IDENTITY_ID" = "$IDENTITY_ID" ]; then
  pass "GET /v1/identities/$IDENTITY_ID → id matches"
else
  fail "GET /v1/identities/$IDENTITY_ID → $GOT_IDENTITY"
fi

# ── 8. Get identity by handle ─────────────────────────────────────────────────
info "8. Get identity by handle"
GOT_BY_HANDLE=$(curl -s "$BASE_URL/v1/identities/handle/$HANDLE?tenant_id=$TENANT_ID")
GOT_HANDLE_ID=$(echo "$GOT_BY_HANDLE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")
if [ "$GOT_HANDLE_ID" = "$IDENTITY_ID" ]; then
  pass "GET /v1/identities/handle/$HANDLE → id matches"
else
  fail "GET /v1/identities/handle/$HANDLE → $GOT_BY_HANDLE"
fi

# ── 9. Generate additional key ────────────────────────────────────────────────
info "9. Generate key"
KEY=$(curl -s -X POST "$BASE_URL/v1/keys" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"identity_id\":\"$IDENTITY_ID\",\"key_type\":\"ecdsa_p256\",\"purpose\":\"authentication\"}")
KEY_ID=$(echo "$KEY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")
if [ -n "$KEY_ID" ] && [ "$KEY_ID" != "null" ]; then
  pass "POST /v1/keys → key_id=$KEY_ID"
else
  fail "POST /v1/keys → $KEY"
fi

# ── 10. CORS preflight ────────────────────────────────────────────────────────
info "10. CORS preflight"
CORS_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$BASE_URL/v1/tenants" \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST")
if [ "$CORS_STATUS" = "204" ]; then
  pass "OPTIONS /v1/tenants → 204"
else
  fail "OPTIONS /v1/tenants → expected 204, got $CORS_STATUS"
fi

# ── 11. Suspend identity ──────────────────────────────────────────────────────
info "11. Suspend identity"
SUSPEND=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  "$BASE_URL/v1/identities/$IDENTITY_ID/suspend" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\"}")
if [ "$SUSPEND" = "200" ] || [ "$SUSPEND" = "204" ]; then
  pass "POST /v1/identities/$IDENTITY_ID/suspend → $SUSPEND"
else
  fail "POST /v1/identities/$IDENTITY_ID/suspend → $SUSPEND"
fi

# ── 12. Reactivate identity ───────────────────────────────────────────────────
info "12. Reactivate identity"
REACTIVATE=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  "$BASE_URL/v1/identities/$IDENTITY_ID/reactivate" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\"}")
if [ "$REACTIVATE" = "200" ] || [ "$REACTIVATE" = "204" ]; then
  pass "POST /v1/identities/$IDENTITY_ID/reactivate → $REACTIVATE"
else
  fail "POST /v1/identities/$IDENTITY_ID/reactivate → $REACTIVATE"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
TOTAL=$((PASS+FAIL))
if [ "$FAIL" -eq 0 ]; then
  echo -e "║  ${GREEN}✓ ALL $TOTAL TESTS PASSED${NC}                                  ║"
else
  echo -e "║  ${RED}✗ $FAIL/$TOTAL TESTS FAILED${NC}                                 ║"
fi
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
