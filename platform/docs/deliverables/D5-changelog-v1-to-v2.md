# D5 — API v1 → v2 Changelog (Breaking Changes)

This changelog accompanies the D5 endpoint specification and records the
breaking changes between the original **API v1** description and the delivered
**v2.3.0** API surface. Changes are listed per scenario; each scenario keeps
its own spec (scenario isolation).

> Version note: both gateways now serve `info.version: 2.3.0`. Scenario A was
> already at 2.3.0/74 ops. Scenario B is bumped from the previously-served
> **2.2.0 → 2.3.0** as part of this deliverable (the `/api/v2` AMM surface was
> implemented but undocumented).

## Breaking changes — Scenario B (International Hub)

### B1. New `/api/v2` namespace for the entire AMM surface (additive but version-gated)
The Hub-and-Spoke AMM is exposed under a **new `/api/v2` prefix**, distinct
from the `/api/v1` core surface. Clients written against the v1 description
have no knowledge of swaps, bridging, liquidity, pairs, hub-currency,
circuit-breaker (v2), or oversight. To use them, clients must target `/api/v2`.

### B2. New authentication schemes required for v2 routes
- `/api/v2` machine-to-machine sovereign Central Bank flows accept a
  **Bearer JWT** (`Authorization: Bearer …`) in addition to the session
  cookie (`RequireAnyAuth`). v1 documented cookie auth only.
- `/internal/amm/*` routes require the **`X-Relay-Auth`** shared-secret header
  and are **not** user-authenticated. These are relay-to-gateway only and must
  not be called by end-user clients.

### B3. Role gating differs from v1
Several v2 operations are gated to specific roles not described in v1:
- Circuit-breaker `pause` / `resume-request` / `resume-sign`, oversight
  disclosure, hub-currency register/remove, pair propose/confirm,
  `token/mint-and-approve`, and `liquidity/sovereign-add` require the
  **Central Bank** role.
- `swap/exact-output` and `swap/cross-currency` require the **commercial bank**
  role.
- `liquidity/*` commit/add/remove require the **liquidity-provider** role.
Calls with an otherwise-valid session but the wrong role now receive `403`.

### B4. Conditional availability of v2 routes (per-gateway)
v2 route groups are registered only when their backing service is wired
(per-gateway dependency injection). A given gateway deployment therefore
exposes a **subset** of the v2 surface based on its role. A route that exists
on a Central Bank gateway may legitimately be absent (404) on a commercial bank
gateway, and vice-versa. v1 assumed a uniform surface.

### B5. Atomicity semantics for value movement changed
Cross-network value movement is now **lock → mint** (`/api/v2/bridge/lock-mint`)
and **burn → unlock** (`/api/v2/bridge/burn-unlock`), gated by the asymmetric
circuit breaker (1-of-N pause, 2-of-N resume). This replaces any v1 assumption
of a direct transfer; partial settlement is forbidden on production paths.

### B6. Spec consolidation (source-of-truth change)
The previously-fragmented, **unwired** specs
`apis/openapi/amm.yaml` (v2.0.0) and
`backend/services/api-gateway/openapi/v2/scenario-b.yaml` (v2.0.0) are
**superseded** by the single served `docs/openapi.yaml` (v2.3.0), and have now
been **deleted** — nothing served or linted them, so they could only drift.
Tooling that consumed those fragment files must switch to the served
`/openapi.yaml`.

## Breaking changes — Scenario A (Enhanced Correspondent Banking)

### A1. FX Agreement lifecycle + cross-spoke relay (already in v2.3.0)
Beyond the v1 core (auth, onboarding, compliance, governance, HTLC, token,
escrow), v2.3.0 documents the **FX Agreement** lifecycle (propose / accept /
reject / cancel / settle) with cross-spoke relay delivery tracking and the
internal relay endpoints consumed by the Cacti bridge. Clients on the v1
description are unaware of these.

> Scenario A's served spec was already at v2.3.0/74 ops prior to this
> deliverable; the only change made here was making its `/docs` page CDN-free
> (see below). No request/response contract changed.

## Non-API change applied to both scenarios (not breaking)

### Self-contained Swagger UI (offline `/docs`)
The `/docs` page no longer loads Swagger UI assets from `unpkg.com`. Assets
(swagger-ui-dist 5.32.6) are vendored and embedded, and served from
`GET /docs/swagger-ui/:asset`, and the bundle is initialised with
`validatorUrl: null` so the page never posts the gateway's spec URL to Swagger's
hosted validator. This is transparent to API consumers — no endpoint behavior or
contract changed — and makes `/docs` work fully offline on any host.

### Published Postman collections
Each scenario now ships a Postman v2.1.0 collection generated from its served
spec: `scenario-a/apis/postman/` (83 requests) and `scenario-b/apis/postman/`
(110 requests). They are generated artifacts, not a parallel source of truth —
regenerate with `bash apis/postman/generate.sh` (generator versions pinned in the
script) and let `.github/workflows/api-artifacts.yml` fail the build on drift.
See §6 of [`D5-endpoint-specification.md`](./D5-endpoint-specification.md).

### Scenario B redeem / fiat-exchange field names corrected in the spec
The served spec described the Scenario A field names on four Scenario B schemas.
`FiatExchangeResponse.tx_hash` and `ApproveRedeemResponse.fiat_mint_tx_hash` are
documented as **`mint_tx_hash`**, `RedeemRecord` carries `mint_tx_hash` instead of
`zeto_transfer_tx_hash` + `fiat_mint_tx_hash`, and `zeto_transfer_tx_hash` is no
longer a required field of `RequestRedeemRequest` (the handler never parsed it).
The API responses themselves are unchanged — only the documentation was wrong.
