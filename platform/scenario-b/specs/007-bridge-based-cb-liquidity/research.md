# Research: Bridge-Based CB Liquidity (spec-007)

**Feature**: `007-bridge-based-cb-liquidity`  
**Branch**: `007-bridge-based-cb-liquidity`  
**Date**: 2026-05-20  
**Status**: Complete — todas as questões NEEDS CLARIFICATION resolvidas

---

## 1. Contrato W-tCeBM: mesmo tipo ou contrato separado?

**Decision**: W-tCeBM_BRL e W-tCeBM_ARS são **novos deploys do mesmo contrato `TokenizedCentralBankMoney.sol`** com símbolos distintos (`W-tCeBM_BRL`, `W-tCeBM_ARS`). Não é um novo tipo de contrato Solidity.

**Rationale**:
- `TokenizedCentralBankMoney` já implementa ERC-20 com `CENTRAL_BANK_ROLE` para mint/burn — exatamente a semântica necessária para o Bridge Relayer.
- O Relayer recebe `CENTRAL_BANK_ROLE` no contrato W-tCeBM deployado no Hub.
- O `mirrored_asset` no campo da API `POST /api/v2/bridge/lock-mint` é o endereço Ethereum deste novo contrato Hub.
- O CB-A gateway registra a intenção no `LiquidityCommitRegistry` passando `w_token_address = <W-tCeBM_BRL_address>`.

**Evidência** (`SeedHub.s.sol`, `TokenizedCentralBankMoney.sol`):
- `SeedHub.s.sol` demonstra que tokens Hub são instâncias de `TokenizedCentralBankMoney` com `CENTRAL_BANK_ROLE` concedido ao holder.
- O Relayer passa a ser grantee de `CENTRAL_BANK_ROLE` no contrato W-tCeBM de cada CB no Hub.

**Impacto no deploy**: Um script Foundry **parametrizado** (`SeedNewSovereignPair.s.sol`) deployará 2 novos contratos `TokenizedCentralBankMoney` (W-tCeBM_BRL + W-tCeBM_ARS) no Hub, concederá `CENTRAL_BANK_ROLE` ao endereço do Relayer e chamará `IdentityRegistry.setCentralBankOf(W-tCeBM_BRL_addr, CB_A_hub_signer)`.

**Alternatives considered**:
- Novo contrato `WrappedTokenizedCentralBankMoney.sol` — rejeitado (duplicação desnecessária de código Solidity; `TokenizedCentralBankMoney` já tem a semântica correta).
- Reutilizar `HUB_TOKEN_A_ADDRESS` / `HUB_TOKEN_B_ADDRESS` — rejeitado explicitamente em Q1 da sessão de clarificação (os tokens Hub existentes representam o padrão G5-cross direto; W-tCeBM são contratos distintos).

---

## 2. IdentityRegistry.getCentralBankOf(): existe? Qual a assinatura?

**Decision**: A função **existe** em `contracts/src/interfaces/IIdentityRegistry.sol` e `contracts/src/IdentityRegistry.sol`.

**Assinatura confirmada**:
```solidity
function getCentralBankOf(address token) external view returns (address);
function setCentralBankOf(address token, address centralBank) external; // DEFAULT_ADMIN_ROLE
```

**Uso no `LiquidityCommitRegistry`**:
```solidity
require(
    IDENTITY_REGISTRY.getCentralBankOf(w_token_address) == msg.sender,
    "LCR: caller not CB of token"
);
```

**Uso no guard anti-G5-cross (FR-004)**: O handler de `mint-and-approve` percorre os tokens conhecidos no IdentityRegistry para verificar se `recipient` é signer de algum CB. Implementação: checar se `getCentralBankOf(any_registered_token) == recipient` para qualquer token configurado. Alternativa mais simples: verificar role `CENTRAL_BANK` no `IdentityRegistry.getParticipant(recipient).role`.

**Recommendation**: Usar `getParticipant(recipient).role == ParticipantRole.CENTRAL_BANK` para FR-004 (mais direto; `getCentralBankOf` mapeia token → CB, não recipient → role).

**Evidência**: `PairRegistry.sol:125`, `CurrencyRegistry.sol`, `IdentityRegistry.sol:151-160`.

---

## 3. Event watcher pattern: como o Relayer Cacti assiste eventos?

**Decision**: O Relayer existente (`interop/hub-and-spoke/cacti/`) é dedicado a **HTLC** e não deve ser modificado. Para `LiquidityCommitRegistry`, criar um **novo watcher TypeScript** com a mesma infraestrutura (`PluginLedgerConnectorBesu`, `watchBlocksV1`).

**Padrão identificado** em `interop/hub-and-spoke/cacti/src/htlc-relay.ts`:
- Usa `PluginLedgerConnectorBesu.watchBlocksV1()` com Socket.IO para assinar novos blocos.
- Para cada bloco: chama `getPastLogs` com filtro de ABI do contrato e processa os logs.
- Resultado: chama o gateway Go via gRPC ou HTTP interno.

**Para `LiquidityCommitRegistry`**:
- Novo serviço TypeScript `interop/hub-and-spoke/cacti/src/liquidity-commit-watcher.ts`.
- Assiste evento `CommitMatched(pool_pair, commit_id_a, signer_a, amount_a, commit_id_b, signer_b, amount_b)`.
- Ao receber evento: POST para o gateway interno do CB correspondente (`/internal/amm/execute-matched-commit`).
- O gateway valida que `signer_a` ou `signer_b` é o signer local antes de executar `addSingleSidedLiquidity`.

**Alternatives considered**:
- Polling periódico via `eth_getLogs` no Go backend — viável mas mais lento; watcher TypeScript reutiliza infraestrutura já testada.
- Extend HTLC relay — rejeitado (separação de concerns; HTLC é Scenario A / spoke-spoke; commit-registry é Hub-only).

---

## 4. addSingleSidedLiquidity: assinatura e validação de msg.sender

**Decision**: Assinatura confirmada no `AutomatedMarketMaker.sol`:
```solidity
function addSingleSidedLiquidity(bool isTokenA, uint256 amount)
    external
    nonReentrant
    whenNotPaused
    onlyLiquidityProvider(msg.sender);
```

**`onlyLiquidityProvider`** verifica `IdentityRegistry.canTransact(msg.sender)` — o signer do CB deve estar registrado no `IdentityRegistry` do Hub.

**Impacto**: O CB signer de cada gateway DEVE estar registrado no `IdentityRegistry` do Hub com `canTransact == true`. Isso já é feito em `SeedHub.s.sol` para os signers existentes. O novo script `SeedNewSovereignPair.s.sol` registrará os signers adicionais necessários (ex: CB-A hub signer, CB-B hub signer) se ainda não estiverem.

**Flag `isTokenA`**: Determinado pela ordem de deployment do novo AMM. No AMM soberano (W-tCeBM_BRL, W-tCeBM_ARS), `isTokenA = true` → W-tCeBM_BRL, `isTokenA = false` → W-tCeBM_ARS. O gateway mapeia `side: "A"|"B"` do commit para o flag correto.

---

## 5. PairRegistry.proposePair / confirmPair: fluxo completo

**Decision**: O fluxo para criar o novo AMM soberano é:

```
1. Deploy: new AutomatedMarketMaker(W-tCeBM_BRL_addr, W-tCeBM_ARS_addr, identityRegistry, ...)
2. Deploy: LiquidityCommitRegistry(identityRegistry_addr)
3. IdentityRegistry.setCentralBankOf(W-tCeBM_BRL_addr, CB_A_hub_signer)       [admin tx]
4. IdentityRegistry.setCentralBankOf(W-tCeBM_ARS_addr, CB_B_hub_signer)       [admin tx]
5. PairRegistry.proposePair("W-BRL-ARS", W-tCeBM_BRL_addr, W-tCeBM_ARS_addr, newAMM_addr)
   → msg.sender must == getCentralBankOf(W-tCeBM_BRL_addr) == CB_A_hub_signer [CB-A tx]
6. PairRegistry.confirmPair("W-BRL-ARS")
   → msg.sender must == getCentralBankOf(W-tCeBM_ARS_addr) == CB_B_hub_signer [CB-B tx]
7. Pair "W-BRL-ARS" is ACTIVE — AMM soberano pronto para liquidez
```

**Evidência**: `PairRegistry.sol:112-155` (`proposePair` e `confirmPair` com validação via `getCentralBankOf`).

**Constraint importante**: `proposePair` e `confirmPair` são txs bilaterais — exigem participação de ambos os gateways (CB-A propõe, CB-B confirma). No script de deploy / setup, ambas as txs são assinadas com as chaves corretas.

---

## 6. Bridge lock-mint para CBs: mesmo endpoint, restrições adicionais?

**Decision**: Mesmo endpoint `POST /api/v2/bridge/lock-mint` já existente (spec-002). Nenhuma mudança de assinatura de API. A diferença é contextual: o chamador autenticado é um CB (com `CENTRAL_BANK_ROLE` no spoke). O payload inclui `mirrored_asset` = endereço do W-tCeBM no Hub.

**Novo gate no commit handler** (FR-001):
```go
// Antes de persistir commit ou enviar tx on-chain:
pos, err := db.FindActiveBridgePosition(cb_id, w_token_address)
if err != nil || pos.BridgeState != ACTIVE {
    return fiber.NewError(422, `{"error":"no active bridge position...","code":"BRIDGE_POSITION_NOT_ACTIVE"}`)
}
```

**Polling pattern** (tryout): `GET /api/v2/bridge/positions?owner_bank_id=<cb_id>` + filtro `bridge_state = ACTIVE` até timeout (120s / intervalo 5s).

---

## 7. Modelo de expiração dos commits no LiquidityCommitRegistry

**Decision**: Commits expiram após 72h sem match (herdado de spec-005 FR-013). O contrato armazena `expires_at = block.timestamp + 72h` no `registerCommit`. Uma função `expireCommit(commit_id)` (callable por qualquer address) verifica o vencimento e emite `CommitExpired(commit_id)` — padrão de garbage collection pull.

**On-chain reconciliation**: Se `CommitMatched` foi emitido mas um dos gateways não executou `addSingleSidedLiquidity` em até 300s, o gateway passa o commit local para `RECONCILIATION_REQUIRED`. O contrato on-chain não precisa rastrear este estado — é tracking off-chain no DB do gateway.

---

## 8. Proteção anti-G5-cross no mint-and-approve (FR-004)

**Decision**: Usar `getParticipant(recipient).role == ParticipantRole.CENTRAL_BANK` via `IdentityRegistry` no Hub.

```go
participant, err := identityRegistry.GetParticipant(ctx, req.Recipient)
if err == nil && participant.Role == "CENTRAL_BANK" {
    return fiber.NewError(403, `{"error":"recipient is a Central Bank signer — cross-CB minting is prohibited","code":"CROSS_CB_MINT_PROHIBITED"}`)
}
```

**Alternativa rejeitada**: Manter lista de endereços CB hardcoded no backend — frágil, não escala com N CBs futuros.

---

## NEEDS CLARIFICATION restantes

Nenhum. Todos os itens foram resolvidos nesta pesquisa.

## Referências

| Arquivo | Relevância |
|---|---|
| `contracts/src/TokenizedCentralBankMoney.sol` | Contrato base para W-tCeBM |
| `contracts/src/interfaces/IIdentityRegistry.sol` | `getCentralBankOf` / `setCentralBankOf` |
| `contracts/src/PairRegistry.sol` | `proposePair` / `confirmPair` bilaterais |
| `contracts/src/interfaces/IAutomatedMarketMaker.sol` | `addSingleSidedLiquidity(bool isTokenA, uint256 amount)` |
| `contracts/src/SpokeBridge.sol` | Interface de lock no spoke |
| `contracts/script/SeedHub.s.sol` | Padrão de deploy de tokens Hub |
| `backend/shared/blockchain/scenariob/relayer/client.go` | `SubmitLockRequest` com `mirrored_asset` |
| `backend/services/payment-orchestrator/internal/workers/relayer_worker.go` | Padrão RelayerWorker |
| `interop/hub-and-spoke/cacti/src/htlc-relay.ts` | Padrão de event watcher Cacti |
