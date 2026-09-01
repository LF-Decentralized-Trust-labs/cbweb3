# Tasks: Admission Profile for Commercial-Bank Onboarding

> **Amendment 1 (2026-09-01).** The project lead chose to keep the on-chain register+verify inside the
> Admission-authorized approval. Tasks that existed only to deliver or document the split — T027's
> post-split assertion, T046, and the FR-017 traceability row — are withdrawn and marked `[~]`. See the
> Amendment 1 section of `spec.md`. Every other task stands.

**Input**: Design documents from `/specs/042-admission-onboarding-profile/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/authorization-matrix.md, contracts/manifest-admission-user.md, quickstart.md

**Tests**: INCLUDED — Constitution Principle V (Test-First) and the spec's Test-First plan mandate a failing test before each implementation layer.

**Organization**: This feature ships as **two independent, scenario-isolated PRs** (Scenario B and Scenario A). Tasks are grouped by scenario track; within each track they follow test-first, layer order. `[US#]` labels map each task to a spec user story for traceability:
- **US1** Admission operator manages onboarding · **US2** Central bank retains on-chain signing · **US3** Governance keeps read + value/freeze, loses onboarding mutations · **US4** Local/pilot dual-grant migration.

> **Deviation from template note**: The standard template orders phases strictly by user story. Because delivery is two isolated PRs (Constitution I), the primary axis here is **scenario**; user stories are threaded through each track via `[US#]` labels. This keeps each PR independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- Paths are exact and repo-relative.

---

## Phase 1: Setup (shared design baseline)

**Purpose**: Confirm preconditions common to both PRs. No production code.

- [ ] T001 Confirm no contract change/redeploy is in scope: `IdentityRegistry` and all Solidity untouched; `contracts.sync-addresses` NOT run (per [research.md](./research.md) R7). Record in each PR description.
- [ ] T002 Confirm the local/pilot bootstrap operator will be dual-granted (`ROLE_GOVERNANCE` + `ROLE_ADMISSION`) BEFORE any route guard flips (per [research.md](./research.md) R6). Capture the ordering in the runbook task (T040).
- [ ] T003 Flag both PRs for project-lead approval (authorization-boundary move, per plan Constitution Check).

**Checkpoint**: Preconditions agreed; both tracks may begin.

---

## Phase 2: Foundational — role constant (BLOCKS all stories in each track)

**Purpose**: The `ROLE_ADMISSION` constant every downstream task depends on. Per scenario (each in its own PR).

> **THREE definition sites per scenario, not two.** Verification found the role constants duplicated in
> `auth/internal/domain/roles.go`, `compliance/internal/domain/roles.go` **and**
> `api-gateway/internal/domain/auth.go`. The routers resolve `domain.RoleGovernance` from the
> **api-gateway** package (`api-gateway/internal/domain/auth.go:59` in both scenarios) — omitting it makes
> the route re-gating tasks (T014/T030) fail to compile.

- [ ] T004 [P] Add `RoleAdmission = "ROLE_ADMISSION"` to `scenario-b/backend/services/auth/internal/domain/roles.go` — NOT added to `pkiRoles`/`onChainRoles`/`kmsRoles`.
- [ ] T005 [P] Add `RoleAdmission = "ROLE_ADMISSION"` to `scenario-b/backend/services/compliance/internal/domain/roles.go` — NOT added to `PKIRoles`.
- [ ] T005a [P] Add `RoleAdmission = "ROLE_ADMISSION"` to `scenario-b/backend/services/api-gateway/internal/domain/auth.go` (the constant the router's `RequireRole` calls resolve) — NOT added to the `adminRoles` map (Admission logins come from the manifest, not from `/compliance/register`; see T044).
- [X] T006 [P] Add `RoleAdmission = "ROLE_ADMISSION"` to `scenario-a/backend/services/auth/internal/domain/roles.go` — NOT added to `pkiRoles`/`onChainRoles`/`kmsRoles`.
- [X] T007 [P] Add `RoleAdmission = "ROLE_ADMISSION"` to `scenario-a/backend/services/compliance/internal/domain/roles.go` — NOT added to `PKIRoles`.
- [X] T007a [P] Add `RoleAdmission = "ROLE_ADMISSION"` to `scenario-a/backend/services/api-gateway/internal/domain/auth.go` — NOT added to the `adminRoles` map (see T044).

**Checkpoint**: Role constant exists at all three sites in both scenarios; route/handler/test work can begin.

---

## Phase 3: Scenario B track (PR-B) 🎯 MVP — cleaner (approve already off-chain)

**Goal**: Move mutating onboarding to `ROLE_ADMISSION`, keep reads on both profiles, keep CB as sole on-chain signer, add required manifest operator.

**Independent Test**: Admission-only signs in to the portal, reads `GET /compliance/participants` and completes `POST /compliance/approve-kyc` (the portal-critical pair in B), plus `POST /governance/participants` via the API; governance-only reads participants but is 403 on those mutations; on-chain register+verify in `CompleteOnboarding` is CB-signed.

### Tests for Scenario B (write FIRST, must FAIL) ⚠️

> **Test anchor correction.** `actor_authz_test.go` exists ONLY at
> `{scenario}/backend/services/compliance/internal/grpc/server/actor_authz_test.go` and it tests
> **audit-actor derivation** (R2-H-8), not HTTP route authorization. The route-authz regression suite is
> `api-gateway/internal/http/router/router_test.go` — see `TestRequireRoleBlocksCommercialBank`,
> `TestGovernanceUsersRoutesRequireRole`, `TestTransferLimitRoutesRequireTreasuryRole` for the existing
> pattern (build the app via `Setup`, inject claims via a stub validator, assert status codes).

- [ ] T008 [P] [US1] Authz regression: `ROLE_ADMISSION` caller is 2xx on `GET /api/v1/compliance/participants`, `POST /api/v1/compliance/approve-kyc`, `POST /api/v1/governance/participants` — extend `scenario-b/backend/services/api-gateway/internal/http/router/router_test.go` (per contracts/authorization-matrix.md).
- [ ] T009 [P] [US3] Authz regression: governance-only caller is 2xx on `GET /api/v1/compliance/participants` and `GET /api/v1/governance/registry` but 403 on `POST /approve-kyc` and `POST /governance/participants` — same test file. Then assert **INV-5 in both directions** over the exhaustive `govGroup` table in contracts/authorization-matrix.md: governance-only STILL 2xx **and** admission-only 403 on `POST /governance/registry/csr`, `/governance/accounts*`, circuit-breaker, parameters, audit and users. The admission-only negative half catches a route left unguarded after the group relaxation; enumerate the full table.
- [ ] T010 [P] [US4] Authz regression: caller holding BOTH roles is 2xx on all onboarding routes (INV-1) — same test file.
- [ ] T010a [P] [US1] Authz regression: `ROLE_ADMISSION`-only caller is 2xx on `GET /api/v1/governance/registry` — this is the read surface `RegistryPage` consumes, and it lives INSIDE the prefix-scoped `govGroup`, so it is the direct regression test for the group-restructure in T014 (INV-4).
- [ ] T011 [P] [US1] Approve-flow test: admission-authorized `ApproveKYC` moves participant to `KYC_APPROVED` and issues PoP nonce, with NO chain call — `scenario-b/backend/services/compliance/internal/grpc/server/` test.
- [ ] T012 [P] [US2] Invariant test (INV-2): enumerate ALL on-chain register+verify call sites in Scenario B and assert none is reachable from a `ROLE_ADMISSION`-authorized HTTP route. Verification found **three**, not one: (a) `auth/internal/grpc/server/onboarding.go:227,231` (`CompleteOnboarding` — bank-driven, CB-signed, the intended path); (b) `auth/internal/grpc/server/server.go:317,321` (`OnboardParticipant` — currently unreachable in B because B's router registers no `/compliance/register`; the test must LOCK that absence so it is safe by design, not by accident); (c) `compliance/internal/grpc/server/server.go` `RegisterParticipantOnChain` → `registry.EnsureVerifiedParticipant`, reached from `/internal/v1/spokes/register` (relay-auth, `router.go:181` — machine-to-machine, no user role, out of the Admission surface).
- [ ] T013 [P] [US3] Frontend route test: governance-only profile sees read views but not approve/register controls; admission-only profile authenticates successfully AND sees mutating controls — `scenario-b/frontend/apps/governance/` route + store tests. Must cover the login path (`stores/auth.store.ts`), not only `ProtectedRoute`.
- [ ] T013a [P] [US3] Frontend per-page authorization test (for T016a): an **admission-only** profile is redirected away from `/accounts`, `/circuit-breaker`, `/transfer-limits`, `/swap-monitor`, `/oversight`, `/audit` and `/settings`, and the sidebar renders only the pages it may use; a **governance-only** profile still reaches all of them. Without this test the widened portal gate passes unnoticed.

### Implementation for Scenario B

- [ ] T014 [US1] [US3] Re-gate the onboarding routes in `scenario-b/backend/services/api-gateway/internal/http/router/router.go`. **Two different mechanisms are involved — do not treat them the same:**
  - **(1) Per-route guards — a simple change.** `GET /compliance/participants` (`:78`) → `RequireRole("ROLE_GOVERNANCE","ROLE_ADMISSION")`; `POST /compliance/approve-kyc` (`:79`) → `RequireRole("ROLE_ADMISSION")`. These two are the **portal-critical** pair (Scenario B's portal calls exactly these — `frontend/apps/governance/src/services/api/registry.api.ts:20,35,65`), and they carry per-route guards already, so nothing structural is needed.
  - **(2) `govGroup` — requires relaxing the GROUP guard, not swapping a route guard.** `govGroup` (`:89-92`) carries `RequireRole("ROLE_GOVERNANCE")` at the group level, and Fiber group middleware is **prefix-scoped**. Verified against Fiber v2.52.9: adding `RequireRole("ROLE_ADMISSION")` to `POST /participants` makes it 403 for **both** roles (group wants governance, route wants admission, nobody passes); re-registering the same path at app level after the group also 403s. The only correct fix that preserves URLs is to change the **group** guard to the union `RequireRole("ROLE_GOVERNANCE","ROLE_ADMISSION")` and then give **every** route inside the group an explicit per-route guard. ⚠️ Relaxing the group widens it — any route left without a per-route guard becomes reachable by both profiles. Use the **exhaustive table** in contracts/authorization-matrix.md ("Scenario B — `govGroup`"): `POST /participants` → admission; `GET /registry` → both; `POST /registry/csr` → governance (T045); accounts, circuit-breaker, parameters, audit, users → governance.
  - Do NOT rely on registration order to escape a group guard (it works, but is fragile and unreviewable). Do NOT change any URL — the frontends call these paths directly.
  - Unchanged: supervisor summary (`:85`), `/api/v1/audit/logs` (`:111-115`, outside the group by path), and all `/api/v2/*` routes (different prefix).
  - Note: because no portal-critical route is inside `govGroup` in Scenario B, part (2) could in principle be deferred — but then `POST /governance/participants` stays governance-gated and FR-002 is unmet for the API. Land both parts.
- [ ] T015 [US1] [US2] Attribute the off-chain admission action to the Admission operator (from JWT claims) and the on-chain signature to the CB signer in the audit log — `scenario-b/backend/services/api-gateway/internal/http/handlers/governance.go` and `.../handlers/compliance.go` (additive, no schema change). Reuse the existing `actorForAudit` derivation in compliance (`actor_authz_test.go` locks its precedence: authenticated identity > `x-actor-subject` header > payload actor) — do NOT introduce a second actor source.
- [ ] T016 [US1] [US3] Frontend, part 1 — **session + authorization helpers** (`scenario-b/frontend/apps/governance/src/`): add `hasAdmissionAccess(profile)` and a combined `hasPortalAccess(profile)` (governance **or** admission) to `auth/authorization.ts`; admit `hasPortalAccess` at the login/session gate in `stores/auth.store.ts` (~lines 39 and 73 — `hasGovernanceAccess` currently rejects the session with `GOVERNANCE_UNAUTHORIZED_MESSAGE`, so an Admission-only operator cannot authenticate at all) and at the post-login redirect in `pages/LoginPage.tsx` (~line 48); render approve/register controls only when `hasAdmissionAccess` in `pages/RegistryPage.tsx`.
- [ ] T016a [US1] [US3] Frontend, part 2 — **per-page authorization** (`scenario-b/frontend/apps/governance/src/`). Relaxing the single portal-wide gate is not sufficient and is not safe on its own: `components/auth/ProtectedRoute.tsx` takes **no props** and wraps every page (`routes/index.tsx:58-67`), and `components/layout/Sidebar.tsx` is a **static nav array with no role awareness**. Without this task an Admission-only operator can navigate to Accounts, SwapMonitor, CircuitBreaker, TransferLimits, Oversight, Audit and Settings — all of which call governance/treasury/supervisor-only APIs and would render 403 errors — and every one of those links stays visible in the sidebar. Required: (a) give `ProtectedRoute` an optional required-roles prop (or add a per-route `handle`/metadata field consumed by it) so each entry in `routes/index.tsx` declares its own requirement; (b) mark `registry` as governance-or-admission and every other page as governance-only (Oversight stays supervisor-gated as today); (c) filter `Sidebar.tsx` nav items by the same predicate so an Admission-only operator sees only Dashboard and Registry; (d) confirm `DashboardPage` degrades without governance-only data, or mark it governance-only too. Satisfies FR-008's "governance-only controls MUST NOT be presented to an Admission-only operator".
- [ ] T017 [US4] Toolkit: map the bare `ADMISSION` manifest role to realm roles (`ADMISSION → {central_bank, ROLE_ADMISSION}`) in `scenario-b/toolkit/engine/orchestrator/admin_users.go` `realmRolesForAdminRole`. Note the `default` branch returns the role verbatim, so WITHOUT this the JWT claim lands as literal `ADMISSION` and every guard fails. `appendKeycloakUsers` (`step_found_spoke.go:379-396`) then creates the realm role idempotently and grants it — no separate realm-role task is needed on the toolkit path.
- [ ] T018 [US4] Toolkit: enforce the Admission operator as REQUIRED for central-bank manifests in `scenario-b/toolkit/engine/manifest/validate.go`; add a failing→passing validation test (contracts/manifest-admission-user.md V1/V2/V4). **Signature change required**: `validateAdminUsers(users []AdminUser, r *Result)` (`validate.go:222`) receives only the slice and has no entity context. Pass the entity role through (`spec.topology.role`, e.g. `central-bank` in `samples/brazil/central-bank-brazil.yaml:10`) and require `ADMISSION` only for `topology.role == "central-bank"`. `topology.role: hub` (`samples/hub/hub-cbweb3.yaml`) is EXEMPT — commercial banks join spokes, not the hub — and `join`-mode bank manifests are unaffected.
- [ ] T019 [US4] Keycloak local realm: add `ROLE_ADMISSION` to `CENTRAL_BANK_ROLES` in `scenario-b/deploy/local/keycloak/init.sh:370` (K1). **No edit needed** to `scenario-b/deploy/local/keycloak/realms/scenario-b-realm.json` — it enumerates only the bare Scenario B roles (`central_bank`, `commercial_bank`, `hub_operator`); the `ROLE_*` family is created by `init.sh` (K2, resolved). This is the `deploy/local` docker-compose path only; the toolkit path creates the role itself via T017 — both paths must be covered because both are used.
- [ ] T020 [P] [US4] Add the `ADMISSION` `adminUsers[]` entry to every `scenario-b/samples/*/central-bank-*.yaml` (argentina, colombia, brazil, proxy-smoke).
- [ ] T021 [US4] Local/pilot: dual-grant the bootstrap operator both `ROLE_GOVERNANCE` and `ROLE_ADMISSION` so existing single-operator flows pass.
- [ ] T022 [US1] [US2] [US3] [US4] E2E: run `make scenario-b.test`, `make scenario-b.tryout-us1/us2/us3` with the dual-grant bootstrap; add a governance-only negative case (mutation 403, read 200).

**Checkpoint**: Scenario B PR is complete and independently verifiable via [quickstart.md](./quickstart.md) (Scenario B section).

---

## Phase 4: Scenario A track (PR-A) — adds the A2 decoupling

**Goal**: Same outcome as B, plus split the currently-coupled `ApproveKYC` (A2) so the on-chain register+verify moves to a distinct CB-signed completion step.

**Independent Test**: Admission-only signs in to the portal, reads `GET /governance/registry`, completes approve-KYC (now chain-free) and registers a participant record via `POST /governance/participants`; on-chain register+verify runs in the CB-signed `CompleteOnboarding`; governance-only reads but is 403 on those mutations, while retaining certificate issuance and operator provisioning.

### Tests for Scenario A (write FIRST, must FAIL) ⚠️

> **Test anchor**: same correction as the Scenario B track — use
> `scenario-a/backend/services/api-gateway/internal/http/router/router_test.go` (existing patterns:
> `TestRequireRoleBlocksCommercialBank`, `TestHTLCMutatingRoutesRoleGate`,
> `TestTransferLimitRoutesRequireTreasuryRole`), NOT `actor_authz_test.go`.

- [X] T023 [P] [US1] Authz regression: `ROLE_ADMISSION` caller is 2xx on `GET /compliance/participants`, `POST /compliance/approve-kyc`, `POST /governance/approve-kyc`, `POST /governance/participants`, `GET /governance/registry` — extend `scenario-a/backend/services/api-gateway/internal/http/router/router_test.go`. The last two are **portal-critical** and both live inside `govGroup`, so they are the direct regression test for the group relaxation in T030 (INV-4). `POST /compliance/register` is **excluded** — it stays governance-gated per T029a option (i); if option (ii) is chosen instead, add it here and update contracts/authorization-matrix.md.
- [X] T024 [P] [US3] Authz regression: governance-only is 2xx on `GET /compliance/participants` and `GET /governance/registry` but 403 on the mutating onboarding routes — same test file. Then assert **INV-5 in both directions** over the exhaustive per-route tables in contracts/authorization-matrix.md: governance-only STILL 2xx **and** admission-only 403 on every governance-retained route — `POST /compliance/participants/provision`, `POST /compliance/accounts/{freeze,unfreeze}`, `POST /compliance/register`, `POST /governance/registry/csr`, `/governance/accounts*`, circuit-breaker, parameters, audit, users. The admission-only negative half is what catches a route left unguarded after the group was relaxed; enumerate the full table, not a sample.
- [X] T025 [P] [US4] Authz regression: caller with BOTH roles is 2xx on all onboarding routes (INV-1) — same file.
- [X] T026 [P] [US2] A2 invariant test (INV-2): `scenario-a/backend/services/compliance/internal/grpc/server/server.go` `ApproveKYC` NO LONGER calls `RegisterParticipant`/`VerifyParticipant`; and **no `ROLE_ADMISSION`-authorized HTTP route reaches `IdentityRegistry`**. Scenario A has TWO inline on-chain paths, not one — the test must cover both: (a) compliance `ApproveKYC` (`server.go:333`, register `:369`, verify `:372`) — removed by T029; (b) auth `OnboardParticipant` (`auth/internal/grpc/server/server.go:261`, register `:317`, verify `:321`), reached from `POST /api/v1/compliance/register` (`router.go:104` → `handlers/compliance.go:205`) — addressed by T029a.
- [~] T027 [P] [US1] **Amendment 1: rewritten.** Approve-flow test for an admission-authorized `ApproveKYC` — but it reaches `KYC_APPROVED` **with** the on-chain register+verify, not without. The post-split assertion this task described (`KYC_APPROVED` no longer implies on-chain `Verified`) is dropped: approval still confers verification. Coverage currently rests on the router-level `admission_authz_test.go`.
- [ ] T028 [P] [US3] Frontend route test: governance-only read-only vs admission mutate controls, plus admission-only login success — `scenario-a/frontend/apps/governance/` route + store tests.
- [ ] T028a [P] [US3] Frontend per-page authorization test (for T032a): an **admission-only** profile is redirected away from `/accounts`, `/audit` and `/settings` and sees only Dashboard + Registry in the sidebar; a **governance-only** profile still reaches all pages.

### Implementation for Scenario A

- [X] T029 [US2] A2 split: remove the inline `RegisterParticipant`/`VerifyParticipant` from `ApproveKYC` in `scenario-a/backend/services/compliance/internal/grpc/server/server.go:360-377`; move register+verify into the CB-signed completion step. **Host decision is already answered by the code** — Scenario A ALREADY HAS `CompleteOnboarding` at `scenario-a/backend/services/auth/internal/grpc/server/onboarding.go:172` (same file and line as Scenario B's) and it is currently chain-free. Port Scenario B's step-6 block (`scenario-b/.../auth/internal/grpc/server/onboarding.go:218-235` — `RequiresOnChain` guard, register at `:227`, verify at `:231`) into it, after the existing CSR-signing step. This supersedes the open implementation choice in research.md R2: no new completion endpoint is needed. Update the now-stale doc comments at `server.go:327-332` and `:351-368`, which assert the coupling as a deliberate invariant.
- [X] T029a [US2] A2 split, second path: `POST /api/v1/compliance/register` (`router.go:104`) → `ComplianceHandler.RegisterParticipant` (`handlers/compliance.go:205`) → auth `OnboardParticipant` (`auth/.../server.go:261`) performs an inline CB-signed register+verify (`:317,321`). T030 re-gates that route to `ROLE_ADMISSION`, which would leave an on-chain call inside an Admission-authorized request — the exact A1 shape the clarification rejected. Pick ONE and record it in the PR: **(i) recommended** — keep `POST /compliance/register` on `ROLE_GOVERNANCE` (it is CB operator-provisioning, not commercial-bank admission: note its `IsAdminRole` allowlist covers TREASURY/NOC/SUPERVISOR too) and drop it from the Admission set; or **(ii)** split `OnboardParticipant` so the on-chain leg defers to `CompleteOnboarding` as in T029. Option (i) is the smaller change and keeps FR-006 structural; it requires updating contracts/authorization-matrix.md and T023.
- [X] T030 [US1] [US3] Re-gate the onboarding routes in `scenario-a/backend/services/api-gateway/internal/http/router/router.go`. Scenario A is the harder of the two: **both portal-critical routes are inside `govGroup`**, so the group relaxation is mandatory — without it an Admission-only operator has no working onboarding surface at all (Scenario A's portal reads `GET /governance/registry` and approves via `POST /governance/approve-kyc` — `frontend/apps/governance/src/services/api/registry.api.ts:14,19,39`; it does **not** use `/compliance/participants`).
  - **Two groups must be relaxed, not swapped.** `centralBankRoutes := complianceGroup.Group("", RequireRole(domain.RoleGovernance))` (`:98`) and `govGroup` (`:112-115`). Verified against Fiber v2.52.9: because `Group("", …)` mounts its middleware at the parent prefix, a route registered on `complianceGroup` *after* line 98 still inherits the governance guard — "register it outside the sub-group" does **not** work unless it also moves earlier in the function, which is fragile. Change each group's guard to the union `RequireRole(domain.RoleGovernance, domain.RoleAdmission)`, then give **every** route inside an explicit per-route guard.
  - ⚠️ Relaxing widens: any route left unguarded inside a relaxed group becomes reachable by both profiles. Use the two **exhaustive tables** in contracts/authorization-matrix.md ("Scenario A — `centralBankRoutes`" and "Scenario A — `govGroup`"). Summary: admission-only → `/compliance/approve-kyc` (`:100`), `/governance/approve-kyc` (`:116`), `/governance/participants` (`:120`); both → `/compliance/participants` (`:99`), `/governance/registry` (`:118`); governance-only → `/compliance/participants/provision` (`:101`), `/compliance/accounts/{freeze,unfreeze}` (`:102-103`), `/compliance/register` (`:104`, per T029a), `/governance/registry/csr` (`:119`, T045), `/governance/accounts*` (`:122-124`), circuit-breaker (`:126-127`), parameters (`:129-130`), audit (`:132`), users (`:134-135`).
  - Do NOT change any URL, and do NOT rely on registration order. Supervisor summary (`:87`) unchanged.
- [ ] T031 [US1] [US2] Audit attribution (approver vs signer) in `scenario-a/backend/services/api-gateway/internal/http/handlers/governance.go` and `.../handlers/compliance.go`. Reuse the existing `actorForAudit` derivation (already wired at `compliance/.../server.go:399`); do not add a second actor source.
- [X] T032 [US1] [US3] Frontend, part 1 — **session + authorization helpers** (`scenario-a/frontend/apps/governance/src/`): `hasAdmissionAccess` + `hasPortalAccess` in `auth/authorization.ts`; login/session gate in `stores/auth.store.ts` (~lines 39, 73) and post-login redirect in `pages/LoginPage.tsx` (~line 49) — without these an Admission-only operator is rejected at login with `GOVERNANCE_UNAUTHORIZED_MESSAGE`; read-vs-mutate control gating in `pages/RegistryPage.tsx`.
- [X] T032a [US1] [US3] Frontend, part 2 — **per-page authorization** (`scenario-a/frontend/apps/governance/src/`), same rationale as T016a: `components/auth/ProtectedRoute.tsx` takes no props and wraps every page (`routes/index.tsx:21-36`), and `components/layout/Sidebar.tsx` is a static nav array (`Dashboard`, `Registry`, `Accounts`, `Audit`, `Settings`) with no role awareness. Add per-route required-roles support, mark `registry` governance-or-admission and the rest governance-only, and filter the sidebar by the same predicate so an Admission-only operator sees only Dashboard and Registry.
- [X] T033 [US4] Toolkit: add `ROLE_ADMISSION` to `requiredAdminRolesByEntity["central-bank"]` (`scenario-a/toolkit/engine/manifest/validate.go:286`, keyed off `spec.role`); add failing→passing validation test (V1/V2/V4). `commercial-bank` and `noc` entities stay unaffected.
- [X] T034 [US4] Toolkit: route `ROLE_ADMISSION` into the central-bank realm in `scenario-a/toolkit/engine/orchestrator/keycloak.go` — add it to the `adminUsersForRealmRoles(admins, "ROLE_GOVERNANCE", "ROLE_TREASURY", "ROLE_SUPERVISOR")` call inside `centralBankRealmPlans` (`keycloak.go:144`) so `renderRealmJSON` (`:171`) emits the login as a non-temporary password user (V3).
- [X] T035 [US4] Keycloak local realm: add `ROLE_ADMISSION` to `CENTRAL_BANK_ROLES` in `scenario-a/deploy/local/keycloak/init.sh:332` (K1). Note this is the `deploy/local` docker-compose path only; the toolkit path creates the realm role itself via T034 — both paths must be covered because both are used.
- [X] T036 [P] [US4] Add the `ROLE_ADMISSION` `adminUsers[]` entry to every `scenario-a/samples/*/central-bank-*.yaml` (argentina, colombia, brazil, proxy-smoke).
- [X] T037 [US4] Local/pilot: dual-grant the bootstrap operator both roles.
- [ ] T038 [US1] [US2] [US3] [US4] E2E: run the Scenario A backend + E2E/tryout suites with the dual-grant bootstrap; add a governance-only negative case; assert the A2 invariant end-to-end.

**Checkpoint**: Scenario A PR is complete and independently verifiable via [quickstart.md](./quickstart.md) (Scenario A section).

---

## Phase 5: Polish & Cross-Cutting

> **Runbook path correction.** `docs/runbooks/` does NOT exist at the repository root. The
> role-separation runbook is duplicated per scenario at
> `scenario-a/docs/runbooks/identity-registry-role-separation.md` and
> `scenario-b/docs/runbooks/identity-registry-role-separation.md`. A single shared runbook would violate
> Scenario-Scoped Independence — each PR edits only its own copy.

- [ ] T039 [P] (PR-B) Update `scenario-b/docs/runbooks/identity-registry-role-separation.md` to describe the Admission profile and the approver-vs-signer separation.
- [ ] T039a [P] (PR-A) Update `scenario-a/docs/runbooks/identity-registry-role-separation.md` with the same content, adapted to Scenario A's post-A2 flow.
- [ ] T040 [P] Add a short Admission-profile onboarding runbook per scenario (`scenario-{a,b}/docs/runbooks/admission-profile-onboarding.md`) covering provisioning order (grant `ROLE_ADMISSION` **before** flipping guards), the local/pilot dual-grant, and the approver-vs-signer audit trail.
- [ ] T041 Update each scenario `README.md` implementation-status where onboarding roles are described.
- [ ] T042 [P] Run `make contracts.test` / scenario contract suites to confirm NO contract change/regression (should be untouched).
- [ ] T043 Run [quickstart.md](./quickstart.md) validation for BOTH scenarios; confirm SC-001, SC-002, SC-002a, SC-003…SC-007.
- [ ] T043a **A/B parity check (FR-014 / SC-007)** — the one requirement no single-scenario task can verify, since the two PRs are independent. After both land, walk the same operator journey in each portal and confirm the observable outcomes match: which actions an Admission-only operator can perform, which are refused, which pages are visible, and that the on-chain signer is the central bank in both. Record the comparison in the second PR. Note the legitimate wiring differences that must NOT be "fixed" into parity: A's portal reads `/governance/registry` while B's reads `/compliance/participants`; `/compliance/register` exists only in A; the manifest role form is `ROLE_ADMISSION` in A and bare `ADMISSION` in B.

### Decisions to confirm (block their dependent tasks, not the whole phase)

> **Status**: the **recommended** position for each of these is already written into `spec.md` (FR-002a,
> FR-003b, FR-017, Assumptions) and into the contracts, so planning is not blocked and nothing is missing
> from the spec. These tasks are **project-lead confirmation** of a recorded decision, not open drafting. If
> a decision is reversed, the listed artifacts must be updated with it.

- [ ] T044 Confirm (recorded in spec Assumptions): the Admission operator is **manifest-only**, not creatable through `POST /compliance/register`. The `IsAdminRole` allowlist (`api-gateway/internal/domain/auth.go` `adminRoles`, both scenarios), the hard-coded error string at `handlers/compliance.go:229-232` (A) / `:230-233` (B), and the role enums in `api-gateway/docs/openapi.yaml:1774,3458` (plus `scenario-a/apis/openapi/api-gateway.yaml`) all enumerate assignable roles. **Recommended: manifest-only** — leave `adminRoles` and the OpenAPI enums untouched, and state it in the spec Assumptions so the omission is deliberate rather than an oversight.
- [ ] T045 Confirm the authority for `POST /governance/registry/csr` (`scenario-a/.../router.go:119`, `scenario-b/.../router.go:95` → `GovernanceHandler.SubmitCSR` → `SignParticipantCSR`). This mints a certificate signed by the **central bank's CA key** — the PKI analogue of the on-chain signing question. **Recorded position: stays on `ROLE_GOVERNANCE`** — FR-002 has been narrowed to the off-chain record-keeping steps and FR-002a added, preserving the invariant that every central-bank-key signature (on-chain *and* CA) stays with governance; the route appears as governance-retained in both authorization-matrix tables. If the lead reverses this, update FR-002/FR-002a, both matrix tables, T014/T030's per-route lists, T009/T024's INV-5 assertions and quickstart check 4, and add a CA-key invariant mirroring INV-2.
- [~] T046 **Amendment 1: not needed — there is no behavioural change to record.** Kept for provenance; reinstate only if the split is revisited. ~~Record the behavioural change A2 introduces in Scenario A: `scenario-a/.../compliance/.../server.go:351-368` documents the current coupling as a deliberate invariant — register+verify is blocking inside `ApproveKYC` so "no bank is ever marked KYC_APPROVED while missing on-chain", because `registerParticipant` alone leaves the participant `Pending`, which fails HTLC's `onlyVerified` check. After A2, `KYC_APPROVED` no longer implies on-chain `Verified` (matching Scenario B, where the bank-driven `CompleteOnboarding` closes the gap). Audit Scenario A tests, tryout scripts and E2E flows for any assumption that approval alone makes a bank able to transact, and note the change in the runbook (T039a).~~
- [ ] T047 Confirm the disposition of the pre-existing `ROLE_GOVERNANCE_OFFICER` (recorded in spec Assumptions and research R14). It is defined in both scenarios (`api-gateway/internal/domain/auth.go:72`/`:64`, `auth/internal/domain/roles.go:15`, `compliance/internal/domain/roles.go:12`), listed in both `deploy/local/keycloak/init.sh` role arrays, present in `IsAdminRole` and in both OpenAPI role enums — yet **no `RequireRole` in either scenario references it**. It is password-only and admin-assignable: structurally the same shape as the Admission profile. **Recommended: keep the new `ROLE_ADMISSION`** (clear name, and `ROLE_GOVERNANCE_OFFICER` is already published as a role assignable to *onboarded participants*, so reusing it would overload the semantics), and open a separate cleanup ticket to either wire or remove the dead role. Record the rationale so the platform is not seen to be carrying two governance-adjacent password-only profiles by accident.
- [ ] T048 Confirm with the rework-document stakeholder (Carolina, per that document's reporting line) that a **per-spoke** Admission profile is acceptable for this phase. Already stated in spec Assumptions: the login is declared in each central bank's manifest and created in that central bank's own Keycloak realm (A: `centralBankRealmPlans`; B: `appendKeycloakUsers` into the spoke realm, `step_found_spoke.go:358`), so one external operator needs N credentials across N realms. This satisfies the current scope but NOT the rework document's "onboarding must be operable by the external orchestrator **across spokes**", which requires the deferred realm-federation / shared-IdP phase. Confirming this prevents SC-001 being read as closing that requirement.

---

## Dependencies & Execution Order

- **Phase 1 (Setup)** → no deps.
- **Phase 2 (Foundational role constant)** → blocks all story tasks in the respective scenario.
- **Phase 3 (Scenario B / PR-B)** and **Phase 4 (Scenario A / PR-A)** are **independent of each other** — different PRs, no shared code. They may proceed in parallel or B-first (recommended MVP).
- **Phase 5 (Polish)** → after both tracks (or per-PR for that scenario's runbook/README slice).

### Within each scenario track

- Tests (T008–T013a incl. T010a for B; T023–T028a for A) MUST be written and FAIL before their implementation tasks.
- Frontend work is **two tasks per scenario**, both required: T016 + T016a (B), T032 + T032a (A). Landing only part 1 widens the portal gate without adding per-page authorization, which is a net regression — an Admission-only operator would reach every governance page. Do not merge part 1 alone.
- A2 split (T029 **and T029a**) should land before/with route re-gating in Scenario A so the invariant test (T026) is meaningful.
- Route re-gating before frontend gating before E2E.
- Manifest validation + keycloak wiring (T017–T020 / T033–T036) can proceed in parallel with route work but must be present before E2E dual-grant (T022 / T038).

### Decision tasks that gate implementation

- **T045** (CSR authority) gates the final route list in T014 and T030. Resolve it before writing the authz regression tests, or the tests encode the wrong contract.
- **T029a** (second Scenario A on-chain path) gates T030's route list and T023's assertions.
- **T044** (manifest-only Admission) gates T005a/T007a — specifically whether `ROLE_ADMISSION` joins the `adminRoles` map.
- **T047** (`ROLE_GOVERNANCE_OFFICER`) gates Phase 2 in principle: if the decision were to reuse that role instead, T004–T007a and every Keycloak/manifest task change shape. Resolve first; the recommendation is to proceed with the new role.
- **T046** and **T048** are recording tasks — they change no code and may land at any point in their PR.

### Parallel opportunities

- T004–T007a (role constants, all six) are `[P]`.
- All test tasks within a track marked `[P]` run together.
- The two scenario tracks (Phase 3 vs Phase 4) run fully in parallel across two developers/PRs.
- Sample-manifest edits (T020, T036) and runbook docs (T039, T040) are `[P]`.

---

## Implementation Strategy

### MVP first (Scenario B / PR-B)

1. Phase 1 Setup → Phase 2 role constants for B (T004, T005, T005a — all three sites) → Phase 3 (B).
2. STOP and validate Scenario B independently via quickstart.
3. Open PR-B (project-lead approval per T003).

### Incremental delivery

1. Scenario B (MVP) → test → PR-B.
2. Scenario A (adds A2 split) → test → PR-A.
3. Polish (runbooks, README, quickstart validation) per PR and finally cross-cutting.

---

## Requirement traceability (FR → tasks)

Every functional requirement maps to at least one task. Use this to answer "is FR-x implemented?" without
reading the whole file.

| Requirement | Tasks |
|---|---|
| FR-001 define the Admission profile | T004, T005, T005a, T006, T007, T007a |
| FR-002 Admission authorizes mutating onboarding | T014, T030; tests T008, T023 |
| FR-002a certificate issuance stays governance | T045; T014, T030; tests T009, T024 (INV-5) |
| FR-003 governance refused mutations | T014, T030; tests T009, T024 |
| FR-003a reads accessible to both | T014, T030; tests T009, T010a, T023, T024 |
| FR-003b operator provisioning stays governance | T029a, T030; test T024 |
| FR-004 governance retains all else | T014, T030; tests T009, T024 (INV-5 both directions) |
| FR-005 central bank signs on-chain | T029, T029a; tests T012, T026 |
| FR-006 Admission holds no key/on-chain role | T004–T007a (excluded from pki/onChain/kms); tests T012, T026 |
| FR-007 audit attributes approver vs signer | T015, T031 |
| FR-008 portal read-vs-mutate + hide governance controls | T016, T016a, T032, T032a; tests T013, T013a, T028, T028a |
| FR-009 assignable independently of governance | T021, T037 (dual-grant is optional, not required); proven by the admission-only cases in T008, T013, T023, T028 |
| FR-010 local/pilot dual-grant | T002, T021, T037; tests T010, T025 (INV-1) |
| FR-011 declarable via entity manifest | T017, T018, T020, T033, T034, T036 |
| FR-012 compliance gate not weakened or bypassed | T024, T009 (INV-5 — nothing is un-gated), T042, T001 |
| FR-013 no contract redeploy | T001, T042 |
| FR-014 identical observable behaviour A vs B | **T043a** (the only cross-scenario task; no single-PR task can verify it) |
| FR-015 Scenario A A2 split | T029; tests T026, T027 |
| FR-015a split covers **every** inline on-chain path | T029a; test T026 (both call sites enumerated) |
| FR-016 Admission-only can sign in | T016, T032; tests T013, T028; quickstart check 1 |
| ~~FR-017 approved ≠ authorised to transact~~ *(Amendment 1: withdrawn)* | — |

## Notes

- `[P]` = different files, no dependency on an incomplete task.
- `[US#]` maps tasks to spec user stories for traceability across the two PRs.
- Verify each test FAILS before implementing (Constitution V).
- No contract redeploy; `IdentityRegistry` untouched (FR-013).
- Provision `ROLE_ADMISSION` (dual-grant in local/pilot) BEFORE flipping route guards to avoid locking out operators (spec Risks).
