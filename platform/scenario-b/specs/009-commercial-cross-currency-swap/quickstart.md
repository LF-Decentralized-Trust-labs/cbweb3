# Quickstart: Commercial Cross-Currency Swap

**Feature**: 009-commercial-cross-currency-swap  
**Purpose**: Manual E2E validation do fluxo BRL → ARS para banco comercial  
**Date**: 2026-05-26 | **Updated**: 2026-05-27 (FR-012)

## Prerequisites

1. **Pool W-BRL-ARS ACTIVE no CB** (provisionado via specs 007/008 — tryout soberano antes):
   ```bash
   curl http://localhost:38080/api/v2/amm/pool/W-BRL-ARS/status \
     -H "Authorization: Bearer $TOKEN_CB_A"
   # Expected: {"pool_status": "ACTIVE", "reserve_a": "...", "reserve_b": "..."}
   ```

1b. **Mesmo status visível no banco comercial** (FR-012: proxy ao CB do spoke, sem auth no pool status):
   ```bash
   curl http://localhost:18080/api/v2/amm/pool/W-BRL-ARS/status | jq '.pool_status, .reserve_a'
   # Expected: ACTIVE + reservas iguais ao CB-A
   ```

2. **Banco comercial autenticado** (bank-a no Spoke-A):
   ```bash
   export BANK_A_URL="http://localhost:18080"  # Commercial bank API (api-gateway-bank-a)
   export TOKEN_BANK_A=$(curl -sf -X POST "$BANK_A_URL/api/v1/auth/login" \
     -H "Content-Type: application/json" \
     -d '{"clientId":"bank-a-client","clientSecret":"bank-a-secret"}' \
     | jq -r '.accessToken')
   ```

3. **Saldo de BRL disponível** no Spoke-A (banco comercial):
   ```bash
   # Verificar saldo via API ou tryout
   # Mínimo: 1050 BRL (1000 para swap + 50 slippage buffer)
   ```

4. **Beneficiário registrado** no Spoke-B (ex: bank-d):
   ```bash
   export BENEFICIARY_BANK_ID="bank-d"
   ```

---

## Step-by-Step E2E Flow

### Step 1: Obter Quote (15s de validade)

```bash
# Request quote para 2000 ARS (output desejado)
QUOTE_RESPONSE=$(curl -sf "$BANK_A_URL/api/v2/amm/quote/exact-output?pool_pair=W-BRL-ARS&amount_out=2000000000000000000000&max_slippage_pct=0.01" \
  -H "Authorization: Bearer $TOKEN_BANK_A")

echo "$QUOTE_RESPONSE" | jq .

# Parse quote details
QUOTE_ID=$(echo "$QUOTE_RESPONSE" | jq -r '.quote_id')
AMOUNT_IN=$(echo "$QUOTE_RESPONSE" | jq -r '.amount_in')
TIME_REMAINING=$(echo "$QUOTE_RESPONSE" | jq -r '.time_remaining_seconds')

echo "Quote ID: $QUOTE_ID"
echo "Amount In Required: $AMOUNT_IN wei (~1020 BRL with 0.3% fee)"
echo "Time Remaining: $TIME_REMAINING seconds"
```

**Expected Response**:
```json
{
  "quote_id": "550e8400-e29b-41d4-a716-446655440000",
  "pool_pair": "W-BRL-ARS",
  "amount_out": "2000000000000000000000",
  "amount_in": "1020600000000000000000",
  "effective_rate": 1.9598,
  "fee_bps": 30,
  "max_slippage_pct": 0.01,
  "reserve_a_snapshot": "100000000000000000000000",
  "reserve_b_snapshot": "200000000000000000000000",
  "created_at": "2026-05-26T15:00:00Z",
  "valid_until": "2026-05-26T15:00:15Z",
  "time_remaining_seconds": 15
}
```

**Success Criteria**: `time_remaining_seconds` deve ser exatamente 15 na criação.

---

### Step 2: Executar Swap Cross-Currency (dentro de 15s!)

```bash
# ATENÇÃO: Execute dentro de 15s após obter quote, ou ela expirará!

SWAP_REQUEST=$(jq -nc \
  --arg amount_out "2000000000000000000000" \
  --arg max_amount_in "1050000000000000000000" \
  --arg quote_id "$QUOTE_ID" \
  '{
    "source_currency": "BRL",
    "target_currency": "ARS",
    "amount_out": $amount_out,
    "max_amount_in": $max_amount_in,
    "payer_bank_id": "bank-a",
    "beneficiary_bank_id": "bank-d",
    "quote_id": $quote_id
  }')

SWAP_RESPONSE=$(curl -sf -X POST "$BANK_A_URL/api/v2/amm/swap/cross-currency" \
  -H "Authorization: Bearer $TOKEN_BANK_A" \
  -H "Content-Type: application/json" \
  -d "$SWAP_REQUEST")

echo "$SWAP_RESPONSE" | jq .

# Parse swap tracking IDs
SWAP_ID=$(echo "$SWAP_RESPONSE" | jq -r '.swap_id')
CORRELATION_ID=$(echo "$SWAP_RESPONSE" | jq -r '.correlation_id')
BRIDGE_IN_POS=$(echo "$SWAP_RESPONSE" | jq -r '.bridge_in_position_id')

echo "Swap ID: $SWAP_ID"
echo "Correlation ID: $CORRELATION_ID"
echo "Bridge-In Position: $BRIDGE_IN_POS"
```

**Expected Response** (HTTP 200):
```json
{
  "swap_id": "7a3d2f1e-8b4c-4d5e-9f6a-1b2c3d4e5f67",
  "correlation_id": "9c8e7d6f-5a4b-3c2d-1e0f-a9b8c7d6e5f4",
  "status": "BRIDGE_IN_PROGRESS",
  "amount_in": "1020600000000000000000",
  "amount_out": "2000000000000000000000",
  "effective_rate": 1.9598,
  "bridge_in_position_id": "b1e2c3d4-5f6a-7b8c-9d0e-1f2a3b4c5d6e",
  "swap_tx_hash": null,
  "bridge_out_position_id": null,
  "created_at": "2026-05-26T15:00:05Z"
}
```

**Success Criteria**: `status` deve ser `BRIDGE_IN_PROGRESS` inicialmente.

---

### Step 3: Monitor Bridge-In até ACTIVE (polling 5s, timeout 120s)

```bash
echo "Monitoring bridge-in position $BRIDGE_IN_POS (expect ~30s)..."

for i in {1..24}; do  # 24 iterations × 5s = 120s timeout
  BRIDGE_IN_STATE=$(curl -sf "$BANK_A_URL/api/v2/bridge/positions?state=ACTIVE" \
    -H "Authorization: Bearer $TOKEN_BANK_A" \
    | jq -r --arg pos "$BRIDGE_IN_POS" '.positions[] | select(.position_id==$pos) | .bridge_state')
  
  if [[ "$BRIDGE_IN_STATE" == "ACTIVE" ]]; then
    echo "✓ Bridge-in ACTIVE after $((i * 5))s"
    break
  fi
  
  echo "  [$i/24] Bridge-in state: ${BRIDGE_IN_STATE:-PENDING}... (waiting 5s)"
  sleep 5
done

if [[ "$BRIDGE_IN_STATE" != "ACTIVE" ]]; then
  echo "✗ Bridge-in timeout after 120s — check Cacti Relayer logs"
  exit 1
fi
```

**Expected Behavior**: Bridge-in atinge `ACTIVE` em ~20-30s (depende de Cacti Relayer).

---

### Step 4: Monitor Swap até BRIDGE_OUT_PROGRESS (polling 3s, timeout 60s)

```bash
echo "Monitoring swap operation $SWAP_ID (expect ~10s for on-chain confirmation)..."

for i in {1..20}; do  # 20 iterations × 3s = 60s timeout
  SWAP_STATUS_RESPONSE=$(curl -sf "$BANK_A_URL/api/v2/amm/swap/cross-currency/$SWAP_ID" \
    -H "Authorization: Bearer $TOKEN_BANK_A")
  
  SWAP_STATUS=$(echo "$SWAP_STATUS_RESPONSE" | jq -r '.status')
  SWAP_TX_HASH=$(echo "$SWAP_STATUS_RESPONSE" | jq -r '.swap_tx_hash // "null"')
  
  if [[ "$SWAP_STATUS" == "BRIDGE_OUT_PROGRESS" ]]; then
    echo "✓ Swap confirmed on-chain after $((i * 3))s"
    echo "  Tx Hash: $SWAP_TX_HASH"
    BRIDGE_OUT_POS=$(echo "$SWAP_STATUS_RESPONSE" | jq -r '.bridge_out_position_id')
    echo "  Bridge-out position: $BRIDGE_OUT_POS"
    break
  fi
  
  echo "  [$i/20] Swap status: $SWAP_STATUS... (waiting 3s)"
  
  if [[ "$SWAP_STATUS" == "FAILED" ]]; then
    FAILURE_REASON=$(echo "$SWAP_STATUS_RESPONSE" | jq -r '.failure_reason')
    echo "✗ Swap FAILED: $FAILURE_REASON"
    exit 1
  fi
  
  sleep 3
done

if [[ "$SWAP_STATUS" != "BRIDGE_OUT_PROGRESS" ]]; then
  echo "✗ Swap timeout after 60s — status stuck at $SWAP_STATUS"
  exit 1
fi

# SC-003 latency warning
if [[ $i -gt 10 ]]; then
  echo "⚠ WARNING: Swap took $((i * 3))s > 30s target (SC-003)"
fi
```

**Expected Behavior**: Swap transiciona para `BRIDGE_OUT_PROGRESS` em ~5-15s (confirmação on-chain no Hub).

---

### Step 5: Monitor Bridge-Out até UNLOCKED (polling 5s, timeout 120s)

```bash
echo "Monitoring bridge-out position $BRIDGE_OUT_POS (expect ~30s)..."

for i in {1..24}; do  # 24 iterations × 5s = 120s timeout
  BRIDGE_OUT_STATE=$(curl -sf "$BANK_A_URL/api/v2/bridge/positions" \
    -H "Authorization: Bearer $TOKEN_BANK_A" \
    | jq -r --arg pos "$BRIDGE_OUT_POS" '.positions[] | select(.position_id==$pos) | .bridge_state')
  
  if [[ "$BRIDGE_OUT_STATE" == "UNLOCKED" ]]; then
    echo "✓ Bridge-out UNLOCKED after $((i * 5))s"
    break
  fi
  
  echo "  [$i/24] Bridge-out state: ${BRIDGE_OUT_STATE:-PENDING}... (waiting 5s)"
  sleep 5
done

if [[ "$BRIDGE_OUT_STATE" != "UNLOCKED" ]]; then
  echo "✗ Bridge-out timeout after 120s — check Relayer logs for Spoke-B"
  exit 1
fi
```

**Expected Behavior**: Bridge-out atinge `UNLOCKED` em ~20-30s (Relayer processa burn no Hub e unlock no Spoke-B).

---

### Step 6: Verificar Status Final COMPLETED

```bash
FINAL_STATUS=$(curl -sf "$BANK_A_URL/api/v2/amm/swap/cross-currency/$SWAP_ID" \
  -H "Authorization: Bearer $TOKEN_BANK_A")

echo "$FINAL_STATUS" | jq .

STATUS=$(echo "$FINAL_STATUS" | jq -r '.status')
DURATION=$(echo "$FINAL_STATUS" | jq -r '.duration_seconds')

if [[ "$STATUS" == "COMPLETED" ]]; then
  echo "✓ Swap COMPLETED in ${DURATION}s"
  
  # SC-002 validation
  if [[ "$DURATION" -le 90 ]]; then
    echo "✓ SC-002 PASS: Latency ${DURATION}s ≤ 90s target (p95)"
  else
    echo "✗ SC-002 FAIL: Latency ${DURATION}s > 90s target"
  fi
  
  # SC-001 validation (tx hashes verificáveis)
  SWAP_TX=$(echo "$FINAL_STATUS" | jq -r '.swap_tx_hash')
  echo "✓ SC-001: Swap tx hash: $SWAP_TX"
  echo "  Verify on Hub explorer: http://localhost:8645/tx/$SWAP_TX"
else
  echo "✗ Final status is $STATUS (expected COMPLETED)"
  exit 1
fi
```

**Expected Final Response**:
```json
{
  "swap_id": "7a3d2f1e-8b4c-4d5e-9f6a-1b2c3d4e5f67",
  "correlation_id": "9c8e7d6f-5a4b-3c2d-1e0f-a9b8c7d6e5f4",
  "status": "COMPLETED",
  "amount_in": "1020600000000000000000",
  "amount_out": "2000000000000000000000",
  "effective_rate": 1.9598,
  "bridge_in_position_id": "b1e2c3d4-5f6a-7b8c-9d0e-1f2a3b4c5d6e",
  "bridge_in_status": "BURNED",
  "swap_tx_hash": "0xabc123...",
  "swap_confirmations": 12,
  "bridge_out_position_id": "f7e8d9c0-b1a2-3c4d-5e6f-7a8b9c0d1e2f",
  "bridge_out_status": "UNLOCKED",
  "created_at": "2026-05-26T15:00:05Z",
  "completed_at": "2026-05-26T15:01:15Z",
  "duration_seconds": 70
}
```

**Success Criteria**:
- ✅ `status` = `COMPLETED`
- ✅ `duration_seconds` ≤ 90 (SC-002 p95 target)
- ✅ `swap_tx_hash` presente e verificável on-chain (SC-001)
- ✅ `bridge_out_status` = `UNLOCKED` (ARS desbloqueado no Spoke-B para beneficiário)

---

## Error Scenarios Testing

### Test 1: Quote Expired (aguardar >15s antes de executar swap)

```bash
# Step 1: Obter quote
QUOTE_ID=$(curl -sf "$BANK_A_URL/api/v2/amm/quote/exact-output?pool_pair=W-BRL-ARS&amount_out=2000000000000000000000" \
  -H "Authorization: Bearer $TOKEN_BANK_A" | jq -r '.quote_id')

# Step 2: Aguardar 20 segundos (expire quote propositalmente)
echo "Waiting 20s to expire quote..."
sleep 20

# Step 3: Tentar executar swap com quote expirada
SWAP_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BANK_A_URL/api/v2/amm/swap/cross-currency" \
  -H "Authorization: Bearer $TOKEN_BANK_A" \
  -H "Content-Type: application/json" \
  -d "{\"source_currency\":\"BRL\",\"target_currency\":\"ARS\",\"amount_out\":\"2000000000000000000000\",\"max_amount_in\":\"1050000000000000000000\",\"payer_bank_id\":\"bank-a\",\"beneficiary_bank_id\":\"bank-d\",\"quote_id\":\"$QUOTE_ID\"}")

HTTP_CODE=$(echo "$SWAP_RESPONSE" | tail -1)
BODY=$(echo "$SWAP_RESPONSE" | sed '$d')

if [[ "$HTTP_CODE" == "422" ]] && echo "$BODY" | jq -e '.error_code == "QUOTE_EXPIRED"' &>/dev/null; then
  echo "✓ SC-005 PASS: Quote expired correctly detected"
  echo "$BODY" | jq .
else
  echo "✗ Expected HTTP 422 QUOTE_EXPIRED, got $HTTP_CODE"
  exit 1
fi
```

**Expected Error**:
```json
{
  "error": "quote ... expired at ...",
  "error_code": "QUOTE_EXPIRED",
  "recommended_action": "Obtain new quote via GET /api/v2/amm/quote/exact-output"
}
```

---

### Test 2: Slippage Limit Exceeded (max_amount_in muito baixo)

```bash
# Obter quote
AMOUNT_IN=$(curl -sf "$BANK_A_URL/api/v2/amm/quote/exact-output?pool_pair=W-BRL-ARS&amount_out=2000000000000000000000" \
  -H "Authorization: Bearer $TOKEN_BANK_A" | jq -r '.amount_in')

# Executar com max_amount_in artificialmente baixo (abaixo do amount_in requerido)
MAX_TOO_LOW=$((AMOUNT_IN - 50000000000000000000))  # 50 BRL abaixo do necessário

SWAP_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BANK_A_URL/api/v2/amm/swap/cross-currency" \
  -H "Authorization: Bearer $TOKEN_BANK_A" \
  -H "Content-Type: application/json" \
  -d "{\"source_currency\":\"BRL\",\"target_currency\":\"ARS\",\"amount_out\":\"2000000000000000000000\",\"max_amount_in\":\"$MAX_TOO_LOW\",\"payer_bank_id\":\"bank-a\",\"beneficiary_bank_id\":\"bank-d\"}")

HTTP_CODE=$(echo "$SWAP_RESPONSE" | tail -1)
BODY=$(echo "$SWAP_RESPONSE" | sed '$d')

if [[ "$HTTP_CODE" == "422" ]] && echo "$BODY" | jq -e '.error_code == "SLIPPAGE_LIMIT_EXCEEDED"' &>/dev/null; then
  echo "✓ SC-004 PASS: Slippage limit correctly enforced"
  echo "$BODY" | jq .
else
  echo "✗ Expected HTTP 422 SLIPPAGE_LIMIT_EXCEEDED, got $HTTP_CODE"
fi
```

---

### Test 3: Pool Not Active

```bash
# Pré-requisito: parar o pool via circuit breaker ou remover liquidez CBs

SWAP_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BANK_A_URL/api/v2/amm/swap/cross-currency" \
  -H "Authorization: Bearer $TOKEN_BANK_A" \
  -H "Content-Type: application/json" \
  -d '{"source_currency":"BRL","target_currency":"ARS","amount_out":"2000000000000000000000","max_amount_in":"1050000000000000000000","payer_bank_id":"bank-a","beneficiary_bank_id":"bank-d"}')

HTTP_CODE=$(echo "$SWAP_RESPONSE" | tail -1)
BODY=$(echo "$SWAP_RESPONSE" | sed '$d')

if [[ "$HTTP_CODE" == "422" ]] && echo "$BODY" | jq -e '.error_code == "POOL_NOT_ACTIVE"' &>/dev/null; then
  echo "✓ SC-004 PASS: Pool not active correctly blocked"
  echo "$BODY" | jq .
else
  echo "Note: Pool is ACTIVE — test requires pool to be EMPTY or PENDING_COUNTERPART"
fi
```

---

## Automated Tryout Script

Para automatizar os testes acima:

```bash
bash tryouts/tryout-commercial-swap-e2e.sh
```

Expected output:
```
✓ Pool W-BRL-ARS is ACTIVE
✓ Bank-a authenticated
✓ Quote obtained (quote_id=..., time_remaining=15s)
✓ Swap initiated (swap_id=..., correlation_id=...)
✓ Bridge-in ACTIVE after 25s
✓ Swap confirmed after 8s (tx_hash=0x...)
✓ Bridge-out UNLOCKED after 28s
✓ Swap COMPLETED in 61s
✓ SC-001 PASS: Tx hash verified on-chain
✓ SC-002 PASS: Latency 61s ≤ 90s
✓ SC-005 PASS: Quote expiry test passed
✓ SC-004 PASS: Error messages user-friendly
```

---

## Troubleshooting

### Issue: "Quote expired" error mesmo após 5s

**Cause**: Clock skew entre cliente e servidor  
**Solution**: Sincronizar relógio via NTP (`sudo ntpdate pool.ntp.org`)

### Issue: Bridge-in timeout após 120s

**Cause**: Cacti Relayer offline ou spoke desconectado do Hub  
**Debug**:
```bash
docker logs cacti-relayer-spoke-a | tail -50
docker logs cbweb3-spoke-a-besu | tail -50
```

### Issue: Swap tx reverts on-chain

**Cause**: Slippage on-chain maior que esperado (high volatility)  
**Solution**: Aumentar `max_slippage_pct` de 0.01 (1%) para 0.02 (2%) na quote

### Issue: Bridge-out stuck em BURNING

**Cause**: Relayer Hub→Spoke-B offline  
**Debug**:
```bash
docker logs cacti-relayer-spoke-b | grep "burn detected"
```

---

## Next Steps

Com quickstart validado manualmente, prosseguir para:
- **tasks.md**: Decompor implementação em tarefas executáveis (via `/speckit.tasks`)
- **Automated tryout**: Converter steps acima em `tryouts/tryout-commercial-swap-e2e.sh`
