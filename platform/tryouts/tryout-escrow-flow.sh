#!/usr/bin/env bash
# tryout-escrow-flow.sh — End-to-end escrow lifecycle (spoke-a: bank-a ↔ central-bank-a)
#
# Exercises the full deposit → fiat-exchange → escrow (fCeBM→tCeBM) → redeem
# (tCeBM→fCeBM) lifecycle between a commercial bank (bank-a) and the Central
# Bank (central-bank-a), both on spoke-a (chain 1338).
#
# Flow (15 steps):
#
#   Step  1  Bank-A operator login (Bank-A Keycloak)
#   Step  2  Central Bank governance login (CB Keycloak)
#   Step  3  Register deposit (Bank-A → CB proxy)
#   Step  4  List deposits (Bank-A)
#   Step  5  Approve deposit (CB governance)
#   Step  6  Fiat exchange — mint fCeBM (CB governance)
#   Step  7  Request escrow fCeBM→tCeBM (Bank-A → CB proxy)
#   Step  8  List escrows (Bank-A)
#   Step  9  Approve escrow — burn fCeBM + mint tCeBM (CB governance)
#   Step 10  Check tCeBM balance (Bank-A)
#   Step 11  Request redeem tCeBM→fCeBM (Bank-A → CB proxy — includes Zeto transfer)
#   Step 12  List redeems (Bank-A)
#   Step 13  Approve redeem — mint fCeBM (CB governance)
#   Step 14  List deposits (CB — final state)
#   Step 15  List escrows + redeems (CB — final state)
#
# Prerequisites:
#   - Both stacks running: make dev.up (or deploy.up-backend-spoke-a)
#   - curl, jq
#   - backend/config/.env.infra.bank-a    with KC_CLIENT_SECRET
#   - backend/config/.env.infra.central-bank-a with KC_CLIENT_SECRET
#   - Contracts deployed: make contracts.deploy-all-with-sync
#
# Usage:
#   ./tryout-escrow-flow.sh
#
# Environment variables (optional):
#   BANK_URL    Bank-A API base         (default: http://localhost:18080/api/v1)
#   CB_URL      Central Bank API base   (default: http://localhost:38080/api/v1)
#   BANK_ENV    Bank-A .env file        (default: backend/config/.env.infra.bank-a)
#   CB_ENV      Central Bank .env file  (default: backend/config/.env.infra.central-bank-a)
#   AMOUNT      Amount to use for all operations (default: 10000000)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BANK_URL="${BANK_URL:-http://localhost:18080/api/v1}"
CB_URL="${CB_URL:-http://localhost:38080/api/v1}"
BANK_ENV="${BANK_ENV:-backend/config/.env.infra.bank-a}"
CB_ENV="${CB_ENV:-backend/config/.env.infra.central-bank-a}"
AMOUNT="${AMOUNT:-10000000}"

# Globals populated during execution
BANK_TOKEN=""
CB_TOKEN=""
DEPOSIT_ID=""
ESCROW_ID=""
REDEEM_ID=""

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
  for f in "$BANK_ENV" "$CB_ENV"; do
    if [ ! -f "$f" ]; then
      echo "ERROR: env file '$f' not found." >&2
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

# http_post URL TOKEN PAYLOAD EXPECTED_CODE
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
    echo "  ERROR: POST $url failed (HTTP $code, expected $expected)." >&2
    echo "  Response: $body" >&2
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
    echo "  ERROR: GET $url failed (HTTP $code, expected $expected)." >&2
    echo "  Response: $body" >&2
    exit 1
  fi
  echo "$body"
}

# ---------------------------------------------------------------------------
# Step 1 — Bank-A operator login
# ---------------------------------------------------------------------------
step_01_bank_login() {
  echo "=== Step 1: Bank-A operator login ==="
  local kc_secret resp
  kc_secret=$(read_kc_secret "$BANK_ENV")
  resp=$(curl -s -X POST "$BANK_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"bank-a-client\", \"clientSecret\": \"$kc_secret\"}")
  BANK_TOKEN=$(echo "$resp" | jq -r '.accessToken // empty')
  if [ -z "$BANK_TOKEN" ]; then
    echo "  ERROR: Bank-A login failed: $resp" >&2; exit 1
  fi
  echo "  OK — Bank-A token obtained"
}

# ---------------------------------------------------------------------------
# Step 2 — Central Bank governance login
# ---------------------------------------------------------------------------
step_02_cb_login() {
  echo "=== Step 2: Central Bank governance login ==="
  local kc_secret resp
  kc_secret=$(read_kc_secret "$CB_ENV")
  resp=$(curl -s -X POST "$CB_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"central-bank-a-client\", \"clientSecret\": \"$kc_secret\"}")
  CB_TOKEN=$(echo "$resp" | jq -r '.accessToken // empty')
  if [ -z "$CB_TOKEN" ]; then
    echo "  ERROR: CB login failed: $resp" >&2; exit 1
  fi
  echo "  OK — CB governance token obtained"
}

# ---------------------------------------------------------------------------
# Step 3 — Register deposit (Bank-A → CB proxy)
# ---------------------------------------------------------------------------
step_03_register_deposit() {
  echo "=== Step 3: Register deposit (amount=$AMOUNT) ==="
  local resp
  resp=$(http_post "$BANK_URL/payments/deposits" "$BANK_TOKEN" \
    "{\"amount\": \"$AMOUNT\"}" "201")
  DEPOSIT_ID=$(echo "$resp" | jq -r '.deposit_id')
  echo "  deposit_id=$DEPOSIT_ID"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 4 — List deposits (Bank-A)
# ---------------------------------------------------------------------------
step_04_list_deposits() {
  echo "=== Step 4: List deposits (Bank-A) ==="
  local resp
  resp=$(http_get "$BANK_URL/payments/deposits" "$BANK_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 5 — Approve deposit (CB governance)
# ---------------------------------------------------------------------------
step_05_approve_deposit() {
  echo "=== Step 5: Approve deposit $DEPOSIT_ID (CB governance) ==="
  local resp
  resp=$(http_post "$CB_URL/payments/deposits/approve" "$CB_TOKEN" \
    "{\"deposit_id\": \"$DEPOSIT_ID\"}" "201")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 6 — Fiat exchange: mint fCeBM (CB governance)
# ---------------------------------------------------------------------------
step_06_fiat_exchange() {
  echo "=== Step 6: Fiat exchange — mint fCeBM for deposit $DEPOSIT_ID ==="
  local resp
  resp=$(http_post "$CB_URL/payments/deposits/fiat-exchange" "$CB_TOKEN" \
    "{\"deposit_id\": \"$DEPOSIT_ID\"}" "201")
  echo "  fCeBM minted. tx_hash=$(echo "$resp" | jq -r '.tx_hash')"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 7 — Request escrow fCeBM→tCeBM (Bank-A → CB proxy)
# ---------------------------------------------------------------------------
step_07_request_escrow() {
  echo "=== Step 7: Request escrow fCeBM→tCeBM (amount=$AMOUNT) ==="
  local resp
  resp=$(http_post "$BANK_URL/payments/escrows" "$BANK_TOKEN" \
    "{\"amount\": \"$AMOUNT\"}" "201")
  ESCROW_ID=$(echo "$resp" | jq -r '.escrow_id')
  echo "  escrow_id=$ESCROW_ID"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 8 — List escrows (Bank-A)
# ---------------------------------------------------------------------------
step_08_list_escrows() {
  echo "=== Step 8: List escrows (Bank-A) ==="
  local resp
  resp=$(http_get "$BANK_URL/payments/escrows" "$BANK_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 9 — Approve escrow: burn fCeBM + mint tCeBM (CB governance)
# ---------------------------------------------------------------------------
step_09_approve_escrow() {
  echo "=== Step 9: Approve escrow $ESCROW_ID (CB governance) ==="
  local resp
  resp=$(http_post "$CB_URL/payments/escrows/approve" "$CB_TOKEN" \
    "{\"escrow_id\": \"$ESCROW_ID\"}" "201")
  echo "  burn_tx_hash=$(echo "$resp" | jq -r '.burn_tx_hash')"
  echo "  zeto_mint_tx_hash=$(echo "$resp" | jq -r '.zeto_mint_tx_hash')"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 10 — Check tCeBM balance (Bank-A)
# ---------------------------------------------------------------------------
step_10_check_balance() {
  echo "=== Step 10: Check tCeBM balance (Bank-A) ==="
  local resp
  resp=$(http_get "$BANK_URL/token/balance" "$BANK_TOKEN" "200")
  echo "  tCeBM balance=$(echo "$resp" | jq -r '.balance')"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 11 — Request redeem tCeBM→fCeBM (Bank-A → CB proxy, includes Zeto transfer)
# ---------------------------------------------------------------------------
step_11_request_redeem() {
  echo "=== Step 11: Request redeem tCeBM→fCeBM (amount=$AMOUNT) ==="
  local resp
  resp=$(http_post "$BANK_URL/payments/redeems" "$BANK_TOKEN" \
    "{\"amount\": \"$AMOUNT\"}" "201")
  REDEEM_ID=$(echo "$resp" | jq -r '.redeem_id')
  echo "  redeem_id=$REDEEM_ID"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 12 — List redeems (Bank-A)
# ---------------------------------------------------------------------------
step_12_list_redeems() {
  echo "=== Step 12: List redeems (Bank-A) ==="
  local resp
  resp=$(http_get "$BANK_URL/payments/redeems" "$BANK_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 13 — Approve redeem: mint fCeBM (CB governance)
# ---------------------------------------------------------------------------
step_13_approve_redeem() {
  echo "=== Step 13: Approve redeem $REDEEM_ID (CB governance) ==="
  local resp
  resp=$(http_post "$CB_URL/payments/redeems/approve" "$CB_TOKEN" \
    "{\"redeem_id\": \"$REDEEM_ID\"}" "201")
  echo "  fiat_mint_tx_hash=$(echo "$resp" | jq -r '.fiat_mint_tx_hash')"
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 14 — List deposits (CB — final state)
# ---------------------------------------------------------------------------
step_14_cb_list_deposits() {
  echo "=== Step 14: List deposits — final state (CB) ==="
  local resp
  resp=$(http_get "$CB_URL/payments/deposits" "$CB_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 15 — List escrows + redeems (CB — final state)
# ---------------------------------------------------------------------------
step_15_cb_final_state() {
  echo "=== Step 15: List escrows + redeems — final state (CB) ==="
  local resp

  echo "--- Escrows ---"
  resp=$(http_get "$CB_URL/payments/escrows" "$CB_TOKEN" "200")
  echo "$resp" | jq .

  echo "--- Redeems ---"
  resp=$(http_get "$CB_URL/payments/redeems" "$CB_TOKEN" "200")
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  require_cmds
  require_env_files

  echo ""
  echo "=========================================="
  echo "  ESCROW FLOW TRYOUT (spoke-a)"
  echo "=========================================="
  echo "  Bank-A URL : $BANK_URL"
  echo "  CB URL     : $CB_URL"
  echo "  Amount     : $AMOUNT"
  echo "=========================================="
  echo ""

  step_01_bank_login
  echo ""
  step_02_cb_login
  echo ""
  step_03_register_deposit
  echo ""
  step_04_list_deposits
  echo ""
  step_05_approve_deposit
  echo ""
  step_06_fiat_exchange
  echo ""
  step_07_request_escrow
  echo ""
  step_08_list_escrows
  echo ""
  step_09_approve_escrow
  echo ""
  step_10_check_balance
  echo ""
  step_11_request_redeem
  echo ""
  step_12_list_redeems
  echo ""
  step_13_approve_redeem
  echo ""
  step_14_cb_list_deposits
  echo ""
  step_15_cb_final_state

  echo ""
  echo "=========================================="
  echo "  ESCROW FLOW COMPLETE"
  echo "=========================================="
  echo "  Deposit  : $DEPOSIT_ID"
  echo "  Escrow   : $ESCROW_ID"
  echo "  Redeem   : $REDEEM_ID"
  echo "=========================================="
}

main "$@"
