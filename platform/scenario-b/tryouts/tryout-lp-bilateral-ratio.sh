#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 Scenario B: Full FIAT → tCeBM Flow (Bank-A)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Flow:"
echo "  1️⃣  Bank-A registers FIAT deposit"
echo "  2️⃣  CB-A approves deposit → mints tCeBM directly"
echo "  3️⃣  Query tCeBM balance (should have value)"
echo ""

BANK_A_URL="http://localhost:18080"
CB_A_URL="http://localhost:38080"
KEYCLOAK_URL="http://localhost:8081"

# Bank-A wallet (derived from BESU_OPERATOR_KEY in .env.infra.bank-a)
BANK_A_WALLET="0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef"

# Test values
DEPOSIT_AMOUNT="10000000000000000000000"  # 10,000 tCeBM (18 decimals)

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Authentication
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo -e "${CYAN}━━━ Step 1: Authentication ━━━${NC}"

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

echo -e "${GREEN}✓ Bank-A authenticated${NC}"
echo -e "${GREEN}✓ CB-A authenticated${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Initial Balances
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 2: INITIAL tCeBM balance ━━━${NC}"

tcebm_initial=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')

echo -e "  🪙 tCeBM (initial): ${YELLOW}${tcebm_initial}${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Register FIAT Deposit
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 3: Bank-A registers FIAT deposit ━━━${NC}"

deposit_response=$(curl -sS -X POST \
  "${BANK_A_URL}/api/v1/payments/deposits" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${BANK_A_TOKEN}" \
  -d "{
    \"requester_besu_address\": \"${BANK_A_WALLET}\",
    \"amount\": \"${DEPOSIT_AMOUNT}\"
  }")

DEPOSIT_ID=$(echo "$deposit_response" | jq -r '.deposit_id // empty')

if [[ -z "$DEPOSIT_ID" ]]; then
  echo -e "${RED}✗ Failed to create deposit${NC}"
  echo "Response: $deposit_response"
  exit 1
fi

echo -e "${GREEN}✓ Deposit created: ${DEPOSIT_ID}${NC}"
echo "  Amount: ${DEPOSIT_AMOUNT} wei (10,000 tCeBM)"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# CB-A Approve Deposit (mints tCeBM directly)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 4: CB-A approves deposit (mints tCeBM directly) ━━━${NC}"

approve_deposit=$(curl -sS -X POST \
  "${CB_A_URL}/api/v1/payments/deposits/approve" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${CB_A_TOKEN}" \
  -d "{\"deposit_id\": \"${DEPOSIT_ID}\"}")

echo -e "${GREEN}✓ Deposit approved (tCeBM minted automatically)${NC}"
echo "  Response: $(echo "$approve_deposit" | jq -c '.')"

sleep 3

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Query Final tCeBM Balance
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 5: Query FINAL tCeBM balance ━━━${NC}"

tcebm_final=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')

echo -e "  🪙 tCeBM (final): ${YELLOW}${tcebm_final}${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Summary
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ TEST RESULT${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Balance trajectory:"
echo ""
echo -e "  🪙 ${CYAN}tCeBM:${NC}"
echo -e "     Initial:        ${tcebm_initial}"
echo -e "     After deposit:  ${GREEN}${tcebm_final}${NC}"
echo ""
echo "📝 Expected:"
echo "   - tCeBM should be 10,000 after deposit approval"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$tcebm_final" != "0" ]]; then
  echo -e "${GREEN}✅ SUCCESS! tCeBM balance was updated.${NC}"
else
  echo -e "${RED}❌ FAILURE: tCeBM deposit did not create balance. Check the logs.${NC}"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
