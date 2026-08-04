# Feature Specification: Admission Profile for Commercial-Bank Onboarding

**Feature Branch**: `042-admission-onboarding-profile`
**Created**: 2026-08-03
**Status**: Draft
**Input**: User description: "Introduce an Admission profile (ROLE_ADMISSION) that takes over commercial-bank onboarding management from the central-bank governance profile in scenarios A and B, while the central bank retains all on-chain signing authority"

**Source documents**: [`docs/admission-onboarding-profile-change-proposal.md`](../../docs/admission-onboarding-profile-change-proposal.md) · [`docs/rework-governance-inter-spoke-scope-with-external-orchestrators.md`](../../docs/rework-governance-inter-spoke-scope-with-external-orchestrators.md)

> **Cross-scenario note.** This feature applies to **both** Scenario A (Enhanced Correspondent
> Banking) and Scenario B (International Hub). Per the Scenario-Scoped Independence principle,
> the implementation is delivered as **two independent, scenario-isolated PRs** (one per scenario,
> no shared code). This single specification defines the common behaviour and the per-scenario
> differences so both deliveries start from one validated design.

## Clarifications

### Session 2026-08-03

- Q: For Scenario A, how should ApproveKYC satisfy "central bank signs on-chain, Admission never signs" (it currently registers+verifies inline)? → A: **A2 — full split now.** Move the on-chain register+verify out of the Admission-authorized off-chain approval into a separate central-bank-signed completion step, mirroring Scenario B. Both scenarios converge on the same two-step flow.
- Q: Should every central-bank entity declaration be required to include an Admission operator login, or optional during migration? → A: **Required immediately.** A central-bank declaration without an Admission operator fails validation; all committed sample declarations are updated in this change.
- Q: Confirm delivery structure (you asked for "a spec"/"a branch"). → A: **One cross-scenario spec (042), two scenario-isolated PRs** (Scenario A and Scenario B), no shared code.
- Q: Beyond onboarding, should any other governance capability move to Admission? → A: **Read-only participant/pending views must be accessible to BOTH the Admission and Governance profiles.** Only the *mutating* onboarding actions (approve KYC, register/drive onboarding pipeline) move to Admission-exclusive authorization; governance-only operators keep read access but lose the mutating onboarding actions.
  > **Refined by the 2026-08-04 session below.** "Drive onboarding pipeline" was too broad: certificate
  > issuance (FR-002a) and central-bank operator provisioning (FR-003b) stay with governance, and the read
  > surface includes the registry view, not just the participant/pending list (FR-003a).

### Session 2026-08-04 — decisions from the second codebase-verification pass

These resolve ambiguities that only became visible once the full onboarding surface was enumerated. Details
and file anchors in [research.md](./research.md) R8–R14.

- Q: Does "drive the CSR pipeline" put certificate issuance under Admission? → A: **No.** Signing a CSR
  produces a signature from the central bank's CA key, which is the same class of act as an on-chain
  signature. Certificate issuance stays with governance; Admission owns the onboarding *record*. Captured as
  FR-002a.
- Q: Are the read-only views limited to the participant list? → A: **No** — the registry view is part of the
  same read surface and must admit both profiles, otherwise an Admission-only operator sees an empty
  onboarding page. Captured as FR-003a.
- Q: Should central-bank operator provisioning (creating treasury/supervisor/NOC/bank operator accounts) move
  to Admission? → A: **No.** It is not commercial-bank admission, and it triggers a central-bank-signed
  on-chain registration inline. Stays with governance. Captured as FR-003b.
- Q: Is splitting the KYC-approval path enough to guarantee "Admission never signs"? → A: **No** — Scenario A
  has a second inline on-chain register+verify path reached from operator provisioning. The invariant must be
  stated over every such path. Captured as FR-015a.
- Q: Is route-level authorization sufficient for the portal? → A: **No.** The portal's authorization check
  runs at authentication, so an Admission-only operator would be unable to sign in. Captured as FR-016.
- Q: Does the Scenario A split change any observable behaviour beyond authorization? → A: **Yes.** Approval
  currently guarantees the participant is registered and verified on-chain before it is marked approved;
  after the split it does not. Captured as FR-017.
- Q: Reuse the dormant `ROLE_GOVERNANCE_OFFICER` role instead of adding a new one? → A: **No** — it is
  already published as a participant-assignable role; reusing it would conflate two concepts. Recorded in
  Assumptions, with a separate cleanup ticket for the dormant role.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Admission operator manages commercial-bank onboarding (Priority: P1)

An onboarding operator holding the **Admission** profile signs in to the governance portal and
performs the day-to-day onboarding of commercial banks: viewing the list of participants, the
pending-onboarding queue and the registry, and approving (or holding) a commercial bank's KYC/admission
decision. This is the record-keeping and process-ownership role — it decides *whether a bank is admitted
to the process* and advances the off-chain onboarding record (credential-request intake, CSR submission
record, Proof-of-Possession tracking). It never signs the authoritative on-chain admission, and it never
issues the participant certificate: both are central-bank key operations (FR-002a).

**Why this priority**: This is the core of the change — moving the onboarding-management
responsibility off the central-bank governance profile and onto a dedicated Admission profile. Without
it, the feature delivers nothing. It establishes the new record-keeper role that a later phase can
assign to an external orchestrator entity (e.g. SEMLA/FLAR).

**Independent Test**: Provision an operator with only the Admission profile; confirm they can **sign in to
the portal**, list participants, see the pending queue and the registry, and drive a commercial bank from
"pending" to "KYC approved", and that the participant reaches the approved state — with no other profile
granted.

**Acceptance Scenarios**:

1. **Given** an operator holding only the Admission profile, **When** they sign in and open the participant
   list, pending-onboarding queue and registry view, **Then** authentication succeeds and all three are
   displayed.
2. **Given** a commercial bank awaiting admission and an operator holding the Admission profile,
   **When** the operator approves the bank's KYC/admission, **Then** the bank advances to the approved
   onboarding state and the off-chain onboarding record progresses (credential-request intake, CSR
   submission record, Proof-of-Possession tracking). Issuing the certificate itself remains a governance
   action.
3. **Given** the approval above, **When** the audit trail is inspected, **Then** the off-chain
   admission action is attributed to the Admission operator.

**Note on read vs. mutate**: The read-only participant list, pending-onboarding queue and registry view are
visible to both the Admission and Governance profiles (see User Story 3). Only the *mutating* onboarding
actions — approve KYC and register the participant record — are exclusive to the Admission profile.
Certificate issuance and central-bank operator provisioning remain governance actions (FR-002a, FR-003b).

---

### User Story 2 - Central bank retains all on-chain signing authority (Priority: P1)

Every authoritative on-chain admission (the identity-registry registration and verification that lets a
bank transact) continues to be signed by the **central bank**. The Admission profile holds no signing
key and cannot, by itself, cause an on-chain identity registration. The central bank remains the sole
signer even though it no longer *owns* the onboarding workflow.

**Why this priority**: This is the load-bearing safety and governance invariant of the whole change. It
is what makes the responsibility move acceptable: the onboarding operator becomes a record-keeper, not a
decision-maker, exactly as the rework document requires ("central banks retain signing authority
on-chain"). If this invariant is not guaranteed, the change would hand on-chain authority to a
non–central-bank actor.

**Independent Test**: Complete an onboarding driven by the Admission operator and inspect the on-chain
identity registration and verification: the signer must be the central bank, never the Admission
operator. Assert that no Admission-signed transaction reaches the identity registry.

**Acceptance Scenarios**:

1. **Given** an Admission-approved onboarding, **When** the on-chain registration and verification are
   recorded, **Then** the signer of both is the central bank.
2. **Given** the Admission profile, **When** its granted capabilities are enumerated, **Then** it holds
   no on-chain signing role and no signing key.
3. **Given** a completed onboarding, **When** the audit trail is inspected, **Then** the off-chain
   admission is attributed to the Admission operator and the on-chain signature is attributed to the
   central-bank signer — the two actors are distinguishable.

---

### User Story 3 - Governance operator keeps value/freeze authority, key operations and read access, but not onboarding mutations (Priority: P2)

A central-bank governance operator retains reserve/value, account freeze/unfreeze, circuit-breaker and
parameter authority; retains every operation that uses a central-bank key — **certificate issuance**
(FR-002a) and on-chain signing (FR-005); retains **central-bank operator provisioning** (FR-003b); and
retains **read-only** access to the participant list, pending-onboarding queue and registry view. However, a
governance-only operator can **no longer** approve KYC or register the participant record. Those two actions
now require the Admission profile.

**Why this priority**: This is the negative half of the responsibility move and the primary regression
risk. It proves the authorization boundary actually moved rather than merely being duplicated, and it
protects against silently leaving onboarding mutations open to the old profile — while preserving the
governance operator's visibility into participants.

**Independent Test**: Provision an operator with only the governance profile; confirm the read-only
participant/pending/registry views still load, that approving KYC and registering the participant record are
refused, and that freeze/unfreeze, circuit-breaker, parameter, certificate-issuance and operator-provisioning
actions all still succeed.

**Acceptance Scenarios**:

1. **Given** an operator holding only the governance profile, **When** they open the participant list,
   pending-onboarding queue or registry view, **Then** the read-only views load successfully.
2. **Given** the same operator, **When** they attempt to approve KYC or register the participant record,
   **Then** the action is refused (not authorized).
3. **Given** the same operator, **When** they perform a freeze/unfreeze, circuit-breaker or parameter
   action, issue a participant certificate, or provision a central-bank operator account, **Then** the
   action succeeds.
4. **Given** the governance portal, **When** a governance-only operator opens the onboarding surface,
   **Then** they can see the participant/pending/registry views but the approve/register controls are not
   available to them.

---

### User Story 4 - Local/pilot bootstrap keeps working during migration (Priority: P3)

In local development and pilot mode, a single bootstrap operator can hold **both** the governance and
Admission profiles, so existing end-to-end flows, tryout walkthroughs, and single-operator dev stacks
continue to work without a second human. Separation of the two profiles becomes meaningful only when
they are assigned to different identities in a production/staging deployment.

**Why this priority**: Migration safety. Without a dual-grant path, introducing the new profile would
break every existing single-operator local/E2E flow and every current governance-only operator's access
to onboarding.

**Independent Test**: Bring up a local/pilot stack with one bootstrap operator granted both profiles;
run the existing onboarding end-to-end flow and confirm it still completes.

**Acceptance Scenarios**:

1. **Given** a local/pilot bootstrap operator granted both profiles, **When** the existing onboarding
   end-to-end / tryout flow runs, **Then** it completes successfully.
2. **Given** a production/staging deployment, **When** the profiles are assigned to different
   identities, **Then** the Admission operator manages onboarding and the governance/central-bank signer
   signs on-chain, with no single identity required to hold both.

### Edge Cases

- **Governance-only operator after cutover**: an operator who held only the governance profile before
  this change loses access to onboarding routes until the Admission profile is provisioned. The new
  profile must be provisioned before the authorization boundary is flipped.
- **Admission operator attempts to sign on-chain**: the Admission profile holds no key; any attempt to
  produce an authoritative on-chain admission must be impossible for it, not merely discouraged.
- **Central bank declaration without an Admission operator**: an entity declaration that does not include
  an Admission operator MUST fail validation (the Admission operator is required for every central bank —
  see Assumptions). All committed sample declarations are updated in this change.
- **Scenario A coupled approval**: in Scenario A the off-chain approval and the on-chain signature are
  currently a single action. The approach is settled (full split — FR-015), and it must cover **every** such
  coupled path, not only the KYC-approval one (FR-015a): central-bank operator provisioning also performs an
  inline on-chain registration today, which is why it stays governance-authorized (FR-003b).
- **Scenario difference in wiring**: the two scenarios provision the operator login through different
  mechanisms; the observable behaviour (Admission manages onboarding, central bank signs) must be
  identical in both.
- **Admission-only operator signing in**: the portal must authenticate an operator who holds the Admission
  profile and nothing else, and present the onboarding surface without the governance-only pages
  (accounts, circuit breaker, parameters) appearing as available.
- **Approved but not yet on-chain**: after the Scenario A separation, a participant may sit in the approved
  off-chain state while not yet registered/verified on-chain. Attempts to transact in that window must fail
  at the existing on-chain participation check, not be silently permitted.
- **Non-onboarding entities**: an entity declaration that hosts no commercial-bank onboarding (the
  interoperability hub, an operations entity, a commercial bank itself) MUST NOT be required to declare an
  Admission operator; requiring it would break provisioning of those entities.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST define a new, first-class **Admission** authorization profile, distinct
  from the central-bank governance profile, available in both Scenario A and Scenario B.
- **FR-002**: The Admission profile MUST authorize the mutating commercial-bank onboarding actions:
  approving the KYC/admission decision and registering the participant record, and it MUST own the
  record-keeping steps of the onboarding pipeline (credential-request intake, Proof-of-Possession tracking,
  progression of the off-chain onboarding state).
- **FR-002a**: Issuing the participant certificate — the act that signs a CSR with the **central bank's
  certificate authority key** — MUST remain with the governance profile. The Admission profile drives the
  pipeline up to and including the CSR submission record, but MUST NOT authorize an operation that produces
  a signature from a central-bank key, whether on-chain or PKI. (This is the same invariant as FR-005/FR-006,
  extended to the CA key.)
- **FR-003**: The mutating onboarding actions (approve KYC, register participant record) MUST NO LONGER
  be authorized by the governance profile; a governance-only operator MUST be refused these actions.
- **FR-003a**: The read-only onboarding views — the participant list, the pending-onboarding queue, and the
  registry view — MUST remain accessible to BOTH the Admission and the Governance profiles (an operator
  holding either profile can view them).
- **FR-003b**: Central-bank operator provisioning (creating treasury, supervisor, NOC and commercial-bank
  operator accounts) is NOT commercial-bank admission and MUST remain with the governance profile.
- **FR-004**: The governance profile MUST retain all authority it holds today other than the two mutating
  onboarding actions named in FR-003 — reserve/value operations, account freeze/unfreeze, circuit-breaker
  control, parameters, audit and user administration, certificate issuance (FR-002a), central-bank operator
  provisioning (FR-003b), and the read-only onboarding views (FR-003a) — unchanged. This list is
  non-exhaustive by intent: anything not explicitly moved by FR-002/FR-003 stays with governance.
- **FR-005**: All authoritative on-chain admission operations (identity-registry registration and
  verification) MUST continue to be signed by the central bank. The on-chain roles and signing key MUST
  be unchanged by this feature.
- **FR-006**: The Admission profile MUST hold no on-chain signing role and no signing key. It MUST NOT be
  able to produce an authoritative on-chain admission by itself; it may only *trigger* the operation,
  which the central-bank signer executes.
- **FR-007**: The system MUST record an audit trail that attributes the off-chain admission action to the
  Admission operator and the on-chain signature to the central-bank signer, such that approver and signer
  are distinguishable.
- **FR-008**: The governance portal MUST present the read-only participant, pending-onboarding and registry
  views to operators holding either the Admission or the Governance profile, while exposing the mutating
  onboarding controls (approve KYC, register the participant record) only to operators holding the Admission
  profile. Governance-only controls (certificate issuance, accounts, circuit breaker, parameters) MUST NOT be
  presented to an Admission-only operator.
- **FR-009**: The Admission profile MUST be assignable as an independent identity — either to a
  central-bank-internal onboarding operator or, in a later phase, to an external orchestrator/record-
  keeper entity — without requiring it to also hold the governance profile.
- **FR-010**: The system MUST support a local/pilot mode in which a single bootstrap operator holds both
  the governance and Admission profiles, so existing single-operator onboarding flows continue to work.
- **FR-011**: The Admission operator login MUST be declarable through the existing entity-declaration
  (manifest) mechanism used for other operator logins, following the same per-role pattern.
- **FR-012**: The feature MUST NOT weaken or bypass the compliance gate: onboarding MUST still pass
  through compliance and identity-registry checks; only the profile that authorizes the operational step
  changes.
- **FR-013**: The feature MUST require no smart-contract/on-chain redeployment; the on-chain identity
  registry and its roles are untouched.
- **FR-014**: The observable onboarding behaviour (Admission manages onboarding; central bank signs
  on-chain) MUST be identical across Scenario A and Scenario B, even where the underlying wiring differs.
- **FR-015**: In Scenario A, the off-chain admission approval MUST be separated from the on-chain
  register+verify so that the Admission-authorized approval no longer performs the on-chain call inline;
  the on-chain register+verify MUST execute in a distinct central-bank-signed completion step (converging
  on the same two-step flow Scenario B already uses).
- **FR-015a**: The separation in FR-015 MUST cover **every** path that performs an inline on-chain
  register+verify, not only the KYC-approval path. No route authorized by the Admission profile may trigger
  an on-chain identity registration within the same request, in either scenario.
- **FR-016**: The onboarding surface MUST be reachable by an operator holding **only** the Admission
  profile — including authentication into the governance portal. A profile that authorizes the onboarding
  actions but cannot sign in does not satisfy FR-002.
- **FR-017**: After the FR-015 separation, the off-chain approved state MUST NOT be interpreted as
  authorisation to transact. Only the central-bank-signed on-chain verification confers that. Any existing
  behaviour, test or operational procedure that treats approval as sufficient MUST be updated accordingly.

### Key Entities *(include if feature involves data)*

- **Admission profile**: A new off-chain authorization identity that owns the commercial-bank onboarding
  *record* — reading the participant/pending/registry views, approving KYC, and registering and advancing the
  off-chain onboarding record. Holds no on-chain role, no signing key and no certificate authority. Assignable
  to a central-bank-internal operator now, or an external orchestrator/record-keeper later.
- **Governance profile (central bank)**: The existing central-bank profile. After this change it retains
  reserve/value, freeze/unfreeze, circuit-breaker and parameter authority, certificate issuance,
  central-bank operator provisioning, and read access to the onboarding views; it remains the sole holder of
  every central-bank key operation (on-chain and PKI). What it loses is ownership of the onboarding
  *decision*: approving KYC and registering the participant record.
- **Commercial-bank onboarding record**: The participant record and its lifecycle state (pending →
  admitted/KYC-approved → on-chain registered/verified). The Admission profile advances the off-chain
  states; the central-bank signer performs the on-chain transition.
- **On-chain admission (identity registration + verification)**: The authoritative registry entries that
  permit a bank to transact. Always central-bank-signed; unchanged by this feature.
- **Operator login declaration**: The per-role entity-declaration entry that provisions an operator
  login, extended to include the Admission operator.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator holding only the Admission profile can complete 100% of onboarding-record actions
  (sign in, list participants, view pending queue and registry, approve KYC, register and advance the
  off-chain onboarding record) in both scenarios. Certificate issuance is excluded by FR-002a and is not part
  of this measure.
- **SC-002**: An operator holding only the governance profile is refused 100% of the mutating onboarding
  actions (approve KYC, register participant record), while retaining 100% success on the read-only
  participant/pending/registry views and on freeze/unfreeze, circuit-breaker, parameter, certificate-issuance
  and operator-provisioning actions.
- **SC-002a**: An operator holding only the Admission profile succeeds on 100% of the onboarding read views
  and mutating actions — including authenticating into the portal — with no governance grant.
- **SC-003**: In 100% of onboarding completions, the on-chain registration and verification are signed
  by the central bank, and 0% are signed by the Admission operator.
- **SC-004**: The audit trail distinguishes the off-chain approver (Admission) from the on-chain signer
  (central bank) in 100% of completed onboardings.
- **SC-005**: All existing onboarding end-to-end and tryout flows in both scenarios pass with a
  bootstrap operator holding both profiles, with no regression to onboarding completion time or success
  rate.
- **SC-006**: The change introduces zero smart-contract redeployments and zero new runtime dependencies.
- **SC-007**: The behaviour observed by an onboarding operator is identical across Scenario A and
  Scenario B (same actions available, same refusals, same signer). Because the two scenarios ship as
  independent PRs, this **cannot** be verified by the per-scenario suites alone — it requires an explicit
  cross-scenario comparison once both have landed, walking the same operator journey in each portal.
  Documented differences in wiring (which route serves a view, which provisioning mechanism is used, the
  manifest role spelling) do not violate this criterion; differences in what the operator can *do* would.

## Assumptions

- **Profile home is central-bank-internal for now.** This feature creates the Admission *profile* and
  moves the onboarding responsibility to it. Assigning the profile to an external orchestrator/record-
  keeper entity (SEMLA/FLAR) and standing up that entity's infrastructure is a later phase and out of
  scope here (see Out of Scope).
- **Naming.** The profile is named **Admission** to avoid clashing with the platform's existing
  "orchestrator" components (payment-orchestrator, swap orchestrator, deployment orchestrator). The
  rework document's external "orchestrator / record-keeper" role is realised here as this Admission
  profile.
- **Governance keeps everything except the onboarding decision.** Exactly two actions become
  Admission-exclusive: approving KYC and registering the participant record. The read-only onboarding views
  are **shared**, not moved (FR-003a). Certificate issuance (FR-002a) and central-bank operator provisioning
  (FR-003b) stay with governance. All other governance capabilities are untouched, and read-only supervisor
  summaries are unchanged.
- **Local/pilot dual-grant is acceptable.** Granting the bootstrap operator both profiles in local/pilot
  mode mirrors how the on-chain registry already grants all roles to a single admin in dev, and is the
  chosen migration path.
- **Manifest declaration is required for every central bank.** A central-bank entity declaration without
  an Admission operator fails validation. All committed sample declarations are updated in this change to
  include the Admission operator.
- **Scenario A converges on Scenario B's two-step flow.** Scenario B already separates the off-chain
  approval from the on-chain signature; Scenario A currently couples them and will be **fully split**
  (option A2): the on-chain register+verify moves out of the Admission-authorized approval into a
  distinct central-bank-signed completion step. Both scenarios then share the same two-actor,
  approve-then-sign flow.
- **Delivery is two scenario-isolated PRs.** One validated design here; two independent implementations,
  no shared code, per the Scenario-Scoped Independence principle.
- **Existing compliance/identity gate is reused.** No new gate is added and none is removed; the
  authorization boundary moves from one profile to another.
- **The profile is per-spoke, and that is deliberate for this phase.** The Admission operator login is
  declared in each central bank's own entity declaration and created in that central bank's own identity
  realm. An external operator acting for several jurisdictions would therefore hold one credential per
  spoke. This satisfies the scope of this feature but does **not** yet deliver the rework document's
  "onboarding operable by the external orchestrator across spokes" — that needs the deferred identity-
  federation phase (see Out of Scope). SC-001 must not be read as closing that requirement.
- **The Admission operator is provisioned only through the entity declaration.** It is not creatable through
  the participant/operator registration API, so it is deliberately absent from that API's assignable-role
  allowlist and published role enumerations.
- **A new profile is introduced rather than reusing the dormant governance-officer role.** Both scenarios
  already declare a password-only `ROLE_GOVERNANCE_OFFICER` that no authorization gate references. It is not
  reused because it is already published as a role assignable to *onboarded participants*, so overloading it
  would conflate two distinct concepts. Wiring or removing that dormant role is tracked separately.
- **Certificate issuance stays with governance.** See FR-002a. The Admission profile owns the onboarding
  record, not any central-bank key operation.

## Out of Scope

- External orchestrator (SEMLA/FLAR) infrastructure — shared cross-spoke wallet, RPC access to
  central-bank nodes — and assigning the Admission profile to such an external entity. This feature only
  creates the profile and moves the responsibility.
- Inter-spoke governance (how central banks govern across spokes) — a separate phase per the rework
  document.
- Splitting the on-chain verifier signer from the registrar signer (a dedicated distinct key) — a
  pre-existing residual, independent of this change.
- Any smart-contract change or redeployment, and any unrelated onboarding-UI cleanup.
