# Phase 0 Research — Admission Profile for Commercial-Bank Onboarding

All decisions below are validated against the current codebase. File references are indicative anchors for
implementation; line numbers may drift.

**Verification passes**: initial 2026-08-03; **second pass 2026-08-04** (full re-verification against both
scenario trees). The second pass confirmed R1, R3, R4, R5, R6 and R7, **corrected R2**, and added
**R8–R14** — seven constraints the first pass missed. Corrections are marked inline.

## R1. Authorization middleware semantics (read-both vs mutate-only)

**Decision**: Reuse the existing `RequireRole(roles ...string)` middleware. It grants when the caller
holds **at least one** of the listed roles (any-of). Read-only routes use
`RequireRole(RoleGovernance, RoleAdmission)`; mutating onboarding routes use `RequireRole(RoleAdmission)`.

**Rationale**: `scenario-{a,b}/backend/services/api-gateway/internal/http/middleware/require_role.go:18-33`
loops over the supplied roles and returns on the first match. Both scenarios' copies are byte-identical.
No new middleware is needed to satisfy the "read-only views visible to both profiles" clarification.

**Alternatives considered**: A dedicated `RequireAnyRole` helper — rejected as redundant (variadic
`RequireRole` already is any-of). An all-of semantic — not applicable; we need either-role for reads.

**Confirmed 2026-08-04.** But see **R8**: any-of semantics at the *route* level do not help when the
governing guard sits on the *group*. R1 is necessary but not sufficient.

## R2. Scenario A `ApproveKYC` split (option A2)

**Decision**: Fully split Scenario A's onboarding, mirroring Scenario B. The Admission-authorized
`approve-kyc` sets the participant to `KYC_APPROVED` and issues the Proof-of-Possession nonce **without
touching the chain**; the on-chain `RegisterParticipant` + `VerifyParticipant` move to a separate
central-bank-signed completion step (mirroring Scenario B's auth-service `CompleteOnboarding`).

**Rationale**: Today `scenario-a/backend/services/compliance/internal/grpc/server/server.go` `ApproveKYC`
performs `RegisterParticipant` and `VerifyParticipant` inline. Moving these out makes the "central bank
signs on-chain, Admission never signs" invariant structural, and converges both scenarios on one
approve-then-sign flow (FR-005, FR-006, FR-015). Scenario B already has the target shape
(`scenario-b/.../compliance/.../server.go` `ApproveKYC` is chain-free; on-chain register+verify live in
`scenario-b/backend/services/auth/internal/grpc/server/onboarding.go` `CompleteOnboarding`).

**Alternatives considered**: **A1** (keep register+verify inline, only move authz, ensure CB signer) —
rejected in clarification: leaves the on-chain call inside the Admission-authorized request and preserves
A/B divergence.

**~~Open implementation choice~~ — RESOLVED by the 2026-08-04 verification pass.** The first pass left open
"whether Scenario A reuses/introduces an auth-service `CompleteOnboarding` equivalent, or hosts the
CB-signed completion inside compliance". No choice is needed: **Scenario A already has
`CompleteOnboarding`** at `scenario-a/backend/services/auth/internal/grpc/server/onboarding.go:172` — the
same file and line as Scenario B's — and it is currently chain-free. The A2 split is a port of Scenario B's
step-6 block (`scenario-b/.../auth/internal/grpc/server/onboarding.go:218-235`) into A's existing function. No new
endpoint, no new gRPC method. This makes T029 materially smaller than planned.

**Two corrections to the A2 scope** (tracked as tasks T029a and T046; call sites tabulated in
[contracts/authorization-matrix.md](./contracts/authorization-matrix.md) INV-2):

1. **A has a second inline on-chain path.** `ApproveKYC` is not the only one. Auth `OnboardParticipant`
   (`auth/internal/grpc/server/server.go:261`) also performs an inline CB-signed register+verify
   (`:317,321`) and is reached from `POST /api/v1/compliance/register` (`router.go:104` →
   `handlers/compliance.go:205`) — a route the original matrix moved to `ROLE_ADMISSION`. Splitting only
   `ApproveKYC` leaves the rejected A1 shape in place on that route. See task T029a.
2. **A2 removes an invariant the code deliberately documents.** `compliance/.../server.go:351-368` states
   that register+verify is blocking inside `ApproveKYC` so that "no bank is ever marked KYC_APPROVED while
   missing on-chain", precisely because `registerParticipant` alone leaves the participant `Pending`, which
   fails HTLC's `onlyVerified` check. After A2, `KYC_APPROVED` no longer implies on-chain `Verified`.
   Scenario B already operates this way (the bank-driven `CompleteOnboarding` closes the gap), so the
   choice stands — but it is a behavioural change in A's HTLC path, not a pure refactor. See task T046.

## R3. Per-scenario toolkit divergence (manifest → Keycloak)

**Decision**: Treat the toolkit as **two independent implementations**. Each PR edits only its scenario's
toolkit.

- **Scenario A** (`scenario-a/toolkit/`): manifest roles are `ROLE_`-prefixed. Add `ROLE_ADMISSION` to
  `requiredAdminRolesByEntity["central-bank"]` in `engine/manifest/validate.go`, and route it into the
  central-bank realm in `engine/orchestrator/keycloak.go` (`centralBankRealmPlans` /
  `adminUsersForRealmRoles`) so `renderRealmJSON` emits the login.
- **Scenario B** (`scenario-b/toolkit/`): manifest roles are **bare** (`GOVERNANCE`, not
  `ROLE_GOVERNANCE`). Add `ADMISSION` to the bare→realm mapping in
  `engine/orchestrator/admin_users.go:19` `realmRolesForAdminRole` (`ADMISSION → {central_bank,
  ROLE_ADMISSION}`). Scenario B has **no** `requiredAdminRolesByEntity` equivalent, so the "required"
  enforcement must be added to `validateAdminUsers` and keyed on the entity — see item 3 below for the
  precise mechanism and why it needs a signature change.

**Rationale**: Verification confirmed the functions named in the proposal exist only in Scenario A's
toolkit; Scenario B uses a different mechanism. A single edit would not work for both.

**Alternatives considered**: Extracting a shared toolkit — rejected (violates Scenario-Scoped
Independence; out of scope).

**Confirmed 2026-08-04, with two additions:**

1. **The mapping is mandatory, not cosmetic.** `realmRolesForAdminRole`'s `default` branch returns the role
   string verbatim. Without the `ADMISSION` case, a manifest entry yields a literal `ADMISSION` realm role
   and JWT claim — every `RequireRole("ROLE_ADMISSION")` guard then fails silently.
2. **The proposal's claim that Scenario B has no Keycloak rendering is false.** B *does* provision users:
   `engine/orchestrator/step_found_spoke.go:354-358` reads `spec.adminUsers` and calls
   `appendKeycloakUsers` (`:379-396`), which creates each mapped realm role idempotently (`:385`), creates
   the user, sets the password and grants the roles via `kcadm`. Consequence: on the toolkit path, T017
   alone provisions the role — no separate realm-role task is needed there. (The `deploy/local` path still
   needs `init.sh`; see R4.)
3. **B's required-role enforcement needs a signature change.** `engine/manifest/validate.go:222`
   `validateAdminUsers(users []AdminUser, r *Result)` receives only the slice and has no entity context.
   The keying value exists — `spec.topology.role` (`central-bank` in
   `samples/brazil/central-bank-brazil.yaml:10`, `hub` in `samples/hub/hub-cbweb3.yaml`) — so pass it
   through and require `ADMISSION` for `central-bank` only. `hub` is exempt (banks join spokes, not the
   hub) and `join`-mode bank manifests are unaffected. See task T018.

## R4. Keycloak realm-role provisioning (local/dev)

**Decision**: Add `ROLE_ADMISSION` to the `CENTRAL_BANK_ROLES` array in **both**
`scenario-a/deploy/local/keycloak/init.sh:332` and `scenario-b/deploy/local/keycloak/init.sh:370`.
`scenario-b/deploy/local/keycloak/realms/scenario-b-realm.json` needs **no** edit — see the confirmation
below.

**Rationale**: Verification found **no** rendered `keycloak-import/*.json` under `samples/`; realm roles
are created by `init.sh`. The JWT `ROLE_`-prefix normalization already in place carries the claim through.

**Confirmed 2026-08-04.** Exact anchors: `scenario-a/deploy/local/keycloak/init.sh:332`
(`CENTRAL_BANK_ROLES=(...)`) and `scenario-b/deploy/local/keycloak/init.sh:370` (same array, plus the bare
`central_bank hub_operator` Scenario B roles). **K2 resolves to "not required"**:
`scenario-b/deploy/local/keycloak/realms/scenario-b-realm.json` enumerates only the bare realm roles
(`central_bank`, `commercial_bank`, `hub_operator`) — the `ROLE_*` family is created exclusively by
`init.sh`, so the realm JSON needs no edit. Note also that **both** provisioning paths must be covered:
`deploy/local/keycloak/init.sh` (docker-compose stacks) and the toolkit (see R3, item 2).

## R5. Audit attribution (approver vs signer)

**Decision**: Additively record the off-chain admission action against the Admission operator (from JWT
subject/claims already available in the handler) and the on-chain signature against the central-bank
signer, reusing the existing `audit_log` structured-logging path. No schema change.

**Rationale**: FR-007 / Constitution VI. The handlers already have the actor from the token; the
compliance service already logs onboarding events. This is an attribution/logging addition, not a new
store.

## R6. Migration & local/pilot dual-grant

**Decision**: In local/pilot bootstrap, grant the single bootstrap operator **both** `ROLE_GOVERNANCE`
and `ROLE_ADMISSION` (mirroring how the `IdentityRegistry` constructor grants all on-chain roles to one
admin in dev). The role must be provisioned **before** route guards flip. Governance-only operators keep
read access but lose the mutating onboarding actions.

**Rationale**: FR-010, SC-005 — existing single-operator E2E/tryout flows must keep passing.
Separation is meaningful only when the two profiles are assigned to different identities in
staging/production.

## R7. No contract change

**Decision**: `IdentityRegistry` and all Solidity are untouched; `GOVERNANCE_ROLE` and `VERIFIER_ROLE`
stay with the central-bank admin key. No `contracts.deploy-*` / `contracts.sync-addresses` needed.

**Rationale**: FR-013; the change is entirely off-chain authorization + provisioning.

**Confirmed 2026-08-04** in both scenarios: `contracts/src/IdentityRegistry.sol` — constructor grants
`DEFAULT_ADMIN_ROLE` + `GOVERNANCE_ROLE` + `VERIFIER_ROLE` to the single admin (`:27-30` in A, `:43-46` in
B); `registerParticipant` is `onlyRole(GOVERNANCE_ROLE)` (`:41` / `:57`); `verifyParticipant` is
`onlyRole(VERIFIER_ROLE)` (`:71` / `:87`); `updateStatus` carries the split gate (`:124-128` / `:139-143`).
Nothing in this feature touches any of it.

---

# Second-pass findings (2026-08-04)

The following were not identified in the first pass. Four are hard blockers: **R8** (the re-gated routes
would 403 the Admission profile), **R10** (the route change would not compile), **R12** (an Admission-only
operator could not sign in), and the **R2 correction** (the feature would miss its own central invariant on
Scenario A's second on-chain path). **R9** and **R11** are correctness/precision defects — an unclassified
route silently defaults to governance-only, and the named test anchor does not exist. **R13** and **R14** are
scope statements with no code impact.

## R8. Fiber group middleware is prefix-scoped — relax the group guard, do not swap route guards

**Decision**: For each group that gates onboarding routes, change the **group** guard to the **union** of the
roles used by the routes inside it, then give **every** route inside an explicit per-route guard. URLs stay
unchanged.

**Rationale**: Both routers gate onboarding at the group level — Scenario A
`centralBankRoutes := complianceGroup.Group("", RequireRole(domain.RoleGovernance))` (`router.go:98`,
covering participants, approve-kyc, participants/provision, accounts/freeze, accounts/unfreeze, register)
and `govGroup` (`router.go:112-115`); Scenario B `govGroup` (`router.go:89-92`). Fiber applies group
middleware to the whole prefix, so a route-level `RequireRole(RoleAdmission)` runs *in addition to* the
group's guard. The original T014/T030 wording ("ensure the group-level guard does not keep governance able
to mutate") identified the wrong failure mode — the actual failure is that **Admission is locked out**.

**Verified empirically against Fiber v2.52.9 (2026-08-04)** — a standalone reproduction was built and run
against the same version the services use, rather than reasoning from the framework docs:

| Attempted fix | Result |
|---|---|
| Route on the parent group **after** a `Group("", …)` sub-group, carrying its own any-of guard | **403 for admission** — the sub-group's middleware is mounted at the parent prefix and still runs |
| Same path re-registered at **app level** after the group | **403 for BOTH roles** — group wants governance, route wants admission, nobody passes. A silent total lockout |
| Registered **before** the group exists | works, but order-dependent — rejected as fragile for a security boundary |
| **Group guard relaxed to the union + per-route guards** | **correct** — all nine matrix assertions passed |

**The in-repo precedent does not transfer.** `GET /api/v1/audit/logs` escapes `govGroup` only because it was
moved to a *different path* (`scenario-b/.../router.go:106-115` documents the reasoning). That remedy is
unavailable here: the governance portals call the affected paths directly — Scenario A
`services/api/registry.api.ts:14,19,39`, Scenario B `:20,35,65` — so the URLs are part of the contract.

**Consequence — relaxing widens.** After the change, any route inside a relaxed group without an explicit
per-route guard is reachable by **both** profiles. The exhaustive per-route tables in
contracts/authorization-matrix.md must be implemented in full; a forgotten route silently grants Admission a
governance capability. INV-5 asserts both directions over those tables.

**Asymmetry between the scenarios** (from the frontend API surface):
- **Scenario A**: both portal-critical routes (`GET /governance/registry`, `POST /governance/approve-kyc`)
  are inside `govGroup`. The relaxation is mandatory or the Admission profile is unusable in the portal.
- **Scenario B**: the portal-critical pair is `/compliance/participants` + `/compliance/approve-kyc`, both
  already per-route guarded, so they re-gate with a simple guard change. The `govGroup` relaxation is needed
  only for the API-only `POST /governance/participants` and `GET /governance/registry`.

**Test consequence**: INV-4 — every re-gated route needs an admission-only **positive** assertion; INV-5 —
every retained route needs an admission-only **negative** assertion. A governance-only negative assertion
alone passes even when the restructure was not done.

## R9. Route surface is wider than the original matrix — three unclassified routes

**Decision**: Classify all three explicitly rather than letting the group guard decide by default.

| Route | Where | Handler | Decision |
|---|---|---|---|
| `POST /governance/registry/csr` | A `:119`, B `:95` | `SubmitCSR` → `SignParticipantCSR` | **Stays `ROLE_GOVERNANCE`** — it mints a cert signed with the CB's **CA key**. Same invariant class as on-chain signing. FR-002's "CSR" is narrowed to off-chain record-keeping. See T045. |
| `GET /governance/registry` | A `:118`, B `:94` | `GetRegistry` | **`ROLE_GOVERNANCE` OR `ROLE_ADMISSION`** — this is the read surface `RegistryPage` consumes. Omitting it leaves an admission-only operator with an empty registry page despite FR-003a. |
| `POST /compliance/participants/provision` | A `:101` only | `ProvisionParticipant` | **Stays `ROLE_GOVERNANCE`** — sets arbitrary KYC status *including* `FROZEN`, so it is a freeze lever (FR-004), not onboarding admission. |

**Rationale**: All three sit inside groups being restructured, so "unlisted" silently resolves to
"governance-only" — which contradicts FR-002 for the CSR route and FR-003a for the registry read.

## R10. Role constants live at three sites per scenario, not two

**Decision**: Add `RoleAdmission` to **three** files per scenario:
`auth/internal/domain/roles.go`, `compliance/internal/domain/roles.go` **and**
`api-gateway/internal/domain/auth.go`.

**Rationale**: The routers resolve `domain.RoleGovernance` from the api-gateway's own domain package
(`api-gateway/internal/domain/auth.go:59` in both scenarios), which the original plan never mentions.
Without it, `RequireRole(domain.RoleAdmission)` in T030 does not compile.

**Related, deliberately excluded**: the same file's `adminRoles` map (and `IsAdminRole`) governs which roles
the CB may assign through `POST /compliance/register`; the hard-coded error string at
`handlers/compliance.go:229-232` (A) / `:230-233` (B) and the OpenAPI role enums
(`api-gateway/docs/openapi.yaml:1774,3458`, plus `scenario-a/apis/openapi/api-gateway.yaml`) mirror it.
Because the Admission login is provisioned from the entity manifest, `ROLE_ADMISSION` is **not** added
there. Recorded as a decision (T044) so the omission is deliberate.

## R11. Test anchor named in the first pass does not exist

**Decision**: Route-authz regression tests go in
`{scenario}/backend/services/api-gateway/internal/http/router/router_test.go`.

**Rationale**: `actor_authz_test.go` exists **only** at
`{scenario}/backend/services/compliance/internal/grpc/server/actor_authz_test.go`, and it is an R2-H-8
regression suite for **audit-actor derivation** (authenticated identity > `x-actor-subject` header > payload
actor) — it contains no HTTP or route-authorization tests at all. The real suite is `router_test.go`, with
`TestRequireRoleBlocksCommercialBank`, `TestGovernanceUsersRoutesRequireRole`,
`TestTransferLimitRoutesRequireTreasuryRole` and `TestHTLCMutatingRoutesRoleGate` as the patterns to extend.

**Reuse note for audit attribution (R5)**: `actorForAudit` already implements the approver-attribution
derivation and is already wired into `ApproveKYC` (`compliance/.../server.go:399` in A). FR-007 should reuse
it rather than introduce a second actor source — the existing test file locks its precedence rules.

## R12. The frontend gate is at login, not only at the route

**Decision**: The read-vs-mutate split must touch `stores/auth.store.ts` and `pages/LoginPage.tsx` in
addition to `authorization.ts`, `ProtectedRoute.tsx` and `RegistryPage.tsx`.

**Rationale**: `hasGovernanceAccess` has four consumers in each scenario's governance portal:
`stores/auth.store.ts:39` and `:73` (which reject the **session** with
`GOVERNANCE_UNAUTHORIZED_MESSAGE`), `pages/LoginPage.tsx:49`/`:48` (post-login redirect) and
`components/auth/ProtectedRoute.tsx:21`. Because the store gate runs at authentication, an Admission-only
operator cannot sign in at all — US1 acceptance scenario 1 ("signs in to the governance portal") fails
before any route is evaluated. The original plan listed only three of the five files.

**Addendum (third pass, 2026-08-04): relaxing the gate is necessary but NOT sufficient — per-page
authorization is required too.** `ProtectedRoute` takes **no props** and wraps every page through a single
route entry (`routes/index.tsx:21-36` in A, `:58-67` in B), and `components/layout/Sidebar.tsx` is a static
nav array with no role awareness. Admitting admission at the portal gate therefore grants navigation into
every governance page — A: Accounts, Audit, Settings; B: those plus SwapMonitor, CircuitBreaker,
TransferLimits, Oversight — each of which calls governance/treasury/supervisor-only APIs and would render
403s, with every link still visible. FR-008 requires the opposite. The frontend work is consequently **two
parts per scenario**: session/helpers (T016/T032) and per-route requirements + sidebar filtering
(T016a/T032a). Landing only the first part is a net regression, so they must ship together.

## R13. Scope boundary — the Admission profile is per-spoke

**Decision**: Record explicitly that 042 delivers a **per-spoke** Admission profile, and that single-identity
cross-spoke operation is deferred.

**Rationale**: The Admission login is declared in each central bank's own manifest and created in that
central bank's own Keycloak realm (Scenario A: `centralBankRealmPlans` → `renderRealmJSON`; Scenario B:
`appendKeycloakUsers` into the spoke realm, `step_found_spoke.go:358`). An external operator would need N
credentials across N realms. This is consistent with the declared Out of Scope, but the rework document's
functional requirement is "onboarding of banks must be operable by the external orchestrator **across
spokes**" — which needs the deferred realm-federation / shared-IdP phase. Stated so SC-001 is not read as
closing that requirement. See task T048.

## R14. A pre-existing, unused role occupies the same design slot

**Decision**: Proceed with the new `ROLE_ADMISSION`, and record why rather than leaving the overlap
unexamined.

**Rationale**: `ROLE_GOVERNANCE_OFFICER` already exists in both scenarios — declared at
`api-gateway/internal/domain/auth.go:72` (A) / `:64` (B), `auth/internal/domain/roles.go:15`,
`compliance/internal/domain/roles.go:12`; present in both `deploy/local/keycloak/init.sh` `BANK_ROLES` and
`CENTRAL_BANK_ROLES` arrays; in `adminRoles`/`IsAdminRole`; and in both OpenAPI role enums — yet **no
`RequireRole` call in either scenario references it**. It is password-only and admin-assignable: the same
structural slot as the Admission profile.

**Why a new role anyway**: `ROLE_GOVERNANCE_OFFICER` is already published as a role assignable to
*onboarded participants* (it appears in the `/compliance/register` allowlist and the public OpenAPI enums),
so reusing it would overload its meaning across two different concepts. `Admission` also names the
responsibility directly. **Follow-up**: open a separate cleanup ticket to wire or remove the dead role, so
the platform does not appear to carry two governance-adjacent password-only profiles by accident. See T047.
