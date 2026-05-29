#!/usr/bin/env bash
set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 Tryout: Full Deposit Flow FIAT → tCeBM (Bank-A)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

BANK_A_URL="http://localhost:18080"
CB_A_URL="http://localhost:38080"
KEYCLOAK_URL="http://localhost:8081"

# Test values
DEPOSIT_AMOUNT="10000"  # Deposit 10,000 tCeBM

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 1. Authentication
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${BLUE}━━━ Step 1: Authentication ━━━${NC}"

KC_BANK_A_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.bank-a | cut -d= -f2)
KC_CB_A_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.central-bank-a | cut -d= -f2)

# Bank-A token
BANK_A_TOKEN=$(curl -sS -X POST \
  "${KEYCLOAK_URL}/realms/bank-a/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=bank-a-client" \
  -d "client_secret=${KC_BANK_A_SECRET}" | jq -r '.access_token')

# CB-A token
CB_A_TOKEN=$(curl -sS -X POST \
  "${KEYCLOAK_URL}/realms/central-bank-a/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=central-bank-a-client" \
  -d "client_secret=${KC_CB_A_SECRET}" | jq -r '.access_token')

if [[ -z "$BANK_A_TOKEN" || -z "$CB_A_TOKEN" ]]; then
  echo -e "${RED}✗ Authentication failed${NC}"
  exit 1
fi

echo -e "${GREEN}✓ Bank-A authenticated${NC}"
echo -e "${GREEN}✓ CB-A authenticated${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 2. Bank-A: Register FIAT Deposit
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${BLUE}━━━ Step 2: Bank-A registers FIAT deposit (${DEPOSIT_AMOUNT}) ━━━${NC}"

deposit_response=$(curl -sS -X POST \
  "${BANK_A_URL}/api/v1/payments/deposits" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${BANK_A_TOKEN}" \
  -d "{\"amount\":\"${DEPOSIT_AMOUNT}\"}")

DEPOSIT_ID=$(echo "$deposit_response" | jq -r '.deposit_id // .id // empty')

if [[ -z "$DEPOSIT_ID" ]]; then
  echo -e "${RED}✗ Failed to create deposit${NC}"
  echo "Response: $deposit_response"
  exit 1
fi

echo -e "${GREEN}✓ Deposit created: ${DEPOSIT_ID}${NC}"
echo "  Amount: ${DEPOSIT_AMOUNT} FIAT"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 3. CB-A: Approve Deposit
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${BLUE}━━━ Step 3: CB-A approves deposit ━━━${NC}"

approve_response=$(curl -sS -X POST \
  "${CB_A_URL}/api/v1/payments/deposits/approve" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${CB_A_TOKEN}" \
  -d "{\"deposit_id\":\"${DEPOSIT_ID}\"}")

echo -e "${GREEN}✓ Deposit approved by CB-A${NC}"
echo "  Response: $(echo "$approve_response" | jq -c '.')"

# Wait for processing
sleep 2

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 4. Bank-A: Query tCeBM balance (should have value after approve)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${BLUE}━━━ Step 4: Query Bank-A tCeBM balance ━━━${NC}"

tcebm_balance_final=$(curl -sS \
  "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ FINAL RESULT${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "🪙 tCeBM balance: ${YELLOW}${tcebm_balance_final}${NC}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✓ Test complete! tCeBM balance queried.${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
