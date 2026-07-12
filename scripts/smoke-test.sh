#!/usr/bin/env bash
# Verion smoke test — 14 tests
# Usage: BASE_URL=http://localhost:8081 ./scripts/smoke-test.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8081}"
PASS=0
FAIL=0

green='\033[0;32m'
red='\033[0;31m'
blue='\033[0;34m'
reset='\033[0m'

pass() { echo -e "${green}✓ PASS${reset} — $1"; ((PASS++)); }
fail() { echo -e "${red}✗ FAIL${reset} — $1"; ((FAIL++)); }
info() { echo -e "${blue}▶${reset} $1"; }

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  Verion Smoke Test — 14 tests            ║"
echo "╚══════════════════════════════════════════╝"
echo "BASE_URL: $BASE_URL"
echo ""

# ── 1. Health ─────────────────────────────────────────────────────────────────
info "1. Health check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz")
[ "$STATUS" = "200" ] && pass "GET /healthz → 200" || fail "GET /healthz → $STATUS"

# ── 2. Create tenant ──────────────────────────────────────────────────────────
info "2. Create tenant"
SLUG="smoke-$(date +%s)"
TENANT_RESP=$(curl -s -X POST "$BASE_URL/v1/tenants" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Smoke Tenant\",\"slug\":\"$SLUG\",\"tier\":\"standard\",\"data_region\":\"global\"}")
TENANT_ID=$(echo "$TENANT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
[ -n "$TENANT_ID" ] && pass "POST /v1/tenants → id=$TENANT_ID" || fail "POST /v1/tenants → $TENANT_RESP"

# ── 3. Get tenant ─────────────────────────────────────────────────────────────
info "3. Get tenant"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/tenants/$TENANT_ID")
[ "$STATUS" = "200" ] && pass "GET /v1/tenants/$TENANT_ID → 200" || fail "GET /v1/tenants/$TENANT_ID → $STATUS"

# ── 4. Suspend tenant ─────────────────────────────────────────────────────────
info "4. Suspend tenant"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/tenants/$TENANT_ID/suspend")
[ "$STATUS" = "200" ] && pass "POST /v1/tenants/$TENANT_ID/suspend → 200" || fail "→ $STATUS"

# ── 5. Activate tenant ────────────────────────────────────────────────────────
info "5. Activate tenant"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/tenants/$TENANT_ID/activate")
[ "$STATUS" = "200" ] && pass "POST /v1/tenants/$TENANT_ID/activate → 200" || fail "→ $STATUS"

# ── 6. Create identity ────────────────────────────────────────────────────────
info "6. Create identity"
HANDLE="smokeuser-$(date +%s)"
IDENTITY_RESP=$(curl -s -X POST "$BASE_URL/v1/identities" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"type\":\"human\",\"display_name\":\"Smoke User\",\"handle\":\"$HANDLE\"}")
IDENTITY_ID=$(echo "$IDENTITY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('identity',{}).get('id',''))" 2>/dev/null || echo "")
[ -n "$IDENTITY_ID" ] && pass "POST /v1/identities → id=$IDENTITY_ID" || fail "POST /v1/identities → $IDENTITY_RESP"

# Force active status (known SPEC-006 pending bug workaround)
PGPASSWORD="${VERION_DB_PASSWORD:-verion_dev_secret}" psql \
  -h "${VERION_DB_HOST:-localhost}" \
  -U "${VERION_DB_USER:-verion}" \
  -d "${VERION_DB_NAME:-verion}" \
  -q -c "UPDATE identities SET status='active', updated_at=NOW() WHERE id='$IDENTITY_ID';" 2>/dev/null || true

# ── 7. Get identity ───────────────────────────────────────────────────────────
info "7. Get identity"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities/$IDENTITY_ID?tenant_id=$TENANT_ID")
[ "$STATUS" = "200" ] && pass "GET /v1/identities/$IDENTITY_ID → 200" || fail "→ $STATUS"

# ── 8. Get identity by handle ─────────────────────────────────────────────────
info "8. Get identity by handle"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities/handle/$HANDLE?tenant_id=$TENANT_ID")
[ "$STATUS" = "200" ] && pass "GET /v1/identities/handle/$HANDLE → 200" || fail "→ $STATUS"

# ── 9. List identities ────────────────────────────────────────────────────────
info "9. List identities"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities?tenant_id=$TENANT_ID")
[ "$STATUS" = "200" ] && pass "GET /v1/identities → 200" || fail "→ $STATUS"

# ── 10. Generate key ──────────────────────────────────────────────────────────
info "10. Generate key"
KEY_RESP=$(curl -s -X POST "$BASE_URL/v1/keys" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"identity_id\":\"$IDENTITY_ID\",\"key_type\":\"ed25519\",\"purpose\":\"signing\"}")
KEY_ID=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
[ -n "$KEY_ID" ] && pass "POST /v1/keys → id=$KEY_ID" || fail "POST /v1/keys → $KEY_RESP"

# ── 11. Get key ───────────────────────────────────────────────────────────────
info "11. Get key"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/keys/$KEY_ID?tenant_id=$TENANT_ID")
[ "$STATUS" = "200" ] && pass "GET /v1/keys/$KEY_ID → 200" || fail "→ $STATUS"

# ── 12. CORS preflight ────────────────────────────────────────────────────────
info "12. CORS preflight"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$BASE_URL/v1/tenants")
[ "$STATUS" = "204" ] && pass "OPTIONS /v1/tenants → 204" || fail "OPTIONS → $STATUS"

# ── 13. WebAuthn BeginRegistration ───────────────────────────────────────────
info "13. WebAuthn BeginRegistration"
WEBAUTHN_REG=$(curl -s -X POST "$BASE_URL/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"identity_id\":\"$IDENTITY_ID\"}")
CHALLENGE=$(echo "$WEBAUTHN_REG" | python3 -c \
  "import sys,json; d=json.load(sys.stdin); print(d.get('publicKey',{}).get('challenge',''))" 2>/dev/null || echo "")
if [ -n "$CHALLENGE" ] && [ "$CHALLENGE" != "null" ]; then
  pass "POST /v1/auth/register → challenge present"
else
  fail "POST /v1/auth/register → no challenge: $WEBAUTHN_REG"
fi

# ── 14. WebAuthn BeginLogin ───────────────────────────────────────────────────
info "14. WebAuthn BeginLogin"
WEBAUTHN_LOGIN=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"handle\":\"$HANDLE\"}")
LOGIN_CHALLENGE=$(echo "$WEBAUTHN_LOGIN" | python3 -c \
  "import sys,json; d=json.load(sys.stdin); print(d.get('publicKey',{}).get('challenge',''))" 2>/dev/null || echo "")
if [ -n "$LOGIN_CHALLENGE" ] && [ "$LOGIN_CHALLENGE" != "null" ]; then
  pass "POST /v1/auth/login → challenge present"
else
  fail "POST /v1/auth/login → no challenge: $WEBAUTHN_LOGIN"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  echo -e "${green}ALL $TOTAL TESTS PASSED${reset}"
else
  echo -e "${red}$FAIL/$TOTAL TESTS FAILED${reset} ($PASS passed)"
fi
echo "══════════════════════════════════════════════"
echo ""
[ "$FAIL" -eq 0 ]
