# Implementation Plan: Retire the Vestigial Scenario A AMM and Keep the Circuit Breaker as a Database-Only Governance Flag

**Branch**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/044-scenario-a-amm-retirement/spec.md`

## Summary

Delete Scenario A's `AutomatedMarketMaker` and everything that exists only to serve it — the interface, the empty library stub, the standalone deploy script, its 30-test suite, the `backend/shared/blockchain/amm` chain client and bindings, the `AMMAddress` wiring through the provisioning toolkit, the `AMM_ADDRESS` configuration rows, the `contracts.deploy-amm-besu` build target, the unreachable governance breaker page, and the bank portal's commented-out AMM/Liquidity surfaces including a pool refresh the dashboard still calls on every load. Strip the AMM block from the **legacy** `CBWeb3Hub` deploy script and its assertions from that script's test.

Worth stating plainly, because it reframes the risk: **no working deployment path deploys this contract.** `CBWeb3Hub.s.sol` is reachable only through `contracts.deploy-hub`, which deliberately `exit 1`s with "hub-besu was removed"; the live path is `CBWeb3Spoke.s.sol`, whose header states that hub-only contracts including the AMM are not deployed there. The engine reaches a chain only if an operator runs `contracts.deploy-amm-besu` by hand — and that target is being removed too. So this is not "removing a contract from a live deployment"; it is removing a contract nothing can deploy.

Keep the circuit breaker. Its two RPCs, gateway routes, handlers and governance status badge stay exactly as they are; only the implementation narrows, from "on-chain when wired, database otherwise" to "database, always". That is a no-op in Scenario A, because the toolkit never wires an AMM, so the database path is already the one that runs. Document the flag as a governance marker with no on-chain enforcement. Also retire the inert `useMocks` flag and unimported `mock-db` in the Scenario A governance portal, and correct the manual statement that tells operators to ask which mode is active.

**No proto change.** The compliance contract definition and its committed generated code must come out byte-for-byte unchanged.

## Technical Context

**Language/Version**: Solidity (Foundry) for contracts; Go 1.26+ for the compliance service, api-gateway and toolkit; TypeScript 5.x / React for the governance and bank portals
**Primary Dependencies**: None added; several removed from use — `go-ethereum` bindings for the AMM disappear with the client package. No `go.mod` edit is expected, because the module is shared with code that still uses go-ethereum.
**Storage**: Compliance parameters (`circuit_breaker_paused`, `circuit_breaker_updated_by`, `circuit_breaker_updated_at`). **No migration** — the database path already reads and writes these.
**Testing**: `make contracts.build` / `contracts.test` (Foundry, available); `make test.all` (compliance + auth + api-gateway); `go build ./... && go test ./...` in `toolkit/`; `npm run build|lint --workspace=<app>` for the two portals. Scenario A's frontend is an **npm** workspace — there is no `pnpm-workspace.yaml`, and the root `package.json` uses `workspaces`.
**Target Platform**: Linux containers via Docker Compose (Scenario A stack)
**Project Type**: Multi-layer — Solidity contracts, Go services and toolkit, React frontends
**Performance Goals**: None. Removing an unexecuted contract and an uncalled client changes no hot path. The bank dashboard loses one synthetic call per load, which is a marginal improvement, not a goal.
**Constraints**: `buf`, `protoc` and `protoc-gen-go` are **not installed**, and the generated `compliance.pb.go` / `compliance_grpc.pb.go` are committed — so the proto is untouchable (FR-011). No new runtime or dev dependency (FR-025). Every change confined to Scenario A and shared docs (FR-024). Any new source file carries the SPDX header (FR-027), though this feature is expected to create none.
**Scale/Scope**: 5 contract files deleted and 2 edited; 1 Go package deleted (3 files + `bindings/`) and 3 Go files edited; 3 toolkit files edited; 2 config templates and ~7 documents edited; 1 build file edited; **8 frontend files deleted and 9 edited** (4 barrels/consumers in the bank app, 2 in governance, plus routes, sidebar and the shared sample data set). Roughly 40 files touched, **net strongly negative** in lines. About 36 tests deleted.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Assessment | Verdict |
|---|---|---|
| **I. Scenario-Scoped Independence** | Every code change is in `scenario-a/`; the only shared touches are `docs/` and the rescope plan. Verified that Scenario A's AMM is a **separate file** from Scenario B's (304 lines vs 558, `diff` reports them different), with its own tests and its own client — so nothing is shared and no PR spans both scenarios' code. The duplication itself predates this feature; removing one copy is a single-scenario change. | PASS |
| **II. Privacy by Design** | Nothing is added to any surface. No PII, no amounts, no plaintext value data. Removing an unused contract cannot weaken privacy. `tCeBM` remains reserve-layer only — the AMM was never in a retail or settlement path. | PASS |
| **III. Atomic Settlement Guarantee** | Untouched. Scenario A settles through dual-layer HTLC lock plus secret reveal; the AMM never participated (FR-028). No timeout, refund or partial-settlement behaviour is modified. The HTLC contract does not import the AMM — verified. | PASS |
| **IV. Compliance Gate Before Participation** | Untouched. No authentication, authorisation or compliance check is added, removed or bypassed. The compliance service keeps its identity, KYC and freeze paths; only its breaker implementation narrows. Notably, the gate is **not** what the AMM breaker guarded. | PASS |
| **V. Test-First at Every Layer** | This is where a deletion feature genuinely differs, so it is addressed rather than waved through. There is no new behaviour to write a failing test for. The discipline is inverted: **characterise before deleting.** Before the chain path is removed, add or confirm a test asserting the database-only breaker path works end to end — that test must pass before removal and still pass after, which is what proves the retirement changed nothing. Deleting ~36 tests for deleted code is not a coverage regression, and FR-003 requires stating it openly. | PASS (with a stated inversion) |
| **VI. Observability and Auditability** | Preserved and slightly clarified. The halt action is still recorded through the database path, which already writes the audit parameters. The start-up log line that reported "on-chain breaker disabled — database-only toggle" becomes unconditional, which is *more* honest than a message implying a chain path might have been chosen. No error swallowing is introduced: the removed on-chain error branch cannot fire because the operation it reported on no longer exists. | PASS |

**Result: all six gates pass. No violations, so Complexity Tracking is empty — and this feature net-removes complexity rather than adding any.**

Note on Principle V: the inversion is deliberate and is the main methodological risk in this feature. A retirement is the one case where "write a failing test first" cannot apply literally, because success is defined by absence. Substituting a characterisation test that must pass on both sides of the change is the closest faithful equivalent, and it is stronger than a post-hoc check because it is written while the old path still exists.

## Project Structure

### Documentation (this feature)

```text
specs/044-scenario-a-amm-retirement/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── breaker-db-only.md   # Phase 1 output — the unchanged breaker surface
├── checklists/
│   └── requirements.md      # Spec quality checklist
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
scenario-a/
├── contracts/
│   ├── src/AutomatedMarketMaker.sol                    # DELETE
│   ├── src/interfaces/IAutomatedMarketMaker.sol         # DELETE
│   ├── src/libraries/AutomatedMarketMakerLibrary.sol    # DELETE (empty stub, imported by nothing)
│   ├── script/AutomatedMarketMaker.s.sol                # DELETE
│   ├── script/CBWeb3Hub.s.sol                           # EDIT — strip import, field, deploy, log (LEGACY: deploy-hub exits 1)
│   ├── script/CBWeb3Spoke.s.sol                         # DO NOT TOUCH — the live deploy; never deploys the AMM
│   ├── test/AutomatedMarketMaker.t.sol                  # DELETE (30 tests)
│   └── test/CBWeb3Hub.t.sol                             # EDIT — strip AMM assertions (still runs in `forge test`)
├── backend/
│   ├── shared/blockchain/amm/                           # DELETE whole package (+ bindings, + 2 tests)
│   ├── services/compliance/cmd/compliance/main.go       # EDIT — drop newAMMBreakerClient + injection
│   └── services/compliance/internal/grpc/server/
│       ├── server.go                                    # EDIT — drop breaker field/param + on-chain branches
│       └── circuit_breaker_onchain_test.go              # DELETE (4 tests)
├── toolkit/engine/
│   ├── addrs/addrs.go                                   # EDIT — drop AMMAddress field + AMM_ADDRESS parse
│   └── orchestrator/
│       ├── entityenv.go                                 # EDIT — drop AMMAddress field + template line
│       └── step_render_cb_env.go                        # EDIT — drop AMMAddress assignment  ← plan omitted this
├── backend/config/.env.infra.central-bank-{a,b}.example # EDIT — drop AMM_ADDRESS row
├── make/30-contracts.mk                                 # EDIT — drop target AND .PHONY entry
├── docs/runbooks/configuration-reference.md             # EDIT — drop AMM_ADDRESS row
├── docs/{architecture,charts,runbooks}/…, README.md      # EDIT — correct Scenario A capability claims
└── frontend/apps/
    ├── governance/src/pages/CircuitBreakerPage.tsx      # DELETE (+ its export at pages/index.ts:5)
    ├── governance/src/services/api/http-client.ts       # EDIT — drop inert useMocks export
    ├── governance/src/services/mocks/mock-db.ts         # DELETE (imported by nothing)
    └── bank/src/
        ├── pages/{AMMTradingPage,LiquidityTransfersPage}.tsx   # DELETE — NO page barrel exists here
        ├── services/api/amm.api.ts                             # DELETE (mockDb-backed, no live path)
        ├── stores/amm.store.ts, types/amm.types.ts             # DELETE
        ├── hooks/useAMM.ts                                     # DELETE — orphaned; no barrel export, no importer
        ├── stores/index.ts:6, services/api/index.ts:6, types/index.ts:7  # EDIT — the 3 real barrels
        ├── services/mocks/mock-db.ts                           # EDIT — drop AMM types/pool/3 methods, keep the rest
        ├── routes/index.tsx, components/layout/Sidebar.tsx      # EDIT — drop commented entries
        └── pages/DashboardPage.tsx                             # EDIT — drop the live refreshPool() call

docs/r2-h-2-rescope-and-plan.md    # UNTRACKED working doc — local edit only, never committed (see below)
docs/user-manuals/scenario-a/governance.md               # EDIT — reconcile with 043 (FR-026a)
```

> ⚠ **`docs/r2-h-2-rescope-and-plan.md` and `docs/R2-H-2.md` are outside version control.** Both are deliberately untracked working documents, tracked on no branch, by decision of the project owner. So FR-023's update to the plan is a local edit that will never appear in a diff and cannot be verified by a reviewer. It is a should-do, not a gate; committing the plan to make it verifiable is prohibited; and the completion fact goes in the change description instead. See FR-023 and FR-026(b).

**Structure Decision**: No new directory, package or module. The feature is subtractive: it removes files and narrows one interface. The only additions are documentation sentences and, possibly, one characterisation test.

## Phase 0 — Research

All unknowns were resolved by code inspection; nothing required external research. Consolidated in [research.md](./research.md). The findings that changed the plan:

1. **The proto is duplicated per scenario, not shared** (R2). Removing the breaker RPCs would be scenario-isolated — so coupling was never the obstacle. The obstacle is that `buf`/`protoc` are absent and the generated code is committed. This converts FR-011 from a preference into a hard constraint.
2. **The rescope plan's site inventory is both incomplete and partly wrong** (R4). Missing: `step_render_cb_env.go:90` (omitting it alone breaks the toolkit build), `entityenv.go:175`, `addrs.go:85`, the `.PHONY` declaration at `make/30-contracts.mk:128`, three bank frontend barrels, and AMM content inside the bank sample data set. Wrong: it implies a bank page barrel (`bank/src/pages/index.ts` does not exist) and a `useAMM` barrel export (nothing exports or imports that hook). Hence FR-006 requires deriving the inventory from the code.
3. **Nothing can deploy the contract** (R1). The script that constructs it is legacy behind a deliberately failing target, and the live per-spoke script excludes it by design and says so. This lowers the deployment risk substantially and changes how the deploy-script edit is verified — by compiling and running its test, not by running the script.
4. **The bank AMM layer is dead *work*, not just dead code** (R5). `DashboardPage` still calls `refreshPool()` in a live effect while the variable reading the result is commented out, and `amm.api.ts` delegates to `mockDb` unconditionally with no live path at all. This makes FR-017's "no rendered change" verifiable rather than hopeful.
5. **The Scenario A governance badge is not mock-gated** (R3), unlike Scenario B's, which feature 043 had to repoint. Scenario A's badge needs no change.
6. **The two scenarios' AMMs are separate files of different sizes** (R6), so deletion cannot affect Scenario B.
7. **The only real coupling with feature 043 is a wording collision on a tracked manual** (R8). An earlier revision recorded a second, harder dependency — the rescope plan being committed on the 043 branch and absent here. That no longer holds: the plan and ticket are now untracked on every branch, so there is no ordering precondition from them, and FR-023 is unverifiable rather than blocked.
8. **The documentation defect is not mis-attribution** (R10). Most mentions already correctly credit the other scenario. What breaks on retirement are presence and coverage claims — above all a test plan asserting Foundry coverage of the engine's invariants, which the deleted suite makes false.

## Phase 1 — Design & Contracts

**Artifacts generated**: [research.md](./research.md), [data-model.md](./data-model.md), [contracts/breaker-db-only.md](./contracts/breaker-db-only.md), [quickstart.md](./quickstart.md).

Design decisions:

- **Order matters: characterise, then remove inward-out.** Confirm the database-only breaker path under test *first*, while the chain path still exists. Then delete leaves before trunks — the standalone AMM test and deploy script before the contract; the on-chain test before the branches it covers; the frontend pages before their stores. Deleting a trunk first produces a broken tree and a large, hard-to-review diff.
- **The breaker narrows by deleting the branch, not by keeping a nil guard.** Once the AMM is gone, `if s.breaker != nil` guards a path that cannot exist. Retaining it would leave exactly the misleading structure this feature removes (rule D-7). The field, the constructor parameter and the interface go with it.
- **The proto is a hard stop.** No edit to `apis/proto/` or the generated Go. Verified by an empty `git diff` on those paths (SC-006). This is checked as a task, not assumed.
- **`AMM_ADDRESS` becomes ignored, not rejected** (rules D-4, D-5). An operator's older provisioning record or environment file may still carry it; reading it must not fail. Unknown keys are already ignored, so no defensive code is needed — but it must be confirmed rather than presumed.
- **Documentation is read individually, not bulk-edited.** Seven documents mention the AMM. Those describing Scenario B or the platform architecture are correct and stay; only Scenario A capability claims are corrected (FR-022). A blanket find-and-replace would corrupt accurate cross-scenario text.
- **Commented-out blocks are removed only when they are AMM or mock plumbing** (rule D-11). Both portals contain unrelated commented blocks — the governance Registry's disabled sections, for instance. Widening into those would inflate the diff and mix concerns.
- **The falling test count is declared, not discovered.** ~36 tests go. FR-003 requires saying so in the change description so a reviewer reads the drop as intended.

### Post-design Constitution re-check

Re-evaluated after the above: still **PASS on all six principles**. The design adds no dependency, no schema change, no proto change, no new service and no cross-scenario coupling — and deletes a contract, a package and two portal surfaces. Complexity Tracking remains empty.

### Post-implementation Constitution re-check (2026-09-02)

Run against the finished branch rather than the design, on constitution **v1.0.4**.

| Principle | Evidence from the implementation | Verdict |
|---|---|---|
| **I. Scenario-Scoped Independence** | Made verifiable instead of argued: `git diff --name-only origin/develop...HEAD -- scenario-b/` returns **zero** files, and the same check is quoted in the pull request. Both engines still exist as separate files — 330 lines in A (deleted here), 602 in B (untouched), different hashes. | PASS |
| **II. Privacy by Design** | Nothing added to any surface; no PII, amounts or plaintext value data. `tCeBM` untouched and still reserve-layer only. | PASS |
| **III. Atomic Settlement Guarantee** | No settlement path changed. Scenario A settles over the two-layer hash-locked transfer; the HTLC never imported the engine (re-verified: only `CBWeb3Hub.s.sol` and `CBWeb3Hub.t.sol` referenced it). HTLC tests, including its invariant suites, are untouched and green. | PASS |
| **IV. Compliance Gate Before Participation** | No auth, authorisation or compliance check added, removed or bypassed. The compliance service keeps identity, KYC and freeze paths; only its breaker implementation narrowed. `institutionId` — which the gate does rely on — was **kept**, its live off-chain consumer confirmed at `registry/besu.go:261`. | PASS |
| **V. Test-First at Every Layer** | The planned inversion was executed. Three characterisation tests (`circuit_breaker_dbonly_test.go`) were written **while both paths still existed**, passed then, and passed **unmodified** after the on-chain path was deleted — they never reference the `breaker` field, so the removal could not have quietly weakened them. One regression test was added for the ignore-not-reject rule on a stale `.deployed-addrs.env`. 52 tests were deleted with the code they covered, stated openly per FR-003. | PASS (inversion as designed) |
| **VI. Observability and Auditability** | The halt action is still audited through the database path. The start-up log became unconditional and now names what the control actually is. No error swallowing introduced: the removed on-chain error branch reported on an operation that no longer exists. | PASS |

**Result: six PASS. Complexity Tracking stays empty — the change net-removes 5,950 lines against 108 added.**

One finding outside the gates is recorded in [spec.md](./spec.md) Amendment 1: a pre-existing
`vm.setEnv` race between parallel contract suites, latent on `develop` and surfaced by this
change's effect on suite scheduling. It was fixed here and verified over five consecutive runs,
and it is flagged separately in the pull request rather than folded into the retirement.

## Complexity Tracking

No constitution violations. Table intentionally empty. This feature is a net reduction in complexity, which is its purpose.

## Risks and mitigations

| Risk | Why it is plausible | Mitigation |
|---|---|---|
| An inventory miss breaks a build late | The rescope plan's list was found incomplete in six places **and wrong in two** | Derive the inventory by grep before deleting (Phase 2); the repo-wide grep in [quickstart.md](./quickstart.md) §2 is the closing check |
| The breaker silently degrades | Its tests are among those being deleted, so a regression could vanish with its own coverage | Characterise the database path **before** removal; the same test must pass after (Principle V inversion) |
| The legacy deploy script edit disturbs remaining contracts | `CBWeb3Hub.s.sol` constructs several contracts in sequence with identity-registry wiring | Edit is confined to the AMM's import, field, construction and log; `CBWeb3Hub.t.sol` asserts the remaining contracts' wiring, runs in `forge test`, and must stay green. Risk is lower than it first appears — the script is legacy and its deploy target already fails |
| The **live** deploy script gets edited by mistake | Both scripts have similar names and the plan spoke loosely of "the deployment script" | `CBWeb3Spoke.s.sol` is explicitly out of bounds (FR-002); T048's diff check catches any change |
| Bulk doc editing corrupts accurate cross-scenario text | Most AMM mentions are correct **because** they describe Scenario B | Read each of the seven documents individually (FR-022b); find-and-replace is prohibited |
| Stale coverage claims survive the retirement | The test plan asserts Foundry coverage of AMM invariants in four places; deleting the suite makes all four false, and no build will flag it | FR-022(a) names presence and coverage claims specifically; T042 lists the exact lines |
| **FR-023 is faked by committing the plan** | It is unverifiable as an untracked local edit, so an implementer may "fix" that by committing the document — reversing a deliberate decision to buy a checkbox | FR-023(b) prohibits it outright; T005a establishes the document's status before Phase 6, and T045 moves the verifiable part into the change description instead |
| Collision with feature 043 on the manual line | Both features edit the same statement from different bases | FR-026(a): whichever lands second reconciles to a defined end state, rather than overwriting |
| A reviewer reads the falling test count as an accident | ~36 tests disappear | FR-003: state it in the change description with the count and the reason |

## Sequencing and PR split

Four commits, ordered so each is independently reviewable and the tree is never broken. All four are Scenario A plus shared docs, so scenario isolation holds throughout.

1. **Characterise the database-only breaker** (Principle V). Add or confirm the test that proves the database path works, while the chain path still exists. Must be green before anything is deleted — this commit is the safety net for commit 2.
2. **Retire the AMM: contracts, backend, toolkit, config, build target** (FR-001…FR-009, FR-012). The substance. Ends with contracts, services and toolkit green and the proto diff empty.
3. **Retire the dead portal surfaces and inert mock plumbing** (FR-015…FR-020). Separated because it touches two frontends and no backend, and because "this changes nothing an operator can see" is a claim best reviewed on its own.
4. **Documentation** (FR-013, FR-021…FR-023, FR-026). The breaker's DB-only nature, the corrected presence and coverage claims, the manual data-source line reconciled with 043, and a note in the change description that Workstream 2 is complete — with the rescope plan itself updated locally, outside the commit.

**Relationship to feature 043.** 043 (Workstreams 1 and 3) is a Scenario B feature plus shared docs and is independently mergeable. This branch is cut from `develop`, **not** from 043, so the two stay independently reviewable. One coupling remains:

- Both features edit the tracked `docs/user-manuals/scenario-a/governance.md`. FR-026(a) governs the reconciliation, and whichever lands second reconciles to the defined end state.

There is **no document precondition**. The rescope plan and the R2-H-2 ticket are untracked on every branch, so merging 043 neither provides nor withholds them.

**Recommended order: land 043 first, then rebase this branch.** This is now a convenience rather than a requirement — it lets commit 4 revise a manual statement that is already accurate instead of one that is wrong in a different way. If 044 lands first, nothing is blocked; commit 4 simply writes the end state directly, and 043 reconciles when it lands.
