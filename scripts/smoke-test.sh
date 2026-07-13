#!/usr/bin/env bash
# Verion smoke test — 14 tests
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
echo "║  Verion Smoke Test — 14 tests            ║"
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

info "3. Get tenant"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/tenants/$TENANT_ID")
check "GET /v1/tenants → 200" "200" "$STATUS"

info "4. Suspend tenant"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/tenants/$TENANT_ID/suspend")
check "POST /v1/tenants/$TENANT_ID/suspend → 200" "200" "$STATUS"

info "5. Activate tenant"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/tenants/$TENANT_ID/activate")
check "POST /v1/tenants/$TENANT_ID/activate → 200" "200" "$STATUS"

info "6. Create identity"
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

PGPASSWORD="${VERION_DB_PASSWORD:-verion_dev_secret}" psql \
  -h "${VERION_DB_HOST:-localhost}" -U "${VERION_DB_USER:-verion}" \
  -d "${VERION_DB_NAME:-verion}" -q \
  -c "UPDATE identities SET status='active', updated_at=NOW() WHERE id='$IDENTITY_ID';" 2>/dev/null || true

info "7. Get identity"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities/$IDENTITY_ID?tenant_id=$TENANT_ID")
check "GET /v1/identities/$IDENTITY_ID → 200" "200" "$STATUS"

info "8. Get identity by handle"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities/handle/$HANDLE?tenant_id=$TENANT_ID")
check "GET /v1/identities/handle/$HANDLE → 200" "200" "$STATUS"

info "9. List identities"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/identities?tenant_id=$TENANT_ID")
check "GET /v1/identities → 200" "200" "$STATUS"

info "10. Generate key (recovery — no conflict with CreateIdentity signing key)"
KEY_RESP=$(curl -s -X POST "$BASE_URL/v1/keys" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"identity_id\":\"$IDENTITY_ID\",\"key_type\":\"ed25519\",\"purpose\":\"recovery\"}")
KEY_ID=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
if [ -n "$KEY_ID" ] && [ "$KEY_ID" != "None" ]; then
  pass "POST /v1/keys → id=$KEY_ID"
else
  fail "POST /v1/keys → $KEY_RESP"
  KEY_ID="missing"
fi

info "11. Get key"
if [ "$KEY_ID" != "missing" ]; then
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/keys/$KEY_ID?tenant_id=$TENANT_ID")
  check "GET /v1/keys/$KEY_ID → 200" "200" "$STATUS"
else
  fail "GET /v1/keys — skipped (no key ID from test 10)"
fi

info "12. CORS preflight"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$BASE_URL/v1/tenants")
check "OPTIONS /v1/tenants → 204" "204" "$STATUS"

info "13. WebAuthn BeginRegistration"
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

info "14. WebAuthn BeginLogin"
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
