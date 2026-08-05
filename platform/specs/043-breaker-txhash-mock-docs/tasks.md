# Tasks: Circuit-Breaker Transaction-Hash Visibility and Mock-vs-Live Documentation Reconciliation

**Feature**: `043-breaker-txhash-mock-docs` | **Date**: 2026-08-05
**Input**: [spec.md](./spec.md) · [plan.md](./plan.md) · [research.md](./research.md) · [data-model.md](./data-model.md) · [contracts/circuit-breaker-v2.md](./contracts/circuit-breaker-v2.md) · [quickstart.md](./quickstart.md)

**Tests are included and are written first.** Constitution Principle V (Test-First at Every Layer) is non-negotiable for this repository: the test MUST fail before the implementation is written.

**Frontend testing limit.** The governance app declares `vitest` only — no `test` block in `vite.config.ts`, no `jsdom`/`happy-dom`, no `@testing-library/react`. Component-rendering tests are therefore **not available**, and FR-026 forbids adding the dependencies. Portal tests are store- and api-level, following the existing `src/features/liquidity/format.test.ts` pattern; rendering is verified manually per [quickstart.md](./quickstart.md).

**Licence headers.** Every new `.go` and `.ts` file created by these tasks MUST start with `// SPDX-License-Identifier: Apache-2.0` followed by a blank line (FR-027). This is mandatory per `CLAUDE.md` and gated in CI.

**Paths** are relative to the repository root unless shown otherwise.

---

## Phase 1: Setup

- [ ] T001 Record the pre-change baseline by running `cd scenario-b && make test.api-gateway` and saving the result, so any later failure is attributable to this feature and not pre-existing
- [ ] T002 [P] Confirm the governance portal builds and tests clean before changes with `cd scenario-b/frontend && pnpm --filter governance test --run && pnpm --filter governance type-check && pnpm --filter governance build` (note `test` is bare `vitest`, so `--run` is required in CI to avoid watch mode)
- [ ] T003 [P] Confirm the licence-header gate passes before changes with `bash tools/check-license-headers.test.sh && bash tools/check-license-headers.sh` (both must exit 0)

---

## Phase 2: Foundational (blocking prerequisites)

**Purpose**: Know every site that must change before widening a shared interface, so the widening cannot silently miss an implementation.

- [ ] T004 Inventory every implementation and caller of `AMMCircuitBreakerCaller` and record them in the PR description — currently the production adapter `scenario-b/backend/services/api-gateway/internal/app/amm_adapter.go`, the wiring in `scenario-b/backend/services/api-gateway/internal/http/router/v2/router.go`, and the test fakes in `scenario-b/backend/services/api-gateway/internal/http/handlers/governance_scenariob_handler_test.go` and `scenario-b/backend/services/api-gateway/internal/services/relay_cb_coverage_test.go`
- [ ] T005 Confirm no consumer outside Scenario B imports `backend/shared/blockchain/scenariob/amm` before changing its signature, by grepping the repository for that import path, to protect Constitution Principle I

**Checkpoint**: The blast radius of the interface change is known and confined to Scenario B.

---

## Phase 3: User Story 1 — Operator can audit the on-chain action behind a breaker decision (Priority: P1) 🎯 MVP

**Goal**: Every breaker write action returns and displays a copyable on-chain transaction hash that survives reload and is identical for every central bank.

**Independent test**: Pause a pair on a chain-wired stack; the hash appears, copies, survives reload, and matches the ledger.

### Tests first (MUST fail before implementation)

- [ ] T006 [US1] Create the new service test file with the mandatory `// SPDX-License-Identifier: Apache-2.0` header and a fake `AMMCircuitBreakerCaller`, then write a failing test asserting `Pause` returns a non-empty tx hash and persists it to `OnChainTxRef`, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service_test.go` (file does not exist yet)
- [ ] T007 [US1] Write a failing service test asserting `ProposeResume` returns **both** the proposal ID and a distinct tx hash, and persists the hash, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service_test.go`
- [ ] T008 [US1] Write a failing service test asserting `SignResume` returns a tx hash and persists it, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service_test.go`
- [ ] T009 [US1] Write a failing service test asserting `GetStatus` reports the pair's most recent action hash ordered by `SignedAt`, including when the latest action was taken by a different institution, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service_test.go`
- [ ] T010 [US1] Write a failing service test asserting that in no-chain mode all actions still succeed and no hash is produced, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service_test.go`
- [ ] T011 [US1] Write failing handler tests asserting `tx_hash` is present in the `pause`, `resume-request`, `resume-sign` and `status` JSON bodies when chain-wired, and that `resume-request` returns `request_id` and `tx_hash` as different values, in `scenario-b/backend/services/api-gateway/internal/http/handlers/governance_scenariob_handler_test.go`
- [ ] T012 [US1] Write a failing handler test asserting `tx_hash` is **absent as a JSON key** (not `""`) in all four responses in no-chain mode, per contract rule C-1, in `scenario-b/backend/services/api-gateway/internal/http/handlers/governance_scenariob_handler_test.go`
- [ ] T013 [P] [US1] Create `scenario-b/frontend/apps/governance/src/features/circuit-breaker/circuit-breaker-v2.store.test.ts` with the mandatory SPDX header, and write failing store-level tests (Vitest, Node environment — no DOM) asserting that a stubbed api response's `tx_hash` reaches `cbStatus` for `pause`, `signResume` and `fetchStatus`, and — critically — that `proposeResume` also carries it despite hand-rebuilding `cbStatus`

### Implementation — carry the value to the boundary

- [ ] T014 [US1] Change `ProposeResume` to return the receipt transaction hash alongside the proposal ID it already extracts from the `LogResumeProposed` topic, in `scenario-b/backend/shared/blockchain/scenariob/amm/client.go`
- [ ] T015 [US1] Widen the `AMMCircuitBreakerCaller` interface so `ProposeResume` returns `(proposalID, txHash string, err error)` and `SignResume` returns `(txHash string, err error)`, leaving `ExecuteResume` unchanged because it makes no chain call, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service.go`
- [ ] T016 [US1] Update the production adapter to propagate both values from `ProposeResume` and to stop discarding the hash from `SignResume` (currently `_, err = c.SignResume(...)`), in `scenario-b/backend/services/api-gateway/internal/app/amm_adapter.go`
- [ ] T017 [P] [US1] Update the test fakes to satisfy the widened interface, in `scenario-b/backend/services/api-gateway/internal/http/handlers/governance_scenariob_handler_test.go` and `scenario-b/backend/services/api-gateway/internal/services/relay_cb_coverage_test.go`

### Implementation — carry the value through the service

- [ ] T018 [US1] Change `Pause` to return `(txHash string, err error)` instead of discarding `txRef`, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service.go`
- [ ] T019 [US1] Change `ProposeResume` and `SignResume` to return the tx hash, and populate `OnChainTxRef` on the signature records they create — currently only `Pause` sets it, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service.go`
- [ ] T020 [US1] Keep `RequestID` and `OnChainTxRef` distinct on resume signature records per data-model rule D-3, and do not extend the existing `Pause` conflation of the two, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service.go`
- [ ] T021 [US1] Add `TxHash string \`json:"tx_hash,omitempty"\`` to `CircuitBreakerStatus` and populate it in `GetStatus` from the pair's most recent `CircuitBreakerSignature` by `SignedAt`, regardless of signer, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service.go`

### Implementation — expose over HTTP

- [ ] T022 [US1] Include `tx_hash` in the `pause`, `resume-request`, `resume-sign` and `status` JSON bodies, omitting the key entirely when no hash exists, in `scenario-b/backend/services/api-gateway/internal/http/handlers/governance_scenariob_handler.go`
- [ ] T023 [US1] Update the router wiring for any changed service signatures, in `scenario-b/backend/services/api-gateway/internal/http/router/v2/router.go`

### Implementation — surface in the portal

- [ ] T024 [P] [US1] Add optional `tx_hash?: string` to **both** `CircuitBreakerV2Status` and `ProposeResumeResponse` — the resume proposal returns its own type, so one alone is insufficient — in `scenario-b/frontend/apps/governance/src/types/circuit-breaker-v2.types.ts`
- [ ] T025 [US1] Confirm `tx_hash` flows through all four calls; the typed responses carry it once T024 lands, so change only what does not, in `scenario-b/frontend/apps/governance/src/services/api/circuit-breaker-v2.api.ts`
- [ ] T026 [US1] Update the `proposeResume` reducer to carry `tx_hash: response.tx_hash` into `cbStatus` — it hand-rebuilds the object field by field and already drops `resume_signatures`/`resume_quorum`, so the hash is lost without this; no new store field is needed because `pause`/`signResume`/`fetchStatus` replace `cbStatus` wholesale, in `scenario-b/frontend/apps/governance/src/features/circuit-breaker/circuit-breaker-v2.store.ts`
- [ ] T027 [US1] Render `cbStatus.tx_hash` as a plain selectable value with a one-click copy control, never as a hyperlink, and never gate the breaker action on it, in `scenario-b/frontend/apps/governance/src/pages/CircuitBreakerPage.tsx`
- [ ] T028 [US1] Render a neutral "no on-chain reference" state distinct from empty or unknown when the key is absent, in `scenario-b/frontend/apps/governance/src/pages/CircuitBreakerPage.tsx`

**Checkpoint**: US1 is independently shippable. All tests from T006–T013 pass. Satisfies FR-001 to FR-010. Rendering (T027, T028) is confirmed manually — no DOM test environment exists.

---

## Phase 4: User Story 2 — One consistent breaker state everywhere (Priority: P2)

**Goal**: The chrome indicator and the breaker page can never disagree about whether swaps are halted.

**Independent test**: With the portal in its default configuration, halt one of several pairs and confirm the indicator and the page agree.

> **This is a derivation, not a source swap.** The indicator claims "swaps are globally halted", but V2 status is per-pair. It must therefore be computed: halted if **any** pair is halted (FR-012), indeterminate if the pair set is unknown (FR-014). Reuse the existing `ammPairsApi.getPairs()` and the `usePolling` hook — no new endpoint, no backend change.

- [ ] T029 [P] [US2] Create a store-level test file with the mandatory SPDX header and write failing Vitest cases (Node environment — no DOM) for the derivation rules: halted when any of several pairs is halted; operational when none is; indeterminate and never "halted" when the pair list is empty or unavailable; a pair whose status fails to load is treated as not-known-halted, under `scenario-b/frontend/apps/governance/src/stores/`
- [ ] T030 [US2] Replace the `useMocks`-gated `governanceApi.getCircuitBreaker` call with the derived network-wide condition — fetch pairs via `ammPairsApi.getPairs()`, read each pair's state via `circuitBreakerV2Api.getStatus`, and reduce per rules N-1 to N-4 in [data-model.md](./data-model.md) — in `scenario-b/frontend/apps/governance/src/stores/circuit-breaker.store.ts`
- [ ] T031 [US2] Keep `hooks/useCircuitBreaker.ts` as the single seam so `components/layout/AppLayout.tsx`, `components/layout/Sidebar.tsx` and `pages/DashboardPage.tsx` update together; verify each still reads only `.state` and adjust the exposed shape if the V1-only `updatedAt`/`updatedBy` fields are dropped, in `scenario-b/frontend/apps/governance/src/hooks/useCircuitBreaker.ts`
- [ ] T032 [US2] Remove the now-unused V1 `getCircuitBreaker` (and `setCircuitBreaker`/`toggle` if likewise unreferenced) from the api, store and `mock-db`, or record why any must stay, in `scenario-b/frontend/apps/governance/src/services/api/governance.api.ts` and `scenario-b/frontend/apps/governance/src/stores/circuit-breaker.store.ts`

**Checkpoint**: US2 is independently shippable. Satisfies FR-011 to FR-014.

---

## Phase 5: User Story 3 — Manuals can be trusted about data provenance (Priority: P2)

**Goal**: Every data-source statement matches the code, the settings screen agrees with its manual, and no inert switch is offered.

**Independent test**: Cross-read each of the four statements against the corresponding code path and confirm agreement.

### Scenario B and shared documentation

- [ ] T033 [P] [US3] Correct the data-source line to state that a mock toggle exists, is consumed, and defaults to mock unless set to exactly `"false"`, at line 5 of `docs/user-manuals/scenario-b/governance.md`
- [ ] T034 [P] [US3] Replace the stale Environment card copy — "API mode: mock services enabled" and "WebSocket: simulated relay events for UI flow" — so it describes actual behaviour, in `scenario-b/frontend/apps/bank/src/pages/SettingsPage.tsx`
- [ ] T035 [US3] Align the bank data-source line with the corrected settings screen, at line 5 of `docs/user-manuals/scenario-b/bank.md`
- [ ] T036 [US3] Reconcile the implementation-status table and reattribute the live mock toggle to Scenario B Governance, in `docs/user-manuals/README.md`
- [ ] T037 [P] [US3] Remove the inert `VITE_USE_MOCKS` declaration, which no bank source file reads, from `scenario-b/frontend/apps/bank/.env.example`
- [ ] T038 [P] [US3] Annotate the ticket to record that its Scenario A framing is inaccurate and that finding 1's mock-default behaviour belongs to Scenario B governance, in `docs/R2-H-2.md`

### Scenario A (separate commit — scenario isolation)

- [ ] T039 [P] [US3] Correct the data-source line to describe the portal as live-backed with no selectable mock mode, without promising removal of the inert flag or unused mock data, at line 4 of `docs/user-manuals/scenario-a/governance.md`
- [ ] T040 [P] [US3] Remove the inert `VITE_USE_MOCKS` declaration, which no bank source file reads, from `scenario-a/frontend/apps/bank/.env.example`

**Checkpoint**: US3 is independently shippable. Satisfies FR-015 to FR-021.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T041 [P] Record the known limitation that resume reports the live state after a no-op finalise, which would be premature for any quorum above 2, as a comment where a maintainer will meet it before changing quorum, in `scenario-b/backend/services/api-gateway/internal/services/circuit_breaker_service.go`
- [ ] T042 [P] Record the discovered pre-existing gap that breaker actions write no governance audit-trail entries, with enough detail for separate triage, in `docs/R2-H-2.md`
- [ ] T043 Run the full backend suite and confirm green with `cd scenario-b && make scenario-b.test-backend`
- [ ] T044 [P] Run the governance portal tests, type-check, lint and build and confirm green with `cd scenario-b/frontend && pnpm --filter governance test --run && pnpm --filter governance type-check && pnpm --filter governance lint && pnpm --filter governance build`
- [ ] T045 [P] Verify every new `.go` and `.ts` file carries the SPDX header, then confirm the gate passes with `bash tools/check-license-headers.test.sh && bash tools/check-license-headers.sh` (both must exit 0)
- [ ] T046 Confirm no new runtime or dev dependency was added, by checking `git diff` on `scenario-b/frontend/apps/governance/package.json`, the lockfile and `scenario-b/backend/**/go.mod` (FR-026)
- [ ] T047 Walk the manual verification steps on a chain-wired stack and confirm each expectation, including the rendering not covered by automated tests, per [quickstart.md](./quickstart.md) section 2
- [ ] T048 Split the work into the four commits described in [plan.md](./plan.md) — Scenario B tx hash, Scenario B badge, docs plus Scenario B text and config, then Scenario A text and config — so no PR spans both scenarios' code

---

## Dependencies

```text
Phase 1 (Setup: T001–T003)
        │
Phase 2 (Foundational: T004–T005)   ← blocks US1 only; US2 and US3 may start any time
        │
        ├─► Phase 3  US1  T006–T013 (tests, fail first) ─► T014–T017 (boundary)
        │                                                 ─► T018–T021 (service)
        │                                                 ─► T022–T023 (HTTP)
        │                                                 ─► T024–T028 (portal)
        │
        ├─► Phase 4  US2  T029 (test) ─► T030 ─► T031 ─► T032
        │
        └─► Phase 5  US3  T033–T040 (independent of all code work)
                          │
Phase 6 (Polish: T041–T048)
```

**Story independence**: US1, US2 and US3 are independently deliverable. US3 depends on no code change. US2 touches only the indicator path and needs no backend change. Only US1 requires the Foundational phase.

**Ordering constraints within US1**: T014 → T015 → T016 → T017 must run in sequence (each depends on the previous signature change compiling). T018–T021 all edit the same file and are therefore sequential. T022 depends on T021. T026 depends on T024.

**Ordering constraint within US2**: T030 depends on T029; T031 depends on T030; T032 last, because it can only judge what is unreferenced once the repoint has landed.

---

## Parallel execution examples

**Phase 3 tests** — T006 to T010 all add cases to the same new Go file, so they are one sequential batch (T006 first: it creates the file, its SPDX header and the fake). T011 and T012 share the handler test file, so they are a second sequential batch, parallel with the first. T013 is a different file again and runs in parallel with both.

**Phase 5** — T033, T034, T037, T038, T039 and T040 touch six different files and can all run in parallel. T035 should follow T034 so the manual matches the final settings copy; T036 should follow T033 and T035 so the table matches the corrected manuals.

**Phase 6** — T041, T042, T044 and T045 are parallel; T043, T046, T047 and T048 are sequential gates.

---

## Implementation strategy

**MVP = User Story 1 only.** It is the sole functional residual of R2-H-2 and delivers the auditability the finding asked for. Ship and verify it on a chain-wired stack before starting anything else.

**Then US3**, which is pure text and configuration and carries no runtime risk, followed by **US2**, which is small but touches shared portal chrome and introduces a derived network-wide condition, so it benefits from being reviewed on its own.

**Out of scope**: Workstream 2 of the rescope plan — the Scenario A vestigial-AMM retirement — is excluded by decision of the project owner and remains available as separate future work. Two known gaps are recorded rather than fixed: the premature live report for quorum above 2 (T041) and the missing breaker audit-trail entries (T042).

**Verification limit to keep in view**: the governance portal has no DOM test environment and FR-026 forbids adding one, so T027, T028 and the indicator's rendering are covered by T047's manual walkthrough rather than by automated tests. The data-handling logic beneath them — where the reference could be silently dropped — is covered automatically by T013 and T029.

---

## Requirement traceability

Every functional requirement maps to at least one task. Use this to validate coverage before approving.

| Requirement | Tasks |
|---|---|
| FR-001 retain hash on all three write actions | T018, T019 |
| FR-002 return hash to caller; widen boundary where discarded | T015, T016, T018, T019 |
| FR-003 resume proposal returns both proposal ID and hash | T014, T015, T016, T019, T020 |
| FR-004 finalise-resume yields no hash, not an error | T015 (interface unchanged), T012 |
| FR-005 status carries pair's latest hash, any institution | T009, T021 |
| FR-006 portal shows pair's latest hash, survives reload | T026, T027 |
| FR-007 copyable and selectable | T013, T027 |
| FR-008 plain value, not a hyperlink; link-ready later | T027 |
| FR-009 absence distinct from empty/unknown | T010, T012, T022, T028 |
| FR-010 never gates the action | T027 |
| FR-011 indicator uses same authoritative, non-mockable source | T029, T030, T031 |
| FR-012 indicator halted if **any** pair halted | T029, T030 |
| FR-013 indicator cannot show synthetic while page shows live | T029, T030, T032 |
| FR-014 degrade safely when pair set unknown or empty | T029, T030 |
| FR-015 manuals accurately state data source | T033, T035, T039 |
| FR-016 Scenario A manual: live-backed, no removal promise | T039 |
| FR-017 Scenario B governance manual: toggle exists, state default | T033 |
| FR-018 bank manual and settings screen agree | T034, T035 |
| FR-019 summary table agrees and reattributes toggle | T036 |
| FR-020 no inert variables in templates | T037, T040 |
| FR-021 annotate ticket mis-scoping | T038 |
| FR-022 record quorum > 2 limitation, do not fix | T041 |
| FR-023 record missing audit-trail entries, do not fix | T042 |
| FR-024 scenario isolation; Scenario A separable | T005, T039, T040, T048 |
| FR-025 decision model unchanged | T020 (no conflation), T043 (suite green), enforced by review |
| FR-026 no new dependencies | T013, T029 (test approach), T046 (verified) |
| FR-027 SPDX header on every new file | T006, T013, T029, T045 |

## Task count summary

| Phase | Tasks | Count |
|---|---|---|
| 1 — Setup | T001–T003 | 3 |
| 2 — Foundational | T004–T005 | 2 |
| 3 — US1 (P1) | T006–T028 | 23 |
| 4 — US2 (P2) | T029–T032 | 4 |
| 5 — US3 (P2) | T033–T040 | 8 |
| 6 — Polish | T041–T048 | 8 |
| **Total** | | **48** |
