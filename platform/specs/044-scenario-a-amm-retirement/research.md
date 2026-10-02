# Phase 0 Research: Scenario A AMM Retirement

**Feature**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06

Every question was resolved by direct inspection of the repository. No external research was required — each unknown was a question about what this codebase does. Findings below are the basis for the requirements; where they contradict the originating rescope plan, the code wins.

---

## R1. Is the Scenario A AMM genuinely vestigial?

**Decision**: Yes. Retire it.

**Evidence** (all verified):

| Claim | Verification |
|---|---|
| Nothing else in Scenario A's contracts imports it | Only `AutomatedMarketMaker.sol` ↔ `IAutomatedMarketMaker.sol` reference each other, plus `script/CBWeb3Hub.s.sol`, `test/CBWeb3Hub.t.sol`, `script/AutomatedMarketMaker.s.sol` and `test/AutomatedMarketMaker.t.sol`. No other contract imports it and it is never passed into another contract's constructor. `HTLCHashTimeLocked` does not reference it, so the settlement path is untouched. |
| The deploy creates no pools and no liquidity | `CBWeb3Hub.s.sol:77` constructs it with two tokens plus the identity registry, logs the address, and stops. |
| **No working deployment path deploys it at all** | Stronger than the plan claimed. `CBWeb3Hub.s.sol` is reachable only via `contracts.deploy-hub`, which is `@echo "WARNING: hub-besu was removed…" && exit 1` (`make/30-contracts.mk:60–63`), and `contracts.deploy-cbweb3-besu` merely depends on it. The live path is `contracts.deploy-all → deploy-spoke-a/deploy-spoke-b → CBWeb3Spoke.s.sol`, whose own header states "Hub-only contracts (AMM) are **NOT** deployed here" (`CBWeb3Spoke.s.sol:16`) and which contains no AMM construction. The engine reaches a chain only if an operator runs `contracts.deploy-amm-besu` by hand — and that target is itself being removed. |
| Its only runtime consumer is the circuit breaker | `backend/shared/blockchain/amm` is imported by exactly three files, all in the compliance service: `server.go`, `cmd/compliance/main.go`, and `circuit_breaker_onchain_test.go`. |
| The utility library is an empty stub | `AutomatedMarketMakerLibrary.sol` is 6 lines, declares an empty `library`, and is imported by nothing. |
| The bank AMM screens are unreachable | `frontend/apps/bank/src/routes/index.tsx:34,55` — both route entries are commented out. The sidebar entry at `Sidebar.tsx:23` is commented out too. |
| The governance breaker screen is unreachable | `pages/CircuitBreakerPage.tsx` is exported from the barrel (`pages/index.ts:5`) but appears **nowhere** in `routes/index.tsx`. |
| Scenario A's own docs already disclaim it | `README.md:288` annotates the contract "*(used in Scenario B)*". |

**Rejected alternative**: *Document as inert only.* Offered to the project owner and declined. It would leave 36 tests for unrun code, unreachable screens, and a synthetic call on every dashboard load — the same class of defect (an inert switch presented as a working one) that feature 043 removed elsewhere.

---

## R2. Can the circuit-breaker RPCs be removed instead of kept?

**Decision**: Keep them. Retain the RPCs, gateway routes, handlers and status badge as an explicit database-only governance flag.

**Two independent findings drove this, and they point in opposite directions:**

1. *Coupling is not the obstacle.* The compliance contract is **duplicated per scenario** — `scenario-a/apis/proto/compliance/v1/compliance.proto` and `scenario-b/apis/proto/compliance/v1/compliance.proto` are separate files. Scenario A declares `GetCircuitBreakerStatus` and `ToggleCircuitBreaker` at lines 269–270 of its own copy. Removing them there would **not** affect Scenario B. This corrects the implicit assumption that the contract is shared.
2. *The toolchain is the obstacle.* `buf`, `protoc` and `protoc-gen-go` are **not installed** (`which` finds none of them), the `proto-gen` target is `cd apis/proto && buf generate`, and the generated `compliance.pb.go` / `compliance_grpc.pb.go` are committed. A contract change therefore could not be regenerated, compiled or verified in this environment.

**Rationale**: The RPCs already run a database-only path whenever no breaker is wired — `server.go:479` branches to `toggleCircuitBreakerLocal` when `s.breaker == nil`, which is the Scenario A default because the toolkit never sets the address. Keeping them costs nothing and preserves a working governance control; removing them is blocked on tooling. So keep, and make the database path the sole implementation.

**Consequence**: FR-011 is a hard constraint, not a preference. The contract definition must come out of this feature byte-for-byte unchanged.

---

## R3. Does removing the AMM break the circuit breaker or the governance badge?

**Decision**: No. Both keep working, and the badge needs no change at all.

**Rationale**: Two separate paths were checked.

- *Service.* `server.go:457` reads on-chain state only `if s.breaker != nil`; `:479` falls back to the local toggle when it is nil. `main.go:132` `newAMMBreakerClient()` already returns nil and logs "on-chain circuit breaker disabled (database-only toggle)" when the address or key is absent. Removing the AMM permanently selects the path that already runs.
- *Portal.* Scenario A's `governanceApi.getCircuitBreaker` (`governance.api.ts:66`) calls `/governance/circuit-breaker/status` directly with **no** mock branch. This is the opposite of Scenario B, where the equivalent V1 call was mock-gated and had to be repointed in feature 043. Scenario A's badge therefore already reads the real backend and is unaffected.

---

## R4. What is the true inventory of sites to change?

**Decision**: Derive the inventory from the code. The originating plan's list is accurate but **incomplete**.

**Sites the plan lists and the code confirms**: the four contract files plus the two script/test edits; the `backend/shared/blockchain/amm` package; `newAMMBreakerClient` and its injection; the breaker field, parameter and on-chain branches in `server.go`; `circuit_breaker_onchain_test.go`; `AMMAddress` in `toolkit/engine/orchestrator/entityenv.go` and `toolkit/engine/addrs/addrs.go`; the `AMM_ADDRESS` lines in both central-bank config templates; the row in `docs/runbooks/configuration-reference.md`; `contracts.deploy-amm-besu` in `make/30-contracts.mk`; and the dead frontend surfaces.

**Sites the plan omits, found by grep**:

- `toolkit/engine/orchestrator/step_render_cb_env.go:90` also assigns `AMMAddress: addrs.AMMAddress`. Missing this would leave the toolkit failing to compile.
- `entityenv.go:175` emits `AMM_ADDRESS={{.AMMAddress}}` into the rendered environment template.
- `addrs.go:85` parses `kv["AMM_ADDRESS"]` from the deployed-addresses file.
- `make/30-contracts.mk:128` declares `contracts.deploy-amm-besu` in the `.PHONY` list, separately from the target at line 51. No other target depends on it.
- Three **frontend barrels** re-export the bank engine modules and must be edited: `bank/src/stores/index.ts:6`, `bank/src/services/api/index.ts:6`, `bank/src/types/index.ts:7`.
- The bank portal's **sample data set** (`bank/src/services/mocks/mock-db.ts`) is not merely a dependency of the engine layer — it *contains* engine content: three type imports (lines 4–6), a module-level `pool` value (line 58) and three methods (lines 149, 167, 177), interleaved with unrelated sample data. This is a partial edit, not a deletion.
- Six documents besides the configuration reference mention the contract: `docs/architecture/architecture-overview.md`, `docs/test-execution-plan.md`, `docs/charts/scenario-a/architecture.md`, `docs/runbooks/contract-configuration.md`, `docs/runbooks/deployment-runbook.md`, and `README.md` (lines 137 and 288). Each needs reading individually — see R10, because the nature of the required correction is not what the plan implies.

**Two sites the plan lists that do NOT exist**, and would send an implementer hunting:

- There is **no** `bank/src/pages/index.ts`. The bank portal has no page barrel; `routes/index.tsx` imports each page directly. So "delete the pages and their barrel exports" is half-wrong for the bank app — only the two commented route lines and the sidebar entry reference them.
- `bank/src/hooks/index.ts` does **not** export `useAMM`, and nothing imports it. That hook is already fully orphaned, so it can be deleted with no consumer to update.

**Consequence**: FR-006 explicitly requires deriving the inventory from the code rather than copying the plan's list. This research pass found four omissions and two phantom sites, which is the evidence for that requirement.

---

## R5. Is the bank portal's AMM layer dead code, or dead work?

**Decision**: Both, and the distinction matters for what "no behaviour change" means.

**Rationale**: The screens are commented out, but `DashboardPage.tsx` still does real work for them on every load:

- `:31` imports `useAmmStore`; `:71` selects `refreshPool`; `:85–91` invokes `void refreshPool()` inside a **live** `useEffect`.
- `:72` — `const pool = useAmmStore((state) => state.pool);` — is **commented out**, and every card that rendered `pool` (`AMM Pool Status`, `AMM Reserves`, `Liquidity Overview`, `Liquidity Mix`) is inside a commented block.

So the store is populated on every dashboard load and the result is never read. Further, `services/api/amm.api.ts` delegates all three of its methods to `mockDb` **unconditionally** — there is no `useMocks` branch and no live path. This is not a mocked-out live capability; it is a fixed sample data set with no counterpart.

**Consequence**: FR-017 can promise that removing the call changes no rendered output, and that promise is verifiable rather than hopeful. It also means nothing of value is lost with the layer.

---

## R6. Is deleting Scenario A's AMM safe for Scenario B?

**Decision**: Yes, unconditionally. They are separate files.

**Rationale**: `scenario-a/contracts/src/AutomatedMarketMaker.sol` is 304 lines; `scenario-b/contracts/src/AutomatedMarketMaker.sol` is 558. `diff` reports them as different. Scenario B has its own test suite (`scenario-b/contracts/test/AutomatedMarketMaker.t.sol`) and its own chain client (`scenario-b/backend/shared/blockchain/scenariob/amm`, which feature 043 modified). Nothing is shared between them, so scenario isolation holds by construction rather than by care.

**Note**: this is also why the constitution's "no shared code across scenarios except via an explicitly versioned shared library" rule is not violated by the duplication — and why removing one copy is a single-scenario change.

---

## R7. What does retirement cost in test coverage?

**Decision**: About 36 tests are deleted. Accept it, and say so plainly.

**Counts**: `contracts/test/AutomatedMarketMaker.t.sol` has 30 `function test` declarations; `backend/shared/blockchain/amm/amm_test.go` has 2 `func Test`; `circuit_breaker_onchain_test.go` has 4.

**Rationale**: Every one of these exercises a contract or client that never executes in Scenario A. Deleting a test for deleted code is not a coverage regression. Scenario B keeps its own suite for the engine that does run.

**Consequence**: FR-003 requires stating the deletion in the change description, so a reviewer sees a falling test count as intended rather than as an accident.

---

## R8. How does this interact with feature 043?

**Decision**: Record a merge-order dependency; do not coordinate branches.

The dependency has **two parts of different severity**, and conflating them was the main error in the first draft of this specification.

### (a) Soft — a wording collision on a shared manual line

Feature 043 (Workstreams 1 and 3 of the same rescope) corrected `docs/user-manuals/scenario-a/governance.md`'s data-source line and, per its FR-016, was deliberately worded **not** to promise that the inert flag or unused mock data would be removed — precisely because that removal is this feature. This branch is cut from the integration branch and does not contain 043's correction, so both features edit the same line from different starting points.

**Resolution**: FR-026(a) requires whichever lands second to reconcile rather than overwrite, with a defined end state: a statement describing a portal that has one live data source and no mock plumbing at all.

### (b) Not a dependency at all — the rescope plan is outside version control

An earlier revision of this document recorded a hard dependency here: the plan and ticket were committed on the 043 branch and absent from `develop`, so FR-023 looked blocked until 043 merged. **That is no longer the situation.**

By decision of the project owner, `docs/r2-h-2-rescope-and-plan.md` and `docs/R2-H-2.md` are **untracked working documents**. They were removed from the 043 branch's history for that purpose, and `git ls-tree` now confirms they are tracked on **no** branch — not `develop`, not 043, not this one.

The consequence is a reclassification rather than an escalation:

- There is **no ordering precondition** on this feature from the documents. Merging 043 neither provides nor withholds them, so 044 may land before or after 043 as far as they are concerned.
- FR-023 becomes unverifiable rather than blocked. The update can be made by anyone holding the file, but it will never appear in a diff, so it cannot serve as a gate.
- The only genuine coupling left is (a), the wording collision on the tracked manual.

**Resolution**: FR-023 is downgraded to a should-do local step with three constraints — not a gate, never satisfied by committing the plan, and with the verifiable part (a completion note) moved into the change description so the fact is recorded where a reviewer can see it. FR-026(b) records that no document dependency exists.

**Rejected alternatives**:

- *Commit the plan so FR-023 becomes verifiable.* Rejected: it reverses a deliberate decision to keep these documents out of the repository, to buy a checkbox.
- *Drop FR-023 entirely.* Rejected: the plan is the team's working record of the rescope, and leaving Workstream 2 marked open there would misinform the next reader of it. Unverifiable is not the same as worthless.
- *Branch 044 from 043.* Rejected on its own merits: it would make 044 unmergeable until 043 lands and mix a Scenario B feature's commits into a Scenario A cleanup, weakening the isolation both features rely on.

---

## R10. What is actually wrong with the documentation?

**Decision**: Not mis-attribution. Two different corrections are needed, and only one of them is about scenario labelling.

**Rationale**: Reading the seven documents shows most mentions are **already correct** — they describe the engine as the other scenario's concern: "AMM pool (used in Scenario B)" (`docs/runbooks/contract-configuration.md:77`, `docs/runbooks/deployment-runbook.md:151`, `README.md:288`), "Constant-product AMM liquidity pool (Scenario B)" (`docs/architecture/architecture-overview.md:140`), "the hub network (chain 1337) is reserved for Scenario B (AMM)" (`architecture-overview.md:257`). A blanket rewrite would corrupt accurate text.

The real defect after retirement is **stale presence and coverage claims**:

- `docs/test-execution-plan.md` claims Foundry coverage of the engine at lines 44, 169, 215 and 235 — "fuzz tests for HTLC and AMM invariants", "AMM constant-product formula". Deleting the 30-test suite makes all four false. This is the most concrete correction in the feature.
- `docs/runbooks/contract-configuration.md:77` and `docs/runbooks/deployment-runbook.md:151` list it in contract inventories *for this repository*, with constructor arguments.
- `docs/runbooks/deployment-runbook.md:335` points at `make contracts.deploy-hub` deploying the AMM — a target that already fails, and which would then reference a deleted script.
- `docs/charts/scenario-a/architecture.md:185` diagrams the engine inside a **Scenario A** chart, wired to the oracle and spoke bridge.

**Consequence**: FR-022 is split into (a) correct presence and coverage claims, and (b) preserve accurate other-scenario descriptions — with an explicit prohibition on find-and-replace.

---

## R9. Is a migration or on-chain action required?

**Decision**: No.

**Rationale**: Deleting contract source does not retract an already-deployed instance; it stops being referenced. The `AMM_ADDRESS` entries in the config templates are already commented out (`.env.infra.central-bank-{a,b}.example:72`), so no live environment is asserting one. The halt flag is stored as compliance parameters (`circuit_breaker_paused` and friends, `server.go:441–443`), which the database path already reads and writes — so the recorded state survives untouched and no schema changes.
