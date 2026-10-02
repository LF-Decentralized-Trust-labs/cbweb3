# Specification Quality Checklist

**Feature**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06
**Reviewed against**: [spec.md](../spec.md)

## Content quality

- [x] No implementation detail leaks into the requirements — file paths and symbol names live in [plan.md](../plan.md) and [tasks.md](../tasks.md), while the spec describes the swap engine, the halt control and the dead surfaces in behavioural terms
- [x] Written for the people affected: a maintainer misled by a swap engine that never runs, a governance operator whose halt control must not change, an operator or tester shown unreachable screens
- [x] Every mandatory section is present and populated
- [x] No unresolved `[NEEDS CLARIFICATION]` marker remains

## Requirement completeness

- [x] All four clarification questions are answered and recorded with their decisions
- [x] Requirements are testable — each maps to at least one task in the traceability table, and the deletion requirements are verified by a repository-wide grep plus green builds
- [x] Success criteria are measurable, including the awkward ones: zero grep matches, an empty proto diff, a test-count drop matching the deletions, and a net-negative line count
- [x] Success criteria avoid naming files or symbols, staying at the level of observable outcomes
- [x] Acceptance scenarios cover both halves of a deletion: that the removed thing is gone, and that everything remaining still works
- [x] Edge cases address the cases a deletion actually fails on — an environment still supplying the removed variable, a contract already deployed on a chain, a provisioning record containing the removed field, a halt recorded before the change, and documentation that is correct because it describes the other scenario
- [x] Scope boundaries are explicit: the interface contract is untouchable, the other scenario is out of bounds, the live per-spoke deploy script must not be modified, and unrelated commented blocks are left alone
- [x] Dependencies are stated and **graded** — FR-026 records the one real coupling with feature 043 (a wording collision on a tracked manual) and records explicitly that there is no document dependency, because the rescope plan is untracked on every branch

## Feature readiness

- [x] Each requirement traces to a user story and to at least one task
- [x] The three user stories are prioritised, and the P1 pair is ordered so the characterisation precedes the removal
- [x] Measurable outcomes cover all three stories plus the cross-cutting boundaries
- [x] No speculative requirement: everything asserted about the current code was verified, and the findings that contradicted the originating plan are recorded in [research.md](../research.md)

## Deletion-specific checks

These are not in the standard template. A retirement fails in ways an additive feature does not, so they are checked explicitly.

- [x] **The claim of vestigiality is evidenced, not assumed.** Seven independent checks are recorded in [research.md](../research.md) R1 — no importers, no pools or liquidity created, a single runtime consumer, an empty library stub, unreachable screens, and the scenario's own documentation already disclaiming the contract.
- [x] **Safety of the removal is proven before it happens.** Principle V is inverted into characterisation (Phase 3 before Phase 4), because the breaker's own on-chain tests are among the files being deleted and a regression could otherwise vanish with its coverage.
- [x] **The cost is stated, not hidden.** Roughly 36 deleted tests are declared in FR-003 and re-checked in T051, so a falling test count reads as intended rather than accidental.
- [x] **"No behaviour change" is verifiable rather than asserted.** For the bank dashboard it rests on the variable holding the fetched value being commented out; for the inert mock plumbing, on there being zero consumers; for the halt control, on the database path already being the one that runs.
- [x] **The inventory is derived from code.** The originating plan's list was found incomplete in six places **and wrong in two** — it names a bank page barrel and a `useAMM` barrel export that do not exist — so FR-006 and T005 require rebuilding it rather than trusting it.
- [x] **The verification method matches what can actually be run.** The script that constructs the engine is legacy behind a deliberately failing build target, so the edit is verified by compiling and running its test rather than by running the script. The obvious-looking check (`make -n contracts.deploy-all`) is explicitly rejected in the quickstart because it would pass whether or not the work was done.
- [x] **An unverifiable requirement is marked unverifiable, not quietly treated as a gate.** FR-023 targets a document held outside version control, so its update can never appear in a diff. It is downgraded to a should-do, split so the reviewer-visible half lands in the change description, and carries an explicit prohibition on the tempting workaround — committing the plan to buy a checkbox.
- [x] **A hard external constraint is recorded as such.** The absent generation toolchain makes the untouched interface contract a requirement (FR-011) with its own verification task (T030), not a stylistic preference.

## Notes

**Reviewed against the repository on 2026-08-06** (branch `044-scenario-a-amm-retirement`, cut from `develop`). Every path the plan cites was confirmed to exist, and five substantive corrections were folded back into all artifacts in one pass:

1. The rescope plan and R2-H-2 ticket are held outside version control — untracked on every branch by decision of the project owner — so FR-023 is unverifiable rather than order-dependent, and this feature has no document precondition at all.
2. No working deployment path deploys the engine — the constructing script is legacy behind a failing target, and the live spoke script excludes it by design. This changed both the risk profile and the verification method.
3. The bank portal has no page barrel, while three other barrels do export the engine's modules, and its hook is already fully orphaned.
4. The bank sample data set requires a partial edit, not a deletion.
5. The documentation defect is stale presence and coverage claims — most notably a test plan asserting Foundry coverage of the engine — not mis-attribution, which is mostly already correct.

**One requirement is unverifiable** and is marked as such: FR-023 updates a document that is intentionally not in the repository. That is a property of the working arrangement, not a defect in the specification — it is recorded with a status check, a two-part task that puts the reviewer-visible half in the change description, and the workaround explicitly ruled out.

**Nothing is blocked.** This feature has no ordering precondition; it may land before or after feature 043, subject only to reconciling one shared manual statement.

The specification, plan and tasks are consistent with the repository as it stands and ready for implementation.
