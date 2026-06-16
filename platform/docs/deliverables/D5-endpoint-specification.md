# D5 — API Endpoint Specification (v2.3.0)

**Deliverable:** D5 — Endpoint Specification
**Status:** Updated to v2.3.0 (supersedes the original API v1 description)
**Scope:** REST API surface exposed by the per-entity **API Gateway** of each
scenario. gRPC intra-entity contracts are out of scope for D5.

## 1. Source of truth

Each scenario ships and **serves its own** self-contained OpenAPI 3.0 spec from
its API Gateway. There is no shared spec across scenarios (scenario isolation).

| Scenario | Spec file (served + embedded) | `info.version` | Operations | Paths |
|----------|-------------------------------|----------------|-----------|-------|
| Scenario A (Enhanced Correspondent Banking) | `scenario-a/backend/services/api-gateway/docs/openapi.yaml` | **2.3.0** | **74** | 66 |
| Scenario B (International Hub) | `scenario-b/backend/services/api-gateway/docs/openapi.yaml` | **2.3.0** | **93** | 87 |

The spec is embedded into the gateway binary (`//go:embed openapi.yaml`) and
served at:

- `GET /openapi.yaml` — the raw OpenAPI document.
- `GET /docs` — Swagger UI rendering of that document (see §5, now CDN-free).

Because the spec is the same file that is embedded and served, the **documented
version, the served version, and this D5 document are guaranteed to agree** for
each scenario.

### 1.1 Reconciliation note (Scenario B)

Before this deliverable, Scenario B was fragmented:

- The gateway **served** `docs/openapi.yaml` at **v2.2.0 / 57 ops** (counted as
  56 routed operations + the `GET /healthz` probe), documenting only the
  `/api/v1` core surface.
- Two **unwired, out-of-band** spec fragments existed and were never served:
  - `scenario-b/apis/openapi/amm.yaml` — "AMM API" **v2.0.0**, ~16 ops.
  - `scenario-b/backend/services/api-gateway/openapi/v2/scenario-b.yaml` —
    "Liquidity API" **v2.0.0**, ~15 ops.

The `/api/v2/*` AMM surface is in fact implemented and routed by
`scenario-b/backend/services/api-gateway/internal/http/router/v2/router.go`,
but was absent from the served spec.

**Reconciliation performed:** the `/api/v2/*` operations were folded into the
single served `docs/openapi.yaml`, and `info.version` was bumped **2.2.0 →
2.3.0** so the served spec matches the documented v2.3.0 surface. The two
unwired fragments are now **superseded** by the served spec; they are left in
place for historical reference but are no longer the source of truth.

> **Method / assumptions.** The v2 operation list was derived directly from the
> route registrations in `internal/http/router/v2/router.go` (method + path +
> auth middleware), not invented. Request/response bodies for v2 operations are
> documented as permissive free-form JSON objects where the handler contract is
> service-defined; the path, method, tag, auth scheme, and status codes are
> authoritative. v2 route groups are registered conditionally (per-gateway
> dependency injection), so a given deployment may expose a **subset** of the
> v2 surface depending on its role (commercial bank vs. central bank). The
> documented surface is the **union** of all routable v2 operations.

## 2. Scenario A — v2 surface (74 operations)

Grouped by domain (tags as they appear in the spec):

- **Health** — service liveness probe.
- **Authentication** — login, refresh, logout, `me`, client-secret change,
  PKI wallet bind / PKI login.
- **Onboarding (3-phase PKI + Blockchain)** — credential request / initiate,
  status polling (`status/{requestId}`, `my-status`), complete. Central Bank
  mode (public) and commercial-bank proxy mode.
- **Compliance** — KYC status, AML screen, participant provisioning,
  account freeze/unfreeze, register.
- **Governance** — participant registration, registry + CSR, accounts,
  circuit breaker status/toggle, parameters, audit logs, users.
- **HTLC (dual-layer)** — lock, settle (happy path), refund (timeout path),
  status, search.
- **Token** — mint, transfer, balance, fiat-balance.
- **Escrow / Payments** — deposit / escrow / redeem lifecycle with
  approve/reject and fiat-exchange.
- **FX Agreement** — propose / accept / reject / cancel / settle with
  cross-spoke relay delivery tracking, plus internal relay endpoints consumed
  by the Hyperledger Cacti cross-spoke bridge.

(Scenario A was already at v2.3.0/74 ops; this deliverable did not change its
spec content, only made its `/docs` page CDN-free — see §5.)

## 3. Scenario B — v2.3.0 surface (93 operations)

### 3.1 Core `/api/v1` surface (56 operations)

- **Health** — `GET /healthz`.
- **Authentication** — `/api/v1/auth/{login,refresh,logout,wallet/bind,
  client-secret/change,me,pki-login}`.
- **Onboarding (3-phase PKI + Blockchain)** — `/api/v1/onboarding/{initiate,
  credential-request,status/{requestId},my-status,complete}` (Central Bank
  mode + commercial-bank proxy mode).
- **Compliance** — `/api/v1/compliance/{kyc/status/{subject},aml/screen,
  participants,participants/provision,approve-kyc,register,accounts/freeze,
  accounts/unfreeze}`.
- **Governance** — `/api/v1/governance/{participants,registry,registry/csr,
  accounts,accounts/freeze,accounts/unfreeze,circuit-breaker/status,
  circuit-breaker/toggle,parameters,audit/logs,users,users/{userId}}`.
- **HTLC** — `/api/v1/htlc/{lock,settle,refund,status/{contractId},search}`.
- **Token** — `/api/v1/token/{mint,transfer,balance,fiat-balance}`.
- **Escrow / Payments** — `/api/v1/payments/{deposits,deposits/approve,
  deposits/reject,deposits/fiat-exchange,escrows,escrows/approve,
  escrows/reject,redeems,redeems/approve,redeems/reject}`.

### 3.2 Hub-and-Spoke AMM `/api/v2` surface (34 operations) + internal (3)

- **AMM Swap** (`/api/v2/amm`) — `quote/exact-output`,
  `quote/cross-currency`, `swap/exact-output`, `swap/cross-currency`,
  `swap/cross-currency/{id}`, `pool/{pair}/status`, `hub-liquidity-config`.
- **AMM Bridge** (`/api/v2/bridge`) — `lock-mint`, `burn-unlock`, `positions`.
  Implements the Scenario B atomicity rule (lock → mint, burn → unlock).
- **AMM Liquidity** (`/api/v2/amm/liquidity` + `/api/v2/amm/lp-balance`) —
  `add`, `remove`, `commit`, `commits`, `commits/{commit_id}` (GET + DELETE),
  `positions`, `sovereign-add`, and `lp-balance` (on-chain CBW3-LP shares).
- **AMM Token** (`/api/v2/amm/token`) — `mint-and-approve`, `approve-amm`.
- **Hub Currency Registry** (`/api/v2/hub`) — `currencies` (GET/POST),
  `currencies/{symbol}` (DELETE).
- **AMM Pair Registry** (`/api/v2/amm/pairs`) — `pairs` (GET), `pairs/propose`,
  `pairs/confirm` (two-Central-Bank flow).
- **Circuit Breaker** (`/api/v2/governance/circuit-breaker`) — `status`,
  `pause` (1-of-N), `resume-request` + `resume-sign` (2-of-N).
- **Oversight** (`/api/v2/oversight`) — `disclosure-request`,
  `disclosure-sign` (quorum), `disclosure-status/{requestID}`.
- **AMM Internal** (`/internal/amm`) — `execute-matched-commit`,
  `cross-currency-bridge-out`, `cross-currency-bridge-in`. Consumed by the
  Cacti relay; protected by the `X-Relay-Auth` shared secret, **not** user
  auth.

## 4. Security schemes

| Scheme | Type | Where | Used by |
|--------|------|-------|---------|
| `CookieAuth` | apiKey (cookie `access_token`) | Browser/session flows | most protected endpoints |
| `BearerAuth` | http bearer (JWT) | `Authorization` header | `/api/v2` M2M sovereign CB flows (RequireAnyAuth) |
| `RelayAuth` | apiKey (header `X-Relay-Auth`) | relay → gateway | `/internal/amm/*` |

`BearerAuth` and `RelayAuth` were added to the Scenario B spec in v2.3.0 to
describe the M2M and relay paths that the v1 spec omitted.

## 5. Self-contained Swagger (offline-capable `/docs`)

Previously, `/docs` loaded Swagger UI CSS/JS from `https://unpkg.com/...`, so
the page failed in an air-gapped/offline deployment. This is now fixed
**per scenario** with no change to API behavior:

1. The Swagger UI assets (`swagger-ui.css`, `swagger-ui-bundle.js`,
   swagger-ui-dist **5.32.6**) are vendored under
   `<scenario>/backend/services/api-gateway/docs/swagger-ui/` and embedded into
   the gateway binary via `//go:embed swagger-ui/*`.
2. `docs/spec.go` exposes `SwaggerUIAsset(name)` returning the embedded bytes
   for an allow-listed asset (anything else → 404).
3. The `/docs` HTML now references **gateway-local** paths
   (`/docs/swagger-ui/swagger-ui.css`, `/docs/swagger-ui/swagger-ui-bundle.js`)
   served by `GET /docs/swagger-ui/:asset`. No `unpkg`/CDN reference remains.

**Verification.** `TestSwaggerUI` asserts the HTML contains the local asset
paths and contains **no** external CDN reference (`unpkg.com`, `jsdelivr`,
`cdnjs`, or any `://`). `TestSwaggerUIAsset` asserts the CSS and JS bundle are
served with the correct content type and that an unknown asset name returns
404. Both gateways `go build` and these tests pass.

See the v1→v2 breaking-change list in
[`D5-changelog-v1-to-v2.md`](./D5-changelog-v1-to-v2.md).
