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
| Scenario A (Enhanced Correspondent Banking) | `scenario-a/backend/services/api-gateway/docs/openapi.yaml` | **2.3.0** | **83** | 74 |
| Scenario B (International Hub) | `scenario-b/backend/services/api-gateway/docs/openapi.yaml` | **2.3.0** | **110** | 99 |

Both counts are verified against the route registrations in
`internal/http/router/` — every registered route is documented, and the spec
documents no route that is not registered. `.github/workflows/api-artifacts.yml`
keeps the published Postman collections in step (see §6).

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

  Both have since been **deleted**: nothing served or linted them, so they could
  only drift further from the API they claimed to describe.

The `/api/v2/*` AMM surface is in fact implemented and routed by
`scenario-b/backend/services/api-gateway/internal/http/router/v2/router.go`,
but was absent from the served spec.

**Reconciliation performed:** the `/api/v2/*` operations were folded into the
single served `docs/openapi.yaml`, and `info.version` was bumped **2.2.0 →
2.3.0** so the served spec matches the documented v2.3.0 surface. The two
unwired fragments were **superseded and removed**, leaving exactly one spec per
scenario.

> **Method / assumptions.** The v2 operation list was derived directly from the
> route registrations in `internal/http/router/v2/router.go` (method + path +
> auth middleware), not invented. Request/response bodies for v2 operations are
> documented as permissive free-form JSON objects where the handler contract is
> service-defined; the path, method, tag, auth scheme, and status codes are
> authoritative. v2 route groups are registered conditionally (per-gateway
> dependency injection), so a given deployment may expose a **subset** of the
> v2 surface depending on its role (commercial bank vs. central bank). The
> documented surface is the **union** of all routable v2 operations.

## 2. Scenario A — v2 surface (83 operations)

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

(Scenario A was already at v2.3.0/83 ops; this deliverable did not change its
spec content, only made its `/docs` page fully self-contained — see §5.)

## 3. Scenario B — v2.3.0 surface (110 operations)

### 3.1 Core `/api/v1` surface (49 operations, including `GET /healthz`)

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

### 3.2 Hub-and-Spoke AMM `/api/v2` surface (44 operations) + internal (17)

- **AMM Swap** (`/api/v2/amm`) — `quote/exact-output`,
  `quote/cross-currency`, `swap/exact-output`, `swap/cross-currency`
  (POST initiate + GET own history), `swap/cross-currency/{id}`,
  `pool/{pair}/status`, `hub-liquidity-config`, `hub-config`.
- **AMM Bridge** (`/api/v2/bridge`) — `lock-mint`, `burn-unlock`, `positions`.
  Implements the Scenario B atomicity rule (lock → mint, burn → unlock).
- **AMM Liquidity** (`/api/v2/amm/liquidity`, `/api/v2/amm/lp-balance`,
  `/api/v2/amm/hub-reconciliation`) — sovereign seeding by escrow-and-finalize
  (`deposit-side`, `finalize`, `reclaim-side`, `escrow`), `remove`, `commit`,
  `commits`, `commits/{commit_id}` (GET + DELETE), `positions`,
  `sovereign-add`, `lp-balance` (on-chain CBW3-LP shares), and
  `hub-reconciliation`. Each Central Bank supplies **only its own side**; the
  dual-sided `liquidity/add` was removed as a sovereignty breach.
- **AMM Token** (`/api/v2/amm/token`) — `mint-and-approve`, `approve-amm`.
- **Hub Currency Registry** (`/api/v2/hub`) — `currencies` (GET/POST),
  `currencies/{symbol}` (DELETE), `token/supply` (this CB's outstanding
  wrapped-token supply).
- **AMM Pair Registry** (`/api/v2/amm/pairs`) — `pairs` (GET), `pairs/propose`,
  `pairs/confirm` (two-Central-Bank flow).
- **Circuit Breaker** (`/api/v2/governance/circuit-breaker`) — `status`,
  `pause` (1-of-N), `resume-request` + `resume-sign` (2-of-N).
- **Transfer Limits** (`/api/v2/governance/transfer-limits`) — CRUD, plus the
  CB-internal pre-auth relay under `/internal/v2/transfer-limits`.
- **Oversight** (`/api/v2/oversight`) — `disclosure-request`,
  `disclosure-sign` (quorum), `disclosure-status/{requestID}`.
- **AMM Internal** (`/internal/amm`) — `execute-matched-commit`,
  `cross-currency-bridge-out`, `cross-currency-bridge-in`,
  `cross-currency-hub-swap` (the CB-signed Hub AMM leg) and
  `cross-currency-residue-return`. Consumed by the Cacti relay; protected by
  the `X-Relay-Auth` shared secret, **not** user auth.
- **Internal payment relay** (`/internal/v1/payments`) and **spoke
  self-registration** (`/internal/v1/spokes/{register,register-currency,
  register-pair}`, hub gateway only, used by the provisioning toolkit). Same
  relay-auth guard; never exposed publicly.

## 4. Security schemes

| Scheme | Type | Where | Used by |
|--------|------|-------|---------|
| `CookieAuth` | apiKey (cookie `access_token`) | Browser/session flows | most protected endpoints |
| `BearerAuth` | http bearer (JWT) | `Authorization` header | `/api/v2` M2M sovereign CB flows (RequireAnyAuth) |
| `RelayAuth` | apiKey (header `X-Relay-Auth`) | relay → gateway | `/internal/amm/*`, `/internal/v1/*`, `/internal/v2/*` |

Operations that are genuinely public (health, login/refresh/wallet-bind, the
public AMM reads, hub currency and supply discovery) declare `security: []`
explicitly, so "no authentication" is a stated fact in the spec rather than an
omission. The onboarding endpoints declare `- {}` **and** `- CookieAuth: []`:
they are public on a Central Bank gateway and cookie-protected when a commercial
bank gateway serves the same path in proxy mode.

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
4. `SwaggerUIBundle` is initialised with `validatorUrl: null`. Left unset, the
   bundle posts the gateway's spec URL to Swagger's hosted online validator on
   every `/docs` load, suppressing that only when the URL resolves to localhost —
   so a page served from any other host would still call out.

**Verification.** `TestSwaggerUI` asserts the HTML contains the local asset
paths and contains **no** external CDN reference (`unpkg.com`, `jsdelivr`,
`cdnjs`, or any `://`). `TestSwaggerUIAsset` asserts the CSS and JS bundle are
served with the correct content type and that an unknown asset name returns
404. Both gateways `go build` and these tests pass.

## 6. Postman collections (packaged artifacts)

Each scenario publishes a Postman v2.1.0 collection generated from its **served**
spec — the same file described in §1, never a hand-maintained copy:

| Scenario | Collection | Requests | Folders (by OpenAPI tag) |
|----------|------------|----------|--------------------------|
| Scenario A | `scenario-a/apis/postman/cbweb3-scenario-a.postman_collection.json` | 83 | 13 |
| Scenario B | `scenario-b/apis/postman/cbweb3-scenario-b.postman_collection.json` | 110 | 17 |

Regenerate or verify with `bash apis/postman/generate.sh [--check|--lint-only]`
from the scenario directory (`make scenario-b.gen-postman`,
`scenario-b.check-postman` and `scenario-b.validate-openapi` alias the three
modes). Generator versions are pinned exactly in the script — `openapi-to-postmanv2`
**6.3.3** and `@redocly/cli` **1.34.5**; a floating range silently rewrites the
committed artifact.

`.github/workflows/api-artifacts.yml` runs the drift check on every change to a
served spec or a collection, so a documented surface that no longer matches the
published collection fails CI rather than shipping. The check compares the
request surface (folder, name, method, path, query keys, auth), which is
deterministic; per-item UUIDs and converter-faked example responses are stripped
during generation.

See the v1→v2 breaking-change list in
[`D5-changelog-v1-to-v2.md`](./D5-changelog-v1-to-v2.md).
