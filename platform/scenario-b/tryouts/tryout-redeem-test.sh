#!/usr/bin/env bash
# Test for POST /api/v1/payments/redeems (Bank-A → CB-A)
# Prerequisite: Bank-A must have tCeBM to transfer and convert back to FIAT.

set -e

BANK_A_BASE_URL="http://localhost:18080"
CB_A_BASE_URL="http://localhost:38080"

echo "======================================"
echo "Redeem Test (tCeBM → FIAT)"
echo "======================================"
echo ""

# 1. Obtain Bank-A token
echo "1. Obtaining Bank-A access token..."
TOKEN_RESPONSE=$(curl -s -X POST "http://localhost:8081/realms/bank-a/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=bank-a-client" \
  -d "client_secret=bank-a-local-secret" \
  -d "grant_type=client_credentials")

BANK_A_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token')

if [ "$BANK_A_TOKEN" == "null" ] || [ -z "$BANK_A_TOKEN" ]; then
  echo "❌ Failed to obtain Bank-A token"
  echo "Response: $TOKEN_RESPONSE"
  exit 1
fi

echo "✅ Token obtained: ${BANK_A_TOKEN:0:20}..."
echo ""

# 2. Query initial Bank-A tCeBM balance
echo "2. Querying initial Bank-A tCeBM balance..."
INITIAL_TCEBM_BALANCE=$(curl -s -X GET "$BANK_A_BASE_URL/api/v1/token/balance" \
  --cookie "access_token=$BANK_A_TOKEN" | jq -r '.balance // "0"')

echo "   Initial tCeBM balance: $INITIAL_TCEBM_BALANCE"
echo ""

# 4. Request redeem of 1000 units (1000 wei tCeBM → FIAT)
echo "4. Requesting redeem of 1000 tCeBM units..."
REDEEM_AMOUNT="1000"
REDEEM_RESPONSE=$(curl -s -X POST "$BANK_A_BASE_URL/api/v1/payments/redeems" \
  --cookie "access_token=$BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"amount\": \"$REDEEM_AMOUNT\"}")

echo "   Response: $REDEEM_RESPONSE"

REDEEM_ID=$(echo "$REDEEM_RESPONSE" | jq -r '.redeem_id // .id // empty')

if [ -z "$REDEEM_ID" ]; then
  echo "❌ Failed to request redeem!"
  echo "Full response: $REDEEM_RESPONSE"
  exit 1
fi

echo "✅ Redeem requested successfully! ID: $REDEEM_ID"
echo ""

# 5. Wait a few seconds for CB-A to process
echo "5. Waiting for CB-A to process..."
sleep 5
echo ""

# 6. Query final tCeBM balance
echo "6. Querying final tCeBM balance..."
FINAL_TCEBM_BALANCE=$(curl -s -X GET "$BANK_A_BASE_URL/api/v1/token/balance" \
  --cookie "access_token=$BANK_A_TOKEN" | jq -r '.balance // "0"')

echo "   Final tCeBM balance: $FINAL_TCEBM_BALANCE"
echo "   Difference: $((INITIAL_TCEBM_BALANCE - FINAL_TCEBM_BALANCE)) (expected: +$REDEEM_AMOUNT to CB-A)"
echo ""

echo "======================================"
echo "✅ Redeem test completed!"
echo "======================================"
echo ""
echo "Summary:"
echo "  - Redeem ID: $REDEEM_ID"
echo "  - Amount: $REDEEM_AMOUNT"
echo "  - tCeBM: $INITIAL_TCEBM_BALANCE → $FINAL_TCEBM_BALANCE"
echo "  - FIAT: $INITIAL_FIAT_BALANCE → $FINAL_FIAT_BALANCE"
echo ""
