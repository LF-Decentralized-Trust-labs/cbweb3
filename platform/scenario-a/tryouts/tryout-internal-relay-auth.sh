#!/usr/bin/env bash
# tryout-internal-relay-auth.sh — Verify X-Relay-Auth protection on /internal/v1 endpoints
#
# Tests that all internal service-to-service endpoints:
#   1) Reject requests with NO X-Relay-Auth header           → 401
#   2) Reject requests with an INVALID X-Relay-Auth secret   → 401
#   3) Accept requests with the CORRECT X-Relay-Auth secret  → 200
#
# Endpoints tested:
#   GET  /internal/v1/payments/fx/agreements     (relay polls FX state)
#   GET  /internal/v1/payments/deposits          (proxy from commercial bank)
#   GET  /internal/v1/payments/escrows           (proxy from commercial bank)
#   GET  /internal/v1/payments/redeems           (proxy from commercial bank)
#
# Notes:
#   - deposit/escrow/redeem internal routes only exist on Central Bank gateways.
#   - FX internal route exists on commercial bank gateways.
#
# Usage:
#   ./tryout-internal-relay-auth.sh
#
# Environment variables (optional):
#   BANK_A_URL       Bank-A gateway         (default: http://localhost:18080)
#   CB_A_URL         Central Bank-A gateway (default: http://localhost:38080)
#   RELAY_SECRET     Correct shared secret  (default: read from .env.infra.bank-a)

set -euo pipefail

BANK_A_URL="${BANK_A_URL:-http://localhost:18080}"
CB_A_URL="${CB_A_URL:-http://localhost:38080}"
BANK_A_ENV="${BANK_A_ENV:-backend/config/.env.infra.bank-a}"
CB_A_ENV="${CB_A_ENV:-backend/config/.env.infra.central-bank-a}"

WRONG_SECRET="definitely-wrong-secret-$(date +%s)"
PASS=0
FAIL=0
TOTAL=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

log_step()  { echo "" >&2; echo "=== $* ===" >&2; }
log_ok()    { PASS=$((PASS + 1)); TOTAL=$((TOTAL + 1)); echo "  [PASS] $*" >&2; }
log_fail()  { FAIL=$((FAIL + 1)); TOTAL=$((TOTAL + 1)); echo "  [FAIL] $*" >&2; }
log_info()  { echo "  [INFO] $*" >&2; }

read_relay_secret() {
  local env_file=$1
  if [ ! -f "$env_file" ]; then
    echo ""
    return
  fi
  grep -E '^INTERNAL_RELAY_AUTH_SECRET=' "$env_file" 2>/dev/null | head -1 | cut -d= -f2- || true
}

# assert_http CODE URL [HEADERS...]
# Sends GET and checks the HTTP status code matches.
assert_http() {
  local expected=$1 url=$2
  shift 2
  local code body tmp
  tmp=$(mktemp)
  code=$(curl -sS -X GET "$url" \
    --connect-timeout 5 \
    --max-time 10 \
    "$@" \
    -o "$tmp" -w "%{http_code}" 2>/dev/null || echo "000")
  body=$(cat "$tmp" 2>/dev/null || true)
  rm -f "$tmp"

  if [ "$code" = "$expected" ]; then
    log_ok "GET $url → HTTP $code (expected $expected)"
  else
    log_fail "GET $url → HTTP $code (expected $expected) | body: ${body:0:200}"
  fi
}

# ---------------------------------------------------------------------------
# Resolve relay secret
# ---------------------------------------------------------------------------

RELAY_SECRET="${RELAY_SECRET:-}"
if [ -z "$RELAY_SECRET" ]; then
  RELAY_SECRET=$(read_relay_secret "$BANK_A_ENV")
fi
if [ -z "$RELAY_SECRET" ]; then
  RELAY_SECRET=$(read_relay_secret "$CB_A_ENV")
fi
if [ -z "$RELAY_SECRET" ]; then
  echo "[ERROR] Could not resolve INTERNAL_RELAY_AUTH_SECRET from env files." >&2
  echo "        Set RELAY_SECRET env var or ensure .env.infra files contain the key." >&2
  exit 1
fi

log_info "Relay secret resolved (${#RELAY_SECRET} chars)"
log_info "Bank-A gateway: $BANK_A_URL"
log_info "CB-A gateway:   $CB_A_URL"

# ---------------------------------------------------------------------------
# Test 1: FX Agreements internal endpoint (Bank-A gateway)
# ---------------------------------------------------------------------------

log_step "FX Agreements internal endpoint — Bank-A ($BANK_A_URL)"

log_info "No header → expect 401"
assert_http 401 "$BANK_A_URL/internal/v1/payments/fx/agreements"

log_info "Wrong secret → expect 401"
assert_http 401 "$BANK_A_URL/internal/v1/payments/fx/agreements" \
  -H "X-Relay-Auth: $WRONG_SECRET"

log_info "Correct secret → expect 200"
assert_http 200 "$BANK_A_URL/internal/v1/payments/fx/agreements" \
  -H "X-Relay-Auth: $RELAY_SECRET"

# ---------------------------------------------------------------------------
# Test 2: FX Agreements internal endpoint (CB-A gateway)
# ---------------------------------------------------------------------------

log_step "FX Agreements internal endpoint — CB-A ($CB_A_URL)"

log_info "No header → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/fx/agreements"

log_info "Wrong secret → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/fx/agreements" \
  -H "X-Relay-Auth: $WRONG_SECRET"

log_info "Correct secret → expect 200"
assert_http 200 "$CB_A_URL/internal/v1/payments/fx/agreements" \
  -H "X-Relay-Auth: $RELAY_SECRET"

# ---------------------------------------------------------------------------
# Test 3: Deposits internal endpoint (CB-A only)
# ---------------------------------------------------------------------------

log_step "Deposits internal endpoint — CB-A ($CB_A_URL)"

log_info "No header → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/deposits"

log_info "Wrong secret → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/deposits" \
  -H "X-Relay-Auth: $WRONG_SECRET"

log_info "Correct secret → expect 200"
assert_http 200 "$CB_A_URL/internal/v1/payments/deposits" \
  -H "X-Relay-Auth: $RELAY_SECRET"

# ---------------------------------------------------------------------------
# Test 4: Escrows internal endpoint (CB-A only)
# ---------------------------------------------------------------------------

log_step "Escrows internal endpoint — CB-A ($CB_A_URL)"

log_info "No header → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/escrows"

log_info "Wrong secret → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/escrows" \
  -H "X-Relay-Auth: $WRONG_SECRET"

log_info "Correct secret → expect 200"
assert_http 200 "$CB_A_URL/internal/v1/payments/escrows" \
  -H "X-Relay-Auth: $RELAY_SECRET"

# ---------------------------------------------------------------------------
# Test 5: Redeems internal endpoint (CB-A only)
# ---------------------------------------------------------------------------

log_step "Redeems internal endpoint — CB-A ($CB_A_URL)"

log_info "No header → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/redeems"

log_info "Wrong secret → expect 401"
assert_http 401 "$CB_A_URL/internal/v1/payments/redeems" \
  -H "X-Relay-Auth: $WRONG_SECRET"

log_info "Correct secret → expect 200"
assert_http 200 "$CB_A_URL/internal/v1/payments/redeems" \
  -H "X-Relay-Auth: $RELAY_SECRET"

# ---------------------------------------------------------------------------
# Test 6: Verify commercial bank gateway does NOT expose deposit/escrow/redeem
# ---------------------------------------------------------------------------

log_step "Commercial bank (Bank-A) should NOT have deposit/escrow/redeem internal routes"

# These should return 404 (route not registered) since PaymentProxyHandler != nil
# on commercial bank gateways. If auth is hit instead of 404, it means the route
# was incorrectly registered.
log_info "Deposits on Bank-A → expect 404"
assert_http 404 "$BANK_A_URL/internal/v1/payments/deposits" \
  -H "X-Relay-Auth: $RELAY_SECRET"

log_info "Escrows on Bank-A → expect 404"
assert_http 404 "$BANK_A_URL/internal/v1/payments/escrows" \
  -H "X-Relay-Auth: $RELAY_SECRET"

log_info "Redeems on Bank-A → expect 404"
assert_http 404 "$BANK_A_URL/internal/v1/payments/redeems" \
  -H "X-Relay-Auth: $RELAY_SECRET"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

echo "" >&2
echo "============================================================" >&2
echo "  RESULTS: $PASS/$TOTAL passed, $FAIL failed" >&2
echo "============================================================" >&2

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

echo "[OK] All internal relay auth tests passed." >&2
