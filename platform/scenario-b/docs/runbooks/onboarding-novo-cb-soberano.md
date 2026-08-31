# Runbook: Onboarding de Novo CB Soberano

**Feature**: 007-bridge-based-cb-liquidity  
**Versão**: 1.0  
**Audiência**: Operadores de infraestrutura e CBs parceiros

---

## Visão Geral

Este runbook descreve o processo para adicionar um novo Central Bank (CB) soberano ao protocolo CBWeb3 Hub, habilitando liquidez bilateral autônoma (sem MLP) via `LiquidityCommitRegistry`.

Ao final deste processo:
- O novo CB terá tokens W-tCeBM próprios no Hub
- Um novo par de pool AMM estará ativo no `PairRegistry`
- O `LiquidityCommitRegistry` estará pronto para coordenar commits bilaterais on-chain
- Os gateways de ambos os CBs estarão configurados com as variáveis de ambiente necessárias

---

## Pré-requisitos

| Requisito | Descrição |
|---|---|
| Hub running | Besu Hub rodando e acessível |
| IdentityRegistry | Endereço da `IdentityRegistry` no Hub |
| PairRegistry | Endereço do `PairRegistry` no Hub |
| Forge/Foundry | `forge` CLI instalado e funcionando |
| `make` | GNU Make disponível |
| Chaves privadas | CB-A e CB-B Hub private keys (hex, sem `0x`) |
| Admin key | Admin private key com `GOVERNANCE_ROLE` na `IdentityRegistry` |

---

## Passo 1: Registrar os CBs na IdentityRegistry (se ainda não feito)

```bash
# Registrar CB-A como CENTRAL_BANK via governance (admin)
curl -X POST $CB_A_URL/api/v2/governance/participants \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"address":"0xCB_A_ADDRESS","name":"Central Bank A","role":"CENTRAL_BANK","certFingerprint":"0x00"}'

# Registrar CB-B
curl -X POST $CB_B_URL/api/v2/governance/participants \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"address":"0xCB_B_ADDRESS","name":"Central Bank B","role":"CENTRAL_BANK","certFingerprint":"0x00"}'
```

---

## Passo 2: Executar `make contracts.seed-sovereign-pair`

Este comando:
1. Deploya 2 tokens W-tCeBM (um por CB)
2. Mapeia cada token ao seu CB emitente na `IdentityRegistry` (`setCentralBankOf`)
3. Deploya `LiquidityCommitRegistry` (ou reutiliza existente via `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`)
4. Deploya novo AMM para o par soberano
5. CB-A propõe o par (`proposePair`)
6. CB-B confirma (`confirmPair`) → par entra como `ACTIVE`
7. Exibe todos os endereços para configuração dos gateways

```bash
make contracts.seed-sovereign-pair \
  TOKEN_SYMBOL_A=BRL \
  TOKEN_SYMBOL_B=ARS \
  PAIR_ID=W-BRL-ARS \
  CB_A_HUB_PRIVATE_KEY=<hex_sem_0x> \
  CB_B_HUB_PRIVATE_KEY=<hex_sem_0x> \
  ADMIN_PRIVATE_KEY=<hex_sem_0x> \
  ADMIN_ADDRESS=0xADMIN_ADDRESS \
  HUB_IDENTITY_REGISTRY=0xIDENTITY_REG_ADDR \
  PAIR_REGISTRY_ADDRESS=0xPAIR_REG_ADDR \
  BESU_HUB_RPC=http://localhost:8645
```

**Saída esperada** (console.log do script):
```
===============================================
 SOVEREIGN PAIR SEEDED SUCCESSFULLY
===============================================
Pair ID:                          W-BRL-ARS
W-tCeBM_BRL:                      0xTOKEN_BRL_ADDR
W-tCeBM_ARS:                      0xTOKEN_ARS_ADDR
AMM Address:                      0xAMM_ADDR
LiquidityCommitRegistry:          0xLCR_ADDR
...
===============================================
Set in your gateway compose files:
  LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xLCR_ADDR
  SOVEREIGN_PAIR_AMM_MAP={"W-BRL-ARS":"0xAMM_ADDR"}
  SOVEREIGN_PAIR_IDS=W-BRL-ARS
  W_TOKEN_BRL_ADDRESS=0xTOKEN_BRL_ADDR
  W_TOKEN_ARS_ADDRESS=0xTOKEN_ARS_ADDR
  LOCAL_CB_HUB_SIGNER=<your CB's signer address>
===============================================
```

---

## Passo 3: Adicionar um novo par a um LCR existente (N CBs)

Se `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` já estiver definido, o script reutiliza o LCR existente:

```bash
make contracts.seed-sovereign-pair \
  TOKEN_SYMBOL_A=BRL \
  TOKEN_SYMBOL_B=EUR \
  PAIR_ID=W-BRL-EUR \
  LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xLCR_ADDR_EXISTENTE \
  ...
```

Não é necessário redesenhar o LCR — ele suporta múltiplos pares simultaneamente.

---

## Passo 4: Configurar variáveis de ambiente nos gateways

Adicione ao `backend/config/.env.infra.<entidade>` (não comitar secrets):

```env
# Gateway CB-A
LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xLCR_ADDR
SOVEREIGN_PAIR_AMM_MAP={"W-BRL-ARS":"0xAMM_ADDR"}
SOVEREIGN_PAIR_IDS=W-BRL-ARS
W_TOKEN_BRL_ADDRESS=0xTOKEN_BRL_ADDR
W_TOKEN_ARS_ADDRESS=0xTOKEN_ARS_ADDR
LOCAL_CB_HUB_SIGNER=0xCB_A_SIGNER_ADDRESS
INTERNAL_RELAY_AUTH_SECRET=<segredo_compartilhado>
```

```env
# Gateway CB-B
LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xLCR_ADDR
SOVEREIGN_PAIR_AMM_MAP={"W-BRL-ARS":"0xAMM_ADDR"}
SOVEREIGN_PAIR_IDS=W-BRL-ARS
W_TOKEN_BRL_ADDRESS=0xTOKEN_BRL_ADDR
W_TOKEN_ARS_ADDRESS=0xTOKEN_ARS_ADDR
LOCAL_CB_HUB_SIGNER=0xCB_B_SIGNER_ADDRESS
INTERNAL_RELAY_AUTH_SECRET=<mesmo_segredo>
```

---

## Passo 5: Configurar o Cacti Watcher

> **Arquitetura**: O `LiquidityCommitWatcher` é uma **instância única compartilhada** entre
> todos os gateways de CB — roda no contêiner Cacti relay centralizado, não em cada gateway
> individualmente. Ele escuta o Hub uma vez e notifica todos os gateways configurados via HTTP.
> Para adicionar um novo CB (CB-C), basta acrescentar a URL do novo gateway à variável
> `GATEWAY_INTERNAL_URLS` (separada por vírgula) e reiniciar o Cacti — sem novo contêiner
> de watcher.

Adicione ao docker-compose do Cacti relay:

```yaml
environment:
  LIQUIDITY_COMMIT_REGISTRY_ADDRESS: "0xLCR_ADDR"
  HUB_BESU_RPC: "http://host.docker.internal:8645"
  HUB_BESU_WS: "ws://host.docker.internal:8655"
  # Lista de gateways a notificar quando CommitMatched é detectado.
  # Para adicionar CB-C: acrescentar ",http://gateway-cb-c:8080" e reiniciar o Cacti.
  GATEWAY_INTERNAL_URLS: "http://gateway-cb-a:8080,http://gateway-cb-b:8080"
  INTERNAL_RELAY_AUTH_SECRET: "<segredo_compartilhado>"
  LCR_WATCHER_START_BLOCK: "0"
```

**Adicionando CB-C**: atualize apenas `GATEWAY_INTERNAL_URLS`:
```yaml
  GATEWAY_INTERNAL_URLS: "http://gateway-cb-a:8080,http://gateway-cb-b:8080,http://gateway-cb-c:8080"
```
O watcher enviará o payload `CommitMatched` para todos os gateways; cada gateway ignorará
silenciosamente os eventos que não correspondem ao seu `LOCAL_CB_HUB_SIGNER` (resposta
`{"status":"ignored"}`).

O `LiquidityCommitWatcher` iniciará automaticamente ao subir o serviço Cacti.

---

## Passo 6: Reiniciar os serviços

```bash
docker compose -f backend/docker-compose-backend.central-bank-a.yaml up -d --no-deps api-gateway-central-bank-a
docker compose -f backend/docker-compose-backend.central-bank-b.yaml up -d --no-deps api-gateway-central-bank-b
# Reiniciar Cacti relay
docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml up -d
```

---

## Passo 7: Validar

```bash
# Verificar par ativo no PairRegistry
curl http://localhost:38080/api/v2/amm/pairs | jq '.pairs[] | select(.pair_id=="W-BRL-ARS")'

# Rodar tryout completo
./tryouts/tryout-sovereign-cb-liquidity.sh

# Rodar testes Foundry do LCR
make contracts.test-sovereign
```

---

## Adicionando mais pares ao mesmo CB

Para cada novo par (`W-BRL-EUR`, `W-BRL-USD`, etc.):

1. Repita os Passos 2-6 com `TOKEN_SYMBOL_A`, `TOKEN_SYMBOL_B`, `PAIR_ID` diferentes
2. Atualize `SOVEREIGN_PAIR_AMM_MAP` para incluir o novo par:
   ```env
   SOVEREIGN_PAIR_AMM_MAP={"W-BRL-ARS":"0xAMM1","W-BRL-EUR":"0xAMM2"}
   SOVEREIGN_PAIR_IDS=W-BRL-ARS,W-BRL-EUR
   ```
3. Reinicie o gateway (sem rebuild de imagem — apenas variáveis de ambiente)

---

## Troubleshooting

| Sintoma | Causa | Solução |
|---|---|---|
| `BRIDGE_POSITION_NOT_ACTIVE` ao commitar | Bridge lock-mint ainda em processamento | Aguardar Relayer confirmar (estado `ACTIVE`) |
| `LCR__NotTokenCentralBank` | Signer não é o CB do token | Verificar `setCentralBankOf` na IdentityRegistry |
| `LCR__CommitAlreadyPending` | Já existe commit para este par/side | Cancelar commit existente ou aguardar expiração (72h) |
| `PROVIDER_ID_MISMATCH` | `provider_id` ≠ JWT `client_id` | Usar o `BANK_CODE` da entidade autenticada |
| Watcher não dispara | `GATEWAY_INTERNAL_URLS` vazia ou errada | Verificar env var no Cacti docker-compose |
| `CommitMatched` não emitido | Apenas um side registrado | Verificar que ambos os CBs enviaram commits |
