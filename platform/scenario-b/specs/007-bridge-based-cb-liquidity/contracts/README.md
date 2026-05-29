# Contracts: Bridge-Based CB Liquidity (spec-007)

**Feature**: `007-bridge-based-cb-liquidity`  
**Branch**: `007-bridge-based-cb-liquidity`  
**Date**: 2026-05-20

---

## Contratos Novos

### 1. LiquidityCommitRegistry.sol

**Arquivo de interface**: [`ILiquidityCommitRegistry.sol`](./ILiquidityCommitRegistry.sol)  
**Localização alvo na implementação**: `contracts/src/LiquidityCommitRegistry.sol`  
**Rede**: Hub (Besu)

**Propósito**: Coordenação on-chain bilateral de intenções de depósito de liquidez entre Bancos Centrais soberanos. Substitui comunicação inter-gateway off-chain pelo padrão de eventos on-chain trustless.

**Dependências externas** (contratos já deployados no Hub):
- `IIdentityRegistry` → `IDENTITY_REGISTRY` (endereço via construtor)
- Sem dependência de AMM ou PairRegistry (o contrato apenas coordena intenções)

**Padrão de deploy**:
```solidity
constructor(address _identityRegistry) {
    require(_identityRegistry != address(0));
    IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
}
```

**Segurança**:
- `registerCommit`: valida `getCentralBankOf(wTokenAddress) == msg.sender` → `LCR__NotTokenCentralBank`
- `cancelCommit`: valida `commit.signer == msg.sender` → `LCR__NotCommitOwner`
- `expireCommit`: permissionless — qualquer address pode executar após `expiresAt`
- Anti-reentrância: sem transferências de tokens; apenas armazenamento + eventos → reentrância não aplicável
- `CommitMatched` emitido atomicamente no mesmo `registerCommit` que completa o par

**Estimativa de gas**:
- `registerCommit` (sem match): ~50k gas
- `registerCommit` (com match, emite `CommitMatched`): ~70k gas
- `cancelCommit`: ~25k gas
- `expireCommit`: ~25k gas

---

### 2. W-tCeBM_BRL (TokenizedCentralBankMoney — novo deploy)

**Contrato**: `TokenizedCentralBankMoney.sol` (existente — sem modificação)  
**Script de deploy**: `contracts/script/SeedNewSovereignPair.s.sol` (novo — parametrizado via env vars)

**Configuração pós-deploy**:
```bash
# 1. Conceder CENTRAL_BANK_ROLE ao Bridge Relayer
TokenizedCentralBankMoney(W_BRL_ADDR).grantRole(CENTRAL_BANK_ROLE, RELAYER_ADDR)

# 2. Registrar relação CB-A → W-tCeBM_BRL no IdentityRegistry
IdentityRegistry.setCentralBankOf(W_BRL_ADDR, CB_A_HUB_SIGNER)
```

---

### 3. W-tCeBM_ARS (TokenizedCentralBankMoney — novo deploy)

**Contrato**: `TokenizedCentralBankMoney.sol` (existente — sem modificação)  
**Script de deploy**: `contracts/script/SeedNewSovereignPair.s.sol` (novo — parametrizado via env vars)

**Configuração pós-deploy**:
```bash
# 1. Conceder CENTRAL_BANK_ROLE ao Bridge Relayer
TokenizedCentralBankMoney(W_ARS_ADDR).grantRole(CENTRAL_BANK_ROLE, RELAYER_ADDR)

# 2. Registrar relação CB-B → W-tCeBM_ARS no IdentityRegistry
IdentityRegistry.setCentralBankOf(W_ARS_ADDR, CB_B_HUB_SIGNER)
```

---

### 4. AMM Soberano (AutomatedMarketMaker — novo deploy)

**Contrato**: `AutomatedMarketMaker.sol` (existente — sem modificação)  
**Script de deploy**: `contracts/script/SeedNewSovereignPair.s.sol` (novo — parametrizado via env vars)  
**Par**: `W-tCeBM_BRL` × `W-tCeBM_ARS`

**Ativação do par via PairRegistry**:
```bash
# Passo 5: CB-A propõe o par (msg.sender deve ser getCentralBankOf(W_BRL_ADDR))
PairRegistry.proposePair("W-BRL-ARS", W_BRL_ADDR, W_ARS_ADDR, SOVEREIGN_AMM_ADDR)
# tx assinada por CB_A_HUB_SIGNER

# Passo 6: CB-B confirma o par (msg.sender deve ser getCentralBankOf(W_ARS_ADDR))
PairRegistry.confirmPair("W-BRL-ARS")
# tx assinada por CB_B_HUB_SIGNER
```

---

## Contratos Modificados

### IdentityRegistry.sol

**Mudança**: Nenhuma — a função `setCentralBankOf` já existe e é chamada pelo script de deploy.

### TokenizedCentralBankMoney.sol

**Mudança**: Nenhuma — apenas novos deploys com símbolos distintos.

### AutomatedMarketMaker.sol

**Mudança**: Nenhuma — nova instância deployada para o par soberano.

### PairRegistry.sol

**Mudança**: Nenhuma — `proposePair`/`confirmPair` existentes são usados.

---

## Novo Script de Deploy: SeedNewSovereignPair.s.sol

**Localização**: `contracts/script/SeedNewSovereignPair.s.sol`
**Makefile target**: `contracts.seed-sovereign-pair`

> **Nota C2 (remediação 2026-05-20)**: O script foi renomeado de `SeedSovereignPool.s.sol` para `SeedNewSovereignPair.s.sol` e **parametrizado via env vars** para ser reutilizável com qualquer par de CBs (CB-C, CB-D, etc.) sem modificação de código.

**Variáveis de ambiente obrigatórias** (sem defaults hardcoded):
```
PAIR_ID                # ex: "W-BRL-ARS" ou "W-BRL-CLP"
TOKEN_SYMBOL_A         # ex: "W-tCeBM_BRL" ou "W-tCeBM_CLP"
TOKEN_SYMBOL_B         # ex: "W-tCeBM_ARS"
CB_A_HUB_PRIVATE_KEY   # chave privada do signer do CB emissor do token A
CB_B_HUB_PRIVATE_KEY   # chave privada do signer do CB emissor do token B
RELAYER_ADDR           # endereço do Bridge Relayer (recebe CENTRAL_BANK_ROLE)
ADMIN_PRIVATE_KEY      # governance/admin key
HUB_IDENTITY_REGISTRY  # endereço do IdentityRegistry no Hub
PAIR_REGISTRY_ADDRESS  # endereço do PairRegistry no Hub
# Opção: LIQUIDITY_COMMIT_REGISTRY_ADDRESS (se já deployado, reaproveitar)
```

**Sequência de operações** (em ordem obrigatória):
```
1. Deploy W-tCeBM_A (TokenizedCentralBankMoney, symbol=$TOKEN_SYMBOL_A)
2. Deploy W-tCeBM_B (TokenizedCentralBankMoney, symbol=$TOKEN_SYMBOL_B)
3. Deploy LiquidityCommitRegistry(identityRegistry_addr)  # se LIQUIDITY_COMMIT_REGISTRY_ADDRESS não definido
4. Deploy AutomatedMarketMaker(W_A_ADDR, W_B_ADDR, identityRegistry_addr, ...)
5. W-tCeBM_A.grantRole(CENTRAL_BANK_ROLE, $RELAYER_ADDR)    [admin tx]
6. W-tCeBM_B.grantRole(CENTRAL_BANK_ROLE, $RELAYER_ADDR)    [admin tx]
7. IdentityRegistry.setCentralBankOf(W_A_ADDR, CB_A_hub_signer)  [admin tx]
8. IdentityRegistry.setCentralBankOf(W_B_ADDR, CB_B_hub_signer)  [admin tx]
9. PairRegistry.proposePair($PAIR_ID, W_A_ADDR, W_B_ADDR, AMM_ADDR)
   → tx must be signed by CB_A_HUB_SIGNER (getCentralBankOf(W_A_ADDR))
10. PairRegistry.confirmPair($PAIR_ID)
    → tx must be signed by CB_B_HUB_SIGNER (getCentralBankOf(W_B_ADDR))
11. console.log addresses for env vars (SOVEREIGN_PAIR_AMM_MAP entry)
```

**Variáveis de ambiente necessitáveis** (passadas via shell / .env):
```
PAIR_ID  CB_A_HUB_PRIVATE_KEY  CB_B_HUB_PRIVATE_KEY
RELAYER_ADDR  ADMIN_PRIVATE_KEY  HUB_IDENTITY_REGISTRY  PAIR_REGISTRY_ADDRESS
```

**Uso para segundo par (CB-A + CB-C, par W-BRL-CLP)**:
```bash
PAIR_ID=W-BRL-CLP TOKEN_SYMBOL_A=W-tCeBM_BRL TOKEN_SYMBOL_B=W-tCeBM_CLP \
  CB_A_HUB_PRIVATE_KEY=$KEY_A CB_B_HUB_PRIVATE_KEY=$KEY_C \
  make contracts.seed-sovereign-pair
```
Nenhuma modificação de código necessária para onboarding de CB-C.

---

## Endpoint Interno Novo: /internal/amm/execute-matched-commit

**Serviço**: api-gateway  
**Autenticação**: header `X-Internal-Auth: $INTERNAL_RELAY_AUTH_SECRET` (já existente em `internal_relay_auth.go`)

**Request**:
```json
{
  "pool_pair": "W-BRL-ARS",
  "commit_id_a": "0x...",
  "signer_a": "0x...",
  "amount_a": "100000000000000000000",
  "commit_id_b": "0x...",
  "signer_b": "0x...",
  "amount_b": "100000000000000000000"
}
```

**Lógica do handler**:
1. Determinar se `signer_a == local_cb_hub_signer` (executar side A) ou `signer_b == local_cb_hub_signer` (executar side B)
2. Validar que `provider_id` do `PoolCommit` local bate com o signer local (FR-007)
3. Executar `addSingleSidedLiquidity(isTokenA, amount)` on-chain com signer local
4. Atualizar `PoolCommit.status = EXECUTED` e criar `LiquidityPosition`
5. Se nenhum dos signers for local: ignorar silenciosamente (evento destinado ao outro gateway)

**Resposta**: `{ "status": "executed" | "ignored" }`

---

## Testes de Contrato (Foundry)

**Localização**: `contracts/test/LiquidityCommitRegistry.t.sol`

**Casos obrigatórios**:
1. `test_registerCommit_singleSide` — registra apenas side A, verifica `CommitRegistered`
2. `test_registerCommit_triggerMatch` — registra A depois B, verifica `CommitMatched`
3. `test_registerCommit_revertsIfNotCB` — signer não é CB do token → `LCR__NotTokenCentralBank`
4. `test_registerCommit_revertsIfAlreadyPending` — duplicata de (poolPair, side) → `LCR__CommitAlreadyPending`
5. `test_cancelCommit` — signer cancela próprio commit → `CommitCancelled`
6. `test_cancelCommit_revertsIfNotOwner` — outro signer tenta cancelar → `LCR__NotCommitOwner`
7. `test_expireCommit` — vm.warp + expireCommit → `CommitExpired`
8. `test_expireCommit_revertsIfNotExpired` — antes do vencimento → `LCR__CommitNotExpired`
