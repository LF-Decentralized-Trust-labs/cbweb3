#!/usr/bin/env bash
# tryout-htlc-cross-spoke.sh — Cross-spoke FX atomic swap using 4 banks
#
# Scenario A: Enhanced Correspondent Banking
# Bank-A (Spoke-A) and Bank-B (Spoke-B) exchange tCeBM via dual-layer HTLC.
# All Zeto operations are INTRA-SPOKE; the relay bridges the secret across.
#
# Participants:
#   Spoke-A: Bank-A (sender) → Bank-C (receiver)
#   Spoke-B: Bank-B (sender) → Bank-D (receiver)
#
# Flow (17 steps across 6 phases):
#
#   Phase 1 — Authentication
#     Step  1  Bank-A operator login
#     Step  2  Bank-B operator login
#     Step  3  Bank-C operator login
#     Step  4  Bank-D operator login
#
#   Phase 2 — Token Minting & Balances
#     Step  5  Mint tCeBM for Bank-A (Spoke-A)
#     Step  6  Mint tCeBM for Bank-B (Spoke-B)
#     Step  7  Check initial balances (A, B, C, D)
#
#   Phase 3 — Initiator Lock (Spoke-A)
#     Step  8  Bank-A locks for Bank-C (generates secret + hashLock)
#     Step  9  Verify lock status & retrieve secret
#
#   Phase 4 — Responder Lock (Spoke-B)
#     Step 10  Bank-B locks for Bank-D (LockHTLCWithHashLock, same hashLock)
#     Step 11  Verify lock status on Spoke-B
#
#   Phase 5 — Settle (secret reveal + relay)
#     Step 12  Bank-A settles on Spoke-A (reveals secret on-chain)
#     Step 13  Wait for relay to auto-settle on Spoke-B (or settle manually)
#     Step 14  Verify settled status on both spokes
#
#   Phase 6 — Verification
#     Step 15  Check final balances (A, B, C, D)
#     Step 16  Search settled HTLCs
#     Step 17  Final summary
#
# Prerequisites:
#   - All 4 banks onboarded:
#     ./tryout-spoke-a-bank-a.sh, ./tryout-spoke-a-bank-c.sh
#     ./tryout-spoke-b-bank-b.sh, ./tryout-spoke-b-bank-d.sh
#   - Paladin nodes running on both spokes
#   - Backend stacks running for all 4 banks
#   - Relay service running (interop/hub-and-spoke/relay)
#   - curl, jq
#
# Usage:
#   ./tryout-htlc-cross-spoke.sh

set -euo pipefail

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

# Paladin identities (per-bank, intra-spoke)
IDENTITY_BANK_A="funded_operator@spoke-a-bank-a"
IDENTITY_BANK_B="funded_operator@spoke-b-bank-b"
IDENTITY_BANK_C="funded_operator@spoke-a-bank-c"
IDENTITY_BANK_D="funded_operator@spoke-b-bank-d"

# Timeout waiting for relay to auto-settle on Spoke-B
RELAY_SETTLE_TIMEOUT="${RELAY_SETTLE_TIMEOUT:-30}"

# Globals populated during execution
BANK_A_TOKEN=""
BANK_B_TOKEN=""
BANK_C_TOKEN=""
BANK_D_TOKEN=""
CB_A_TOKEN=""
CB_B_TOKEN=""

MINT_TX_A=""
MINT_TX_B=""
BALANCE_A_BEFORE=""
BALANCE_B_BEFORE=""
BALANCE_C_BEFORE=""
BALANCE_D_BEFORE=""

# Spoke-A HTLC (Bank-A → Bank-C)
CONTRACT_ID_A=""
HASH_LOCK_A=""
SECRET_A=""
ZETO_TX_LOCK_A=""
ZETO_TX_SETTLE_A=""

# Spoke-B HTLC (Bank-B → Bank-D)
CONTRACT_ID_B=""
ZETO_TX_LOCK_B=""
ZETO_TX_SETTLE_B=""

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
  for f in "$BANK_A_ENV" "$BANK_B_ENV" "$BANK_C_ENV" "$BANK_D_ENV"; do
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

# http_post URL TOKEN PAYLOAD EXPECTED_CODE
# Writes response body to stdout; exits on unexpected status code.
http_post() {
  local url=$1 token=$2 payload=$3 expected=$4
  local tmp code body
  tmp=$(mktemp)
  code=$(curl -sS -X POST "$url" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    --data "$payload" \
    -o "$tmp" -w "%{http_code}")
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "$expected" ]; then
    echo "ERROR: POST $url failed (HTTP $code, expected $expected)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  echo "$body"
}

# http_get URL TOKEN EXPECTED_CODE
http_get() {
  local url=$1 token=$2 expected=$3
  local tmp code body
  tmp=$(mktemp)
  code=$(curl -sS -X GET "$url" \
    --cookie "access_token=$token" \
    -H "Content-Type: application/json" \
    -o "$tmp" -w "%{http_code}")
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "$expected" ]; then
    echo "ERROR: GET $url failed (HTTP $code, expected $expected)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  echo "$body"
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 1 — Authentication (all 4 banks)
# ═══════════════════════════════════════════════════════════════════════════

bank_login() {
  local bank_name=$1 bank_url=$2 env_file=$3 client_id=$4
  local kc_secret login_resp token
  kc_secret=$(read_kc_secret "$env_file")
  login_resp=$(curl -s -X POST "$bank_url/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"$client_id\", \"clientSecret\": \"$kc_secret\"}")
  token=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$token" ]; then
    echo "ERROR: $bank_name operator login failed." >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
  echo "$token"
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 2 — Token Minting
# ═══════════════════════════════════════════════════════════════════════════

mint_tokens() {
  local bank_url=$1 token=$2 identity=$3
  local resp
  resp=$(http_post "$bank_url/token/mint" "$token" \
    "$(jq -n --arg to "$identity" --arg amt "$MINT_AMOUNT" \
      '{to: $to, amount: $amt}')" \
    "201")
  echo "$resp" | jq -r '.tx_hash // empty'
}

check_balance() {
  local bank_url=$1 token=$2 identity=$3
  local resp
  resp=$(http_get "$bank_url/token/balance?identity=$identity" "$token" "200")
  echo "$resp" | jq -r '.balance // "0"'
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 3 — Initiator Lock (Spoke-A: Bank-A → Bank-C)
# ═══════════════════════════════════════════════════════════════════════════

lock_htlc_initiator() {
  local time_lock resp
  time_lock=$(( $(date +%s) + 3600 ))
  resp=$(http_post "$BANK_A_URL/htlc/lock" "$BANK_A_TOKEN" \
    "$(jq -n \
      --arg aid "FX_CROSS_SPOKE_001" \
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

verify_lock_status_spoke_a() {
  local resp state_val
  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  SECRET_A=$(echo "$resp" | jq -r '.secret // empty')
  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected HTLC_STATE_LOCKED, got '$state_val'." >&2
    exit 1
  fi
  if [ -z "$SECRET_A" ]; then
    echo "ERROR: Secret not found in HTLC status." >&2
    exit 1
  fi
  echo "$resp" | jq .
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 4 — Responder Lock (Spoke-B: Bank-B → Bank-D, same hashLock)
# ═══════════════════════════════════════════════════════════════════════════

lock_htlc_responder() {
  local time_lock resp
  # Responder timeLock MUST be shorter than initiator's
  time_lock=$(( $(date +%s) + 1800 ))
  resp=$(http_post "$BANK_B_URL/htlc/lock-with-hash" "$BANK_B_TOKEN" \
    "$(jq -n \
      --arg aid "FX_CROSS_SPOKE_001" \
      --arg rcv "$IDENTITY_BANK_D" \
      --arg amt "$LOCK_AMOUNT" \
      --argjson tl "$time_lock" \
      --arg hl "$HASH_LOCK_A" \
      '{agreement_id: $aid, receiver: $rcv, amount: $amt, time_lock: $tl, hash_lock: $hl}')" \
    "201")
  CONTRACT_ID_B=$(echo "$resp" | jq -r '.contract_id')
  ZETO_TX_LOCK_B=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

verify_lock_status_spoke_b() {
  local resp state_val
  resp=$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" "$BANK_B_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected HTLC_STATE_LOCKED on Spoke-B, got '$state_val'." >&2
    exit 1
  fi
  echo "$resp" | jq .
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 5 — Settle (secret reveal + relay auto-settle)
# ═══════════════════════════════════════════════════════════════════════════

settle_htlc_spoke_a() {
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

wait_for_relay_settle() {
  # Wait for the relay to auto-settle on Spoke-B by polling the status.
  # Falls back to manual settle if the relay doesn't pick it up in time.
  local i resp state_val
  echo "  Waiting up to ${RELAY_SETTLE_TIMEOUT}s for relay to settle on Spoke-B..."
  for (( i=0; i<RELAY_SETTLE_TIMEOUT; i+=3 )); do
    sleep 3
    resp=$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" "$BANK_B_TOKEN" "200" 2>/dev/null || true)
    state_val=$(echo "$resp" | jq -r '.state // empty' 2>/dev/null || true)
    if [ "$state_val" = "HTLC_STATE_SETTLED" ]; then
      echo "  Relay auto-settled on Spoke-B  ✓"
      ZETO_TX_SETTLE_B=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
      return 0
    fi
    echo "  ... still waiting (${i}s, state=$state_val)"
  done

  echo "  Relay did not settle in time — settling manually on Spoke-B"
  resp=$(http_post "$BANK_B_URL/htlc/settle" "$BANK_B_TOKEN" \
    "$(jq -n \
      --arg cid "$CONTRACT_ID_B" \
      --arg sec "$SECRET_A" \
      '{contract_id: $cid, secret: $sec}')" \
    "200")
  ZETO_TX_SETTLE_B=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
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
  resp=$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" "$BANK_B_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.state // empty')
  if [ "$state_val" != "HTLC_STATE_SETTLED" ]; then
    echo "ERROR: Spoke-B HTLC expected SETTLED, got '$state_val'." >&2
    exit 1
  fi
  echo "  Spoke-B: HTLC_STATE_SETTLED  ✓"
}

# ═══════════════════════════════════════════════════════════════════════════
# Main
# ═══════════════════════════════════════════════════════════════════════════

main() {
  require_cmds
  require_env_files

  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  HTLC Cross-Spoke FX Atomic Swap — 4-Bank Flow"
  echo "  Spoke-A: Bank-A → Bank-C   |   Spoke-B: Bank-B → Bank-D"
  echo ""
  echo "  Bank-A Gateway : $BANK_A_URL"
  echo "  Bank-B Gateway : $BANK_B_URL"
  echo "  Bank-C Gateway : $BANK_C_URL"
  echo "  Bank-D Gateway : $BANK_D_URL"
  echo "  Mint amount    : $MINT_AMOUNT tCeBM"
  echo "  Lock amount    : $LOCK_AMOUNT tCeBM"
  echo "══════════════════════════════════════════════════════════════"

  # ─── Phase 1: Authentication ──────────────────────────────────────────

  echo ""
  echo "═══ Phase 1 — Authentication (4 banks) ═══"

  echo ""
  echo "=== [1/17] Bank-A operator login ==="
  BANK_A_TOKEN=$(bank_login "Bank-A" "$BANK_A_URL" "$BANK_A_ENV" "bank-a-client")
  echo "  BANK_A_TOKEN: ${BANK_A_TOKEN:0:60}..."

  echo ""
  echo "=== [2/17] Bank-B operator login ==="
  BANK_B_TOKEN=$(bank_login "Bank-B" "$BANK_B_URL" "$BANK_B_ENV" "bank-b-client")
  echo "  BANK_B_TOKEN: ${BANK_B_TOKEN:0:60}..."

  echo ""
  echo "=== [3/17] Bank-C operator login ==="
  BANK_C_TOKEN=$(bank_login "Bank-C" "$BANK_C_URL" "$BANK_C_ENV" "bank-c-client")
  echo "  BANK_C_TOKEN: ${BANK_C_TOKEN:0:60}..."

  echo ""
  echo "=== [4/17] Bank-D operator login ==="
  BANK_D_TOKEN=$(bank_login "Bank-D" "$BANK_D_URL" "$BANK_D_ENV" "bank-d-client")
  echo "  BANK_D_TOKEN: ${BANK_D_TOKEN:0:60}..."

  echo ""
  echo "=== [4b/17] Central-Bank-A operator login ==="
  CB_A_TOKEN=$(bank_login "CB-A" "$CB_A_URL" "$CB_A_ENV" "central-bank-a-client")
  echo "  CB_A_TOKEN: ${CB_A_TOKEN:0:60}..."

  echo ""
  echo "=== [4c/17] Central-Bank-B operator login ==="
  CB_B_TOKEN=$(bank_login "CB-B" "$CB_B_URL" "$CB_B_ENV" "central-bank-b-client")
  echo "  CB_B_TOKEN: ${CB_B_TOKEN:0:60}..."

  # ─── Phase 2: Token Minting & Balances ───────────────────────────────

  echo ""
  echo "═══ Phase 2 — Token Minting & Initial Balances ═══"

  echo ""
  echo "=== [5/17] Mint $MINT_AMOUNT tCeBM for Bank-A (via CB-A) ==="
  echo "  to: $IDENTITY_BANK_A"
  MINT_TX_A=$(mint_tokens "$CB_A_URL" "$CB_A_TOKEN" "$IDENTITY_BANK_A")
  echo "  mint_tx_hash: ${MINT_TX_A:-"(noop)"}"

  echo ""
  echo "=== [6/17] Mint $MINT_AMOUNT tCeBM for Bank-B (via CB-B) ==="
  echo "  to: $IDENTITY_BANK_B"
  MINT_TX_B=$(mint_tokens "$CB_B_URL" "$CB_B_TOKEN" "$IDENTITY_BANK_B")
  echo "  mint_tx_hash: ${MINT_TX_B:-"(noop)"}"

  echo ""
  echo "=== [7/17] Check initial balances ==="
  BALANCE_A_BEFORE=$(check_balance "$BANK_A_URL" "$BANK_A_TOKEN" "$IDENTITY_BANK_A")
  BALANCE_B_BEFORE=$(check_balance "$BANK_B_URL" "$BANK_B_TOKEN" "$IDENTITY_BANK_B")
  BALANCE_C_BEFORE=$(check_balance "$BANK_C_URL" "$BANK_C_TOKEN" "$IDENTITY_BANK_C")
  BALANCE_D_BEFORE=$(check_balance "$BANK_D_URL" "$BANK_D_TOKEN" "$IDENTITY_BANK_D")
  echo "  Bank-A: $BALANCE_A_BEFORE"
  echo "  Bank-B: $BALANCE_B_BEFORE"
  echo "  Bank-C: $BALANCE_C_BEFORE"
  echo "  Bank-D: $BALANCE_D_BEFORE"

  # ─── Phase 3: Initiator Lock (Spoke-A) ──────────────────────────────

  echo ""
  echo "═══ Phase 3 — Initiator Lock (Bank-A → Bank-C, Spoke-A) ═══"

  echo ""
  echo "=== [8/17] Bank-A locks $LOCK_AMOUNT tCeBM for Bank-C ==="
  echo "  agreement: FX_CROSS_SPOKE_001"
  echo "  receiver:  $IDENTITY_BANK_C (intra-spoke)"
  lock_htlc_initiator
  echo "  contract_id:   $CONTRACT_ID_A"
  echo "  hash_lock:     ${HASH_LOCK_A:0:32}..."
  echo "  zeto_tx_hash:  ${ZETO_TX_LOCK_A:-"(pending)"}"

  echo ""
  echo "=== [9/17] Verify HTLC lock & retrieve secret (Spoke-A) ==="
  verify_lock_status_spoke_a
  echo "  state:  HTLC_STATE_LOCKED  ✓"
  echo "  secret: ${SECRET_A:0:16}... (${#SECRET_A} hex chars)"

  # ─── Phase 4: Responder Lock (Spoke-B) ──────────────────────────────

  echo ""
  echo "═══ Phase 4 — Responder Lock (Bank-B → Bank-D, Spoke-B) ═══"
  echo "  Using hashLock from Spoke-A: ${HASH_LOCK_A:0:32}..."

  echo ""
  echo "=== [10/17] Bank-B locks $LOCK_AMOUNT tCeBM for Bank-D (same hashLock) ==="
  echo "  agreement: FX_CROSS_SPOKE_001"
  echo "  receiver:  $IDENTITY_BANK_D (intra-spoke)"
  lock_htlc_responder
  echo "  contract_id:   $CONTRACT_ID_B"
  echo "  zeto_tx_hash:  ${ZETO_TX_LOCK_B:-"(pending)"}"

  echo ""
  echo "=== [11/17] Verify HTLC lock (Spoke-B) ==="
  verify_lock_status_spoke_b
  echo "  state: HTLC_STATE_LOCKED  ✓"

  # ─── Phase 5: Settle ────────────────────────────────────────────────

  echo ""
  echo "═══ Phase 5 — Settle (secret reveal + relay) ═══"

  echo ""
  echo "=== [12/17] Bank-A settles on Spoke-A (reveals secret) ==="
  echo "  contract_id: $CONTRACT_ID_A"
  echo "  secret:      ${SECRET_A:0:16}..."
  settle_htlc_spoke_a
  echo "  zeto_tx_hash: ${ZETO_TX_SETTLE_A:-"(pending)"}"

  echo ""
  echo "=== [13/17] Wait for relay to auto-settle on Spoke-B ==="
  wait_for_relay_settle
  echo "  zeto_tx_hash: ${ZETO_TX_SETTLE_B:-"(pending)"}"

  echo ""
  echo "=== [14/17] Verify settled status on both spokes ==="
  verify_settled_both

  # ─── Phase 6: Verification ──────────────────────────────────────────

  echo ""
  echo "═══ Phase 6 — Verification ═══"

  echo ""
  echo "=== [15/17] Check final balances ==="
  BALANCE_A_AFTER=$(check_balance "$BANK_A_URL" "$BANK_A_TOKEN" "$IDENTITY_BANK_A")
  BALANCE_B_AFTER=$(check_balance "$BANK_B_URL" "$BANK_B_TOKEN" "$IDENTITY_BANK_B")
  BALANCE_C_AFTER=$(check_balance "$BANK_C_URL" "$BANK_C_TOKEN" "$IDENTITY_BANK_C")
  BALANCE_D_AFTER=$(check_balance "$BANK_D_URL" "$BANK_D_TOKEN" "$IDENTITY_BANK_D")
  echo "  Bank-A: $BALANCE_A_BEFORE → $BALANCE_A_AFTER (locked $LOCK_AMOUNT)"
  echo "  Bank-B: $BALANCE_B_BEFORE → $BALANCE_B_AFTER (locked $LOCK_AMOUNT)"
  echo "  Bank-C: $BALANCE_C_BEFORE → $BALANCE_C_AFTER (received $LOCK_AMOUNT)"
  echo "  Bank-D: $BALANCE_D_BEFORE → $BALANCE_D_AFTER (received $LOCK_AMOUNT)"

  echo ""
  echo "=== [16/17] Search settled HTLCs ==="
  echo "  Spoke-A:"
  http_get "$BANK_A_URL/htlc/search?state=SETTLED" "$BANK_A_TOKEN" "200" | jq .
  echo "  Spoke-B:"
  http_get "$BANK_B_URL/htlc/search?state=SETTLED" "$BANK_B_TOKEN" "200" | jq .

  # ─── Summary ────────────────────────────────────────────────────────

  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  [17/17] HTLC Cross-Spoke FX Atomic Swap — Summary"
  echo "══════════════════════════════════════════════════════════════"
  echo ""
  echo "  ── Spoke-A HTLC (Bank-A → Bank-C) ──"
  echo "  Contract ID:    $CONTRACT_ID_A"
  echo "  Hash Lock:      ${HASH_LOCK_A:0:32}..."
  echo "  Secret:         ${SECRET_A:0:16}..."
  echo "  Lock Zeto TX:   ${ZETO_TX_LOCK_A:-"(n/a)"}"
  echo "  Settle Zeto TX: ${ZETO_TX_SETTLE_A:-"(n/a)"}"
  echo "  State:          SETTLED ✓"
  echo ""
  echo "  ── Spoke-B HTLC (Bank-B → Bank-D) ──"
  echo "  Contract ID:    $CONTRACT_ID_B"
  echo "  Lock Zeto TX:   ${ZETO_TX_LOCK_B:-"(n/a)"}"
  echo "  Settle Zeto TX: ${ZETO_TX_SETTLE_B:-"(n/a)"}"
  echo "  State:          SETTLED ✓"
  echo ""
  echo "  ── Balances ──"
  echo "  Bank-A: $BALANCE_A_BEFORE → $BALANCE_A_AFTER"
  echo "  Bank-B: $BALANCE_B_BEFORE → $BALANCE_B_AFTER"
  echo "  Bank-C: $BALANCE_C_BEFORE → $BALANCE_C_AFTER"
  echo "  Bank-D: $BALANCE_D_BEFORE → $BALANCE_D_AFTER"
  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  HTLC Cross-Spoke FX Atomic Swap completed successfully"
  echo "══════════════════════════════════════════════════════════════"
  echo ""
}

main "$@"
