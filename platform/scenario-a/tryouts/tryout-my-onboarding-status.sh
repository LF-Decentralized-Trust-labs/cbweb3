#!/usr/bin/env bash
# tryout-my-onboarding-status.sh — Test GET /api/v1/onboarding/my-status
#
# Self-contained tryout that exercises the onboarding status endpoint via the
# Commercial Bank proxy — the same flow the frontend uses.
#
# Sequence:
#
#   Step 1  Login — authenticate as Bank-A operator
#   Step 2  Check status — no pending request → {"status":"NONE"}
#   Step 3  Submit credential request → creates onboarding record
#   Step 4  Check status — pending request → status + request_id returned
#
# The frontend calls the Bank proxy with only the JWT session cookie (no query
# parameters). The proxy resolves BankID from the JWT and forwards to the
# Central Bank with bank_code resolved server-side.
#
# Prerequisites:
#   - Stacks running: make dev.up (or at least dev.up-central-bank-a + dev.up-bank-a)
#   - curl, jq
#   - backend/config/.env.infra.bank-a  with KC_CLIENT_SECRET
#
# Usage:
#   ./tryout-my-onboarding-status.sh
#
# Environment variables (optional):
#   BANK_URL  Bank-A API base URL  (default: http://localhost:18080/api/v1)
#   BANK_ENV  Bank-A .env file     (default: backend/config/.env.infra.bank-a)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BANK_URL="${BANK_URL:-http://localhost:18080/api/v1}"
BANK_ENV="${BANK_ENV:-backend/config/.env.infra.bank-a}"

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
# ---------------------------------------------------------------------------
# Setup — Submit credential request to create onboarding record
# Calls Bank-A's protected proxy which forwards to the CB's public
# /onboarding/credential-request endpoint.
# After this step the onboarding record is in CREDENTIAL_REQUESTED status.
# If the bank is already registered (409 Conflict), that's fine — we proceed.
# ---------------------------------------------------------------------------

setup_credential_request() {
  echo ""
  echo "Setup — Submit credential request (via Bank-A proxy)"
  echo "  POST $BANK_URL/onboarding/initiate"

  local tmp code body
  tmp=$(mktemp)
  code=$(curl -sS -X POST "$BANK_URL/onboarding/initiate" \
    --cookie "access_token=$BANK_TOKEN" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg inst "Bank A S.A." \
      --arg bank "a" \
      --arg country "BR" \
      --arg role "ROLE_COMMERCIAL_BANK" \
      --arg email "ops@bank-a.com.br" \
      --arg user "bank-a" \
      '{
        institution_name: $inst,
        bank_code: $bank,
        country: $country,
        role: $role,
        email: $email,
        username: $user
      }')" \
    -o "$tmp" -w "%{http_code}")
  body=$(cat "$tmp"); rm -f "$tmp"

  if [ "$code" = "201" ]; then
    echo "  ✅ Credential request created (CREDENTIAL_REQUESTED)"
    echo "  $(echo "$body" | jq -c '{request_id, user_id, status: "CREDENTIAL_REQUESTED"}')"
  elif [ "$code" = "409" ]; then
    echo "  ✅ Bank already registered (proceeding with existing record)"
  else
    echo "  ❌ ERROR: credential request failed (HTTP $code)." >&2
    echo "  Response: $body" >&2
    exit 1
  fi
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
# Step 2 — Check status before credential request → NONE
# ---------------------------------------------------------------------------

step_2_check_status_none() {
  echo ""
  echo "=== [2/4] Check status — no pending request ==="
  echo "  GET $BANK_URL/onboarding/my-status  (cookie: access_token)"

  local result code body
  result=$(get_request "$BANK_URL/onboarding/my-status" "$BANK_TOKEN")
  code="${result%% *}"
  body="${result#* }"

  assert_http_code "step2_http" "200" "$code" "$body"

  if [ "$code" = "200" ]; then
    assert_json_field "step2_status" "$body" "status" "NONE"
  fi
}

# ---------------------------------------------------------------------------
# Step 3 — Submit credential request
# ---------------------------------------------------------------------------

step_3_submit_credential_request() {
  echo ""
  echo "=== [3/4] Submit credential request ==="
  setup_credential_request
}

# ---------------------------------------------------------------------------
# Step 4 — Check status after credential request → pending with request_id
# ---------------------------------------------------------------------------

step_4_check_status_pending() {
  echo ""
  echo "=== [4/4] Check status — pending request ==="
  echo "  GET $BANK_URL/onboarding/my-status  (cookie: access_token)"

  local result code body
  result=$(get_request "$BANK_URL/onboarding/my-status" "$BANK_TOKEN")
  code="${result%% *}"
  body="${result#* }"

  assert_http_code "step4_http" "200" "$code" "$body"

  if [ "$code" = "200" ]; then
    local status request_id
    status=$(echo "$body" | jq -r '.status // empty')
    request_id=$(echo "$body" | jq -r '.request_id // empty')
    echo "  status:     $status"
    echo "  request_id: $request_id"

    assert_json_field_present "step4_status"     "$body" "status"
    assert_json_field_present "step4_request_id" "$body" "request_id"

    # Status must NOT be NONE anymore
    if [ "$status" = "NONE" ]; then
      echo "  ❌ FAIL — status is still NONE after credential request"
      FAIL=$((FAIL + 1))
    else
      echo "  ✅ PASS — status is no longer NONE (value: \"$status\")"
      PASS=$((PASS + 1))
    fi
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  require_cmds
  require_env_files

  echo ""
  echo "======================================================"
  echo "  Tryout: my-onboarding-status (Bank-A)"
  echo "  Bank-A Gateway: $BANK_URL"
  echo "======================================================"

  echo ""
  echo "=== [1/4] Login — Bank-A operator ==="
  bank_login

  step_2_check_status_none
  step_3_submit_credential_request
  step_4_check_status_pending

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Results: PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if [ "$FAIL" -gt 0 ]; then
    exit 1
  fi
}

main "$@"
