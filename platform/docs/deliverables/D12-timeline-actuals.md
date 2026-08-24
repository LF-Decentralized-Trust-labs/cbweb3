# D12 — Timeline Actuals and Deviations

**Finding:** R1-12.9 (P2) — deliverables review: "No defect log, UAT records, or timeline actuals."
**Scope:** Deliverable 12 test execution, both scenarios
**Plan of record:** [`scenario-a/docs/test-execution-plan.md`](../../scenario-a/docs/test-execution-plan.md) ·
[`scenario-b/docs/test-execution-plan.md`](../../scenario-b/docs/test-execution-plan.md) (both published 2026-05-29)
**Companion artifacts:** [defect log](D12-defect-log.md) · [UAT records](D12-uat-records.md) ·
[measured results](../../D12_results.md)
**Last updated:** 2026-08-21

## 1. Why this file exists

Both test execution plans state a nine-week, seven-phase timeline in **relative** weeks and
defer the calendar to the IDB Technical Committee. No anchor date was ever agreed, so
"Week 4" names nothing that can be checked, and the plans have no place to record what was
actually run and when. This file is that record: absolute dates for what executed, an
explicit list of where execution departed from the plan, and the phases that have not
started.

Every date below is taken from a commit, a recorded run artifact, or a CI workflow in this
repository. Where a phase produced no evidence, it says so rather than inferring one.

## 2. Planned versus actual

Phase names and windows are the plan's. Actual dates are the first and last dated evidence
for that phase.

| Phase | Planned | Actual window | Status | Evidence |
| --- | --- | --- | --- | --- |
| **0 — Mobilization** | Week 1 | 2026-05-29 → open | **Partially met** | Plan published 2026-05-29. Devnet bring-up is available and documented, but the provisioning path kept moving through August: per-entity infra credentials on 2026-08-20, then the retirement of the legacy `deploy/local` path on 2026-08-21 (`3b14ecaa`), which makes the toolkit the only way to provision and removes `make spoke-all` — the command this deliverable's own Phase 0 checklist still names (DEF-021). Bank UAT testers were never identified — the one Phase 0 item the banks own. |
| **1 — Unit Testing** | Week 2 | 2026-06-08 → 2026-06-19 (extended 2026-08-21) | **Met, then continuous** | Coverage/vet/gosec gates in CI from 2026-06-08 (`backend-scenario-{a,b}.yml`, `contracts-scenario-{a,b}.yml`). Measured coverage recorded 2026-06-19 — see [`D12_results.md`](../../D12_results.md) §A.1/A.2/B.1/B.2. Invariant suites added 2026-08-20 (Scenario B) and 2026-08-21 (Scenario A). |
| **2 — Integration Testing** | Week 3 | 2026-06-08 → 2026-06-19 | **Met, then continuous** | Hermetic `integration_lite` gated per PR from 2026-06-08. Live-infrastructure integration confirmed 2026-06-19 (`D12_results.md` §A.3/B.3). |
| **3 — E2E Core Flows** | Week 4 | 2026-06-19 (single capture) | **Met once; not currently reproducible** | `TestFullHappyPath` PASS both scenarios on 2026-06-19 — 8/8 phases, 56.1 s (A) and 63.1 s (B). Machine-readable bundles: `tools/gen_evidence_bundles.py`, added 2026-06-19. The capture ran on the legacy `deploy/local` stack, which was deleted on 2026-08-21; the test still names that topology and reads secrets the toolkit does not write, so it has no current reproduction path — DEF-007, restated the same day. |
| **4 — Performance & Security** | Week 5 | 2026-06-11 → 2026-08-11 | **Partially met** | Harness 2026-06-11. Scenario B run `20260618T162301Z` (2026-06-18, [`RESULTS.md`](../../scenario-b/docs/performance/RESULTS.md)); Scenario A run `20260619T160230Z` plus an AWS c6a.8xlarge confirmation run (2026-06-19, [`scenario-a/docs/performance/`](../../scenario-a/docs/performance/)). Security: gosec + Slither in CI from 2026-06-08; gitleaks gate 2026-07-23 with the [D11 report](D11-secret-scan-report.md) 2026-08-11. OWASP ZAP baseline: **not executed** (D-4). Six threshold findings remain open — DEF-001 … DEF-006. |
| **5 — User Acceptance Testing** | Weeks 6–7 | — | **Not started** | Zero UAT records on file; no bank testers named. See [UAT records](D12-uat-records.md) §5 for the preconditions that are still open. |
| **6 — Regression & Retest** | Week 8 | — (continuous instead) | **Not started as a phase** | There is no fix-and-retest cycle against a bank-signed defect list, because Phase 5 has not run. Regression itself is continuous: every PR re-runs the coverage, `integration_lite` and contract gates, and defects found by internal review were fixed and covered between 2026-06-19 and 2026-08-21 (see the [defect log](D12-defect-log.md)). |
| **7 — Sign-off & Submission** | Week 9 | — | **Not started** | No signed test report, no bank acceptance form. |

**Position as of 2026-08-21:** Phases 1–4 have executed and produced evidence; Phase 4 has
six open threshold findings and one missing scan; Phase 3 needs a local provisioning fix
before it can be re-run on demand; Phases 5–7 are unscheduled and blocked on participants
and preconditions rather than on platform work alone.

## 3. Deviations

Each deviation states what the plan says, what happened, and whether anything is owed.

**D-1 — The timeline has no calendar anchor.**
Both plans express the schedule as "Week 1 … Week 9" and defer real dates to the IDB
Technical Committee. No anchor was agreed, so no phase can be judged early or late; only
absolute dates are meaningful. This file records those. *Owed:* an agreed start date before
Phases 5–7 are scheduled.

**D-2 — Phases 1–4 ran as one overlapping burst, not four sequential weeks.**
Coverage, integration, E2E and performance evidence all land between 2026-06-08 and
2026-06-19, largely on one branch (`test/r1-12.3-perf-harness`). The work was done; the
sequencing was not. *Owed:* nothing — recorded so the dates are not read as four
consecutive weeks.

**D-3 — Phases 1, 2 and 4 became standing CI gates rather than one-time events.**
The plan models unit, integration and performance testing as phases that complete. In
practice coverage, `go vet`, `gosec`, Slither, `integration_lite` and the contract suites
re-run on every pull request, and the performance suites are re-runnable targets
(`make scenario-{a,b}.perf-all`). The phase model understates this. *Owed:* nothing.

**D-4 — Tooling substitution; one scan not executed.**
The plans name Hyperledger Caliper for contract benchmarking and OWASP ZAP for a baseline
DAST scan. Caliper was replaced by k6 plus a scenario-native harness, which is a
substitution with equivalent output. ZAP has **no** substitute: gosec and Slither are static
analysis and gitleaks is secret scanning, so no dynamic scan of the running API surface has
been performed. *Owed:* either execute a baseline DAST scan or record the decision not to.

**D-5 — Fuzz-run count differs between local and CI.**
The plans require ≥ 10 000 fuzz runs per invariant. The default Foundry profile does run
10 000 (`fuzz = { runs = 10000 }`), but CI sets `FOUNDRY_FUZZ_RUNS=256` to stay inside its
time budget, and the invariant suites run 256 sequences at depth 50. The plan's figure holds
for a local or dedicated run, not for the per-PR gate. *Owed:* nothing, provided the
distinction is stated when the numbers are reported.

**D-6 — No persistent test environment.**
All evidence comes from ephemeral devnet stacks. The AWS c6a.8xlarge used for the Scenario A
performance confirmation was a one-off host, not a standing environment. This is the open
finding R1-12.5 and it directly constrains Phase 5: bank testers cannot be handed a stack
that exists only while a developer's laptop is up. *Owed:* R1-12.5.

**D-7 — Phase 3 evidence is real but not currently reproducible, and the reason changed.**
`TestFullHappyPath` passed on 2026-06-19 against the legacy `deploy/local` stack. Through
August the blocker was authentication: the 2026-06-30 decision to accept only the OIDC
password grant refused the test's client credentials, and the legacy Keycloak init created
no users to authenticate as instead. That reading expired on 2026-08-21, when `3b14ecaa`
deleted the legacy path outright. Login is no longer the problem — the toolkit provisions
per-role users from `spec.adminUsers` and the samples walkthrough authenticates every
entity — but the test still targets the deleted topology (legacy entities and ports, plus
eight secrets from `backend/config/.env.infra.*` that the toolkit deliberately does not
write), and `scenario-a.test-integration` no longer brings a stack up. *Owed:* DEF-007, now
an E2E-to-toolkit migration rather than a Keycloak fix.

**D-8 — Phases 5–7 are unscheduled, and the blockers are not all platform-side.**
No bank UAT testers were identified in Phase 0; there is no persistent environment (D-6);
Phase 3 is not on-demand reproducible (D-7); Scenario B's plan assigns UAT to an "MLP
Portal" that does not exist as a frontend application; and the catalog still records
E2E-B-03 (commercial-bank swap path, US3) as Partial. *Owed:* see
[UAT records](D12-uat-records.md) §5.

**D-10 — The environment that produced the Phase 1–4 evidence no longer exists.**
Every dated capture in §2 was taken on the legacy `deploy/local` stack. That path — the
compose files, the Keycloak init, `make spoke-all`, and the tryout scripts the plan's Phase 3
checklist enumerates — was removed on 2026-08-21 (`3b14ecaa`, `a9f837b2`), leaving the
provisioning toolkit as the only way to stand a stack up. The evidence remains valid as a
record of what passed on the day; what is gone is the ability to re-run it by following the
plan. Reproduction now means the toolkit topology plus the migration tracked in DEF-007, and
the plan text that still names the retired commands is DEF-021. *Owed:* DEF-007 and DEF-021,
after which §2's commands describe something a reader can actually run.

**D-9 — Scenario B did not wait for a Scenario A baseline.**
Scenario B's plan states that its execution "begins after Scenario A baseline has been
established". The two ran in parallel: Scenario B's performance run (2026-06-18) preceded
Scenario A's (2026-06-19). No harm resulted — the scenarios are independent by
constitution — but the stated ordering was not followed. *Owed:* nothing; the plan
sentence is stale.

## 4. Keeping this file current

Update it when a phase produces new dated evidence, when a deviation is closed, or when a
date is agreed for Phases 5–7. Two rules:

1. **Cite, do not assert.** Every date must point at a commit, a recorded run, a workflow,
   or a signed record. If evidence does not exist, the phase stays "not started" — an
   undated claim of completion is worse than an admitted gap.
2. **Deviations are closed explicitly.** A deviation is removed only when what it says is
   owed has been delivered, and the deliverable is linked here.
