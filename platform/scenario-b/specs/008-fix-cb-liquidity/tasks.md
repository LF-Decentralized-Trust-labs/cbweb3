# Tasks: Fix CB Liquidity (Frontend Alignment Only)

**Input**: Design documents from `/specs/008-fix-cb-liquidity/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/frontend-scenario-b-alignment.md, quickstart.md

**Validation**: Verification tasks include lint, type-check, and manual Scenario B plus Scenario A non-regression checks.

**Organization**: Tasks are grouped by user story to enable independent implementation and validation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: User story mapping (`[US1]`, `[US2]`, `[US3]`)
- All tasks include concrete file targets

## Phase 1: Setup (Frontend Scope)

**Purpose**: Prepare frontend workspaces and scenario-specific task scaffolding.

- [X] T001 Align feature verification checklist and commands in `specs/008-fix-cb-liquidity/quickstart.md`
- [X] T002 Confirm frontend workspace scripts for lint/type-check in `frontend/package.json`
- [X] T003 [P] Capture Scenario B UX alignment notes for implementers in `specs/008-fix-cb-liquidity/plan.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared frontend foundations that MUST be complete before user stories.

**CRITICAL**: No user story implementation starts before this phase completes.

- [X] T004 Normalize Scenario B gating constants and helpers in `frontend/apps/bank/src/config/scenario.ts`
- [X] T005 [P] Normalize Scenario B gating constants and helpers in `frontend/apps/governance/src/config/scenario.ts`
- [X] T006 Implement Scenario B route guards for bank-only surfaces in `frontend/apps/bank/src/routes/index.tsx`
- [X] T007 [P] Implement Scenario B route guards for governance-only surfaces in `frontend/apps/governance/src/routes/index.tsx`
- [X] T008 Enforce sidebar role separation for bank surfaces in `frontend/apps/bank/src/components/layout/Sidebar.tsx`
- [X] T009 [P] Enforce sidebar role separation for governance surfaces in `frontend/apps/governance/src/components/layout/Sidebar.tsx`
- [X] T010 Define canonical Scenario B operational error types for bank in `frontend/apps/bank/src/types/amm-v2.types.ts`
- [X] T011 [P] Define canonical Scenario B operational error types for governance in `frontend/apps/governance/src/types/liquidity.types.ts`
- [X] T011-A [US1] [P] Implement LP position repository query methods (FindByPoolPair, FindByProviderAndPoolPair) in `backend/services/api-gateway/internal/app/lp_position_repository.go` (FR-028)
- [X] T011-B [US1] Implement ListPositions handler and wire LPPositionRepo dependency in `backend/services/api-gateway/internal/http/handlers/liquidity_handler.go` (FR-028)
- [X] T011-C [US1] [P] Register route `GET /liquidity/positions` in `backend/services/api-gateway/internal/http/router/v2/router.go` and update OpenAPI spec in `apis/openapi/amm.yaml` (FR-028)

**Checkpoint**: Foundation ready; US1/US2/US3 can start.

---

## Phase 3: User Story 1 - CB Sovereign Liquidity UX (Priority: P1) 🎯 MVP

**Goal**: Governance app follows the canonical sovereign flow (`LOCK_MINT -> BRIDGE_ACTIVE -> COMMIT_PENDING/EXECUTED -> POOL_ACTIVE`) and blocks legacy G5-cross behavior.

**Independent Validation**: In Scenario B, a CB operator can finish sovereign liquidity flow with bridge/commit/pool tracking, sees correct wait/error states, and cannot proceed through legacy cross-CB mint UX.

### Implementation for User Story 1

- [X] T012 [P] [US1] Implement sovereign flow phase model and timers in `frontend/apps/governance/src/types/liquidity.types.ts`
- [X] T013 [US1] Implement bridge-active polling policy (5s, 120s timeout) in `frontend/apps/governance/src/services/api/liquidity.api.ts`
- [X] T014 [US1] Implement commit execution polling policy (3s, warn >30s, timeout 60s) in `frontend/apps/governance/src/services/api/liquidity.api.ts`
- [X] T015 [US1] Handle `CROSS_CB_MINT_PROHIBITED` as definitive legacy-block error in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
- [X] T016 [US1] Handle `BRIDGE_POSITION_NOT_ACTIVE` with blocked progression and wait state in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
- [X] T017 [US1] Render canonical 4-phase sovereign UX and actionable error guidance in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`
- [X] T018 [US1] Surface pool activation verification (`pool_status=ACTIVE`) in `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`

**Checkpoint**: US1 fully functional and independently verifiable.

---

## Phase 4: User Story 2 - Commercial Swap UX (Priority: P1)

**Goal**: Bank app enforces pool/circuit preconditions and provides consistent quote/swap error UX for commercial operators.

**Independent Validation**: In Scenario B, commercial operator can run quote->approve->swap when pool is active; otherwise receives deterministic actionable errors and blocked submit behavior.

### Implementation for User Story 2

- [X] T019 [P] [US2] Standardize commercial swap error contracts in `frontend/apps/bank/src/types/amm-v2.types.ts`
- [X] T020 [US2] Enforce pre-check for `pool_status != ACTIVE` before swap submission in `frontend/apps/bank/src/features/amm/amm-v2.store.ts`
- [X] T021 [US2] Enforce circuit-breaker halted blocking for swap submission in `frontend/apps/bank/src/features/amm/amm-v2.store.ts`
- [X] T022 [US2] Implement quote refresh window (10-15s) and stale handling in `frontend/apps/bank/src/services/api/amm-v2.api.ts`
- [X] T023 [US2] Map `POOL_NOT_ACTIVE`, `SLIPPAGE_LIMIT_EXCEEDED`, `INSUFFICIENT_POOL_LIQUIDITY`, `CIRCUIT_BREAKER_HALTED` to actionable UX in `frontend/apps/bank/src/pages/AMMTradingPage.tsx`
- [X] T024 [US2] Integrate circuit breaker status preflight into swap readiness in `frontend/apps/bank/src/services/api/circuit-breaker-status.api.ts`
- [X] T025 [US2] Align commercial bridge UX with tryout flow (lock-mint, wait ACTIVE via positions polling, burn-unlock, wait BURNED/UNLOCKED) in `frontend/apps/bank/src/features/bridge/BridgePage.tsx`, `frontend/apps/bank/src/features/bridge/bridge.store.ts`, and `frontend/apps/bank/src/services/api/bridge.api.ts`
- [X] T026 [US2] Ensure explicit approve-amm UX step and state wiring in quote->approve->swap journey in `frontend/apps/bank/src/pages/AMMTradingPage.tsx`, `frontend/apps/bank/src/features/amm/amm-v2.store.ts`, and `frontend/apps/bank/src/services/api/amm-v2.api.ts`

**Checkpoint**: US2 fully functional and independently verifiable.

---

## Phase 5: User Story 3 - Governance Observability UX (Priority: P2)

**Goal**: Governance operators can clearly monitor pool/liquidity progression, pending commits, and operational blockers without backend logs.

**Independent Validation**: Governance screen clearly shows commit/pool/circuit states, transitions, and next recommended action for each operational state.

### Implementation for User Story 3

- [X] T027 [P] [US3] Normalize governance operational status models in `frontend/apps/governance/src/types/circuit-breaker-v2.types.ts`
- [X] T028 [US3] Expose commit/pool/bridge observability selectors and derived states in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`
- [X] T029 [US3] Render actionable status cards and transition hints in `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`
- [X] T030 [US3] Align circuit-breaker status polling and last-known-state behavior in `frontend/apps/governance/src/services/api/circuit-breaker-v2.api.ts`
- [X] T031 [US3] Update governance operational status UX language and escalation guidance in `frontend/apps/governance/src/pages/CircuitBreakerPage.tsx`

**Checkpoint**: US3 independently verifiable and non-ambiguous for governance operators.

---

## Phase 6: Polish & Cross-Cutting Verification

**Purpose**: Final frontend verification, Scenario A non-regression, and documentation close-out.

- [X] T032 [P] Run bank frontend lint and capture result expectations in `frontend/apps/bank/package.json`
- [X] T033 [P] Run governance frontend lint and capture result expectations in `frontend/apps/governance/package.json`
- [X] T034 [P] Run bank frontend type-check and capture result expectations in `frontend/apps/bank/tsconfig.json`
- [X] T035 [P] Run governance frontend type-check and capture result expectations in `frontend/apps/governance/tsconfig.json`
- [ ] T036 Execute manual Scenario B sovereign flow validation checklist in `specs/008-fix-cb-liquidity/quickstart.md` *(PENDENTE — requer ambiente Scenario B up + tryout-sovereign-cb-liquidity.sh)*
- [ ] T037 Execute manual Scenario B commercial swap validation checklist in `specs/008-fix-cb-liquidity/quickstart.md` *(PENDENTE — requer pool ACTIVE e tryout-scenario-b-e2e.sh)*
- [X] T038 Execute Scenario A non-regression route/menu checks for bank app in `frontend/apps/bank/src/routes/index.tsx`
- [X] T039 Execute Scenario A non-regression route/menu checks for governance app in `frontend/apps/governance/src/routes/index.tsx`
- [X] T040 Finalize feature traceability notes and completion evidence in `specs/008-fix-cb-liquidity/tasks.md`
- [X] T040-A [P] Synchronize backend environment templates with production config: update `.env.infra.*.example` files (bank-a/b/c/d, central-bank-a/b, contracts) with HUB_IDENTITY_REGISTRY_ADDRESS, PAIR_REGISTRY_CONTRACT_ADDRESS, FX fields, PENTE config, CENTRAL_BANK_B keys

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: starts immediately.
- **Phase 2 (Foundational)**: depends on Phase 1; blocks all user stories.
- **Phase 3 (US1)**: depends on Phase 2.
- **Phase 4 (US2)**: depends on Phase 2 (can run in parallel with US1 if staffed).
- **Phase 5 (US3)**: depends on Phase 2 and integrates best after US1 observability states exist.
- **Phase 6 (Polish)**: depends on completion of all selected stories.

### User Story Dependency Graph

- **US1 (CB sovereign liquidity UX)**: no user-story dependency after Phase 2.
- **US2 (commercial swap UX)**: no user-story dependency after Phase 2.
- **US3 (governance observability)**: depends on shared foundations and benefits from US1 state model completion.

Execution order recommendation:
1. US1 (MVP)
2. US2
3. US3

### Within-Story Ordering Rules

- Types/models first, then services/store logic, then page/UI integration.
- `[P]` tasks only when files are independent and no unfinished dependency exists.

---

## Parallel Execution Examples

### US1

```bash
# Parallel model/UI prep
T012, T017
```

### US2

```bash
# Parallel implementation on independent files
T019, T022, T024
```

### US3

```bash
# Parallel implementation on independent files
T027, T030
```

---

## Implementation Strategy

### MVP First (US1 only)

1. Complete Phase 1.
2. Complete Phase 2 (blocking foundation).
3. Complete Phase 3 (US1).
4. Validate US1 independently via T036.
5. Demo sovereign CB flow.

### Incremental Delivery

1. Deliver US1 (sovereign CB flow).
2. Deliver US2 (commercial swap alignment).
3. Deliver US3 (governance observability refinement).
4. Finish with Phase 6 verification and Scenario A non-regression (T038, T039).

### Parallel Team Plan

1. Team aligns on Phase 1 and Phase 2.
2. Then split:
   - Dev A: US1
   - Dev B: US2
   - Dev C: US3 (after US1 state contracts are stable)
3. Rejoin for Phase 6 verification.

---

## Notes

- Scope is intentionally frontend-only (`frontend/apps/bank`, `frontend/apps/governance`).
- Backend, contracts, infra, and API semantics are out of scope for these tasks.
- Scenario A regression checks are explicit and mandatory before close-out.

## Execution Evidence (2026-05-21 to 2026-05-26)

**Frontend Static Checks:**
- `npm run lint --workspace=bank` -> PASS
- `npm run lint --workspace=governance` -> PASS
- `npm run type-check --workspace=bank` -> PASS
- `npm run type-check --workspace=governance` -> PASS

**Backend Extensions (FR-028, FR-029):**
- LP positions repository implementation: COMPLETE (T011-A)
- LP positions handler + route: COMPLETE (T011-B, T011-C)
- OpenAPI spec update: COMPLETE (T011-C)
- Simplified API payload: COMPLETE (implemented in 007-bridge-based-cb-liquidity, documented in FR-029)
- Backend compilation: PASS (`go build` no errors)

**Environment Synchronization (T040-A):**
- `.env.infra.bank-a.example`: COMPLETE (3 fields added)
- `.env.infra.bank-b.example`: COMPLETE (3 fields added)
- `.env.infra.bank-c.example`: COMPLETE (13 fields added)
- `.env.infra.bank-d.example`: COMPLETE (13 fields added)
- `.env.infra.central-bank-a.example`: COMPLETE (1 field added)
- `.env.infra.central-bank-b.example`: COMPLETE (1 field added)
- `contracts/.env.example`: COMPLETE (2 fields added)

**Documentation:**
- Frontend integration guide: COMPLETE (`docs-reference/Scenario-B-Frontend-Integration-Guide.md`, 12.5KB)
- Tryout scripts updated: COMPLETE (3 scripts with LP position queries)

**Manual Validation (Pending):**
- T036 / T037: **PENDING** — executar `make scenario-b.up` + tryouts antes de marcar como concluídas.
