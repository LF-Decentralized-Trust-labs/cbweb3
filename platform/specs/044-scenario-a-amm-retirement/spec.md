# Feature Specification: Retire the Vestigial Scenario A AMM and Keep the Circuit Breaker as a Database-Only Governance Flag

**Feature Branch**: `044-scenario-a-amm-retirement`
**Created**: 2026-08-06
**Status**: Draft
**Input**: Workstream 2 of `docs/r2-h-2-rescope-and-plan.md`, the part of the R2-H-2 rescope deliberately excluded from feature `043-breaker-txhash-mock-docs`. Workstreams 1 and 3 were delivered by 043; this specification covers the remaining cleanup.

> **The rescope plan and the R2-H-2 ticket are not in version control at all.** `docs/r2-h-2-rescope-and-plan.md` and `docs/R2-H-2.md` are deliberately **untracked working documents** — they are tracked on no branch, by decision of the project owner, and were removed from the 043 branch's history for that purpose. They exist only in the working tree of whoever holds them.
>
> Consequences, which shape FR-023 and FR-026:
> - The links above resolve on a machine that has the files and nowhere else. Merging 043 will **not** make them appear.
> - FR-023's update to the rescope plan is a **local edit that no pull request can show and no reviewer can verify**. It is still worth doing — the plan is the team's working record — but it cannot be a gate.
> - Satisfying FR-023 by committing the plan is **prohibited**: that would reverse a deliberate decision to keep these documents out of the repository.

## Clarifications

### Session 2026-08-06

- Q: How deep should the retirement go? → A: Full retirement — remove the contract, interface, library stub, deploy script and tests, the backend AMM package, the toolkit and environment wiring, and the dead frontend surfaces. Not the lower-effort "document as inert" alternative.
- Q: What happens to the circuit-breaker RPCs? → A: Keep them, together with the gateway routes, handlers and the governance status badge, as an explicit database-only governance flag. Do not remove them from the interface contract.
- Q: What happens to the governance breaker screen that exists but is unreachable? → A: Delete it. The flag stays reachable through the API and is already displayed by the chrome badge, and Scenario A has no swaps for a breaker screen to protect.
- Q: Should the inert mock plumbing in the Scenario A governance portal be retired as well? → A: Yes. Remove the flag that nothing reads and the mock data set that nothing imports, and correct the manual statement that describes a selectable mock mode.

### Discovered during clarification

Verification against the code refined the originating plan in ways that change the work. Each of these was checked, not assumed.

| Finding | Consequence |
|---|---|
| The interface contract is **duplicated per scenario**, not shared — each scenario has its own compliance contract definition. Removing the breaker operations from Scenario A's copy would therefore not affect Scenario B. | Removal is scenario-isolated in principle, so the reason to keep the operations is the toolchain, not coupling. |
| The contract-generation toolchain is **not installed** in this environment, and the generated code is committed to the repository. | Removing the breaker operations cannot be completed or verified here. Confirms the decision to keep them. |
| Scenario A's swap-engine source is a **different, smaller variant** than Scenario B's (roughly 300 lines against 560). They are separate files, not a shared library. | Deleting Scenario A's copy cannot affect Scenario B's behaviour or tests. |
| The Scenario A governance status badge reads the breaker through a call that is **not** mock-gated. | The badge keeps working unchanged once the flag becomes the sole path. |
| The bank dashboard's swap-engine cards are commented out, **but the dashboard still invokes the pool refresh on every load** and never reads its result. | This is dead work on every page load, not merely dead code. Removing it changes no rendered output. |
| The bank portal's swap-engine data access is backed by the mock data set **unconditionally** — it has no live path and no toggle at all. | Nothing is lost by deleting it; there is no live capability hiding behind it. |
| The environment-rendering step in the provisioning toolkit also emits the swap-engine address. The originating plan did not list this site. | The inventory must be derived from the code, not copied from the plan. |
| The swap-engine utility library is an **empty stub** that nothing imports. | Deleting it is unconditionally safe. |
| Retirement deletes roughly **36 tests** (about 30 contract tests plus 6 service tests). | The tests exercise code that never runs in this scenario. This must be stated openly rather than presented as a coverage-neutral change. |
| Feature 043 corrected the Scenario A governance manual's data-source statement, and was explicitly worded **not** to promise the removal this feature performs. This branch is cut from the integration branch, so it does not contain that correction. | A merge-order dependency exists and must be recorded, so the two features do not silently overwrite each other's wording. |
| **The originating rescope plan and ticket are untracked working documents**, held outside version control by decision of the project owner. | Stronger than a wording collision, and not fixable by merge order: the document FR-023 must update is in no branch, so the update can never appear in a diff or be verified by a reviewer. FR-023 becomes a local record-keeping step, explicitly not a gate, and committing the document to satisfy it is prohibited. |
| **The engine is not deployed by any working deployment path.** The script that constructs it is legacy — its only build target deliberately fails with "hub-besu was removed" — and the live per-spoke deployment script states in its own header that hub-only contracts including the engine are **not** deployed there. | Vestigiality is stronger than the originating plan claimed: the engine reaches a chain only if an operator runs the standalone deploy target by hand, and that target is itself being removed. It also means stripping the legacy script is lower-risk than "editing the shared deployment script" implies. |
| The bank portal has **no page barrel** (`pages/index.ts` does not exist; pages are imported directly), while three other barrels do export the engine's modules. Its hook module is exported by no barrel and imported by nothing at all. | The removal points must be named individually. A generic instruction to "remove the barrel exports" would send an implementer looking for a file that does not exist while missing three that do. |
| The bank portal's sample data set contains the engine's **types, a module-level pool value and three methods**, interleaved with unrelated sample data. | This is a partial edit of a shared file, not a deletion, and must be scoped so the non-engine sample data survives. |
| Most platform documentation already attributes the engine **correctly** to the other scenario ("used in Scenario B", "hub network"). | The documentation problem is therefore not mis-attribution. It is that several documents will reference a contract and a test suite that no longer exist in this repository — most concretely, a test plan that claims Foundry coverage of the engine's invariants. |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A maintainer can trust that what is present in Scenario A is actually used (Priority: P1)

An engineer new to the platform opens Scenario A to make a change. They find a complete constant-product swap engine — contract, interface, deploy script, an extensive test suite, a chain client package and address wiring through the provisioning toolkit — and reasonably conclude that Scenario A executes swaps. It does not: Scenario A's value flow is a two-layer hash-locked transfer, and the swap engine is deployed only so that a circuit breaker has somewhere to live. Scenario A's own contract documentation already annotates the contract as belonging to the other scenario.

The engineer therefore has to prove a negative before touching anything nearby, and every future reader repeats that work. Worse, a reader who does not investigate may extend or depend on the vestigial engine.

After this change, Scenario A contains no swap engine, and nothing in Scenario A refers to one.

**Why this priority**: This is the substance of the workstream and the largest source of misdirection. It is also the riskiest part to get wrong, so it goes first and is verified before anything else proceeds.

**Independent Test**: Build and test the Scenario A contracts and services with the swap engine removed; both are green, and a repository-wide search for the engine's name returns nothing in Scenario A except intentional historical notes. Delivers the clarity benefit on its own.

**Acceptance Scenarios**:

1. **Given** the Scenario A contract project, **When** it is built and its tests are run after the removal, **Then** both succeed with no reference to the removed engine remaining.
2. **Given** the legacy hub deployment script that constructed the engine alongside other contracts, **When** the contract suite is compiled and run, **Then** the script still compiles, its test still passes, and it neither constructs nor logs the removed engine. It is verified this way rather than by running it, because its build target deliberately fails independently of this feature.
3. **Given** the live per-spoke deployment script, **When** the feature is complete, **Then** it is unmodified — it never deployed the engine, so any change there indicates the wrong script was edited.
4. **Given** the Scenario A services, **When** they are built and tested, **Then** they compile and pass with the chain-client package for the engine absent.
5. **Given** the provisioning toolkit, **When** it renders an entity environment, **Then** it neither expects nor emits an address for the removed engine, and provisioning still succeeds.
6. **Given** a reviewer searching Scenario A for the engine by name, **When** they search source, scripts, configuration templates and build targets, **Then** the only matches are deliberate historical or cross-scenario references, each of them accurate.
7. **Given** the other scenario's own swap engine, **When** its contracts and services are built and tested, **Then** they are entirely unaffected — the two are separate files, not a shared library.

---

### User Story 2 - The governance halt control keeps working, and its nature is written down (Priority: P1)

A central bank governance operator uses a control that halts the system and a status indicator that reports whether it is halted. Today that control has two possible paths behind it: an on-chain path when a swap engine is wired, and a database-only path when it is not. In Scenario A the database path is what actually runs, because the provisioning toolkit never wires an engine.

Removing the engine must therefore be invisible to this operator: the control must behave exactly as it does today. What must change is the documentation, because an operator or auditor is currently entitled to assume the halt has on-chain force in Scenario A. It does not, and after this change it definitively cannot.

**Why this priority**: Equal to Story 1 because it is the constraint that makes Story 1 safe. A retirement that silently degraded or removed a governance control would be a regression regardless of how much dead code it deleted.

**Independent Test**: Exercise the halt control and the status indicator before and after the removal; behaviour is identical, and the documentation now states plainly that the flag is a governance marker with no swap engine behind it.

**Acceptance Scenarios**:

1. **Given** the breaker operations exposed on the service interface, **When** the removal is complete, **Then** they still exist and are still reachable through the gateway, unchanged for every caller.
2. **Given** an operator toggling the halt flag, **When** they toggle it after the removal, **Then** the outcome and the reported state are the same as before, and the action is still recorded.
3. **Given** the governance status indicator in the portal chrome, **When** the portal loads after the removal, **Then** the indicator still reports the halt state correctly.
4. **Given** a maintainer or auditor reading the Scenario A documentation, **When** they look up what the halt control does, **Then** they find an explicit statement that it is a governance flag recording a halt decision, with no swap engine behind it and therefore no on-chain enforcement in this scenario.
5. **Given** the database-only path, **When** the removal is complete, **Then** it is the **only** path, and no branch remains that could select a chain path that can no longer exist.
6. **Given** the interface contract for the breaker operations, **When** the removal is complete, **Then** it is **unchanged**, so no contract regeneration is required.

---

### User Story 3 - Operators and testers are not shown dead surfaces or served synthetic data (Priority: P2)

An operator or tester working through the Scenario A portals encounters three artefacts that misrepresent the product. A governance breaker screen exists in the codebase but no navigation reaches it, so a reader of the code believes there is a screen that no operator can open. The bank portal carries swap and liquidity screens that are commented out, along with their whole supporting layer, which is wired to a fixed sample data set with no live path at all. And the bank dashboard still triggers a pool refresh against that sample data on every single load, discarding the result.

Separately, the governance portal declares a mock-mode switch that no code reads and ships a sample data set that no code imports, while its manual tells the reader to ask an administrator which mode is active. There is no mode to ask about.

After this change, no unreachable screen remains, no synthetic call is made, and the manual describes a portal that has exactly one data source.

**Why this priority**: Below Stories 1 and 2 because no operator can currently reach the dead screens and no settlement path is involved. It is still real: the misleading manual statement and the pointless synthetic call both waste the time of the people validating the platform.

**Independent Test**: Build and lint both Scenario A portals with the dead surfaces removed; both are green, every remaining route resolves to a reachable screen, and the manual's data-source statement matches the code.

**Acceptance Scenarios**:

1. **Given** the unreachable governance breaker screen, **When** the removal is complete, **Then** the screen and its export are gone, and no navigation entry or route referred to it beforehand or afterwards.
2. **Given** the bank portal's commented-out swap and liquidity screens and their supporting layer, **When** the removal is complete, **Then** they are gone along with their commented navigation and route entries, and the portal builds and lints clean.
3. **Given** the bank dashboard, **When** it loads after the removal, **Then** it makes no pool-refresh call, and everything it renders is unchanged — the call's result was never displayed.
4. **Given** the governance portal's inert mock switch and unimported sample data set, **When** the removal is complete, **Then** both are gone and no behaviour changes, because nothing consumed them.
5. **Given** the Scenario A governance manual's data-source statement, **When** a reader consults it, **Then** it describes a single live data source and does not instruct the reader to determine which mode is active.
6. **Given** any remaining commented-out block in the Scenario A portals unrelated to the swap engine, **When** the removal is complete, **Then** it is left untouched — this feature removes the swap-engine surfaces and the inert mock plumbing, not every commented block it passes.

---

### Edge Cases

- **An environment still supplying a swap-engine address.** Configuration templates carry the variable in commented-out form today. After removal the variable is ignored rather than rejected, so an operator upgrading from an older configuration is not blocked by a stale line. The variable must not remain documented as meaningful.
- **The legacy hub deployment script.** It deploys several contracts in sequence and includes the engine, so removing one entry must not disturb the ordering, the identity wiring of the remaining contracts, or the addresses recorded for them. Note that this script is **not** the live deployment path — its build target already fails deliberately — but its test still runs in the contract suite, so it must keep compiling and passing.
- **The live per-spoke deployment script.** It never deploys the engine and documents that in its header. It must come out of this feature unmodified; a change there means the inventory captured the wrong script.
- **A deployment already on a running chain.** Removing the source cannot retract a contract already deployed. The already-deployed instance simply stops being referenced. Nothing in the retirement attempts on-chain cleanup, and no migration is implied.
- **The halt flag while an engine used to be wired.** If an environment previously ran the on-chain path, the recorded flag state remains and is now interpreted by the database path alone. The reported state must not flip as a result of the removal.
- **Provisioning that recorded an engine address.** A previously written provisioning record may contain the address. Reading such a record must not fail after the field is gone.
- **The other scenario's swap engine.** Every removal in this feature is confined to Scenario A. Any change that would touch the other scenario's engine, its tests or its shared client is out of scope by definition and indicates a mistake in the inventory.
- **Documentation describing the engine across the platform.** Several documents mention it in a cross-scenario or architectural context. Those that describe the other scenario stay; those that describe it as a Scenario A capability must be corrected, not deleted wholesale.
- **Build targets referencing the removed deploy script.** A target that deploys the engine must be removed together with any aggregate target that invokes it, so no build entry point breaks.

## Requirements *(mandatory)*

### Functional Requirements

**Retirement of the swap engine (Scenario A)**

- **FR-001**: The Scenario A swap-engine contract, its interface, its empty utility-library stub and its standalone deploy script MUST be removed.
- **FR-002**: The swap engine MUST be removed from the legacy hub deployment script that constructs it — its import, any field holding it, its construction and any logging of its address — without altering the deployment or wiring of any remaining contract in that script. The live per-spoke deployment script MUST NOT be modified, because it never deploys the engine and says so explicitly in its own header; if that script needs an edit, the inventory is wrong.
- **FR-003**: The Scenario A test that asserts the shared deployment produced a swap engine MUST have those assertions removed, and the standalone swap-engine test suite MUST be deleted. The deletion of roughly 36 tests MUST be stated in the change description rather than left for a reviewer to discover.
- **FR-004**: The Scenario A chain-client package for the swap engine, including its generated bindings and its own tests, MUST be removed, together with the construction and injection of that client in the service that used it.
- **FR-005**: The on-chain branches of the breaker handling in that service MUST be removed, leaving the database-only path as the sole implementation, with no residual branch that could select a path that no longer exists.
- **FR-006**: The swap-engine address MUST be removed from the provisioning toolkit — both the address record it is parsed into and every step that renders it into an entity environment. The inventory of sites MUST be derived from the code, because the originating plan's list is incomplete.
- **FR-007**: The swap-engine address MUST be removed from the Scenario A configuration templates and from the configuration reference documentation, so no operator is directed to set a variable that nothing reads.
- **FR-008**: The build target that deploys the swap engine MUST be removed, together with its declaration in any aggregate target list, leaving no build entry point that references a deleted script.
- **FR-009**: Removal MUST NOT alter the other scenario's swap engine, its tests, or any code outside Scenario A. The two engines are separate files; any cross-scenario change indicates an error.

**Preservation of the halt control (Scenario A)**

- **FR-010**: The breaker operations on the service interface, the gateway routes and handlers that expose them, and the governance status indicator MUST all be retained and MUST behave exactly as they do today.
- **FR-011**: The interface contract definition MUST NOT be modified, so that no contract regeneration is required. The generation toolchain is not available in this environment, which makes this a hard constraint rather than a preference.
- **FR-012**: The database-only path MUST become the sole implementation of the halt control, and its behaviour MUST be unchanged for every caller.
- **FR-013**: The halt control MUST be documented, in a place a maintainer encounters before working on it, as a governance flag that records a halt decision and has no swap engine behind it — and therefore no on-chain enforcement in Scenario A. Documenting this MUST NOT be deferred to a follow-up.
- **FR-014**: The recorded halt state MUST NOT change as a result of the removal, including in an environment that previously ran the on-chain path.

**Removal of dead portal surfaces (Scenario A)**

- **FR-015**: The unreachable governance breaker screen and its export MUST be removed. No route or navigation entry referenced it, so its removal MUST change no reachable behaviour.
- **FR-016**: The bank portal's commented-out swap and liquidity screens, their supporting data-access, state, hook and type modules, and their commented route and navigation entries MUST be removed.
- **FR-017**: The bank dashboard MUST stop invoking the pool refresh. Its rendered output MUST be unchanged, because the result of that call was never displayed.
- **FR-018**: The inert mock switch and the unimported sample data set in the Scenario A governance portal MUST be removed. Because nothing consumes them, this MUST change no behaviour.
- **FR-019**: Commented-out blocks in the Scenario A portals that are unrelated to the swap engine or the inert mock plumbing MUST be left untouched.
- **FR-020**: Both Scenario A portals MUST build and lint cleanly after removal, with no unused import, export or type left behind.

**Documentation accuracy**

- **FR-021**: The Scenario A governance manual's data-source statement MUST describe a single live data source and MUST NOT instruct the reader to determine which mode is active, because no selectable mode exists.
- **FR-022**: Documentation MUST NOT be left referring to a contract or a test suite that no longer exists in this repository. Two distinct corrections are required and MUST NOT be conflated:
  - **(a)** Statements claiming the engine is *present or tested here* — contract inventories, constructor-argument tables, and any claim of Foundry unit or fuzz coverage of the engine's invariants — MUST be corrected or removed, because deleting the suite makes them false.
  - **(b)** Statements describing the engine as the **other scenario's** capability, or as a planned hub capability, are already accurate and MUST be preserved. Most existing mentions fall in this category, so a blanket find-and-replace would corrupt correct text.
  Build-target references in documentation MUST also be checked: at least one runbook points at a deploy target that already fails and would then reference a deleted script.
- **FR-023**: The originating workstream SHOULD be recorded as complete in the rescope plan, which is an **untracked working document** held outside version control. Three constraints follow and MUST be honoured:
  - **(a)** The update MUST NOT be treated as a merge gate or an acceptance criterion for this feature, because no pull request can show it and no reviewer can verify it.
  - **(b)** The rescope plan MUST NOT be committed in order to satisfy this requirement. Keeping these documents untracked is a deliberate decision, and reversing it silently would be worse than leaving the record un-updated.
  - **(c)** Whoever performs the retirement MUST note, in the change description, that Workstream 2 is complete — so the fact is recorded somewhere a reviewer *can* see, even though the plan itself is not.

**Ordering and boundaries**

- **FR-024**: Every change MUST be confined to Scenario A and to shared documentation. No change may touch the other scenario's code.
- **FR-025**: This feature MUST NOT introduce any new runtime or development dependency.
- **FR-026**: The relationship with feature 043 MUST be recorded and honoured. There is exactly **one** real coupling, and it concerns a tracked file:
  - **(a) A wording collision on a tracked manual.** 043 corrects the same Scenario A governance data-source statement this feature revises, and was deliberately worded not to promise this removal. Whichever lands second MUST reconcile rather than overwrite, and the final wording MUST describe a portal with no mock plumbing at all.
  - **(b) There is no document dependency.** The rescope plan and ticket are untracked, so merging 043 neither provides nor withholds them. This feature has **no ordering precondition** beyond (a): it may land before or after 043.
- **FR-027**: Every newly created source file MUST carry the project's mandatory licence header. This feature is expected to create few or no source files, but the requirement applies to any it does create.
- **FR-028**: The removal MUST NOT change any settlement path. Scenario A's value flow is the hash-locked transfer path, which the swap engine never participated in.

### Key Entities

- **Vestigial swap engine**: A complete constant-product exchange implementation deployed in Scenario A that executes no swaps, because Scenario A settles through hash-locked transfers. Its only runtime consumer is the halt control it hosts. Distinct from the other scenario's swap engine, which is a separate and larger implementation that genuinely performs swaps.
- **Halt control (database-only)**: The governance capability that records whether the system is halted. It has two implementations today, selected by whether a swap engine is wired; after this feature it has one. Its externally visible behaviour is identical either way in Scenario A, because the database path is already the one that runs.
- **Dead portal surface**: A screen, or a supporting data-access, state, hook or type module, that no navigation reaches and no reachable screen consumes. Removing one changes nothing an operator can observe — which is precisely why its presence misleads.
- **Inert configuration switch**: A declared environment variable, or an exported flag, that no code reads. Indistinguishable from a working switch to anyone configuring the system, and therefore a defect in its own right.
- **Cross-scenario documentation reference**: A mention of the swap engine that is accurate because it describes the other scenario or the platform architecture. Must survive the retirement; only Scenario A capability claims are corrected.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A search of Scenario A for the swap engine by name returns zero matches in source, scripts, configuration templates and build targets — only deliberate, accurate cross-scenario or historical references remain in documentation.
- **SC-002**: The Scenario A contract project builds and its tests pass, and the Scenario A services build and their tests pass, with the swap engine and its chain client entirely absent.
- **SC-003**: The other scenario's contracts and services build and test exactly as before, demonstrating that the two swap engines were never shared — zero changes to files outside Scenario A and shared documentation.
- **SC-004**: The halt control and its status indicator behave identically before and after the removal, across toggling in both directions — no observable difference for a governance operator.
- **SC-005**: A maintainer can determine, from documentation alone and without reading code, that the Scenario A halt control is a governance flag with no on-chain enforcement.
- **SC-006**: The interface contract definition is byte-for-byte unchanged, so no regeneration is performed and none is required.
- **SC-007**: Both Scenario A portals build and lint with no new warning or error, and every route resolves to a reachable screen — zero unreachable screens remain.
- **SC-008**: The bank dashboard performs no swap-engine data call on load, and its rendered output is unchanged.
- **SC-009**: No environment template or configuration reference offers a swap-engine variable — zero inert variables of this kind remain in Scenario A.
- **SC-010**: The net change is a reduction: substantially more lines removed than added, with no new dependency and no new abstraction introduced.

## Assumptions

- The swap engine is genuinely vestigial in Scenario A. This was verified rather than assumed: nothing else in Scenario A imports it, the only script that constructs it does so standalone and creates no pools or liquidity, its only runtime consumer is the halt control, the bank screens that would use it are commented out and were only ever backed by a fixed sample data set, and the governance breaker screen is unreachable. Verification went **further** than the originating plan: that constructing script is legacy — its build target deliberately fails — and the live per-spoke deployment explicitly excludes hub-only contracts including the engine. So no working deployment path deploys it at all.
- The halt control's database-only path is already the path that runs in Scenario A, because the provisioning toolkit does not wire a swap-engine address. Removing the engine therefore permanently selects a path that is already in use, rather than changing behaviour.
- The interface contract stays untouched, so no generation toolchain is needed. This is load-bearing: the toolchain is absent from this environment and the generated code is committed, so a contract change could not be completed or verified here.
- Scenario A's swap engine and the other scenario's are separate implementations in separate files, so deleting one cannot affect the other. Verified by comparing them directly; they differ substantially in size and content.
- Deleting roughly 36 tests reduces the test count without reducing meaningful coverage, because those tests exercise a contract and a client that never execute in Scenario A. The other scenario retains its own equivalent suite for the engine it actually uses.
- A contract already deployed on a running chain is unaffected by deleting its source; it simply stops being referenced. No on-chain migration or cleanup is in scope.
- Feature 043 lands separately. The wording collision on the shared manual statement is reconciled by merge order rather than by coordinating branches, and the reconciliation is an explicit requirement rather than an assumption that it will not collide.
- The rescope plan and the R2-H-2 ticket are intentionally outside version control. Anything this feature "records" in them is therefore invisible to review, which is why FR-023 is a should-do local step with its verifiable part (the completion note) moved into the change description instead. This was verified, not assumed: both documents are tracked on no branch.
- Removing the pool refresh from the bank dashboard changes no rendered output, because the value it fetched is not read — the variable holding it is commented out.
- The lower-effort alternative of documenting the engine as inert was considered and rejected by the project owner in favour of full retirement.

---

## Amendment 1 — Re-implemented from `develop` (2026-09-02)

Everything above is the original specification, written on branch `044-scenario-a-amm-retirement`
on 2026-08-06. That branch **never opened a pull request** and its `specs/044/` never reached
`develop`. This feature was re-implemented from `develop` on branch
`feat/044-scenario-a-amm-retirement` (base `82cd6aa7`).

The original text is preserved rather than rewritten, because its analysis is what made the
re-implementation cheap. This amendment records every place reality diverged from it. Each item
below was measured, not assumed.

### Why the old branch could not simply be merged

`develop` had moved **262 commits** past that branch's base, and **8 of the 21 files the branch
deletes were modified on `develop` afterwards** — `modify/delete` conflicts, the AMM contract among
them. The changes were real work: `ac17f45b` (count institutions, not keys, in the AMM resume
quorum), `e8c24b59` (contract hygiene, PR #164) and `7207c9ac` (confirmation on money-creating
operations). Even the task line numbers had drifted: the make target moved from `:51` to `:59`, its
`.PHONY` entry from `:128` to `:151`.

Those commits are also the evidence that **the retirement decision had never actually been taken** —
the team kept investing in the engine for three weeks after the spec declared it vestigial. The
decision was taken on 2026-09-02, in favour of full retirement, as the Clarifications section
records.

### The premise was re-verified and still holds

All four legs, checked against `develop`: `AMMTradingPage` still commented out of the bank router
(`routes/index.tsx:55`); `CircuitBreakerPage` routed nowhere; no toolkit or samples step deploys the
engine (only the manual `contracts.deploy-amm-besu` target, itself removed here); `AMM_ADDRESS`
present only as a commented row in both Central Bank templates, with compliance logging
`on-chain circuit breaker disabled (database-only toggle)` and returning nil.

### The inventory was larger than the original found

The grep-derived inventory (T005) turned up **three removal sites the original never listed**,
because they landed after it was written:

| Site | Why it was missed |
|---|---|
| `contracts/test/invariant/AutomatedMarketMakerInvariant.t.sol` (7 tests), `AMMHandlerReachability.t.sol` (6 tests), `AMMHandler.sol` | A whole AMM invariant/fuzz suite added to `develop` after 2026-08-06 |
| `frontend/apps/bank/src/services/websocket/events.service.ts` and `types/events.types.ts` | The `amm.pool.updated` event type and its subscription — matched none of the planned grep patterns |
| `frontend/apps/bank/src/components/layout/Sidebar.tsx:29` | A commented-out nav entry for the deleted page |

### The test count was understated

FR-003 requires stating the deleted coverage openly, so the corrected figure replaces the estimate:

| | Original estimate | Measured |
|---|---|---|
| Contract tests | ~30 | **46** (33 in `AutomatedMarketMaker.t.sol` including `DeployAMMTest`, 7 invariant, 6 reachability) |
| Service tests | 6 | 6 (4 on-chain breaker, 2 chain client) |
| **Total** | ~36 | **52** |

Suite totals: **241 tests / 20 suites → 195 tests / 16 suites**. The fall of 46 is the AMM's tests
and nothing else — which is what the recorded baseline exists to prove.

### Scope changed in two places

- **FR-016 narrowed.** `LiquidityTransfersPage.tsx` was **not** deleted. It is dead (commented out
  of the router at line 34) but it is **not an AMM surface** — it imports `LiquidityRequestType`,
  `OnRampRequestStatus` and `useTokenStore`, and references the engine nowhere. FR-019 forbids
  touching commented-out blocks unrelated to the engine, and that governs: deleting unrelated dead
  code would be scope taken without being asked for.
- **A correction the original did not anticipate.** Roughly fifteen comments across the tree
  justified the `institutionId` invariant by "the AMM resume quorum counts distinct institutions".
  With Scenario A's engine gone that justification is false there, so each was reworded to the
  justification that does hold. The `institutionId` itself **stays**: `getInstitutionId` is read
  off-chain by `registry/besu.go:261` for the auth and compliance services, and the toolkit writes
  it at onboarding. This answers the open question about `ac17f45b`'s registry half — it keeps a
  live consumer.

### FR-017 confirmed by reading, not assumed

Only `refreshPool` was live on the bank dashboard — dead work on every page load whose result was
never read. The pool cards that appeared to render live were inside JSX comment blocks
(`{/* … */}` at lines 183-242, 244-320, 322-350), so removing them changes no rendered output.

### A latent defect this change exposed

After the AMM suites were removed the contract suite became **flaky**: one failure in three runs, in
`DeployFiatCBMTest`, a suite this feature never touched. It passed in isolation 3/3, and `develop`
passed 4/4 — so it could not be called pre-existing without evidence, and was investigated.

**Cause.** `vm.setEnv` writes the **process** environment and `forge` runs suites in parallel.
`CBWeb3Spoke.t.sol` and `CBWeb3Hub.t.sol` wrote `ADMIN_ADDRESS`/`CENTRAL_BANK_ADDRESS` as literals
while `FiatCentralBankMoney`, `TokenizedCentralBankMoney` and `ManualOracle` use
`makeAddr(...)` — so the literal writers clobber the others' deploy scripts. The failing run's log
printed `Admin: 0x1234…`, the literal value, where `makeAddr("admin")` was expected.
`TokenizedCentralBankMoney.t.sol:124` already documented the race and the convergence workaround.

The defect is pre-existing and latent; editing `CBWeb3Hub.t.sol` changed the interleaving and
surfaced it. Fixed by converging the two literal writers on the documented convention. Five
consecutive full runs are stable at 195. Reported as a separate, labelled fix rather than folded in,
because claiming "195 tests pass" with an unstable suite would be false.

### Not done, recorded rather than ticked

- **FR-023** (record the workstream complete in the rescope plan) was **not performed**.
  `docs/r2-h-2-rescope-and-plan.md` is an untracked working document and is not present in this
  working tree. The original text already makes this a should-do rather than a gate; it is recorded
  as not done instead of silently marked complete.
- **T053** (operator walkthrough on a running Scenario A stack) remains open, as on the original
  branch. It needs a live stack.
- **`docs/runbooks/SUPERVISOR-PORTAL-DOCS.md`** still documents pool-health screens and
  `/api/v2/amm/pool/{pair}/status` endpoints. Left untouched: that is documentation drift which
  predates this feature and deserves its own decision, not silent scope expansion here.
- **`docs/architecture/cbweb3-component-diagram-with-paladin.excalidraw`** still carries AMM boxes.
  Hand-editing diagram JSON is risky for little return; it needs opening in the tool.
- Three Go files were **already** unformatted on `develop`
  (`compliance/internal/domain/roles.go`, `toolkit/.../step_start_frontend_stack.go`,
  `step_start_backend_stack_test.go`). Confirmed pre-existing and left alone.

### Constitution re-check deltas (v1.0.4)

The six-gate assessment above stands. Two figures in it are corrected: Scenario A's and Scenario B's
engines are **330 and 602 lines** (not 304/558), still separate files with different hashes; and the
deleted-test figure is 52, not ~36. Principle I was made verifiable rather than argued —
`git diff --name-only` against `develop` returns **zero** files under `scenario-b/`, and that check
belongs in the pull request. The Principle V inversion was executed as designed: the
characterisation tests were written while both paths existed, passed then, and passed unmodified
afterwards.
