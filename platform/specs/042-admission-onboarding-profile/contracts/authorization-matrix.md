# Contract — Route Authorization Matrix (both scenarios)

This is the **source of truth** for the authorization regression tests. Each row is a testable assertion:
given a caller holding exactly the listed role, the route MUST return the stated outcome.

**Test location**: `{scenario}/backend/services/api-gateway/internal/http/router/router_test.go`.
(`actor_authz_test.go` exists only under `compliance/internal/grpc/server/` and tests **audit-actor
derivation**, not route authorization — it is not the anchor for this contract.) Existing patterns to
follow: `TestRequireRoleBlocksCommercialBank`, `TestGovernanceUsersRoutesRequireRole`,
`TestTransferLimitRoutesRequireTreasuryRole`, `TestHTLCMutatingRoutesRoleGate`.

Legend: ✅ allowed (2xx/next) · ⛔ forbidden (403) · — not applicable.

---

## ⚠️ Load-bearing implementation constraint — group guards are prefix-scoped

In both scenarios the onboarding routes are gated at the **group** level, and Fiber group middleware
applies to the whole prefix. Adding a route-level `RequireRole(RoleAdmission)` does **not** override the
group's `RequireRole(RoleGovernance)` — it runs *in addition*, so an admission-only caller is rejected
with 403 before the handler is reached.

| Scenario | Group | Line | Routes it covers |
|---|---|---|---|
| A | `centralBankRoutes := complianceGroup.Group("", RequireRole(domain.RoleGovernance))` | `router.go:98` | `/participants`, `/approve-kyc`, `/participants/provision`, `/accounts/freeze`, `/accounts/unfreeze`, `/register` |
| A | `govGroup` with group-level `RequireRole(domain.RoleGovernance)` | `router.go:112-115` | all `/api/v1/governance/*` |
| B | `complianceGroup` per-route guards | `router.go:78-79` | `/participants`, `/approve-kyc` |
| B | `govGroup` with group-level `RequireRole("ROLE_GOVERNANCE")` | `router.go:89-92` | all `/api/v1/governance/*` |

### Verified behaviour (Fiber v2.52.9, executed 2026-08-04)

Three candidate fixes were tested against real Fiber. Only one works:

| Shape | What was tried | Result |
|---|---|---|
| Register the route on the parent group **after** the guarded sub-group (Scenario A's `Group("", …)` pattern) | route carries its own `RequireRole(gov, adm)` | **403 for admission** — the sub-group's middleware is mounted at the same prefix and still runs |
| Register the route at **app level** under the group's prefix, after the group | `app.Post("/api/v1/governance/participants", RequireRole(adm), …)` | **403 for BOTH roles** — group requires governance, route requires admission, nobody passes. Total lockout. |
| Register **before** the group is created | same route, earlier in the function | works, but depends on registration order — rejected as fragile and unreviewable for a security boundary |
| **Relax the group guard to the union, add precise per-route guards** | group `RequireRole(gov, adm)` + per-route `RequireRole(adm)` / `RequireRole(gov)` / none | **correct** — all nine matrix assertions passed, and URLs are unchanged |

**Required approach**: change each affected group's guard to the **union of the roles used by the routes
inside it**, then give every route inside an explicit per-route guard. Because `RequireRole` is any-of, the
group admits both profiles and each route enforces its own requirement.

**The URLs cannot change.** The `GET /api/v1/audit/logs` precedent escapes `govGroup` only because it was
moved to a *different path*. That option is not available here: the governance portals call these paths
directly — Scenario A `services/api/registry.api.ts:14,19` (`/governance/registry`) and `:39`
(`/governance/approve-kyc`); Scenario B `:20,35` (`/compliance/participants`) and `:65`
(`/compliance/approve-kyc`). The precedent proves prefix-scoping is real; its remedy does not apply.

> ⚠️ **Relaxing a group guard widens it.** After the change, any route inside the group that lacks an
> explicit per-route guard is reachable by **both** profiles. The per-route lists below are therefore
> exhaustive and must be implemented in full — a forgotten route silently grants Admission access to a
> governance capability. INV-5 exists to catch exactly that.

Consequence for testing: a guard swap that *looks* correct still fails INV-4. Every re-gated route needs an
admission-only positive case, and every retained route an admission-only **negative** case.

---

## Scenario A — `scenario-a/backend/services/api-gateway/internal/http/router/router.go`

> **Reading the "After guard" column.** Both groups in this scenario are relaxed to the role union, so
> **every** route inside them needs an explicit per-route guard — including the routes whose *authority* does
> not change. "Authority unchanged" is never the same as "code unchanged" for an in-group route: leaving one
> without a guard makes it admission-reachable. Only routes outside a relaxed group are literally untouched.

| Route | Method | Line | Before guard | After guard | Governance-only | Admission-only | Supervisor-only |
|---|---|---|---|---|---|---|---|
| `/api/v1/compliance/participants` | GET | `:99` | group `RequireRole(RoleGovernance)` | per-route `RequireRole(RoleGovernance, RoleAdmission)`; group relaxed to the union | ✅ | ✅ | ⛔ |
| `/api/v1/compliance/participants/summary` | GET | `:87` | `RequireSupervisorRole()` | **literally unchanged** — registered before the relaxed sub-group, so outside it | ⛔ | ⛔ | ✅ |
| `/api/v1/compliance/approve-kyc` | POST | `:100` | group `RequireRole(RoleGovernance)` | per-route `RequireRole(RoleAdmission)`; group relaxed to the union | ⛔ | ✅ | ⛔ |
| `/api/v1/compliance/participants/provision` | POST | `:101` | group `RequireRole(RoleGovernance)` | authority unchanged, but **must gain** an explicit per-route `RequireRole(RoleGovernance)` | ✅ | ⛔ | ⛔ |
| `/api/v1/compliance/accounts/freeze` | POST | `:102` | group `RequireRole(RoleGovernance)` | authority unchanged, but **must gain** an explicit per-route `RequireRole(RoleGovernance)` | ✅ | ⛔ | ⛔ |
| `/api/v1/compliance/accounts/unfreeze` | POST | `:103` | group `RequireRole(RoleGovernance)` | authority unchanged, but **must gain** an explicit per-route `RequireRole(RoleGovernance)` | ✅ | ⛔ | ⛔ |
| `/api/v1/compliance/register` | POST | `:104` | group `RequireRole(RoleGovernance)` | **pending T029a — recommended: authority stays `ROLE_GOVERNANCE`**, and **must gain** an explicit per-route guard | ✅ | ⛔ | ⛔ |
| `/api/v1/governance/approve-kyc` | POST | `:116` | group `RequireRole(RoleGovernance)` | per-route `RequireRole(RoleAdmission)`; group relaxed to the union | ⛔ | ✅ | ⛔ |
| `/api/v1/governance/registry` | GET | `:118` | group `RequireRole(RoleGovernance)` | per-route `RequireRole(RoleGovernance, RoleAdmission)`; group relaxed to the union | ✅ | ✅ | ⛔ |
| `/api/v1/governance/registry/csr` | POST | `:119` | group `RequireRole(RoleGovernance)` | authority unchanged (**CB CA signing**; T045), but **must gain** an explicit per-route `RequireRole(RoleGovernance)` | ✅ | ⛔ | ⛔ |
| `/api/v1/governance/participants` | POST | `:120` | group `RequireRole(RoleGovernance)` | per-route `RequireRole(RoleAdmission)`; group relaxed to the union | ⛔ | ✅ | ⛔ |
| Accounts, circuit-breaker, parameters, audit, users | * | `:122-135` | group `RequireRole(RoleGovernance)` | authority unchanged, but **each must gain** an explicit per-route `RequireRole(RoleGovernance)` — see the exhaustive table below | ✅ | ⛔ | ⛔ |

### Why `/compliance/register` is not simply re-gated (Scenario A)

`POST /api/v1/compliance/register` (`router.go:104`) → `ComplianceHandler.RegisterParticipant`
(`handlers/compliance.go:205`) → auth `OnboardParticipant` (`auth/internal/grpc/server/server.go:261`) →
inline CB-signed `RegisterParticipant` + `VerifyParticipant` (`server.go:317,321`). Moving this route to
`ROLE_ADMISSION` would place an on-chain call inside an Admission-authorized request — the A1 shape the
clarification rejected. It is also not commercial-bank admission: its `IsAdminRole` allowlist admits
`ROLE_TREASURY`, `ROLE_NOC` and `ROLE_SUPERVISOR` too, i.e. it is central-bank operator provisioning.
Recommended disposition is therefore "stays on `ROLE_GOVERNANCE`". See task T029a.

### Why `/governance/registry/csr` is not moved

`SubmitCSR` → `SignParticipantCSR` issues a certificate signed with the **central bank's CA key**. Under
the spec's own invariant — every central-bank-key signature stays with the central bank — this belongs on
`ROLE_GOVERNANCE`, and FR-002's mention of "CSR" should be read as the off-chain record-keeping steps of
the pipeline (credential-request intake, PoP tracking), not CA issuance. See task T045; if the decision
reverses, add a CA-key invariant mirroring INV-2 plus a corresponding test.

---

## Scenario B — `scenario-b/backend/services/api-gateway/internal/http/router/router.go`

> **Two mechanisms in this table.** The first three rows carry **per-route** guards already, so they are a
> plain guard change — and they are the **portal-critical** pair plus the supervisor summary. The `govGroup`
> rows require relaxing the group guard, after which every route inside it needs an explicit per-route guard
> even where its authority does not change.

| Route | Method | Line | Before guard | After guard | Governance-only | Admission-only | Supervisor-only |
|---|---|---|---|---|---|---|---|
| `/api/v1/compliance/participants` | GET | `:78` | per-route `RequireRole("ROLE_GOVERNANCE")` | per-route `RequireRole("ROLE_GOVERNANCE","ROLE_ADMISSION")` — plain swap, no group involved | ✅ | ✅ | ⛔ |
| `/api/v1/compliance/participants/summary` | GET | `:85` | `RequireSupervisorRole()` | **literally unchanged** | ⛔ | ⛔ | ✅ |
| `/api/v1/compliance/approve-kyc` | POST | `:79` | per-route `RequireRole("ROLE_GOVERNANCE")` | per-route `RequireRole("ROLE_ADMISSION")` — plain swap, no group involved | ⛔ | ✅ | ⛔ |
| `/api/v1/governance/participants` | POST | `:93` | group `RequireRole("ROLE_GOVERNANCE")` | per-route `RequireRole("ROLE_ADMISSION")`; group relaxed to the union | ⛔ | ✅ | ⛔ |
| `/api/v1/governance/registry` | GET | `:94` | group `RequireRole("ROLE_GOVERNANCE")` | per-route `RequireRole("ROLE_GOVERNANCE","ROLE_ADMISSION")`; group relaxed to the union | ✅ | ✅ | ⛔ |
| `/api/v1/governance/registry/csr` | POST | `:95` | group `RequireRole("ROLE_GOVERNANCE")` | authority unchanged (**CB CA signing**; T045), but **must gain** an explicit per-route `RequireRole("ROLE_GOVERNANCE")` | ✅ | ⛔ | ⛔ |
| Accounts, circuit-breaker, parameters, audit, users | * | `:96-116` | group `RequireRole("ROLE_GOVERNANCE")` | authority unchanged, but **each must gain** an explicit per-route `RequireRole("ROLE_GOVERNANCE")` — see the exhaustive table below | ✅ | ⛔ | ⛔ |
| `/api/v1/audit/logs` | GET | `:111-115` | `RequireRole("ROLE_GOVERNANCE","ROLE_TREASURY","ROLE_SUPERVISOR")` | **literally unchanged** — outside `govGroup` by path | ✅ | ⛔ | ✅ |

> **Note**: Scenario B uses **string literals** in `router.go`, not `domain.*` constants, for these
> routes. The new literal is `"ROLE_ADMISSION"`. The `domain.RoleAdmission` constant is still required —
> `api-gateway/internal/domain/auth.go` is the third role-definition site and Scenario B's v2 router uses
> `domain.*` constants throughout.
>
> **Not affected**: `router/v2` (`v2router.Register`) gates on the bare Scenario B realm roles
> (`central_bank`, `commercial_bank`, `ROLE_SUPERVISOR`) for swap/liquidity/oversight flows and registers
> no onboarding or participant-governance routes. `/internal/v1/spokes/*` (`:178-184`) is relay-auth
> machine-to-machine, with no user role involved.

---

---

## Exhaustive per-route guards after group relaxation

Because relaxing a group guard widens it (see the warning above), **every** route inside a relaxed group
needs an explicit guard. These lists are complete as of 2026-08-04; re-derive them if the routers change.

### Scenario A — `centralBankRoutes` (`complianceGroup.Group("", …)`, `router.go:98-104`)

Group guard becomes `RequireRole(domain.RoleGovernance, domain.RoleAdmission)`.

| Line | Route | Per-route guard |
|---|---|---|
| `:99` | `GET /participants` | explicit `RequireRole(RoleGovernance, RoleAdmission)` — state it even though the relaxed group already admits both, so the route's intent is auditable and survives a later group change |
| `:100` | `POST /approve-kyc` | `RequireRole(RoleAdmission)` |
| `:101` | `POST /participants/provision` | `RequireRole(RoleGovernance)` |
| `:102` | `POST /accounts/freeze` | `RequireRole(RoleGovernance)` |
| `:103` | `POST /accounts/unfreeze` | `RequireRole(RoleGovernance)` |
| `:104` | `POST /register` | `RequireRole(RoleGovernance)` (per T029a) |

### Scenario A — `govGroup` (`router.go:112-135`)

Group guard becomes `RequireRole(domain.RoleGovernance, domain.RoleAdmission)`.
**Both portal-critical routes are in this group** — Scenario A's portal reads the registry and approves KYC
here, so without this relaxation an Admission-only operator has no working onboarding surface at all.

| Line | Route | Per-route guard |
|---|---|---|
| `:116` | `POST /approve-kyc` | `RequireRole(RoleAdmission)` — **portal-critical** |
| `:118` | `GET /registry` | explicit `RequireRole(RoleGovernance, RoleAdmission)` — **portal-critical** |
| `:119` | `POST /registry/csr` | `RequireRole(RoleGovernance)` |
| `:120` | `POST /participants` | `RequireRole(RoleAdmission)` |
| `:122` | `GET /accounts` | `RequireRole(RoleGovernance)` |
| `:123` | `POST /accounts/freeze` | `RequireRole(RoleGovernance)` |
| `:124` | `POST /accounts/unfreeze` | `RequireRole(RoleGovernance)` |
| `:126` | `GET /circuit-breaker/status` | `RequireRole(RoleGovernance)` |
| `:127` | `POST /circuit-breaker/toggle` | `RequireRole(RoleGovernance)` |
| `:129` | `GET /parameters` | `RequireRole(RoleGovernance)` |
| `:130` | `PUT /parameters` | `RequireRole(RoleGovernance)` |
| `:132` | `GET /audit/logs` | `RequireRole(RoleGovernance)` |
| `:134` | `GET /users` | `RequireRole(RoleGovernance)` |
| `:135` | `GET /users/:userId` | `RequireRole(RoleGovernance)` |

### Scenario B — `govGroup` (`router.go:89-116`)

Group guard becomes `RequireRole("ROLE_GOVERNANCE", "ROLE_ADMISSION")`.
**No portal-critical route is in this group** — Scenario B's portal uses the per-route-guarded
`/compliance/participants` and `/compliance/approve-kyc` (`router.go:78-79`), which re-gate cleanly with a
simple guard change. The relaxation here is needed only for the two API-only routes below, so this group
restructure could be deferred without breaking the portal — but then `POST /governance/participants` stays
governance-gated and FR-002 is not met for the API.

| Line | Route | Per-route guard |
|---|---|---|
| `:93` | `POST /participants` | `RequireRole("ROLE_ADMISSION")` |
| `:94` | `GET /registry` | explicit `RequireRole("ROLE_GOVERNANCE","ROLE_ADMISSION")` |
| `:95` | `POST /registry/csr` | `RequireRole("ROLE_GOVERNANCE")` |
| `:96` | `GET /accounts` | `RequireRole("ROLE_GOVERNANCE")` |
| `:97` | `POST /accounts/freeze` | `RequireRole("ROLE_GOVERNANCE")` |
| `:98` | `POST /accounts/unfreeze` | `RequireRole("ROLE_GOVERNANCE")` |
| `:99` | `GET /circuit-breaker/status` | `RequireRole("ROLE_GOVERNANCE")` |
| `:100` | `POST /circuit-breaker/toggle` | `RequireRole("ROLE_GOVERNANCE")` |
| `:101` | `GET /parameters` | `RequireRole("ROLE_GOVERNANCE")` |
| `:102` | `PUT /parameters` | `RequireRole("ROLE_GOVERNANCE")` |
| `:103` | `GET /audit/logs` | `RequireRole("ROLE_GOVERNANCE")` |
| `:104` | `GET /users` | `RequireRole("ROLE_GOVERNANCE")` |
| `:116` | `GET /users/:userId` | `RequireRole("ROLE_GOVERNANCE")` |

`/api/v1/audit/logs` (`:111-115`) is outside the group by path and is unaffected. Scenario B's **v2** routes
live under `/api/v2/governance` (`router/v2/router.go:418`), a different prefix, so they are unaffected too.

---

## Invariant assertions (both scenarios)

- **INV-1 (dual-grant)**: a caller holding **both** roles is ✅ on every onboarding route (read + mutate).
- **INV-2 (no Admission signer)**: **no `ROLE_ADMISSION`-authorized HTTP route reaches
  `IdentityRegistry`.** Enumerate every on-chain register+verify call site per scenario and assert each is
  outside the Admission surface:

  | Scenario | Call site | Reached from | Disposition |
  |---|---|---|---|
  | A | `compliance/internal/grpc/server/server.go:369,372` (`ApproveKYC`) | `/compliance/approve-kyc`, `/governance/approve-kyc` | **removed by T029** |
  | A | `auth/internal/grpc/server/server.go:317,321` (`OnboardParticipant`) | `/compliance/register` (`router.go:104`) | route stays `ROLE_GOVERNANCE` per T029a |
  | A | `auth/internal/grpc/server/onboarding.go:172` (`CompleteOnboarding`) | `/api/v1/onboarding/complete` (bank-driven) | intended CB-signed path after T029 |
  | B | `auth/internal/grpc/server/onboarding.go:227,231` (`CompleteOnboarding`) | `/api/v1/onboarding/complete` (bank-driven) | intended CB-signed path |
  | B | `auth/internal/grpc/server/server.go:317,321` (`OnboardParticipant`) | **no route** — B's router registers no `/compliance/register` | lock the absence in the test so it is safe by design, not by accident |
  | B | `compliance/internal/grpc/server/server.go` `RegisterParticipantOnChain` → `registry.EnsureVerifiedParticipant` | `/internal/v1/spokes/register` (`router.go:181`, relay-auth) | machine-to-machine, no user role — outside the Admission surface |

- **INV-3 (read parity)**: governance-only and admission-only both succeed on the participant list **and**
  on `GET /governance/registry`; neither is silently downgraded.
- **INV-4 (group-guard regression)**: for every route moved out of a governance group, an
  **admission-only** caller is ✅. This is the assertion that fails if the implementation swaps guards
  without restructuring the groups.
- **INV-5 (no over-relaxation)**: relaxing a group guard widens it, so this invariant runs in **both**
  directions over the exhaustive per-route tables above:
  - a **governance-only** caller is still ✅ on every route the tables mark `RequireRole(Governance)` —
    certificate issuance, freeze/unfreeze, `/compliance/participants/provision` (A), `/compliance/register`
    (A), circuit-breaker, parameters, audit, users (FR-004);
  - an **admission-only** caller is ⛔ on every one of those same routes. This is the assertion that catches
    a route left without a per-route guard after the group was relaxed. It must enumerate the full table,
    not a sample.
