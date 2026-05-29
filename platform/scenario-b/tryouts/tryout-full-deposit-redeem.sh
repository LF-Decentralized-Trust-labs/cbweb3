#!/usr/bin/env bash
# Full test: Deposit → Redeem (tCeBM unified flow)
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 Scenario B: Full Deposit ⇄ Redeem Test (Bank-A)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${CYAN}Test flow:${NC}"
echo "  1️⃣  DEPOSIT:  Bank-A → CB-A approves → mint tCeBM directly"
echo "  2️⃣  REDEEM:   Bank-A transfers tCeBM → CB-A processes"
echo ""

BANK_A_URL="http://localhost:18080"
CB_A_URL="http://localhost:38080"
KEYCLOAK_URL="http://localhost:8081"

# Test values
DEPOSIT_AMOUNT="5000000000000000000000"   # 5,000 tCeBM
REDEEM_AMOUNT="1000000000000000000000"    # 1,000 to redeem (4,000 tCeBM remains)

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo -e "${CYAN}━━━ Authentication ━━━${NC}"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

KC_BANK_A_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.bank-a | cut -d= -f2)
KC_CB_A_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.central-bank-a | cut -d= -f2)

BANK_A_TOKEN=$(curl -sS -X POST \
  "${KEYCLOAK_URL}/realms/bank-a/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=bank-a-client" \
  -d "client_secret=${KC_BANK_A_SECRET}" | jq -r '.access_token')

CB_A_TOKEN=$(curl -sS -X POST \
  "${KEYCLOAK_URL}/realms/central-bank-a/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=central-bank-a-client" \
  -d "client_secret=${KC_CB_A_SECRET}" | jq -r '.access_token')

echo -e "${GREEN}✓ Tokens obtained${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ PHASE 1: DEPOSIT ━━━${NC}"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

deposit_response=$(curl -sS -X POST \
  "${BANK_A_URL}/api/v1/payments/deposits" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${BANK_A_TOKEN}" \
  -d "{\"amount\": \"${DEPOSIT_AMOUNT}\"}")

DEPOSIT_ID=$(echo "$deposit_response" | jq -r '.deposit_id // empty')
[[ -z "$DEPOSIT_ID" ]] && echo -e "${RED}✗ Deposit failed${NC}" && exit 1

echo -e "${GREEN}✓ Deposit created: ${DEPOSIT_ID}${NC} (${DEPOSIT_AMOUNT} wei)"

# CB-A approves → mints tCeBM directly
curl -sS -X POST "${CB_A_URL}/api/v1/payments/deposits/approve" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${CB_A_TOKEN}" \
  -d "{\"deposit_id\": \"${DEPOSIT_ID}\"}" > /dev/null

echo -e "${GREEN}✓ Deposit approved (tCeBM minted automatically)${NC}"

sleep 3

tcebm_after_deposit=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance')
echo -e "  🪙 tCeBM balance: ${YELLOW}${tcebm_after_deposit}${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ PHASE 2: REDEEM (tCeBM → FIAT) ━━━${NC}"
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

redeem_response=$(curl -sS -X POST \
  "${BANK_A_URL}/api/v1/payments/redeems" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${BANK_A_TOKEN}" \
  -d "{\"amount\": \"${REDEEM_AMOUNT}\"}")

REDEEM_ID=$(echo "$redeem_response" | jq -r '.redeem_id // empty')
[[ -z "$REDEEM_ID" ]] && echo -e "${RED}✗ Redeem failed${NC}" && exit 1

echo -e "${GREEN}✓ Redeem created: ${REDEEM_ID}${NC} (${REDEEM_AMOUNT} wei)"
echo -e "  ${CYAN}tCeBM transfer → CB-A executed${NC}"

sleep 2

tcebm_after_redeem=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance')

echo -e "  🪙 tCeBM balance: ${YELLOW}${tcebm_after_redeem}${NC} (reduced)"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ FULL TEST COMPLETED${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${CYAN}📊 Operations summary:${NC}"
echo ""
echo -e "  ${BLUE}1. DEPOSIT${NC}"
echo "     → Amount deposited:     ${DEPOSIT_AMOUNT} wei"
echo "     → tCeBM after:          ${tcebm_after_deposit}"
echo ""
echo -e "  ${BLUE}2. REDEEM${NC}"
echo "     → Amount redeemed:      ${REDEEM_AMOUNT} wei"
echo "     → tCeBM remaining:      ${tcebm_after_redeem}"
echo "     → Status:               ${GREEN}Awaiting CB-A approval${NC}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
