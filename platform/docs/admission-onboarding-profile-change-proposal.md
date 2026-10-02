# Change Proposal — Introduce an `Admission` Profile for Commercial-Bank Onboarding

**Scenarios:** A (Enhanced Correspondent Banking) and B (International Hub)
**Related docs:** [`rework-governance-inter-spoke-scope-with-external-orchestrators.md`](./rework-governance-inter-spoke-scope-with-external-orchestrators.md) · [`scenario-a/docs/runbooks/identity-registry-role-separation.md`](../scenario-a/docs/runbooks/identity-registry-role-separation.md) · [`scenario-b/docs/runbooks/identity-registry-role-separation.md`](../scenario-b/docs/runbooks/identity-registry-role-separation.md)
**Date:** 2026-08-03
**Status:** ⚠️ **SUPERSEDED by [`specs/042-admission-onboarding-profile/`](../specs/042-admission-onboarding-profile/spec.md)** (2026-08-04). Retained for provenance.
**Owner:** TBD

> ## Superseded — read the 042 spec instead
>
> This proposal was validated and turned into spec **042-admission-onboarding-profile**. The spec is the
> authoritative design; where the two disagree, the spec wins. Do not implement from this document.
>
> **Open questions §12 are all resolved** in the spec's Clarifications: Q1 central-bank-internal for now ·
> Q2 **A2, not A1** (full split; §7.2's A1 recommendation is overridden) · Q3 dual-grant accepted ·
> Q4 read views accessible to **both** profiles (so §5 and §6.3 below are wrong: listing is *not*
> Admission-exclusive) · Q5 name `Admission` confirmed · Q6 manifest entry **required immediately**, not
> optional.
>
> **Three factual errors** found by the 2026-08-04 verification pass, corrected in the spec:
>
> 1. **§6.5 / §7.1 — "Scenario B's toolkit has no Keycloak rendering or role→realm mapping yet" is false.**
>    Scenario B does provision Keycloak users from `spec.adminUsers`:
>    `scenario-b/toolkit/engine/orchestrator/step_found_spoke.go:354-358` calls `appendKeycloakUsers`
>    (`:379-396`), which creates each mapped realm role idempotently, creates the user, sets its password
>    and grants the roles. The mapping table is `orchestrator/admin_users.go:19` `realmRolesForAdminRole`,
>    and because its `default` branch returns the role verbatim, adding the `ADMISSION` case is
>    **mandatory** — otherwise the JWT claim lands as a bare `ADMISSION` and every guard fails.
> 2. **§7.2 / §6.5 — "Regenerate the committed sample realm-import JSONs" is not applicable.** No rendered
>    `keycloak-import/*.json` exists under either scenario's `samples/`. Realm roles come from
>    `deploy/local/keycloak/init.sh` on the compose path and from the toolkit on the `apply` path.
> 3. **Runbook link was broken.** `docs/runbooks/` does not exist at the repository root; the role-separation
>    runbook is duplicated per scenario. Fixed in the header above.
>
> **Scope items this proposal did not surface**, added in the spec (see its `research.md` R8–R14): Fiber
> group guards are prefix-scoped so route re-gating requires restructuring the groups, not swapping guards;
> the role constants live at a third site (`api-gateway/internal/domain/auth.go`); three routes were
> unclassified (`/governance/registry/csr`, `/governance/registry`, `/compliance/participants/provision`);
> the portal's authorization check runs at **login**, so an Admission-only operator could not sign in;
> Scenario A has a **second** inline on-chain register+verify path; and the A2 split removes a documented
> invariant (approval no longer implies on-chain verification).

> **Naming note.** This profile is called **`Admission`** rather than "orchestrator" to avoid confusion
> with the existing orchestrator components in the platform (the payment-orchestrator service, the
> cross-currency swap orchestrator, and the toolkit deployment orchestrator). The rework document refers
> to this same onboarding responsibility as the external "orchestrator / record-keeper" role; here that
> role is realised as the **Admission** profile.

---

## 1. Objective

Move the responsibility for **managing commercial-bank onboarding** out of the central-bank
governance profile and into a new, dedicated **`Admission`** profile.

Concretely, the admission profile becomes responsible for:

- **Listing** participants and the pending-onboarding queue.
- **Approving** commercial-bank admission (the off-chain KYC/admission decision) and driving the
  onboarding lifecycle (credential request, CSR, Proof-of-Possession).

This realises the intent captured in the rework document: the onboarding operator is a distinct
**record-keeper** (the rework doc's external "orchestrator / record-keeper"), while the **central bank
retains all on-chain signing authority**. The admission profile manages the process; the central bank
remains the sole signer of the authoritative on-chain admission.

This is a **cross-scenario proposal**. Per the scenario-isolation rule (Constitution v1.0.4), the
implementation will be delivered as **two independent specs and PRs** — one for Scenario A and one for
Scenario B — with no shared code. This document defines the common model and the per-scenario deltas so
both specs start from a single, validated design.

---

## 2. Current state — who approves commercial banks today

In both scenarios the authority to onboard and approve commercial banks is held **entirely by the
central-bank governance profile**, at three layers:

### 2.1 Off-chain authorization (`ROLE_GOVERNANCE`)

The Keycloak/JWT role `ROLE_GOVERNANCE` (documented as "Banco Central") gates every onboarding-approval
and participant-listing route at the API gateway, and the governance frontend requires it to render.

| Layer | Scenario A | Scenario B |
|---|---|---|
| List participants | `router.go:99` `centralBankRoutes.Get("/participants")` (`RequireRole(RoleGovernance)`) | `router.go:78` `complianceGroup.Get("/participants", RequireRole("ROLE_GOVERNANCE"))` |
| Approve KYC | `router.go:100` `centralBankRoutes.Post("/approve-kyc")` and `router.go:116` `govGroup.Post("/approve-kyc")` | `router.go:79` `complianceGroup.Post("/approve-kyc", RequireRole("ROLE_GOVERNANCE"))` |
| Register participant | `router.go:120` `govGroup.Post("/participants")` | `router.go:89–93` `govGroup` gated by `RequireRole("ROLE_GOVERNANCE")` |
| Governance frontend gate | `frontend/apps/governance/src/auth/authorization.ts` — `GOVERNANCE_REQUIRED_ROLE = "ROLE_GOVERNANCE"` | same file/constant |

Backend role constants are defined identically in `backend/services/auth/internal/domain/roles.go` and
`backend/services/compliance/internal/domain/roles.go`. No admission role exists in either scenario.

### 2.2 On-chain authority (`IdentityRegistry`)

Both scenarios use the same two-step, separation-of-duties `IdentityRegistry`:

- `registerParticipant(...)` — `onlyRole(GOVERNANCE_ROLE)` — creates the participant in `Pending`.
- `verifyParticipant(...)` — `onlyRole(VERIFIER_ROLE)` — promotes `Pending → Verified`.
- `updateStatus(...)` — split gate: promotion **to `Verified`** requires `VERIFIER_ROLE`; all other
  transitions (freeze / suspend / reopen) require `GOVERNANCE_ROLE`.

The constructor grants `DEFAULT_ADMIN_ROLE`, `GOVERNANCE_ROLE` **and** `VERIFIER_ROLE` to the single
central-bank admin. In practice both on-chain roles are held by the one central-bank key
(`CB_PRIVATE_KEY`). See the per-scenario role-separation runbooks:
[Scenario A](../scenario-a/docs/runbooks/identity-registry-role-separation.md) ·
[Scenario B](../scenario-b/docs/runbooks/identity-registry-role-separation.md).

### 2.3 Where the on-chain call is triggered — the key A/B difference

This is the load-bearing difference for the change:

- **Scenario A — coupled.** `compliance/internal/grpc/server/server.go:333` `ApproveKYC` performs the
  on-chain `RegisterParticipant` (`server.go:369`) **and** `VerifyParticipant` (`server.go:372`) inline,
  then sets `KYC_APPROVED`. The off-chain admission decision and the on-chain signing are a single action.
- **Scenario B — already split.** `compliance` `ApproveKYC` moves the participant to `KYC_APPROVED` and
  issues the Proof-of-Possession nonce **without touching the chain**. The on-chain `register+verify`
  happens later in the auth service's `CompleteOnboarding` (signed with `CB_PRIVATE_KEY`).

---

## 3. Proposed change (summary)

Introduce a new profile, **`Admission`** (`ROLE_ADMISSION`), and move the onboarding-management
responsibility to it, while keeping every on-chain signature with the central bank.

- **New off-chain role `ROLE_ADMISSION`.** It gates participant listing, KYC approval, and the
  onboarding-pipeline endpoints. This is the "approve and list" responsibility moved off the governance
  profile.
- **The admission profile holds no signing key.** It *triggers* the on-chain admission through the
  compliance service, which continues to sign `registerParticipant` / `verifyParticipant` with the
  **central bank's** `CB_PRIVATE_KEY`. On-chain roles (`GOVERNANCE_ROLE`, `VERIFIER_ROLE`) are **unchanged**.
- **The `Admission` profile is a distinct authorization identity**, assignable either to a
  central-bank-internal onboarding operator or to an external orchestrator/record-keeper entity
  (SEMLA/FLAR, as named in the rework doc) in a later phase. This proposal only creates the profile and
  moves the responsibility; it does not stand up external SEMLA/FLAR infrastructure or inter-spoke
  governance (both remain out of scope — see §11).

The central-bank governance profile (`ROLE_GOVERNANCE`) retains reserve/value and freeze authority
(account freeze/unfreeze, circuit breaker, parameters) and remains the on-chain signer. It simply no
longer *owns* the commercial-bank onboarding workflow.

---

## 4. Constitution Check

Ref: `.specify/memory/constitution.md` (v1.0.4).

| Rule | Impact of this proposal | Compliant? |
|---|---|---|
| **Scenario isolation** | Model is shared in this doc only; implementation is two independent specs/PRs, no shared code. A/B deltas are called out in §7. | Yes |
| **Compliance gate at the API gateway** (IdentityRegistry + Compliance + Keycloak OIDC; never bypass; re-check at initiation) | The gate is **not weakened**. Onboarding still passes through Compliance + IdentityRegistry; the only change is *which* role authorizes the operational step. The on-chain signing path (`CB_PRIVATE_KEY`) is untouched and never bypassed. | Yes (moves the authz boundary, does not remove it) |
| **Atomicity** (two-step onboarding; a single governance action must never mint a transacting participant) | **Strengthened.** Splitting off-chain admission (admission profile) from on-chain signing (central bank) reinforces the register-vs-verify separation of duties the registry already models. | Yes (improves) |
| **Observability** (structured JSON logs; no silent swallowing; actor recorded) | Audit records must attribute the onboarding action to the admission actor and the on-chain signature to the central-bank signer. Additive logging only. | Yes |
| **Privacy** (`ZetoToken`/`NotoToken` for value; `tCeBM` reserve-only; no plaintext PII/amounts) | No token-path or on-chain data changes. | Yes |
| **Test-first, every layer** | Failing tests precede implementation at each layer (§10). | Yes |
| **QBFT / stack** | No new runtime dependency. Reuses Keycloak roles, existing Go middleware, existing contracts. | Yes |
| **PRs weakening compliance/security need project-lead approval** | This moves an authorization boundary; it must carry project-lead sign-off even though it does not weaken the gate. | Flagged for lead review |

**Deviations requiring Complexity Tracking:** none. The proposal removes a concentration of authority
(governance = onboarding + everything) rather than adding structural complexity.

---

## 5. Responsibility matrix (before → after)

> **Superseded row.** The first row is wrong: spec 042 keeps the read views on **either** profile
> (`ROLE_GOVERNANCE` **OR** `ROLE_ADMISSION`), and adds `GET /governance/registry` to that read surface.
> Only the mutating actions become Admission-exclusive. Certificate issuance (`/governance/registry/csr`)
> and central-bank operator provisioning (`/compliance/register`) also stay with governance.

| Capability | Before | After |
|---|---|---|
| List participants / pending queue | `ROLE_GOVERNANCE` | ~~**`ROLE_ADMISSION`**~~ → `ROLE_GOVERNANCE` **OR** `ROLE_ADMISSION` |
| Approve KYC (off-chain admission) | `ROLE_GOVERNANCE` | **`ROLE_ADMISSION`** |
| Register participant record / drive CSR + PoP pipeline | `ROLE_GOVERNANCE` | **`ROLE_ADMISSION`** |
| Sign on-chain `registerParticipant` (`GOVERNANCE_ROLE`) | Central bank (`CB_PRIVATE_KEY`) | Central bank (`CB_PRIVATE_KEY`) — **unchanged** |
| Sign on-chain `verifyParticipant` (`VERIFIER_ROLE`) | Central bank (`CB_PRIVATE_KEY`) | Central bank (`CB_PRIVATE_KEY`) — **unchanged** |
| Freeze / unfreeze accounts, circuit breaker, parameters | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` — **unchanged** |
| Supervisor read-only participant summary | `ROLE_SUPERVISOR` | `ROLE_SUPERVISOR` — **unchanged** |

The invariant: **every on-chain signature stays with the central bank**. The admission profile is a
record-keeper and process owner, never a signer.

---

## 6. Design

### 6.1 Actor model

```
   ADMISSION (ROLE_ADMISSION)                       CENTRAL BANK (ROLE_GOVERNANCE)
   - list pending / participants                    - holds GOVERNANCE_ROLE + VERIFIER_ROLE
   - approve-kyc (off-chain admission)              - signs registerParticipant + verifyParticipant
   - manage CSR / PoP onboarding pipeline           - retains ALL on-chain signing (CB_PRIVATE_KEY)
   - triggers on-chain tx  ───────────────────────▶ - freeze/unfreeze, circuit breaker, parameters
     (holds NO signing key)
```

The admission profile initiates the onboarding operation; the compliance service (running with the
central bank's signer) executes the on-chain admission. The admission profile never holds
`GOVERNANCE_ROLE`, `VERIFIER_ROLE`, or `CB_PRIVATE_KEY`.

### 6.2 New role definition

Add `ROLE_ADMISSION` as a first-class role in each scenario:

- **Backend:** `RoleAdmission = "ROLE_ADMISSION"` in `auth/internal/domain/roles.go` and
  `compliance/internal/domain/roles.go`. Password-only profile (no PKI/on-chain/KMS requirement — it
  signs nothing on-chain), so it is **not** added to `pkiRoles`, `onChainRoles`, or `kmsRoles`.
- **Keycloak (local dev):** add an `admission` realm role in the realm bootstrap
  (`deploy/local/keycloak/...`), mapped to `ROLE_ADMISSION` in JWT claims consistently with the
  existing `ROLE_`-prefix normalization.
- **Entity manifest:** the admission profile gets its login from the manifest — see §6.5.
- **Frontend:** the governance portal's onboarding views move behind an admission gate (see §7).

### 6.3 Authorization change (both scenarios)

> **Superseded.** Two corrections in spec 042: (1) the list route becomes **any-of**
> (`RequireRole(governance, admission)`), not Admission-exclusive; (2) "change the guard" is not achievable
> as written — these routes are gated by **group-level** middleware, which Fiber applies to the whole
> prefix, so a route-level Admission guard runs *in addition* and 403s an Admission-only caller. The routes
> must be re-registered outside their governance group. See
> [contracts/authorization-matrix.md](../specs/042-admission-onboarding-profile/contracts/authorization-matrix.md).

Change the `RequireRole(...)` guard on the **onboarding-management** routes from `ROLE_GOVERNANCE` to
`ROLE_ADMISSION`:

- `GET /api/v1/compliance/participants` (list)
- `POST /api/v1/compliance/approve-kyc` (approve)
- the participant-registration / onboarding-pipeline routes

Leave freeze/unfreeze, circuit breaker, parameters, and audit routes on `ROLE_GOVERNANCE`.

### 6.4 On-chain — no change

`IdentityRegistry` is untouched. `GOVERNANCE_ROLE` and `VERIFIER_ROLE` remain with the central-bank
admin; the compliance service continues to sign with `CB_PRIVATE_KEY`. This keeps the change **additive**
(consistent with the rework doc's "additive layer, no refactoring") and preserves the constitutional
"central banks retain on-chain signing authority" invariant.

### 6.5 Login provisioning via the entity manifest

The admission profile is a human login, and human logins in this platform are declared in the **entity
manifest** under `spec.adminUsers[]` (`role` / `username` / `password`), one entry per role the entity
hosts. Passwords there are plaintext and explicitly **local-profile only** — the schema and code
comments (`toolkit/engine/manifest/types.go`, `provisioning/schema/v1/participant-deployment.schema.yaml`)
require sourcing them from a secret store for staging/production. The central bank's `ROLE_GOVERNANCE`
login already comes from this list (e.g. `scenario-a/samples/brazil/central-bank-brazil.yaml`).

**The admission profile therefore gets a new `adminUsers[]` entry on the central-bank manifest**,
following the existing pattern:

```yaml
# Scenario A (roles carry the ROLE_ prefix)
adminUsers:
  - role: ROLE_ADMISSION
    username: admin@<entity>.admission.gov
    password: <entity>-admission-local   # local profile only; secret store in staging/prod

# Scenario B (manifest uses bare role names, no ROLE_ prefix)
adminUsers:
  - role: ADMISSION
    username: admin@<entity>.admission.gov
    password: <entity>-admission-local
```

Wiring differs by scenario, because the manifest→Keycloak toolkit is at different maturity:

- **Scenario A** renders `adminUsers[]` into a Keycloak realm-import JSON. Adding the admission profile
  requires: (1) add `ROLE_ADMISSION` to `requiredAdminRolesByEntity["central-bank"]` in
  `toolkit/engine/manifest/validate.go` (so a central bank must declare an admission login); and
  (2) route that admin user into the central-bank realm in `toolkit/engine/orchestrator/keycloak.go`
  (`centralBankRealmPlans` / `adminUsersForRealmRoles`), so `renderRealmJSON` emits it as a
  non-temporary password user. The committed sample manifests and their rendered realm JSONs under
  `samples/` must be regenerated to include the new user.
- ~~**Scenario B** stores `adminUsers[]` in the manifest but the toolkit currently only parses/validates
  them (`toolkit/engine/manifest/validate.go` `validateAdminUsers`) — there is no Keycloak rendering or
  role→realm mapping yet. So for Scenario B, adding the admission profile to the manifest is a data
  addition; the actual Keycloak user is provisioned through the existing local realm path
  (`deploy/local/keycloak/`). The manifest→Keycloak rendering for Scenario B is a separate, pre-existing
  toolkit gap, not created by this change.~~
  **CORRECTED (2026-08-04):** Scenario B **does** render and provision Keycloak users from
  `spec.adminUsers` — `orchestrator/step_found_spoke.go:354-358` → `appendKeycloakUsers` (`:379-396`),
  mapping through `orchestrator/admin_users.go:19` `realmRolesForAdminRole`. Adding the `ADMISSION` case to
  that mapping is **mandatory** (its `default` branch returns the role verbatim, yielding a bare `ADMISSION`
  claim). Separately, B's `validateAdminUsers` (`manifest/validate.go:222`) takes only the user slice with no
  entity context, so making the entry *required for central banks* needs a signature change keyed on
  `spec.topology.role`.

Decision to confirm (see §12): whether the admission profile is a **required** manifest admin user for
every central bank, or an **optional** one during migration (so existing manifests without an admission
user still validate). Recommendation: optional first (back-compat), promoted to required once every
entity declares one.

---

## 7. Per-scenario deltas

### 7.1 Scenario B (cleaner — off-chain approve already decoupled)

`approve-kyc` is already off-chain, so the change is almost purely an **authorization move**:

- Re-gate `router.go:78` (`/participants`) and `router.go:79` (`/approve-kyc`) from `ROLE_GOVERNANCE`
  to `ROLE_ADMISSION`.
- Re-gate the onboarding-pipeline routes in the `govGroup` (`router.go:89–93`) that pertain to
  participant registration/onboarding.
- The on-chain `register+verify` in `CompleteOnboarding` (auth service, `CB_PRIVATE_KEY`) is unchanged.
- Frontend: move `RegistryPage` onboarding actions behind `ROLE_ADMISSION`.
- Manifest: add the `ADMISSION` `adminUsers[]` entry to the central-bank manifests. Keycloak
  provisioning uses the existing `deploy/local/keycloak/` path (the Scenario B manifest→Keycloak toolkit
  is a separate, pre-existing gap — see §6.5).

### 7.2 Scenario A (requires decoupling off-chain approve from on-chain signing)

Scenario A's `ApproveKYC` (`compliance/.../server.go:333`) performs the on-chain `RegisterParticipant`
(`server.go:369`) and `VerifyParticipant` (`server.go:372`) **inline**. To keep the invariant while
moving the "approve" responsibility to the admission profile, the off-chain admission and the on-chain
signature must be separated so that:

- the **admission profile** authorizes the off-chain admission (`approve-kyc` → `KYC_APPROVED` + PoP), and
- the **central bank's signer** executes `register+verify` on-chain.

Two implementation options (to be decided in the Scenario A spec):

| Option | How | Trade-off |
|---|---|---|
| **A1 — keep coupled, run under CB signer** | Admission profile triggers `approve-kyc`; the compliance service still performs register+verify inline, but the signing key remains the CB's. Authz boundary moves; execution unchanged. | Smallest change; the on-chain call fires within the admission-authorized request but is CB-signed. Matches Scenario A's current shape. |
| **A2 — split like Scenario B** | Move register+verify out of `ApproveKYC` into a separate CB-signed completion step (mirrors Scenario B's `CompleteOnboarding`). | Larger change; converges both scenarios onto the same two-actor flow. Recommended for long-term consistency. |

Recommendation: **A1 for the first spec** (minimal, ships the responsibility move), with **A2 tracked as
a follow-up** to align both scenarios. The Scenario A spec must state which option it implements in its
Complexity Tracking.

Manifest: add the `ROLE_ADMISSION` `adminUsers[]` entry to the central-bank manifests, add it to
`requiredAdminRolesByEntity["central-bank"]`, and route it into the central-bank realm in `keycloak.go`
so `renderRealmJSON` emits the login (see §6.5). ~~Regenerate the committed sample realm-import JSONs.~~
**CORRECTED (2026-08-04): there are no committed realm-import JSONs** — no `keycloak-import/*.json` exists
under either scenario's `samples/`. Realm roles come from `deploy/local/keycloak/init.sh` on the compose path
and from the toolkit on the `apply` path; both must be covered.

> **Also superseded here:** §7.2's recommendation of **A1 for the first spec** was rejected — spec 042
> chooses **A2** (full split) for both scenarios. And A2 must cover a **second** inline on-chain path the
> proposal did not identify: auth `OnboardParticipant`
> (`scenario-a/backend/services/auth/internal/grpc/server/server.go:261`, register+verify at `:317,321`),
> reached from `POST /api/v1/compliance/register` (`router.go:104`).

---

## 8. Files touched (per scenario, indicative)

| Area | Scenario A | Scenario B |
|---|---|---|
| Role constant `RoleAdmission` | `backend/services/auth/internal/domain/roles.go`, `backend/services/compliance/internal/domain/roles.go` | same paths |
| Route re-gating | `backend/services/api-gateway/internal/http/router/router.go` (lines ~99–101, ~116–120) | `.../router/router.go` (lines ~78–79, ~89–93) |
| Approve/onboarding handler authz + audit actor | `.../http/handlers/governance.go`, `.../handlers/compliance.go` | same paths |
| On-chain coupling split (A2 only) | `backend/services/compliance/internal/grpc/server/server.go:333–372` | n/a (already split) |
| Keycloak realm role | `deploy/local/keycloak/...` | `deploy/local/keycloak/realms/scenario-b-realm.json` |
| Manifest login (`adminUsers[]`) | `samples/**/central-bank-*.yaml` (+ rendered `samples/**/keycloak-import/*.json`) | `samples/**/central-bank-*.yaml` |
| Manifest validation / realm mapping | `toolkit/engine/manifest/validate.go` (`requiredAdminRolesByEntity`), `toolkit/engine/orchestrator/keycloak.go` (`centralBankRealmPlans`, `renderRealmJSON`) | `toolkit/engine/manifest/validate.go` (`validateAdminUsers`) — realm rendering not yet wired |
| Frontend gate | `frontend/apps/governance/src/auth/authorization.ts`, `ProtectedRoute.tsx`, `RegistryPage.tsx` | same paths |
| Tests | `*_test.go`, `actor_authz_test.go`, manifest validation tests, frontend route tests | same |

No Solidity/contract changes. `contracts.sync-addresses` is not required (no redeploy).

---

## 9. Migration & backward compatibility

- **Local / single-operator dev and E2E.** These bootstrap a single central-bank operator and rely on
  onboarding working end-to-end. To avoid breaking them, grant the bootstrap operator **both**
  `ROLE_GOVERNANCE` and `ROLE_ADMISSION` by default in local/pilot mode — mirroring how the
  `IdentityRegistry` constructor grants all on-chain roles to one admin. Separation of the two profiles
  becomes meaningful only when they are assigned to different identities in production.
- **`EnsureGovernanceParticipant` bootstrap** (compliance startup) and the seed/tryout scripts continue
  to run under the central-bank signer; no change to `CB_PRIVATE_KEY` handling.
- **Existing tokens/sessions.** Operators currently holding only `ROLE_GOVERNANCE` will lose access to
  the onboarding routes unless also granted `ROLE_ADMISSION`. Provision the new role before flipping
  the guards (documented in the runbook update).

---

## 10. Test-first plan (per layer)

Write the failing tests before implementation, in both scenarios:

- **Authz (go test).** A caller with only `ROLE_GOVERNANCE` is now **rejected** (403) on
  `/compliance/approve-kyc` and `/compliance/participants`; a caller with `ROLE_ADMISSION` is
  **accepted**. Extend the existing `actor_authz_test.go` regression suite.
- **Approve flow (go test).** An admission-authorized `approve-kyc` still results in the participant
  reaching `KYC_APPROVED`; the on-chain signature is attributed to the **central-bank signer**, not the
  admission profile. For Scenario A option A2, assert `ApproveKYC` no longer calls the chain directly.
- **Invariant test.** No admission-signed transaction reaches `IdentityRegistry`; on-chain
  `register+verify` is always CB-signed.
- **Frontend.** Governance onboarding views require `ROLE_ADMISSION`; a governance-only profile is
  redirected.
- **E2E / tryout.** `make scenario-b.test`, `make scenario-b.tryout-*`, and the Scenario A equivalents
  pass with the bootstrap operator holding both roles; add a negative case for a governance-only actor.

---

## 11. Out of scope

- **External SEMLA/FLAR infrastructure** (shared wallet across spokes, RPC access to CB nodes) — this
  proposal only creates the `Admission` *profile* and moves the responsibility. Assigning the profile
  to an external entity, and the infra Samuel proposed, is a later phase.
- **Inter-spoke governance** (how central banks govern across spokes) — a separate phase per the rework
  doc.
- **On-chain verifier-signer split** (dedicated `VERIFIER_ROLE` key distinct from the registrar) — a
  pre-existing residual tracked in the role-separation runbook; independent of this change.
- **Removing the disabled "Issue Credential" UI** and other unrelated registry cleanup.

---

## 12. Open questions (for validation before spec)

> **All six are resolved** in [spec 042's Clarifications](../specs/042-admission-onboarding-profile/spec.md#clarifications).
> Summary: Q1 yes (central-bank-internal for now) · Q2 **A2, not A1** · Q3 yes · Q4 read views go to
> **both** profiles; certificate issuance and operator provisioning stay with governance · Q5 `Admission`
> confirmed · Q6 **required immediately** (recommendation of "optional first" overridden).

1. **Profile home.** Is `Admission` a central-bank-internal profile now, with the external-entity
   (SEMLA/FLAR) assignment deferred to the inter-spoke phase? (This proposal assumes yes.)
2. **Scenario A option.** A1 (minimal, keep coupled under CB signer) for the first spec, with A2
   (full split) as follow-up — or go straight to A2 for cross-scenario consistency?
3. **Local-dev dual-grant.** Confirm the bootstrap operator holding both `ROLE_GOVERNANCE` and
   `ROLE_ADMISSION` in local/pilot mode is acceptable (mirrors the on-chain constructor default).
4. **Governance residuals.** Should any capability beyond onboarding (e.g. registry read) also move to
   the admission profile, or does governance keep everything except list/approve/register?
5. **Naming.** The profile is named `Admission` (constant `ROLE_ADMISSION`) to avoid clashing with the
   platform's orchestrator components. Confirm this final name before it is baked into Keycloak realms
   and JWT claims.
6. **Manifest admin user — required or optional?** Should every central-bank manifest be *required* to
   declare an admission `adminUsers[]` entry (fails validation otherwise), or optional during
   migration? Recommendation: optional first for back-compat, promoted to required once all entities
   declare one (see §6.5).

---

## 13. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Flipping route guards locks out existing governance operators from onboarding | Provision `ROLE_ADMISSION` (dual-grant in local/pilot) **before** changing the guards; document in the runbook. |
| Scenario A coupled `ApproveKYC` accidentally lets the admission profile become the on-chain signer | Explicit invariant test (§10): on-chain `register+verify` must be CB-signed; admission profile holds no key. |
| Perceived weakening of the compliance gate | It is an authz *move*, not a removal; carry project-lead approval (§4) and keep the audit trail attributing signer vs. approver. |
| Divergence between the two scenarios during rollout | Single shared model here; A2 tracked to converge; both specs cross-reference this document. |
| Keycloak realm/JWT naming mismatch (realm uses bare `central_bank`) | Reuse the existing `ROLE_`-prefix normalization; add the realm role and mapper together and test claim resolution. |

---

## Appendix — role glossary (after change)

| Profile | Layer | Onboarding responsibility | On-chain signing |
|---|---|---|---|
| `ROLE_ADMISSION` (new) | off-chain | list, approve KYC, drive CSR/PoP pipeline | none |
| `ROLE_GOVERNANCE` (central bank) | off-chain + on-chain signer | freeze/unfreeze, circuit breaker, parameters | `GOVERNANCE_ROLE` + `VERIFIER_ROLE` via `CB_PRIVATE_KEY` |
| `ROLE_SUPERVISOR` | off-chain | read-only participant summary | none |
| `ROLE_TREASURY` | off-chain + on-chain | reserve operations (unchanged) | treasury operations |
| `ROLE_COMMERCIAL_BANK` | off-chain + on-chain | self-service onboarding (CSR/PoP), transacting | own key, after CB verify |
