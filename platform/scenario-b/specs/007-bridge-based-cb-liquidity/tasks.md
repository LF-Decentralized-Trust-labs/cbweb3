# Tasks: Bridge-Based CB Liquidity (Fluxo Soberano)

**Input**: Design documents from `/specs/007-bridge-based-cb-liquidity/`  
**Branch**: `007-bridge-based-cb-liquidity`  
**Prerequisites**: plan.md ✅ · spec.md ✅ · research.md ✅ · data-model.md ✅ · contracts/ ✅ · quickstart.md ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos distintos, sem dependência em task incompleta)
- **[Story]**: User story correspondente (US1, US2, US3)
- Paths de arquivo explícitos em cada task

---

## Phase 1: Setup

**Purpose**: Infraestrutura compartilhada e inicialização do projeto.

- [X] T001 Copiar `ILiquidityCommitRegistry.sol` da spec para `contracts/src/interfaces/ILiquidityCommitRegistry.sol` conforme interface definida em `specs/007-bridge-based-cb-liquidity/contracts/ILiquidityCommitRegistry.sol`
- [X] T002 [P] Documentar variáveis de ambiente necessárias em `deploy/local/README.md`: `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`, `SOVEREIGN_PAIR_AMM_MAP` (JSON, ex: `{"W-BRL-ARS":"0x..."}`), `SOVEREIGN_PAIR_IDS` (lista separada por vírgula), `W_TOKEN_BRL_ADDRESS`, `W_TOKEN_ARS_ADDRESS`, `LOCAL_CB_HUB_SIGNER`, `CB_A_HUB_PRIVATE_KEY`, `CB_B_HUB_PRIVATE_KEY` — *remediação C1*

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Contratos on-chain + migração de DB + adapter Go. DEVE ser completo antes de qualquer user story.

**⚠️ CRÍTICO**: Nenhum trabalho de User Story pode começar até que esta fase seja concluída.

- [X] T003 Implementar `contracts/src/LiquidityCommitRegistry.sol` — contrato completo implementando `ILiquidityCommitRegistry`: storage `mapping(bytes32 => mapping(uint8 => Commit)) commits`, `mapping(bytes32 => Commit) commitById`; função `registerCommit` com validação `IdentityRegistry.getCentralBankOf(wTokenAddress) == msg.sender`, detecção de match bilateral e emissão de `CommitMatched` na mesma tx; funções `cancelCommit` e `expireCommit`; expiração `block.timestamp + 72h` (FR-003)
- [X] T004 [P] Criar `contracts/test/LiquidityCommitRegistry.t.sol` — 8 testes Foundry: `test_registerCommit_singleSide`, `test_registerCommit_triggerMatch`, `test_registerCommit_revertsIfNotCB`, `test_registerCommit_revertsIfAlreadyPending`, `test_cancelCommit`, `test_cancelCommit_revertsIfNotOwner`, `test_expireCommit` (vm.warp), `test_expireCommit_revertsIfNotExpired` — conforme especificado em `specs/007-bridge-based-cb-liquidity/contracts/README.md`
- [X] T005 [P] Criar `contracts/script/SeedNewSovereignPair.s.sol` — script Foundry **parametrizado via env vars** (sem valores hardcoded de BRL/ARS) que executa em ordem: (1) deploy `TokenizedCentralBankMoney` como W-tCeBM com símbolo `$TOKEN_SYMBOL_A`; (2) deploy `TokenizedCentralBankMoney` como W-tCeBM com símbolo `$TOKEN_SYMBOL_B`; (3) deploy `LiquidityCommitRegistry(identityRegistry_addr)` *se não houver `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` no env* (reaproveitar instância existente para novo par); (4) deploy `AutomatedMarketMaker(W_A_ADDR, W_B_ADDR, identityRegistry, ...)`; (5) `grantRole(CENTRAL_BANK_ROLE, $RELAYER_ADDR)` nos dois tokens; (6) `setCentralBankOf(W_A_ADDR, CB_A_HUB_SIGNER)` e `setCentralBankOf(W_B_ADDR, CB_B_HUB_SIGNER)`; (7) `PairRegistry.proposePair($PAIR_ID, ...)` assinado por CB_A; (8) `PairRegistry.confirmPair($PAIR_ID)` assinado por CB_B; (9) `console.log` de todos os endereços para geração de env vars — conforme `specs/007-bridge-based-cb-liquidity/contracts/README.md`. *Remediação C2: usar com `make contracts.seed-sovereign-pair` para qualquer par de CBs futuros sem modificação de código*
- [X] T006 [P] Adicionar migração `pool_commits`: coluna `on_chain_commit_id` do tipo `bytea` (bytes32), nullable, padrão NULL — em `backend/services/api-gateway/internal/domain/pool_commit.go` (campo `OnChainCommitID *[]byte`) e migration file SQL correspondente; sem breaking change (backfill NULL para registros existentes)
- [X] T007 Implementar adapter `backend/services/api-gateway/internal/blockchain/liquidity_commit_registry.go` — client go-ethereum para `LiquidityCommitRegistry`: método `RegisterCommit(ctx, poolPair, side, amount, wTokenAddress string) (bytes32, error)` que assina e envia tx on-chain usando o signer local do gateway; método `GetPendingCommit(ctx, poolPair, side string) (bytes32, error)` para consulta; ABI derivada de `ILiquidityCommitRegistry.sol`; variável de ambiente `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`

**Checkpoint**: Contratos, migração e adapter prontos — implementação de user stories pode começar

---

## Phase 3: User Story 1 — CB Injeta Liquidez Soberana via Bridge (Priority: P0) 🎯 MVP

**Goal**: CB-A e CB-B provisionam liquidez no AMM soberano via bridge lock-mint, cada um usando seu próprio signer soberano em todas as etapas on-chain. Coordenação bilateral via `LiquidityCommitRegistry` on-chain sem comunicação inter-gateway.

**Independent Test**: Executar apenas CB-A: bridge lock-mint → polling ACTIVE → commit → aguardar evento `CommitMatched` on-chain → handler local executa `addSingleSidedLiquidity(true, amount)` com signer de CB-A. Verificar: pool `reserve_a > 0, reserve_b = 0`; `cast logs` confirma `msg.sender == CB_A_HUB_SIGNER` no evento `LogSingleSidedLiquidityAdded`.

- [X] T008 [P] [US1] Adicionar gate de `bridge_state = ACTIVE` no handler de commit em `backend/services/api-gateway/internal/http/handlers/bridge_handler.go`: antes de qualquer persistência ou tx on-chain, verificar existência de `BridgedAssetPosition` com `bridge_state = ACTIVE` e `mirrored_asset` correspondente ao `side` e CB autenticado; retornar HTTP 422 `{"error":"no active bridge position found for this side — wait for Relayer confirmation","code":"BRIDGE_POSITION_NOT_ACTIVE"}` se não encontrado (FR-001)
- [X] T009 [P] [US1] Adicionar validação `provider_id == JWT client_id` no handler de commit em `backend/services/api-gateway/internal/http/handlers/bridge_handler.go`: extrair `client_id` do JWT autenticado; comparar com `req.ProviderID`; retornar HTTP 403 `{"error":"provider_id mismatch — must match authenticated CB identity","code":"PROVIDER_ID_MISMATCH"}` se diferente (FR-009)
- [X] T010 [US1] Integrar chamada on-chain `RegisterCommit` no handler de commit em `backend/services/api-gateway/internal/http/handlers/bridge_handler.go`: após validações T008 e T009, chamar `liquidityCommitRegistryAdapter.RegisterCommit(ctx, poolPair, side, amount, wTokenAddress)`; persistir `PoolCommit.OnChainCommitID` com o `bytes32` retornado; apenas então retornar HTTP 201 com `on_chain_commit_id` no response body (FR-003) — depende T007, T008, T009
- [X] T011 [US1] Adicionar rota `POST /internal/amm/execute-matched-commit` em `backend/services/api-gateway/internal/http/router/v2/router.go` com middleware `internal_relay_auth` existente (header `X-Internal-Auth: $INTERNAL_RELAY_AUTH_SECRET`) (FR-003)
- [X] T012 [US1] Implementar handler `executeMatchedCommit` em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`: deserializa payload `{pool_pair, commit_id_a, signer_a, amount_a, commit_id_b, signer_b, amount_b}`; **faz lookup `ammAddress = SOVEREIGN_PAIR_AMM_MAP[event.pool_pair]`** (mapa de config carregado do env `SOVEREIGN_PAIR_AMM_MAP` em JSON); retorna erro 400 se `pool_pair` desconhecido; determina se `signer_a` ou `signer_b` corresponde ao signer local (`LOCAL_CB_HUB_SIGNER`); se nenhum corresponder, retorna `{"status":"ignored"}`; se corresponder, delega para `SovereignLiquidityService.ExecuteMatchedCommit(ctx, ammAddress, ...)` (FR-003, FR-005) — *remediação C1* — depende T011
- [X] T013 [US1] Implementar `backend/services/api-gateway/internal/services/sovereign_liquidity_service.go` com método `ExecuteMatchedCommit(ctx, ammAddress, side, amount, wTokenAddress)`: (1) valida que signer local possui role adequado para o side (`getCentralBankOf(wTokenAddress) == local_signer`); (2) aprova allowance de W-tCeBM para `ammAddress` (lookup via `SOVEREIGN_PAIR_AMM_MAP` — *não hardcoded*); (3) executa `addSingleSidedLiquidity(isTokenA, amount)` com signer local soberano; (4) atualiza `PoolCommit.status = EXECUTED`; (5) cria registro em `LiquidityPosition`; (6) em caso de falha, transiciona commit para `RECONCILIATION_REQUIRED` após timeout configurável de 300s (FR-003, FR-005, NFR-001) — *remediação C1* — depende T012
- [X] T014 [P] [US1] Implementar `interop/hub-and-spoke/cacti/src/liquidity-commit-watcher.ts` — novo módulo TypeScript que usa `PluginLedgerConnectorBesu.watchBlocksV1()` para assitir evento `CommitMatched` do `LiquidityCommitRegistry` no Hub; ao detectar o evento, faz POST para `$GATEWAY_INTERNAL_URL/internal/amm/execute-matched-commit` com o header `X-Internal-Auth`; registrado no `index.ts` do Cacti como watcher adicional ao lado do HTLC relay; variáveis de ambiente: `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`, `HUB_RPC_URL`, `GATEWAY_INTERNAL_URL`, `INTERNAL_RELAY_AUTH_SECRET` (FR-003, NFR-001)

**Checkpoint**: US1 completo — fluxo soberano end-to-end funcional; cada CB adiciona liquidez com signer soberano

---

## Phase 4: User Story 2 — CB Remove Liquidez com Unlock no Spoke (Priority: P1)

**Goal**: CB-A remove sua LP position; apenas o owner (validado por JWT) pode remover a posição. W-tCeBM devolvido ao signer de CB-A; opção de burn-unlock para recuperar BRL no Spoke-A.

**Independent Test**: Criar `LiquidityPosition` com `provider_id=cb-a`; chamar `DELETE /api/v2/amm/liquidity/{position_id}` com JWT de CB-A → posição `WITHDRAWN`, W-tCeBM devolvido; chamar com JWT de CB-B → HTTP 403 `PROVIDER_ID_MISMATCH`.

- [X] T015 [US2] Adicionar validação `provider_id == JWT client_id` em `removeLiquidity` handler em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`: extrair `client_id` do JWT; comparar com `LiquidityPosition.ProviderID` da posição solicitada; retornar HTTP 403 `{"error":"provider_id mismatch — you can only remove your own liquidity position","code":"PROVIDER_ID_MISMATCH"}` se diferente; apenas então executar `removeLiquidity` on-chain com signer local (FR-007)

**Checkpoint**: US2 completo — remoção de liquidez com validação de ownership JWT funcional

---

## Phase 5: User Story 3 — Endpoint Rejeita G5-cross (Priority: P1)

**Goal**: Qualquer tentativa de `mint-and-approve` com `recipient` sendo signer de outro CB é rejeitada antes de qualquer tx on-chain, impedindo recriação acidental do padrão G5-cross.

**Independent Test**: POST `mint-and-approve` com `recipient = CB_A_HUB_SIGNER` via gateway CB-B → HTTP 403 `CROSS_CB_MINT_PROHIBITED`; POST com `recipient = <commercial_bank_addr>` → HTTP 200 normal (backward-compat).

- [X] T016 [P] [US3] Implementar guard anti-G5-cross em `backend/services/api-gateway/internal/http/handlers/mint_handler.go`: se request body contiver campo `recipient` (não vazio), chamar `identityRegistry.GetParticipant(ctx, recipient)`; se `participant.Role == "CENTRAL_BANK"`, retornar HTTP 403 `{"error":"recipient is a Central Bank signer — cross-CB minting is prohibited","code":"CROSS_CB_MINT_PROHIBITED"}` antes de qualquer tx on-chain; se `recipient` ausente ou não for CB, prosseguir normalmente (FR-004, SC-002)

**Checkpoint**: US3 completo — bloqueio de G5-cross ativo em 100% das tentativas

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Integração de deploy, script E2E e validações finais.

- [X] T017 Adicionar variáveis de ambiente ao `backend/docker-compose-backend.bank-a.yaml` e `backend/docker-compose-backend.bank-b.yaml`: `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`, **`SOVEREIGN_PAIR_AMM_MAP`** (JSON com todos os pares do CB, ex: `{"W-BRL-ARS":"0xAMM_ADDR"}`), **`SOVEREIGN_PAIR_IDS`** (lista separada por vírgula, ex: `W-BRL-ARS`), `W_TOKEN_BRL_ADDRESS`, `W_TOKEN_ARS_ADDRESS`, `LOCAL_CB_HUB_SIGNER` — valores preenchidos após execução de `SeedNewSovereignPair.s.sol`; adicionar container do watcher `liquidity-commit-watcher` a ambos os compose files referenciando `interop/hub-and-spoke/cacti/`; *nota*: para novo CB-C, copiar e adaptar `bank-a.yaml` com env vars do novo CB — *remediações C1 e C2*
- [X] T018 [P] Criar `tryouts/tryout-sovereign-cb-liquidity.sh` — script Bash standalone que executa o fluxo completo: (1) CB-A mint no Spoke-A; (2) CB-A lock-mint + polling até ACTIVE (timeout 120s); (3) CB-A commit → `LiquidityCommitRegistry.registerCommit`; (4) CB-B mint no Spoke-B; (5) CB-B lock-mint + polling até ACTIVE; (6) CB-B commit ao seu próprio gateway → match automático; (7) polling de ambas as posições LP até EXECUTED com **timestamp de início e fim** desde o evento `CommitMatched` para validação de **p95 ≤ 30s** (NFR-001, SC-003); (8) verificação SC-001: `cast logs` confirma `msg.sender` por CB correto (zero violações); (9) teste SC-002: mint-and-approve com recipient CB signer → assertar HTTP 403; (10) imprimir `LATENCY_COMMIT_MATCHED_TO_EXECUTED=Xs` ao final; output colorido com status por etapa (FR-008, SC-001 a SC-005, NFR-001)
- [X] T019 [P] Adicionar Makefile target `contracts.seed-sovereign-pair` em `make/30-contracts.mk` que executa `SeedNewSovereignPair.s.sol` via `forge script` com as variáveis de ambiente `PAIR_ID`, `TOKEN_SYMBOL_A`, `TOKEN_SYMBOL_B`, `CB_A_HUB_PRIVATE_KEY`, `CB_B_HUB_PRIVATE_KEY`, `RELAYER_ADDR`, `ADMIN_PRIVATE_KEY`, `HUB_IDENTITY_REGISTRY`, `PAIR_REGISTRY_ADDRESS` — *remediação C2*
- [X] T020 [P] Executar `forge test --match-contract LiquidityCommitRegistry` e confirmar que os 8 testes passam; documentar resultado no `contracts/README.md`
- [X] T021 [US1] Implementar depósito adicional sem bridge em pool ACTIVE (FR-010): adicionar rota `POST /api/v2/amm/liquidity/add` (ou endpoint equivalente) em `backend/services/api-gateway/internal/http/router/v2/router.go`; handler valida que CB autenticado possui saldo de W-tCeBM correspondente via chamada `tokenContract.balanceOf(cb_hub_signer)`; se saldo suficiente, executa diretamente `addSingleSidedLiquidity(isTokenA, amount)` via signer local — sem gate `bridge_state=ACTIVE`, sem commit-reveal; retorna HTTP 422 `{"error":"insufficient W-tCeBM balance for direct deposit","code":"INSUFFICIENT_BALANCE"}` se saldo insuficiente; cria `LiquidityPosition` normalmente — *cobertura de FR-010 (finding H2)*
- [X] T022 [P] Criar `docs/runbooks/onboarding-novo-cb-soberano.md` — runbook operacional documentando a sequência completa para adicionar um novo CB-C ao protocolo soberano: (1) deploy W-tCeBM_C via `SeedNewSovereignPair.s.sol`; (2) configurar `SOVEREIGN_PAIR_AMM_MAP` no docker-compose do novo gateway; (3) registrar CB-C no `IdentityRegistry` do Hub; (4) ativar par via `proposePair`+`confirmPair`; (5) iniciar container watcher com novo endereço de LCR; estimativa de tempo por passo; erros comuns e como resolver — *remediação M2*

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — começar imediatamente
- **Foundational (Phase 2)**: Depende de Setup — BLOQUEIA todas as user stories
  - T003 (contrato) deve preceder T007 (adapter Go usa ABI do contrato)
  - T004 e T005 podem rodar em paralelo com T003 (arquivos distintos)
  - T006 é independente — pode rodar em paralelo com T003-T005
- **User Stories (Phase 3+)**: Todas dependem da conclusão de Foundational
  - US1 (T008–T014): criticamente bloqueada por T007 (adapter Go)
  - US3 (T016): totalmente independente de US1 e US2 — pode rodar em paralelo
  - US2 (T015): independente de US1 mas logicamente pós-US1 (precisa de LP position)
- **Polish (Phase 6)**: Depende de todas as user stories completas

### User Story Dependencies

| User Story | Bloqueado por | Pode rodar em paralelo com |
|---|---|---|
| US1 (P0) | Phase 2 completa | US3 |
| US2 (P1) | Phase 2 completa | US3 |
| US3 (P1) | Phase 2 completa | US1, US2 |

### Dentro de US1

```
T008 ──┐
T009 ──┤── T010 ── T011 ── T012 ── T013
       │
T014 ──┘  (paralelo completo — TypeScript, sem dependência Go)
```

### Parallel Execution Examples

**US1** (com dois desenvolvedores):
- Dev 1: T008 → T009 → T010 → T013 (fluxo commit handler + service)
- Dev 2: T011 → T012 (rota + handler interno) ∥ T014 (TS watcher — totalmente independente)

**US3** (independente de US1):
- Pode ser desenvolvido em paralelo com qualquer task de US1 após T002 (setup)

---

## Implementation Strategy

### MVP Scope (mínimo para SC-001 e SC-005 passarem)

Implement apenas: **Phase 2 completa** + **T008–T013** (US1 sem watcher) + **T017** (env vars)

Com isso, o fluxo manual é demonstrável via chamadas cURL:
1. Confirmar deploy dos contratos
2. Bridge lock-mint via API
3. Commit via API → tx on-chain
4. POST manual para `/internal/amm/execute-matched-commit`
5. Verificar pool ACTIVE + cast logs

### Entrega Incremental

| Incremento | Tasks | Valida |
|---|---|---|
| MVP — Fluxo manual | Phase 2 + T008–T013 + T017 | SC-001 (parcial), SC-005 (manual) |
| US1 automático | + T014 (watcher TS) | SC-001, SC-003, SC-005 (automático) |
| US3 bloqueio | + T016 | SC-002 |
| US2 remoção | + T015 | FR-007 |
| E2E completo | + T018–T020 | Todos os SCs |

### Format Validation

Todas as tasks seguem o formato obrigatório:
- ✅ Inicia com `- [ ]`
- ✅ ID sequencial (T001–T020)
- ✅ Marcador `[P]` onde aplicável
- ✅ Label `[US1]`, `[US2]`, `[US3]` nas fases de user story
- ✅ Path de arquivo explícito em cada task de implementação

**Total de tasks**: 22  
**Tasks de US1**: 9 (T008–T014, T021 — fluxo soberano + depósito adicional FR-010)  
**Tasks de US2**: 1 (T015)  
**Tasks de US3**: 1 (T016)  
**Oportunidades de paralelo identificadas**: 10 tasks marcadas `[P]`  
**Critério de teste independente**: Definido para cada user story acima
