# Quickstart: Fluxo Soberano de Liquidez via Bridge (spec-007)

**Feature**: `007-bridge-based-cb-liquidity`  
**Branch**: `007-bridge-based-cb-liquidity`  
**Pré-requisitos**: Ambiente local funcional (`make spoke-all` OK, spokes A e B up, Hub up, Bridge Relayer up)

---

## 1. Deploy dos Contratos Soberanos (uma vez por ambiente)

```bash
# Deploy W-tCeBM_BRL, W-tCeBM_ARS, LiquidityCommitRegistry, AMM soberano
# e ativação do par via PairRegistry
# O script é parametrizado: ajuste PAIR_ID, TOKEN_SYMBOL_A/B e CB keys para novos pares
PAIR_ID=W-BRL-ARS TOKEN_SYMBOL_A=W-tCeBM_BRL TOKEN_SYMBOL_B=W-tCeBM_ARS \
  CB_A_HUB_PRIVATE_KEY=$CB_A_KEY CB_B_HUB_PRIVATE_KEY=$CB_B_KEY \
  make contracts.seed-sovereign-pair
```

Após o deploy, as variáveis de ambiente abaixo são populadas (ex: em `.env.sovereign`):
```
W_TOKEN_BRL_ADDRESS=0x...
W_TOKEN_ARS_ADDRESS=0x...
SOVEREIGN_PAIR_AMM_MAP='{"W-BRL-ARS":"0xAMM_ADDR"}'  # JSON map: pair_id → AMM address
LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0x...
SOVEREIGN_PAIR_IDS=W-BRL-ARS  # lista separada por vírgula para múltiplos pares
```

---

## 2. Fluxo CB-A: Lock → Bridge → Commit

### 2.1 CB-A minta BRL no Spoke-A

```bash
# CB-A minta tCeBM_BRL no seu próprio spoke (CENTRAL_BANK_ROLE local)
curl -X POST https://gateway-spoke-a/api/v2/amm/token/mint-and-approve \
  -H "Authorization: Bearer $CB_A_JWT" \
  -d '{
    "token_address": "'$SPOKE_A_BRL_TOKEN'",
    "amount": "100000000000000000000"
  }'
# Nota: sem campo "recipient" — minta para o próprio CB-A signer (soberano)
```

### 2.2 CB-A bloqueia BRL no Bridge do Spoke-A

```bash
curl -X POST https://gateway-spoke-a/api/v2/bridge/lock-mint \
  -H "Authorization: Bearer $CB_A_JWT" \
  -d '{
    "owner_bank_id": "cb-a",
    "spoke_network": "spoke-a",
    "native_asset": "'$SPOKE_A_BRL_TOKEN'",
    "mirrored_asset": "'$W_TOKEN_BRL_ADDRESS'",
    "amount": "100000000000000000000"
  }'
# Resposta: { "position_id": "uuid-xxx", "bridge_state": "LOCKING" }
```

### 2.3 CB-A aguarda confirmação do Bridge Relayer

```bash
# Polling até bridge_state = ACTIVE (timeout: 120s / intervalo: 5s)
POSITION_ID="uuid-xxx"
for i in $(seq 1 24); do
  STATE=$(curl -s https://gateway-spoke-a/api/v2/bridge/positions?owner_bank_id=cb-a \
    -H "Authorization: Bearer $CB_A_JWT" | \
    jq -r ".[] | select(.position_id == \"$POSITION_ID\") | .bridge_state")
  echo "[$i] bridge_state: $STATE"
  [ "$STATE" = "ACTIVE" ] && break
  sleep 5
done
```

### 2.4 CB-A registra commit no gateway (→ LiquidityCommitRegistry on-chain)

```bash
curl -X POST https://gateway-spoke-a/api/v2/amm/liquidity/commit \
  -H "Authorization: Bearer $CB_A_JWT" \
  -d '{
    "pool_pair": "W-BRL-ARS",
    "side": "A",
    "amount": "100000000000000000000",
    "provider_id": "cb-a"
  }'
# Resposta: { "commit_id": "local-uuid", "on_chain_commit_id": "0x...", "status": "PENDING" }
```

---

## 3. Fluxo CB-B: Lock → Bridge → Commit (gateway independente)

```bash
# 3.1 CB-B minta ARS no Spoke-B
curl -X POST https://gateway-spoke-b/api/v2/amm/token/mint-and-approve \
  -H "Authorization: Bearer $CB_B_JWT" \
  -d '{ "token_address": "'$SPOKE_B_ARS_TOKEN'", "amount": "100000000000000000000" }'

# 3.2 CB-B bloqueia ARS no Bridge do Spoke-B
curl -X POST https://gateway-spoke-b/api/v2/bridge/lock-mint \
  -H "Authorization: Bearer $CB_B_JWT" \
  -d '{
    "owner_bank_id": "cb-b",
    "spoke_network": "spoke-b",
    "native_asset": "'$SPOKE_B_ARS_TOKEN'",
    "mirrored_asset": "'$W_TOKEN_ARS_ADDRESS'",
    "amount": "100000000000000000000"
  }'

# 3.3 CB-B aguarda bridge_state = ACTIVE (mesmo padrão de polling de 2.3)

# 3.4 CB-B registra commit NO SEU PRÓPRIO GATEWAY (gateway-spoke-b, não gateway-spoke-a)
curl -X POST https://gateway-spoke-b/api/v2/amm/liquidity/commit \
  -H "Authorization: Bearer $CB_B_JWT" \
  -d '{ "pool_pair": "W-BRL-ARS", "side": "B", "amount": "100000000000000000000", "provider_id": "cb-b" }'
```

---

## 4. Match Automático via LiquidityCommitRegistry

Quando CB-B submete o commit (passo 3.4), o `LiquidityCommitRegistry` detecta que ambas as sides A e B estão `PENDING` para o par `W-BRL-ARS` e emite:

```
CommitMatched(
  poolPair: "W-BRL-ARS",
  commitIdA: 0x..., signerA: <CB_A_hub_signer>, amountA: 100e18,
  commitIdB: 0x..., signerB: <CB_B_hub_signer>, amountB: 100e18
)
```

O event watcher de cada gateway detecta o evento e notifica o gateway local via:
```
POST /internal/amm/execute-matched-commit
```

Cada gateway executa `addSingleSidedLiquidity(isTokenA, amount)` usando o signer local soberano:
- Gateway-A: `addSingleSidedLiquidity(true, 100e18)` com chave de CB-A (side A = W-BRL)
- Gateway-B: `addSingleSidedLiquidity(false, 100e18)` com chave de CB-B (side B = W-ARS)

---

## 5. Verificação On-Chain do msg.sender (SC-001)

```bash
# Verificar que o msg.sender de cada depósito é o signer soberano correto
cast logs --rpc-url $HUB_RPC_URL \
  --address $(echo $SOVEREIGN_PAIR_AMM_MAP | python3 -c "import sys,json; print(json.load(sys.stdin)['W-BRL-ARS'])") \
  "LogSingleSidedLiquidityAdded(address,bool,uint256,uint256)" \
  --from-block <deploy_block>

# Resultado esperado:
# { "provider": "<CB_A_hub_signer>", "isTokenA": true,  "amount": 100e18 }
# { "provider": "<CB_B_hub_signer>", "isTokenA": false, "amount": 100e18 }

# ZERO ocorrências de CB_A_signer com isTokenA=false ou CB_B_signer com isTokenA=true
```

---

## 6. Verificar Pool ACTIVE

```bash
# Via gateway de qualquer CB
curl https://gateway-spoke-a/api/v2/amm/pool/W-BRL-ARS/status \
  -H "Authorization: Bearer $CB_A_JWT"

# Resposta esperada:
# {
#   "pool_pair": "W-BRL-ARS",
#   "pool_status": "ACTIVE",
#   "reserve_a": "100000000000000000000",
#   "reserve_b": "100000000000000000000"
# }
```

---

## 7. Teste do Bloqueio Anti-G5-cross (SC-002)

```bash
# Tentativa de mint-and-approve com recipient sendo signer de outro CB
curl -X POST https://gateway-spoke-b/api/v2/amm/token/mint-and-approve \
  -H "Authorization: Bearer $CB_B_JWT" \
  -d '{
    "token_address": "'$SPOKE_B_ARS_TOKEN'",
    "amount": "1000",
    "recipient": "'$CB_A_HUB_SIGNER'"
  }'

# Resposta esperada: HTTP 403
# { "error": "recipient is a Central Bank signer — cross-CB minting is prohibited", "code": "CROSS_CB_MINT_PROHIBITED" }
```

---

## 8. Script E2E Dedicado

O script dedicado executa todos os passos acima de forma automatizada:

```bash
# Validação E2E completa do fluxo soberano
bash tryouts/tryout-sovereign-cb-liquidity.sh
```

O script inclui:
- Setup de variáveis de ambiente a partir de `.env.sovereign`
- Fluxos paralelos de CB-A e CB-B
- Polling de bridge state com timeout e erro explícito em caso de timeout
- Verificação on-chain via `cast logs`
- Contagem de violações (zero tolerância para G5-cross on-chain)
- Output colorido com status por etapa

---

## Depósito Adicional sem Bridge (FR-010)

Se o CB já possui saldo de W-tCeBM no Hub (de bridge anterior), pode depositar diretamente:

```bash
# Sem lock-mint, sem commit-reveal — depósito adicional direto
curl -X POST https://gateway-spoke-a/api/v2/amm/liquidity/add \
  -H "Authorization: Bearer $CB_A_JWT" \
  -d '{
    "pool_pair": "W-BRL-ARS",
    "side": "A",
    "amount": "50000000000000000000"
  }'
```

O gateway valida que o CB-A possui saldo W-BRL disponível antes de executar `addSingleSidedLiquidity`.

---

## Remoção de Liquidez (User Story 2)

```bash
# 1. CB-A remove sua LP position
curl -X DELETE https://gateway-spoke-a/api/v2/amm/liquidity/<position_id> \
  -H "Authorization: Bearer $CB_A_JWT"
# Valida: provider_id da posição == client_id do JWT (FR-007)

# 2. CB-A faz burn-unlock para reaver BRL no Spoke-A
curl -X POST https://gateway-spoke-a/api/v2/bridge/burn-unlock \
  -H "Authorization: Bearer $CB_A_JWT" \
  -d '{ "position_id": "<bridge_position_id>" }'
```

---

## Troubleshooting

| Sintoma | Causa provável | Solução |
|---|---|---|
| HTTP 422 em `/commit` | `bridge_state` ≠ `ACTIVE` | Aguardar Relayer confirmar; verificar `GET /api/v2/bridge/positions` |
| HTTP 403 em `/commit` | `provider_id` ≠ JWT `client_id` | Usar `provider_id` do próprio CB autenticado |
| `CommitMatched` não emitido | Um dos commits ainda `PENDING` | Verificar on-chain via `cast call $LCR_ADDR "getPendingCommit(string,uint8)"` |
| `addSingleSidedLiquidity` revertida | Signer não registrado no IdentityRegistry Hub | Executar `SeedNewSovereignPair.s.sol` steps 7/8 (setCentralBankOf) |
| Pool `reserve_b = 0` após CB-A depositar | CB-B ainda não executou sua perna | Normal — pool é unilateral até CB-B executar |
