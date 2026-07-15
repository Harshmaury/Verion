#!/usr/bin/env bash
# Verion smoke test — 15 tests
set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8081}"
PASS=0
FAIL=0

green='\033[0;32m'
red='\033[0;31m'
blue='\033[0;34m'
reset='\033[0m'

pass() { echo -e "${green}✓ PASS${reset} — $1"; PASS=$((PASS+1)); }
fail() { echo -e "${red}✗ FAIL${reset} — $1"; FAIL=$((FAIL+1)); }
info() { echo -e "${blue}▶${reset} $1"; }

check() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    pass "$desc"
  else
    fail "$desc (expected=$expected got=$actual)"
  fi
}

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  Verion Smoke Test — 15 tests            ║"
echo "╚══════════════════════════════════════════╝"
echo "BASE_URL: $BASE_URL"
echo ""

info "1. Health check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz")
check "GET /healthz → 200" "200" "$STATUS"

info "2. Create tenant"
SLUG="smoke-$(date +%s)"
TENANT_RESP=$(curl -s -X POST "$BASE_URL/v1/tenants" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Smoke Tenant\",\"slug\":\"$SLUG\",\"tier\":\"standard\",\"data_region\":\"global\"}")
TENANT_ID=$(echo "$TENANT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
if [ -n "$TENANT_ID" ] && [ "$TENANT_ID" != "None" ]; then
  pass "POST /v1/tenants → id=$TENANT_ID"
else
  fail "POST /v1/tenants → $TENANT_RESP"; exit 1
fi

info "3. Create identity (no auth — create tenant is public)"
HANDLE="smokeuser-$(date +%s)"
IDENTITY_RESP=$(curl -s -X POST "$BASE_URL/v1/identities" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"type\":\"human\",\"display_name\":\"Smoke User\",\"handle\":\"$HANDLE\"}")
IDENTITY_ID=$(echo "$IDENTITY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('identity',{}).get('id',''))" 2>/dev/null || echo "")
if [ -n "$IDENTITY_ID" ] && [ "$IDENTITY_ID" != "None" ]; then
  pass "POST /v1/identities → id=$IDENTITY_ID"
else
  fail "POST /v1/identities → $IDENTITY_RESP"; exit 1
fi

# Workaround: force active status (SPEC-006 pending bug)
PGPASSWORD="${VERION_DB_PASSWORD:-verion_dev_secret}" psql \
  -h "${VERION_DB_HOST:-localhost}" -U "${VERION_DB_USER:-verion}" \
  -d "${VERION_DB_NAME:-verion}" -q \
  -c "UPDATE identities SET status='active', updated_at=NOW() WHERE id='$IDENTITY_ID';" 2>/dev/null || true

info "4. Get tenant (protected — no token → 401)"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/tenants/$TENANT_ID")
check "GET /v1/tenants/$TENANT_ID without token → 401" "401" "$STATUS"

info "5. WebAuthn BeginRegistration (public)"
WEBAUTHN_REG=$(curl -s -X POST "$BASE_URL/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"identity_id\":\"$IDENTITY_ID\"}")
CHALLENGE=$(echo "$WEBAUTHN_REG" | python3 -c \
  "import sys,json; d=json.load(sys.stdin); print(d.get('publicKey',{}).get('challenge',''))" 2>/dev/null || echo "")
if [ -n "$CHALLENGE" ] && [ "$CHALLENGE" != "None" ]; then
  pass "POST /v1/auth/register → challenge present"
else
  fail "POST /v1/auth/register → no challenge: $WEBAUTHN_REG"
fi

info "6. WebAuthn BeginLogin (public)"
WEBAUTHN_LOGIN=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"handle\":\"$HANDLE\"}")
LOGIN_CHALLENGE=$(echo "$WEBAUTHN_LOGIN" | python3 -c \
  "import sys,json; d=json.load(sys.stdin); print(d.get('publicKey',{}).get('challenge',''))" 2>/dev/null || echo "")
if [ -n "$LOGIN_CHALLENGE" ] && [ "$LOGIN_CHALLENGE" != "None" ]; then
  pass "POST /v1/auth/login → challenge present"
else
  fail "POST /v1/auth/login → no challenge: $WEBAUTHN_LOGIN"
fi

info "7. CORS preflight (public)"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$BASE_URL/v1/tenants")
check "OPTIONS /v1/tenants → 204" "204" "$STATUS"

# Issue a test token via a small Go program for integration testing
# Since we can't do full WebAuthn in CI, we test the JWT issue/verify path
# by confirming protected routes require auth and return 401 without it.

info "8. Protected: GET /v1/identities without token → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities?tenant_id=$TENANT_ID")
check "GET /v1/identities without token → 401" "401" "$STATUS"

info "9. Protected: GET /v1/identities/{id} without token → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities/$IDENTITY_ID?tenant_id=$TENANT_ID")
check "GET /v1/identities/$IDENTITY_ID without token → 401" "401" "$STATUS"

info "10. Protected: POST /v1/keys without token → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/keys" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\"}")
check "POST /v1/keys without token → 401" "401" "$STATUS"

info "11. Protected: POST /v1/keys/fake-key/rotate without token → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/keys/fake-key/rotate" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\"}")
check "POST /v1/identities without token → 401" "401" "$STATUS"

info "12. Invalid Bearer token → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities?tenant_id=$TENANT_ID" \
  -H "Authorization: Bearer notavalidtoken")
check "GET /v1/identities with bad token → 401" "401" "$STATUS"

info "13. Malformed Authorization header → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities?tenant_id=$TENANT_ID" \
  -H "Authorization: Basic dXNlcjpwYXNz")
check "GET /v1/identities with Basic auth → 401" "401" "$STATUS"

info "14. Suspend tenant without token → 401"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/tenants/$TENANT_ID/suspend")
check "POST /v1/tenants/$TENANT_ID/suspend without token → 401" "401" "$STATUS"

info "15. Auth middleware enforcement — protected route rejects unauthenticated request"
UNAUTH=$(curl -s -o /dev/null -w "%{http_code}" \
  "$BASE_URL/v1/identities/$IDENTITY_ID?tenant_id=$TENANT_ID")
if [ "$UNAUTH" = "401" ]; then
  pass "GET /v1/identities without token → 401"
else
  fail "GET /v1/identities without token → expected 401, got $UNAUTH"
fi

echo ""
echo "══════════════════════════════════════════════"
TOTAL=$((PASS+FAIL))
if [ "$FAIL" -eq 0 ]; then
  echo -e "${green}ALL $TOTAL TESTS PASSED${reset}"
else
  echo -e "${red}$FAIL/$TOTAL TESTS FAILED${reset} ($PASS passed)"
fi
echo "══════════════════════════════════════════════"
echo ""
exit $FAIL
