# Tasks: Retire the Vestigial Scenario A AMM and Keep the Circuit Breaker as a Database-Only Governance Flag

**Feature**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06
**Re-executed**: 2026-09-02 from `develop` on branch `feat/044-scenario-a-amm-retirement`.

> **This task list is the original record from 2026-08-06 and its `[x]` marks belong to that run.**
> The work was redone against `develop`, which had moved 262 commits on. Every divergence — three
> removal sites this list never knew about, a corrected test count (52, not ~36), `LiquidityTransfersPage`
> deliberately excluded, FR-023 recorded as not done, and a latent test-isolation race found and fixed —
> is in [spec.md](./spec.md) Amendment 1. Read that before treating any line here as current.
**Input**: [spec.md](./spec.md) · [plan.md](./plan.md) · [research.md](./research.md) · [data-model.md](./data-model.md) · [contracts/breaker-db-only.md](./contracts/breaker-db-only.md) · [quickstart.md](./quickstart.md)

**Test-first, inverted for a deletion.** Constitution Principle V is non-negotiable, but a retirement has no new behaviour to write a failing test for — success is defined by absence. The faithful equivalent is **characterisation**: before removing the chain path, pin the database-only breaker path with a test that passes *while both paths still exist* and must still pass afterwards. That test is the proof the retirement changed nothing, and it is written first (Phase 3) before any deletion (Phase 4).

**Deletion order.** Leaves before trunks. Delete a test suite before the contract it covers, a store before the module that imports it. Deleting a trunk first breaks the tree and produces a diff nobody can review.

**The inventory is derived, not copied.** The originating rescope plan's site list was verified incomplete in **six** places and **wrong in two** — it implies a bank page barrel and a `useAMM` barrel export, neither of which exists (see [research.md](./research.md) R4). Phase 2 rebuilds it from the code before anything is removed.

**One requirement is unverifiable, by design.** `docs/r2-h-2-rescope-and-plan.md` and `docs/R2-H-2.md` are deliberately **untracked** working documents, held outside version control on every branch. FR-023's update to the plan is therefore a local edit that no diff will show. It is a should-do, not a gate. **Do not commit the plan to make it verifiable** — that reverses a deliberate decision. T005a establishes the status; T045 records the completion fact in the change description instead.

**Package manager.** Scenario A's frontend is an **npm** workspace — no `pnpm-workspace.yaml`, root `package.json` uses `workspaces`. Use `npm run <script> --workspace=<app>`.

**Licence headers.** This feature is expected to create no new source file. Any it does create MUST start with `// SPDX-License-Identifier: Apache-2.0` followed by a blank line (FR-027).

**Paths** are relative to `scenario-a/` unless shown otherwise.

---

## Phase 1: Setup — baseline before touching anything

**Purpose**: Make every later failure attributable to this feature. In a deletion feature this matters more than usual, because a broken build is the expected symptom of a missed reference.

- [x] T001 Record the contract baseline with `cd scenario-a && make contracts.build && make contracts.test`, saving the **test count** as well as the result — the count must fall by a known amount later, and an unrecorded starting point makes that unverifiable
- [x] T002 [P] Record the Go baseline with `cd scenario-a && make test.all` and `cd scenario-a/toolkit && go build ./... && go test ./...`, noting any pre-existing failure and its cause so it is neither inherited silently nor used to excuse a real regression
- [x] T003 [P] Record the portal baseline with `cd scenario-a/frontend && npm run build --workspace=governance && npm run build --workspace=bank && npm run lint --workspace=governance && npm run lint --workspace=bank`, saving the warning/error counts
- [x] T004 [P] Record the licence-header gate from the repository root with **bash** (never zsh): `bash tools/check-license-headers.test.sh && bash tools/check-license-headers.sh` — both must exit 0

**Checkpoint**: Every gate's starting state is known, including counts.

---

## Phase 2: Foundational — derive the true inventory (blocking)

**Purpose**: Know every site before removing anything. The rescope plan omits `step_render_cb_env.go`, `entityenv.go:175`, `addrs.go:85`, the `.PHONY` entry, three frontend barrels and the AMM content inside the bank sample data set — and missing the first alone breaks the toolkit build. It also names two sites that do not exist. This phase additionally fixes the boundary of the feature: which deploy script is in scope, and how the rescope-plan record is kept given that the plan is untracked.

- [x] T005 Build the authoritative removal inventory by grepping Scenario A for `AutomatedMarketMaker`, `IAutomatedMarketMaker`, `AutomatedMarketMakerLibrary`, `AMM_ADDRESS`, `AMMAddress`, `amm.`, `useAmmStore`, `useAMM` and `ammApi` across `contracts/ backend/ toolkit/ frontend/ make/ docs/`, and record the file list in the PR description — this list, not the plan's, is what Phase 4 works through. The plan's list is known incomplete in six places and **wrong in two** (it implies a bank page barrel and a `useAMM` barrel export, neither of which exists), so do not reconcile the grep against it
- [x] T005a **Establish the rescope plan's status before planning Phase 6**: run `git ls-tree -r --name-only HEAD -- docs/r2-h-2-rescope-and-plan.md docs/R2-H-2.md` and `git log --all --oneline -- docs/r2-h-2-rescope-and-plan.md`. Both are expected to be **empty** — these are deliberately untracked working documents on every branch. Confirm the files are present in the working tree (so the local edit in T045 is possible), and confirm they are **not** to be committed
- [x] T005b [P] Confirm the live deployment path does **not** deploy the engine: `contracts.deploy-hub` exits 1 by design ("hub-besu was removed"), `contracts.deploy-all` depends only on `deploy-spoke-a`/`deploy-spoke-b`, and `CBWeb3Spoke.s.sol` states in its header that hub-only contracts including the AMM are not deployed there. This establishes that `CBWeb3Spoke.s.sol` is **out of bounds** for the whole feature
- [x] T006 [P] Confirm no Scenario A contract other than the AMM's own interface imports it, and that `CBWeb3Hub.s.sol` only ever constructs it standalone and never passes it into another contract's constructor, so the removal cannot disturb the remaining contracts' wiring
- [x] T007 [P] Confirm nothing outside `scenario-a/` references `scenario-a/backend/shared/blockchain/amm`, and that Scenario B's AMM contract, tests and client are separate files — protecting Constitution Principle I before a shared-looking package is deleted
- [x] T008 [P] Confirm `buf`, `protoc` and `protoc-gen-go` are absent and that `apis/proto/compliance/v1/compliance.proto` plus the generated `compliance.pb.go` / `compliance_grpc.pb.go` are the files that must NOT change, recording this as the FR-011 boundary

**Checkpoint**: The blast radius is known, confined to Scenario A, and the untouchable proto boundary is explicit.

---

## Phase 3: User Story 2 — characterise the halt control BEFORE removing anything (Priority: P1)

**Goal**: Prove the database-only breaker path works while the chain path still exists, so the same test passing afterwards demonstrates the retirement changed nothing.

**Independent test**: Toggle the halt flag in both directions and read it back through the database path with no chain client configured; the test passes both before and after Phase 4.

> **This phase must complete and be green before Phase 4 begins.** Its tests are the safety net for a change whose own coverage is partly being deleted. Running it afterwards instead would prove nothing.

- [x] T009 [US2] Write or confirm a compliance-service test that toggles the breaker to halted and back through the **database-only** path (breaker client nil) and asserts the reported state, the persisted `circuit_breaker_paused` parameter and the recorded updater/timestamp, in `backend/services/compliance/internal/grpc/server/`
- [x] T010 [US2] Write or confirm a test asserting `GetCircuitBreakerStatus` reports the recorded state through the database path and that the response's on-chain transaction reference is **empty without that being an error**, per contract rules C-3 and C-4
- [x] T011 [US2] Run `make test.compliance` and `make test.api-gateway` and confirm green, recording that these tests pass **with the chain path still present** — this is the before-half of the characterisation

**Checkpoint**: The halt control's real behaviour is pinned by tests that do not depend on the AMM. Phase 4 may now begin.

---

## Phase 4: User Story 1 — retire the AMM (Priority: P1) 🎯 core of the feature

**Goal**: Nothing in Scenario A contains or references a swap engine.

**Independent test**: Contracts, services and toolkit build and test green with the AMM absent, and a repository-wide grep finds it nowhere in Scenario A.

### Contracts — leaves first

- [x] T012 [US1] Delete the standalone AMM test suite (~30 tests) at `contracts/test/AutomatedMarketMaker.t.sol`, and note the count in the commit message per FR-003 so the falling total reads as intended
- [x] T013 [US1] Delete the standalone deploy script `contracts/script/AutomatedMarketMaker.s.sol`
- [x] T014 [US1] Strip the AMM assertions from `contracts/test/CBWeb3Hub.t.sol` — the deployment assertion, the bytecode assertion, the token/reserve/paused assertions and the identity-registry wiring assertion — leaving every assertion about the remaining contracts intact
- [x] T015 [US1] Strip the AMM from the **legacy** `contracts/script/CBWeb3Hub.s.sol`: the import, the public field, the construction and the address log, without altering the construction order or the identity-registry wiring of any remaining contract. Leave `contracts/script/CBWeb3Spoke.s.sol` **untouched** — it is the live deploy and never deploys the engine (T005b)
- [x] T016 [US1] Delete `contracts/src/AutomatedMarketMaker.sol`, `contracts/src/interfaces/IAutomatedMarketMaker.sol` and the empty stub `contracts/src/libraries/AutomatedMarketMakerLibrary.sol`
- [x] T017 [US1] Remove the `contracts.deploy-amm-besu` target from `make/30-contracts.mk:51` **and** its entry in the `.PHONY` declaration at line 128, confirming first that no other target depends on it (none does)
- [x] T018 [US1] Run `make contracts.build && make contracts.test` and confirm green, with the test count down by the deleted suite and by nothing else. `CBWeb3Hub.t.sol` still runs under `forge test -vvv`, so the legacy script must keep compiling and passing — that, not running the script, is how T015 is verified, because its deploy target fails by design

### Backend — the breaker narrows to one path

- [x] T019 [US1] Delete the on-chain breaker test file (4 tests) at `backend/services/compliance/internal/grpc/server/circuit_breaker_onchain_test.go`, before the branches it covers are removed
- [x] T020 [US1] Remove the on-chain branches from the breaker handling in `backend/services/compliance/internal/grpc/server/server.go`: the on-chain state read, the pause and resume-vote calls, and the on-chain error branch — leaving `toggleCircuitBreakerLocal` as the sole implementation
- [x] T021 [US1] Remove the breaker field from the service struct and the breaker parameter from its constructor in the same file, so **no nil-guard survives for a path that cannot exist** (rule D-7); update the constructor's callers
- [x] T022 [US1] Remove `newAMMBreakerClient` and its injection from `backend/services/compliance/cmd/compliance/main.go`, and make the start-up log state unconditionally that the halt control is a database-only governance flag
- [x] T023 [US1] Delete the whole chain-client package `backend/shared/blockchain/amm/` — `besu.go`, `interfaces.go`, `amm_test.go` (2 tests) and the generated `bindings/`
- [x] T024 [US1] Run `make test.all` and confirm green, and confirm the **Phase 3 characterisation tests still pass** — this is the after-half that proves the halt control is unchanged

### Toolkit and configuration

- [x] T025 [US1] Remove the AMM address field and its `AMM_ADDRESS` parsing from `toolkit/engine/addrs/addrs.go`, confirming that a previously written deployed-addresses file still containing the key is **ignored rather than rejected** (rule D-4)
- [x] T026 [US1] Remove the AMM address field from `toolkit/engine/orchestrator/entityenv.go` **and** the `AMM_ADDRESS={{.AMMAddress}}` line from its rendered environment template (rule D-5)
- [x] T027 [US1] Remove the AMM address assignment from `toolkit/engine/orchestrator/step_render_cb_env.go` — **the site the rescope plan omits**; missing it leaves the toolkit failing to compile
- [x] T028 [US1] Remove the `AMM_ADDRESS` row from `backend/config/.env.infra.central-bank-a.example` and `backend/config/.env.infra.central-bank-b.example` (both currently commented out, so no live environment asserts one)
- [x] T029 [US1] Run `cd toolkit && go build ./... && go test ./...` and confirm green
- [x] T030 [US1] Confirm the FR-011 boundary held: `git diff` on `scenario-a/apis/proto/**` and `scenario-a/backend/shared/proto/**` must be **empty**

**Checkpoint**: US1 complete. Contracts, services and toolkit are green with the AMM gone, the breaker runs one path, and the proto is untouched. Satisfies FR-001…FR-009, FR-012.

---

## Phase 5: User Story 3 — remove dead portal surfaces and inert mock plumbing (Priority: P2)

**Goal**: No unreachable screen, no synthetic call, no switch that nothing reads.

**Independent test**: Both portals build and lint green; every route resolves to a reachable screen; the bank dashboard issues no swap-engine request.

- [x] T031 [P] [US3] Delete the unreachable `frontend/apps/governance/src/pages/CircuitBreakerPage.tsx` and its export at `frontend/apps/governance/src/pages/index.ts:5`, confirming first that no route or navigation entry referenced it
- [x] T032 [US3] Remove the live `refreshPool()` call and the `useAmmStore` import/selector from `frontend/apps/bank/src/pages/DashboardPage.tsx` (lines 31, 71 and the `void refreshPool()` in the effect, plus the commented selector at line 72), and confirm the rendered output is unchanged — the variable reading the result is already commented out, so nothing displayed depends on it (FR-017, rule D-9)
- [x] T033 [US3] Delete the commented-out route entries for the liquidity and AMM screens at `frontend/apps/bank/src/routes/index.tsx:34,55` and the commented navigation entry at `frontend/apps/bank/src/components/layout/Sidebar.tsx:23` — these are the **only** references to the two pages
- [x] T034 [US3] Delete the bank AMM screens `frontend/apps/bank/src/pages/AMMTradingPage.tsx` and `frontend/apps/bank/src/pages/LiquidityTransfersPage.tsx`. **There is no bank page barrel** — `bank/src/pages/index.ts` does not exist and `routes/index.tsx` imports each page directly — so no page-barrel edit is needed or possible here
- [x] T035 [US3] Delete the bank AMM supporting layer — `services/api/amm.api.ts` (unconditionally backed by the sample data set, with no live path), `stores/amm.store.ts`, `types/amm.types.ts`, and `hooks/useAMM.ts` (already orphaned: exported by no barrel and imported by nothing) — then remove the three **real** barrel exports at `stores/index.ts:6`, `services/api/index.ts:6` and `types/index.ts:7`
- [x] T036 [US3] Edit the bank portal's sample data set `frontend/apps/bank/src/services/mocks/mock-db.ts` to drop its AMM content — the `AMMPoolStatus` / `AMMQuoteRequest` / `AMMQuoteResponse` type imports (lines 4–6), the module-level `pool` value (line 58) and the `quoteExactOutput`, `getPoolStatus` and `swapExactOutput` methods (lines 149, 167, 177) — leaving every unrelated sample entry intact. This is a partial edit of a shared file, not a deletion
- [x] T037 [P] [US3] Delete the inert mock plumbing in `frontend/apps/governance/src` — the `useMocks` export in `services/api/http-client.ts` (consumed by nothing) and `services/mocks/mock-db.ts` (imported by nothing) — confirming zero consumers immediately before deleting (FR-018, rule D-10)
- [x] T038 [US3] Leave every commented-out block unrelated to the AMM or the mock plumbing untouched, including the governance Registry's disabled sections (FR-019, rule D-11)
- [x] T039 [US3] Run `cd scenario-a/frontend && npm run build --workspace=governance && npm run build --workspace=bank && npm run lint --workspace=governance && npm run lint --workspace=bank` and confirm green with **no new** warning or error against the T003 baseline — this is what catches an unused import or type left behind

**Checkpoint**: US3 complete. Satisfies FR-015…FR-020.

---

## Phase 6: Documentation

**Goal**: The halt control's nature is written down, no document claims the AMM as a Scenario A capability, and the rescope plan reflects reality.

- [x] T040 Document the halt control as a governance flag that records a halt decision with **no swap engine behind it and therefore no on-chain enforcement in Scenario A**, in a place a maintainer meets before working on it — the compliance service's breaker code and the Scenario A README (FR-013)
- [x] T041 [P] Remove the `AMM_ADDRESS` row from `docs/runbooks/configuration-reference.md`
- [x] T042 Correct the **presence and coverage** claims — the statements that break because the contract and its suite no longer exist here (FR-022a). Read each file individually; **find-and-replace is prohibited**:
  - `docs/test-execution-plan.md:44,169,215,235` — claims Foundry unit and fuzz coverage of the AMM's invariants and constant-product formula. All four become false once the 30-test suite is deleted. This is the most concrete correction in the feature.
  - `docs/runbooks/contract-configuration.md:77,102` — contract inventory with constructor arguments, and a "planned contracts for the hub" list.
  - `docs/runbooks/deployment-runbook.md:151,335` — contract inventory, plus a pointer to `make contracts.deploy-hub` deploying the AMM (a target that already fails and would then reference a deleted script).
  - `docs/architecture/architecture-overview.md:140` — contract table entry.
  - `docs/charts/scenario-a/architecture.md:185,188,189,281` — the AMM node and its wiring inside a **Scenario A** chart.
  - `README.md:137,288` — deployment description and contract table.
- [x] T043 [P] Preserve every statement that is **already accurate** because it credits the AMM to the other scenario or to a planned hub — for example "used in Scenario B", "Constant-product AMM liquidity pool (Scenario B)", and "the hub network is reserved for Scenario B (AMM)". Most existing mentions are of this kind, so verify they survived T042 unaltered (FR-022b)
- [x] T044 Reconcile the data-source statement in `docs/user-manuals/scenario-a/governance.md` so it describes a single live data source and does not tell the reader to determine which mode is active. **Check whether feature 043 has landed first**: if it has, revise its corrected wording rather than reverting it; if not, write the end state directly (FR-021, FR-026a)
- [x] T045 Record Workstream 2 as complete, in **two** places with different audiences (FR-023). Neither step may be skipped in favour of the other:
  - **(a) In the untracked rescope plan**, if it is present in your working tree — a local edit for the team's working record. It will not appear in the diff, and `git status` must still show the file as untracked afterwards. **Do not `git add` it.**
  - **(b) In the change description** — the reviewer-visible half, since (a) is invisible to review. State that Workstream 2 of the R2-H-2 rescope is complete and that the plan was updated locally.

**Checkpoint**: Documentation matches the code. Satisfies FR-013, FR-021…FR-023, FR-026.

---

## Phase 7: Polish, gates and cross-cutting verification

- [x] T046 Run the closing repository-wide grep from [quickstart.md](./quickstart.md) §2 over `contracts/ backend/ toolkit/ frontend/ make/` and confirm **zero** matches for the AMM by any of its names (SC-001); justify any deliberate exception in the PR
- [x] T047 Confirm the deleted files are gone and no build entry point references them: the five contract files, the chain-client package, the on-chain breaker test, and the frontend surfaces
- [x] T048 Run the full gate set and confirm green against the recorded baselines — `make contracts.build`, `make contracts.test`, `make test.all`, `toolkit` build and test, both portal builds and lints
- [x] T049 [P] Confirm Scenario B is untouched: `git diff --name-only origin/develop...HEAD | grep '^scenario-b/'` returns nothing, and `cd scenario-b && make contracts.test && make scenario-b.test-backend` stays green (FR-009, FR-024, SC-003)
- [x] T050 [P] Confirm no dependency was added or removed by checking `git diff` on every `go.mod`, `go.sum`, `package.json` and lockfile (FR-025)
- [x] T051 [P] Confirm the licence gate still passes with **bash** — `bash tools/check-license-headers.test.sh && bash tools/check-license-headers.sh` — and that any newly created source file carries the SPDX header (FR-027)
- [x] T052 Confirm the test-count drop matches the deletions exactly — roughly 30 contract tests plus 6 Go tests — and that no other test disappeared; state the figure in the PR description (FR-003)
- [ ] T053 Walk the manual halt-control verification on a running Scenario A stack per [quickstart.md](./quickstart.md) §4, including that a halt recorded **before** the retirement reads back identically after it (FR-014, rule D-2), and that the bank dashboard issues no swap-engine request (SC-008) — **OUTSTANDING: requires a running Scenario A stack; not performed during implementation.** The automated characterisation (T009/T010) covers the halt logic, and the bank dashboard's rendered output is unchanged by construction (the value it fetched was never read), but the end-to-end operator walkthrough and the browser network-tab check are still open.
- [x] T054 Confirm the change is a net reduction — substantially more lines removed than added, no new abstraction (SC-010) — and split the work into the four commits described in [plan.md](./plan.md)

---

## Dependencies

```text
Phase 1 (Setup: T001–T004)
        │
Phase 2 (Foundational inventory: T005, T005a, T005b, T006–T008)   ← blocks ALL removal
        │       │
        │       └─ T005a establishes the rescope plan's untracked status, for T045
        │       └─ T005b fixes the deploy-script boundary for T015
        │
Phase 3 (US2 characterise breaker: T009–T011) ← MUST be green before Phase 4
        │
        ├─► Phase 4  US1 retire AMM
        │      contracts  T012→T013→T014→T015→T016→T017→T018
        │      backend    T019→T020→T021→T022→T023→T024
        │      toolkit    T025→T026→T027→T028→T029→T030
        │
        ├─► Phase 5  US3 portals  T031–T039   (independent of Phase 4)
        │
        └─► Phase 6  Docs        T040–T045    (T041/T042 need Phase 4 landed for accuracy;
                          │                    T045 = local plan edit + a note in the PR)
Phase 7 (Gates: T046–T054)
```

**Hard ordering**: Phase 3 before Phase 4 — the characterisation is worthless after the fact. Phase 2 before any deletion — the inventory is what prevents a missed reference.

**Within Phase 4**: each block is sequential, because every step either edits the same file as the previous one or depends on it compiling. Deleting the contract (T016) before stripping its consumers (T014, T015) would break the build mid-phase.

**Story independence**: US3 (Phase 5) touches only frontends and needs no backend change, so it can proceed in parallel with Phase 4. US1 and US2 are coupled by design — US2's characterisation exists to make US1 safe.

---

## Parallel execution examples

**Phase 1** — T002, T003 and T004 cover different toolchains and run in parallel after T001.

**Phase 2** — T005a, T005b, T006, T007 and T008 are independent read-only confirmations and run together after T005.

**Phase 4** — the three blocks (contracts, backend, toolkit) touch disjoint trees and could run in parallel by different hands, but each block is internally sequential. The safest single-threaded order is contracts → backend → toolkit, because it moves from the least to the most dependent layer.

**Phase 5** — T031 (governance page) and T037 (governance mock plumbing) are independent of the bank tasks and of each other. The bank tasks T032→T036 are sequential: remove the consumer before the modules it imports, and edit the shared sample data set last so its remaining content is judged against a settled tree.

**Phase 6** — T041 and T043 are parallel; T042 must precede T043, because T043 verifies what T042 left alone.

**Phase 7** — T049, T050 and T051 are parallel; T046, T047, T048, T052, T053 and T054 are sequential gates.

---

## Implementation strategy

**Characterise first, then delete inward-out.** Phase 3 is not ceremony: the breaker's own on-chain tests are among the files being deleted, so without a characterisation test written while the old path still exists, a regression could disappear together with the coverage that would have caught it.

**MVP = Phases 3 and 4.** They deliver the whole point of the workstream: Scenario A no longer contains a swap engine, and the halt control provably still works. Phases 5 and 6 are cleanup and accuracy, valuable but not load-bearing.

**Out of scope**: removing the breaker RPCs from the interface contract (blocked — `buf`/`protoc` absent, generated code committed); wiring the deleted governance breaker screen to the database toggle (the flag is already reachable via the API and the badge); any change to Scenario B's AMM; any on-chain retraction of an already-deployed contract.

**Verification limit to keep in view**: T053 needs a running Scenario A stack. The halt control's end-to-end behaviour and the absence of the bank dashboard's synthetic call are confirmed there, not by the automated suites. The logic beneath them — the database-only toggle and status read — is covered automatically by T009 and T010.

---

## Requirement traceability

Every functional requirement maps to at least one task. Use this to validate coverage before approving.

| Requirement | Tasks |
|---|---|
| FR-001 remove contract, interface, library stub, deploy script | T013, T016 |
| FR-002 strip the legacy hub script; leave the live spoke script alone | T005b, T006, T015, T018, T049 |
| FR-003 remove AMM assertions + suite; state the ~36-test drop | T012, T014, T052 |
| FR-004 remove chain-client package and its injection | T022, T023 |
| FR-005 database-only path becomes the sole implementation | T020, T021 |
| FR-006 remove toolkit address wiring; derive inventory from code | T005, T025, T026, T027 |
| FR-007 remove address from config templates and config reference | T028, T041 |
| FR-008 remove build target and its `.PHONY` entry | T017 |
| FR-009 no change outside Scenario A | T007, T049 |
| FR-010 breaker RPCs, routes, handlers, badge retained | T009, T010, T024 |
| FR-011 interface contract definition unchanged | T008, T030 |
| FR-012 database path unchanged for every caller | T009, T011, T024 |
| FR-013 document the flag as having no on-chain enforcement | T040 |
| FR-014 recorded halt state unchanged by the removal | T053 |
| FR-015 remove unreachable governance breaker screen | T031 |
| FR-016 remove bank AMM screens and supporting layer | T033, T034, T035, T036 |
| FR-017 dashboard stops the pool refresh; output unchanged | T032 |
| FR-018 remove inert mock switch and unimported sample data | T037 |
| FR-019 leave unrelated commented blocks alone | T038 |
| FR-020 both portals build and lint clean | T039 |
| FR-021 manual describes one live data source | T044 |
| FR-022(a) correct presence and coverage claims | T041, T042 |
| FR-022(b) preserve accurate other-scenario descriptions | T043 |
| FR-023 record Workstream 2 complete (precondition-gated) | T005a, T045 |
| FR-024 every change confined to Scenario A and shared docs | T007, T049 |
| FR-025 no new dependency | T050 |
| FR-026(a) reconcile the manual line with 043, do not overwrite | T044 |
| FR-026(b) no document dependency; do not commit the plan | T005a, T045 |
| FR-027 SPDX header on any new file | T051 |
| FR-028 no settlement path changed | T006, T018, T024 |

## Task count summary

| Phase | Tasks | Count |
|---|---|---|
| 1 — Setup / baseline | T001–T004 | 4 |
| 2 — Foundational inventory | T005, T005a, T005b, T006–T008 | 6 |
| 3 — US2 characterise breaker (P1) | T009–T011 | 3 |
| 4 — US1 retire the AMM (P1) | T012–T030 | 19 |
| 5 — US3 portal surfaces (P2) | T031–T039 | 9 |
| 6 — Documentation | T040–T045 | 6 |
| 7 — Polish and gates | T046–T054 | 9 |
| **Total** | | **56** |

One of the 56 — T045(a), the rescope-plan edit — is a **local change that will not appear in the diff**, because that document is deliberately untracked. Its reviewer-visible half is T045(b). Nothing is blocked; see T005a.
