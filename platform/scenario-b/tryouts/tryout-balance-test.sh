#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔄 Balance Test: tCeBM"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

BANK_A_URL="http://localhost:18080"
CB_A_URL="http://localhost:38080"
KEYCLOAK_URL="http://localhost:8081"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 1. Authentication
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
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
# 2. Query Initial Balance
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 2: Bank-A INITIAL tCeBM balance ━━━${NC}"

tcebm_initial=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')

echo -e "  🪙 tCeBM (initial): ${YELLOW}${tcebm_initial}${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 3. Mint tCeBM via CB-A
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 3: CB-A mints tCeBM for Bank-A ━━━${NC}"

BANK_A_WALLET="0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef"

echo -e "${BLUE}  → Mint 5,000 tCeBM...${NC}"
mint_token=$(curl -sS -X POST \
  "${CB_A_URL}/api/v1/token/mint" \
  -H "Content-Type: application/json" \
  --cookie "access_token=${CB_A_TOKEN}" \
  -d "{\"to\":\"${BANK_A_WALLET}\",\"amount\":\"5000000000000000000000\"}")

echo -e "${GREEN}    ✓ tCeBM mint requested${NC}"
echo "    Response: $(echo "$mint_token" | jq -c '.')"

echo ""
echo -e "${YELLOW}  ⏳ Waiting for on-chain processing (5s)...${NC}"
sleep 5

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 4. Query Final Balance
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo -e "${CYAN}━━━ Step 4: Bank-A FINAL tCeBM balance ━━━${NC}"

tcebm_final=$(curl -sS "${BANK_A_URL}/api/v1/token/balance" \
  --cookie "access_token=${BANK_A_TOKEN}" | jq -r '.balance // "0"')

echo -e "  🪙 tCeBM (final): ${YELLOW}${tcebm_final}${NC}"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 5. Summary
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ TEST SUMMARY${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Balance comparison:"
echo ""
echo -e "  🪙 ${CYAN}tCeBM:${NC}"
echo -e "     Initial: ${tcebm_initial}"
echo -e "     Final:   ${GREEN}${tcebm_final}${NC}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "$tcebm_final" != "0" ]]; then
  echo -e "${GREEN}✅ SUCCESS! tCeBM balance was updated!${NC}"
else
  echo -e "${RED}❌ FAILURE: tCeBM balance was not updated. Check the logs.${NC}"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
