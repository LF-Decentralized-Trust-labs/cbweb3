# Feature Specification: Circuit-Breaker Transaction-Hash Visibility and Mock-vs-Live Documentation Reconciliation

**Feature Branch**: `043-breaker-txhash-mock-docs`
**Created**: 2026-08-05
**Status**: Draft
**Input**: Residual scope of code-review finding R2-H-2, re-scoped per [`docs/r2-h-2-rescope-and-plan.md`](../../docs/r2-h-2-rescope-and-plan.md). Covers Workstream 1 (Scenario B transaction-hash visibility) and Workstream 3 (documentation reconciliation) of that plan. Workstream 2 (Scenario A vestigial-AMM retirement) is explicitly excluded by decision of the project owner.

## Clarifications

### Session 2026-08-05

- Q: How much of the rescope plan should this specification cover? → A: Workstream 1 and Workstream 3 only. Workstream 2 (Scenario A vestigial-AMM retirement) is excluded and remains available as separate future work.
- Q: Surfacing a reference for the resume proposal requires widening the shared component that talks to the chain, because it currently returns only the proposal identifier and discards the receipt. How far should this go? → A: Widen it, so all three write actions (pause, propose resume, sign resume) carry an auditable on-chain reference.
- Q: Breaker actions currently write no governance audit-trail entries at all. Should audit-trail writes be added here? → A: No — out of scope. The absence of audit-trail entries for breaker actions is a pre-existing gap unrelated to this review finding, and is recorded here for separate triage.
- Q: Which action's on-chain reference should the breaker view display? → A: The pair's most recent breaker action, regardless of which institution performed it, so the reference survives reload and every central bank sees the same value.

### Discovered during clarification

The originating plan assumed the on-chain reference was already available at the boundary for every action. Verification against the code refined this, and the difference shapes the work:

| Write action | Reference available today | Consequence |
|---|---|---|
| Pause | Yes, obtained and then discarded above the boundary | Propagation only |
| Sign resume | Yes, obtained at the chain boundary and discarded there | Boundary contract must widen to return it |
| Propose resume | No — the identifier of the proposal is returned instead, and the receipt carrying the reference is discarded | Shared chain-facing component must return both |
| Execute resume | None exists — it performs no chain call, because resumption happens automatically once the final signature lands | Correctly has no reference; not a gap |

### Discovered during review pass

A second verification pass against the running code surfaced two further facts that change what the work must do:

- **The at-a-glance indicator is network-wide, but the authoritative status is per-pair.** The indicator today reads a pair-less source and renders a global claim ("swaps are globally halted"). The authoritative source is scoped to one currency pair. Making the two agree therefore requires deciding what "halted" means network-wide, not merely swapping one data source for another. Resolved as FR-012: the indicator reports halted when **any** pair is halted. All three places that render the indicator read only the halted/operational condition, so no other information is lost in the switch.
- **The governance portal cannot currently run component-rendering tests.** Its test runner is configured for plain module tests with no browser-like environment and no component-testing library. Introducing them would add new development dependencies, which this project requires justification for. Resolved by testing the portal's data-handling logic — the parts that capture, retain and derive the reference and the indicator state — and verifying rendering manually. See the testing note in Assumptions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Governance operator can audit the on-chain action behind a breaker decision (Priority: P1)

A central bank governance operator pauses cross-border swaps for a currency pair during an incident. The pause is a consequential, network-affecting act that is recorded on-chain. Today the operator sees only that the state changed to halted; they have no way to identify the on-chain record of their own action, so they cannot cite it in an incident report, cross-check it against the ledger, or hand it to an auditor. The same blindness applies to proposing and signing a resume.

After this change, every breaker action the operator takes returns a visible on-chain transaction reference, shown alongside the resulting state and copyable in one gesture.

**Why this priority**: This is the only genuine functional gap remaining from R2-H-2, and it is a direct obligation under the project's breaker-lifecycle observability rule. The data is already captured at the server boundary and then discarded, so the value is high and the change is contained.

**Independent Test**: Perform a pause on a chain-wired Scenario B stack and confirm the resulting transaction reference is displayed and matches the record on the ledger. Delivers auditability on its own, with no dependency on any other story.

**Acceptance Scenarios**:

1. **Given** a chain-wired environment and an operational pair, **When** the operator pauses the breaker, **Then** the resulting on-chain transaction reference is displayed with the new halted state and can be copied in one action.
2. **Given** a halted pair, **When** the operator proposes a resume, **Then** the transaction reference for that proposal is displayed alongside the awaiting-quorum state.
3. **Given** a resume proposal awaiting a second signature, **When** a second, distinct central bank signs, **Then** the transaction reference for the signing action is displayed alongside the resulting state.
4. **Given** any prior breaker action on a pair, **When** the operator loads or refreshes the breaker view, **Then** the most recent action's transaction reference is shown as part of the current status.
5. **Given** an environment with no chain wired, **When** the operator performs a breaker action, **Then** the action still succeeds and the absence of a transaction reference is handled gracefully, with no error and no empty-looking artefact presented as if it were a real reference.
6. **Given** any breaker action, **When** the transaction reference is unavailable or fails to render, **Then** the breaker action itself still completes and the resulting state is still shown — visibility of the reference never gates the control.

---

### User Story 2 - Operators see one consistent breaker state everywhere (Priority: P2)

An operator glances at the persistent status indicator in the portal chrome to judge whether swaps are currently halted. Today that at-a-glance indicator is fed from a different, mock-capable source than the dedicated breaker screen, so in a default configuration the indicator can display synthetic or stale state while the breaker screen shows the true on-chain state. An operator who trusts the indicator can reach the opposite conclusion from an operator who opens the screen.

After this change, the indicator and the breaker screen report the same state from the same authoritative source.

**Why this priority**: A control surface that contradicts itself about whether the network is halted is a safety problem, not a cosmetic one. It is ranked below Story 1 only because Story 1 is the tracked R2-H-2 residual; the fix here is small and closely related.

**Independent Test**: With the portal in its default configuration, halt a pair and confirm the chrome indicator and the breaker screen agree. Testable without Story 1.

**Acceptance Scenarios**:

1. **Given** the portal in its default configuration, **When** a pair is halted, **Then** the at-a-glance indicator reports halted, matching the breaker screen.
2. **Given** the portal in its default configuration, **When** no pair is halted, **Then** the indicator reports operational, matching the breaker screen.
3. **Given** more than one pair exists and **one** of them is halted, **When** the operator views the indicator, **Then** it reports halted — the indicator is a network-wide claim, so any halted pair halts it.
4. **Given** a halted pair is resumed while other pairs remain operational, **When** the indicator next refreshes, **Then** it returns to operational.
5. **Given** any configuration of the mock toggle, **When** the operator compares the indicator with the breaker screen, **Then** the two never disagree about whether swaps are halted.
6. **Given** no pairs exist yet, or the pair list cannot be retrieved, **When** the operator views the indicator, **Then** it reports an unknown or operational condition without asserting a halt, and without breaking the surrounding layout.

---

### User Story 3 - Readers can trust the manuals about whether a portal shows real data (Priority: P2)

An operator, tester, or reviewer opens a portal manual to learn whether the screens in front of them are backed by the live network or by synthetic data. Four separate statements across the manual set currently contradict the code: one portal is documented as live-by-default when its mock path is in fact dead code, another is documented as having no mock capability at all when it has one that is on by default, a third is documented as live while its own settings screen tells the operator that mock services are enabled, and the summary table attributes the mock capability to the wrong portal entirely. A reader cannot currently determine from the documentation which data they are looking at.

Separately, two environment templates advertise a configuration switch that no code reads, inviting operators to configure something inert.

After this change, each manual states the actual behaviour of its portal, the summary table agrees with the manuals, the settings screen agrees with the manual, and the dead switch is gone.

**Why this priority**: Wrong statements about data provenance directly mislead the people validating the platform, and one of them is actively contradicted by the product's own UI. It is ranked alongside Story 2 because it carries no functional risk to the settlement path.

**Independent Test**: Cross-read each of the four documentation statements against the corresponding code path and confirm agreement. Fully testable on its own; no dependency on Stories 1 or 2.

**Acceptance Scenarios**:

1. **Given** a reader consulting any portal manual's data-source statement, **When** they compare it against that portal's actual behaviour, **Then** the statement is accurate, including which default applies when the toggle is unset.
2. **Given** a reader consulting the manual summary table, **When** they compare it against the individual manuals, **Then** the table agrees with them and attributes the mock capability to the correct portal.
3. **Given** an operator viewing the bank portal's settings screen, **When** they read its environment statements, **Then** those statements describe the portal's actual behaviour and do not contradict the bank manual.
4. **Given** an operator configuring a bank portal environment from its template, **When** they review the available variables, **Then** no variable is offered that has no effect.
5. **Given** a reader of the originating review ticket, **When** they read it, **Then** it records that its scenario attribution was found to be inaccurate, so the ticket is not re-actioned against the wrong scenario.

---

### Edge Cases

- **No chain wired.** The breaker continues to operate against its off-chain projection. Actions succeed, and the absence of a transaction reference is represented as genuinely absent rather than as a blank or placeholder value that could be mistaken for a real reference.
- **Chain call succeeds but returns no usable reference.** Treated identically to the no-chain case: the action stands, the reference is reported absent.
- **Chain call fails.** The failure surfaces as a failed action with its reason. No partially-applied state is reported as success.
- **Status requested for a pair that has never had a breaker action.** No transaction reference is reported; this is a normal empty state, not an error.
- **Resume quorum greater than two.** Out of scope to fix. The current signing behaviour reports the resumed state after the final expected signature in a two-signature quorum; for any larger quorum it would report resumed prematurely. This must be recorded as a known limitation so a future quorum change does not inherit a silent defect.
- **Operator has no copy affordance available** (for example, a restricted browser context). The reference remains visible and selectable, so it is never locked behind the copy control.
- **No pairs exist, or the pair list cannot be retrieved.** The network-wide indicator must not claim a halt it cannot substantiate; it degrades to unknown or operational and recovers on a later refresh (FR-014).
- **One pair halted, others operational.** The network-wide indicator reports halted, because it is a claim about the network rather than about the pair the operator happens to have selected (FR-012).
- **A single pair's status cannot be retrieved while others can.** The indicator must not silently report operational on partial information; an indeterminate pair is treated as not-known-halted and the condition is surfaced rather than swallowed.
- **Documentation drift after this change.** Each corrected statement must be traceable to the specific behaviour it describes, so that a future behavioural change makes the inaccuracy findable rather than silently reintroducing drift.

## Requirements *(mandatory)*

### Functional Requirements

**Transaction-hash visibility (Scenario B)**

- **FR-001**: The system MUST retain the on-chain transaction reference produced by each of the three breaker write actions — pause, resume proposal, and resume signature — instead of discarding it.
- **FR-002**: The system MUST return the on-chain transaction reference to the caller for each of the three breaker write actions. Where the reference is currently discarded at or below the chain-facing boundary, that boundary MUST be widened to carry it outward.
- **FR-003**: For the resume proposal, the system MUST return both the proposal identifier and the on-chain transaction reference. Returning the identifier alone is insufficient, and the reference MUST NOT be obtained by re-querying the chain when it is already present in the original result.
- **FR-004**: Finalising a resume MUST NOT be expected to yield a transaction reference, because it performs no chain call — resumption occurs automatically once the final required signature is recorded. Its absence MUST NOT be reported as an error or a missing value.
- **FR-005**: The system MUST include the pair's most recent breaker action's on-chain transaction reference when reporting current breaker status for that pair, regardless of which institution performed that action.
- **FR-006**: The governance portal MUST display the on-chain transaction reference for the pair's most recent breaker action together with the current state, so the value survives a page reload and is identical for every central bank viewing the same pair.
- **FR-007**: The governance portal MUST allow the displayed transaction reference to be copied in a single action, and MUST keep it selectable so it remains obtainable if the copy control is unavailable.
- **FR-008**: The governance portal MUST present the transaction reference as a plain reference value, not as a hyperlink, because no block-explorer location is configured in this environment. The presentation MUST be able to become a link later without reworking how the value is obtained.
- **FR-009**: Absence of a transaction reference MUST be represented distinctly from an empty or unknown value, so an operator can tell "this environment records no on-chain reference" apart from "a reference exists but was not shown".
- **FR-010**: Displaying, copying, or failing to obtain a transaction reference MUST NOT block, delay, or alter the outcome of the breaker action itself.

**Breaker state consistency (Scenario B)**

- **FR-011**: The at-a-glance breaker indicator in the portal chrome MUST derive its state from the same authoritative source as the dedicated breaker screen, and MUST NOT depend on any source that can be served from synthetic data.
- **FR-012**: Because the indicator makes a network-wide claim while the authoritative status is per-pair, the indicator MUST report halted when **any** known pair is halted, and operational only when no known pair is halted.
- **FR-013**: The at-a-glance indicator MUST NOT be capable of displaying synthetic state while the dedicated breaker screen displays live state.
- **FR-014**: When no pairs exist, or the set of pairs cannot be determined, the indicator MUST NOT assert that swaps are halted. It MUST degrade to an unknown or operational condition without breaking the surrounding layout, and MUST recover on a later refresh.

**Documentation and configuration accuracy**

- **FR-015**: Each portal manual's data-source statement MUST accurately describe whether that portal is backed by live data, including which behaviour applies when the mock toggle is unset.
- **FR-016**: The Scenario A governance manual MUST describe that portal as live-backed, and MUST NOT describe a selectable mock mode, because no code consumes that portal's mock path. It MUST NOT state or imply that the inert flag or unused mock data will be removed, as that removal is out of scope here.
- **FR-017**: The Scenario B governance manual MUST state that a mock toggle exists and is consumed, and MUST state the default that applies when the toggle is unset or set to any value other than the exact disabling value.
- **FR-018**: The Scenario B bank manual and the bank portal's own settings screen MUST agree with each other and with the portal's actual behaviour; statements describing mock services or simulated events MUST be removed or corrected where they do not reflect that behaviour.
- **FR-019**: The manual summary table MUST agree with the individual manuals and MUST attribute the live mock toggle to the portal that actually has one.
- **FR-020**: Environment templates MUST NOT declare configuration variables that no code reads; the inert mock variable MUST be removed from both bank portal templates.
- **FR-021**: The originating review ticket MUST be annotated to record that its scenario attribution is inaccurate, specifically that the mock-default behaviour it attributes to Scenario A governance in fact belongs to Scenario B governance.

**Known limitations**

- **FR-022**: The premature resumed-state report that would occur for any resume quorum greater than two MUST be recorded as a known limitation in a location a maintainer will encounter before changing the quorum. It MUST NOT be fixed under this specification.
- **FR-023**: The absence of governance audit-trail entries for breaker actions MUST be recorded as a discovered pre-existing gap, with enough detail for separate triage. It MUST NOT be fixed under this specification.

**Boundaries**

- **FR-024**: Changes to Scenario B behaviour MUST NOT alter Scenario A behaviour, and the single Scenario A documentation and template correction MUST be separable from the Scenario B changes for independent review.
- **FR-025**: The change MUST NOT alter the breaker's decision model: pause remains a single-actor emergency action and resume remains a multi-party quorum. Only the visibility of, and confidence in, those actions changes.
- **FR-026**: The change MUST NOT introduce new runtime or development dependencies. Any verification that would require one MUST be achieved another way or performed manually.
- **FR-027**: Every newly created source file MUST carry the project's mandatory licence header, which is enforced in continuous integration for the affected file types.

### Key Entities

- **Breaker action**: A governance act against a currency pair — pause, resume proposal, or resume signature — performed by an identified institution, producing a resulting state and, where a chain is wired, an on-chain transaction reference. Finalising a resume is not a breaker action in this sense: it performs no chain call and carries no reference.
- **Breaker status**: The current condition of a pair — operational, halted, or awaiting resume quorum — together with who initiated it, why, how many signatures a pending resume has collected against how many it needs, and the reference for the pair's most recent action by any institution.
- **On-chain transaction reference**: The ledger identifier of a breaker action. Present only where a chain is wired; absent otherwise. Never a precondition for the action. Distinct from the **resume proposal identifier**, which names a specific resume proposal so it can be signed — both are returned when a resume is proposed, and neither substitutes for the other.
- **Network-wide breaker condition**: A derived, not stored, value — halted if any known pair is halted, operational if none is, and indeterminate if the set of pairs is unknown. This is what the chrome indicator renders; it has no single authoritative record of its own because the authoritative state is held per pair.
- **Data-source statement**: The per-portal claim in a manual about whether the screens show live or synthetic data, which must correspond to that portal's real behaviour.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For every breaker action performed in a chain-wired environment, the operator can obtain the corresponding on-chain reference from the portal without leaving it or consulting logs — 100% of pause, resume-proposal and resume-signature actions.
- **SC-002**: An operator can produce the on-chain reference for the pair's most recent breaker action in a single copy gesture, in under 10 seconds from the action completing, and can still do so after reloading the view.
- **SC-003**: An auditor reconstructing a breaker incident can match every portal-reported action to a ledger record using only what the portal displays, with no gaps.
- **SC-004**: The at-a-glance indicator and the dedicated breaker screen report the same halted/operational condition in 100% of observations, in every supported configuration, including when several pairs exist and only one is halted.
- **SC-005**: Breaker actions succeed at the same rate as before the change in environments with no chain wired — reference visibility introduces no new failure mode.
- **SC-006**: Every data-source statement in the manual set matches the behaviour of the portal it describes, verified statement by statement — 4 of 4 corrected, 0 remaining contradictions.
- **SC-007**: No environment template offers a variable that has no effect — 0 inert variables remaining in the bank portal templates.
- **SC-008**: A reader of the originating ticket can determine which scenario each of its findings actually applies to, without re-deriving it from the code.

## Assumptions

- The breaker's existing decision model is correct and is not under revision; only observability and reporting consistency change. This follows the rescope plan's verified finding that the quorum model already meets the review's requirements.
- The on-chain reference is produced by the chain interaction that already happens for each write action, so this work carries an existing value outward rather than performing any additional chain call. For the resume proposal the value is present in the transaction receipt but currently discarded, so carrying it outward widens a shared boundary — it still adds no chain round-trip.
- No block-explorer location is configured for this environment, so the reference is presented as a plain value. Should one be configured later, turning the value into a link is a presentation change only.
- Resume quorum is two in all current environments, so the premature-resumed-state limitation is latent rather than active, and documenting it is sufficient for now.
- The Scenario A governance portal's mock path is inert — nothing consumes the toggle and nothing imports the mock data. Its removal is deliberately out of scope, so documentation must describe the portal's behaviour without promising cleanup.
- No bank portal code reads the mock toggle in either scenario, so removing it from those templates changes no behaviour.
- The Scenario A vestigial-AMM retirement described as Workstream 2 of the rescope plan is excluded from this specification and remains available as separate future work.
- No interface-contract regeneration is required, because the affected flow is the current governance interface rather than the legacy compliance contract.
- Correcting the manuals here is compatible with the in-flight user-manual screenshot work on other branches; the statements corrected here are distinct from the screenshot-driven text already amended elsewhere.
- **Portal verification is logic-level, not render-level.** The governance portal's test setup runs plain module tests with no browser-like environment and no component-testing library, and FR-026 forbids adding dependencies. Portal requirements are therefore verified by testing the data-handling logic — capturing the reference from each response, retaining it across a status refresh, and deriving the network-wide condition — with visual rendering confirmed manually against the walkthrough. This is a deliberate trade, not an omission: the logic that could silently drop the reference is covered, and the part left to manual checking is the part a human must look at anyway.
- The number of currency pairs in any environment is small, so deriving the network-wide condition by consulting each pair's status is acceptable and needs no new aggregate interface.
- All three places that render the at-a-glance indicator consume only the halted/operational condition, so changing where that condition comes from loses no other information they display.
