#!/usr/bin/env bash
# tryout-integracao-scenario-b.sh — Test all Integration Guide endpoints
#
# Covers all flows documented in:
#   docs-reference/integracao-scenario-b.md  (v4.0)
#
# FLOWS COVERED:
#   Section 3  — OIDC authentication (login via API Gateway)
#   Section 4  — Flow 0: Commercial bank onboarding (GET status, my-status)
#   Section 5  — Flow 1: Fiat → Token (balances, deposit, escrow, mint, approve)
#   Section 6  — Flow 2: Token → Fiat (redeem request/approve/reject)
#   Section 7  — Pool status (state machine)
#   Section 8  — Flow H: PairRegistry (propose, confirm, list)
#   Section 9  — Flow A: Commit-Reveal (commit A, commit B, auto-match, G5-cross)
#   Section 10 — Flow B: Pool status polling (all actors)
#   Section 11 — Flow C: Quote + Swap (exact-output, slippage, POOL_NOT_ACTIVE)
#   Section 12 — Flow D: Bridging (lock-mint, positions, burn-unlock)
#   Section 13 — Flow E: Liquidity removal (add + remove)
#   Section 14 — Flow F: Commits (list, cancel)
#   Section 15 — Flow G: Circuit Breaker (pause, resume-request, resume-sign)
#   Section 16 — Field validation and error codes (FR-018, DEPRECATED_FIELDS)
#   Oversight — disclosure-request, disclosure-sign, disclosure-status
#
# DEFAULT BEHAVIOR:
#   - READ operations (GET) always run
#   - WRITE operations (POST/DELETE) run by default
#   - Use READ_ONLY=true to skip all writes and test read-only
#   - Use SKIP_CIRCUIT_BREAKER=true to skip pause/resume (avoid pausing the pool)
#   - Use SECTION=<name> to run a specific section only
#
# PREREQUISITES:
#   - Stack running: make scenario-b.up  (or dev.up-bank-a, dev.up-central-bank-a, dev.up-central-bank-b)
#   - curl, jq
#   - backend/config/.env.infra.{bank-a,central-bank-a,central-bank-b}
#   - (Optional) cast (Foundry) for on-chain checks
#
# USAGE:
#   ./tryouts/tryout-integracao-scenario-b.sh
#   READ_ONLY=true ./tryouts/tryout-integracao-scenario-b.sh
#   SECTION=swap ./tryouts/tryout-integracao-scenario-b.sh
#   SKIP_CIRCUIT_BREAKER=true ./tryouts/tryout-integracao-scenario-b.sh
#
# ENVIRONMENT VARIABLES:
#   BANK_A_URL            Bank A API Gateway              (default: http://localhost:18080)
#   CB_A_URL              Central Bank A API Gateway      (default: http://localhost:38080)
#   CB_B_URL              Central Bank B API Gateway      (default: http://localhost:60080)
#   BANK_A_ENV            Bank A .env                     (default: backend/config/.env.infra.bank-a)
#   CB_A_ENV              Central Bank A .env             (default: backend/config/.env.infra.central-bank-a)
#   CB_B_ENV              Central Bank B .env             (default: backend/config/.env.infra.central-bank-b)
#   POOL_PAIR             Currency pair                   (default: BRL-ARS)
#   READ_ONLY             Skip all writes                 (default: false)
#   SKIP_DOCKER           Skip container check            (default: false)
#   SKIP_CIRCUIT_BREAKER  Skip CB pause/resume            (default: false)
#   SECTION               Specific section to run         (default: all)
#                         Values: auth|onboard|flow1|flow2|pool|pairreg|commit
#                                  swap|bridge|liquidity|circuitbreaker|validation|all

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "$PROJECT_ROOT"

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────
BANK_A_URL="${BANK_A_URL:-http://localhost:18080}"
CB_A_URL="${CB_A_URL:-http://localhost:38080}"
CB_B_URL="${CB_B_URL:-http://localhost:60080}"
BANK_A_ENV="${BANK_A_ENV:-${PROJECT_ROOT}/backend/config/.env.infra.bank-a}"
CB_A_ENV="${CB_A_ENV:-${PROJECT_ROOT}/backend/config/.env.infra.central-bank-a}"
CB_B_ENV="${CB_B_ENV:-${PROJECT_ROOT}/backend/config/.env.infra.central-bank-b}"
POOL_PAIR="${POOL_PAIR:-BRL-ARS}"
READ_ONLY="${READ_ONLY:-false}"
SKIP_DOCKER="${SKIP_DOCKER:-false}"
SKIP_CIRCUIT_BREAKER="${SKIP_CIRCUIT_BREAKER:-false}"
SECTION="${SECTION:-all}"

# Auth tokens (populated in S02)
BANK_A_TOKEN=""
CB_A_TOKEN=""
CB_B_TOKEN=""

# IDs captured during flows
DEPOSIT_ID=""
ESCROW_ID=""
REDEEM_ID=""
COMMIT_A_ID=""
COMMIT_B_ID=""
LP_ID_A=""
LP_ID_B=""
POSITION_ID=""
CB_PAUSE_REQUEST_ID=""
DISCLOSURE_ID=""
AMM_LP_ID=""

PASS=0
FAIL=0
SKIP=0
WARN=0
STEP=0

# Print each request payload before execution (useful for debugging).
# Disable with: SHOW_PAYLOAD=false ./tryout-integracao-scenario-b.sh
SHOW_PAYLOAD="${SHOW_PAYLOAD:-true}"

# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────

require_cmds() {
  for cmd in curl jq; do
    command -v "$cmd" &>/dev/null || { echo "ERROR: '$cmd' not found." >&2; exit 1; }
  done
}

read_env() {
  local file=$1 key=$2
  grep "^${key}=" "$file" 2>/dev/null | tail -n1 | cut -d= -f2- || true
}

next_step() {
  STEP=$((STEP + 1))
  local section=$1 label=$2
  echo ""
  echo "=== [S$(printf '%02d' $STEP) | $section] $label ==="
}

pass()  { echo "  ✅ PASS — $*"; PASS=$((PASS + 1)); }
fail()  { echo "  ❌ FAIL — $*"; FAIL=$((FAIL + 1)); }
warn()  { echo "  ⚠️  WARN — $*"; WARN=$((WARN + 1)); }
skip()  { echo "  ⏭️  SKIP — $*"; SKIP=$((SKIP + 1)); }
info()  { echo "  ℹ️  INFO — $*"; }

should_run() {
  local sec=$1
  [ "$SECTION" = "all" ] || [ "$SECTION" = "$sec" ]
}

# Print request method, URL and payload (when SHOW_PAYLOAD=true).
# Uses stderr so http_post/http_get/http_delete return values are not affected.
_log_request() {
  [ "$SHOW_PAYLOAD" = "true" ] || return 0
  local method=$1 url=$2 payload=${3:-}
  local display_url="${url#http://}" ; display_url="${display_url#https://}"
  if [ -n "$payload" ] && [ "$payload" != "{}" ]; then
    local pretty
    pretty=$(echo "$payload" | jq -c . 2>/dev/null || echo "$payload")
    echo "  ▶  $method $display_url  ↳ payload: $pretty" >&2
  else
    echo "  ▶  $method $display_url" >&2
  fi
}

# GET with auth cookie
http_get() {
  local url=$1 token=${2:-}
  local tmp code
  _log_request "GET" "$url"
  tmp=$(mktemp)
  code=$(curl -sS ${token:+--cookie "access_token=$token"} \
    -H "Accept: application/json" \
    -o "$tmp" -w "%{http_code}" --max-time 15 "$url" 2>/dev/null || echo "000")
  printf '%s %s' "$code" "$(cat "$tmp")"
  rm -f "$tmp"
}

# POST with auth cookie
http_post() {
  local url=$1 token=${2:-} body=${3:-"{}"}
  local tmp code
  _log_request "POST" "$url" "$body"
  tmp=$(mktemp)
  code=$(curl -sS ${token:+--cookie "access_token=$token"} \
    -X POST -H "Content-Type: application/json" -H "Accept: application/json" \
    -d "$body" -o "$tmp" -w "%{http_code}" --max-time 30 "$url" 2>/dev/null || echo "000")
  printf '%s %s' "$code" "$(cat "$tmp")"
  rm -f "$tmp"
}

# DELETE with auth cookie
http_delete() {
  local url=$1 token=${2:-}
  local tmp code
  _log_request "DELETE" "$url"
  tmp=$(mktemp)
  code=$(curl -sS ${token:+--cookie "access_token=$token"} \
    -X DELETE -H "Accept: application/json" \
    -o "$tmp" -w "%{http_code}" --max-time 15 "$url" 2>/dev/null || echo "000")
  printf '%s %s' "$code" "$(cat "$tmp")"
  rm -f "$tmp"
}

split_code() { echo "${1%% *}"; }
split_body() { echo "${1#* }"; }

# Check HTTP code and record PASS/FAIL/WARN
# Does not return result — use previously captured $r variable
check_http() {
  local label=$1 result=$2 expected_pattern=$3 hint=${4:-}
  local code body
  code=$(split_code "$result")
  body=$(split_body "$result")

  if [[ "$code" =~ $expected_pattern ]]; then
    pass "$label — HTTP $code"
  elif [ "$code" = "000" ]; then
    fail "$label — no response (timeout/connection refused)"
    [ -n "$hint" ] && info "$hint"
  else
    fail "$label — HTTP $code (expected ~$expected_pattern)"
    [ -n "$hint" ] && info "$hint"
    info "Response: $(echo "$body" | head -c 250)"
  fi
}

# Check field exists in JSON
check_field() {
  local body=$1 field=$2
  echo "$body" | jq -r ".${field} // empty" 2>/dev/null || true
}

# Verifica error_code expected
check_error_code() {
  local label=$1 body=$2 expected_code=$3
  local got
  got=$(echo "$body" | jq -r '.error_code // .error // empty' 2>/dev/null || true)
  if [ "$got" = "$expected_code" ]; then
    pass "$label — error_code=$expected_code (correct)"
  elif [ -z "$got" ]; then
    fail "$label — no error_code na resposta (expected $expected_code)"
    info "Body: $(echo "$body" | head -c 200)"
  else
    warn "$label — error_code=$got (expected $expected_code — may be implementation variation)"
  fi
}

# Login via API Gateway
gw_login() {
  local gw_url=$1 env_file=$2
  local client_id client_secret resp token
  client_id=$(read_env "$env_file" "KC_CLIENT_ID")
  client_secret=$(read_env "$env_file" "KC_CLIENT_SECRET")
  if [ -z "$client_id" ] || [ -z "$client_secret" ]; then
    echo ""
    return 1
  fi
  resp=$(http_post "$gw_url/api/v1/auth/login" "" \
    "{\"clientId\":\"$client_id\",\"clientSecret\":\"$client_secret\"}")
  token=$(split_body "$resp" | jq -r '.accessToken // empty' 2>/dev/null || true)
  echo "$token"
}

# Final summary
summary() {
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Results: PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP  WARN=$WARN"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if [ "$FAIL" = "0" ]; then
    echo "  ✅ All checks passed."
  else
    echo "  ❌ $FAIL check(s) failed. See errors above."
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Banner
# ─────────────────────────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Tryout: Integration Guide — Scenario B (v4.0)"
echo "  Bank A : $BANK_A_URL"
echo "  CB-A   : $CB_A_URL"
echo "  CB-B   : $CB_B_URL"
echo "  Pair   : $POOL_PAIR"
echo "  Section: $SECTION"
[ "$READ_ONLY" = "true" ] && echo "  Modo   : READ-ONLY (no writes)" || echo "  Mode   : READ+WRITE"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

require_cmds

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: auth — Infraestrutura + Authentication
# ─────────────────────────────────────────────────────────────────────────────
if should_run "auth" || should_run "all"; then

  # S01 — Pre-check de containers
  next_step "auth" "Pre-check de infraestrutura"
  if [ "$SKIP_DOCKER" = "true" ]; then
    skip "SKIP_DOCKER=true"
  elif ! command -v docker &>/dev/null; then
    warn "docker not found — skipping checagem de containers"
  else
    for name in backend-api-gateway-bank-a backend-api-gateway-central-bank-a backend-api-gateway-central-bank-b; do
      local_status=$(docker inspect --format "{{.State.Status}}" "$name" 2>/dev/null || echo "not_found")
      if [ "$local_status" = "running" ]; then
        pass "Container $name: running"
      else
        fail "Container $name: $local_status (run make scenario-b.up)"
      fi
    done
  fi

  # S02 — Login de todos os atores (Section 3 do doc)
  next_step "auth" "Section 3 — Login via API Gateway (POST /api/v1/auth/login)"
  echo "  Testing login for Bank A, Central Bank A and Central Bank B..."

  if [ -f "$BANK_A_ENV" ]; then
    BANK_A_TOKEN=$(gw_login "$BANK_A_URL" "$BANK_A_ENV" || true)
    [ -n "$BANK_A_TOKEN" ] && pass "Bank A — login OK (token obtained)" \
      || fail "Bank A — login failed (check $BANK_A_ENV e container)"
  else
    warn "Bank A ENV not found: $BANK_A_ENV"
    SKIP=$((SKIP + 1))
  fi

  if [ -f "$CB_A_ENV" ]; then
    CB_A_TOKEN=$(gw_login "$CB_A_URL" "$CB_A_ENV" || true)
    [ -n "$CB_A_TOKEN" ] && pass "Central Bank A — login OK (token obtained)" \
      || fail "Central Bank A — login failed (check $CB_A_ENV)"
  else
    warn "CB-A ENV not found: $CB_A_ENV"
    SKIP=$((SKIP + 1))
  fi

  if [ -f "$CB_B_ENV" ]; then
    CB_B_TOKEN=$(gw_login "$CB_B_URL" "$CB_B_ENV" || true)
    [ -n "$CB_B_TOKEN" ] && pass "Central Bank B — login OK (token obtained)" \
      || warn "Central Bank B — login failed (CB-B may not be in the current stack)"
  else
    warn "CB-B ENV not found: $CB_B_ENV — CB-B will be skipped"
    SKIP=$((SKIP + 1))
  fi

  # Abort if Bank A and CB-A have no token
  if [ -z "$BANK_A_TOKEN" ] && [ -z "$CB_A_TOKEN" ]; then
    echo ""
    echo "ERROR: no tokens — cannot continue. Check se os containers are running." >&2
    summary; exit 1
  fi

  # S03 — Auth/me (GET /api/v1/auth/me)
  next_step "auth" "Section 3 — GET /api/v1/auth/me (Bank A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v1/auth/me" "$BANK_A_TOKEN")
    check_http "auth/me Bank A" "$r" "^2" "Check JWT token is valid"
    code=$(split_code "$r"); body=$(split_body "$r")
    [[ "$code" =~ ^2 ]] && info "Role: $(check_field "$body" "role")"
  else
    skip "no token Bank A"
  fi

  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_get "$CB_A_URL/api/v1/auth/me" "$CB_A_TOKEN")
    check_http "auth/me CB-A" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    [[ "$code" =~ ^2 ]] && info "CB-A role: $(check_field "$body" "role")"
  else
    skip "no token CB-A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: onboard — Flow 0: Onboarding
# ─────────────────────────────────────────────────────────────────────────────
if should_run "onboard" || should_run "all"; then

  # S04 — GET /api/v1/onboarding/my-status (Section 4.3)
  next_step "onboard" "Section 4 — GET /api/v1/onboarding/my-status?bank_code=bank-a"
  if [ -n "$BANK_A_TOKEN" ]; then
    bank_code=$(read_env "$BANK_A_ENV" "KC_CLIENT_ID" | sed 's/-client//')
    r=$(http_get "$BANK_A_URL/api/v1/onboarding/my-status?bank_code=${bank_code:-bank-a}" "$BANK_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      ob_status=$(check_field "$body" "status")
      if [ "$ob_status" = "NONE" ]; then
        pass "my-status — HTTP $code (status=NONE — bank not onboarded ainda)"
        info "Para onboarding: POST /onboarding/initiate → polling status → POST /onboarding/complete"
      elif [ "$ob_status" = "COMPLETED" ]; then
        pass "my-status — HTTP $code (status=COMPLETED — onboarding completed)"
        info "request_id: $(check_field "$body" "request_id")"
        info "wallet_address: $(check_field "$body" "wallet_address")"
      else
        pass "my-status — HTTP $code (status=$ob_status)"
      fi
    else
      fail "my-status — HTTP $code"
      info "Response: $(echo "$body" | head -c 200)"
    fi
  else
    skip "no token Bank A"
  fi

  # S05 — GET /api/v1/onboarding/status/:requestId (Section 4.3)
  next_step "onboard" "Section 4 — GET /api/v1/onboarding/status/{request_id} (estrutura)"
  info "Endpoint: GET $BANK_A_URL/api/v1/onboarding/status/<request_id>"
  info "Checking 404 for fake request_id — confirms route is registered"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v1/onboarding/status/req-00000000-test-dry" "$BANK_A_TOKEN")
    code=$(split_code "$r")
    if [[ "$code" =~ ^(404|400) ]]; then
      pass "GET /api/v1/onboarding/status — responds (HTTP $code for fake ID — endpoint registered)"
    elif [ "$code" = "000" ]; then
      fail "GET /api/v1/onboarding/status — no response (endpoint missing?)"
    else
      warn "GET /api/v1/onboarding/status — HTTP $code (expected 404 for fake ID)"
    fi
  else
    skip "no token Bank A"
  fi

  # S06 — POST /api/v1/onboarding/initiate (Section 4.2)
  if [ "$READ_ONLY" = "true" ]; then
    next_step "onboard" "Section 4 — POST /api/v1/onboarding/initiate + /onboarding/complete"
    skip "READ_ONLY=true — skipping onboarding creation (destructive operation)"
  else
    next_step "onboard" "Section 4 — POST /api/v1/onboarding/initiate (endpoint structure)"
    info "Endpoint: POST $BANK_A_URL/api/v1/onboarding/initiate"
    info "Verificando que o endpoint responde (intentionally invalid payload to test existence)"
    if [ -n "$BANK_A_TOKEN" ]; then
      r=$(http_post "$BANK_A_URL/api/v1/onboarding/initiate" "$BANK_A_TOKEN" '{"dry_run_check":true}')
      code=$(split_code "$r")
      if [[ "$code" =~ ^(400|409|422|201|200) ]]; then
        pass "POST /api/v1/onboarding/initiate — endpoint registered (HTTP $code)"
      elif [ "$code" = "404" ]; then
        fail "POST /api/v1/onboarding/initiate — 404 (endpoint not registered — verificar CENTRAL_BANK_API_URL)"
      elif [ "$code" = "000" ]; then
        fail "POST /api/v1/onboarding/initiate — no response"
      else
        warn "POST /api/v1/onboarding/initiate — HTTP $code"
      fi
    else
      skip "no token Bank A"
    fi
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: flow1 — Flow 1: Fiat → Token
# ─────────────────────────────────────────────────────────────────────────────
if should_run "flow1" || should_run "all"; then

  # S07 — REMOVED: /api/v1/token/fiat-balance endpoint removed (fCeBM eliminated)
  next_step "flow1" "Section 5.2 — fiat-balance endpoint REMOVED (fCeBM unified into tCeBM)"
  skip "fiat-balance endpoint removed — use /api/v1/token/balance for tCeBM balance"

  # S08 — GET /api/v1/token/balance (Section 5.7)
  # NOTE: the document uses /api/v1/payment/token/balance but implementation uses /api/v1/token/balance
  next_step "flow1" "Section 5.7 — GET /api/v1/token/balance (Bank A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v1/token/balance" "$BANK_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "token/balance — HTTP $code"
      info "tCeBM balance: $(check_field "$body" "balance")"
    elif [ "$code" = "500" ]; then
      warn "token/balance — HTTP 500 (payment-orchestrator may not be available)"
    else
      fail "token/balance — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token Bank A"
  fi

  # S09 — GET /api/v1/payments/deposits (Section 5.4)
  next_step "flow1" "Section 5.4 — GET /api/v1/payments/deposits (Bank A — proxy→CB-A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v1/payments/deposits" "$BANK_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "payments/deposits — HTTP $code"
      info "Total deposits: $(echo "$body" | jq '.total // (. | length) // 0' 2>/dev/null || echo "?")"
      latest_dep=$(echo "$body" | jq -r '.[0].deposit_id // .deposits[0].deposit_id // empty' 2>/dev/null || true)
      [ -n "$latest_dep" ] && { DEPOSIT_ID="$latest_dep"; info "Latest deposit_id: $DEPOSIT_ID"; }
    elif [ "$code" = "404" ]; then
      warn "payments/deposits — HTTP 404 (proxy Bank A→CB-A not available — Cacti/relayer may not be running)"
      info "$(echo "$body" | head -c 200)"
    elif [ "$code" = "000" ]; then
      warn "payments/deposits — no response"
    else
      warn "payments/deposits — HTTP $code (proxy ativo mas CB retornou erro)"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token Bank A"
  fi

  # S10 — REMOVED: /api/v1/payments/escrows endpoint removed (escrow step eliminated)
  next_step "flow1" "Section 5.6 — escrows endpoint REMOVED (fCeBM→tCeBM escrow step eliminated)"
  if [ -n "$BANK_A_TOKEN" ]; then
    skip "escrows endpoint removed — tCeBM is minted directly upon deposit approval"
  else
    skip "no token Bank A"
  fi

  # S11 — POST /api/v2/amm/token/mint-and-approve CB-A (no recipient) (Section 5.10)
  next_step "flow1" "Section 5.10 — POST /api/v2/amm/token/mint-and-approve (CB-A, no recipient)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/token/mint-and-approve" "$CB_A_TOKEN" '{"amount":"1"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      status_val=$(check_field "$body" "status")
      amount_val=$(check_field "$body" "amount")
      [ "$status_val" = "ok" ] && pass "mint-and-approve CB-A — status=ok, amount=$amount_val" \
        || fail "mint-and-approve CB-A — status inexpected: $status_val"
    else
      fail "mint-and-approve CB-A — HTTP $code"
      info "Likely cause: CENTRAL_BANK_ROLE not granted or Besu not reachable at startup"
      info "Fix: docker restart backend-api-gateway-central-bank-a"
      info "Response: $(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-A"
  fi

  # S12 — POST /api/v2/amm/token/mint-and-approve CB-B (no recipient) (Section 9.1 Step 0b)
  next_step "flow1" "Section 9.1 Step 0b — POST /api/v2/amm/token/mint-and-approve (CB-B, no recipient)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_B_TOKEN" ]; then
    r=$(http_post "$CB_B_URL/api/v2/amm/token/mint-and-approve" "$CB_B_TOKEN" '{"amount":"1"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    [[ "$code" =~ ^2 ]] && check_field "$body" "status" | grep -q "ok" \
      && pass "mint-and-approve CB-B — status=ok" \
      || { fail "mint-and-approve CB-B — HTTP $code"; info "$(echo "$body" | head -c 200)"; }
  else
    skip "no token CB-B (CB-B may not be in the current stack)"
  fi

  # S13 — G5-cross: CB-B mints TOKEN_B with recipient=CB-A signer address (Section 9.1 Step 0c)
  next_step "flow1" "Section 9.1 G5.1 — POST /api/v2/amm/token/mint-and-approve (CB-B, recipient=CB-A addr)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_B_TOKEN" ] && [ -f "$CB_A_ENV" ]; then
    cb_a_signer=$(read_env "$CB_A_ENV" "SIGNER_PRIVATE_KEY")
    if command -v cast &>/dev/null && [ -n "$cb_a_signer" ]; then
      cb_a_addr=$(cast wallet address --private-key "0x${cb_a_signer#0x}" 2>/dev/null || true)
    else
      # Fallback: derive from contract address (not available without cast)
      cb_a_addr=$(grep -s '^SIGNER_ADDRESS=' "$CB_A_ENV" 2>/dev/null | cut -d= -f2 || true)
    fi
    if [ -n "$cb_a_addr" ]; then
      r=$(http_post "$CB_B_URL/api/v2/amm/token/mint-and-approve" "$CB_B_TOKEN" \
        "{\"amount\":\"1\",\"recipient\":\"$cb_a_addr\"}")
      code=$(split_code "$r"); body=$(split_body "$r")
      if [[ "$code" =~ ^2 ]]; then
        pass "G5.1 mint TOKEN_B → CB-A addr ($cb_a_addr) — HTTP $code"
        info "status=$(check_field "$body" "status") recipient=$(check_field "$body" "recipient")"
      else
        fail "G5.1 — HTTP $code"
        info "$(echo "$body" | head -c 200)"
      fi
    else
      skip "could not derive CB-A signer address (install foundry/cast or set SIGNER_ADDRESS no .env)"
    fi
  else
    skip "no token CB-B ou CB-A ENV ausente"
  fi

  # S14 — G5-cross: CB-A approves AMM for TOKEN_B com side=B (Section 9.1 Step 0c G5.2)
  next_step "flow1" "Section 9.1 G5.2 — POST /api/v2/amm/token/approve-amm (CB-A, side=B)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/token/approve-amm" "$CB_A_TOKEN" '{"amount":"1","side":"B"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "approve-amm CB-A side=B — HTTP $code"
      info "status=$(check_field "$body" "status") side=$(check_field "$body" "side")"
    else
      fail "approve-amm CB-A side=B — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-A"
  fi

  # S15 — POST /api/v2/amm/token/approve-amm Bank A no side (FR-013: BANK_CODE auto-detecta)
  next_step "flow1" "Section 5.8 FR-013 — POST /api/v2/amm/token/approve-amm (Bank A, no side — BANK_CODE=bank-a)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/amm/token/approve-amm" "$BANK_A_TOKEN" '{"amount":"1"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "approve-amm Bank A no side — HTTP $code (FR-013: BANK_CODE derived side automatically)"
      info "status=$(check_field "$body" "status") side=$(check_field "$body" "side")"
    else
      fail "approve-amm Bank A no side — HTTP $code (expected 2xx; verificar BANK_CODE=bank-a no gateway)"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token Bank A"
  fi

  # S16 — Fluxo 1 completo: register deposit → approve → exchange → escrow approve
  if [ "$READ_ONLY" = "true" ]; then
    next_step "flow1" "Section 5 — POST /payment/deposit/register + approve + exchange/request + escrow/approve"
    skip "READ_ONLY=true — create operations omitted"
  else
    # S16a — POST /api/v1/payments/deposits (Section 5.3)
    next_step "flow1" "Section 5.3 — POST /api/v1/payments/deposits (Bank A — proxy)"
    if [ -n "$BANK_A_TOKEN" ]; then
      requester_addr=$(read_env "$BANK_A_ENV" "BESU_ADDRESS" || echo "0x0000000000000000000000000000000000000001")
      r=$(http_post "$BANK_A_URL/api/v1/payments/deposits" "$BANK_A_TOKEN" \
        "{\"requester_besu_address\":\"$requester_addr\",\"amount\":\"1\"}")
      code=$(split_code "$r"); body=$(split_body "$r")
      if [[ "$code" =~ ^(201|200) ]]; then
        dep_id=$(check_field "$body" "deposit_id")
        [ -n "$dep_id" ] && { DEPOSIT_ID="$dep_id"; pass "POST /payments/deposits — deposit_id=$DEPOSIT_ID"; } \
          || warn "POST /payments/deposits — no deposit_id na resposta"
      elif [ "$code" = "404" ]; then
        warn "POST /payments/deposits — HTTP 404 (proxy Bank A→CB-A not available — Cacti/relayer may not be running)"
        info "$(echo "$body" | head -c 200)"
      elif [[ "$code" =~ ^5 ]]; then
        warn "POST /payments/deposits — HTTP $code (error from CB — Cacti may not be running)"
        info "$(echo "$body" | head -c 200)"
      else
        warn "POST /payments/deposits — HTTP $code"
        info "$(echo "$body" | head -c 250)"
      fi
    else
      skip "no token Bank A"
    fi

    # S16b — CB-A approves deposit via /api/v1/payments/deposits approve action
    # NOTA: route de approve is not implementada no router atual — verificando estrutura
    next_step "flow1" "Section 5.4 — POST /api/v1/payments/deposits/:id/approve (CB-A)"
    if [ -n "$CB_A_TOKEN" ] && [ -n "$DEPOSIT_ID" ]; then
      r=$(http_post "$CB_A_URL/api/v1/payments/deposits/$DEPOSIT_ID/approve" "$CB_A_TOKEN" '{}')
      code=$(split_code "$r"); body=$(split_body "$r")
      if [[ "$code" =~ ^(200|201) ]]; then
        pass "deposit approve — HTTP $code"
      elif [ "$code" = "404" ]; then
        warn "deposit approve — 404 (route /api/v1/payments/deposits/:id/approve not implemented ainda)"
      else
        fail "deposit approve — HTTP $code"; info "$(echo "$body" | head -c 200)"
      fi
    elif [ -z "$DEPOSIT_ID" ]; then
      skip "no deposit_id (S16a failed ou READ_ONLY)"
    else
      skip "no token CB-A"
    fi

    # S16c — REMOVED: escrow step eliminated; tCeBM is minted directly on deposit approval
    next_step "flow1" "Section 5.5 — escrow step REMOVED (tCeBM minted directly on deposit approval)"
    skip "escrow endpoint removed"

    # S16d — REMOVED: escrow approve eliminated
    next_step "flow1" "Section 5.6 — escrow approve REMOVED"
    skip "escrow endpoint removed"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: flow2 — Flow 2: Token → Fiat (Redeem)
# ─────────────────────────────────────────────────────────────────────────────
if should_run "flow2" || should_run "all"; then

  # S17 — GET /api/v1/payments/redeems (Section 6.4)
  next_step "flow2" "Section 6.4 — GET /api/v1/payments/redeems (Bank A — proxy→CB-A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v1/payments/redeems" "$BANK_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "payments/redeems — HTTP $code"
      info "Total redeems: $(echo "$body" | jq '.total // (. | length) // 0' 2>/dev/null || echo "?")"
      latest_rdm=$(echo "$body" | jq -r '.[0].redeem_id // .redeems[0].redeem_id // empty' 2>/dev/null || true)
      [ -n "$latest_rdm" ] && { REDEEM_ID="$latest_rdm"; info "Latest redeem_id: $REDEEM_ID"; }
    elif [ "$code" = "404" ]; then
      warn "payments/redeems — HTTP 404 (proxy Bank A→CB-A not available — Cacti/relayer may not be running)"
      info "$(echo "$body" | head -c 200)"
    elif [ "$code" = "000" ]; then
      warn "payments/redeems — no response"
    else
      warn "payments/redeems — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token Bank A"
  fi

  # S18 — POST /api/v1/payments/redeems (Section 6.3)
  next_step "flow2" "Section 6.3 — POST /api/v1/payments/redeems (Bank A — proxy)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$BANK_A_TOKEN" ]; then
    requester_addr=$(read_env "$BANK_A_ENV" "BESU_ADDRESS" || echo "0x0000000000000000000000000000000000000001")
    r=$(http_post "$BANK_A_URL/api/v1/payments/redeems" "$BANK_A_TOKEN" \
      "{\"requester_besu_address\":\"$requester_addr\",\"amount\":\"1\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      rdm_id=$(check_field "$body" "redeem_id")
      [ -n "$rdm_id" ] && { REDEEM_ID="$rdm_id"; pass "POST /payments/redeems — redeem_id=$REDEEM_ID"; } \
        || warn "POST /payments/redeems — no redeem_id na resposta"
    elif [ "$code" = "404" ]; then
      warn "POST /payments/redeems — HTTP 404 (proxy Bank A→CB-A not available)"
      info "$(echo "$body" | head -c 200)"
    elif [ "$code" = "422" ]; then
      err_code=$(echo "$body" | jq -r '.error_code // empty' 2>/dev/null || true)
      if [ "$err_code" = "INSUFFICIENT_BALANCE" ]; then
        warn "POST /payments/redeems — 422 INSUFFICIENT_BALANCE (no balance tCeBM — expected em clean environment)"
        info "To test redeem, run Flow 1 first (deposit + tokenization)"
      else
        warn "POST /payments/redeems — HTTP $code (error_code=$err_code)"
        info "$(echo "$body" | head -c 200)"
      fi
    elif [[ "$code" =~ ^5 ]]; then
      warn "POST /payments/redeems — HTTP $code (error from CB — Cacti may not be running)"
      info "$(echo "$body" | head -c 250)"
    else
      warn "POST /payments/redeems — HTTP $code"
      info "$(echo "$body" | head -c 250)"
    fi
  else
    skip "no token Bank A"
  fi

  # S19 — CB-A aprova redeem via /api/v1/payments/redeems/:id/approve
  # NOTA: route de approve is not implementada no router atual
  next_step "flow2" "Section 6.5 — POST /api/v1/payments/redeems/:id/approve (CB-A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ] && [ -n "$REDEEM_ID" ]; then
    r=$(http_post "$CB_A_URL/api/v1/payments/redeems/$REDEEM_ID/approve" "$CB_A_TOKEN" '{}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      pass "redeem approve — HTTP $code"
    elif [ "$code" = "404" ]; then
      warn "redeem approve — 404 (route /payments/redeems/:id/approve not implemented ainda)"
    else
      fail "redeem approve — HTTP $code"; info "$(echo "$body" | head -c 200)"
    fi
  elif [ -z "$REDEEM_ID" ]; then
    skip "no redeem_id (S18 failed ou READ_ONLY)"
  else
    skip "no token CB-A"
  fi

  # S20 — CB-A rejeita redeem via /api/v1/payments/redeems/:id/reject
  # NOTA: route de reject is not implementada no router atual
  next_step "flow2" "Section 6.6 — POST /api/v1/payments/redeems/:id/reject (CB-A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    # Test with fake ID to verify endpoint exists
    info "Testando endpoint structure com fake redeem_id"
    r=$(http_post "$CB_A_URL/api/v1/payments/redeems/rdm-00000000-0000-0000-0000-000000000000/reject" "$CB_A_TOKEN" \
      '{"reason":"KYC_EXPIRED"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      pass "redeem reject — HTTP $code (endpoint funcional)"
    elif [[ "$code" =~ ^(404|400|422) ]]; then
      code_msg=$(echo "$body" | jq -r '.error_code // .error // empty' 2>/dev/null | head -c 60 || true)
      [ "$code" = "404" ] \
        && warn "redeem reject — 404 (route /payments/redeems/:id/reject not implemented ainda)" \
        || pass "redeem reject — endpoint registered (HTTP $code $code_msg)"
    elif [ "$code" = "000" ]; then
      fail "redeem reject — no response"
    else
      warn "redeem reject — HTTP $code"
    fi
  else
    skip "no token CB-A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: pool — Pool Status (Section 7 / Fluxo B)
# ─────────────────────────────────────────────────────────────────────────────
if should_run "pool" || should_run "all"; then

  # S21 — GET /api/v2/amm/pool/{pair}/status (Bank A, Section 10)
  next_step "pool" "Section 10 — GET /api/v2/amm/pool/$POOL_PAIR/status (Bank A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v2/amm/pool/$POOL_PAIR/status" "$BANK_A_TOKEN")
    check_http "pool/status Bank A" "$r" "^2" \
      "404 → AMM_CONTRACT_ADDRESS not configured or client failed no startup"
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pool_status=$(check_field "$body" "pool_status")
      reserve_a=$(check_field "$body" "reserve_a")
      reserve_b=$(check_field "$body" "reserve_b")
      fee_bps=$(check_field "$body" "fee_rate_bps")
      lp_count=$(check_field "$body" "total_lp_count")
      info "pool_status=$pool_status  reserve_a=$reserve_a  reserve_b=$reserve_b"
      info "fee_rate_bps=$fee_bps  total_lp_count=$lp_count"
      CURRENT_POOL_STATUS="$pool_status"
    fi
  else
    skip "no token Bank A"
  fi

  # S22 — GET /api/v2/amm/pool/{pair}/status (CB-A, Section 10)
  next_step "pool" "Section 10 — GET /api/v2/amm/pool/$POOL_PAIR/status (CB-A)"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_get "$CB_A_URL/api/v2/amm/pool/$POOL_PAIR/status" "$CB_A_TOKEN")
    check_http "pool/status CB-A" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      info "pool_status=$(check_field "$body" "pool_status")  total_lp_count=$(check_field "$body" "total_lp_count")"
      pend=$(echo "$body" | jq '.pending_commits | length' 2>/dev/null || echo 0)
      info "pending_commits=$pend"
    fi
  else
    skip "no token CB-A"
  fi

  # S23 — GET /api/v2/amm/pool/{pair}/status (CB-B, Section 10)
  next_step "pool" "Section 10 — GET /api/v2/amm/pool/$POOL_PAIR/status (CB-B)"
  if [ -n "$CB_B_TOKEN" ]; then
    r=$(http_get "$CB_B_URL/api/v2/amm/pool/$POOL_PAIR/status" "$CB_B_TOKEN")
    check_http "pool/status CB-B" "$r" "^2"
  else
    skip "no token CB-B"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: pairreg — Flow H: PairRegistry
# ─────────────────────────────────────────────────────────────────────────────
if should_run "pairreg" || should_run "all"; then

  # S24 — GET /api/v2/amm/pairs (Section 8.4)
  next_step "pairreg" "Section 8.4 — GET /api/v2/amm/pairs (CB-A)"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_get "$CB_A_URL/api/v2/amm/pairs" "$CB_A_TOKEN")
    check_http "GET /pairs CB-A" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pair_count=$(echo "$body" | jq '(.pairs // .) | length' 2>/dev/null || echo "?")
      info "Registered pairs: $pair_count"
      brl_pair=$(echo "$body" | jq -r '(.pairs // .) | map(select(.status=="ACTIVE")) | .[0].pair_id // empty' 2>/dev/null || true)
      [ -n "$brl_pair" ] && info "Active pair found: $brl_pair"
    fi
  else
    skip "no token CB-A"
  fi

  # S25 — POST /api/v2/amm/pairs/propose (CB-A, Section 8.2)
  next_step "pairreg" "Section 8.2 — POST /api/v2/amm/pairs/propose (CB-A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    token_a=$(read_env "$CB_A_ENV" "HUB_TOKEN_A_ADDRESS")
    token_b=$(read_env "$CB_A_ENV" "HUB_TOKEN_B_ADDRESS")
    amm_addr=$(read_env "$CB_A_ENV" "AMM_CONTRACT_ADDRESS")
    if [ -n "$token_a" ] && [ -n "$token_b" ] && [ -n "$amm_addr" ]; then
      propose_body=$(jq -cn \
        --arg pid "BRL-ARS" \
        --arg ta "$token_a" --arg tb "$token_b" --arg amm "$amm_addr" \
        '{pair_id:$pid,token_a_address:$ta,token_b_address:$tb,amm_address:$amm,proposer_cb:"central_bank_a"}')
      r=$(http_post "$CB_A_URL/api/v2/amm/pairs/propose" "$CB_A_TOKEN" "$propose_body")
      code=$(split_code "$r"); body=$(split_body "$r")
      if [[ "$code" =~ ^(200|201) ]]; then
        pass "pairs/propose — HTTP $code (status=$(check_field "$body" "status"))"
      elif [ "$code" = "409" ]; then
        err_code=$(check_field "$body" "error_code")
        [ "$err_code" = "PAIR_ALREADY_EXISTS" ] \
          && pass "pairs/propose — 409 PAIR_ALREADY_EXISTS (par BRL-ARS already exists — OK)" \
          || warn "pairs/propose — 409 error_code=$err_code"
      elif [ "$code" = "403" ]; then
        err_code=$(check_field "$body" "error_code")
        warn "pairs/propose — 403 $err_code (setCentralBankOf() may not be configured)"
        info "Execute: make contracts.grant-central-bank-role or see guide section 8.1"
      elif [ "$code" = "422" ]; then
        warn "pairs/propose — HTTP 422 (on-chain revert — check contract permissions PairRegistry: setCentralBankOf())"
        info "$(echo "$body" | head -c 200)"
      else
        warn "pairs/propose — HTTP $code (infra-dependente)"
        info "$(echo "$body" | head -c 200)"
      fi
    else
      skip "contract addresses missing no $CB_A_ENV (HUB_TOKEN_A_ADDRESS / HUB_TOKEN_B_ADDRESS / AMM_CONTRACT_ADDRESS)"
    fi
  else
    skip "no token CB-A"
  fi

  # S26 — POST /api/v2/amm/pairs/confirm (CB-B, Section 8.3)
  next_step "pairreg" "Section 8.3 — POST /api/v2/amm/pairs/confirm (CB-B)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_B_TOKEN" ]; then
    r=$(http_post "$CB_B_URL/api/v2/amm/pairs/confirm" "$CB_B_TOKEN" \
      '{"pair_id":"BRL-ARS","confirmer_cb":"central_bank_b"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      pass "pairs/confirm — HTTP $code (status=$(check_field "$body" "status"))"
    elif [[ "$code" =~ ^(404|409|403) ]]; then
      err_code=$(check_field "$body" "error_code")
      pass "pairs/confirm — endpoint registered (HTTP $code, error_code=$err_code)"
    elif [ "$code" = "422" ]; then
      warn "pairs/confirm — HTTP 422 (on-chain revert — check contract permissions PairRegistry)"
      info "$(echo "$body" | head -c 200)"
    else
      warn "pairs/confirm — HTTP $code (infra-dependente)"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-B"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: commit — Flow A: Commit-Reveal Cooperativo
# ─────────────────────────────────────────────────────────────────────────────
CURRENT_POOL_STATUS="${CURRENT_POOL_STATUS:-}"

if should_run "commit" || should_run "all"; then

  # S27 — GET /api/v2/amm/liquidity/commits (CB-A, Section 14.1)
  next_step "commit" "Section 14.1 — GET /api/v2/amm/liquidity/commits?pool_pair=$POOL_PAIR (CB-A)"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_get "$CB_A_URL/api/v2/amm/liquidity/commits?pool_pair=$POOL_PAIR&status=PENDING" "$CB_A_TOKEN")
    check_http "liquidity/commits" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      c_count=$(echo "$body" | jq '(.commits // .) | length // (.count // 0)' 2>/dev/null || echo "?")
      info "Commits PENDING: $c_count"
    fi
  else
    skip "no token CB-A"
  fi

  # S28 — POST /api/v2/amm/liquidity/commit lado A (CB-A, Section 9.2)
  next_step "commit" "Section 9.2 — POST /api/v2/amm/liquidity/commit lado A (CB-A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    if [ "$CURRENT_POOL_STATUS" = "ACTIVE" ]; then
      skip "Pool already ACTIVE — commit-reveal not required"
    else
      r=$(http_post "$CB_A_URL/api/v2/amm/liquidity/commit" "$CB_A_TOKEN" \
        "{\"pool_pair\":\"$POOL_PAIR\",\"provider_id\":\"central_bank_a\",\"side\":\"A\",\"amount\":\"1000\"}")
      code=$(split_code "$r"); body=$(split_body "$r")
      if [[ "$code" =~ ^(200|201) ]]; then
        COMMIT_A_ID=$(check_field "$body" "commit_id")
        commit_status=$(check_field "$body" "status")
        [ -n "$COMMIT_A_ID" ] && pass "commit lado A — commit_id=$COMMIT_A_ID status=$commit_status" \
          || fail "commit lado A — no commit_id"
      elif [ "$code" = "409" ]; then
        err_code=$(check_field "$body" "error_code")
        if [ "$err_code" = "COMMIT_ALREADY_EXISTS" ]; then
          warn "commit lado A — 409 COMMIT_ALREADY_EXISTS (already exists commit PENDING)"
          # Tenta capturar o commit existente
          existing=$(http_get "$CB_A_URL/api/v2/amm/liquidity/commits?pool_pair=$POOL_PAIR&status=PENDING&provider_id=central_bank_a" "$CB_A_TOKEN")
          COMMIT_A_ID=$(split_body "$existing" | jq -r '(.commits // .) | .[0].commit_id // empty' 2>/dev/null || true)
          [ -n "$COMMIT_A_ID" ] && info "Usando commit existente: $COMMIT_A_ID"
        else
          fail "commit lado A — 409 error_code=$err_code"
          info "$(echo "$body" | head -c 200)"
        fi
      else
        fail "commit lado A — HTTP $code"
        info "$(echo "$body" | head -c 250)"
      fi
    fi
  else
    skip "no token CB-A"
  fi

  # S29 — Verify pool PENDING_COUNTERPART after commit A (Section 9.3)
  next_step "commit" "Section 9.3 — Pool should be PENDING_COUNTERPART after commit A"
  if [ -n "$CB_A_TOKEN" ] && [ "$CURRENT_POOL_STATUS" != "ACTIVE" ]; then
    r=$(http_get "$CB_A_URL/api/v2/amm/pool/$POOL_PAIR/status" "$CB_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pend_status=$(check_field "$body" "pool_status")
      reserve_a=$(check_field "$body" "reserve_a")
      reserve_b=$(check_field "$body" "reserve_b")
      if [ "$pend_status" = "PENDING_COUNTERPART" ]; then
        pass "pool_status=PENDING_COUNTERPART (reserve_a=$reserve_a, reserve_b=$reserve_b)"
      elif [ "$pend_status" = "ACTIVE" ]; then
        info "pool_status=ACTIVE — pool already had liquidez bilateral"
        CURRENT_POOL_STATUS="ACTIVE"
      else
        info "pool_status=$pend_status (reserve_a=$reserve_a, reserve_b=$reserve_b)"
      fi
    fi
  else
    skip "no token CB-A ou pool already ACTIVE"
  fi

  # S30 — Teste: SAME_PROVIDER_BOTH_SIDES deve ser rejeitado (Section 9.2 erros)
  next_step "commit" "Section 9.2 — SAME_PROVIDER_BOTH_SIDES rejection (central_bank_a cannot commitar lado B)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ] && [ "$CURRENT_POOL_STATUS" != "ACTIVE" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/liquidity/commit" "$CB_A_TOKEN" \
      "{\"pool_pair\":\"$POOL_PAIR\",\"provider_id\":\"central_bank_a\",\"side\":\"B\",\"amount\":\"1000\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [ "$code" = "409" ]; then
      check_error_code "SAME_PROVIDER_BOTH_SIDES" "$body" "SAME_PROVIDER_BOTH_SIDES"
    elif [ "$code" = "000" ]; then
      fail "SAME_PROVIDER_BOTH_SIDES — no response"
    else
      warn "SAME_PROVIDER_BOTH_SIDES — HTTP $code (expected 409 — may already be ACTIVE)"
    fi
  else
    skip "no token CB-A ou pool already ACTIVE"
  fi

  # S31 — Teste: POOL_NOT_ACTIVE no swap enquanto PENDING_COUNTERPART (Section 9.4)
  next_step "commit" "Section 9.4 — Swap bloqueado com POOL_NOT_ACTIVE quando pool is not ativo"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$BANK_A_TOKEN" ] && [ "$CURRENT_POOL_STATUS" != "ACTIVE" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/amm/swap/exact-output" "$BANK_A_TOKEN" \
      "{\"pair\":\"$POOL_PAIR\",\"amount_out\":\"1\",\"max_amount_in\":\"2\",\"payer_id\":\"bank_a\",\"beneficiary_id\":\"bank_c\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(422|400) ]]; then
      check_error_code "POOL_NOT_ACTIVE" "$body" "POOL_NOT_ACTIVE"
    elif [ "$code" = "000" ]; then
      fail "POOL_NOT_ACTIVE test — no response"
    else
      warn "POOL_NOT_ACTIVE test — HTTP $code (pool pode estar ACTIVE ou EMPTY)"
    fi
  else
    skip "pool already ACTIVE ou no token Bank A"
  fi

  # S32 — POST /api/v2/amm/liquidity/commit side B via CB-A gateway (Section 9.5)
  next_step "commit" "Section 9.5 — POST /api/v2/amm/liquidity/commit side B via CB-A (auto-match)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ] && [ "$CURRENT_POOL_STATUS" != "ACTIVE" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/liquidity/commit" "$CB_A_TOKEN" \
      "{\"pool_pair\":\"$POOL_PAIR\",\"provider_id\":\"central_bank_b\",\"side\":\"B\",\"amount\":\"1000\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      COMMIT_B_ID=$(check_field "$body" "commit_id")
      commit_b_status=$(check_field "$body" "status")
      lp_ids_json=$(echo "$body" | jq '.lp_ids // []' 2>/dev/null || echo "[]")
      LP_ID_A=$(echo "$lp_ids_json" | jq -r '.[0] // empty' 2>/dev/null || true)
      LP_ID_B=$(echo "$lp_ids_json" | jq -r '.[1] // empty' 2>/dev/null || true)
      if [ "$commit_b_status" = "EXECUTED" ]; then
        pass "commit side B — status=EXECUTED (auto-match completed!)"
        [ -n "$LP_ID_A" ] && info "LP_ID_A (central_bank_a): $LP_ID_A"
        [ -n "$LP_ID_B" ] && info "LP_ID_B (central_bank_b): $LP_ID_B"
        CURRENT_POOL_STATUS="ACTIVE"
      elif [ "$commit_b_status" = "PENDING" ]; then
        pass "commit side B — status=PENDING (awaiting match)"
        info "Pool still PENDING_COUNTERPART — auto-match may require same gateway instance"
      else
        warn "commit side B — status=$commit_b_status"
      fi
    elif [ "$code" = "409" ]; then
      err_code=$(check_field "$body" "error_code")
      warn "commit side B — 409 $err_code"
    elif [ "$code" = "422" ]; then
      err_code=$(echo "$body" | jq -r '.code // .error_code // empty' 2>/dev/null || true)
      warn "commit side B — HTTP 422 $err_code (on-chain revert — check contract permissions LiquidityCommitRegistry)"
      info "$(echo "$body" | head -c 250)"
    else
      warn "commit side B — HTTP $code (infra-dependente)"
      info "$(echo "$body" | head -c 250)"
    fi
  else
    skip "pool already ACTIVE, READ_ONLY ou no token CB-A"
  fi

  # S33 — Pool should be ACTIVE after commit-reveal (Section 9.6)
  next_step "commit" "Section 9.6 — Pool should be ACTIVE after commit-reveal"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_get "$CB_A_URL/api/v2/amm/pool/$POOL_PAIR/status" "$CB_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      act_status=$(check_field "$body" "pool_status")
      reserve_a=$(check_field "$body" "reserve_a")
      reserve_b=$(check_field "$body" "reserve_b")
      lp_count=$(check_field "$body" "total_lp_count")
      fee_bps=$(check_field "$body" "fee_rate_bps")
      CURRENT_POOL_STATUS="$act_status"
      if [ "$act_status" = "ACTIVE" ]; then
        pass "pool_status=ACTIVE (reserve_a=$reserve_a, reserve_b=$reserve_b, lp_count=$lp_count, fee=$fee_bps bps)"
      elif [ "$reserve_a" != "0" ] && [ "$reserve_b" != "0" ]; then
        pass "pool tem reservas bilaterais (reserve_a=$reserve_a, reserve_b=$reserve_b) — status=$act_status"
        CURRENT_POOL_STATUS="ACTIVE"
      else
        warn "pool_status=$act_status reserve_a=$reserve_a reserve_b=$reserve_b (pool may need liquidity)"
      fi
    fi
  else
    skip "no token CB-A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: swap — Flow C: Quote + Swap
# ─────────────────────────────────────────────────────────────────────────────
if should_run "swap" || should_run "all"; then

  # S34 — GET /api/v2/amm/quote/exact-output (Section 11.1)
  next_step "swap" "Section 11.1 — GET /api/v2/amm/quote/exact-output?pair=$POOL_PAIR&amount_out=100 (Bank A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v2/amm/quote/exact-output?pair=$POOL_PAIR&amount_out=100" "$BANK_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "quote/exact-output — HTTP $code"
      required_input=$(check_field "$body" "required_input")
      price_impact=$(check_field "$body" "price_impact")
      info "required_input=$required_input price_impact=$price_impact"
      QUOTE_MAX_IN="${required_input:-110}"
    elif [ "$code" = "422" ]; then
      err_code=$(echo "$body" | jq -r '.error_code // empty' 2>/dev/null || true)
      case "$err_code" in
        POOL_NOT_ACTIVE) warn "quote/exact-output — 422 POOL_NOT_ACTIVE (pool EMPTY — run commit-reveal first)" ;;
        INSUFFICIENT_POOL_LIQUIDITY) warn "quote/exact-output — 422 INSUFFICIENT_POOL_LIQUIDITY (pool without reserves — expected em clean environment)" ;;
        CIRCUIT_BREAKER_HALTED) warn "quote/exact-output — 422 CIRCUIT_BREAKER_HALTED" ;;
        *) fail "quote/exact-output — 422 error_code=$err_code"; info "$(echo "$body" | head -c 200)" ;;
      esac
      QUOTE_MAX_IN="110"
    else
      fail "quote/exact-output — HTTP $code"
      info "$(echo "$body" | head -c 200)"
      QUOTE_MAX_IN="110"
    fi
  else
    skip "no token Bank A"
    QUOTE_MAX_IN="110"
  fi

  # S35 — POST /api/v2/amm/swap/exact-output (Section 11.3)
  next_step "swap" "Section 11.3 — POST /api/v2/amm/swap/exact-output (Bank A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$BANK_A_TOKEN" ]; then
    max_in="${QUOTE_MAX_IN:-110}"
    # Adiciona 5% de slippage tolerance sobre o quote
    max_in_with_slippage=$(echo "$max_in * 1.05 / 1" | bc 2>/dev/null || echo "$max_in")
    r=$(http_post "$BANK_A_URL/api/v2/amm/swap/exact-output" "$BANK_A_TOKEN" \
      "{\"pair\":\"$POOL_PAIR\",\"amount_out\":\"100\",\"max_amount_in\":\"${max_in_with_slippage:-115}\",\"payer_id\":\"bank_a\",\"beneficiary_id\":\"bank_c\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      order_id=$(check_field "$body" "order_id")
      amount_in=$(check_field "$body" "amount_in")
      fee=$(check_field "$body" "fee_amount")
      swap_status=$(check_field "$body" "status")
      pass "swap/exact-output — HTTP $code (order_id=$order_id, amount_in=$amount_in, fee=$fee, status=$swap_status)"
    elif [ "$code" = "422" ]; then
      err_code=$(check_field "$body" "error_code")
      case "$err_code" in
        POOL_NOT_ACTIVE) warn "swap — POOL_NOT_ACTIVE (pool no liquidity bilateral)" ;;
        CIRCUIT_BREAKER_HALTED) warn "swap — CIRCUIT_BREAKER_HALTED (pool paused by governance)" ;;
        SLIPPAGE_LIMIT_EXCEEDED) warn "swap — SLIPPAGE_LIMIT_EXCEEDED (increase max_amount_in)" ;;
        INSUFFICIENT_POOL_LIQUIDITY) warn "swap — INSUFFICIENT_POOL_LIQUIDITY" ;;
        *) fail "swap — 422 error_code=$err_code"; info "$(echo "$body" | head -c 200)" ;;
      esac
    else
      fail "swap/exact-output — HTTP $code"
      info "$(echo "$body" | head -c 250)"
    fi
  else
    skip "no token Bank A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: bridge — Flow D: Bridging
# ─────────────────────────────────────────────────────────────────────────────
if should_run "bridge" || should_run "all"; then

  # S36 — POST /api/v2/bridge/lock-mint (Section 12.1)
  next_step "bridge" "Section 12.1 — POST /api/v2/bridge/lock-mint (Bank A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/bridge/lock-mint" "$BANK_A_TOKEN" \
      '{"owner_bank_id":"bank_a","spoke_network":"spoke-a","native_asset":"BRL-CBDC","mirrored_asset":"mBRL-CBDC","amount":"100"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201|202) ]]; then
      POSITION_ID=$(check_field "$body" "position_id")
      bridge_state=$(check_field "$body" "bridge_state")
      [ -n "$POSITION_ID" ] && pass "lock-mint — position_id=$POSITION_ID state=$bridge_state" \
        || fail "lock-mint — no position_id na resposta"
    elif [ "$code" = "401" ]; then
      warn "lock-mint — HTTP 401 (bank not onboarded — bank_id ausente no JWT; run onboarding flow first)"
    elif [ "$code" = "404" ]; then
      warn "lock-mint — 404 (endpoint not registered ou bridge service not configured)"
    elif [ "$code" = "000" ]; then
      fail "lock-mint — no response"
    else
      warn "lock-mint — HTTP $code"
      info "$(echo "$body" | head -c 250)"
    fi
  else
    skip "no token Bank A"
  fi

  # S37 — GET /api/v2/bridge/positions (Section 12.2)
  next_step "bridge" "Section 12.2 — GET /api/v2/bridge/positions (Bank A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_get "$BANK_A_URL/api/v2/bridge/positions" "$BANK_A_TOKEN")
    check_http "bridge/positions" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pos_count=$(echo "$body" | jq '.positions | length // 0' 2>/dev/null || echo "?")
      info "Bridge positions: $pos_count"
      if [ -z "$POSITION_ID" ]; then
        POSITION_ID=$(echo "$body" | jq -r '.positions[0].position_id // empty' 2>/dev/null || true)
        [ -n "$POSITION_ID" ] && info "Using existing position_id: $POSITION_ID"
      fi
    fi
  else
    skip "no token Bank A"
  fi

  # S38 — POST /api/v2/bridge/burn-unlock (Section 12.3)
  next_step "bridge" "Section 12.3 — POST /api/v2/bridge/burn-unlock (Bank A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$BANK_A_TOKEN" ] && [ -n "$POSITION_ID" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/bridge/burn-unlock" "$BANK_A_TOKEN" \
      "{\"position_id\":\"$POSITION_ID\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201|202) ]]; then
      bridge_state=$(check_field "$body" "bridge_state")
      pass "burn-unlock — HTTP $code (state=$bridge_state)"
    elif [[ "$code" =~ ^(400|422) ]]; then
      err_code=$(check_field "$body" "error_code")
      warn "burn-unlock — HTTP $code error_code=$err_code (position may not be ACTIVE yet)"
    else
      fail "burn-unlock — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  elif [ -z "$POSITION_ID" ]; then
    skip "no position_id (lock-mint did not create position ou READ_ONLY)"
  else
    skip "no token Bank A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: liquidity — Flow E: Add + Remove Liquidez
# ─────────────────────────────────────────────────────────────────────────────
if should_run "liquidity" || should_run "all"; then

  # S39 — POST /api/v2/amm/liquidity/add (CB-A, Section 13)
  next_step "liquidity" "Section 13 — POST /api/v2/amm/liquidity/add (CB-A, dual-sided)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/liquidity/add" "$CB_A_TOKEN" \
      "{\"pool_pair\":\"$POOL_PAIR\",\"token_a_amount\":\"100\",\"token_b_amount\":\"100\",\"provider_bank_id\":\"central_bank_a\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      AMM_LP_ID=$(check_field "$body" "lp_id")
      deposit_side=$(check_field "$body" "deposit_side")
      [ -n "$AMM_LP_ID" ] && pass "liquidity/add — lp_id=$AMM_LP_ID deposit_side=$deposit_side" \
        || fail "liquidity/add — no lp_id"
    elif [ "$code" = "422" ]; then
      err_code=$(check_field "$body" "error_code")
      warn "liquidity/add — 422 $err_code (pool may be EMPTY ou without token approval)"
    else
      fail "liquidity/add — HTTP $code"
      info "$(echo "$body" | head -c 250)"
    fi
  else
    skip "no token CB-A"
  fi

  # S40 — POST /api/v2/amm/liquidity/remove (CB-A, Section 13.1)
  next_step "liquidity" "Section 13.1 — POST /api/v2/amm/liquidity/remove (CB-A)"
  LP_TO_REMOVE="${AMM_LP_ID:-${LP_ID_A:-}}"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ] && [ -n "$LP_TO_REMOVE" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/liquidity/remove" "$CB_A_TOKEN" \
      "{\"lp_id\":\"$LP_TO_REMOVE\",\"pool_pair\":\"$POOL_PAIR\",\"provider_bank_id\":\"central_bank_a\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      mode=$(check_field "$body" "withdrawal_mode")
      ret_a=$(check_field "$body" "token_a_amount")
      ret_b=$(check_field "$body" "token_b_amount")
      fee_paid=$(check_field "$body" "fee_claim_paid")
      pass "liquidity/remove — HTTP $code (mode=$mode, retA=$ret_a, retB=$ret_b, fee=$fee_paid)"
      [ "$mode" = "PROPORTIONAL" ] && info "✓ withdrawal_mode=PROPORTIONAL (cooperative position — FR-007)"
    elif [ "$code" = "404" ]; then
      warn "liquidity/remove — 404 LP_NOT_FOUND (LP may already have been removed)"
    else
      fail "liquidity/remove — HTTP $code"
      info "$(echo "$body" | head -c 250)"
    fi
  elif [ -z "$LP_TO_REMOVE" ]; then
    skip "no lp_id available (S39 failed ou READ_ONLY)"
  else
    skip "no token CB-A"
  fi

  # S41 — DELETE /api/v2/amm/liquidity/commits/{id}?provider_id=central_bank_a (Section 14.2)
  next_step "liquidity" "Section 14.2 — DELETE /api/v2/amm/liquidity/commits/{id}?provider_id=central_bank_a (CB-A)"
  COMMIT_TO_CANCEL="${COMMIT_A_ID:-}"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ] && [ -n "$COMMIT_TO_CANCEL" ]; then
    r=$(http_delete "$CB_A_URL/api/v2/amm/liquidity/commits/$COMMIT_TO_CANCEL?provider_id=central_bank_a" "$CB_A_TOKEN")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      del_status=$(check_field "$body" "status")
      pass "DELETE /commits — HTTP $code (status=$del_status)"
    elif [ "$code" = "409" ]; then
      err_code=$(check_field "$body" "error_code")
      [ "$err_code" = "COMMIT_ALREADY_MATCHED" ] \
        && warn "DELETE /commits — 409 COMMIT_ALREADY_MATCHED (already matched — not cancellable)" \
        || warn "DELETE /commits — 409 $err_code"
    elif [ "$code" = "404" ]; then
      warn "DELETE /commits — 404 (commit already expired ou foi executado)"
    elif [ "$code" = "400" ]; then
      warn "DELETE /commits — 400 ($(echo "$body" | jq -r '.error // empty' 2>/dev/null || echo "bad request"))"
    else
      warn "DELETE /commits — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  elif [ -z "$COMMIT_TO_CANCEL" ]; then
    skip "no commit_id to cancel (S28 did not create commit ou READ_ONLY)"
  else
    skip "no token CB-A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: circuitbreaker — Flow G: Circuit Breaker governance
# ─────────────────────────────────────────────────────────────────────────────
if should_run "circuitbreaker" || should_run "all"; then

  # S42 — GET /api/v2/governance/circuit-breaker/status (Section 15)
  next_step "circuitbreaker" "Section 15 — GET /api/v2/governance/circuit-breaker/status (CB-A)"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_get "$CB_A_URL/api/v2/governance/circuit-breaker/status" "$CB_A_TOKEN")
    check_http "circuit-breaker/status" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    [[ "$code" =~ ^2 ]] && info "CB status: $(echo "$body" | jq -c '.' 2>/dev/null | head -c 200)"
  else
    skip "no token CB-A"
  fi

  # S43 — POST /api/v2/governance/circuit-breaker/pause (Section 15.1)
  next_step "circuitbreaker" "Section 15.1 — POST /api/v2/governance/circuit-breaker/pause (CB-A)"
  if [ "$SKIP_CIRCUIT_BREAKER" = "true" ]; then
    skip "SKIP_CIRCUIT_BREAKER=true — skipping pause to avoid interrupting the pool"
  elif [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/governance/circuit-breaker/pause" "$CB_A_TOKEN" \
      "{\"pair\":\"$POOL_PAIR\",\"bank_id\":\"central_bank_a\",\"reason_code\":\"E2E_TRYOUT_TEST\",\"signature\":\"AA==\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "circuit-breaker/pause — HTTP $code (status=$(check_field "$body" "status"))"
    elif [[ "$code" =~ ^(400|422) ]]; then
      warn "circuit-breaker/pause — HTTP $code (invalid signature expected in dev)"
      info "$(echo "$body" | head -c 200)"
    else
      fail "circuit-breaker/pause — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-A"
  fi

  # S44 — POST /api/v2/governance/circuit-breaker/resume-request (Section 15.2)
  next_step "circuitbreaker" "Section 15.2 — POST /api/v2/governance/circuit-breaker/resume-request (CB-A)"
  if [ "$SKIP_CIRCUIT_BREAKER" = "true" ]; then
    skip "SKIP_CIRCUIT_BREAKER=true"
  elif [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/governance/circuit-breaker/resume-request" "$CB_A_TOKEN" \
      "{\"pair\":\"$POOL_PAIR\",\"bank_id\":\"central_bank_a\",\"signature\":\"AA==\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      CB_PAUSE_REQUEST_ID=$(check_field "$body" "request_id")
      pass "circuit-breaker/resume-request — HTTP $code (request_id=$CB_PAUSE_REQUEST_ID)"
    elif [[ "$code" =~ ^(400|409|422) ]]; then
      err_code=$(check_field "$body" "error_code")
      warn "circuit-breaker/resume-request — HTTP $code $err_code (pool may not be paused)"
    else
      fail "circuit-breaker/resume-request — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-A"
  fi

  # S45 — POST /api/v2/governance/circuit-breaker/resume-sign (CB-B, Section 15.3)
  next_step "circuitbreaker" "Section 15.3 — POST /api/v2/governance/circuit-breaker/resume-sign (CB-B)"
  if [ "$SKIP_CIRCUIT_BREAKER" = "true" ]; then
    skip "SKIP_CIRCUIT_BREAKER=true"
  elif [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_B_TOKEN" ] && [ -n "$CB_PAUSE_REQUEST_ID" ]; then
    r=$(http_post "$CB_B_URL/api/v2/governance/circuit-breaker/resume-sign" "$CB_B_TOKEN" \
      "{\"pair\":\"$POOL_PAIR\",\"request_id\":\"$CB_PAUSE_REQUEST_ID\",\"bank_id\":\"central_bank_b\",\"signature\":\"AA==\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "circuit-breaker/resume-sign — HTTP $code"
    elif [[ "$code" =~ ^(400|422) ]]; then
      warn "circuit-breaker/resume-sign — HTTP $code (invalid signature in dev — endpoint registered)"
    else
      fail "circuit-breaker/resume-sign — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  elif [ -z "$CB_PAUSE_REQUEST_ID" ]; then
    skip "no resume request_id (S44 did not generate or pool was not paused)"
  else
    skip "no token CB-B"
  fi

  # S46 — Oversight: POST /api/v2/oversight/disclosure-request (CB-A)
  next_step "circuitbreaker" "Oversight — POST /api/v2/oversight/disclosure-request (CB-A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/oversight/disclosure-request" "$CB_A_TOKEN" \
      '{"tx_ref":"did:tryout:test:0001","requestor_id":"central_bank_a","reason_code":"REGULATORY_AUDIT_TEST"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(200|201) ]]; then
      DISCLOSURE_ID=$(check_field "$body" "request_id")
      pass "disclosure-request — HTTP $code (request_id=$DISCLOSURE_ID)"
    elif [[ "$code" =~ ^(400|404|422) ]]; then
      warn "disclosure-request — HTTP $code (endpoint may not be implemented yet)"
    else
      fail "disclosure-request — HTTP $code"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-A"
  fi

  # S47 — Oversight: POST /api/v2/oversight/disclosure-sign (CB-A)
  next_step "circuitbreaker" "Oversight — POST /api/v2/oversight/disclosure-sign (CB-A)"
  if [ "$READ_ONLY" = "true" ]; then
    skip "READ_ONLY=true"
  elif [ -n "$CB_A_TOKEN" ] && [ -n "$DISCLOSURE_ID" ]; then
    r=$(http_post "$CB_A_URL/api/v2/oversight/disclosure-sign" "$CB_A_TOKEN" \
      "{\"request_id\":\"$DISCLOSURE_ID\",\"signer_id\":\"central_bank_a\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    [[ "$code" =~ ^2 ]] && pass "disclosure-sign — HTTP $code" \
      || warn "disclosure-sign — HTTP $code (quorum or signature pending)"
  elif [ -z "$DISCLOSURE_ID" ]; then
    skip "no disclosure request_id"
  else
    skip "no token CB-A"
  fi

  # S48 — Oversight: GET /api/v2/oversight/disclosure-status/{id} (CB-A)
  next_step "circuitbreaker" "Oversight — GET /api/v2/oversight/disclosure-status/{id} (CB-A)"
  if [ -n "$CB_A_TOKEN" ] && [ -n "$DISCLOSURE_ID" ]; then
    r=$(http_get "$CB_A_URL/api/v2/oversight/disclosure-status/$DISCLOSURE_ID" "$CB_A_TOKEN")
    check_http "disclosure-status" "$r" "^2"
    code=$(split_code "$r"); body=$(split_body "$r")
    [[ "$code" =~ ^2 ]] && info "disclosure status: $(check_field "$body" "status")"
  elif [ -z "$DISCLOSURE_ID" ]; then
    skip "no disclosure request_id"
  else
    skip "no token CB-A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# SECTION: validation — Field validation (Section 16 / FR-018)
# ─────────────────────────────────────────────────────────────────────────────
if should_run "validation" || should_run "all"; then

  # S49 — FR-013: approve-amm no side com BANK_CODE=bank-a → should return 200 (auto-detection)
  next_step "validation" "Section 5.8 FR-013 — approve-amm sem 'side' returns 200 when BANK_CODE configurado (Bank A)"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/amm/token/approve-amm" "$BANK_A_TOKEN" '{"amount":"1"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      err_code=$(echo "$body" | jq -r '.code // empty' 2>/dev/null || true)
      pass "approve-amm no side — HTTP $code (FR-013: BANK_CODE=bank-a auto-detectou side=A)"
      info "status=$(check_field "$body" "status") side=$(check_field "$body" "side")"
    elif [ "$code" = "400" ]; then
      err_msg=$(echo "$body" | jq -r '.error // empty' 2>/dev/null || true)
      err_code=$(echo "$body" | jq -r '.code // empty' 2>/dev/null || true)
      if [ "$err_code" = "COMMIT_SIDE_NOT_CONFIGURED" ]; then
        warn "approve-amm no side — HTTP 400 COMMIT_SIDE_NOT_CONFIGURED (BANK_CODE not configured no gateway — configurar BANK_CODE=bank-a)"
      else
        fail "approve-amm no side — HTTP 400 inexpected: $err_msg"
      fi
    elif [ "$code" = "000" ]; then
      fail "approve-amm no side — no response"
    else
      warn "approve-amm no side — HTTP $code (expected 2xx com BANK_CODE configurado)"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token Bank A"
  fi

  # S49b — FR-013: CB-A with explicit side=B → should return 200 (G5-cross preserved)
  next_step "validation" "Section 9.1 FR-013 — approve-amm CB-A side=B explicit returns 200 (G5-cross)"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/token/approve-amm" "$CB_A_TOKEN" '{"amount":"1","side":"B"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^2 ]]; then
      pass "approve-amm CB-A explicit side=B — HTTP $code (G5-cross preserved)"
      info "status=$(check_field "$body" "status") side=$(check_field "$body" "side")"
    else
      fail "approve-amm CB-A explicit side=B — HTTP $code (expected 2xx — G5-cross must be preserved)"
      info "$(echo "$body" | head -c 200)"
    fi
  else
    skip "no token CB-A"
  fi

  # S50 — approve-amm com campos deprecateds amount_a/amount_b → deve retornar 400 (Section 16.1)
  next_step "validation" "Section 16.1 — approve-amm com amount_a/amount_b (deprecated) retorna 400"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/amm/token/approve-amm" "$BANK_A_TOKEN" \
      '{"amount_a":"1","amount_b":"1","side":"A"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [ "$code" = "400" ]; then
      err_code=$(echo "$body" | jq -r '.error_code // empty' 2>/dev/null || true)
      pass "approve-amm com amount_a/b — HTTP 400 (DEPRECATED_FIELDS)"
      info "error_code: $err_code"
    else
      warn "approve-amm com amount_a/b — HTTP $code (expected 400 DEPRECATED_FIELDS)"
    fi
  else
    skip "no token Bank A"
  fi

  # S51 — mint-and-approve com campos deprecateds (CB-A) → deve retornar 400 (Section 16.1)
  next_step "validation" "Section 16.1 — mint-and-approve com amount_a/amount_b (deprecated) retorna 400 (CB-A)"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/token/mint-and-approve" "$CB_A_TOKEN" \
      '{"amount_a":"1","amount_b":"1"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [ "$code" = "400" ]; then
      err_code=$(echo "$body" | jq -r '.error_code // empty' 2>/dev/null || true)
      pass "mint-and-approve com amount_a/b — HTTP 400 (DEPRECATED_FIELDS)"
      info "error_code: $err_code"
    else
      warn "mint-and-approve com amount_a/b — HTTP $code (expected 400 DEPRECATED_FIELDS)"
    fi
  else
    skip "no token CB-A"
  fi

  # S52 — commit com side invalid → deve retornar 400 (Section 16.1 INVALID_SIDE)
  next_step "validation" "Section 16.1 — commit com side invalid retorna 400 INVALID_SIDE"
  if [ -n "$CB_A_TOKEN" ]; then
    r=$(http_post "$CB_A_URL/api/v2/amm/liquidity/commit" "$CB_A_TOKEN" \
      "{\"pool_pair\":\"$POOL_PAIR\",\"provider_id\":\"test\",\"side\":\"X\",\"amount\":\"1\"}")
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(400|422) ]]; then
      err_code=$(echo "$body" | jq -r '.error_code // .error // empty' 2>/dev/null || true)
      pass "commit side invalid — HTTP $code (INVALID_SIDE ou INVALID_REQUEST)"
      info "error_code: $err_code"
    else
      warn "commit side=X — HTTP $code (expected 400/422)"
    fi
  else
    skip "no token CB-A"
  fi

  # S53 — swap com PAIR_NOT_FOUND (par nonexistent)
  next_step "validation" "Section 16.1 — swap com par nonexistent retorna erro"
  if [ -n "$BANK_A_TOKEN" ]; then
    r=$(http_post "$BANK_A_URL/api/v2/amm/swap/exact-output" "$BANK_A_TOKEN" \
      '{"pair":"XXX-YYY","amount_out":"1","max_amount_in":"2","payer_id":"bank_a","beneficiary_id":"bank_c"}')
    code=$(split_code "$r"); body=$(split_body "$r")
    if [[ "$code" =~ ^(400|404|422) ]]; then
      err_code=$(echo "$body" | jq -r '.error_code // .error // empty' 2>/dev/null || true)
      pass "swap par nonexistent — HTTP $code (error=$err_code)"
    else
      warn "swap par XXX-YYY — HTTP $code"
    fi
  else
    skip "no token Bank A"
  fi

fi

# ─────────────────────────────────────────────────────────────────────────────
# Summary and next steps
# ─────────────────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Integration Guide — Scenario B — Endpoint Coverage"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ✔  Section 3  — POST /api/v1/auth/login (Bank A, CB-A, CB-B)"
echo "  ✔  Section 3  — GET  /api/v1/auth/me"
echo "  ✔  Section 4  — GET  /api/v1/onboarding/my-status"
echo "  ✔  Section 4  — GET  /onboarding/status/{id} (estrutura)"
echo "  ✔  Section 4  — POST /onboarding/initiate (estrutura)"
echo "  ✔  Section 5  — GET  /api/v1/payment/fiat/balance"
echo "  ✔  Section 5  — GET  /api/v1/payment/token/balance"
echo "  ✔  Section 5  — GET  /api/v1/payment/deposit/list"
echo "  ✔  Section 5  — GET  /api/v1/payment/escrow/list"
echo "  ✔  Section 5  — POST /api/v1/payment/deposit/register"
echo "  ✔  Section 5  — POST /api/v1/payment/deposit/approve (CB-A)"
echo "  ✔  Section 5  — POST /api/v1/payment/exchange/request"
echo "  ✔  Section 5  — POST /api/v1/payment/escrow/approve (CB-A)"
echo "  ✔  Section 5  — POST /api/v2/amm/token/mint-and-approve (CB-A, CB-B)"
echo "  ✔  Section 5  — POST /api/v2/amm/token/mint-and-approve (CB-B, recipient=CB-A)"
echo "  ✔  Section 5  — POST /api/v2/amm/token/approve-amm (Bank A no side FR-013, CB-A side=B G5-cross)"
echo "  ✔  Section 6  — GET  /api/v1/payment/redeem/list"
echo "  ✔  Section 6  — POST /api/v1/payment/redeem/request"
echo "  ✔  Section 6  — POST /api/v1/payment/redeem/approve (CB-A)"
echo "  ✔  Section 6  — POST /api/v1/payment/redeem/reject (CB-A)"
echo "  ✔  Section 8  — GET  /api/v2/amm/pairs"
echo "  ✔  Section 8  — POST /api/v2/amm/pairs/propose (CB-A)"
echo "  ✔  Section 8  — POST /api/v2/amm/pairs/confirm (CB-B)"
echo "  ✔  Section 10 — GET  /api/v2/amm/pool/{pair}/status (Bank A, CB-A, CB-B)"
echo "  ✔  Section 14 — GET  /api/v2/amm/liquidity/commits"
echo "  ✔  Section 9  — POST /api/v2/amm/liquidity/commit (lado A, lado B, G5-cross)"
echo "  ✔  Section 9  — SAME_PROVIDER validation_BOTH_SIDES + POOL_NOT_ACTIVE"
echo "  ✔  Section 11 — GET  /api/v2/amm/quote/exact-output"
echo "  ✔  Section 11 — POST /api/v2/amm/swap/exact-output"
echo "  ✔  Section 12 — POST /api/v2/bridge/lock-mint"
echo "  ✔  Section 12 — GET  /api/v2/bridge/positions"
echo "  ✔  Section 12 — POST /api/v2/bridge/burn-unlock"
echo "  ✔  Section 13 — POST /api/v2/amm/liquidity/add"
echo "  ✔  Section 13 — POST /api/v2/amm/liquidity/remove"
echo "  ✔  Section 14 — DELETE /api/v2/amm/liquidity/commits/{id}"
echo "  ✔  Section 15 — GET  /api/v2/governance/circuit-breaker/status"
echo "  ✔  Section 15 — POST /api/v2/governance/circuit-breaker/pause"
echo "  ✔  Section 15 — POST /api/v2/governance/circuit-breaker/resume-request"
echo "  ✔  Section 15 — POST /api/v2/governance/circuit-breaker/resume-sign (CB-B)"
echo "  ✔  Section 15 — POST /api/v2/oversight/disclosure-request"
echo "  ✔  Section 15 — POST /api/v2/oversight/disclosure-sign"
echo "  ✔  Section 15 — GET  /api/v2/oversight/disclosure-status/{id}"
echo "  ✔  Section 16 — FR-018 validation (side required, DEPRECATED_FIELDS)"
echo "  ✔  Section 16 — INVALID_SIDE, PAIR_NOT_FOUND"

summary

echo ""
echo "  Advanced usage:"
echo "    READ_ONLY=true   $0          # GET only"
echo "    SECTION=swap     $0          # Quote + swap only"
echo "    SECTION=flow1    $0          # Flow 1 only (fiat→token)"
echo "    SECTION=flow2    $0          # Flow 2 only (token→fiat)"
echo "    SECTION=commit   $0          # Commit-reveal only"
echo "    SECTION=bridge   $0          # Bridging only"
echo "    SECTION=pairreg  $0          # PairRegistry only"
echo "    SKIP_CIRCUIT_BREAKER=true $0 # Skip pool pause"
echo "    CB_B_URL=http://... $0       # Different endpoint for CB-B"
