#!/usr/bin/env bash
# tryout-htlc-cross-spoke-a-c-d-b-full.sh — Cross-spoke HTLC with full onboarding
#
# Scenario:
#   - Spoke-A: Bank-A (sender) -> Bank-C (receiver)
#   - Spoke-B: Bank-D (sender) -> Bank-B (receiver)
#
# The two legs are coupled by the same hash_lock/secret for atomic behavior.
#
# Phases:
#   0) Onboarding (auto; optional skip)
#   1) Authentication
#   2) Mint + initial balances
#   3) Lock on Spoke-A (A->C)
#   4) Lock on Spoke-B (D->B with same hash)
#   5) Settle + relay/fallback
#   6) Verification + summary
#
# Prerequisites:
#   - Backend stack up for all entities
#   - Cacti relay running (for auto-settle path)
#   - curl, jq
#
# Usage:
#   ./tryouts/tryout-htlc-cross-spoke-a-c-d-b-full.sh
#   SKIP_ONBOARDING=true ./tryouts/tryout-htlc-cross-spoke-a-c-d-b-full.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BANK_A_URL="${BANK_A_URL:-http://localhost:18080/api/v1}"
BANK_B_URL="${BANK_B_URL:-http://localhost:28080/api/v1}"
BANK_C_URL="${BANK_C_URL:-http://localhost:48080/api/v1}"
BANK_D_URL="${BANK_D_URL:-http://localhost:58080/api/v1}"
CB_A_URL="${CB_A_URL:-http://localhost:38080/api/v1}"
CB_B_URL="${CB_B_URL:-http://localhost:60080/api/v1}"

BANK_A_ENV="${BANK_A_ENV:-backend/config/.env.infra.bank-a}"
BANK_B_ENV="${BANK_B_ENV:-backend/config/.env.infra.bank-b}"
BANK_C_ENV="${BANK_C_ENV:-backend/config/.env.infra.bank-c}"
BANK_D_ENV="${BANK_D_ENV:-backend/config/.env.infra.bank-d}"
CB_A_ENV="${CB_A_ENV:-backend/config/.env.infra.central-bank-a}"
CB_B_ENV="${CB_B_ENV:-backend/config/.env.infra.central-bank-b}"

MINT_AMOUNT="${MINT_AMOUNT:-10000000}"
LOCK_AMOUNT="${LOCK_AMOUNT:-1000000}"
SKIP_ONBOARDING="${SKIP_ONBOARDING:-false}"
HTTP_CONNECT_TIMEOUT_SECONDS="${HTTP_CONNECT_TIMEOUT_SECONDS:-10}"
HTTP_TIMEOUT_SECONDS="${HTTP_TIMEOUT_SECONDS:-120}"
LOCK_WITH_HASH_TIMEOUT_SECONDS="${LOCK_WITH_HASH_TIMEOUT_SECONDS:-420}"

# Paladin identities (intra-spoke)
IDENTITY_BANK_A="funded_operator@spoke-a-bank-a"
IDENTITY_BANK_B="funded_operator@spoke-b-bank-b"
IDENTITY_BANK_C="funded_operator@spoke-a-bank-c"
IDENTITY_BANK_D="funded_operator@spoke-b-bank-d"

# Timeout waiting for Cacti to auto-settle on Spoke-B
RELAY_SETTLE_TIMEOUT="${RELAY_SETTLE_TIMEOUT:-30}"

ONBOARDING_SCRIPTS=(
  "tryout-spoke-a-bank-a.sh"
  "tryout-spoke-a-bank-c.sh"
  "tryout-spoke-b-bank-b.sh"
  "tryout-spoke-b-bank-d.sh"
)

# Globals populated during execution
BANK_A_TOKEN=""
BANK_B_TOKEN=""
BANK_C_TOKEN=""
BANK_D_TOKEN=""
CB_A_TOKEN=""
CB_B_TOKEN=""

MINT_TX_A=""
MINT_TX_D=""
BALANCE_A_BEFORE=""
BALANCE_B_BEFORE=""
BALANCE_C_BEFORE=""
BALANCE_D_BEFORE=""

# Spoke-A HTLC (Bank-A -> Bank-C)
CONTRACT_ID_A=""
HASH_LOCK_A=""
SECRET_A=""
ZETO_TX_LOCK_A=""
ZETO_TX_SETTLE_A=""

# Spoke-B HTLC (Bank-D -> Bank-B)
CONTRACT_ID_D=""
ZETO_TX_LOCK_D=""
ZETO_TX_SETTLE_D=""

BALANCE_A_AFTER=""
BALANCE_B_AFTER=""
BALANCE_C_AFTER=""
BALANCE_D_AFTER=""

# ---------------------------------------------------------------------------
# Dependency checks
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
  for f in "$BANK_A_ENV" "$BANK_B_ENV" "$BANK_C_ENV" "$BANK_D_ENV" "$CB_A_ENV" "$CB_B_ENV"; do
    if [ ! -f "$f" ]; then
      echo "ERROR: Env file '$f' not found." >&2
      exit 1
    fi
  done
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

# ---------------------------------------------------------------------------
# Helper: HTTP request with status code validation
# ---------------------------------------------------------------------------

http_post() {
  local url=$1 token=$2 payload=$3 expected=$4
  local tmp code body
  echo "  Payload:" >&2
  echo "$payload" | jq . >&2
  echo "  -> POST $url" >&2
  tmp=$(mktemp)
  if ! code=$(curl -sS -X POST "$url" \
    --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
    --max-time "$HTTP_TIMEOUT_SECONDS" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    --data "$payload" \
    -o "$tmp" -w "%{http_code}"); then
    body=$(cat "$tmp" 2>/dev/null || true)
    rm -f "$tmp"
    echo "ERROR: POST $url failed due to timeout/network issue." >&2
    echo "Hint: adjust HTTP_TIMEOUT_SECONDS (current: $HTTP_TIMEOUT_SECONDS)." >&2
    if [ -n "$body" ]; then
      echo "Partial response: $body" >&2
    fi
    exit 1
  fi
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "$expected" ]; then
    echo "ERROR: POST $url failed (HTTP $code, expected $expected)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  echo "  Response:" >&2
  echo "$body" | jq . >&2
  echo "$body"
}

http_get() {
  local url=$1 token=$2 expected=$3
  local tmp code body
  echo "  -> GET $url" >&2
  tmp=$(mktemp)
  if ! code=$(curl -sS -X GET "$url" \
    --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
    --max-time "$HTTP_TIMEOUT_SECONDS" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    -o "$tmp" -w "%{http_code}"); then
    body=$(cat "$tmp" 2>/dev/null || true)
    rm -f "$tmp"
    echo "ERROR: GET $url failed due to timeout/network issue." >&2
    echo "Hint: adjust HTTP_TIMEOUT_SECONDS (current: $HTTP_TIMEOUT_SECONDS)." >&2
    if [ -n "$body" ]; then
      echo "Partial response: $body" >&2
    fi
    exit 1
  fi
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "$expected" ]; then
    echo "ERROR: GET $url failed (HTTP $code, expected $expected)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  echo "  Response:" >&2
  echo "$body" | jq . >&2
  echo "$body"
}

bank_login() {
  local bank_name=$1 bank_url=$2 env_file=$3 client_id=$4
  local kc_secret login_resp token payload
  kc_secret=$(read_kc_secret "$env_file")
  payload="{\"clientId\": \"$client_id\", \"clientSecret\": \"$kc_secret\"}"
  echo "  Payload:" >&2
  echo "$payload" | jq . >&2
  echo "  -> POST $bank_url/auth/login" >&2
  if ! login_resp=$(curl -sS -X POST "$bank_url/auth/login" \
    --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
    --max-time "$HTTP_TIMEOUT_SECONDS" \
    -H "Content-Type: application/json" \
    -d "$payload"); then
    echo "ERROR: $bank_name operator login request timed out/failed." >&2
    exit 1
  fi
  token=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$token" ]; then
    echo "ERROR: $bank_name operator login failed." >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
  echo "  Response:" >&2
  echo "$login_resp" | jq . >&2
  echo "$token"
}

check_balance() {
  local bank_url=$1 token=$2
  local resp
  resp=$(http_get "$bank_url/token/balance" "$token" "200")
  echo "$resp" | jq -r '.balance // "0"'
}

mint_tokens() {
  local bank_url=$1 token=$2 identity=$3
  local resp
  resp=$(http_post "$bank_url/token/mint" "$token" \
    "$(jq -n --arg to "$identity" --arg amt "$MINT_AMOUNT" \
      '{to: $to, amount: $amt}')" \
    "201")
  echo "$resp" | jq -r '.tx_hash // empty'
}

# ---------------------------------------------------------------------------
# Phase 0 - Onboarding
# ---------------------------------------------------------------------------

run_onboarding_script() {
  local script_name=$1
  local script_path="$SCRIPT_DIR/$script_name"

  if [ ! -f "$script_path" ]; then
    echo "ERROR: onboarding script not found: $script_path" >&2
    exit 1
  fi

  echo ""
  echo "--- Running $script_name ---"
  (
    cd "$REPO_ROOT"
    bash "$script_path"
  )
}

phase_0_onboarding() {
  if [ "$SKIP_ONBOARDING" = "true" ]; then
    echo ""
    echo "=== [0] Onboarding skipped (SKIP_ONBOARDING=true) ==="
    return 0
  fi

  echo ""
  echo "=== [0] Onboarding all banks (A, C, B, D) ==="
  for script in "${ONBOARDING_SCRIPTS[@]}"; do
    run_onboarding_script "$script"
  done
}

# ---------------------------------------------------------------------------
# Phase 3 - Lock on Spoke-A (A -> C)
# ---------------------------------------------------------------------------

lock_htlc_spoke_a() {
  local time_lock resp
  time_lock=$(( $(date +%s) + 3600 ))
  resp=$(http_post "$BANK_A_URL/htlc/lock" "$BANK_A_TOKEN" \
    "$(jq -n \
      --arg aid "FX_CROSS_SPOKE_A_C_D_B_001" \
      --arg rcv "$IDENTITY_BANK_C" \
      --arg amt "$LOCK_AMOUNT" \
      --argjson tl "$time_lock" \
      '{agreement_id: $aid, receiver: $rcv, amount: $amt, time_lock: $tl}')" \
    "201")

  CONTRACT_ID_A=$(echo "$resp" | jq -r '.contract_id')
  HASH_LOCK_A=$(echo "$resp" | jq -r '.hash_lock')
  ZETO_TX_LOCK_A=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

verify_lock_spoke_a() {
  local resp state_val
  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  SECRET_A=$(echo "$resp" | jq -r '.secret // empty')

  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected HTLC_STATE_LOCKED on Spoke-A, got '$state_val'." >&2
    exit 1
  fi
  if [ -z "$SECRET_A" ]; then
    echo "ERROR: Secret not found in Spoke-A HTLC status." >&2
    exit 1
  fi

  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Phase 4 - Lock on Spoke-B (D -> B with same hash)
# ---------------------------------------------------------------------------

lock_htlc_spoke_b() {
  local time_lock resp payload tmp code body
  time_lock=$(( $(date +%s) + 1800 ))
  payload=$(jq -n \
    --arg aid "FX_CROSS_SPOKE_A_C_D_B_001" \
    --arg rcv "$IDENTITY_BANK_B" \
    --arg amt "$LOCK_AMOUNT" \
    --argjson tl "$time_lock" \
    --arg hl "$HASH_LOCK_A" \
    '{agreement_id: $aid, receiver: $rcv, amount: $amt, time_lock: $tl, hash_lock: $hl}')

  echo "  Payload:" >&2
  echo "$payload" | jq . >&2
  echo "  -> POST $BANK_D_URL/htlc/lock-with-hash" >&2
  tmp=$(mktemp)
  if ! code=$(curl -sS -X POST "$BANK_D_URL/htlc/lock-with-hash" \
    --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
    --max-time "$LOCK_WITH_HASH_TIMEOUT_SECONDS" \
    --cookie "access_token=$BANK_D_TOKEN" \
    -H "Content-Type: application/json" \
    --data "$payload" \
    -o "$tmp" -w "%{http_code}"); then
    body=$(cat "$tmp" 2>/dev/null || true)
    rm -f "$tmp"
    echo "WARN: lock-with-hash timed out from Bank-D endpoint. Trying recovery by search..." >&2
    if [ -n "$body" ]; then
      echo "Partial response: $body" >&2
    fi

    # Recovery path: request may have been processed even if client timed out.
    # Search by agreement_id + receiver and pick the latest locked contract.
    resp=$(http_get "$BANK_D_URL/htlc/search?agreement_id=FX_CROSS_SPOKE_A_C_D_B_001&receiver=$IDENTITY_BANK_B&state=LOCKED" "$BANK_D_TOKEN" "200")
    CONTRACT_ID_D=$(echo "$resp" | jq -r '.locks[-1].contract_id // empty')
    ZETO_TX_LOCK_D=$(echo "$resp" | jq -r '.locks[-1].zeto_tx_hash // empty')
    if [ -z "$CONTRACT_ID_D" ]; then
      echo "ERROR: lock-with-hash timed out and no LOCKED HTLC found for recovery." >&2
      echo "Hint: retry with LOCK_WITH_HASH_TIMEOUT_SECONDS=900." >&2
      exit 1
    fi
    echo "  Recovered contract_id from search: $CONTRACT_ID_D" >&2
    return 0
  fi
  body=$(cat "$tmp")
  rm -f "$tmp"
  if [ "$code" != "201" ]; then
    echo "ERROR: POST $BANK_D_URL/htlc/lock-with-hash failed (HTTP $code, expected 201)." >&2
    echo "Response: $body" >&2
    exit 1
  fi

  echo "  Response:" >&2
  echo "$body" | jq . >&2
  resp="$body"
  CONTRACT_ID_D=$(echo "$resp" | jq -r '.contract_id')
  ZETO_TX_LOCK_D=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

verify_lock_spoke_b() {
  local resp state_val
  resp=$(http_get "$BANK_D_URL/htlc/status/$CONTRACT_ID_D" "$BANK_D_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')

  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected HTLC_STATE_LOCKED on Spoke-B, got '$state_val'." >&2
    exit 1
  fi

  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Phase 5 - Settle
# ---------------------------------------------------------------------------

settle_spoke_a() {
  local resp
  resp=$(http_post "$BANK_A_URL/htlc/settle" "$BANK_A_TOKEN" \
    "$(jq -n \
      --arg cid "$CONTRACT_ID_A" \
      --arg sec "$SECRET_A" \
      '{contract_id: $cid, secret: $sec}')" \
    "200")
  ZETO_TX_SETTLE_A=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

wait_for_relay_or_settle_manual() {
  local i resp state_val
  echo "  Waiting up to ${RELAY_SETTLE_TIMEOUT}s for Cacti to settle on Spoke-B..."

  for (( i=0; i<RELAY_SETTLE_TIMEOUT; i+=3 )); do
    sleep 3
    resp=$(http_get "$BANK_D_URL/htlc/status/$CONTRACT_ID_D" "$BANK_D_TOKEN" "200" 2>/dev/null || true)
    state_val=$(echo "$resp" | jq -r '.state // empty' 2>/dev/null || true)

    if [ "$state_val" = "HTLC_STATE_SETTLED" ]; then
      echo "  Relay auto-settled on Spoke-B  ✓"
      ZETO_TX_SETTLE_D=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
      return 0
    fi

    echo "  ... still waiting (${i}s, state=$state_val)"
  done

  echo "  Relay did not settle in time - settling manually on Spoke-B via Bank-D"
  resp=$(http_post "$BANK_D_URL/htlc/settle" "$BANK_D_TOKEN" \
    "$(jq -n \
      --arg cid "$CONTRACT_ID_D" \
      --arg sec "$SECRET_A" \
      '{contract_id: $cid, secret: $sec}')" \
    "200")
  ZETO_TX_SETTLE_D=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

verify_settled_both() {
  local resp state_val

  echo "  Verifying Spoke-A..."
  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_SETTLED" ]; then
    echo "ERROR: Spoke-A HTLC expected SETTLED, got '$state_val'." >&2
    exit 1
  fi
  echo "  Spoke-A: HTLC_STATE_SETTLED  ✓"

  echo "  Verifying Spoke-B..."
  resp=$(http_get "$BANK_D_URL/htlc/status/$CONTRACT_ID_D" "$BANK_D_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_SETTLED" ]; then
    echo "ERROR: Spoke-B HTLC expected SETTLED, got '$state_val'." >&2
    exit 1
  fi
  echo "  Spoke-B: HTLC_STATE_SETTLED  ✓"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  require_cmds
  require_env_files

  echo ""
  echo "=============================================================="
  echo "  HTLC Cross-Spoke Full Tryout (A->C and D->B)"
  echo "  Spoke-A: Bank-A -> Bank-C   |   Spoke-B: Bank-D -> Bank-B"
  echo ""
  echo "  Bank-A Gateway : $BANK_A_URL"
  echo "  Bank-B Gateway : $BANK_B_URL"
  echo "  Bank-C Gateway : $BANK_C_URL"
  echo "  Bank-D Gateway : $BANK_D_URL"
  echo "  Mint amount    : $MINT_AMOUNT tCeBM"
  echo "  Lock amount    : $LOCK_AMOUNT tCeBM"
  echo "  Skip onboarding: $SKIP_ONBOARDING"
  echo "=============================================================="

  phase_0_onboarding

  echo ""
  echo "=== Phase 1 - Authentication ==="

  echo ""
  echo "--- Bank-A operator login ---"
  BANK_A_TOKEN=$(bank_login "Bank-A" "$BANK_A_URL" "$BANK_A_ENV" "bank-a-client")
  echo "  BANK_A_TOKEN: ${BANK_A_TOKEN:0:60}..."

  echo ""
  echo "--- Bank-B operator login ---"
  BANK_B_TOKEN=$(bank_login "Bank-B" "$BANK_B_URL" "$BANK_B_ENV" "bank-b-client")
  echo "  BANK_B_TOKEN: ${BANK_B_TOKEN:0:60}..."

  echo ""
  echo "--- Bank-C operator login ---"
  BANK_C_TOKEN=$(bank_login "Bank-C" "$BANK_C_URL" "$BANK_C_ENV" "bank-c-client")
  echo "  BANK_C_TOKEN: ${BANK_C_TOKEN:0:60}..."

  echo ""
  echo "--- Bank-D operator login ---"
  BANK_D_TOKEN=$(bank_login "Bank-D" "$BANK_D_URL" "$BANK_D_ENV" "bank-d-client")
  echo "  BANK_D_TOKEN: ${BANK_D_TOKEN:0:60}..."

  echo ""
  echo "--- Central-Bank-A operator login ---"
  CB_A_TOKEN=$(bank_login "CB-A" "$CB_A_URL" "$CB_A_ENV" "central-bank-a-client")
  echo "  CB_A_TOKEN: ${CB_A_TOKEN:0:60}..."

  echo ""
  echo "--- Central-Bank-B operator login ---"
  CB_B_TOKEN=$(bank_login "CB-B" "$CB_B_URL" "$CB_B_ENV" "central-bank-b-client")
  echo "  CB_B_TOKEN: ${CB_B_TOKEN:0:60}..."

  echo ""
  echo "=== Phase 2 - Mint + initial balances ==="

  echo ""
  echo "--- Mint $MINT_AMOUNT tCeBM for Bank-A (via CB-A) ---"
  echo "  to: $IDENTITY_BANK_A"
  MINT_TX_A=$(mint_tokens "$CB_A_URL" "$CB_A_TOKEN" "$IDENTITY_BANK_A")
  echo "  mint_tx_hash: ${MINT_TX_A:-"(noop)"}"

  echo ""
  echo "--- Mint $MINT_AMOUNT tCeBM for Bank-D (via CB-B) ---"
  echo "  to: $IDENTITY_BANK_D"
  MINT_TX_D=$(mint_tokens "$CB_B_URL" "$CB_B_TOKEN" "$IDENTITY_BANK_D")
  echo "  mint_tx_hash: ${MINT_TX_D:-"(noop)"}"

  echo ""
  echo "--- Check initial balances ---"
  BALANCE_A_BEFORE=$(check_balance "$BANK_A_URL" "$BANK_A_TOKEN")
  BALANCE_B_BEFORE=$(check_balance "$BANK_B_URL" "$BANK_B_TOKEN")
  BALANCE_C_BEFORE=$(check_balance "$BANK_C_URL" "$BANK_C_TOKEN")
  BALANCE_D_BEFORE=$(check_balance "$BANK_D_URL" "$BANK_D_TOKEN")
  echo "  Bank-A: $BALANCE_A_BEFORE"
  echo "  Bank-B: $BALANCE_B_BEFORE"
  echo "  Bank-C: $BALANCE_C_BEFORE"
  echo "  Bank-D: $BALANCE_D_BEFORE"

  echo ""
  echo "=== Phase 3 - Lock on Spoke-A (Bank-A -> Bank-C) ==="
  echo "  agreement: FX_CROSS_SPOKE_A_C_D_B_001"
  echo "  receiver : $IDENTITY_BANK_C"
  lock_htlc_spoke_a
  echo "  contract_id : $CONTRACT_ID_A"
  echo "  hash_lock   : ${HASH_LOCK_A:0:32}..."
  echo "  zeto_tx_hash: ${ZETO_TX_LOCK_A:-"(pending)"}"

  echo ""
  echo "--- Verify lock on Spoke-A and retrieve secret ---"
  verify_lock_spoke_a
  echo "  state : HTLC_STATE_LOCKED  ✓"
  echo "  secret: ${SECRET_A:0:16}... (${#SECRET_A} hex chars)"

  echo ""
  echo "=== Phase 4 - Lock on Spoke-B (Bank-D -> Bank-B) ==="
  echo "  same hash_lock from Spoke-A: ${HASH_LOCK_A:0:32}..."
  echo "  receiver: $IDENTITY_BANK_B"
  lock_htlc_spoke_b
  echo "  contract_id : $CONTRACT_ID_D"
  echo "  zeto_tx_hash: ${ZETO_TX_LOCK_D:-"(pending)"}"

  echo ""
  echo "--- Verify lock on Spoke-B ---"
  verify_lock_spoke_b
  echo "  state: HTLC_STATE_LOCKED  ✓"

  echo ""
  echo "=== Phase 5 - Settle + relay ==="

  echo ""
  echo "--- Settle on Spoke-A (Bank-A reveals secret) ---"
  echo "  contract_id: $CONTRACT_ID_A"
  settle_spoke_a
  echo "  zeto_tx_hash: ${ZETO_TX_SETTLE_A:-"(pending)"}"

  echo ""
  echo "--- Wait for relay settle on Spoke-B (or fallback manual settle) ---"
  wait_for_relay_or_settle_manual
  echo "  zeto_tx_hash: ${ZETO_TX_SETTLE_D:-"(pending)"}"

  echo ""
  echo "--- Verify settled state on both spokes ---"
  verify_settled_both

  echo ""
  echo "=== Phase 6 - Verification ==="

  echo ""
  echo "--- Check final balances ---"
  BALANCE_A_AFTER=$(check_balance "$BANK_A_URL" "$BANK_A_TOKEN")
  BALANCE_B_AFTER=$(check_balance "$BANK_B_URL" "$BANK_B_TOKEN")
  BALANCE_C_AFTER=$(check_balance "$BANK_C_URL" "$BANK_C_TOKEN")
  BALANCE_D_AFTER=$(check_balance "$BANK_D_URL" "$BANK_D_TOKEN")
  echo "  Bank-A: $BALANCE_A_BEFORE -> $BALANCE_A_AFTER (locked $LOCK_AMOUNT)"
  echo "  Bank-D: $BALANCE_D_BEFORE -> $BALANCE_D_AFTER (locked $LOCK_AMOUNT)"
  echo "  Bank-C: $BALANCE_C_BEFORE -> $BALANCE_C_AFTER (received $LOCK_AMOUNT)"
  echo "  Bank-B: $BALANCE_B_BEFORE -> $BALANCE_B_AFTER (received $LOCK_AMOUNT)"

  echo ""
  echo "--- Search settled HTLCs ---"
  echo "  Spoke-A (Bank-A view):"
  http_get "$BANK_A_URL/htlc/search?state=SETTLED" "$BANK_A_TOKEN" "200" | jq .
  echo "  Spoke-B (Bank-D view):"
  http_get "$BANK_D_URL/htlc/search?state=SETTLED" "$BANK_D_TOKEN" "200" | jq .

  echo ""
  echo "=============================================================="
  echo "  HTLC Cross-Spoke Full Tryout - Summary"
  echo "=============================================================="
  echo ""
  echo "  -- Spoke-A HTLC (Bank-A -> Bank-C) --"
  echo "  Contract ID    : $CONTRACT_ID_A"
  echo "  Hash Lock      : ${HASH_LOCK_A:0:32}..."
  echo "  Secret         : ${SECRET_A:0:16}..."
  echo "  Lock Zeto TX   : ${ZETO_TX_LOCK_A:-"(n/a)"}"
  echo "  Settle Zeto TX : ${ZETO_TX_SETTLE_A:-"(n/a)"}"
  echo "  State          : SETTLED ✓"
  echo ""
  echo "  -- Spoke-B HTLC (Bank-D -> Bank-B) --"
  echo "  Contract ID    : $CONTRACT_ID_D"
  echo "  Lock Zeto TX   : ${ZETO_TX_LOCK_D:-"(n/a)"}"
  echo "  Settle Zeto TX : ${ZETO_TX_SETTLE_D:-"(n/a)"}"
  echo "  State          : SETTLED ✓"
  echo ""
  echo "  -- Balances --"
  echo "  Bank-A: $BALANCE_A_BEFORE -> $BALANCE_A_AFTER"
  echo "  Bank-B: $BALANCE_B_BEFORE -> $BALANCE_B_AFTER"
  echo "  Bank-C: $BALANCE_C_BEFORE -> $BALANCE_C_AFTER"
  echo "  Bank-D: $BALANCE_D_BEFORE -> $BALANCE_D_AFTER"
  echo ""
  echo "=============================================================="
  echo "  Completed successfully"
  echo "=============================================================="
  echo ""
}

main "$@"
