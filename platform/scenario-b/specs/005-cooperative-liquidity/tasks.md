# Tasks: Provisão Cooperativa de Liquidez no AMM

**Feature**: `005-cooperative-liquidity`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Data Model**: [data-model.md](data-model.md)
**Generated**: 2026-05-18 | **Updated**: 2026-05-20 (G5-cross — T066 concluído; T067 adicionado — doc G5-cross em token-api-fr018.md; D17-D20 em research.md)
**Total Tasks**: 67 | **Parallelizable**: 41 | **Sequential gates**: 7
**Completed**: T001-T058, T061-T067 (65/67) | **Pending**: T059, T060 (Fase 2 — pós-MVP)
**Validated**: I1 ✅ I3 ✅ I4 ✅ (visão gateway LP-owner; 0 de gateway comercial — DB isolation esperada) I2 ⚠️ (cross-gateway — limitação arquitetural documentada em FR-006/SC-006)

---

## Dependency Graph (Story Completion Order)

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational) — bloqueia todas as US
    ↓
Phase 3 (US1 — P1) ← MVP mínimo viável
    ↓ (ambas em paralelo após US1)
Phase 4 (US2 — P2)   Phase 5 (US3 — P2)
                           ↓
                      Phase 6 (US4 — P3)
    ↓ (todos convergem)
Phase 7 (Polish)      Phase 8 (US5 — P1 extension)
                           ↓ (independente das Phases 3–6,
                              depende apenas do Phase 2 foundational)
                      [PairRegistry multi-par]

── MLP PATH B (T049–T054) ─────────────────────────────────────────────────
Phase 9 (Setup MLP — env files, feature toggle)

── BUG-FIX PHASE (T055–T058) ───────────────────────────────────────────────
Phase 11 (Bug Fixes I1–I4) — independente entre si (arquivos distintos)
    T055 ‖ T056 ‖ T057 (paralelos)
        ↓
    T058 (requer LP positions no DB — depende de T056 em testes)
    ↓
Phase 10 (US2 MLP — contratos + docker + makefile + E2E)
    → T051 ‖ T052 ‖ T053  →  T054
```

**MVP Scope**: Phases 1 + 2 + 3 = pool cooperativo BRL-USD operacional com commit-reveal.  
**Extensão Multi-Par (Phase 8)**: pode ser desenvolvida em paralelo com Phases 4–7 após Phase 2, pois depende apenas dos domain structs e do IdentityRegistry.
**MLP Path B (Phases 9–10)**: infraestrutura operacional do MLP; depende de Phases 1–4 concluídas (T001–T048 ✅).

---

## Phase 1: Setup — Infraestrutura de Dados

> Objetivo: preparar o schema de banco de dados e os domain structs Go antes de qualquer lógica de negócio.

- [X] T001 Criar migration SQL `005_cooperative_liquidity.sql` com tabelas `pool_commits`, `lp_fee_events` e alterações em `liquidity_positions` e `pool_state_readings` em `backend/services/api-gateway/internal/db/migrations/005_cooperative_liquidity.sql`

- [X] T002 [P] Criar domain struct `PoolCommit` com campos `CommitID`, `PoolPair`, `ProviderID`, `Side`, `Amount`, `Status`, `CounterpartCommitID`, `CreatedAt`, `ExpiresAt` e método `IsExpired()` em `backend/services/api-gateway/internal/domain/pool_commit.go`

- [X] T003 [P] Criar domain struct `LPFeeEvent` com campos `EventID`, `PoolPair`, `SwapOrderID`, `FeeAmountA`, `FeeAmountB`, `Distribution` (map[string]string) e `CreatedAt` em `backend/services/api-gateway/internal/domain/lp_fee_event.go`

- [X] T004 Estender struct `LiquidityPosition` adicionando campos `DepositSide` (enum: `A`|`B`|`BOTH`, default `BOTH`), `SharesPercentage` (*decimal.Decimal nullable), `FeeClaimAccumulated` (big.Int), `CommitID` (*string nullable) em `backend/services/api-gateway/internal/domain/liquidity_position.go`

---

## Phase 2: Foundational — Contratos e Repositórios Base

> Objetivo: estabelecer as camadas de contrato e repositório que bloqueiam todas as user stories. Deve estar completo antes de iniciar a Phase 3.

- [X] T005 Adicionar `isLiquidityProvider(address) external view returns (bool)`, `grantLiquidityProvider(address) external onlyAdmin`, `revokeLiquidityProvider(address) external onlyAdmin`, mapping `_liquidityProviders` e eventos `LogLiquidityProviderGranted`/`LogLiquidityProviderRevoked` em `contracts/src/IdentityRegistry.sol`

- [X] T006 [P] Adicionar assinaturas `addSingleSidedLiquidity(bool isTokenA, uint256 amount) external`, `setFeeBps(uint256 newFeeBps) external`, `removeSingleSidedLiquidity(bool isTokenA, uint256 amount) external` e `feeBps() external view returns (uint256)` à interface em `contracts/src/interfaces/IAutomatedMarketMaker.sol`

- [X] T007 [P] Criar repositório Go `PoolCommitRepository` com métodos `Create(ctx, commit)`, `FindActiveByPairAndSide(ctx, poolPair, side)`, `UpdateStatus(ctx, commitID, status)`, `FindExpiredPending(ctx)` e `LinkCounterpart(ctx, commitID, counterpartID)` em `backend/services/api-gateway/internal/app/pool_commit_repository.go`

- [X] T008 [P] Criar repositório Go `LPFeeEventRepository` com métodos `Create(ctx, event)`, `SumFeesByLPID(ctx, lpID, since)` e `FindBySwapOrderID(ctx, orderID)` em `backend/services/api-gateway/internal/app/lp_fee_event_repository.go`

---

## Phase 3: US1 — Depósito Independente via Commit-Reveal (P1)

> **Objetivo**: BCB e Fed formam o pool BRL-USD depositando cada um apenas a sua própria moeda.
> **Critério de teste independente**: `POST /commit` (lado A) → pool `PENDING_COUNTERPART` → swap bloqueado → `POST /commit` (lado B) → pool `ACTIVE` → swap executado com sucesso.

- [X] T009 [US1] Adicionar função `addSingleSidedLiquidity(bool isTokenA, uint256 amount)` com modifier `onlyLiquidityProvider(msg.sender)`, novo custom error `AMM__NotLiquidityProvider()`, validação `amount > 0`, `safeTransferFrom` condicional e evento `LogSingleSidedLiquidityAdded(address indexed provider, bool isTokenA, uint256 amount)` em `contracts/src/AutomatedMarketMaker.sol`

- [X] T010 [P] [US1] Adicionar método `AddSingleSidedLiquidity(ctx, poolPair, isTokenA bool, amount *big.Int) error` que chama `ammContract.AddSingleSidedLiquidity(auth, isTokenA, amount)` via go-ethereum binding em `backend/services/api-gateway/internal/app/amm_adapter.go`

- [X] T011 [US1] Implementar lógica de commit-reveal em `LiquidityProvisionService`: método `RegisterCommit(ctx, req CommitRequest) (*PoolCommit, error)` que valida LP role, verifica unicidade de (pool_pair, side), cria `PoolCommit{status: PENDING}`, verifica existência de contraparte e — se MATCHED — chama `AMM.AddSingleSidedLiquidity` para ambos os lados e cria `LiquidityPosition{deposit_side: A|B}` para cada CB em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T012 [P] [US1] Implementar worker de expiração `PoolCommitExpiryWorker` como goroutine com ticker de 5 minutos que busca commits com `status = PENDING AND expires_at < now()` e os marca como `EXPIRED`, com graceful shutdown via context cancellation em `backend/services/api-gateway/internal/services/pool_commit_expiry.go`

- [X] T013 [P] [US1] Implementar handler `CommitLiquidity(c *fiber.Ctx) error` que valida body `LiquidityCommitRequest{PoolPair, ProviderID, Side, Amount}`, chama `svc.RegisterCommit` e retorna 201 com `LiquidityCommitResponse` ou 409 `COMMIT_ALREADY_EXISTS` em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`

- [X] T014 [P] [US1] Implementar handler `ListCommits(c *fiber.Ctx) error` que aceita query params `pool_pair` (required) e `status` (optional) e retorna lista de `LiquidityCommitResponse` em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`

- [X] T015 [P] [US1] Implementar handler `CancelCommit(c *fiber.Ctx) error` que busca commit por `:commit_id`, valida que `status = PENDING` e `provider_id` pertence ao caller, marca como `EXPIRED` e retorna 200 ou 409 `COMMIT_ALREADY_MATCHED` em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`

- [X] T016 [US1] Registrar rotas `POST /api/v2/amm/liquidity/commit`, `GET /api/v2/amm/liquidity/commits` e `DELETE /api/v2/amm/liquidity/commits/:commit_id` no router do api-gateway e iniciar `PoolCommitExpiryWorker` no bootstrap da aplicação em `backend/services/api-gateway/internal/app/app.go`

- [X] T017 [P] [US1] Adicionar schemas `LiquidityCommitRequest`, `LiquidityCommitResponse`, `CommitSide` (enum `A|B`), `PoolStatus` (enum `EMPTY|PENDING_COUNTERPART|ACTIVE`) e `DepositSide` (enum `A|B|BOTH`) e novos endpoints `POST /api/v2/amm/liquidity/commit`, `GET /api/v2/amm/liquidity/commits`, `DELETE /api/v2/amm/liquidity/commits/{commit_id}` em `backend/services/api-gateway/openapi/v2/scenario-b.yaml`

- [X] T018 [P] [US1] Atualizar handler do `GET /api/v2/amm/pool/{pool_pair}/status` para incluir campos `pool_status` (derivado de reserveA/reserveB), `fee_rate_bps`, `total_lp_count` e `pending_commits[]` na resposta em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`

- [X] T019 [US1] Adicionar chamadas `registry.grantLiquidityProvider(centralBankAAddress)` e `registry.grantLiquidityProvider(centralBankBAddress)` no script de deploy após deploy do `IdentityRegistry` em `contracts/script/CBWeb3Hub.s.sol`

---

## Phase 4: US2 — MLP como Provedor Suplementar (P2)

> **Objetivo**: MLP registrado pode depositar ambos os lados do par e remover liquidez com as mesmas garantias dos CBs.
> **Critério de teste independente**: MLP cadastrado deposita via `POST /add` (dual-sided) → LP position criado → MLP remove → receives proporção do pool atual.
> **Pode ser desenvolvido em paralelo com Phase 5 após Phase 3 concluída.**

- [X] T020 [P] [US2] Adicionar validação de papel LP nas rotas de liquidez: extrair claim `role` do JWT (Keycloak) e verificar que é `central_bank` ou `mlp`; retornar 403 `NOT_AUTHORIZED_LP` caso contrário, aplicado aos handlers de `/commit`, `/add` e `/remove` em `backend/services/api-gateway/internal/http/middleware/lp_auth.go`

- [X] T021 [P] [US2] Calcular e persistir `shares_percentage` no momento da criação de `LiquidityPosition` para depósitos MLP via `addLiquidity` (dual-sided): `shares_pct = valor_contribuicao / valor_total_pool * 100`; recalcular `shares_percentage` de todas as posições ACTIVE do mesmo pool_pair após cada novo depósito em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T022 [US2] Atualizar resposta de `POST /api/v2/amm/liquidity/add` para incluir campos `deposit_side` (`BOTH`) e `shares_percentage` (calculado); atualizar schema `LiquidityPositionResponse` no OpenAPI em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`

- [X] T023 [P] [US2] Adicionar schema e endpoint `GET /api/v2/amm/liquidity/providers` que lista todos os provedores autorizados (CBs e MLPs) com seus `lp_id`s ACTIVE e `shares_percentage` correntes em `backend/services/api-gateway/openapi/v2/scenario-b.yaml`

---

## Phase 5: US3 — Distribuição Proporcional de Taxas (P2)

> **Objetivo**: cada swap gera uma taxa de 30 bps retida no pool, distribuída proporcionalmente aos LPs ativos.
> **Critério de teste independente**: BCB (60% shares) + Fed (40% shares) → N swaps geram fees → ao remover, BCB recebe 60% das fees + reserva proporcional; Fed 40%.
> **Pode ser desenvolvido em paralelo com Phase 4 após Phase 3 concluída.**

- [X] T024 [US3] Adicionar `uint256 public feeBps = 30`, `event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps)`, `function setFeeBps(uint256 newFeeBps) external onlyPauser` com validação `newFeeBps <= 1000` (máximo 10%) e `emit LogFeeRateUpdated` em `contracts/src/AutomatedMarketMaker.sol`

- [X] T025 [US3] Modificar função `swapExactOutput` no AMM para calcular `amountInWithFee = amountIn * (10000 - feeBps) / 10000` e usar `amountInWithFee` na fórmula de produto constante `x * y = k`, mantendo `amountIn` completo na transferência (fee fica nas reservas) em `contracts/src/AutomatedMarketMaker.sol`

- [X] T026 [US3] Implementar criação de `LPFeeEvent` a cada swap bem-sucedido: calcular `fee_amount_a = amountIn * feeBps / 10000`, buscar todos os `LiquidityPosition{status: ACTIVE, pool_pair}`, montar `distribution` como snapshot percentual e persistir `LPFeeEvent` em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T027 [US3] Implementar atualização de `fee_claim_accumulated` em cada `LiquidityPosition` após criação do `LPFeeEvent`: `fee_claim += fee_total * shares_percentage / 100` (batch update por pool_pair) em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T028 [P] [US3] Atualizar resposta de `POST /api/v2/amm/liquidity/remove` para incluir `returned_token_a`, `returned_token_b`, `fee_claim_paid` e `withdrawal_mode` (`LEGACY`|`PROPORTIONAL`); atualizar schema `LiquidityRemoveResponse` no OpenAPI em `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go`

---

## Phase 6: US4 — Retirada Proporcional ao Saldo Atual do Pool (P3)

> **Objetivo**: na remoção, LP recebe `reserveX * shares_pct / 100` (saldo atual) + `fee_claim_accumulated`, não os valores originais depositados.
> **Critério de teste independente**: pool inicia 1000/1000, swaps alteram para 1200/833, LP com 100% shares remove → recebe 1200 + 833 (não 1000 + 1000).
> **Depende de Phase 5 (shares_percentage e fee_claim são rastreados lá).**

- [X] T029 [US4] Adicionar função `removeSingleSidedLiquidity(bool isTokenA, uint256 amount) external nonReentrant whenNotPaused onlyLiquidityProvider(msg.sender)` com validação `amount <= reserveA` (ou B), `safeTransfer` condicional e evento `LogSingleSidedLiquidityRemoved` em `contracts/src/AutomatedMarketMaker.sol`

- [X] T030 [P] [US4] Adicionar método `RemoveLiquidityProportional(ctx, poolPair string, amountA, amountB *big.Int) error` que chama `removeSingleSidedLiquidity` para cada lado com os valores calculados proporcionalmente em `backend/services/api-gateway/internal/app/amm_adapter.go`

- [X] T031 [US4] Implementar caminho de retirada `PROPORTIONAL` em `RemoveLiquidity`: para `deposit_side ∈ {A, B}`, calcular `returnA = reserveA * shares_pct / 100` e `returnB = reserveB * shares_pct / 100`, adicionar `fee_claim_accumulated`, chamar `amm.RemoveLiquidityProportional`, recalcular `shares_percentage` dos LPs restantes e persistir `withdrawn_at` em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T032 [P] [US4] Validar que o caminho `LEGACY` (`deposit_side = BOTH`) continua retornando `token_a_contributed` e `token_b_contributed` originais via `removeLiquidity(amountA, amountB)` existente (sem alteração de contrato para este path), adicionando teste de regressão em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

---

## Phase 7: Polish & Cross-Cutting Concerns

> Testes de contrato, testes unitários Go e atualização do script E2E. Pode ser iniciado incrementalmente após cada Phase completar seus contratos/serviços.

- [X] T033 [P] Escrever testes Forge para `IdentityRegistry`: `test_grantLP_emitsEvent`, `test_revokeLP_blocksAddLiquidity`, `test_nonAdmin_cannotGrant` em `contracts/test/IdentityRegistryLP.t.sol`

- [X] T034 [P] Escrever testes Forge para `AutomatedMarketMaker.addSingleSidedLiquidity`: `test_addSideA_updatesReserveA`, `test_addSideB_updatesReserveB`, `test_unverifiedLP_reverts`, `test_zeroAmount_reverts` em `contracts/test/AutomatedMarketMakerCooperative.t.sol`

- [X] T035 [P] Escrever testes Forge para `feeBps` em `swapExactOutput`: `test_feeDeductedFromInput`, `test_feeRetainedInReserves`, `test_setFeeBps_onlyPauser`, `test_feeBps_zeroCaseCompatible` em `contracts/test/AutomatedMarketMakerCooperative.t.sol`

- [X] T036 [P] Escrever testes Forge para `removeSingleSidedLiquidity`: `test_proportionalRemove_matchesSharesPct`, `test_legacyRemove_returnsOriginalAmounts`, `test_dualLPRemove_noResidue` em `contracts/test/AutomatedMarketMakerCooperative.t.sol`

- [X] T037 [P] Escrever testes Go unitários para `PoolCommitExpiryWorker`: `TestExpiry_marksExpiredCommits`, `TestExpiry_doesNotExpireMatched`, `TestExpiry_gracefulShutdown` em `tests/unit/pool_commit_expiry_test.go`

- [X] T038 [P] Escrever testes Go unitários para cálculo de `LPFeeEvent` e `fee_claim_accumulated`: `TestFeeDistribution_twoLPs`, `TestFeeDistribution_lateEntrant`, `TestFeeDistribution_zeroFeeBps` em `tests/unit/lp_fee_distribution_test.go`

- [X] T039 Atualizar script E2E `tryouts/tryout-scenario-b-e2e.sh` substituindo `step4b_seed_liquidity` pelo fluxo cooperativo de commit-reveal e adicionando validação de `pool_status`, `fee_rate_bps` e `withdrawal_mode = PROPORTIONAL`

---

## Phase 8: US5 — Criação Dinâmica de Par via PairRegistry (P1 — extensão multi-par)

> **Objetivo**: qualquer par de moedas pode ser criado dinamicamente por aprovação bilateral on-chain dos dois CBs emissores, sem restart do gateway.
> **Critério de teste independente**: BCB propõe BRL-ARS via `POST /pairs/propose` → BCRA confirma via `POST /pairs/confirm` → `GET /pairs` retorna BRL-ARS sem restart → commit-reveal + swap no novo corredor funcionam normalmente.
> **Pode ser desenvolvido em paralelo com Phases 4–7** após Phase 2 (requer apenas `IdentityRegistry.sol` com `getCentralBankOf` e domain structs).

- [X] T040 Adicionar `getCentralBankOf(address token) external view returns (address)` ao `IdentityRegistry.sol` com mapping `_centralBankOf` populado no `grantLiquidityProvider` e criar `contracts/src/PairRegistry.sol` com structs `PairEntry`, `PairStatus` (PROPOSED/ACTIVE), mapping `pairs`, funções `proposePair(pairId, tokenA, tokenB, ammAddress)`, `confirmPair(pairId)`, `getAllActivePairs()` e eventos `PairProposed`, `PairRegistered` em `contracts/src/PairRegistry.sol` e `contracts/src/IdentityRegistry.sol`

- [X] T041 [P] Escrever testes Forge para `PairRegistry`: `test_proposePair_emitsPairProposed`, `test_confirmPair_emitsPairRegistered`, `test_wrongCB_cannotPropose`, `test_wrongCB_cannotConfirm`, `test_duplicatePairId_reverts`, `test_getAllActivePairs_returnsOnlyActive` em `contracts/test/PairRegistry.t.sol`

- [X] T042 [P] Criar migration SQL `007_pair_proposals.sql` com `CREATE TABLE pair_proposals (pair_id VARCHAR(20) PRIMARY KEY, proposer_cb VARCHAR(100) NOT NULL, confirmer_cb VARCHAR(100), token_a_address VARCHAR(42) NOT NULL, token_b_address VARCHAR(42) NOT NULL, amm_address VARCHAR(42) NOT NULL, status VARCHAR(10) NOT NULL DEFAULT 'PROPOSED' CHECK (status IN ('PROPOSED','ACTIVE')), proposed_at TIMESTAMPTZ NOT NULL DEFAULT now(), confirmed_at TIMESTAMPTZ)` em `backend/services/api-gateway/migrations/007_pair_proposals.sql`

- [X] T043 [P] Criar domain structs `PairProposal{PairID, ProposerCB, ConfirmerCB, TokenAAddress, TokenBAddress, AMMAddress, Status, ProposedAt, ConfirmedAt}` e `PairEntry{PairID, AMMAddress, TokenA, TokenB}` com constantes `PairStatusProposed`, `PairStatusActive` em `backend/services/api-gateway/internal/domain/pair.go`

- [X] T044 [P] Criar `pair_registry_adapter.go` com struct `PairRegistryAdapter` que wraps o contrato go-ethereum-bound `PairRegistry`: métodos `ProposePair(ctx, pairID, tokenA, tokenB, ammAddress string) (txHash string, error)`, `ConfirmPair(ctx, pairID string) (txHash string, error)`, `GetAllActivePairs(ctx) ([]domain.PairEntry, error)` e `SubscribePairRegistered(ctx, ch chan<- domain.PairEntry) (sub ethereum.Subscription, error)` em `backend/services/api-gateway/internal/app/pair_registry_adapter.go`

- [X] T045 Criar `pair_repo.go` com `PairRepository` interface e implementação GORM: métodos `Create(ctx, proposal)`, `UpdateStatusActive(ctx, pairID, confirmerCB, confirmedAt)`, `FindAllActive(ctx) ([]domain.PairProposal, error)`, `FindByPairID(ctx, pairID) (*domain.PairProposal, error)` em `backend/services/api-gateway/internal/repo/pair_repo.go`; criar `pair_router.go` com struct `PairRouter{mu sync.RWMutex; clients map[string]*ammClient}` métodos `ClientFor(pairID string) (*ammClient, error)`, `Add(pairID string, ammAddr string)` e função `StartEventWatcher(ctx, adapter PairRegistryAdapter, router *PairRouter, repo PairRepository)` como goroutine que chama `adapter.SubscribePairRegistered` e ao receber evento chama `router.Add` + `repo.UpdateStatusActive` em `backend/services/api-gateway/internal/services/pair_router.go`

- [X] T046 [P] Criar `pair_handler.go` com handlers `ProposePair(c *fiber.Ctx) error` (valida `PairProposeRequest{PoolPair, TokenAAddress, TokenBAddress, AMMAddress}`, chama adapter.ProposePair, persiste em repo, retorna 201 com `{pair_id, status:"PROPOSED", tx_hash}`), `ConfirmPair(c *fiber.Ctx) error` (valida `PairConfirmRequest{PoolPair}`, chama adapter.ConfirmPair, retorna 200 com `{pair_id, status:"ACTIVE", tx_hash}`) e `ListPairs(c *fiber.Ctx) error` (consulta repo.FindAllActive, retorna 200 com lista); erros: 409 `PAIR_ALREADY_EXISTS`, 403 `NOT_CENTRAL_BANK_OF_TOKEN_A/B`, 404 `PAIR_NOT_FOUND` em `backend/services/api-gateway/internal/handlers/pair_handler.go`

- [X] T047 Registrar rotas `POST /api/v2/amm/pairs/propose`, `POST /api/v2/amm/pairs/confirm`, `GET /api/v2/amm/pairs` em `app.go`; inicializar `PairRouter` via `pair_registry_adapter.GetAllActivePairs()` no startup; iniciar goroutine `StartEventWatcher` com graceful shutdown via context; atualizar `amm_adapter.go` para receber `PairRouter` e usar `router.ClientFor(pair)` em vez de campo `ammAddr` fixo em `backend/services/api-gateway/internal/app/app.go` e `backend/services/api-gateway/internal/app/amm_adapter.go`; adicionar deploy do `PairRegistry` + `grantCentralBankOf(tCeBMa, centralBankA)` + `grantCentralBankOf(tCeBMb, centralBankB)` + `registerPair("BRL-USD", ammAddr, tCeBMa, tCeBMb)` no script `contracts/script/CBWeb3Hub.s.sol`

- [X] T048 [P] Adicionar schemas `PairProposeRequest`, `PairProposeResponse`, `PairConfirmRequest`, `PairConfirmResponse`, `PairListResponse`, `PairEntry` ao `apis/openapi/amm.yaml`; adicionar Fluxo 5 (BRL-ARS propose → confirm → GET /pairs → commit-reveal → swap) ao script `tryouts/tryout-scenario-b-e2e.sh` com checklist de 8 critérios de aceite (PROPOSED status, ACTIVE após confirm, par visível sem restart, commit-reveal no novo par, swap BRL→ARS, PAIR_ALREADY_EXISTS 409, NOT_CENTRAL_BANK 403)em `apis/openapi/amm.yaml` e `tryouts/tryout-scenario-b-e2e.sh`

---

## Parallel Execution Examples

### Paralelo dentro de Phase 1 (após T001):
```
T002 (pool_commit.go) ‖ T003 (lp_fee_event.go)
                              ↓
                           T004 (liquidity_position.go)
```

### Paralelo dentro de Phase 2 (independentes entre si):
```
T005 (IdentityRegistry.sol) ‖ T006 (IAutomatedMarketMaker.sol)
T007 (pool_commit_repo.go)  ‖ T008 (lp_fee_event_repo.go)
```

### Paralelo dentro de Phase 3 (após T009 + T010 + T011):
```
T012 (expiry worker) ‖ T013 (commit handler) ‖ T014 (list handler) ‖ T015 (cancel handler)
T017 (openapi)       ‖ T018 (pool/status)
```

### Paralelo Phase 4 + Phase 5 (após Phase 3):
```
T020 ‖ T021 ‖ T022 ‖ T023      (US2)
T024 → T025 → T026 → T027 ‖ T028   (US3, parcialmente sequencial)
```

### Paralelo Phase 7 (após cada camada concluída):
```
T033 ‖ T034 ‖ T035 ‖ T036   (forge tests — paralelo total)
T037 ‖ T038                  (go unit tests — paralelo total)
```

### Phase 8 — US5 (pode iniciar após Phase 2, em paralelo com Phases 4–7):
```
T040 (PairRegistry.sol + getCentralBankOf)
    ↓              ↓
T041 (forge tests) T044 [P] (pair_registry_adapter.go)
                       ↓
T042 [P] (migration)  T043 [P] (domain/pair.go)
    ↓                     ↓
    └────────┬────────────┘
             ↓
         T045 (pair_repo.go + pair_router.go)
             ↓
         T046 [P] (pair_handler.go)
             ↓
         T047 (app.go wiring + CBWeb3Hub.s.sol)
             ↓
         T048 [P] (openapi + E2E Fluxo 5)
```

---

## Implementation Strategy

**MVP (Phases 1–3)**: Pool cooperativo BRL-USD operacional. CBs podem registrar commits independentes, o pool ativa automaticamente quando ambos os lados estão commitados, commits expiram após 72h, e swaps são bloqueados até o pool estar ACTIVE. Entrega valor imediato de soberania monetária sem precisar de US2/US3/US4.

**Incremento 1 (Phase 4 + 5 em paralelo)**: MLP como provedor adicional + coleta e distribuição de taxas. Completa o modelo de negócio cooperativo.

**Incremento 2 (Phase 6)**: Retirada proporcional ao saldo atual. Alinha com o padrão Uniswap v2 e garante invariante de pool consistente após swaps.

**Incremento 3 (Phase 8 — Multi-Par)**: PairRegistry com aprovação bilateral. Qualquer par de moedas pode ser criado dinamicamente (BRL-ARS, BRL-EUR, etc.) sem restart do gateway. Pode ser desenvolvido em paralelo com Phases 4–7. Escopo: 1 novo contrato Solidity (`PairRegistry.sol`), 1 migration, 3 handlers, 1 goroutine event watcher, 1 adapter Go. Entregável: SC-009 — criação de par em ≤2 interações REST, visível imediatamente sem restart.

**Incremento Final (Phase 7)**: Cobertura de testes e atualização do E2E — pode ser feita incrementalmente ao longo das phases anteriores.

**MLP Path B (Phases 9–10)**: Ativa o MLP como provedor real com gateway próprio, satisfazendo FR-004 (MUST) e SC-004. Nenhum código Go novo — apenas infraestrutura (env files, Docker Compose, Makefile, Solidity, Bash). Pré-requisito: T001–T048 concluídos.

---

## Phase 9: Setup MLP — Feature Toggle e Env Files

> Objetivo: criar os arquivos de configuração que permitem ativar/desativar o MLP via `deploy/local/.env` sem necessidade de `export` manual por sessão de terminal.
>
> **Concluídas antes desta phase**: `deploy/local/keycloak/init.sh` (realm mlp ✅), `deploy/local/compose.yml` + `postgres/init-multi-db.sh` (cbweb3_mlp ✅), `contracts/script/CBWeb3Hub.s.sol` (grantLiquidityProvider ✅), `contracts/script/SeedHub.s.sol` (constante MLP_SIGNER_DEFAULT ✅ — run() pendente no T051).

- [X] T049 Criar `deploy/local/.env.example` com `ENABLE_MLP=false` e `MLP_ADDRESS=0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b` (template rastreado no git); adicionar entrada `deploy/local/.env` ao `.gitignore` na raiz do repositório em `deploy/local/.env.example` e `.gitignore`

- [X] T050 [P] Criar `backend/config/.env.infra.mlp.example` espelhando o padrão de `central-bank-b.example` com `KC_REALM=mlp`, `KC_CLIENT_ID=mlp-client`, `DB_NAME=cbweb3_mlp`, `REDIS_DB=6`, `SIGNER_PRIVATE_KEY=f8f8a2f43c8376ccb0871305060d7b27b0554d2cc72bccf41b2705608452f315`, `POSTGRES_DB_MLP=cbweb3_mlp`, `CA_CERT_FILE=/workspace/backend/config/pki/mlp-ca.crt`, `CA_KEY_FILE=/workspace/backend/config/pki/mlp-ca.key` (gerados por `make pki.gen-all` — clarificação 2026-05-18), placeholders para `HUB_TOKEN_A_ADDRESS`, `HUB_TOKEN_B_ADDRESS`, `AMM_CONTRACT_ADDRESS`, `PARTICIPANT_REGISTRY_ADDRESS` (preenchidos por `contracts.sync-addresses`) em `backend/config/.env.infra.mlp.example`

---

## Phase 10: US2 — MLP Infraestrutura Completa (P2)

> **Objetivo**: MLP registrado com identidade Ethereum própria pode autenticar via Keycloak realm `mlp` e depositar liquidez dual-sided no pool BRL-USD através de seu gateway dedicado na porta 68080.
>
> **Critério de teste independente**: `cp deploy/local/.env.example deploy/local/.env && sed -i 's/ENABLE_MLP=false/ENABLE_MLP=true/' deploy/local/.env && make scenario-b.up-infra && make scenario-b.up-backend-mlp && source backend/config/.env.infra.mlp && [obter MLP_TOKEN via KC] && POST http://localhost:68080/api/v2/amm/liquidity/add com MLP_TOKEN → resposta com lp_id não-nulo e deposit_side=BOTH`
>
> **T051 ‖ T052 ‖ T053 podem ser executados em paralelo (arquivos distintos); T054 depende de T053 para referenciar os make targets corretos.**

- [X] T051 [US2] Completar função `run()` em `contracts/script/SeedHub.s.sol`: adicionar leitura `address mlpSigner = vm.envOr("MLP_ADDRESS", MLP_SIGNER_DEFAULT)` (onde `MLP_SIGNER_DEFAULT = 0x22d491Bde2303f2f43325b2108D26f1eAbA1e32b`); incluir `mlpSigner` no array `recipients[]` para mint de tCeBMa e tCeBMb; chamar `_registerIfNeeded(hubRegistry, mlpSigner, "MLP", ParticipantRole.MLP)` após o bloco de registro dos CBs — usar `ParticipantRole.MLP` (não `CENTRAL_BANK`) conforme FR-004 e clarificação 2026-05-18; se o enum `ParticipantRole` não tiver o valor `MLP`, adicioná-lo antes desta chamada em `contracts/script/SeedHub.s.sol` e no contrato/enum onde `ParticipantRole` é definido

- [X] T052 [P] [US2] Criar `backend/docker-compose-backend.mlp.yaml` com 4 serviços espelhando o padrão de `docker-compose-backend.central-bank-b.yaml`: `compliance-mlp` (container `backend-compliance-mlp`, porta `68093:9093`, env `DB_NAME=cbweb3_mlp`), `auth-mlp` (container `backend-auth-mlp`, porta `68091:9091`, envs `KEYCLOAK_REALM=mlp` `KEYCLOAK_CLIENT_ID=mlp-client` `REDIS_DB=6`), `payment-orchestrator-mlp` (container `backend-payment-orchestrator-mlp`, porta `68094:9094`), `api-gateway-mlp` (container `backend-api-gateway-mlp`, porta `68080:8080`, envs `SIGNER_PRIVATE_KEY=f8f8a2f43c8376ccb0871305060d7b27b0554d2cc72bccf41b2705608452f315` `HUB_BESU_RPC_URL=http://host.docker.internal:8645`); todos com `env_file: - path: ./config/.env.infra.mlp required: true`; networks `cbweb3_backend_mlp` (nova) + `cbweb3_network` (external) em `backend/docker-compose-backend.mlp.yaml`

- [X] T053 [P] [US2] Atualizar `make/60-scenario-b.mk`: adicionar no topo do arquivo `-include deploy/local/.env` e `export ENABLE_MLP` e `export MLP_ADDRESS`; adicionar target `scenario-b.up-backend-mlp` que executa `docker compose -f $(BACKEND_DIR)/docker-compose-backend.mlp.yaml up -d`, target `scenario-b.down-backend-mlp` que executa `down --remove-orphans`, target `scenario-b.tryout-us2-mlp` que executa `ENABLE_MLP=true bash tryouts/tryout-scenario-b-e2e.sh` em `make/60-scenario-b.mk`

- [X] T054 [US2] Atualizar `tryouts/tryout-scenario-b-e2e.sh`: adicionar leitura de variáveis `ENABLE_MLP` (default `false`), `API_GW_MLP_URL` (default `http://localhost:68080`), e `KC_MLP_CLIENT_SECRET` (sourced de `backend/config/.env.infra.mlp` se o arquivo existir); implementar função `step_mlp_us2()` que: (1) obtém `MLP_TOKEN` via `curl -s -d "client_id=mlp-client&client_secret=${KC_MLP_CLIENT_SECRET}&grant_type=client_credentials" http://localhost:8081/realms/mlp/protocol/openid-connect/token | jq -r '.access_token'`; (2) chama `POST $API_GW_MLP_URL/api/v2/amm/liquidity/add` com Authorization Bearer e body `{"pool_pair":"BRL-USD","token_a_amount":"10000","token_b_amount":"10000","provider_bank_id":"mlp"}`; (3) valida `lp_id` não-nulo e `deposit_side=BOTH` via `jq`; adicionar dispatch no passo correspondente ao US2: `if [[ "${ENABLE_MLP:-false}" == "true" ]]; then step_mlp_us2; fi` em `tryouts/tryout-scenario-b-e2e.sh`

---

## Parallel Execution Examples (MLP Path B)

### Phase 9 — Paralelo total (arquivos distintos):
```
T049 (deploy/local/.env.example + .gitignore)  ‖  T050 (.env.infra.mlp.example)
```

### Phase 10 — Paralelo T051/T052/T053, depois T054:
```
T051 (SeedHub.s.sol run()) ‖ T052 (docker-compose-backend.mlp.yaml) ‖ T053 (60-scenario-b.mk)
                                                    ↓
                                          T054 (tryout-scenario-b-e2e.sh)
```

---

## Phase 11: Bug-Fix I1–I4 (branch fix-005-cooperative-liquidity)

> **Objetivo**: Corrigir 4 bugs confirmados no tryout de 2026-05-19. Sem novos endpoints, sem mudança de schema. Cada fix é cirúrgico (≤20 linhas).
>
> **Critério de teste independente**: Executar `./tryouts/tryout-scenario-b-e2e.sh` e verificar:
> - `pool_status = PENDING_COUNTERPART` após Commit A (I3)
> - `total_lp_count = 2` após commit-reveal (I4)
> - `token_a_amount ≈ 50507`, `token_b_amount ≈ 49500` na remoção (I1)
> - `fee_claim_paid > 0` na remoção após swap (I2)
>
> **T055 ‖ T056 ‖ T057 podem ser executados em paralelo (arquivos distintos); T058 requer T056 estável para teste e2e completo.**

- [X] T055 [P] Fix `pool_status_service.go`: no método `GetPoolStatus`, após a chamada ao enricher que popula `pendingCommits`, adicionar bloco `if poolStatus == "EMPTY" && len(pendingCommits) > 0 { poolStatus = "PENDING_COUNTERPART" }` — resolve Bug I3 (FR-001, US1 Scenario 4) em `backend/services/api-gateway/internal/services/pool_status_service.go`

- [X] T056 [P] Fix `executeMatchedCommits` em `liquidity_provision_service.go`: (a) substituir `_ = s.db.Transaction(...)` por `if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { ... }); err != nil { return nil, fmt.Errorf("executeMatchedCommits tx: %w", err) }`; (b) adicionar campo `Status: apidomain.LPStatusActive` em ambas as structs `posA` e `posB` — resolve Bug I4 (SC-007) em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T057 [P] Fix `removeProportional` em `liquidity_provision_service.go`: (a) adicionar método `GetPoolReserves(ctx context.Context, pair string) (reserveA, reserveB string, err error)` à interface `AMLiquidityAdder` em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`; (b) implementar `GetPoolReserves` em `ammAdapter` reutilizando o binding existente de `getReserves` em `backend/services/api-gateway/internal/adapters/amm_adapter.go`; (c) substituir o bloco `s.db.Where("pool_pair = ?")...First(&latest)` em `removeProportional` por `reserveAStr, reserveBStr, err := s.amm.GetPoolReserves(ctx, pos.PoolPair)` — resolve Bug I1 (FR-007, SC-002) em `backend/services/api-gateway/internal/services/liquidity_provision_service.go`

- [X] T058 Fix fee distribution no swap service: (a) criar interface `SwapFeeRecorder { RecordSwapFee(ctx context.Context, poolPair, swapOrderID string, feeAmountA, feeAmountB *big.Int) error }` em `backend/services/api-gateway/internal/services/swap_service.go`; (b) adicionar campo `feeRecorder SwapFeeRecorder` ao `SwapService` e método `WithFeeRecorder(r SwapFeeRecorder) *SwapService`; (c) após confirmação de swap COMPLETED, calcular `feeAmountA = amountIn × feeBps / 10000` e chamar `s.feeRecorder.RecordSwapFee(...)` (erro → log warning, não bloqueia 200); (d) em `backend/services/api-gateway/internal/app/app.go`, chamar `swapSvc.WithFeeRecorder(liquiditySvc)` no bootstrap — resolve Bug I2 (FR-006, SC-006)

---

## Parallel Execution Examples (Bug-Fix Phase)

### Phase 11 — Paralelo T055/T056/T057, depois T058:
```
T055 (pool_status_service.go)  ‖  T056 (executeMatchedCommits)  ‖  T057 (AMLiquidityAdder + ammAdapter + removeProportional)
                                                ↓
                                  T058 (swap_service.go + app.go)
                                                ↓
                                  Re-run ./tryouts/tryout-scenario-b-e2e.sh
```

---

## Phase 12: Future Work (Fase 2 — fora do escopo MVP)

> Tarefas identificadas após validação em produção (tryout 2026-05-20) e sessão de clarificação de 2026-05-20. Não bloqueiam a entrega do MVP.

- [ ] T059 [US1/FR-014] Implementar retry com backoff exponencial (5 tentativas: 2/4/8/16/32s, cap 60s) para execução parcial de commits MATCHED: quando `AddSingleSidedLiquidity` do CB_B falhar após sucesso do CB_A, retentar automático; ao esgotar tentativas, transitar ambos os commits para novo status `RECONCILIATION_REQUIRED` (requer migration SQL + enum `CommitStatus`), emitir alerta de operador via log estruturado e métrica Prometheus; reutilizar padrão de retry do `RelayerQueueItem` existente no Relayer Cacti — resolve FR-014 (Post-MVP / Fase 2)

- [ ] T060 [SC-003] Implementar benchmark automatizado de latência de swap: criar script `tests/performance/swap-latency.sh` usando `hey` ou `k6` que dispara 100 requests de swap concorrentes, coleta p50/p95/p99 e falha se p95 > 6000ms; integrar como target `make scenario-b.perf-test` executado manualmente (não em CI de PR por depender do stack completo rodando) — valida SC-003 de forma automatizada (Post-MVP / Fase 2)

---

## Phase 13: FR-018 — Payload Redesign de mint-and-approve / approve-amm

> Originado na clarificação `/speckit.clarify` de 2026-05-19. Implementar nesta branch (005). Sem alteração de contrato Solidity, DB schema ou gRPC.

- [X] T061 [FR-018-A] `backend/shared/blockchain/scenariob/tcebm/client.go` — adicionar entrada `hasRole` ao `ABIJSON` (`{"type":"function","name":"hasRole","stateMutability":"view","inputs":[{"name":"role","type":"bytes32"},{"name":"account","type":"address"}],"outputs":[{"name":"","type":"bool"}]}`) e implementar método `HasCentralBankRole(ctx context.Context) (bool, error)` que calcula `CENTRAL_BANK_ROLE = crypto.Keccak256Hash([]byte("CENTRAL_BANK_ROLE"))` e chama `hasRole` via `eth_call` retornando o bool

- [X] T062 [FR-018-B] `backend/services/api-gateway/internal/app/amm_adapter.go` — refatorar `tokenPrepareAdapter`: (a) adicionar campos `sideIsA, isCB bool`; (b) criar função `NewTokenPrepareAdapter(ctx context.Context, tokenA, tokenB *tcebmclient.Client, ammAddr string) (*tokenPrepareAdapter, error)` que chama `HasCentralBankRole` em ambos os tokens no init e popula os campos; (c) reescrever `MintAndApproveForAMM(ctx, amount string) error` usando `sideIsA`; (d) reescrever `MintToForAMM(ctx, recipient, amount string) error` usando `sideIsA`; (e) reescrever `ApproveAMM(ctx, amount, side string) error` respeitando `side` explícito quando presente, senão usando `sideIsA`

- [X] T063 [FR-018-C] `backend/services/api-gateway/internal/http/handlers/token_handler.go` — (a) atualizar `AMMTokenPreparer` interface com as 3 novas assinaturas (`MintAndApproveForAMM(ctx, amount)`, `MintToForAMM(ctx, recipient, amount)`, `ApproveAMM(ctx, amount, side)`); (b) handler `MintAndApprove`: novo request struct `{Amount, Recipient string}`, detectar campos depreciados `amount_a`/`amount_b` via raw JSON e retornar HTTP 400; (c) handler `ApproveAMM`: novo request struct `{Amount, Side string}`, detectar campos depreciados → 400, validar `side` ∈ {"", "A", "B"} → 400 se inválido; (d) responses ecoam `amount` (e `side` se present)

- [X] T064 [FR-018-D] `backend/services/api-gateway/internal/app/app.go` — substituir construção inline `&tokenPrepareAdapter{tokenA: ..., tokenB: ..., ammAddr: ...}` por chamada a `NewTokenPrepareAdapter(ctx, tokenA, tokenB, ammAddr)` e propagar o erro de inicialização

- [X] T065 [FR-018-E] `tryouts/tryout-scenario-b-e2e.sh` — atualizar todos os payloads de chamadas `mint-and-approve` e `approve-amm`: (a) CB-A: `{"amount_a":"...","amount_b":"0"}` → `{"amount":"..."}`; (b) CB-B: `{"amount_a":"0","amount_b":"..."}` → `{"amount":"..."}`; (c) chamadas com `recipient`: remover campos `amount_b`/`amount_a` depreciados; (d) chamadas `approve-amm` de banco comercial: adicionar `"side":"A"` ou `"side":"B"` conforme o token a vender

---

## Phase 14: G5-cross — Padrão de Execução Single-Gateway (Session 2026-05-20)

> Originado no tryout failure: `addSingleSidedLiquidity TOKEN_B: transaction reverted` após FR-018.
> G5-cross resolve a restrição de single-gateway matching: CB-A's signer precisa de TOKEN_B balance
> + AMM approval antes de `executeMatchedCommits`. Sem mudança de código Go ou contratos Solidity.

- [X] T066 [G5-cross-A] `tryouts/tryout-scenario-b-e2e.sh` — step4a: adicionar bloco G5-cross após o mint+approve do CB-B: (a) derivar endereço do signer CB-A via `cast wallet address --private-key "0x${cb_a_key#0x}"` (lendo `SIGNER_PRIVATE_KEY` de `backend/config/.env.infra.central-bank-a`); (b) CB-B gateway → `api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/amm/token/mint-and-approve" '{"amount":"200000","recipient":"<cb_a_addr>"}'`; (c) CB-A gateway → `api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/token/approve-amm" '{"amount":"200000","side":"B"}'`; (d) ambas as chamadas guardadas antes do commit-reveal para garantir balance+approval ao tempo de `executeMatchedCommits` — resolve RECONCILIATION_REQUIRED em `addSingleSidedLiquidity(TOKEN_B)` (finding I1/U1, D17 em research.md)

- [X] T067 [P] [G5-cross-B] `specs/005-cooperative-liquidity/contracts/token-api-fr018.md` — adicionar seção "Banco Central com `side` explícito (padrão G5-cross)" na documentação do endpoint `POST /api/v2/amm/token/approve-amm`: (a) exemplo de request `{"amount":"200000","side":"B"}` com CB-A autenticado; (b) nota explicando que CBs podem sobrescrever a auto-detecção quando possuem tokens da contraparte via mint-to (G5-cross); (c) response exemplo `{"status":"ok","amount":"200000","side":"B"}`; (d) atualizar a nota "side é opcional para CBs" para mencionar explicitamente o caso G5-cross como motivação — alinha com FR-018 (spec.md última nota) e D20 (research.md)
