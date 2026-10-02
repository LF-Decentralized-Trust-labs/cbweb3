# Phase 1 Data Model — Admission Profile

This feature adds no database tables and no on-chain data. The "model" here is the **authorization
role model**, the **participant onboarding lifecycle**, and the **read-vs-mutate authorization matrix**.

## Roles / profiles (off-chain, Keycloak/JWT)

| Role | Prefix (A) | Bare (B) | New? | Onboarding capability | On-chain signing |
|---|---|---|---|---|---|
| Admission | `ROLE_ADMISSION` | `ADMISSION` | **Yes** | Approve KYC + register/advance the off-chain onboarding record; read participant/pending/registry. **Not** certificate issuance, **not** operator provisioning | **None** (no key, no on-chain role, no CA) |
| Governance (central bank) | `ROLE_GOVERNANCE` | `GOVERNANCE` | No | Read participant/pending/registry; **retains** certificate issuance (CSR signing) and central-bank operator provisioning; loses approve-KYC and participant-record registration | `GOVERNANCE_ROLE` + `VERIFIER_ROLE` via `CB_PRIVATE_KEY`, plus the **CA key** for CSR signing (all unchanged) |
| Supervisor | `ROLE_SUPERVISOR` | `SUPERVISOR` | No | Read-only summary (unchanged) | None |
| Treasury | `ROLE_TREASURY` | `TREASURY` | No | Reserve ops (unchanged) | Treasury ops (unchanged) |
| Commercial bank | `ROLE_COMMERCIAL_BANK` | — | No | Self-service onboarding (CSR/PoP) | Own key, after CB verify |

**Invariant**: The Admission profile is never added to `pkiRoles`, `onChainRoles`, or `kmsRoles`
(password-only profile). It holds no signing key of any kind and authorizes no operation that produces a
signature from a central-bank key — neither on-chain (`CB_PRIVATE_KEY`) nor PKI (the CB's CA key).

**On-chain roles (contract `IdentityRegistry`)**: `GOVERNANCE_ROLE`, `VERIFIER_ROLE`, `DEFAULT_ADMIN_ROLE`
— **unchanged**, all remain with the central-bank admin. Not touched by this feature.

## Role constant additions (code)

**Three sites per scenario**, not two — the routers resolve their constants from the api-gateway package.

| Scenario | File | Addition |
|---|---|---|
| A & B | `backend/services/auth/internal/domain/roles.go` | `RoleAdmission = "ROLE_ADMISSION"` (not in pki/onChain/kms groupings) |
| A & B | `backend/services/compliance/internal/domain/roles.go` | `RoleAdmission = "ROLE_ADMISSION"` (not in PKIRoles) |
| A & B | `backend/services/api-gateway/internal/domain/auth.go` | `RoleAdmission = "ROLE_ADMISSION"` — the constant `RequireRole(domain.Role…)` calls resolve (`:59` region). **Not** added to the `adminRoles` map / `IsAdminRole`: the Admission login is provisioned from the entity manifest, not through `POST /compliance/register` (decision T044) |

Consequently `ROLE_ADMISSION` does **not** appear in the assignable-role enums
(`api-gateway/docs/openapi.yaml:1774,3458`; `scenario-a/apis/openapi/api-gateway.yaml`) or in the
`handlers/compliance.go` role-validation error string. That omission is deliberate.

## Participant onboarding lifecycle (after A2 split — identical in A and B)

```
pending ──approve-kyc (ROLE_ADMISSION, off-chain)──▶ KYC_APPROVED (+ PoP nonce issued)
                                                          │
                        commercial bank proves possession │ (submits CSR + signed PoP nonce)
                                                          ▼
              sign participant CSR with the CB's CA key   (GOVERNANCE — never Admission, FR-002a)
                                                          ▼
        on-chain RegisterParticipant + VerifyParticipant  (central-bank signed, completion step)
                                                          ▼
                                                   VERIFIED / on-chain registered (can transact)
```

- **Off-chain boundary (Admission)**: `pending → KYC_APPROVED` + PoP nonce, and the onboarding *record* up
  to and including the CSR submission. No chain call, no certificate issued.
- **Key-operation boundary (governance)**: signing the participant CSR with the central bank's CA key, then
  `RegisterParticipant` (`GOVERNANCE_ROLE`) + `VerifyParticipant` (`VERIFIER_ROLE`) signed with
  `CB_PRIVATE_KEY`. Both legs run in the completion step; neither is Admission-authorized.
- ~~**FR-017 consequence**: between `KYC_APPROVED` and the completion step the participant is approved
  off-chain but **not** on-chain `Verified`, so it cannot transact. Only the completion step confers that.~~
  **Amendment 1 (2026-09-01): does not apply.** Approval and the on-chain register+verify stay in one
  action, so `KYC_APPROVED` continues to imply on-chain `Verified`, as it does today.
- **Scenario A change**: today these two boundaries are collapsed inside `ApproveKYC`
  (`compliance/.../server.go` inline `RegisterParticipant`/`VerifyParticipant`). A2 separates them.
- **Scenario B**: already separated — `ApproveKYC` (chain-free) then auth-service `CompleteOnboarding`.

## Authorization matrix (read vs mutate)

| Capability | Route (A) | Route (B) | Before | After |
|---|---|---|---|---|
| List participants / pending queue (read) | `GET /api/v1/compliance/participants` | same | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` **OR** `ROLE_ADMISSION` |
| Registry view (read) | `GET /api/v1/governance/registry` | same | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` **OR** `ROLE_ADMISSION` |
| Approve KYC (mutate) | `POST /api/v1/compliance/approve-kyc`, `POST /api/v1/governance/approve-kyc` | `POST /api/v1/compliance/approve-kyc` | `ROLE_GOVERNANCE` | **`ROLE_ADMISSION`** |
| Register participant record (mutate) | `POST /api/v1/governance/participants` | same | `ROLE_GOVERNANCE` | **`ROLE_ADMISSION`** |
| CB operator provisioning | `POST /api/v1/compliance/register` | (unrouted in B) | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` — **unchanged** (triggers an inline CB-signed on-chain register+verify; see T029a) |
| Sign participant CSR (CB **CA key**) | `POST /api/v1/governance/registry/csr` | same | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` — **unchanged** (T045) |
| Provision participant status (incl. `FROZEN`) | `POST /api/v1/compliance/participants/provision` | — | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` — **unchanged** |
| On-chain register+verify (sign) | `CompleteOnboarding` after A2 | `CompleteOnboarding` | CB (`CB_PRIVATE_KEY`) | CB (`CB_PRIVATE_KEY`) — **unchanged** |
| Supervisor summary (read) | `GET /compliance/participants/summary` | same | `ROLE_SUPERVISOR` | `ROLE_SUPERVISOR` — **unchanged** |
| Freeze/unfreeze, circuit breaker, parameters, audit, users | governance routes | governance routes | `ROLE_GOVERNANCE` | `ROLE_GOVERNANCE` — **unchanged** |

**Implementation constraint**: several of these routes sit behind **group-level**
`RequireRole(ROLE_GOVERNANCE)` middleware, which Fiber applies to the whole prefix. For those, swapping the
route guard does not work — verified against Fiber v2.52.9, it 403s **both** profiles, and re-registering the
path outside the group does not escape the prefix either. The correct change is to relax each affected
**group** guard to the union `RequireRole(governance, admission)` and give every route inside an explicit
per-route guard. Relaxing widens, so the per-route lists must be exhaustive. Routes already carrying
per-route guards (Scenario B's `/compliance/participants` and `/compliance/approve-kyc`) re-gate directly with
no structural change. URLs must not change — the portals call them. See
[contracts/authorization-matrix.md](./contracts/authorization-matrix.md) for the group inventory, the
exhaustive per-route tables, and invariants INV-1…INV-5.

## Operator login declaration (entity manifest)

| Scenario | Manifest role form | Required? | Wiring |
|---|---|---|---|
| A | `ROLE_ADMISSION` | **Yes** — added to `requiredAdminRolesByEntity["central-bank"]` | `keycloak.go` routes into central-bank realm; `renderRealmJSON` emits login |
| B | `ADMISSION` (bare) | **Yes** — enforced in B's `adminUsers[]` validation | `admin_users.go` `realmRolesForAdminRole` maps `ADMISSION → {…, ROLE_ADMISSION}` |

New `adminUsers[]` entry pattern (per central-bank sample manifest):

```yaml
# Scenario A
- role: ROLE_ADMISSION
  username: admin@<entity>.admission.gov
  password: <entity>-admission-local   # local profile only; secret store in staging/prod
# Scenario B
- role: ADMISSION
  username: admin@<entity>.admission.gov
  password: <entity>-admission-local
```

## Frontend view model (governance portal, both scenarios)

`hasGovernanceAccess(profile)` (`src/auth/authorization.ts:8`) has **four** consumers, and the first of them
runs at authentication — not at routing:

| Consumer | Line | Effect today | Required change |
|---|---|---|---|
| `stores/auth.store.ts` | `:39`, `:73` | Rejects the **session** with `GOVERNANCE_UNAUTHORIZED_MESSAGE` | Accept governance **or** admission — otherwise an Admission-only operator cannot sign in at all (US1 acceptance 1 fails before any route is evaluated) |
| `pages/LoginPage.tsx` | `:49` (A) / `:48` (B) | Post-login redirect into the portal | Accept governance **or** admission |
| `components/auth/ProtectedRoute.tsx` | `:21` | Blocks the whole protected surface with one hard-coded predicate | Honour the **route's own** declared requirement (see the per-page table below) — not a single widened predicate |
| `pages/RegistryPage.tsx` | — | Renders onboarding controls | Render approve/register only when `hasAdmissionAccess` |

- Keep `hasGovernanceAccess` as-is for governance-specific surfaces; add `hasAdmissionAccess(profile)` for
  the mutating controls, and a combined `hasPortalAccess(profile)` (governance **or** admission) for the
  session/route gates.

**Per-page authorization is required, not optional.** The portal has a **single** `ProtectedRoute` that takes
no props and wraps every page (`routes/index.tsx`), and `components/layout/Sidebar.tsx` is a static nav array
with no role awareness. Widening the one gate to governance-or-admission therefore grants an Admission-only
operator navigation into every governance page — Accounts, Audit, Settings in Scenario A; those plus
SwapMonitor, CircuitBreaker, TransferLimits and Oversight in Scenario B — each of which calls
governance/treasury/supervisor-only APIs and would render 403s, with every link still visible. The route
table must therefore carry per-route role requirements:

| Page | Required |
|---|---|
| `registry` | governance **or** admission |
| `dashboard` | governance (or admission-safe only if it makes no governance-only calls) |
| `accounts`, `circuit-breaker`, `transfer-limits`, `parameters`, `audit`, `settings`, `swap-monitor`, `htlc-monitor`, deposits/escrows/redeems approvals | governance (unchanged) |
| `oversight` | supervisor (unchanged) |

The sidebar must filter by the same predicate. Tracked as T016a (B) and T032a (A), with tests T013a/T028a.
