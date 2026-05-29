#!/usr/bin/env bash
# tryout-escrow-flow.sh — Deposit → tCeBM lifecycle (spoke-a: bank-a ↔ central-bank-a)
#
# NOTE: The escrow (fCeBM→tCeBM) step has been removed. tCeBM is now minted
# directly upon deposit approval. This script exercises the simplified flow.
#
# Flow:
#   Step  1  Bank-A operator login (Bank-A Keycloak)
#   Step  2  Central Bank governance login (CB Keycloak)
#   Step  3  Register deposit (Bank-A → CB proxy)
#   Step  4  List deposits (Bank-A)
#   Step  5  Approve deposit (CB governance) → mints tCeBM directly
#   Step  6  Check tCeBM balance (Bank-A)
#   Step  7  Request redeem (Bank-A → CB proxy)
#   Step  8  List redeems (Bank-A)
#   Step  9  Approve redeem (CB governance)
#   Step 10  List deposits + redeems (CB — final state)
#
# Prerequisites:
#   - Both stacks running: make dev.up (or deploy.up-backend-spoke-a)
#   - curl, jq
#   - backend/config/.env.infra.bank-a    with KC_CLIENT_SECRET
#   - backend/config/.env.infra.central-bank-a with KC_CLIENT_SECRET
set -euo pipefail

# ─── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

die()   { echo -e "${RED}✗ $*${NC}" >&2; exit 1; }
ok()    { echo -e "${GREEN}✓ $*${NC}"; }
info()  { echo -e "${CYAN}  ℹ $*${NC}"; }
step()  { echo ""; echo -e "${BOLD}${BLUE}━━━ $* ━━━${NC}"; }

# ─── Config ───────────────────────────────────────────────────────────────────
BANK_A_URL="${BANK_A_URL:-http://localhost:18080}"
CB_A_URL="${CB_A_URL:-http://localhost:38080}"
KC_URL="${KEYCLOAK_URL:-http://localhost:8081}"

BANK_A_BESU_ADDR="${BANK_A_BESU_ADDR:-0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef}"
DEPOSIT_AMOUNT="${DEPOSIT_AMOUNT:-10000000000000000000000}"   # 10,000 tCeBM
REDEEM_AMOUNT="${REDEEM_AMOUNT:-5000000000000000000000}"     # 5,000 tCeBM

KC_BANK_A_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.bank-a | cut -d= -f2 || true)
KC_CB_A_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.central-bank-a | cut -d= -f2 || true)
[[ -z "${KC_BANK_A_SECRET}" ]] && die "KC_CLIENT_SECRET not found in .env.infra.bank-a"
[[ -z "${KC_CB_A_SECRET}"   ]] && die "KC_CLIENT_SECRET not found in .env.infra.central-bank-a"

# ─── Helper ───────────────────────────────────────────────────────────────────
get_token() {
  local realm="$1" client_id="$2" secret="$3"
  curl -sS -X POST "${KC_URL}/realms/${realm}/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=client_credentials&client_id=${client_id}&client_secret=${secret}" \
    | jq -r '.access_token'
}

jfield() { echo "$1" | jq -r "$2 // empty"; }

# ─── Step 1 ───────────────────────────────────────────────────────────────────
step "Step 1: Bank-A authentication"
BANK_A_TOKEN=$(get_token "bank-a" "bank-a-client" "${KC_BANK_A_SECRET}")
[[ -z "$BANK_A_TOKEN" || "$BANK_A_TOKEN" == "null" ]] && die "Bank-A token failed"
ok "Bank-A authenticated"

# ─── Step 2 ───────────────────────────────────────────────────────────────────
step "Step 2: Central Bank-A authentication"
CB_A_TOKEN=$(get_token "central-bank-a" "central-bank-a-client" "${KC_CB_A_SECRET}")
[[ -z "$CB_A_TOKEN" || "$CB_A_TOKEN" == "null" ]] && die "CB-A token failed"
ok "CB-A authenticated"

# ─── Step 3 ───────────────────────────────────────────────────────────────────
step "Step 3: Register deposit (Bank-A)"
deposit_payload=$(printf '{"requester_besu_address":"%s","amount":"%s"}' "$BANK_A_BESU_ADDR" "$DEPOSIT_AMOUNT")
deposit_resp=$(curl -sS -X POST "${BANK_A_URL}/api/v1/payments/deposits" \
  -H "Content-Type: application/json" --cookie "access_token=${BANK_A_TOKEN}" \
  -d "$deposit_payload")
DEPOSIT_ID=$(jfield "$deposit_resp" '.deposit_id // .id')
[[ -z "$DEPOSIT_ID" || "$DEPOSIT_ID" == "null" ]] && die "Deposit failed: $deposit_resp"
ok "Deposit registered: ${DEPOSIT_ID}"

# ─── Step 4 ───────────────────────────────────────────────────────────────────
step "Step 4: List deposits (Bank-A)"
curl -sS "${BANK_A_URL}/api/v1/payments/deposits" --cookie "access_token=${BANK_A_TOKEN}" \
  | jq -r '.deposits[]? | "  \(.id) — \(.status)"' || true
ok "Deposits listed"

# ─── Step 5 ───────────────────────────────────────────────────────────────────
step "Step 5: CB-A approves deposit (mints tCeBM directly)"
approve_resp=$(curl -sS -X POST "${CB_A_URL}/api/v1/payments/deposits/approve" \
  -H "Content-Type: application/json" --cookie "access_token=${CB_A_TOKEN}" \
  -d "{\"deposit_id\": \"${DEPOSIT_ID}\"}")
ok "Deposit approved"
info "mint_tx_hash: $(jfield "$approve_resp" '.mint_tx_hash')"
sleep 3

# ─── Step 6 ───────────────────────────────────────────────────────────────────
step "Step 6: Check tCeBM balance (Bank-A)"
tcebm_balance=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')
ok "tCeBM balance: ${YELLOW}${tcebm_balance}${NC}"

# ─── Step 7 ───────────────────────────────────────────────────────────────────
step "Step 7: Request redeem (Bank-A)"
redeem_resp=$(curl -sS -X POST "${BANK_A_URL}/api/v1/payments/redeems" \
  -H "Content-Type: application/json" --cookie "access_token=${BANK_A_TOKEN}" \
  -d "{\"amount\": \"${REDEEM_AMOUNT}\"}")
REDEEM_ID=$(jfield "$redeem_resp" '.redeem_id // .id')
[[ -z "$REDEEM_ID" || "$REDEEM_ID" == "null" ]] && die "Redeem failed: $redeem_resp"
ok "Redeem created: ${REDEEM_ID}"

# ─── Step 8 ───────────────────────────────────────────────────────────────────
step "Step 8: List redeems (Bank-A)"
curl -sS "${BANK_A_URL}/api/v1/payments/redeems" --cookie "access_token=${BANK_A_TOKEN}" \
  | jq -r '.redeems[]? | "  \(.id) — \(.status)"' || true
ok "Redeems listed"

sleep 2

# ─── Step 9 ───────────────────────────────────────────────────────────────────
step "Step 9: CB-A approves redeem"
approve_redeem=$(curl -sS -X POST "${CB_A_URL}/api/v1/payments/redeems/approve" \
  -H "Content-Type: application/json" --cookie "access_token=${CB_A_TOKEN}" \
  -d "{\"redeem_id\": \"${REDEEM_ID}\"}")
ok "Redeem approved"
info "mint_tx_hash: $(jfield "$approve_redeem" '.mint_tx_hash')"
sleep 3

# ─── Step 10 ──────────────────────────────────────────────────────────────────
step "Step 10: Final state (CB-A)"
curl -sS "${CB_A_URL}/api/v1/payments/deposits" --cookie "access_token=${CB_A_TOKEN}" \
  | jq -r '.deposits[]? | "  deposit \(.id) — \(.status)"' || true
curl -sS "${CB_A_URL}/api/v1/payments/redeems" --cookie "access_token=${CB_A_TOKEN}" \
  | jq -r '.redeems[]? | "  redeem  \(.id) — \(.status)"' || true
ok "Final state listed"

tcebm_final=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ LIFECYCLE COMPLETE${NC}"
echo "  Deposit:      ${DEPOSIT_AMOUNT} wei"
echo "  tCeBM before redeem: ${tcebm_balance}"
echo "  Redeem:       ${REDEEM_AMOUNT} wei"
echo "  tCeBM after:  ${tcebm_final}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
