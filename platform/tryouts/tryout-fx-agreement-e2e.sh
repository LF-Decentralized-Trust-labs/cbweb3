#!/usr/bin/env bash
# tryout-fx-agreement-e2e.sh -- End-to-end FX Agreement + HTLC flow
#
# This script uses ONLY platform API endpoints (no direct Keycloak realm calls).
#
# Flow:
#   1) Optional onboarding for bank-a and bank-b
#   2) API login for bank-a, bank-b, cb-a, cb-b
#   3) Optional mint on CB APIs
#   4) Propose/accept FX agreement
#   5) Validate persistence + audit + cross-spoke sync
#   6) Full HTLC cycle (lock initiator, lock responder, settle, relay settle)

set -euo pipefail

BANK_A_URL="${BANK_A_URL:-http://localhost:18080/api/v1}"
BANK_B_URL="${BANK_B_URL:-http://localhost:28080/api/v1}"
CB_A_URL="${CB_A_URL:-http://localhost:38080/api/v1}"
CB_B_URL="${CB_B_URL:-http://localhost:60080/api/v1}"

BANK_A_ENV="${BANK_A_ENV:-backend/config/.env.infra.bank-a}"
BANK_B_ENV="${BANK_B_ENV:-backend/config/.env.infra.bank-b}"
CB_A_ENV="${CB_A_ENV:-backend/config/.env.infra.central-bank-a}"
CB_B_ENV="${CB_B_ENV:-backend/config/.env.infra.central-bank-b}"

# FX/HTLC amounts must stay aligned with agreement terms:
# - spoke-a lock uses origin amount
# - spoke-b lock uses counter amount
FX_ORIGIN_AMOUNT="${FX_ORIGIN_AMOUNT:-100000}"
FX_COUNTER_AMOUNT="${FX_COUNTER_AMOUNT:-520000}"
# Backward-compatible override: if LOCK_AMOUNT is set, force both legs to same value.
LOCK_AMOUNT="${LOCK_AMOUNT:-}"
LOCK_AMOUNT_A="${LOCK_AMOUNT_A:-${LOCK_AMOUNT:-$FX_ORIGIN_AMOUNT}}"
LOCK_AMOUNT_B="${LOCK_AMOUNT_B:-${LOCK_AMOUNT:-$FX_COUNTER_AMOUNT}}"
MINT_AMOUNT="${MINT_AMOUNT:-5000000}"
RELAY_SYNC_TIMEOUT="${RELAY_SYNC_TIMEOUT:-45}"
RELAY_SETTLE_TIMEOUT="${RELAY_SETTLE_TIMEOUT:-45}"
HTTP_TIMEOUT="${HTTP_TIMEOUT:-120}"
SKIP_MINT="${SKIP_MINT:-false}"

SKIP_ONBOARDING=false
VERBOSE=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-onboarding) SKIP_ONBOARDING=true; shift ;;
    --verbose) VERBOSE=true; shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

BANK_A_TOKEN=""
BANK_B_TOKEN=""
CB_A_TOKEN=""
CB_B_TOKEN=""

TRADE_ID=""
CONTRACT_ID_A=""
CONTRACT_ID_B=""
HASH_LOCK_A=""
SECRET_A=""

IDENTITY_BANK_A="funded_operator@spoke-a-bank-a"
IDENTITY_BANK_B="funded_operator@spoke-b-bank-b"

log_info() { echo "[INFO] $*" >&2; }
log_debug() {
  if [ "$VERBOSE" = true ]; then
    echo "[DEBUG] $*" >&2
  fi
}
log_step() {
  echo "" >&2
  echo "============================================================" >&2
  echo "STEP: $*" >&2
  echo "============================================================" >&2
}
log_error() { echo "[ERROR] $*" >&2; exit 1; }
log_ok() { echo "[OK] $*" >&2; }

fail_unavailable() {
  local method=$1 url=$2 body=$3
  {
    echo "[ERROR] $method $url failed: backend service unavailable (rpc Unavailable)."
    echo "[ERROR] Response: $body"
    echo "[ERROR] Check the corresponding payment-orchestrator container and bootstrap/dependency logs."
  } >&2
  exit 1
}

require_cmds() {
  for cmd in curl jq; do
    command -v "$cmd" >/dev/null 2>&1 || log_error "'$cmd' not found"
  done
}

require_env_files() {
  for f in "$BANK_A_ENV" "$BANK_B_ENV" "$CB_A_ENV" "$CB_B_ENV"; do
    [ -f "$f" ] || log_error "env file not found: $f"
  done
}

read_kc_secret() {
  local env_file=$1
  local secret
  secret=$(grep KC_CLIENT_SECRET "$env_file" | cut -d= -f2-)
  [ -n "$secret" ] || log_error "KC_CLIENT_SECRET missing in $env_file"
  echo "$secret"
}

http_post() {
  local url=$1 token=$2 payload=$3 expected=$4
  local tmp code body
  log_debug "POST $url"
  tmp=$(mktemp)
  if ! code=$(curl -sS -X POST "$url" \
    --connect-timeout 10 \
    --max-time "$HTTP_TIMEOUT" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    --data "$payload" \
    -o "$tmp" -w "%{http_code}"); then
    body=$(cat "$tmp" 2>/dev/null || true)
    rm -f "$tmp"
    log_error "POST failed: $url ${body:+| $body}"
  fi
  body=$(cat "$tmp")
  rm -f "$tmp"
  if echo "$body" | grep -q 'rpc error: code = Unavailable'; then
    fail_unavailable "POST" "$url" "$body"
  fi
  if [ "$code" != "$expected" ]; then
    log_error "POST $url returned HTTP $code (expected $expected). Response: $body"
  fi
  echo "$body"
}

http_get() {
  local url=$1 token=$2 expected=$3
  local tmp code body
  log_debug "GET $url"
  tmp=$(mktemp)
  if ! code=$(curl -sS -X GET "$url" \
    --connect-timeout 10 \
    --max-time "$HTTP_TIMEOUT" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    -o "$tmp" -w "%{http_code}"); then
    body=$(cat "$tmp" 2>/dev/null || true)
    rm -f "$tmp"
    log_error "GET failed: $url ${body:+| $body}"
  fi
  body=$(cat "$tmp")
  rm -f "$tmp"
  if echo "$body" | grep -q 'rpc error: code = Unavailable'; then
    fail_unavailable "GET" "$url" "$body"
  fi
  if [ "$code" != "$expected" ]; then
    log_error "GET $url returned HTTP $code (expected $expected). Response: $body"
  fi
  echo "$body"
}

poll_fx_state_quick() {
  local base_url=$1 token=$2 trade_id=$3
  local tmp code body state
  tmp=$(mktemp)
  code=$(curl -sS -X GET "$base_url/payments/fx/agreements/$trade_id" \
    --connect-timeout 3 \
    --max-time 5 \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    -o "$tmp" -w "%{http_code}" 2>/dev/null || echo "000")
  body=$(cat "$tmp" 2>/dev/null || true)
  rm -f "$tmp"
  if [ "$code" = "200" ]; then
    state=$(echo "$body" | jq -r '.agreement.state // empty' 2>/dev/null || true)
    echo "$state"
    return 0
  fi
  echo ""
  return 0
}

api_login() {
  local name=$1 base_url=$2 env_file=$3 client_id=$4
  local kc_secret payload resp token
  kc_secret=$(read_kc_secret "$env_file")
  payload=$(jq -n --arg cid "$client_id" --arg sec "$kc_secret" '{clientId:$cid, clientSecret:$sec}')
  resp=$(http_post "$base_url/auth/login" "" "$payload" "200")
  token=$(echo "$resp" | jq -r '.accessToken // empty')
  [ -n "$token" ] || log_error "login failed for $name: $resp"
  log_ok "$name authenticated"
  echo "$token"
}

run_onboarding() {
  local script_dir
  script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

  log_step "Optional onboarding: bank-a"
  BANK_URL="$BANK_A_URL" CB_URL="$CB_A_URL" BANK_ENV="$BANK_A_ENV" CB_ENV="$CB_A_ENV" \
    bash "$script_dir/tryout-spoke-a-bank-a.sh"

  log_step "Optional onboarding: bank-b"
  BANK_URL="$BANK_B_URL" CB_URL="$CB_B_URL" BANK_ENV="$BANK_B_ENV" CB_ENV="$CB_B_ENV" \
    bash "$script_dir/tryout-spoke-b-bank-b.sh"
}

mint_if_needed() {
  [ "$SKIP_MINT" = "true" ] && { log_info "Skipping mint (SKIP_MINT=true)"; return 0; }

  log_step "Minting test balances"
  http_post "$CB_A_URL/token/mint" "$CB_A_TOKEN" \
    "$(jq -n --arg to "$IDENTITY_BANK_A" --arg amt "$MINT_AMOUNT" '{to:$to, amount:$amt}')" \
    "201" >/dev/null
  http_post "$CB_B_URL/token/mint" "$CB_B_TOKEN" \
    "$(jq -n --arg to "$IDENTITY_BANK_B" --arg amt "$MINT_AMOUNT" '{to:$to, amount:$amt}')" \
    "201" >/dev/null
  log_ok "Mint completed"
}

propose_fx() {
  log_step "Propose FX agreement"
  TRADE_ID="TRADE-$(date +%s%N | cut -b1-13)"

  local payload
  payload=$(jq -n \
    --arg tid "$TRADE_ID" \
    --arg cpb "bank-b" \
    --arg org "bank-a" \
    --arg sa "bank-a" \
    --arg cust "bank-b" \
    --arg ben "bank-b" \
    --arg oamt "$FX_ORIGIN_AMOUNT" \
    --arg camt "$FX_COUNTER_AMOUNT" \
    --arg ocur "USD" \
    --arg ccur "BRL" \
    --arg rate "5.2" \
    --arg sra "$IDENTITY_BANK_A" \
    --arg srb "$IDENTITY_BANK_B" \
    --argjson exp "$(( $(date +%s) + 3600 ))" \
    '{
      trade_id: $tid,
      counterparty_b: $cpb,
      originator: $org,
      settlement_agent: $sa,
      custodian: $cust,
      beneficiary: $ben,
      origin_amount: $oamt,
      counter_amount: $camt,
      origin_currency: $ocur,
      counter_currency: $ccur,
      rate: $rate,
      expiry_date: $exp,
      spoke_a_receiver: $sra,
      spoke_b_receiver: $srb,
      on_behalf: false
    }')

  http_post "$BANK_A_URL/payments/fx/agreements" "$BANK_A_TOKEN" "$payload" "201" >/dev/null
  log_ok "FX proposed: $TRADE_ID"
}

validate_fx_persistence_and_audit() {
  log_step "Validate FX persistence and audit"
  local get_resp state audit_resp total

  get_resp=$(http_get "$BANK_A_URL/payments/fx/agreements/$TRADE_ID" "$BANK_A_TOKEN" "200")
  state=$(echo "$get_resp" | jq -r '.agreement.state // empty')
  case "$state" in
    FX_STATE_PROPOSED|PROPOSED) ;;
    *) log_error "unexpected FX state after propose: $state" ;;
  esac

  audit_resp=$(http_get "$BANK_A_URL/payments/fx/agreements/$TRADE_ID/audit" "$BANK_A_TOKEN" "200")
  total=$(echo "$audit_resp" | jq -r '.total // 0')
  [ "$total" -ge 1 ] || log_error "expected at least one audit event, got $total"
  log_ok "Persistence and audit OK"
}

accept_fx_and_wait_sync() {
  log_step "Accept FX and wait cross-spoke sync"

  local i state
  log_info "Waiting for proposal propagation to Bank-B before accept..."
  for ((i=0; i<RELAY_SYNC_TIMEOUT; i+=3)); do
    sleep 3
    state=$(poll_fx_state_quick "$BANK_B_URL" "$BANK_B_TOKEN" "$TRADE_ID")
    if [ "$state" = "FX_STATE_PROPOSED" ] || [ "$state" = "PROPOSED" ] || [ "$state" = "FX_STATE_ACCEPTED" ] || [ "$state" = "ACCEPTED" ]; then
      log_ok "Proposal visible on Bank-B: $state"
      break
    fi
  done
  if [ "$state" != "FX_STATE_PROPOSED" ] && [ "$state" != "PROPOSED" ] && [ "$state" != "FX_STATE_ACCEPTED" ] && [ "$state" != "ACCEPTED" ]; then
    log_error "timed out waiting FX proposal propagation to Bank-B"
  fi

  http_post "$BANK_B_URL/payments/fx/agreements/$TRADE_ID/accept" "$BANK_B_TOKEN" '{"on_behalf":false}' "200" >/dev/null

  state=""
  for ((i=0; i<RELAY_SYNC_TIMEOUT; i+=3)); do
    sleep 3
    state=$(poll_fx_state_quick "$BANK_A_URL" "$BANK_A_TOKEN" "$TRADE_ID")
    if [ "$state" = "FX_STATE_ACCEPTED" ] || [ "$state" = "ACCEPTED" ]; then
      log_ok "Cross-spoke sync confirmed: $state"
      return 0
    fi
  done
  log_error "timed out waiting FX acceptance sync"
}

lock_both_sides() {
  log_step "HTLC lock on both spokes"

  local lock_a_resp lock_a_status lock_b_resp lock_b_status

  lock_a_resp=$(http_post "$BANK_A_URL/htlc/lock" "$BANK_A_TOKEN" \
    "$(jq -n --arg aid "$TRADE_ID" --arg r "$IDENTITY_BANK_A" --arg amt "$LOCK_AMOUNT_A" '{agreement_id:$aid,receiver:$r,amount:$amt}')" \
    "201")
  CONTRACT_ID_A=$(echo "$lock_a_resp" | jq -r '.contract_id')

  lock_a_status=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200")
  HASH_LOCK_A=$(echo "$lock_a_status" | jq -r '.hash_lock // empty')
  SECRET_A=$(echo "$lock_a_status" | jq -r '.secret // empty')
  [ -n "$HASH_LOCK_A" ] || log_error "hash_lock missing on initiator lock status"
  [ -n "$SECRET_A" ] || log_error "secret missing on initiator lock status"

  lock_b_resp=$(http_post "$BANK_B_URL/htlc/lock-with-hash" "$BANK_B_TOKEN" \
    "$(jq -n --arg aid "$TRADE_ID" --arg r "$IDENTITY_BANK_B" --arg amt "$LOCK_AMOUNT_B" --arg hl "$HASH_LOCK_A" '{agreement_id:$aid,receiver:$r,amount:$amt,hash_lock:$hl}')" \
    "201")
  CONTRACT_ID_B=$(echo "$lock_b_resp" | jq -r '.contract_id')

  lock_b_status=$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" "$BANK_B_TOKEN" "200")
  case "$(echo "$lock_b_status" | jq -r '.state // empty')" in
    HTLC_STATE_LOCKED|LOCKED) ;;
    *) log_error "responder lock is not LOCKED" ;;
  esac

  log_ok "Both HTLC locks created"
}

settle_both_sides() {
  log_step "Settle HTLC on both spokes"

  http_post "$BANK_A_URL/htlc/settle" "$BANK_A_TOKEN" \
    "$(jq -n --arg cid "$CONTRACT_ID_A" --arg sec "$SECRET_A" '{contract_id:$cid,secret:$sec}')" \
    "200" >/dev/null

  local i resp state
  state=""
  for ((i=0; i<RELAY_SETTLE_TIMEOUT; i+=3)); do
    sleep 3
    resp=$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" "$BANK_B_TOKEN" "200" 2>/dev/null || true)
    state=$(echo "$resp" | jq -r '.state // empty' 2>/dev/null || true)
    if [ "$state" = "HTLC_STATE_SETTLED" ] || [ "$state" = "SETTLED" ]; then
      log_ok "Relay settled responder HTLC"
      break
    fi
  done

  if [ "$state" != "HTLC_STATE_SETTLED" ] && [ "$state" != "SETTLED" ]; then
    log_error "relay did not complete settlement within timeout (${RELAY_SETTLE_TIMEOUT}s). No manual fallback in strict mode."
  fi

  case "$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200" | jq -r '.state // empty')" in
    HTLC_STATE_SETTLED|SETTLED) ;;
    *) log_error "initiator HTLC is not settled" ;;
  esac
  case "$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" "$BANK_B_TOKEN" "200" | jq -r '.state // empty')" in
    HTLC_STATE_SETTLED|SETTLED) ;;
    *) log_error "responder HTLC is not settled" ;;
  esac

  log_ok "HTLC flow completed"
}

main() {
  log_info "FX Agreement End-to-End Test Starting..."
  log_info "Skip Onboarding: $SKIP_ONBOARDING"
  log_info "Verbose: $VERBOSE"

  require_cmds
  require_env_files

  if [ "$SKIP_ONBOARDING" = false ]; then
    run_onboarding
  else
    log_info "Skipping onboarding (--skip-onboarding)"
  fi

  log_step "API logins"
  BANK_A_TOKEN=$(api_login "Bank-A" "$BANK_A_URL" "$BANK_A_ENV" "bank-a-client")
  BANK_B_TOKEN=$(api_login "Bank-B" "$BANK_B_URL" "$BANK_B_ENV" "bank-b-client")
  CB_A_TOKEN=$(api_login "CB-A" "$CB_A_URL" "$CB_A_ENV" "central-bank-a-client")
  CB_B_TOKEN=$(api_login "CB-B" "$CB_B_URL" "$CB_B_ENV" "central-bank-b-client")

  mint_if_needed
  propose_fx
  validate_fx_persistence_and_audit
  accept_fx_and_wait_sync
  lock_both_sides
  settle_both_sides

  log_step "Summary"
  log_ok "E2E flow finished successfully"
  echo "trade_id=$TRADE_ID"
  echo "contract_id_a=$CONTRACT_ID_A"
  echo "contract_id_b=$CONTRACT_ID_B"
}

main "$@"
