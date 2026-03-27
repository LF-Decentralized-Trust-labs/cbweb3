#!/usr/bin/env bash
# tryout-htlc-cross-spoke.sh — Simulate a cross-spoke FX currency exchange
#                               using HTLC dual-layer (Besu + Zeto/Paladin)
#
# Scenario A: Enhanced Correspondent Banking
# Bank-A (Spoke-A) exchanges tCeBM with Bank-B (Spoke-B) via HTLC.
# The script demonstrates the full lifecycle:
#   - Token minting (tCeBM via Zeto/Paladin)
#   - HTLC Lock  → Settle (happy path)
#   - HTLC Lock  → Refund (timeout path)
#   - Balance verification and HTLC search
#
# Flow (19 steps across 6 phases):
#
#   Phase 1 — Authentication
#     Step  1  Bank-A operator login
#     Step  2  Bank-B operator login
#
#   Phase 2 — Token Minting
#     Step  3  Mint tCeBM for Bank-A (via Bank-A gateway)
#     Step  4  Mint tCeBM for Bank-B (via Bank-B gateway)
#     Step  5  Check Bank-A balance
#     Step  6  Check Bank-B balance
#
#   Phase 3 — HTLC Lock (Bank-A initiates)
#     Step  7  Bank-A locks 1,000,000 tCeBM for Bank-B
#     Step  8  Verify lock status + retrieve secret
#
#   Phase 4 — HTLC Settle (Happy Path)
#     Step  9  Bank-A settles using revealed secret
#     Step 10  Verify settled status
#     Step 11  Check Bank-A balance after settle
#
#   Phase 5 — HTLC Refund (Timeout Path)
#     Step 12  Bank-B locks tokens with expired timelock
#     Step 13  Bank-B requests refund
#     Step 14  Verify refunded status
#     Step 15  Check Bank-B balance after refund
#
#   Phase 6 — Search & Report
#     Step 16  Search settled HTLCs on Bank-A
#     Step 17  Search refunded HTLCs on Bank-B
#     Step 18  Search by agreement_id on Bank-A
#     Step 19  Final summary
#
# Prerequisites:
#   - Both banks onboarded (run tryout-spoke-a-bank-a.sh + tryout-spoke-b-bank-b.sh first)
#   - Paladin nodes running on both spokes (make setup-spoke-a, make setup-spoke-b)
#   - Backend stacks running (docker compose up for bank-a and bank-b)
#   - curl, jq
#
# Usage:
#   ./tryout-htlc-cross-spoke.sh
#
# Environment variables (optional):
#   BANK_A_URL   Bank-A API base   (default: http://localhost:18080/api/v1)
#   BANK_B_URL   Bank-B API base   (default: http://localhost:28080/api/v1)
#   BANK_A_ENV   Bank-A .env file  (default: backend/config/.env.infra.bank-a)
#   BANK_B_ENV   Bank-B .env file  (default: backend/config/.env.infra.bank-b)
#   MINT_AMOUNT  Initial mint      (default: 10000000)
#   LOCK_AMOUNT  HTLC lock amount  (default: 1000000)
#
# TODO: The current LockHTLC API generates the secret internally. For a full
#       cross-spoke atomic swap with mirrored locks, a new RPC
#       LockHTLCWithHashLock (accepting an external hashLock) is needed.
#       This script demonstrates both paths (settle + refund) on each spoke.

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BANK_A_URL="${BANK_A_URL:-http://localhost:18080/api/v1}"
BANK_B_URL="${BANK_B_URL:-http://localhost:28080/api/v1}"
BANK_A_ENV="${BANK_A_ENV:-backend/config/.env.infra.bank-a}"
BANK_B_ENV="${BANK_B_ENV:-backend/config/.env.infra.bank-b}"
MINT_AMOUNT="${MINT_AMOUNT:-10000000}"
LOCK_AMOUNT="${LOCK_AMOUNT:-1000000}"

# Paladin identities for each spoke
PALADIN_IDENTITY_A="funded_operator@spoke-a-cb"
PALADIN_IDENTITY_B="funded_operator@spoke-b-cb"

# Globals populated during execution
BANK_A_TOKEN=""
BANK_B_TOKEN=""

# Phase 2: Mint
MINT_TX_A=""
MINT_TX_B=""
BALANCE_A_BEFORE=""
BALANCE_B_BEFORE=""

# Phase 3–4: HTLC Settle (Bank-A)
CONTRACT_ID_A=""
HASH_LOCK_A=""
SECRET_A=""
ZETO_TX_LOCK_A=""
ZETO_TX_SETTLE_A=""
BALANCE_A_AFTER=""

# Phase 5: HTLC Refund (Bank-B)
CONTRACT_ID_B=""
HASH_LOCK_B=""
ZETO_TX_LOCK_B=""
ZETO_TX_REFUND_B=""
BALANCE_B_AFTER=""

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
  if [ ! -f "$BANK_A_ENV" ]; then
    echo "ERROR: Bank-A env file '$BANK_A_ENV' not found." >&2
    exit 1
  fi
  if [ ! -f "$BANK_B_ENV" ]; then
    echo "ERROR: Bank-B env file '$BANK_B_ENV' not found." >&2
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
# Phase 1 — Authentication
# ═══════════════════════════════════════════════════════════════════════════

# ---------------------------------------------------------------------------
# Step 1 — Bank-A operator login
# Authenticates against Bank-A's Keycloak to get an access token.
# ---------------------------------------------------------------------------

bank_a_login() {
  local kc_secret login_resp
  kc_secret=$(read_kc_secret "$BANK_A_ENV")
  login_resp=$(curl -s -X POST "$BANK_A_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"bank-a-client\", \"clientSecret\": \"$kc_secret\"}")
  BANK_A_TOKEN=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$BANK_A_TOKEN" ]; then
    echo "ERROR: Bank-A operator login failed." >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Step 2 — Bank-B operator login
# Authenticates against Bank-B's Keycloak to get an access token.
# ---------------------------------------------------------------------------

bank_b_login() {
  local kc_secret login_resp
  kc_secret=$(read_kc_secret "$BANK_B_ENV")
  login_resp=$(curl -s -X POST "$BANK_B_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"bank-b-client\", \"clientSecret\": \"$kc_secret\"}")
  BANK_B_TOKEN=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$BANK_B_TOKEN" ]; then
    echo "ERROR: Bank-B operator login failed." >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 2 — Token Minting
# ═══════════════════════════════════════════════════════════════════════════

# ---------------------------------------------------------------------------
# Step 3 — Mint tCeBM for Bank-A
# Issues tokens to Bank-A's Paladin identity via Zeto.
# ---------------------------------------------------------------------------

mint_tokens_bank_a() {
  local resp
  resp=$(http_post "$BANK_A_URL/token/mint" "$BANK_A_TOKEN" \
    "$(jq -n --arg to "$PALADIN_IDENTITY_A" --arg amt "$MINT_AMOUNT" \
      '{to: $to, amount: $amt}')" \
    "201")
  MINT_TX_A=$(echo "$resp" | jq -r '.tx_hash // empty')
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 4 — Mint tCeBM for Bank-B
# Issues tokens to Bank-B's Paladin identity via Zeto.
# ---------------------------------------------------------------------------

mint_tokens_bank_b() {
  local resp
  resp=$(http_post "$BANK_B_URL/token/mint" "$BANK_B_TOKEN" \
    "$(jq -n --arg to "$PALADIN_IDENTITY_B" --arg amt "$MINT_AMOUNT" \
      '{to: $to, amount: $amt}')" \
    "201")
  MINT_TX_B=$(echo "$resp" | jq -r '.tx_hash // empty')
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 5 — Check Bank-A balance
# ---------------------------------------------------------------------------

check_balance_bank_a() {
  local resp
  resp=$(http_get "$BANK_A_URL/token/balance?identity=$PALADIN_IDENTITY_A" \
    "$BANK_A_TOKEN" "200")
  BALANCE_A_BEFORE=$(echo "$resp" | jq -r '.balance // "0"')
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 6 — Check Bank-B balance
# ---------------------------------------------------------------------------

check_balance_bank_b() {
  local resp
  resp=$(http_get "$BANK_B_URL/token/balance?identity=$PALADIN_IDENTITY_B" \
    "$BANK_B_TOKEN" "200")
  BALANCE_B_BEFORE=$(echo "$resp" | jq -r '.balance // "0"')
  echo "$resp" | jq .
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 3 — HTLC Lock (Bank-A initiates a lock for Bank-B)
# ═══════════════════════════════════════════════════════════════════════════

# ---------------------------------------------------------------------------
# Step 7 — Bank-A locks tokens via HTLC
# Creates a dual-layer HTLC: secret generated internally, tokens locked
# privately via Zeto, coordination registered on-chain.
# ---------------------------------------------------------------------------

lock_htlc_bank_a() {
  local time_lock resp
  # Timelock 1 hour from now
  time_lock=$(( $(date +%s) + 3600 ))
  resp=$(http_post "$BANK_A_URL/htlc/lock" "$BANK_A_TOKEN" \
    "$(jq -n \
      --arg aid "FX_CROSS_SPOKE_001" \
      --arg rcv "$PALADIN_IDENTITY_B" \
      --arg amt "$LOCK_AMOUNT" \
      --argjson tl "$time_lock" \
      '{agreement_id: $aid, receiver: $rcv, amount: $amt, time_lock: $tl}')" \
    "201")
  CONTRACT_ID_A=$(echo "$resp" | jq -r '.contract_id')
  HASH_LOCK_A=$(echo "$resp" | jq -r '.hash_lock')
  ZETO_TX_LOCK_A=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 8 — Verify HTLC status & retrieve secret
# The secret is stored off-chain and returned via GetHTLCStatus. In
# production, the counterparty would learn the secret only after settle
# reveals it on-chain. Here we retrieve it to demonstrate the settle flow.
# ---------------------------------------------------------------------------

verify_lock_status_bank_a() {
  local resp state_val
  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" \
    "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.lock.state // empty')
  SECRET_A=$(echo "$resp" | jq -r '.lock.secret // empty')
  if [ "$state_val" != "HTLC_STATE_LOCKED" ]; then
    echo "ERROR: Expected state HTLC_STATE_LOCKED, got '$state_val'." >&2
    echo "Response: $resp" >&2
    exit 1
  fi
  if [ -z "$SECRET_A" ]; then
    echo "ERROR: Secret not found in HTLC status response." >&2
    echo "Response: $resp" >&2
    exit 1
  fi
  echo "$resp" | jq .
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 4 — HTLC Settle (Happy Path)
# ═══════════════════════════════════════════════════════════════════════════

# ---------------------------------------------------------------------------
# Step 9 — Settle HTLC with secret
# Validates SHA256(secret) == hashLock, then calls Zeto.transferLocked()
# to release tokens to the receiver.
# ---------------------------------------------------------------------------

settle_htlc_bank_a() {
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

# ---------------------------------------------------------------------------
# Step 10 — Verify settled status
# ---------------------------------------------------------------------------

verify_settled_status_bank_a() {
  local resp state_val
  resp=$(http_get "$BANK_A_URL/htlc/status/$CONTRACT_ID_A" \
    "$BANK_A_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.lock.state // empty')
  if [ "$state_val" != "HTLC_STATE_SETTLED" ]; then
    echo "ERROR: Expected state HTLC_STATE_SETTLED, got '$state_val'." >&2
    echo "Response: $resp" >&2
    exit 1
  fi
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 11 — Check Bank-A balance after settle
# ---------------------------------------------------------------------------

check_balance_bank_a_after() {
  local resp
  resp=$(http_get "$BANK_A_URL/token/balance?identity=$PALADIN_IDENTITY_A" \
    "$BANK_A_TOKEN" "200")
  BALANCE_A_AFTER=$(echo "$resp" | jq -r '.balance // "0"')
  echo "$resp" | jq .
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 5 — HTLC Refund (Timeout Path on Bank-B)
# ═══════════════════════════════════════════════════════════════════════════

# ---------------------------------------------------------------------------
# Step 12 — Bank-B locks tokens with expired timelock
# Creates an HTLC with a timelock in the past to demonstrate the refund path.
# ---------------------------------------------------------------------------

lock_htlc_bank_b_expired() {
  local time_lock resp
  # Timelock 60 seconds in the past (already expired)
  time_lock=$(( $(date +%s) - 60 ))
  resp=$(http_post "$BANK_B_URL/htlc/lock" "$BANK_B_TOKEN" \
    "$(jq -n \
      --arg aid "FX_CROSS_SPOKE_002" \
      --arg rcv "$PALADIN_IDENTITY_A" \
      --arg amt "$LOCK_AMOUNT" \
      --argjson tl "$time_lock" \
      '{agreement_id: $aid, receiver: $rcv, amount: $amt, time_lock: $tl}')" \
    "201")
  CONTRACT_ID_B=$(echo "$resp" | jq -r '.contract_id')
  HASH_LOCK_B=$(echo "$resp" | jq -r '.hash_lock')
  ZETO_TX_LOCK_B=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 13 — Bank-B requests refund
# Since the timelock has expired, the sender can reclaim locked tokens
# via Zeto.unlock().
# ---------------------------------------------------------------------------

refund_htlc_bank_b() {
  local resp
  resp=$(http_post "$BANK_B_URL/htlc/refund" "$BANK_B_TOKEN" \
    "$(jq -n --arg cid "$CONTRACT_ID_B" '{contract_id: $cid}')" \
    "200")
  ZETO_TX_REFUND_B=$(echo "$resp" | jq -r '.zeto_tx_hash // empty')
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 14 — Verify refunded status
# ---------------------------------------------------------------------------

verify_refunded_status_bank_b() {
  local resp state_val
  resp=$(http_get "$BANK_B_URL/htlc/status/$CONTRACT_ID_B" \
    "$BANK_B_TOKEN" "200")
  state_val=$(echo "$resp" | jq -r '.lock.state // empty')
  if [ "$state_val" != "HTLC_STATE_REFUNDED" ]; then
    echo "ERROR: Expected state HTLC_STATE_REFUNDED, got '$state_val'." >&2
    echo "Response: $resp" >&2
    exit 1
  fi
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 15 — Check Bank-B balance after refund
# ---------------------------------------------------------------------------

check_balance_bank_b_after() {
  local resp
  resp=$(http_get "$BANK_B_URL/token/balance?identity=$PALADIN_IDENTITY_B" \
    "$BANK_B_TOKEN" "200")
  BALANCE_B_AFTER=$(echo "$resp" | jq -r '.balance // "0"')
  echo "$resp" | jq .
}

# ═══════════════════════════════════════════════════════════════════════════
# Phase 6 — Search & Report
# ═══════════════════════════════════════════════════════════════════════════

# ---------------------------------------------------------------------------
# Step 16 — Search settled HTLCs on Bank-A
# ---------------------------------------------------------------------------

search_settled_bank_a() {
  local resp
  resp=$(http_get "$BANK_A_URL/htlc/search?state=SETTLED" \
    "$BANK_A_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 17 — Search refunded HTLCs on Bank-B
# ---------------------------------------------------------------------------

search_refunded_bank_b() {
  local resp
  resp=$(http_get "$BANK_B_URL/htlc/search?state=REFUNDED" \
    "$BANK_B_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 18 — Search by agreement_id on Bank-A
# ---------------------------------------------------------------------------

search_by_agreement_bank_a() {
  local resp
  resp=$(http_get "$BANK_A_URL/htlc/search?agreement_id=FX_CROSS_SPOKE_001" \
    "$BANK_A_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  require_cmds
  require_env_files

  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  HTLC Cross-Spoke FX Exchange — Scenario A"
  echo "  Enhanced Correspondent Banking (dual-layer: Besu + Zeto)"
  echo ""
  echo "  Bank-A Gateway : $BANK_A_URL"
  echo "  Bank-B Gateway : $BANK_B_URL"
  echo "  Mint amount    : $MINT_AMOUNT tCeBM"
  echo "  Lock amount    : $LOCK_AMOUNT tCeBM"
  echo "══════════════════════════════════════════════════════════════"

  # ─── Phase 1: Authentication ──────────────────────────────────────────

  echo ""
  echo "═══ Phase 1 — Authentication ═══"

  echo ""
  echo "=== [1/19] Bank-A operator login ==="
  bank_a_login
  echo "  BANK_A_TOKEN: ${BANK_A_TOKEN:0:60}..."

  echo ""
  echo "=== [2/19] Bank-B operator login ==="
  bank_b_login
  echo "  BANK_B_TOKEN: ${BANK_B_TOKEN:0:60}..."

  # ─── Phase 2: Token Minting ──────────────────────────────────────────

  echo ""
  echo "═══ Phase 2 — Token Minting (tCeBM via Zeto/Paladin) ═══"

  echo ""
  echo "=== [3/19] Mint $MINT_AMOUNT tCeBM for Bank-A ==="
  echo "  to: $PALADIN_IDENTITY_A"
  mint_tokens_bank_a
  echo "  mint_tx_hash: ${MINT_TX_A:-"(noop)"}"

  echo ""
  echo "=== [4/19] Mint $MINT_AMOUNT tCeBM for Bank-B ==="
  echo "  to: $PALADIN_IDENTITY_B"
  mint_tokens_bank_b
  echo "  mint_tx_hash: ${MINT_TX_B:-"(noop)"}"

  echo ""
  echo "=== [5/19] Check Bank-A balance ==="
  check_balance_bank_a
  echo "  balance: $BALANCE_A_BEFORE"

  echo ""
  echo "=== [6/19] Check Bank-B balance ==="
  check_balance_bank_b
  echo "  balance: $BALANCE_B_BEFORE"

  # ─── Phase 3: HTLC Lock ─────────────────────────────────────────────

  echo ""
  echo "═══ Phase 3 — HTLC Lock (Bank-A → Bank-B) ═══"

  echo ""
  echo "=== [7/19] Bank-A locks $LOCK_AMOUNT tCeBM for Bank-B ==="
  echo "  agreement: FX_CROSS_SPOKE_001"
  echo "  receiver:  $PALADIN_IDENTITY_B"
  lock_htlc_bank_a
  echo "  contract_id:   $CONTRACT_ID_A"
  echo "  hash_lock:     ${HASH_LOCK_A:0:32}..."
  echo "  zeto_tx_hash:  ${ZETO_TX_LOCK_A:-"(pending)"}"

  echo ""
  echo "=== [8/19] Verify HTLC lock status & retrieve secret ==="
  verify_lock_status_bank_a
  echo "  state:  HTLC_STATE_LOCKED  ✓"
  echo "  secret: ${SECRET_A:0:16}... (${#SECRET_A} hex chars)"

  # ─── Phase 4: HTLC Settle ───────────────────────────────────────────

  echo ""
  echo "═══ Phase 4 — HTLC Settle (Happy Path) ═══"

  echo ""
  echo "=== [9/19] Settle HTLC with secret ==="
  echo "  contract_id: $CONTRACT_ID_A"
  echo "  secret:      ${SECRET_A:0:16}..."
  settle_htlc_bank_a
  echo "  zeto_tx_hash: ${ZETO_TX_SETTLE_A:-"(pending)"}"

  echo ""
  echo "=== [10/19] Verify settled status ==="
  verify_settled_status_bank_a
  echo "  state: HTLC_STATE_SETTLED  ✓"

  echo ""
  echo "=== [11/19] Check Bank-A balance after settle ==="
  check_balance_bank_a_after
  echo "  balance before: $BALANCE_A_BEFORE"
  echo "  balance after:  $BALANCE_A_AFTER"

  # ─── Phase 5: HTLC Refund ───────────────────────────────────────────

  echo ""
  echo "═══ Phase 5 — HTLC Refund (Timeout Path on Bank-B) ═══"

  echo ""
  echo "=== [12/19] Bank-B locks $LOCK_AMOUNT tCeBM with expired timelock ==="
  echo "  agreement: FX_CROSS_SPOKE_002"
  echo "  receiver:  $PALADIN_IDENTITY_A"
  echo "  timelock:  (expired — 60s in the past)"
  lock_htlc_bank_b_expired
  echo "  contract_id:  $CONTRACT_ID_B"
  echo "  hash_lock:    ${HASH_LOCK_B:0:32}..."
  echo "  zeto_tx_hash: ${ZETO_TX_LOCK_B:-"(pending)"}"

  echo ""
  echo "=== [13/19] Bank-B requests refund ==="
  echo "  contract_id: $CONTRACT_ID_B"
  refund_htlc_bank_b
  echo "  zeto_tx_hash: ${ZETO_TX_REFUND_B:-"(pending)"}"

  echo ""
  echo "=== [14/19] Verify refunded status ==="
  verify_refunded_status_bank_b
  echo "  state: HTLC_STATE_REFUNDED  ✓"

  echo ""
  echo "=== [15/19] Check Bank-B balance after refund ==="
  check_balance_bank_b_after
  echo "  balance before: $BALANCE_B_BEFORE"
  echo "  balance after:  $BALANCE_B_AFTER"

  # ─── Phase 6: Search & Report ───────────────────────────────────────

  echo ""
  echo "═══ Phase 6 — Search & Report ═══"

  echo ""
  echo "=== [16/19] Search settled HTLCs on Bank-A ==="
  search_settled_bank_a

  echo ""
  echo "=== [17/19] Search refunded HTLCs on Bank-B ==="
  search_refunded_bank_b

  echo ""
  echo "=== [18/19] Search by agreement_id FX_CROSS_SPOKE_001 ==="
  search_by_agreement_bank_a

  # ─── Summary ────────────────────────────────────────────────────────

  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  [19/19] HTLC Cross-Spoke FX Exchange — Summary"
  echo "══════════════════════════════════════════════════════════════"
  echo ""
  echo "  ── Token Minting ──"
  echo "  Bank-A mint tx:   ${MINT_TX_A:-"(noop)"}"
  echo "  Bank-B mint tx:   ${MINT_TX_B:-"(noop)"}"
  echo ""
  echo "  ── HTLC Settle (Happy Path — Bank-A) ──"
  echo "  Contract ID:      $CONTRACT_ID_A"
  echo "  Hash Lock:        ${HASH_LOCK_A:0:32}..."
  echo "  Secret:           ${SECRET_A:0:16}..."
  echo "  Lock Zeto TX:     ${ZETO_TX_LOCK_A:-"(n/a)"}"
  echo "  Settle Zeto TX:   ${ZETO_TX_SETTLE_A:-"(n/a)"}"
  echo "  State:            SETTLED ✓"
  echo ""
  echo "  ── HTLC Refund (Timeout — Bank-B) ──"
  echo "  Contract ID:      $CONTRACT_ID_B"
  echo "  Hash Lock:        ${HASH_LOCK_B:0:32}..."
  echo "  Lock Zeto TX:     ${ZETO_TX_LOCK_B:-"(n/a)"}"
  echo "  Refund Zeto TX:   ${ZETO_TX_REFUND_B:-"(n/a)"}"
  echo "  State:            REFUNDED ✓"
  echo ""
  echo "  ── Balances ──"
  echo "  Bank-A before:    $BALANCE_A_BEFORE"
  echo "  Bank-A after:     $BALANCE_A_AFTER"
  echo "  Bank-B before:    $BALANCE_B_BEFORE"
  echo "  Bank-B after:     $BALANCE_B_AFTER"
  echo ""
  echo "══════════════════════════════════════════════════════════════"
  echo "  HTLC Cross-Spoke FX Exchange completed successfully"
  echo "══════════════════════════════════════════════════════════════"
  echo ""
}

main "$@"
