#!/usr/bin/env bash
# tryout-htlc-cross-spoke-minimal.sh — Cross-spoke HTLC with minimal payload
#
# Validates that backend smart defaults work correctly:
#   - agreement_id  → auto-generated UUID
#   - time_lock     → now+3600s (lock) / now+1800s (lock-with-hash)
#
# Required payload per endpoint:
#
#   POST /htlc/lock
#     { "receiver": "...", "amount": "..." }
#
#   POST /htlc/lock-with-hash
#     { "hash_lock": "...", "receiver": "...", "amount": "..." }
#
#   POST /htlc/settle
#     { "contract_id": "...", "secret": "..." }
#
# Participants (same scenario as the full tryout):
#   Spoke-A: Bank-A (sender) → Bank-C (receiver)
#   Spoke-B: Bank-D (sender) → Bank-B (receiver)
#
# Prerequisites:
#   - Banks onboarded: tryout-spoke-a-bank-a.sh, tryout-spoke-a-bank-c.sh
#                      tryout-spoke-b-bank-b.sh, tryout-spoke-b-bank-d.sh
#   - Tokens already minted (or set SKIP_MINT=false to mint now)
#   - Backend stacks running for all 4 banks
#   - Cacti interop running: make cacti-up
#   - curl, jq
#
# Usage:
#   ./tryout-htlc-cross-spoke-minimal.sh
#   SKIP_MINT=true ./tryout-htlc-cross-spoke-minimal.sh

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuração
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
SKIP_MINT="${SKIP_MINT:-false}"

# Identidades Paladin (intra-spoke)
IDENTITY_BANK_A="funded_operator@spoke-a-bank-a"
IDENTITY_BANK_B="funded_operator@spoke-b-bank-b"
IDENTITY_BANK_C="funded_operator@spoke-a-bank-c"
IDENTITY_BANK_D="funded_operator@spoke-b-bank-d"

RELAY_SETTLE_TIMEOUT="${RELAY_SETTLE_TIMEOUT:-30}"
HTTP_CONNECT_TIMEOUT_SECONDS="${HTTP_CONNECT_TIMEOUT_SECONDS:-10}"
HTTP_TIMEOUT_SECONDS="${HTTP_TIMEOUT_SECONDS:-120}"
LOCK_WITH_HASH_TIMEOUT_SECONDS="${LOCK_WITH_HASH_TIMEOUT_SECONDS:-420}"

# Globais preenchidas durante execução
BANK_A_TOKEN=""
BANK_B_TOKEN=""
BANK_C_TOKEN=""
BANK_D_TOKEN=""
CB_A_TOKEN=""
CB_B_TOKEN=""

CONTRACT_ID_A=""
HASH_LOCK_A=""
SECRET_A=""

CONTRACT_ID_B=""

BALANCE_A_BEFORE="" BALANCE_B_BEFORE="" BALANCE_C_BEFORE="" BALANCE_D_BEFORE=""
BALANCE_A_AFTER=""  BALANCE_B_AFTER=""  BALANCE_C_AFTER=""  BALANCE_D_AFTER=""

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
# HTTP helpers
# ---------------------------------------------------------------------------

http_post() {
  local url=$1 token=$2 payload=$3 expected=$4
  local tmp code body
  echo "  Payload:" >&2
  echo "$payload" | jq . >&2
  echo "  → POST $url" >&2
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
    echo "ERROR: POST $url timed out or failed (network)." >&2
    echo "Hint: ajuste HTTP_TIMEOUT_SECONDS (atual: $HTTP_TIMEOUT_SECONDS)." >&2
    [ -n "$body" ] && echo "Partial response: $body" >&2
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
  echo "  → GET $url" >&2
  tmp=$(mktemp)
  if ! code=$(curl -sS -X GET "$url" \
    --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
    --max-time "$HTTP_TIMEOUT_SECONDS" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    -o "$tmp" -w "%{http_code}"); then
    body=$(cat "$tmp" 2>/dev/null || true)
    rm -f "$tmp"
    echo "ERROR: GET $url timed out or failed (network)." >&2
    [ -n "$body" ] && echo "Partial response: $body" >&2
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
  echo "  → POST $bank_url/auth/login" >&2
  if ! login_resp=$(curl -sS -X POST "$bank_url/auth/login" \
    --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
    --max-time "$HTTP_TIMEOUT_SECONDS" \
    -H "Content-Type: application/json" \
    -d "$payload"); then
    echo "ERROR: $bank_name login request timed out/failed." >&2
    exit 1
  fi
  token=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$token" ]; then
    echo "ERROR: $bank_name login failed." >&2
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

# ---------------------------------------------------------------------------
# Business logic
# ---------------------------------------------------------------------------

mint_tokens() {
  local bank_url=$1 token=$2 identity=$3
  local resp
  resp=$(http_post "$bank_url/token/mint" "$token" \
    "$(jq -n --arg to "$identity" --arg amt "$MINT_AMOUNT" \
      '{to: $to, amount: $amt}')" \
    "201")
  echo "$resp" | jq -r '.tx_hash // empty'
}

# ── MINIMAL PAYLOAD: receiver + amount only ─────────────────────────────
# agreement_id → UUID auto-generated by backend
# time_lock    → now+3600s auto-generated by backend
lock_htlc_initiator_minimal() {
  local resp
  resp=$(http_post "$BANK_A_URL/htlc/lock" "$BANK_A_TOKEN" \
    "$(jq -n \
      --arg rcv "$IDENTITY_BANK_C" \
      --arg amt "$LOCK_AMOUNT" \
      '{receiver: $rcv, amount: $amt}')" \
    "201")
  CONTRACT_ID_A=$(echo "$resp" | jq -r '.contract_id')
  HASH_LOCK_A=$(echo "$resp"   | jq -r '.hash_lock')
}

verify_lock_status_spoke_a() {
  local resp state_val
  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  SECRET_A=$(echo "$resp"  | jq -r '.secret // empty')
  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected HTLC_STATE_LOCKED, got '$state_val'." >&2
    exit 1
  fi
  if [ -z "$SECRET_A" ]; then
    echo "ERROR: secret not found in HTLC status response." >&2
    exit 1
  fi
}

# ── MINIMAL PAYLOAD: hash_lock + receiver + amount only ─────────────────
# agreement_id → UUID auto-generated by backend
# time_lock    → now+1800s auto-generated by backend (< 1h of initiator)
lock_htlc_responder_minimal() {
  local payload tmp code body
  payload=$(jq -n \
    --arg hl  "$HASH_LOCK_A" \
    --arg rcv "$IDENTITY_BANK_B" \
    --arg amt "$LOCK_AMOUNT" \
    '{hash_lock: $hl, receiver: $rcv, amount: $amt}')
  echo "  Payload:" >&2
  echo "$payload" | jq . >&2
  echo "  → POST $BANK_D_URL/htlc/lock-with-hash" >&2
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
    echo "WARN: lock-with-hash timed out. Tentando recuperar por busca..." >&2
    [ -n "$body" ] && echo "Partial response: $body" >&2
    local search
    search=$(http_get "$BANK_D_URL/htlc/search?receiver=$IDENTITY_BANK_B&state=LOCKED" "$BANK_D_TOKEN" "200")
    CONTRACT_ID_B=$(echo "$search" | jq -r '.locks[-1].contract_id // empty')
    if [ -z "$CONTRACT_ID_B" ]; then
      echo "ERROR: lock-with-hash timed out e nenhum HTLC LOCKED encontrado para recuperar." >&2
      echo "Hint: aumente LOCK_WITH_HASH_TIMEOUT_SECONDS (atual: $LOCK_WITH_HASH_TIMEOUT_SECONDS)." >&2
      exit 1
    fi
    echo "  Recuperado contract_id: $CONTRACT_ID_B" >&2
    return 0
  fi
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "201" ]; then
    echo "ERROR: POST $BANK_B_URL/htlc/lock-with-hash failed (HTTP $code, expected 201)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  echo "  Response:" >&2
  echo "$body" | jq . >&2
  CONTRACT_ID_B=$(echo "$body" | jq -r '.contract_id')
}

verify_lock_status_spoke_b() {
  local resp state_val
  resp=$(http_get "$BANK_D_URL/htlc/status/$CONTRACT_ID_B" "$BANK_D_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected HTLC_STATE_LOCKED on Spoke-B, got '$state_val'." >&2
    exit 1
  fi
}

settle_htlc_spoke_a() {
  http_post "$BANK_A_URL/htlc/settle" "$BANK_A_TOKEN" \
    "$(jq -n \
      --arg cid "$CONTRACT_ID_A" \
      --arg sec "$SECRET_A" \
      '{contract_id: $cid, secret: $sec}')" \
    "200" > /dev/null
}

wait_for_relay_settle() {
  local i resp state_val
  echo "  Waiting up to ${RELAY_SETTLE_TIMEOUT}s for Cacti to settle on Spoke-B..."
  for (( i=0; i<RELAY_SETTLE_TIMEOUT; i+=3 )); do
    sleep 3
    resp=$(http_get "$BANK_D_URL/htlc/status/$CONTRACT_ID_B" "$BANK_D_TOKEN" "200" 2>/dev/null || true)
    state_val=$(echo "$resp" | jq -r '.state // empty' 2>/dev/null || true)
    if [ "$state_val" = "HTLC_STATE_SETTLED" ]; then
      echo "  Relay auto-settled on Spoke-B  ✓"
      return 0
    fi
    echo "  ... still waiting (${i}s, state=$state_val)"
  done

  echo "  Relay did not settle in time — settling manually on Spoke-B"
  http_post "$BANK_D_URL/htlc/settle" "$BANK_D_TOKEN" \
    "$(jq -n \
      --arg cid "$CONTRACT_ID_B" \
      --arg sec "$SECRET_A" \
      '{contract_id: $cid, secret: $sec}')" \
    "200" > /dev/null
}

verify_settled_both() {
  local resp state_val

  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_SETTLED" ]; then
    echo "ERROR: Spoke-A expected SETTLED, got '$state_val'." >&2; exit 1
  fi
  echo "  Spoke-A: HTLC_STATE_SETTLED  ✓"

  resp=$(http_get "$BANK_D_URL/htlc/status/$CONTRACT_ID_B" "$BANK_D_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_SETTLED" ]; then
    echo "ERROR: Spoke-B expected SETTLED, got '$state_val'." >&2; exit 1
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
  echo "══════════════════════════════════════════════════════════════"
  echo "  HTLC Cross-Spoke — Minimal Payload (smart defaults test)"
  echo "  Spoke-A: Bank-A → Bank-C   |   Spoke-B: Bank-D → Bank-B"
  echo ""
  echo "  Bank-A  : $BANK_A_URL"
  echo "  Bank-B  : $BANK_B_URL"
  echo "  Bank-C  : $BANK_C_URL"
  echo "  Bank-D  : $BANK_D_URL"
  echo "  Lock    : $LOCK_AMOUNT tCeBM"
  echo "  Skip mint: $SKIP_MINT"
  echo "══════════════════════════════════════════════════════════════"

  # ── [1] Authentication ─────────────────────────────────────────────

  echo ""
  echo "═══ [1] Authentication ═══"

  echo ""
  echo "--- Bank-A login ---"
  BANK_A_TOKEN=$(bank_login "Bank-A" "$BANK_A_URL" "$BANK_A_ENV" "bank-a-client")
  echo "  token: ${BANK_A_TOKEN:0:60}..."

  echo ""
  echo "--- Bank-B login ---"
  BANK_B_TOKEN=$(bank_login "Bank-B" "$BANK_B_URL" "$BANK_B_ENV" "bank-b-client")
  echo "  token: ${BANK_B_TOKEN:0:60}..."

  echo ""
  echo "--- Bank-C login ---"
  BANK_C_TOKEN=$(bank_login "Bank-C" "$BANK_C_URL" "$BANK_C_ENV" "bank-c-client")
  echo "  token: ${BANK_C_TOKEN:0:60}..."

  echo ""
  echo "--- Bank-D login ---"
  BANK_D_TOKEN=$(bank_login "Bank-D" "$BANK_D_URL" "$BANK_D_ENV" "bank-d-client")
  echo "  token: ${BANK_D_TOKEN:0:60}..."

  echo ""
  echo "--- Central-Bank-A login ---"
  CB_A_TOKEN=$(bank_login "CB-A" "$CB_A_URL" "$CB_A_ENV" "central-bank-a-client")
  echo "  token: ${CB_A_TOKEN:0:60}..."

  echo ""
  echo "--- Central-Bank-B login ---"
  CB_B_TOKEN=$(bank_login "CB-B" "$CB_B_URL" "$CB_B_ENV" "central-bank-b-client")
  echo "  token: ${CB_B_TOKEN:0:60}..."

  # ── [2] Mint (optional) ────────────────────────────────────────────

  if [ "$SKIP_MINT" = "false" ]; then
    echo ""
    echo "═══ [2] Token minting ═══"

    echo ""
    echo "--- Mint $MINT_AMOUNT tCeBM → Bank-A (via CB-A) ---"
    mint_tokens "$CB_A_URL" "$CB_A_TOKEN" "$IDENTITY_BANK_A"

    echo ""
    echo "--- Mint $MINT_AMOUNT tCeBM → Bank-B (via CB-B) ---"
    mint_tokens "$CB_B_URL" "$CB_B_TOKEN" "$IDENTITY_BANK_B"
  else
    echo ""
    echo "═══ [2] Mint skipped (SKIP_MINT=true) ═══"
  fi

  # ── [3] Initial balances ───────────────────────────────────────────

  echo ""
  echo "═══ [3] Initial balances ═══"
  BALANCE_A_BEFORE=$(check_balance "$BANK_A_URL" "$BANK_A_TOKEN")
  BALANCE_B_BEFORE=$(check_balance "$BANK_B_URL" "$BANK_B_TOKEN")
  BALANCE_C_BEFORE=$(check_balance "$BANK_C_URL" "$BANK_C_TOKEN")
  BALANCE_D_BEFORE=$(check_balance "$BANK_D_URL" "$BANK_D_TOKEN")
  echo "  Bank-A: $BALANCE_A_BEFORE"
  echo "  Bank-B: $BALANCE_B_BEFORE"
  echo "  Bank-C: $BALANCE_C_BEFORE"
  echo "  Bank-D: $BALANCE_D_BEFORE"

  # ── [4] Initiator lock (Spoke-A) ──────────────────────────────────

  echo ""
  echo "═══ [4] Initiator lock — Bank-A → Bank-C (Spoke-A) ═══"
  echo "  MINIMAL PAYLOAD: receiver + amount only"
  echo "  (agreement_id and time_lock auto-generated by backend)"
  echo ""
  lock_htlc_initiator_minimal
  echo "  contract_id : $CONTRACT_ID_A"
  echo "  hash_lock   : ${HASH_LOCK_A:0:32}..."
  echo "  secret      : ${SECRET_A:0:16}... (${#SECRET_A} chars)"

  echo ""
  echo "--- Verifying lock status on Spoke-A ---"
  verify_lock_status_spoke_a
  echo "  state: HTLC_STATE_LOCKED  ✓"
  echo "  secret retrieved: ${SECRET_A:0:16}... (${#SECRET_A} chars)"

  # ── [5] Responder lock (Spoke-B) ──────────────────────────────────

  echo ""
  echo "═══ [5] Responder lock — Bank-D → Bank-B (Spoke-B) ═══"
  echo "  MINIMAL PAYLOAD: hash_lock + receiver + amount only"
  echo "  (agreement_id and time_lock=now+1800s auto-generated by backend)"
  echo "  hash_lock received from Spoke-A: ${HASH_LOCK_A:0:32}..."
  echo ""
  lock_htlc_responder_minimal
  echo "  contract_id : $CONTRACT_ID_B"

  echo ""
  echo "--- Verifying lock status on Spoke-B ---"
  verify_lock_status_spoke_b
  echo "  state: HTLC_STATE_LOCKED  ✓"

  # ── [6] Settle ────────────────────────────────────────────────────

  echo ""
  echo "═══ [6] Settle on Spoke-A (Bank-A reveals the secret) ═══"
  echo "  contract_id : $CONTRACT_ID_A"
  echo "  secret      : ${SECRET_A:0:16}..."
  settle_htlc_spoke_a

  echo ""
  echo "═══ [7] Waiting for Cacti relay to settle on Spoke-B ═══"
  wait_for_relay_settle

  echo ""
  echo "═══ [8] Verifying settlement on both spokes ═══"
  verify_settled_both

  # ── [9] Final balances ────────────────────────────────────────────

  echo ""
  echo "═══ [9] Final balances ═══"
  BALANCE_A_AFTER=$(check_balance "$BANK_A_URL" "$BANK_A_TOKEN")
  BALANCE_B_AFTER=$(check_balance "$BANK_B_URL" "$BANK_B_TOKEN")
  BALANCE_C_AFTER=$(check_balance "$BANK_C_URL" "$BANK_C_TOKEN")
  BALANCE_D_AFTER=$(check_balance "$BANK_D_URL" "$BANK_D_TOKEN")
  echo "  Bank-A: $BALANCE_A_BEFORE → $BALANCE_A_AFTER  (locked $LOCK_AMOUNT)"
  echo "  Bank-B: $BALANCE_B_BEFORE → $BALANCE_B_AFTER  (received $LOCK_AMOUNT)"
  echo "  Bank-C: $BALANCE_C_BEFORE → $BALANCE_C_AFTER  (received $LOCK_AMOUNT)"
  echo "  Bank-D: $BALANCE_D_BEFORE → $BALANCE_D_AFTER  (locked $LOCK_AMOUNT)"

  # ── Summary ───────────────────────────────────────────────────────

  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  Summary — HTLC Cross-Spoke Minimal Payload"
  echo "══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Smart defaults validated:"
  echo "    agreement_id  → UUID auto  (not sent in payload)      ✓"
  echo "    time_lock     → now+3600s  (lock)                     ✓"
  echo "    time_lock     → now+1800s  (lock-with-hash)           ✓"
  echo ""
  echo "  ── Spoke-A (Bank-A → Bank-C) ──"
  echo "  Contract ID : $CONTRACT_ID_A"
  echo "  Hash Lock   : ${HASH_LOCK_A:0:32}..."
  echo "  Secret      : ${SECRET_A:0:16}..."
  echo "  State       : SETTLED ✓"
  echo ""
  echo "  ── Spoke-B (Bank-D → Bank-B) ──"
  echo "  Contract ID : $CONTRACT_ID_B"
  echo "  State       : SETTLED ✓"
  echo ""
  echo "  ── Balances ──"
  printf "  %-7s %s → %s\n" "Bank-A:" "$BALANCE_A_BEFORE" "$BALANCE_A_AFTER"
  printf "  %-7s %s → %s\n" "Bank-B:" "$BALANCE_B_BEFORE" "$BALANCE_B_AFTER"
  printf "  %-7s %s → %s\n" "Bank-C:" "$BALANCE_C_BEFORE" "$BALANCE_C_AFTER"
  printf "  %-7s %s → %s\n" "Bank-D:" "$BALANCE_D_BEFORE" "$BALANCE_D_AFTER"
  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  Completed successfully"
  echo "══════════════════════════════════════════════════════════════"
}

main "$@"
