# Implementation Plan: Admission Profile for Commercial-Bank Onboarding

**Branch**: `042-admission-onboarding-profile` | **Date**: 2026-08-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/042-admission-onboarding-profile/spec.md`

## Summary

Introduce a new off-chain authorization profile, **`ROLE_ADMISSION`**, that owns the *mutating*
commercial-bank onboarding actions (approve KYC, register/advance the off-chain onboarding record — not
certificate issuance, not central-bank operator provisioning) in both
Scenario A and Scenario B, while the central bank retains **all** central-bank-key operations — on-chain
signing **and** certificate issuance. The read-only onboarding views (participant list, pending queue and
registry) stay accessible to both the Admission and Governance profiles. Scenario A is
brought in line with Scenario B by **fully splitting** its currently-coupled `ApproveKYC` (option A2):
the off-chain admission decision (Admission-authorized) is separated from the on-chain register+verify
(central-bank-signed completion step). No smart-contract change; the on-chain `IdentityRegistry` and its
roles are untouched.

Delivery is **two scenario-isolated PRs** (one per scenario, no shared code) driven by this single
validated design. The Admission operator login becomes a **required** `adminUsers[]` entry on every
central-bank entity declaration; all committed sample manifests are updated.

## Technical Context

**Language/Version**: Go 1.26+ (backend services + per-scenario `toolkit/`); TypeScript / React 18
(governance frontend)
**Primary Dependencies**: Fiber v2 (HTTP + `RequireRole` middleware), Keycloak OIDC (realm roles / JWT
claims), go-ethereum (on-chain register/verify, unchanged), GORM/Postgres (compliance + audit
persistence). No new runtime dependency.
**Storage**: Existing Postgres (`audit_log`, participant records). No schema change required; audit
records gain approver-vs-signer attribution using existing fields.
**Testing**: `go test` — route authorization in `api-gateway/internal/http/router/router_test.go` (the
regression anchor, R11), audit-actor precedence in `compliance/internal/grpc/server/actor_authz_test.go`
(existing, reused not extended), toolkit manifest validation, plus auth/compliance unit tests; Foundry
(contracts unchanged, no new tests needed); per-scenario E2E + tryout suites; frontend route **and store**
tests (R12).
**Target Platform**: Linux; Docker Compose per scenario.
**Project Type**: Web (Go backend services + React frontend) + Go provisioning toolkit, per scenario.
**Performance Goals**: No change to onboarding latency/throughput; authz check is O(roles). SC-005
requires no regression to existing E2E/tryout completion.
**Constraints**: Scenario isolation (no shared code A↔B); compliance gate never bypassed; central bank
remains sole on-chain signer; no contract redeploy (FR-013).
**Scale/Scope**: Two scenarios × {three role-constant sites, compliance/auth services, api-gateway routing
+ handlers, six governance-frontend files, toolkit manifest + keycloak wiring, sample manifests, keycloak
local realm, per-scenario runbooks}. Indicative ~18 code areas per scenario (see Project Structure).

**Resolved unknowns** (details in [research.md](./research.md)):
- `RequireRole(roles ...string)` is **any-of** in both scenarios → read-both routes use
  `RequireRole(governance, admission)`; mutating routes use `RequireRole(admission)`.
- The `toolkit/` is **duplicated per scenario** with different manifest→Keycloak mechanisms (Scenario A:
  `requiredAdminRolesByEntity` + `centralBankRealmPlans`/`renderRealmJSON`, `ROLE_`-prefixed manifest
  roles; Scenario B: `admin_users.go` `realmRolesForAdminRole`, **bare** manifest roles). Each PR touches
  its own toolkit only.
- No rendered `keycloak-import/*.json` exists under `samples/`; realm roles are created by
  `deploy/local/keycloak/init.sh` arrays — both scenarios' `init.sh` need the new role added, **and** the
  toolkit path provisions them separately (R3/K3).

**Second-pass constraints** (verification 2026-08-04, research.md R8–R14) — these change the shape of the
work, not just its detail:
- **R8** Fiber group middleware is prefix-scoped: the onboarding routes are gated at the *group* level in
  both routers, so a per-route `RequireRole(admission)` runs *in addition to* the group's governance guard.
  Verified against Fiber v2.52.9: swapping the route guard 403s **both** profiles, and re-registering the
  path outside the group does not escape the prefix. The fix is to relax each group guard to the **union**
  of roles used inside it and give every route an explicit guard — URLs cannot change, because the portals
  call them directly. Relaxing widens, so the per-route lists must be exhaustive (INV-5).
- **R9** Three routes were unclassified and would silently default to governance-only:
  `POST /governance/registry/csr` (CB **CA-key** signing → stays governance),
  `GET /governance/registry` (the read surface `RegistryPage` uses → must admit both),
  `POST /compliance/participants/provision` (A only; can set `FROZEN` → stays governance).
- **R10** Role constants live at **three** sites per scenario; the routers resolve them from
  `api-gateway/internal/domain/auth.go`, which the first pass omitted.
- **R11** The named test anchor `actor_authz_test.go` is an audit-actor suite under `compliance/`, not a
  route-authz suite; the real anchor is `api-gateway/.../router/router_test.go`.
- **R12** `hasGovernanceAccess` gates **login** (`stores/auth.store.ts`), not only routes — an
  Admission-only operator cannot authenticate without changing the store and `LoginPage`. And relaxing that
  gate is not sufficient: `ProtectedRoute` takes no props and wraps every page, while `Sidebar.tsx` is a
  static nav array, so the widened gate would grant an Admission-only operator navigation into every
  governance page. Per-route requirements + sidebar filtering are required (T016a/T032a).
- **R2 (corrected)** Scenario A **already has** `auth/.../onboarding.go:172 CompleteOnboarding`, chain-free
  — the A2 host question is answered and T029 shrinks. But A has a **second** inline on-chain path
  (`OnboardParticipant`, reached from `POST /compliance/register`) that the A2 split must also address, and
  A2 removes a documented invariant (`KYC_APPROVED` ⇒ on-chain `Verified`).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Ref: `.specify/memory/constitution.md` v1.0.4.

| Principle | Assessment | Status |
|---|---|---|
| **I. Scenario-Scoped Independence** | Single design doc here; two independent PRs, no shared code. The `toolkit/` is already duplicated per scenario, so each PR edits only its own tree. Cross-scenario spec is justified in Complexity Tracking. | PASS |
| **II. Privacy by Design** | No token path, on-chain data, or PII surface changes. | PASS |
| **III. Atomic Settlement Guarantee** | Onboarding is not a settlement path. Splitting off-chain approval from on-chain signing (A2) **strengthens** the register-vs-verify separation of duties; no partial-settlement path introduced. | PASS (improves) |
| **IV. Compliance Gate Before Participation** | Gate is **not weakened or bypassed**: onboarding still passes Compliance + `IdentityRegistry` + Keycloak. Only *which* off-chain role authorizes the mutating step changes. On-chain signing path (`CB_PRIVATE_KEY`) untouched. Re-check at payment initiation unaffected. | PASS (authz move, not removal) |
| **V. Test-First at Every Layer** | Failing tests precede implementation at each layer (authz regression, approve-flow attribution, invariant test, frontend gate, E2E negative case). Contracts unchanged → no new Foundry tests. | PASS |
| **VI. Observability and Auditability** | Additive: audit records attribute off-chain approver (Admission) vs on-chain signer (central bank); no silent swallowing. | PASS |

**PR-level gate**: This moves an authorization boundary → requires **project-lead approval** per the
Development Workflow, even though it does not weaken the gate. Flagged; not a blocker to planning.

**Result**: PASS. Proceed to Phase 0. (Re-checked post-design — see end of Phase 1; still PASS.)

## Project Structure

### Documentation (this feature)

```text
specs/042-admission-onboarding-profile/
├── spec.md              # Feature spec (clarified)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions & rationale
├── data-model.md        # Phase 1 — role/entity model & authz matrix
├── quickstart.md        # Phase 1 — how to run & verify both scenarios
├── contracts/
│   ├── authorization-matrix.md   # Route → required-role contract (both scenarios)
│   └── manifest-admission-user.md# adminUsers[] Admission entry contract (both scenarios)
├── checklists/
│   └── requirements.md  # Spec quality checklist (from /speckit.specify)
└── tasks.md             # Phase 2 — /speckit.tasks output (separate command)
```

### Source Code (repository root) — touched per scenario

Both scenarios share the same logical layout; each PR edits ONLY its own tree.

```text
scenario-a/                                    scenario-b/
  backend/services/
    auth/internal/domain/roles.go              # + RoleAdmission constant (both)
    compliance/internal/domain/roles.go        # + RoleAdmission constant (both)
    api-gateway/internal/domain/auth.go        # + RoleAdmission constant (both) — THIRD site,
                                               #   the one the routers actually resolve (R10).
                                               #   NOT added to adminRoles/IsAdminRole (T044)
    compliance/internal/grpc/server/server.go  # A: A2 split of ApproveKYC (remove inline
                                               #    register+verify at :369/:372; update the
                                               #    now-stale comments at :327-332, :351-368)
                                               # B: no chain change (already split)
    auth/internal/grpc/server/onboarding.go    # B: CB-signed CompleteOnboarding unchanged (:227,231)
                                               # A: CompleteOnboarding ALREADY EXISTS at :172 and is
                                               #    chain-free → port B's block into it (R2 corrected)
    auth/internal/grpc/server/server.go        # A: OnboardParticipant (:261) also registers+verifies
                                               #    inline (:317,321) — second on-chain path, see T029a
                                               # B: same code, but unrouted (no /compliance/register)
    api-gateway/internal/http/router/router.go # RELAX each governance group guard to the role UNION,
                                               #   then give EVERY route inside an explicit per-route
                                               #   guard (R8). Group guards are prefix-scoped, so a
                                               #   route-level swap 403s both profiles. The
                                               #   /api/v1/audit/logs precedent does NOT transfer — it
                                               #   escaped by changing PATH, which is impossible here.
                                               #   Relaxing widens: a route left unguarded becomes
                                               #   admission-reachable (INV-5)
    api-gateway/internal/http/handlers/        # governance.go / compliance.go: audit actor,
                                               #   reusing the existing actorForAudit (R5/R11)
  frontend/apps/governance/src/
    auth/authorization.ts                      # + hasAdmissionAccess / read-vs-mutate gates
    stores/auth.store.ts                       # ⚠ LOGIN gate (:39, :73) — rejects the session with
                                               #   GOVERNANCE_UNAUTHORIZED_MESSAGE; without this an
                                               #   Admission-only operator cannot sign in (R12)
    pages/LoginPage.tsx                        # ⚠ post-login redirect (~:48/:49) (R12)
    components/auth/ProtectedRoute.tsx         # takes NO props today and wraps every page — needs
                                               #   per-route required-roles support (T016a/T032a)
    routes/index.tsx                           # ⚠ declare per-route role requirements; today one
                                               #   portal-wide gate covers all pages
    components/layout/Sidebar.tsx              # ⚠ static nav array, no role awareness — must filter
    pages/RegistryPage.tsx                     # mutating controls behind admission
  toolkit/engine/
    manifest/validate.go                       # A: requiredAdminRolesByEntity += ROLE_ADMISSION (:286)
                                               # B: validateAdminUsers (:222) needs the entity role
                                               #    passed in — no requiredAdminRolesByEntity in B
    manifest/types.go                          # (no struct change expected)
    orchestrator/keycloak.go (A only)          # centralBankRealmPlans (:144) → ROLE_ADMISSION → realm
    orchestrator/admin_users.go (B only)       # realmRolesForAdminRole (:19): ADMISSION →
                                               #   {central_bank, ROLE_ADMISSION}; the default branch
                                               #   returns the role verbatim, so this is mandatory
  samples/*/central-bank-*.yaml                # + Admission adminUsers[] entry (4 per scenario;
                                               #   scenario-b/samples/hub/ EXEMPT)
  deploy/local/keycloak/init.sh                # + ROLE_ADMISSION to CENTRAL_BANK_ROLES
                                               #   (A :332, B :370) — docker-compose path only
  (B) deploy/local/keycloak/realms/scenario-b-realm.json  # NOT needed — enumerates bare roles only
  docs/runbooks/identity-registry-role-separation.md      # per scenario; docs/runbooks/ does NOT
                                               #   exist at the repo root (T039/T039a)
  api-gateway/internal/http/router/router_test.go  # the authz regression anchor (R11) — NOT
                                               #   compliance/.../actor_authz_test.go
```

**Structure Decision**: Web-app-per-scenario + per-scenario Go provisioning toolkit. No new top-level
directory; the change is additive within existing per-scenario trees. Scenario A carries the extra
`ApproveKYC` split (A2) plus the `OnboardParticipant` decision (T029a); Scenario B is authz + provisioning
only. The route work has two distinct shapes: routes already carrying **per-route** guards (Scenario B's
`/compliance/participants` + `/compliance/approve-kyc`) are a plain guard change, while routes behind a
**group** guard require relaxing that guard to the role union plus exhaustive per-route guards. Scenario A's
portal-critical routes are all in the second category, so A cannot ship the feature without the relaxation.

## Complexity Tracking

| Deviation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Single cross-scenario spec** (touches both scenario trees) | The Admission profile is the *same* role concept and must behave identically in A and B (FR-014); one validated design prevents divergence. Constitution I requires justification for cross-scenario work — provided here. | Two independent specs from the start were rejected: they would duplicate the shared model and risk A/B behavioural drift. Isolation is preserved where it matters — **implementation ships as two PRs with no shared code**. |
| **Scenario A `ApproveKYC` A2 full split** | Chosen in clarification to converge both scenarios on one approve-then-sign flow and to make the "central bank signs, Admission never signs" invariant structural rather than incidental. | A1 (keep coupled under CB signer) rejected as it leaves the on-chain call inside the Admission-authorized request, preserving A/B divergence and a weaker invariant. |
| **Admission `adminUsers[]` required immediately** | Chosen in clarification; guarantees every central bank declares an onboarding operator and avoids a half-migrated state. | Optional-first rejected by stakeholder; would leave manifests without an Admission operator silently valid. |
| **Relaxing the governance route-group guards to a role union** (rather than keeping a single narrow group guard) | Fiber group middleware is prefix-scoped, so a route inside a governance group cannot be re-gated to another role from the route level — verified against Fiber v2.52.9, the swap 403s both profiles. Relaxing the group to the union and enforcing precisely per route is the only shape that re-gates the routes without changing their URLs. | **Change the URL** (the `/api/v1/audit/logs` precedent) rejected: the governance portals call these paths directly, so it is a breaking API change for a first-party client. **Rely on registration order** (register before the group) rejected: it works but makes a security boundary depend on statement order, which is fragile and unreviewable. **Leave the routes governance-gated** rejected: FR-002 would be unmet. |
| **Per-route frontend authorization replacing a single portal-wide gate** | The portal has one `ProtectedRoute` with no props wrapping every page and a static, role-blind sidebar. Widening that one gate to admit Admission would expose every governance page to an Admission-only operator, contradicting FR-008. | Keeping the single gate and merely hiding controls inside `RegistryPage` rejected: the other pages remain reachable by URL and visible in the nav, so the operator sees governance surfaces full of 403s. |

## Phase 0 — Outline & Research

See [research.md](./research.md). All Technical-Context unknowns resolved (any-of middleware semantics,
per-scenario toolkit divergence, keycloak realm-role provisioning path, audit attribution approach,
local/pilot dual-grant). No NEEDS CLARIFICATION remains.

## Phase 1 — Design & Contracts

Artifacts produced:
- [data-model.md](./data-model.md) — the Admission/Governance role model, participant onboarding
  lifecycle (with the A2 split state boundary), and the full read-vs-mutate authorization matrix.
- [contracts/authorization-matrix.md](./contracts/authorization-matrix.md) — per-route required-role
  contract for both scenarios (before → after), the source of truth for the authz regression tests.
- [contracts/manifest-admission-user.md](./contracts/manifest-admission-user.md) — the required
  `adminUsers[]` Admission entry and per-scenario validation/keycloak wiring contract.
- [quickstart.md](./quickstart.md) — how to bring up each scenario and verify the five operator checks
  (Admission signs in and sees only its own surface; Admission mutates; Governance reads but cannot mutate
  onboarding; Governance keeps everything else including certificate issuance; the central bank signs), plus
  Scenario A's A2 assertions.

**Post-Design Constitution Re-Check**: Still PASS. The design adds one off-chain role; relaxes two route
groups per scenario to a role union with exhaustive per-route guards; adds per-route authorization to the
governance portal; splits Scenario A's `ApproveKYC` along an existing seam (its `CompleteOnboarding` already
exists) and rules one further coupled path out of the Admission surface; and adds a required manifest entry.
No new dependency, no contract change, no gate removed, no cross-scenario code sharing. Two shapes are
recorded in Complexity Tracking (group relaxation, per-route frontend authorization) because each replaces an
established pattern in the codebase.

## Phase 2 — Task planning approach (informational; executed by `/speckit.tasks`)

Tasks are grouped by **scenario** (A, B) and, within each, by **layer** following test-first order:
(1) role constants at all three sites + failing authz regression tests (including the admission-only
positive and admission-only negative cases that catch group over- and under-relaxation) → (2) route
re-gating via group relaxation + exhaustive per-route guards → (3) handler/audit attribution reusing the
existing `actorForAudit` → (4) Scenario A only: `ApproveKYC` A2 split + the second-path decision + invariant
test → (5) frontend in two parts — session/helpers, then per-route requirements and sidebar filtering →
(6) toolkit manifest validation (required Admission user, entity-keyed) + keycloak wiring → (7) sample
manifests + `init.sh` realm role → (8) E2E/tryout with dual-grant bootstrap + governance-only negative case.
A final cross-cutting phase carries the per-scenario runbooks, the decision confirmations, and the one
cross-scenario parity check that no single-PR task can perform (FR-014/SC-007). Each scenario's tasks are
independently completable and map to its own PR.
