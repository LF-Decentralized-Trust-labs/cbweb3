#!/usr/bin/env bash
set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "────────────────────────────────────────────────────────"
echo "Tryout: Commercial Bank Payment/Token Routes"
echo "────────────────────────────────────────────────────────"

# Configuration
BANK_A_URL="http://localhost:18080"
API_BASE="${BANK_A_URL}/api/v1"
KEYCLOAK_URL="http://localhost:8081"

# Keycloak client credentials (read from env file)
KC_REALM="bank-a"
KC_CLIENT="bank-a-client"
KC_SECRET=$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2 || echo '')

if [[ -z "$KC_SECRET" ]]; then
  echo -e "${RED}✗ KC_CLIENT_SECRET not found in backend/config/.env.infra.bank-a${NC}"
  exit 1
fi

# Route test helper
test_route() {
  local method="$1"
  local route="$2"
  local expected_status="$3"
  local description="$4"

  echo -e "${YELLOW}Testing: ${method} ${route}${NC}"

  response=$(curl -s -w "\n%{http_code}" \
    -X "${method}" \
    -H "Accept: application/json" \
    --cookie "access_token=${TOKEN}" \
    "${API_BASE}${route}")

  http_code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | head -n-1)

  if [[ "$http_code" == "$expected_status" ]]; then
    echo -e "${GREEN}✓ ${description} (HTTP ${http_code})${NC}"
    if [[ -n "$body" ]]; then
      echo "  Response: $(echo "$body" | jq -c '.' 2>/dev/null || echo "$body")"
    fi
    return 0
  else
    echo -e "${RED}✗ ${description}${NC}"
    echo "  Expected HTTP ${expected_status}, got ${http_code}"
    echo "  Body: $body"
    return 1
  fi
}

# 1. Authenticate as Bank-A
echo ""
echo "────────────────────────────────────────────────────────"
echo "1. Authenticating as Bank-A"
echo "────────────────────────────────────────────────────────"

auth_response=$(curl -sS -X POST \
  "${KEYCLOAK_URL}/realms/${KC_REALM}/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=${KC_CLIENT}" \
  -d "client_secret=${KC_SECRET}")

TOKEN=$(echo "$auth_response" | jq -r '.access_token // empty')

if [[ -z "$TOKEN" ]]; then
  echo -e "${RED}✗ Authentication failed${NC}"
  echo "Response: $auth_response"
  exit 1
fi

echo -e "${GREEN}✓ Bank-A authenticated${NC}"

# 2. Test Payment routes
echo ""
echo "────────────────────────────────────────────────────────"
echo "2. Testing Payment routes"
echo "────────────────────────────────────────────────────────"

# Test list
passed=0
failed=0

# Deposits
if test_route "GET" "/payments/deposits" "200" "List deposits"; then
  ((passed++))
else
  ((failed++))
fi

echo ""

# Redeems
if test_route "GET" "/payments/redeems" "200" "List redeems"; then
  ((passed++))
else
  ((failed++))
fi

echo ""

# Escrows
if test_route "GET" "/payments/escrows" "200" "List escrows"; then
  ((passed++))
else
  ((failed++))
fi

echo ""

# Token Balance
if test_route "GET" "/token/balance" "200" "Token balance"; then
  ((passed++))
else
  ((failed++))
  echo "  ⚠ Token balance may fail if Paladin is not configured"
fi

# Final result
echo ""
echo "────────────────────────────────────────────────────────"
echo "Result"
echo "────────────────────────────────────────────────────────"
echo -e "${GREEN}Passed: ${passed}${NC}"
if [[ $failed -gt 0 ]]; then
  echo -e "${RED}Failed: ${failed}${NC}"
  exit 1
else
  echo -e "${GREEN}✓ All routes working correctly${NC}"
fi
