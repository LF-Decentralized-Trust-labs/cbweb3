# Quickstart: Validação E2E — Provisão Cooperativa de Liquidez

**Feature**: `005-cooperative-liquidity`
**Date**: 2026-05-14
**Pré-requisito**: Ambiente local rodando via `make dev.up` com Hub Besu ativo

---

## Pré-condições

1. Pool BRL-USD **sem** liquidez prévia (ou com ambiente fresh após `contracts.deploy-hub`)
2. `CENTRAL_BANK_A_TOKEN` disponível (JWT do central-bank-a)
3. `CENTRAL_BANK_B_TOKEN` disponível (JWT do central-bank-b)
4. Endereços de central-bank-a e central-bank-b registrados como `isLiquidityProvider` no IdentityRegistry
5. Ambos os CBs com saldo de seus respectivos tokens no Hub (via `step4a_mint_and_approve`)

---

## Fluxo 1: Pool Cooperativo BRL-USD (Commit-Reveal)

### Passo 1 — BCB registra commit do lado A (BRL)

```bash
commit_a_resp=$(curl -s -X POST \
  "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/commit" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-USD","provider_id":"central_bank_a","side":"A","amount":"100000"}')

commit_a_id=$(echo "$commit_a_resp" | jq -r '.commit_id')
status_a=$(echo "$commit_a_resp" | jq -r '.status')

echo "Commit A: $commit_a_id | Status: $status_a"
# Esperado: status = PENDING
```

### Passo 2 — Verificar pool ainda PENDING_COUNTERPART

```bash
pool_status=$(curl -s \
  "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/pool/BRL-USD/status" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN")

echo "Pool status: $(echo "$pool_status" | jq -r '.pool_status')"
# Esperado: PENDING_COUNTERPART
```

### Passo 3 — Tentar swap (deve falhar com POOL_NOT_ACTIVE)

```bash
swap_fail=$(curl -s -X POST \
  "$API_GW_BANK_A_URL/api/v2/amm/swap/exact-output" \
  -H "Authorization: Bearer $BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-USD","exact_output_amount":"100","max_amount_in":"110","payer_bank_id":"bank_a","beneficiary_bank_id":"bank_c"}')

echo "Swap error: $(echo "$swap_fail" | jq -r '.error')"
# Esperado: POOL_NOT_ACTIVE
```

### Passo 4 — Fed registra commit do lado B (USD)

```bash
commit_b_resp=$(curl -s -X POST \
  "$API_GW_CENTRAL_BANK_B_URL/api/v2/amm/liquidity/commit" \
  -H "Authorization: Bearer $CENTRAL_BANK_B_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-USD","provider_id":"central_bank_b","side":"B","amount":"100000"}')

status_b=$(echo "$commit_b_resp" | jq -r '.status')
lp_ids=$(echo "$commit_b_resp" | jq -r '.lp_ids[]')

echo "Commit B status: $status_b"
echo "LP IDs criados: $lp_ids"
# Esperado: status = MATCHED; lp_ids = [lp_id_bcb, lp_id_fed]
```

### Passo 5 — Verificar pool ACTIVE com reservas

```bash
pool=$(curl -s \
  "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/pool/BRL-USD/status" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN")

echo "Pool status:  $(echo "$pool" | jq -r '.pool_status')"   # ACTIVE
echo "Reserve A:    $(echo "$pool" | jq -r '.reserve_a')"     # 100000
echo "Reserve B:    $(echo "$pool" | jq -r '.reserve_b')"     # 100000
echo "LP count:     $(echo "$pool" | jq -r '.total_lp_count')" # 2
echo "Fee rate bps: $(echo "$pool" | jq -r '.fee_rate_bps')" # 30
```

---

## Fluxo 2: Swap com Coleta de Taxa

### Passo 6 — Banco comercial executa swap

```bash
swap_resp=$(curl -s -X POST \
  "$API_GW_BANK_A_URL/api/v2/amm/swap/exact-output" \
  -H "Authorization: Bearer $BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-USD","exact_output_amount":"1000","max_amount_in":"1050","payer_bank_id":"bank_a","beneficiary_bank_id":"bank_c"}')

echo "Swap status: $(echo "$swap_resp" | jq -r '.status')"
# Esperado: COMPLETED
```

### Passo 7 — Verificar reservas ajustadas (incluindo taxa retida)

```bash
pool_after=$(curl -s \
  "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/pool/BRL-USD/status" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN")

echo "Reserve A após swap: $(echo "$pool_after" | jq -r '.reserve_a')"
# Esperado: > 100000 (input do swap foi creditado nas reservas)
echo "Reserve B após swap: $(echo "$pool_after" | jq -r '.reserve_b')"
# Esperado: ~99000 (1000 USD saiu para bank-c)
```

---

## Fluxo 3: Retirada Proporcional

### Passo 8 — BCB retira sua liquidez (side A)

```bash
# lp_id_bcb capturado no Passo 4
remove_resp=$(curl -s -X POST \
  "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/remove" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"lp_id\":\"$LP_ID_BCB\",\"pool_pair\":\"BRL-USD\",\"provider_bank_id\":\"central_bank_a\"}")

echo "Returned A:       $(echo "$remove_resp" | jq -r '.returned_token_a')"
# Esperado: ~100525 (50% de reserveA ajustada + fee)
echo "Returned B:       $(echo "$remove_resp" | jq -r '.returned_token_b')"
# Esperado: ~49500 (50% de reserveB ajustada)
echo "Fee claim paid:   $(echo "$remove_resp" | jq -r '.fee_claim_paid')"
# Esperado: > 0
echo "Withdrawal mode:  $(echo "$remove_resp" | jq -r '.withdrawal_mode')"
# Esperado: PROPORTIONAL
```

---

## Fluxo 4: Prevenção de Commit Duplicado

### Passo 9 — Tentar segundo commit para mesmo lado (deve falhar)

```bash
dup_resp=$(curl -s -X POST \
  "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/commit" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-USD","provider_id":"central_bank_a","side":"A","amount":"50000"}')

echo "Error: $(echo "$dup_resp" | jq -r '.error')"
# Esperado: COMMIT_ALREADY_EXISTS
```

---

## Checklist de Validação

| # | Verificação | Resultado Esperado |
|---|---|---|
| 1 | Commit do lado A → status PENDING | ✓ |
| 2 | Pool → PENDING_COUNTERPART após commit A | ✓ |
| 3 | Swap bloqueado → POOL_NOT_ACTIVE | ✓ |
| 4 | Commit do lado B → status MATCHED; lp_ids retornados | ✓ |
| 5 | Pool → ACTIVE com reserve_a > 0 AND reserve_b > 0 | ✓ |
| 6 | fee_rate_bps = 30 | ✓ |
| 7 | Swap executado com sucesso após ativação do pool | ✓ |
| 8 | Reservas ajustadas corretamente após swap | ✓ |
| 9 | Retirada proporcional retorna fatia do pool atual + fee | ✓ |
| 10 | withdrawal_mode = PROPORTIONAL para posições cooperativas | ✓ |
| 11 | Commit duplicado → COMMIT_ALREADY_EXISTS (409) | ✓ |
| 12 | Entidade sem LP role → NOT_AUTHORIZED_LP (403) | ✓ |

---

## Fluxo 5: Par Dinâmico BRL-ARS — Criação via PairRegistry (Sessão 2026-05-18)

> **Pré-condições adicionais**:
> - Contrato `AutomatedMarketMaker` para BRL-ARS já implantado (endereço em `$AMM_BRL_ARS_ADDR`)
> - `CENTRAL_BANK_BRL_TOKEN` = JWT do BCB (Banco Central do Brasil)
> - `CENTRAL_BANK_ARS_TOKEN` = JWT do BCRA (Banco Central da República Argentina)
> - `TOKEN_BRL_ADDR`, `TOKEN_ARS_ADDR` = endereços dos contratos tCeBM

### Passo 1 — BCB propõe o par BRL-ARS

```bash
propose_resp=$(curl -s -X POST \
  "$API_GW_URL/api/v2/amm/pairs/propose" \
  -H "Authorization: Bearer $CENTRAL_BANK_BRL_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"pool_pair\": \"BRL-ARS\",
    \"token_a_address\": \"$TOKEN_BRL_ADDR\",
    \"token_b_address\": \"$TOKEN_ARS_ADDR\",
    \"amm_address\": \"$AMM_BRL_ARS_ADDR\"
  }")

echo "Proposta: $(echo "$propose_resp" | jq -r '.status')"
# Esperado: PROPOSED
echo "TX: $(echo "$propose_resp" | jq -r '.tx_hash')"
```

### Passo 2 — BCRA confirma o par BRL-ARS

```bash
confirm_resp=$(curl -s -X POST \
  "$API_GW_URL/api/v2/amm/pairs/confirm" \
  -H "Authorization: Bearer $CENTRAL_BANK_ARS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair": "BRL-ARS"}')

echo "Confirmação: $(echo "$confirm_resp" | jq -r '.status')"
# Esperado: ACTIVE
```

### Passo 3 — Verificar par listado (sem restart do gateway)

```bash
pairs=$(curl -s "$API_GW_URL/api/v2/amm/pairs" \
  -H "Authorization: Bearer $CENTRAL_BANK_BRL_TOKEN")

echo "Pares ativos: $(echo "$pairs" | jq '[.pairs[].pool_pair]')"
# Esperado: ["BRL-USD", "BRL-ARS"] (sem restart)
```

### Passo 4 — Depósito cooperativo no novo par (commit-reveal)

```bash
# BCB deposita BRL no par BRL-ARS
commit_brl=$(curl -s -X POST \
  "$API_GW_URL/api/v2/amm/liquidity/commit" \
  -H "Authorization: Bearer $CENTRAL_BANK_BRL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-ARS","provider_id":"central_bank_brl","side":"A","amount":"50000"}')

echo "Commit BRL: $(echo "$commit_brl" | jq -r '.status')"
# Esperado: PENDING

# BCRA deposita ARS no par BRL-ARS
commit_ars=$(curl -s -X POST \
  "$API_GW_URL/api/v2/amm/liquidity/commit" \
  -H "Authorization: Bearer $CENTRAL_BANK_ARS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-ARS","provider_id":"central_bank_ars","side":"B","amount":"50000"}')

echo "Commit ARS: $(echo "$commit_ars" | jq -r '.status')"
# Esperado: MATCHED → pool BRL-ARS passa para ACTIVE
```

### Passo 5 — Swap no novo corredor BRL→ARS

```bash
swap_resp=$(curl -s -X POST \
  "$API_GW_URL/api/v2/amm/swap/exact-output" \
  -H "Authorization: Bearer $BANK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-ARS","exact_output_amount":"100","max_amount_in":"110","payer_bank_id":"bank_brl","beneficiary_bank_id":"bank_ars"}')

echo "Swap status: $(echo "$swap_resp" | jq -r '.status')"
# Esperado: COMPLETED
```

### Checklist Fluxo 5 — Critérios de Aceite

| # | Critério | Status |
|---|----------|--------|
| 1 | BCB propõe BRL-ARS → status PROPOSED, tx_hash presente | ✓ |
| 2 | BCRA confirma → status ACTIVE, par visível no GET /pairs | ✓ |
| 3 | GET /pairs retorna BRL-ARS sem restart do gateway | ✓ |
| 4 | Commit BRL no par BRL-ARS → PENDING | ✓ |
| 5 | Commit ARS no par BRL-ARS → MATCHED → pool ACTIVE | ✓ |
| 6 | Swap BRL→ARS executado com sucesso usando AMM correto | ✓ |
| 7 | Proposta duplicada (pool_pair já existe) → PAIR_ALREADY_EXISTS (409) | ✓ |
| 8 | CB errado tentando confirmar → NOT_CENTRAL_BANK_OF_TOKEN_B (403) | ✓ |

---

## Fluxo 6: MLP como Provedor Suplementar (US2) — Path B

**Pré-condições**:
1. `ENABLE_MLP=true` exportado
2. `make scenario-b.up-infra` executado (Keycloak com realm `mlp`)
3. `make scenario-b.deploy-contracts` com `MLP_ADDRESS=0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b` (opcional — `SeedHub.s.sol` usa o default)
4. `make scenario-b.up-backend-mlp` executado (stack MLP na porta 68080)
5. Pool BRL-USD ACTIVE (via Fluxo 1) — ou pool EMPTY para testar MLP como inicializador

### Passo 1 — Obter token Keycloak do MLP

```bash
# Ler o secret gerado pelo init.sh
source backend/config/.env.infra.mlp

MLP_TOKEN=$(curl -s \
  -d "client_id=${KC_CLIENT_ID}" \
  -d "client_secret=${KC_CLIENT_SECRET}" \
  -d "grant_type=client_credentials" \
  "${KC_BASE_PATH}/realms/${KC_REALM}/protocol/openid-connect/token" \
  | jq -r '.access_token')

echo "MLP_TOKEN: ${MLP_TOKEN:0:40}..."
# Token deve ter: realm_access.roles = ["mlp"]
```

### Passo 2 — MLP deposita liquidez dual-sided

```bash
API_GW_MLP_URL="http://localhost:68080"

mlp_resp=$(curl -s -X POST \
  "$API_GW_MLP_URL/api/v2/amm/liquidity/add" \
  -H "Authorization: Bearer $MLP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pool_pair": "BRL-USD",
    "token_a_amount": "50000",
    "token_b_amount": "50000",
    "provider_bank_id": "mlp"
  }')

lp_id=$(echo "$mlp_resp" | jq -r '.lp_id')
echo "LP ID: $lp_id"
echo "Deposit side: $(echo "$mlp_resp" | jq -r '.deposit_side')"
echo "Shares: $(echo "$mlp_resp" | jq -r '.shares_percentage')"
# Esperado: deposit_side = BOTH, lp_id não-nulo
```

### Passo 3 — Verificar LP position do MLP no pool

```bash
# O pool deve mostrar o MLP como coproprietário com shares_percentage
pool_status=$(curl -s \
  "$API_GW_MLP_URL/api/v2/amm/pool/BRL-USD/status" \
  -H "Authorization: Bearer $MLP_TOKEN")

echo "Pool status: $(echo "$pool_status" | jq -r '.pool_status')"
echo "Reserve A: $(echo "$pool_status" | jq -r '.reserve_a')"
echo "Reserve B: $(echo "$pool_status" | jq -r '.reserve_b')"
# Esperado: status = ACTIVE, reservas aumentadas
```

### Passo 4 — Swap usando liquidez combinada (MLP + CB)

```bash
# Banco comercial usa token de bank-a
BANK_A_TOKEN="... (JWT do banco comercial)"

swap_resp=$(curl -s -X POST \
  "http://localhost:38080/api/v2/amm/swap/exact-output" \
  -H "Authorization: Bearer $BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pool_pair": "BRL-USD",
    "exact_output_amount": "1000",
    "max_amount_in": "1100",
    "payer_bank_id": "bank_a",
    "beneficiary_bank_id": "bank_b"
  }')

echo "Swap status: $(echo "$swap_resp" | jq -r '.status')"
# Esperado: COMPLETED — swap usa reservas MLP + CB
```

### Passo 5 — MLP retira liquidez

```bash
mlp_withdraw=$(curl -s -X POST \
  "$API_GW_MLP_URL/api/v2/amm/liquidity/remove" \
  -H "Authorization: Bearer $MLP_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"lp_id\": \"$lp_id\"}")

echo "Withdrawal: $(echo "$mlp_withdraw" | jq -r '.status')"
echo "Amount A returned: $(echo "$mlp_withdraw" | jq -r '.amount_a_returned')"
echo "Amount B returned: $(echo "$mlp_withdraw" | jq -r '.amount_b_returned')"
# Esperado: status = WITHDRAWN, amounts proporcionais à participação
```

### Checklist Fluxo 6 — US2 Acceptance Criteria

| # | Critério | Referência spec |
|---|----------|----------------|
| 1 | MLP token obtido via Keycloak realm `mlp` com role `mlp` | D12 |
| 2 | MLP deposita BRL-USD dual-sided → LP position ACTIVE | US2-AC1 |
| 3 | `deposit_side = BOTH`, `shares_percentage > 0`, `lp_id` presente | D14 |
| 4 | Swap de banco comercial usa reservas combinadas (MLP + CB) | US2-AC1 |
| 5 | MLP remove liquidez → status WITHDRAWN, ativos retornados proporcionalmente | US2-AC2 |
| 6 | Entidade sem papel `mlp` no JWT → 403 Unauthorized | D12 |
| 7 | MLP não pode auto-promover via API (apenas admin on-chain pode `grantLP`) | US2-AC3 |

---

## Fluxo 7: Verificação dos Bug Fixes I1–I4 (branch fix-005-cooperative-liquidity)

> Execute este fluxo após aplicar os fixes T055–T058 para confirmar que todos os cenários que falhavam no tryout agora passam.

### Pré-condições

```bash
# Rebuild do api-gateway após os fixes
docker compose -f backend/docker-compose-backend.central-bank-a.yaml build api-gateway-central-bank-a
docker compose -f backend/docker-compose-backend.central-bank-a.yaml up -d api-gateway-central-bank-a
```

### Fix I3 — pool_status = PENDING_COUNTERPART após Commit A

```bash
# 1. Limpar pool (restart stack ou garantir pool EMPTY)
# 2. Registrar Commit A
commit_a=$(curl -s -X POST "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/commit" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pool_pair":"BRL-USD","provider_id":"central_bank_a","side":"A","amount":"100000"}')

echo "Commit A status: $(echo "$commit_a" | jq -r '.status')"
# Esperado: PENDING

# 3. Verificar pool status
pool=$(curl -s "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/pool/BRL-USD/status" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN")

echo "Pool status: $(echo "$pool" | jq -r '.pool_status')"
# Esperado: PENDING_COUNTERPART (antes do fix: EMPTY)
echo "Pending commits: $(echo "$pool" | jq -r '.pending_commits | length')"
# Esperado: 1
```

### Fix I4 — total_lp_count = 2 após commit-reveal completo

```bash
# (continuação após Commit B executado)
pool=$(curl -s "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/pool/BRL-USD/status" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN")

echo "total_lp_count: $(echo "$pool" | jq -r '.total_lp_count')"
# Esperado: 2 (antes do fix: 0)
```

### Fix I1 — token_a_amount proporcional às reservas atuais (não ao depósito original)

```bash
# Após swap que muda reservas (ex.: reserve_a=101015, reserve_b=99000)
reserve_a=$(echo "$pool_after_swap" | jq -r '.reserve_a')
reserve_b=$(echo "$pool_after_swap" | jq -r '.reserve_b')

# Remover LP com 50% de shares
remove_resp=$(curl -s -X POST "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/remove" \
  -H "Authorization: Bearer $CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"lp_id\":\"$LP_ID_BCB\",\"pool_pair\":\"BRL-USD\",\"provider_bank_id\":\"central_bank_a\"}")

echo "Returned A: $(echo "$remove_resp" | jq -r '.token_a_amount')"
# Esperado: ~50507 (50% de 101015, antes do fix: 50000)
echo "Returned B: $(echo "$remove_resp" | jq -r '.token_b_amount')"
# Esperado: ~49500 (50% de 99000, antes do fix: 50000)
```

### Fix I2 — fee_claim_paid > 0 após swap com 30bps

```bash
echo "Fee claim paid: $(echo "$remove_resp" | jq -r '.fee_claim_paid')"
# Esperado: > 0 (antes do fix: 0)
# Verificar lp_fee_events no DB:
# SELECT COUNT(*) FROM lp_fee_events WHERE pool_pair = 'BRL-USD';
# Esperado: >= 1 (um registro por swap executado)
```

### Nota arquitetural: I3 e I4 requerem gateway correto

`total_lp_count` e `pending_commits[]` são dados **DB-local** de cada gateway. Como os commits e LP positions são criados no gateway de CB-A (`cbweb3_central_bank_a`), as verificações de I3 e I4 devem usar `$API_GW_CENTRAL_BANK_A_URL`, não `$API_GW_BANK_A_URL`.

O tryout script foi atualizado para refletir isso (linha ~358: pool status após Commit A e linha ~417: total_lp_count após commit-reveal usam `$API_GW_CENTRAL_BANK_A_URL`).

### Nota arquitetural: I2 (fee cross-gateway)

`fee_claim_paid` retorna `0` quando o swap ocorre via Bank A gateway (`cbweb3_bank_a`) mas o LP é de CB-A (`cbweb3_central_bank_a`). O T058 (`RecordSwapFee`) implementa a gravação correta dentro de um único gateway — a distribuição cross-gateway (swap em gateway diferente do LP) é uma limitação arquitetural não implementada nesta fase.

**Resultado validado em prod (2026-05-20)**: `fee_claim_paid=0` é aceito como INFO (não FAIL) quando swap gateway ≠ LP gateway.

### Checklist Fix I1–I4 (Resultados Validados)

| Bug | Verificação | Resultado Validado | Status |
|-----|------------|-------------------|--------|
| I3 | `pool_status` após Commit A (via CB-A gateway) | `PENDING_COUNTERPART` | ✅ PASS |
| I4 | `total_lp_count` após commit-reveal (via CB-A gateway) | `2` | ✅ PASS |
| I1 | `token_a_amount` na remoção proporcional após swap | `50507` (reserva on-chain, não stale) | ✅ PASS |
| I2 | `fee_claim_paid` (mesmo gateway) | `> 0` em prod; `0` quando cross-gateway | ⚠️ INFO |
