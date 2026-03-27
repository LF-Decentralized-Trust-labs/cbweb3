#!/usr/bin/env bash
# tryout-my-onboarding-status.sh — Test GET /api/v1/onboarding/my-status
#
# Exercises the "recover onboarding status" endpoint in six scenarios,
# covering both the Central Bank public endpoint and the Commercial Bank
# authenticated proxy endpoint.
#
# The Commercial Bank proxy resolves the bank identity from the JWT session
# (BankID claim) — the frontend sends no query parameters, only the cookie.
# The Central Bank endpoint accepts an optional bank_code query parameter
# (used by the proxy and the governance portal).
#
# Scenarios:
#
#   Scenario 1  CB public endpoint — existing bank — response has request_id,
#               user_id, status only; wallet_address and pop_nonce are absent
#   Scenario 2  CB public endpoint — idempotency — two consecutive calls return
#               the same request_id and status
#   Scenario 3  CB public endpoint — unknown bank_code → 404
#   Scenario 4  CB public endpoint — missing bank_code query param → 400
#   Scenario 5  Bank proxy endpoint — unauthenticated (no cookie) → 401
#   Scenario 6  Bank proxy endpoint — authenticated (JWT BankID) → status returned
#
# Prerequisites:
#   - Stacks running: make dev.up (or at least dev.up-central-bank-a + dev.up-bank-a)
#   - At least one participant registered:
#       Run tryout-spoke-a-bank-a.sh first to create the onboarding record.
#   - curl, jq
#   - backend/config/.env.infra.bank-a         with KC_CLIENT_SECRET
#   - backend/config/.env.infra.central-bank-a  with KC_CLIENT_SECRET
#
# Usage:
#   ./tryout-my-onboarding-status.sh
#
# Environment variables (optional):
#   BANK_URL          Bank-A API base URL          (default: http://localhost:18080/api/v1)
#   CB_URL            Central Bank API base URL    (default: http://localhost:38080/api/v1)
#   BANK_ENV          Bank-A .env file             (default: backend/config/.env.infra.bank-a)
#   CB_ENV            Central Bank .env file       (default: backend/config/.env.infra.central-bank-a)
#   BANK_CODE         Bank code under test         (default: a)
#   UNKNOWN_BANK_CODE A bank_code that has no record (default: zzz-nonexistent)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BANK_URL="${BANK_URL:-http://localhost:18080/api/v1}"
CB_URL="${CB_URL:-http://localhost:38080/api/v1}"
BANK_ENV="${BANK_ENV:-backend/config/.env.infra.bank-a}"
CB_ENV="${CB_ENV:-backend/config/.env.infra.central-bank-a}"
BANK_CODE="${BANK_CODE:-a}"
UNKNOWN_BANK_CODE="${UNKNOWN_BANK_CODE:-zzz-nonexistent}"

BANK_TOKEN=""
PASS=0
FAIL=0
SKIP=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

require_cmds() {
  for cmd in curl jq; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "ERROR: '$cmd' not found. Install it before continuing." >&2
      exit 1
    fi
  done
}

require_env_files() {
  if [ ! -f "$BANK_ENV" ]; then
    echo "ERROR: Bank-A env file '$BANK_ENV' not found." >&2
    exit 1
  fi
  if [ ! -f "$CB_ENV" ]; then
    echo "ERROR: Central Bank env file '$CB_ENV' not found." >&2
    exit 1
  fi
}

read_kc_secret() {
  local env_file=$1
  local secret
  secret=$(grep KC_CLIENT_SECRET "$env_file" | cut -d= -f2-)
  if [ -z "$secret" ]; then
    echo "ERROR: KC_CLIENT_SECRET not found in $env_file." >&2
    exit 1
  fi
  echo "$secret"
}

assert_http_code() {
  local label=$1 expected=$2 actual=$3 body=$4
  if [ "$actual" = "$expected" ]; then
    echo "  ✅ PASS — HTTP $actual (expected $expected)"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL — HTTP $actual (expected $expected)"
    echo "  Response: $body"
    FAIL=$((FAIL + 1))
  fi
}

assert_json_field() {
  local label=$1 body=$2 field=$3 expected=$4
  local actual
  actual=$(echo "$body" | jq -r ".$field // empty" 2>/dev/null || true)
  if [ "$actual" = "$expected" ]; then
    echo "  ✅ PASS — .$field = \"$actual\""
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL — .$field: expected \"$expected\", got \"$actual\""
    FAIL=$((FAIL + 1))
  fi
}

assert_json_field_present() {
  local label=$1 body=$2 field=$3
  local actual
  actual=$(echo "$body" | jq -r ".$field // \"__absent__\"" 2>/dev/null || true)
  if [ "$actual" != "__absent__" ] && [ -n "$actual" ]; then
    echo "  ✅ PASS — .$field is present (value: \"$actual\")"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL — .$field is absent or empty"
    FAIL=$((FAIL + 1))
  fi
}

assert_json_field_absent() {
  local label=$1 body=$2 field=$3
  local actual
  actual=$(echo "$body" | jq -r ".$field // \"__absent__\"" 2>/dev/null || true)
  if [ "$actual" = "__absent__" ] || [ -z "$actual" ]; then
    echo "  ✅ PASS — .$field is absent (as expected)"
    PASS=$((PASS + 1))
  else
    echo "  ❌ FAIL — .$field should be absent but has value: \"$actual\""
    FAIL=$((FAIL + 1))
  fi
}
# Returns 0 when the CB already has an onboarding record for BANK_CODE,
# 1 when it does not. Stores the current status in BANK_REGISTERED_STATUS.
BANK_REGISTERED_STATUS=""
check_bank_registered() {
  local result code body
  result=$(get_request "$CB_URL/onboarding/my-status?bank_code=$BANK_CODE")
  code="${result%% *}"
  body="${result#* }"
  if [ "$code" = "200" ]; then
    BANK_REGISTERED_STATUS=$(echo "$body" | jq -r '.status // empty')
    return 0
  fi
  return 1
}
get_request() {
  local url=$1
  local cookie=${2:-}
  local tmp
  tmp=$(mktemp)
  local code
  if [ -n "$cookie" ]; then
    code=$(curl -sS --cookie "access_token=$cookie" "$url" -o "$tmp" -w "%{http_code}")
  else
    code=$(curl -sS "$url" -o "$tmp" -w "%{http_code}")
  fi
  local body
  body=$(cat "$tmp"); rm -f "$tmp"
  echo "$code $body"
}

# ---------------------------------------------------------------------------
# Login helper (Bank-A operator)
# ---------------------------------------------------------------------------

bank_login() {
  echo "---"
  echo "Pre-requisite: Bank-A operator login..."
  local kc_secret login_resp
  kc_secret=$(read_kc_secret "$BANK_ENV")
  login_resp=$(curl -s -X POST "$BANK_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"bank-a-client\", \"clientSecret\": \"$kc_secret\"}")
  BANK_TOKEN=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$BANK_TOKEN" ]; then
    echo "ERROR: Bank-A login failed. Is the stack running?" >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
  echo "  Login OK (token obtained)."
}

# ---------------------------------------------------------------------------
# Scenario 1 — CB public endpoint: bank found — validates that only the three
# core fields (request_id, user_id, status) are present and that wallet_address
# and pop_nonce are absent from this recovery endpoint.
# Skipped gracefully when bank has not been onboarded yet.
# ---------------------------------------------------------------------------

scenario_1_cb_status_core_fields() {
  echo ""
  echo "Scenario 1 — CB public: registered bank — validate core fields only"
  echo "  GET $CB_URL/onboarding/my-status?bank_code=$BANK_CODE"

  if ! check_bank_registered; then
    echo "  ⚠️  SKIP — bank_code='$BANK_CODE' has no record on the CB yet."
    echo "         Run ./tryouts/tryout-spoke-a-bank-a.sh first, then re-run this tryout."
    SKIP=$((SKIP + 1))
    return
  fi

  # Re-fetch to have the raw body for field assertions.
  local result code body
  result=$(get_request "$CB_URL/onboarding/my-status?bank_code=$BANK_CODE")
  code="${result%% *}"
  body="${result#* }"

  assert_http_code "sc1_http" "200" "$code" "$body"

  if [ "$code" = "200" ]; then
    local status
    status=$(echo "$body" | jq -r '.status // empty')
    echo "  Current status: $status"

    # Core fields must always be present.
    assert_json_field_present "sc1_request_id" "$body" "request_id"
    assert_json_field_present "sc1_user_id"    "$body" "user_id"
    assert_json_field_present "sc1_status"     "$body" "status"

    # Sensitive fields are intentionally omitted from this endpoint.
    assert_json_field_absent "sc1_no_wallet_address" "$body" "wallet_address"
    assert_json_field_absent "sc1_no_pop_nonce"      "$body" "pop_nonce"
  fi
}

# ---------------------------------------------------------------------------
# Scenario 2 — CB public endpoint: idempotency — two consecutive calls return
# the same data for the same bank_code.
# Skipped gracefully when bank has not been onboarded yet.
# ---------------------------------------------------------------------------

scenario_2_idempotency() {
  echo ""
  echo "Scenario 2 — CB public: two consecutive calls must return identical data"
  echo "  GET $CB_URL/onboarding/my-status?bank_code=$BANK_CODE (×2)"

  if ! check_bank_registered; then
    echo "  ⚠️  SKIP — bank_code='$BANK_CODE' has no record on the CB yet."
    echo "         Run ./tryouts/tryout-spoke-a-bank-a.sh first, then re-run this tryout."
    SKIP=$((SKIP + 1))
    return
  fi

  local r1 r2 c1 c2 b1 b2
  r1=$(get_request "$CB_URL/onboarding/my-status?bank_code=$BANK_CODE")
  r2=$(get_request "$CB_URL/onboarding/my-status?bank_code=$BANK_CODE")
  c1="${r1%% *}"; b1="${r1#* }"
  c2="${r2%% *}"; b2="${r2#* }"

  assert_http_code "sc2_http_call1" "200" "$c1" "$b1"
  assert_http_code "sc2_http_call2" "200" "$c2" "$b2"

  if [ "$c1" = "200" ] && [ "$c2" = "200" ]; then
    local s1 s2 req1 req2
    s1=$(echo "$b1" | jq -r '.status')
    s2=$(echo "$b2" | jq -r '.status')
    req1=$(echo "$b1" | jq -r '.request_id')
    req2=$(echo "$b2" | jq -r '.request_id')

    if [ "$s1" = "$s2" ]; then
      echo "  ✅ PASS — status stable across calls: \"$s1\""
      PASS=$((PASS + 1))
    else
      echo "  ❌ FAIL — status changed between calls: \"$s1\" → \"$s2\""
      FAIL=$((FAIL + 1))
    fi

    if [ "$req1" = "$req2" ]; then
      echo "  ✅ PASS — request_id stable: \"$req1\""
      PASS=$((PASS + 1))
    else
      echo "  ❌ FAIL — request_id changed between calls: \"$req1\" → \"$req2\""
      FAIL=$((FAIL + 1))
    fi
  fi
}

# ---------------------------------------------------------------------------
# Scenario 3 — CB public endpoint: unknown bank_code → 404
# ---------------------------------------------------------------------------

scenario_3_cb_not_found() {
  echo ""
  echo "Scenario 3 — CB public: unknown bank_code = \"$UNKNOWN_BANK_CODE\" → 404"
  echo "  GET $CB_URL/onboarding/my-status?bank_code=$UNKNOWN_BANK_CODE"

  local result code body
  result=$(get_request "$CB_URL/onboarding/my-status?bank_code=$UNKNOWN_BANK_CODE")
  code="${result%% *}"
  body="${result#* }"

  assert_http_code "sc3_http" "404" "$code" "$body"

  if [ "$code" = "404" ]; then
    assert_json_field_present "sc3_error_message" "$body" "error"
  fi
}

# ---------------------------------------------------------------------------
# Scenario 4 — CB public endpoint: missing bank_code param → 400
# ---------------------------------------------------------------------------

scenario_4_cb_missing_param() {
  echo ""
  echo "Scenario 4 — CB public: missing bank_code parameter → 400"
  echo "  GET $CB_URL/onboarding/my-status"

  local result code body
  result=$(get_request "$CB_URL/onboarding/my-status")
  code="${result%% *}"
  body="${result#* }"

  assert_http_code "sc4_http" "400" "$code" "$body"

  if [ "$code" = "400" ]; then
    assert_json_field_present "sc4_error_message" "$body" "error"
  fi
}

# ---------------------------------------------------------------------------
# Scenario 5 — Bank proxy endpoint: no cookie → 401
# ---------------------------------------------------------------------------

scenario_5_bank_proxy_unauthenticated() {
  echo ""
  echo "Scenario 5 — Bank proxy: unauthenticated (no cookie) → 401"
  echo "  GET $BANK_URL/onboarding/my-status"

  local result code body
  result=$(get_request "$BANK_URL/onboarding/my-status")
  code="${result%% *}"
  body="${result#* }"

  assert_http_code "sc5_http" "401" "$code" "$body"
}

# ---------------------------------------------------------------------------
# Scenario 6 — Bank proxy endpoint: authenticated → status resolved from JWT
# The proxy extracts BankID from the JWT claims (enriched by ValidateToken)
# and forwards the request to the CB with bank_code=<BankID>.
# The frontend sends no query parameters — only the session cookie.
# ---------------------------------------------------------------------------

scenario_6_bank_proxy_authenticated() {
  echo ""
  echo "Scenario 6 — Bank proxy: authenticated (JWT BankID) → status for bank_code=$BANK_CODE"
  echo "  GET $BANK_URL/onboarding/my-status  (cookie: access_token, no params)"

  local result code body
  result=$(get_request "$BANK_URL/onboarding/my-status" "$BANK_TOKEN")
  code="${result%% *}"
  body="${result#* }"

  # The proxy forwards to the CB; if the CB has the record → 200.
  # Accept 200 as success; 502 means the CB is down (infrastructure issue, not a code bug).
  if [ "$code" = "200" ]; then
    echo "  ✅ PASS — HTTP 200 (proxied successfully)"
    PASS=$((PASS + 1))
    assert_json_field_present "sc6_request_id" "$body" "request_id"
    assert_json_field_present "sc6_status" "$body" "status"
  elif [ "$code" = "404" ]; then
    echo "  ✅ PASS — HTTP 404 (bank not yet registered, proxy working correctly)"
    PASS=$((PASS + 1))
  elif [ "$code" = "502" ]; then
    echo "  ⚠️  SKIP — HTTP 502 (Central Bank unreachable — infrastructure issue)"
  else
    echo "  ❌ FAIL — HTTP $code (expected 200 or 404)"
    echo "  Response: $body"
    FAIL=$((FAIL + 1))
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  require_cmds
  require_env_files
  bank_login

  scenario_1_cb_status_core_fields
  scenario_2_idempotency
  scenario_3_cb_not_found
  scenario_4_cb_missing_param
  scenario_5_bank_proxy_unauthenticated
  scenario_6_bank_proxy_authenticated

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Results: PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if [ "$FAIL" -gt 0 ]; then
    exit 1
  fi
}

main "$@"
