# D12 — UAT Records

**Finding:** R1-12.9 (P2) — deliverables review: "No defect log, UAT records, or timeline actuals."
**Scope:** Deliverable 12, Phase 5 (User Acceptance Testing) and the Phase 7 acceptance sign-off
**Companion artifacts:** [defect log](D12-defect-log.md) · [timeline actuals](D12-timeline-actuals.md)
**Last updated:** 2026-08-21

## 1. Status

**No UAT has been executed. There are zero records on file.**

Phase 5 is bank-led by design — LNET supports it and fixes what it finds, but the banks
execute it and the banks sign it off. It has not started: no UAT testers were identified in
Phase 0, and the preconditions in §5 are open. This file is therefore the **structure** the
phase will use, with the record tables present and empty rather than absent. An empty table
with a defined shape is a schedulable artifact; a missing one is a finding.

## 2. Participants and coverage

From the two plans' Phase 5 matrices. Each row is one UAT record set.

### Scenario A — Enhanced Correspondent Banking

| Entity | Portal | Coverage to validate |
| --- | --- | --- |
| bank-a | Bank | Onboarding, FX agreement initiation, balance view |
| bank-b | Bank | FX agreement acceptance, HTLC settlement |
| bank-c | Bank | Onboarding, token transfers |
| bank-d | Bank | Onboarding, token transfers (custodian leg) |
| central-bank-a | Treasury | Deposit, escrow and redeem approval |
| central-bank-b | Treasury | Equivalent flows to central-bank-a |
| Governance officer | Governance | Participant approval, policy configuration |
| Supervisor | Supervisor | Regulatory oversight views, audit log |
| NOC operator | NOC | Network monitoring, health alerts |

### Scenario B — International Hub

| Entity | Portal | Coverage to validate |
| --- | --- | --- |
| bank-a | Bank | Onboarding, FX swap initiation via AMM, bridge deposit, balance view |
| bank-b | Bank | FX swap acceptance, bridge withdrawal, balance view |
| central-bank-a | Treasury | Pair approval, liquidity provisioning, hub oversight |
| central-bank-b | Treasury | Equivalent flows to central-bank-a |
| MLP | **unresolved — see §5** | Bilateral commit registration, LP share management, rebalancing |

The scenarios are validated separately, with separate records and separate sign-offs. They
are separate products; one bank's acceptance of Scenario A says nothing about Scenario B.

## 3. Record formats

### 3.1 Case execution record

One row per case executed by one tester. This is the table a tester fills in during a
session.

| Field | Rule |
| --- | --- |
| **UAT-ID** | `UAT-<A\|B>-<entity>-nnn`, e.g. `UAT-A-bank-a-004`. Never reused. |
| **Case id** | The catalog id being exercised (`E2E-A-03`, `INT-API-B-08`, …) or `MANUAL` with a one-line description. |
| **Tester** | Name and role at the entity. A record with no named human is not a record. |
| **Date / time** | ISO-8601 with timezone. |
| **Stack / run id** | Which environment and run the session executed against. |
| **Steps deviated** | Empty if the documented path was followed; otherwise what was done instead. |
| **Expected** | The documented expected result. |
| **Observed** | What actually happened. |
| **Result** | Pass / Fail / Blocked / Not executed — defined below. |
| **Evidence** | `X-Correlation-Id`, transaction hash, evidence-bundle run id, or screenshot path. |
| **Defects raised** | `DEF-UAT-nnn` ids, or empty. |

**Result values.** *Pass* — observed matches expected exactly, with no side effect on the
ledger or the privacy layer, inside the time budget. *Fail* — anything else, and a defect
row must exist. *Blocked* — the case could not be started because a dependency failed; name
the blocking defect. *Not executed* — deliberately skipped; say why.

Pass and Fail are binary, as both plans specify. There is no partial pass; a case that
half-worked is a Fail with a Minor defect if the ledger outcome was right.

### 3.2 Session record file

One file per entity per session, committed under `docs/deliverables/uat/`, named
`uat-<scenario>-<entity>-<YYYY-MM-DD>.md`. Screenshots go in
`docs/deliverables/uat/img/<scenario>/<entity>/`. Machine-readable run evidence stays in
`evidence-bundles/` (build output, regenerated — reference it by run id, do not copy it in).

```markdown
# UAT session — Scenario <A|B> · <entity> · <YYYY-MM-DD>

**Tester(s):** <name, role>          **Portal:** <portal>
**Environment:** <stack / run id>    **LNET support present:** <yes/no, who>
**Session window:** <start>–<end> <tz>

## Cases executed

| UAT-ID | Case id | Expected | Observed | Result | Evidence | Defects |
| --- | --- | --- | --- | --- | --- | --- |

## Summary

Executed: <n> · Pass: <n> · Fail: <n> · Blocked: <n> · Not executed: <n>

## Notes

<observations that are not defects: usability friction, wording, documentation gaps>
```

### 3.3 Defect intake

A Fail produces a `DEF-UAT-nnn` row in the [defect log](D12-defect-log.md) §7. The
admissibility requirements — correlation id, ordered reproduction steps, expected versus
observed, amounts, timestamp — are in that file's §5, and the SLA the bank is owed (4 h
acknowledgement, 24 h resolution for Critical and Major) is in its §3.

Records and defect reports contain participant identities, account references and amounts.
They are internal project artifacts: they are not shared outside the project, and no client
or institution name goes into a public channel or an external tool.

### 3.4 Acceptance (sign-off) form

One per entity per scenario, at the end of Phase 6 — after the defects that entity raised
have been fixed **and retested by that entity**.

```markdown
# UAT acceptance — Scenario <A|B> · <entity>

**Entity:** <legal name>             **Scenario:** <A|B>
**UAT window:** <start> – <end>      **Sessions:** <list of session record files>
**Cases in scope:** <n>              **Passed:** <n>   **Failed:** <n>   **Blocked:** <n>

## Defects raised by this entity

| DEF-UAT id | Severity | Status | Retested by | Date |
| --- | --- | --- | --- | --- |

## Decision

- [ ] **Accepted** — all cases in scope pass; no Critical or Major defect open.
- [ ] **Accepted with conditions** — conditions listed below, each with an owner and a date.
- [ ] **Not accepted** — reasons listed below.

## Conditions / reasons

1. …

**Signed:** <name>   **Role:** <role>   **Date:** <ISO-8601>
```

"Accepted with conditions" is a real outcome and must not be recorded as "Accepted". A
condition with no owner and no date is not a condition.

### 3.5 Consolidated sign-off (Phase 7)

The single table that goes into the Deliverable 12 submission.

| Scenario | Entity | Cases | Pass | Fail | Open Critical/Major | Decision | Signed by | Date |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| _(none — Phase 5 not executed)_ | | | | | | | | |

## 4. Record register

Every session record, as it is added.

| Session record | Scenario | Entity | Date | Cases | Pass / Fail / Blocked | Defects raised |
| --- | --- | --- | --- | --- | --- | --- |
| _(none)_ | | | | | | |

## 5. Preconditions before Phase 5 can start

These are open, and each blocks or degrades UAT. They are not all platform-side.

| # | Precondition | State as of 2026-08-21 | Owner |
| --- | --- | --- | --- |
| 1 | UAT testers named per entity and per portal role | Not done — the one Phase 0 item the banks own | Banks |
| 2 | An environment that outlives a session, with seeded balances and registered identities | Not available — all evidence to date is from ephemeral devnet stacks; finding R1-12.5 is open (see [timeline actuals](D12-timeline-actuals.md) D-6) | LNET |
| 3 | Portal login working for real users on that environment | Blocked — DEF-007: local Keycloak provisions no users, and only the password grant is accepted | LNET |
| 4 | The E2E happy path reproducible on demand, so a UAT failure can be told from an environment failure | Blocked by 3 | LNET |
| 5 | Scenario B: which portal the MLP tester uses | Unresolved — the plan names an "MLP Portal"; `scenario-b/frontend/apps/` ships bank, governance, supervisor, treasury and noc. The MLP has its own backend stack and gateway, so its UAT is API-driven unless a portal decision is made | LNET + project lead |
| 6 | Scenario B: commercial-bank swap path (US3 / `E2E-B-03`) | The catalog records it as **Partial** — governance path works, commercial-bank direct path in progress. A bank cannot accept a flow that is not finished | LNET |
| 7 | An agreed calendar for Phases 5–7 | Not agreed (see [timeline actuals](D12-timeline-actuals.md) D-1) | LNET + IDB Technical Committee |
| 8 | Phase 4 threshold findings dispositioned — fixed, or a revised target accepted in writing | Six open: DEF-001 … DEF-006 | LNET + project lead |

Items 1–4 are hard blockers: without them there is no UAT to record. Items 5–8 shape scope,
and each one left open narrows what the banks can be asked to accept.
