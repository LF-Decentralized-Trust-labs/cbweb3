# Tasks: Hub Currency Registry

**Branch**: `006-hub-currency-registry` | **Date**: 2026-05-20  
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)  
**Total tasks**: 15 | **Parallel opportunities**: 7

---

## Phase 1 — Setup

> Project initialization and scaffolding. No user story dependency.

- [X] T001 Add `CURRENCY_REGISTRY_CONTRACT_ADDRESS` env var comment to `backend/config/.env.infra.central-bank-a.example`, `backend/config/.env.infra.central-bank-b.example`, and all other `backend/config/.env.infra.central-bank-*.example` files under the `# Scenario B` block, with an empty value and comment `# CurrencyRegistry — hub currency discovery contract (006-hub-currency-registry)`
- [X] T002 [P] Implement Foundry deploy script `contracts/script/DeployCurrencyRegistry.s.sol` that reads `IDENTITY_REGISTRY_ADDRESS` from env, deploys `CurrencyRegistry(identityRegistry)`, and logs the deployed address; model after existing deploy scripts in `contracts/script/`

---

## Phase 2 — Foundational

> These tasks are blocking prerequisites for all user story phases. Must complete before Phase 3+.

- [X] T003 [P] Implement `contracts/src/interfaces/ICurrencyRegistry.sol` with `CurrencyEntry` struct (symbol, countryName, tokenAddress, proposerCB), 4 function signatures (registerCurrency, removeCurrency, getCurrency, getAllCurrencies), `CurrencyRegistered` and `CurrencyRemoved` events, and 6 custom errors (AlreadyExists, TokenAlreadyRegistered, NotFound, Unauthorized, ZeroAddress, EmptyString) as defined in `specs/006-hub-currency-registry/contracts/hub-currency-api.md`
- [X] T004 Implement `contracts/src/CurrencyRegistry.sol` that: (1) imports and implements `ICurrencyRegistry`, (2) stores `IIdentityRegistry public immutable REGISTRY` set in constructor, (3) uses dual-mapping storage `mapping(bytes32 => CurrencyEntry) _bySymbolKey` + `mapping(address => bytes32) _tokenToSymbolKey` + `mapping(bytes32 => bool) _symbolExists` + `string[] _symbols` for iteration, (4) implements `registerCurrency` with auth check `getCentralBankOf(tokenAddress) == msg.sender`, duplicate symbol check, duplicate tokenAddress check, (5) implements `removeCurrency` with tombstone pattern (`_symbolExists[key] = false` + delete tokenToSymbol), (6) implements `getCurrency` and `getAllCurrencies` skipping tombstoned entries — model the storage and iteration pattern after `contracts/src/PairRegistry.sol`
- [X] T005 [P] Implement `backend/services/api-gateway/internal/domain/currency.go` with `CurrencyEntry` struct (`Symbol`, `CountryName`, `TokenAddress`, `ProposerCB` all `string`) in package `domain`
- [X] T006 Implement `backend/services/api-gateway/internal/app/currency_registry_adapter.go` with: (1) `currencyRegistryABI` constant (inline JSON ABI for registerCurrency, removeCurrency, getCurrency, getAllCurrencies matching `ICurrencyRegistry.sol`), (2) `CurrencyRegistryClient` struct (contract address, ethclient, parsed ABI, signer, timeout), (3) `CurrencyRegistryConfig` struct (RPCURL, ContractAddress, SignerKey, Timeout), (4) `NewCurrencyRegistryClient` constructor, (5) `RegisterCurrency(ctx, symbol, countryName, tokenAddress, proposerCB string) (txHash string, err error)`, (6) `RemoveCurrency(ctx, symbol string) (txHash string, err error)`, (7) `GetAllCurrencies(ctx) ([]domain.CurrencyEntry, error)` — model the entire file after `backend/services/api-gateway/internal/app/pair_registry_adapter.go`

---

## Phase 3 — User Story 1 + 3: Register & Discover Currencies (Priority: P1)

**Story goal**: A CB can register its currency on the hub and any CB can list all registered currencies.  
**US3 is fulfilled by this phase**: the `GET /api/v2/hub/currencies` endpoint delivers all data needed to populate a pair proposal.  
**Independent test**: `POST /api/v2/hub/currencies` → `GET /api/v2/hub/currencies` → verify entry present.

- [X] T007 [P] [US1] Write `contracts/test/CurrencyRegistry.t.sol` covering: (a) `test_registerCurrency_success` — registers a currency and verifies `getCurrency` returns correct fields; (b) `test_registerCurrency_duplicateSymbol_reverts`; (c) `test_registerCurrency_duplicateToken_reverts`; (d) `test_registerCurrency_unauthorized_reverts` (caller not `getCentralBankOf(tokenAddress)`); (e) `test_getAllCurrencies_returnsRegistered` — registers two currencies and asserts both appear in `getAllCurrencies()`
- [X] T008 [US1] Implement `backend/services/api-gateway/internal/services/currency_service.go` with: (1) `CurrencyRegisterRequest` struct (Symbol, CountryName, TokenAddress, ProposerCB strings), (2) `CurrencyRegisterResult` struct (Symbol, TxHash strings), (3) `CurrencyServiceIface` interface (RegisterCurrency, RemoveCurrency, ListCurrencies), (4) `CurrencyRegistryClientIface` interface (RegisterCurrency, RemoveCurrency, GetAllCurrencies), (5) `CurrencyService` struct with `client CurrencyRegistryClientIface`, (6) `RegisterCurrency` method that validates non-empty fields and calls `client.RegisterCurrency`, (7) `ListCurrencies` method that calls `client.GetAllCurrencies` and returns `[]domain.CurrencyEntry`, (8) error sentinels `ErrCurrencyAlreadyExists`, `ErrTokenAlreadyRegistered`, `ErrCurrencyNotFound`, `ErrCurrencyUnauthorized` — model after `backend/services/api-gateway/internal/services/pair_service.go`
- [X] T009 [US1] Implement `backend/services/api-gateway/internal/http/handlers/currency_handler.go` with: (1) `CurrencyHandler` struct + `NewCurrencyHandler(svc CurrencyServiceIface)`, (2) `RegisterCurrency` handler for `POST /api/v2/hub/currencies` — parse body, validate required fields, call `svc.RegisterCurrency`, return 201 on success, (3) `ListCurrencies` handler for `GET /api/v2/hub/currencies` — call `svc.ListCurrencies`, return 200 with `{"currencies": [...]}`, (4) `currencyErrorResponse` helper mapping error sentinels to HTTP 409/403/404/500 — model after `backend/services/api-gateway/internal/http/handlers/pair_handler.go`
- [X] T010 [US1] Wire up CurrencyRegistry in `backend/services/api-gateway/internal/app/app.go`: (1) read `CURRENCY_REGISTRY_CONTRACT_ADDRESS` env var, (2) initialize `CurrencyRegistryClient` using `NewCurrencyRegistryClient` with hub RPC + signer key (model after `NewPairRegistryClient` block at line ~295), (3) create `CurrencyService` and `CurrencyHandler`, (4) register `POST /api/v2/hub/currencies` and `GET /api/v2/hub/currencies` routes on the Fiber app (model after pair routes registration)

---

## Phase 4 — User Story 2: Remove Currency (Priority: P2)

**Story goal**: A CB can remove its own currency from the hub registry.  
**Independent test**: `POST /api/v2/hub/currencies` (register) → `DELETE /api/v2/hub/currencies/BRL` → `GET /api/v2/hub/currencies` → verify entry absent.

- [X] T011 [P] [US2] Add removeCurrency test cases to `contracts/test/CurrencyRegistry.t.sol`: (a) `test_removeCurrency_success` — registers then removes, asserts `getCurrency` reverts with `NotFound`; (b) `test_removeCurrency_notFound_reverts`; (c) `test_removeCurrency_unauthorized_reverts` (different caller); (d) `test_removeCurrency_symbolFreeAfterRemove` — after remove, same symbol can be re-registered; (e) `test_removeCurrency_tokenFreeAfterRemove` — after remove, same tokenAddress can be re-registered
- [X] T012 [US2] Add `RemoveCurrency` method to `backend/services/api-gateway/internal/services/currency_service.go`: `RemoveCurrencyRequest` struct (Symbol string), `RemoveCurrencyResult` struct (Symbol, TxHash strings), `RemoveCurrency(ctx, req RemoveCurrencyRequest) (*RemoveCurrencyResult, error)` method that validates Symbol non-empty and calls `client.RemoveCurrency`, mapping on-chain errors to `ErrCurrencyNotFound` / `ErrCurrencyUnauthorized`
- [X] T013 [US2] Add `DELETE /api/v2/hub/currencies/:symbol` handler in `backend/services/api-gateway/internal/http/handlers/currency_handler.go` (`RemoveCurrency` method reads `:symbol` param, calls `svc.RemoveCurrency`, returns 200 on success); register the route in `backend/services/api-gateway/internal/app/app.go` alongside the existing POST/GET routes

---

## Phase 5 — User Story 4: FR-011 Token-Pair Duplicate Guard + SideA/SideB (Priority: P3)

**Story goal**: Prevent proposing a pair if the same token combination (A+B or B+A) already exists in PROPOSED/ACTIVE state. Formally document proposer_cb ≡ SideA, confirmer_cb ≡ SideB.  
**Independent test**: Propose BRL-EUR → attempt to propose EUR-BRL (same tokens, reversed order) → expect 409 PAIR_ALREADY_EXISTS.

- [X] T014 [US4] Add `FindByTokenPair(ctx context.Context, tokenA, tokenB string) (*domain.PairProposal, error)` method to `backend/services/api-gateway/internal/app/pair_repo.go`: query `pair_proposals` WHERE `(token_a_address = $1 AND token_b_address = $2) OR (token_a_address = $2 AND token_b_address = $1)` AND `status IN ('PROPOSED', 'ACTIVE')` LIMIT 1; add `FindByTokenPair` to `PairRepositoryIface` in `backend/services/api-gateway/internal/services/pair_service.go`
- [X] T015 [US4] Update `ProposePair` in `backend/services/api-gateway/internal/services/pair_service.go` to call `s.repo.FindByTokenPair(ctx, req.TokenAAddress, req.TokenBAddress)` immediately after the existing `FindByPairID` check; if a record is found, return `fmt.Errorf("%w: a pair for this token combination already exists with status %s", ErrPairAlreadyExists, existing.Status)`; add a code comment documenting that `ProposerCB` ≡ SideA and `ConfirmerCB` ≡ SideB per FR-008/009/010

---

## Phase 6 — Polish & Cross-Cutting

> Final hardening and validation pass.

- [X] T016 [P] Run `forge build` in `contracts/` to verify `CurrencyRegistry.sol` and `ICurrencyRegistry.sol` compile cleanly, and `forge test --match-contract CurrencyRegistry` to confirm all Foundry test cases pass; fix any compilation errors before marking complete

---

## Dependencies

```
T001 ──────────────────────────────────────────────────────────► (none)
T002 ──────────────────────────────────────────────────────────► T004 (contract must exist to deploy)
T003 ──────────────────────────────────────────────────────────► T004, T006
T004 ──────────────────────────────────────────────────────────► T007, T011, T016
T005 ──────────────────────────────────────────────────────────► T006, T008
T006 ──────────────────────────────────────────────────────────► T008, T010
T007 ──────────────────────────────────────────────────────────► T016
T008 ──────────────────────────────────────────────────────────► T009, T010, T012
T009 ──────────────────────────────────────────────────────────► T010
T010 ──────────────────────────────────────────────────────────► (US1 complete)
T011 ──────────────────────────────────────────────────────────► T016
T012 ──────────────────────────────────────────────────────────► T013
T013 ──────────────────────────────────────────────────────────► (US2 complete)
T014 ──────────────────────────────────────────────────────────► T015
T015 ──────────────────────────────────────────────────────────► (US4 complete)
T016 ──────────────────────────────────────────────────────────► (Polish complete)
```

### User Story Completion Order

```
Phase 2 (T003–T006) must complete before any Phase 3+ tasks.

Phase 3 (US1+US3): T003 → T004 → T005 → T006 → T008 → T009 → T010  [+ T007 parallel]
Phase 4 (US2):     Phase 3 complete → T012 → T013                     [+ T011 parallel]
Phase 5 (US4):     T014 → T015  (independent from US1/US2 after Phase 2)
Phase 6 (Polish):  T007 + T011 → T016
```

---

## Parallel Execution Examples

### Sprint 1: Foundational (run simultaneously)

```bash
# Worker 1: Solidity interface + contract
vim contracts/src/interfaces/ICurrencyRegistry.sol  # T003
vim contracts/src/CurrencyRegistry.sol               # T004

# Worker 2: Go domain + env vars
vim backend/services/api-gateway/internal/domain/currency.go  # T005
vim backend/config/.env.infra.central-bank-*.example          # T001

# Worker 3: Deploy script
vim contracts/script/DeployCurrencyRegistry.s.sol             # T002
```

### Sprint 2: US1 implementation (after T003–T006 complete)

```bash
# Worker 1: Foundry tests
vim contracts/test/CurrencyRegistry.t.sol  # T007

# Worker 2: Go service + handler
vim backend/.../services/currency_service.go  # T008
vim backend/.../handlers/currency_handler.go  # T009
```

### Sprint 3: US2 + US4 (parallel)

```bash
# Worker 1: Remove tests + service
vim contracts/test/CurrencyRegistry.t.sol          # T011 (append)
vim backend/.../services/currency_service.go        # T012 (append)

# Worker 2: FR-011 duplicate guard
vim backend/.../app/pair_repo.go                    # T014
vim backend/.../services/pair_service.go            # T015
```

---

## Implementation Strategy

**MVP Scope (US1 only — T001–T010)**: Deploy `CurrencyRegistry`, register a currency, list currencies. Delivers the full discovery value without remove or FR-011. Can be demonstrated end-to-end with the quickstart guide.

**Increment 2 (add US2 — T011–T013)**: Add remove capability. Low-risk addition.

**Increment 3 (add US4 — T014–T015)**: FR-011 duplicate pair guard + SideA/SideB documentation. Independent of US1/US2 after foundational phase.

**Format validation**: All 16 tasks follow `- [ ] T### [P]? [US#]? Description with file path` format. ✅
