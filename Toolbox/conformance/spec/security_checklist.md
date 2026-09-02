# Security Checklist — CBWeb3 Toolbox (OWASP API Top 10)

> Contracts covered: `auth` / `pvp` / `amm` at **v2.3.0** | Revised: 2026-08-20
> Based on: [OWASP API Security Top 10 (2023)](https://owasp.org/API-Security/)

---

## Purpose

This checklist adapts the OWASP API Security Top 10 to what the **delivered CBWeb3
API Gateway v2.3.0** actually exposes — tokenised Central Bank money, dual-layer
HTLC settlement (Scenario A), hub-and-spoke AMM settlement (Scenario B) and
privacy-preserving Zeto transfers.

Every row states what the platform **actually does**, not what a well-designed API
would do. Where the platform lacks a control, the row says so plainly and the
absence is carried into the DPG evidence rather than papered over. Nothing here
describes a control the Toolbox invented.

Each risk is classified by **applicability** (Toolbox artefacts, implementations,
or both), **verification method**, and **current coverage**.

---

## Headline findings

These four are the security-relevant facts an integrator or assessor most needs,
and each is a property of the delivered platform:

1. **There is no CSRF protection of any kind.** No `X-CSRF-Token`, no double-submit
   cookie, no synchroniser token, no Origin or Referer check. An exhaustive grep
   across both platform OpenAPI documents and the gateway source finds nothing. The
   session credential is a cookie, so `SameSite=Strict` on `access_token` is the
   **sole** mitigation — and it is a browser-enforced one, absent for any non-browser
   client. **Recorded as an open finding.** The Toolbox does not add a CSRF header:
   inventing one would misrepresent the platform to implementers.
2. **The `Secure` cookie flag is environment-dependent.** It follows the gateway's
   `COOKIE_SECURE` variable — "true for HTTPS, false for HTTP". A misconfigured
   deployment transmits the session credential in clear text with no signal in the
   API surface that it is doing so.
3. **There is no machine-readable error code on `/api/v1`.** The universal body is
   `{error: "<free-form string>"}`, and that same field carries a human sentence in
   one place (`"insufficient permissions"`) and a code in another
   (`INVALID_CERTIFICATE_CHAIN`, `NONCE_SIGNATURE_MISMATCH`). `ErrorCodeResponse
   {error, code}` is declared in both platform specs and referenced by **zero**
   operations, while the live `/api/v2` handlers emit `error_code`. Security
   automation cannot branch on error identity below `/api/v2`.
4. **Authorisation exists only in prose.** No OAuth2 scopes, no `x-` extension, no
   per-role security scheme. Roles appear exclusively in `summary`/`description`
   strings and in request/response enums, so no role matrix can be generated from
   the contract and none can be enforced by a validator.

---

## Checklist

### API1:2023 — Broken Object Level Authorization (BOLA)

| Attribute | Value |
|---|---|
| **Risk** | A participant reads or acts on another participant's FX agreement, HTLC lock, bridge position or swap by supplying its `tradeId`, `contractId`, `position_id` or swap `id`. |
| **Applies to** | Implementations |
| **Verification** | Live deployment only — needs two provisioned credential sets |
| **CI automatable?** | No |
| **What the platform does** | **Structurally sound where it matters:** caller identity is derived from the JWT `BankID` claim, never from the body. `payer_bank_id`, `payer_id`, `owner_bank_id`, `provider_id`, `spoke_network`, `native_asset` and `mirrored_asset` are all marked `deprecated` and ignored; `requester_besu_address` and `requester_paladin_identity` are overwritten by the commercial-bank proxy from the verified caller. Nine deprecated identity fields are annotated in the `amm` contract alone. |
| **Gap** | Path-addressed reads declare **404 but no 403** (`getFXAgreement`, `getHTLCStatus`, `getCrossCurrencySwapStatus`), so the contract cannot express "you are not a party to this". Whether a non-party gets 404 or the record is undocumented. |
| **Toolbox coverage** | Not asserted. A one-credential conformance run cannot distinguish "not found" from "not yours". |
| **Recommended action** | Provision a second participant credential set in the sandbox and add cross-participant read vectors. Ask that 403 be declared on the three path-addressed reads. |

### API2:2023 — Broken Authentication

| Attribute | Value |
|---|---|
| **Risk** | Session forgery, replay, or a client that believes it is authenticated when it is not. |
| **Applies to** | Both |
| **Verification** | CI (401 paths) + live deployment (both login flows) |
| **CI automatable?** | Yes, partly |
| **What the platform does** | `CookieAuth` — `apiKey` in cookie `access_token`, `HttpOnly`, `SameSite=Strict`, `Path=/`. `BearerAuth` exists only in Scenario B, only as an alternative on the 12 `/api/v2` `RequireAnyAuth` routes; the two cross-currency swap routes are cookie-only. `RelayAuth` (`X-Relay-Auth`, shared secret) guards `/internal/*`, which the Toolbox does not publish. Identity provider is Keycloak. `POST /auth/login` branches on the participant record: direct (tokens + cookies) or PKI nonce (`{"nonce": "<hex>"}`, no tokens, no cookies) followed by `/auth/wallet/bind` with a DER-encoded ECDSA signature and the participant certificate. |
| **Findings** | (a) **No CSRF control** — see headline 1. (b) `POST /api/v1/auth/refresh` is **public** and reads `refreshToken` from the JSON **body**, while the `refresh_token` cookie is `HttpOnly` — a browser SPA cannot supply the value it is required to send. (c) The three `Set-Cookie` examples disagree on `Max-Age` (300 / 900 / 3600) and the real access-token lifetime is never stated. (d) The `refresh_token` cookie is set and cleared but is **not** a declared security scheme, so no operation formally consumes it. (e) The PKI 200 on `/auth/login` is typed as `AuthResponse`, which has no `nonce` property — a strict response validator rejects a correct commercial-bank login. (f) The platform never states whether the nonce is signed as its hex text or its decoded bytes. |
| **Toolbox coverage** | **Good.** 12 `mock_safe` error tests assert 401 without the cookie across auth, pvp and amm. The session fixture exercises both real login flows; `test_direct_login_returns_tokens_and_sets_an_httponly_cookie` asserts `HttpOnly` and `SameSite=Strict` on the wire. `Secure` is deliberately not asserted (see headline 2). |
| **Recommended action** | Have deployments pin `COOKIE_SECURE=true`. Ask upstream to type the nonce response, document the nonce signing encoding, and state the token lifetime. |

### API3:2023 — Broken Object Property Level Authorization

| Attribute | Value |
|---|---|
| **Risk** | A caller sets a server-owned property — `state`, `status`, `tx_hash`, `bridge_state`, `created_at` — through a request body. |
| **Applies to** | Both |
| **Verification** | PR review + Spectral |
| **CI automatable?** | Partly |
| **What the platform does** | Request and response schemas are **separate types** throughout (`ProposeFXAgreementRequest` vs `FXAgreement`, `BridgeLockMintRequest` vs `BridgePositionResult`), which is the stronger control: a server-owned field simply has nowhere to be sent. |
| **Gap** | **`readOnly: true` appears zero times in all three contracts**, mirroring the platform, so the separation is structural rather than annotated. Any future merge of a request and response schema would silently lose it. |
| **Toolbox coverage** | Not directly asserted. The suite never sends a server-owned field, which is itself the documented client contract. |
| **Recommended action** | Keep request and response schemas separate as an explicit contract rule. Consider a Spectral rule that fails any schema used in both a `requestBody` and a response. |

### API4:2023 — Unrestricted Resource Consumption

| Attribute | Value |
|---|---|
| **Risk** | Flooding settlement endpoints causes denial of service or unbounded on-chain cost. |
| **Applies to** | Implementations |
| **Verification** | Live deployment |
| **CI automatable?** | No |
| **What the platform does** | **No rate-limiting surface exists.** No `429` anywhere in either spec, no `X-RateLimit-*` headers, and in fact **no operation-level header parameters at all** — the single `in: header` occurrence per file is the `X-Relay-Auth` security-scheme definition. The only volumetric control is a business one: daily transfer limits (Scenario A `/api/v1/treasury/transfer-limits`, Scenario B `/api/v2/governance/transfer-limits`), enforced with `422 TRANSFER_LIMIT_EXCEEDED`. |
| **Gap** | Pagination is inconsistent and mostly absent: three different styles exist and most collections — participants, FX agreements, deposits/escrows/redeems, bridge positions, liquidity commits, HTLC search — are **unpaginated bare arrays**. There is no cursor, no `Link` header and no offset parameter anywhere. Only Scenario B's swap history has a complete `page`/`page_size`/`total` envelope. |
| **Toolbox coverage** | The swap-history pagination envelope is asserted. No rate-limit assertion is possible. |
| **Recommended action** | Rate limiting belongs at the ingress in front of the gateway; document it as a deployment requirement, since the API surface cannot express it. |

### API5:2023 — Broken Function Level Authorization

| Attribute | Value |
|---|---|
| **Risk** | The originator accepts its own FX agreement; a non-bank session locks or settles an HTLC; a non-Central-Bank session approves an escrow or pauses a corridor. |
| **Applies to** | Both |
| **Verification** | Live deployment with multiple credential sets |
| **CI automatable?** | No |
| **What the platform does** | Enforcement is real but **invisible to the contract**: the router gates the four mutating HTLC routes with `RequireRole(ROLE_BANK, ROLE_COMMERCIAL_BANK, ROLE_TREASURY, ROLE_GOVERNANCE)` while **the platform document declares no 403 on any of them** (the Toolbox contract adds it, marked `x-cbweb3-source: implementation-observed`). Governance, supervisor and treasury scoping is stated in tag descriptions and YAML comments only. |
| **Findings** | (a) Two role namespaces coexist in the same token: `ROLE_`-prefixed for `/api/v1`, lowercase (`central_bank`, `commercial_bank`, `mlp`) for `/api/v2`. **`ROLE_CENTRAL_BANK_SCENARIO_B` is a Go constant name written into the prose, not a role a JWT will ever carry.** (b) Four Scenario A endpoints say "CENTRAL_BANK only" using an unprefixed name that matches **no** declared role enum. (c) `ROLE_GOVERNANCE` and `ROLE_GOVERNANCE_OFFICER` both occur and are never reconciled. (d) Authorisation is also a **deployment** property: route groups are registered conditionally, so an unauthorised-looking 404 may simply mean "this gateway is not that kind of node". |
| **Toolbox coverage** | The gateway-profile fixture encodes (d): a test that needs a Central Bank route skips on a commercial-bank profile instead of failing. Role-based 403 provocation is **not** covered — it needs a second credential set. |
| **Recommended action** | Publish the role matrix in a machine-readable form (scopes or an `x-` extension) and reconcile the two role namespaces. Until then, treat the role matrix in the contract READMEs as the authority. |

### API6:2023 — Unrestricted Access to Sensitive Business Flows

| Attribute | Value |
|---|---|
| **Risk** | Double settlement, settle-after-refund, an unsafe timelock ordering, a swap that completes while leaving the payer short, or a corridor resumed without quorum. |
| **Applies to** | Both |
| **Verification** | Automated (live tier) |
| **CI automatable?** | Yes, against a real gateway |
| **What the platform does** | Several genuine multi-party controls: the **asymmetric circuit breaker** (1-of-N pause, 2-of-N resume), the **disclosure quorum** (2-of-N signatures, 72-hour expiry) before private Zeto state can be decrypted, **commit-reveal liquidity** requiring both Central Banks, and **pair propose/confirm** requiring the Central Bank of each side. |
| **Findings** | (a) **The HTLC timelock ordering — the initiator's lock must outlive the responder's — is enforced only by prose.** Nothing in the schema prevents an inverted, unsafe swap. (b) A cross-currency swap bridges in the **full `max_amount_in`**; the unspent buffer returns as a separate `RESIDUE` bridge position, and `residue_status = RETURN_FAILED` means **the payer has not been made whole even though the swap reads COMPLETED**. (c) `signResume` answers **200 with `state: RESUME_PENDING`** when execution fails — a 200 there does not mean trading resumed. (d) **No idempotency mechanism exists** — no `Idempotency-Key`, no `If-Match`/ETag; replay safety is per-endpoint and payload-keyed. (e) FX agreements have **no `EXPIRED` state** despite an expiry worker. |
| **Toolbox coverage** | **Strongest area.** The timelock ordering is asserted on the suite's own request data; the residue is asserted separately from `status == COMPLETED`; the reserve prelude, both bridge legs and hub reconciliation are asserted; replayed FX transitions are asserted to fail with 409 (the code the delivered gateway returns; the platform document's 412 is unreachable — see Toolbox/DIVERGENCES.md row A4). |
| **Recommended action** | Ask that the timelock ordering be enforced server-side rather than documented, and that `signResume`'s failure mode not share a status code with success. |

### API7:2023 — Server Side Request Forgery (SSRF)

| Attribute | Value |
|---|---|
| **Risk** | The gateway fetches a caller-supplied URL. |
| **Applies to** | N/A |
| **Verification** | N/A |
| **CI automatable?** | N/A |
| **What the platform does** | **No published operation accepts a URL.** Address-like inputs are on-chain addresses (`token_address`, `amm_address`) and PEM/CSR text carried as JSON string fields; the media type is `application/json` only. There is no callback or webhook mechanism. |
| **Recommended action** | Re-evaluate if a webhook or notification endpoint is ever added. |

### API8:2023 — Security Misconfiguration

| Attribute | Value |
|---|---|
| **Risk** | Missing transport security, permissive CORS, missing security headers, or a debug surface exposed in production. |
| **Applies to** | Both |
| **Verification** | Live deployment |
| **CI automatable?** | Partly |
| **What the platform does** | Nothing in either OpenAPI document describes CORS, HSTS, CSP or `X-Content-Type-Options`; the Scenario A `servers` block lists seven **plain-HTTP localhost** development URLs and Scenario B lists one. |
| **Findings** | (a) `COOKIE_SECURE` gates the `Secure` flag — see headline 2. (b) **Both gateways self-serve their full OpenAPI document at an unauthenticated `GET /openapi.yaml`** (scenario-a `router.go:38`, scenario-b `router.go:42`). Useful — the Toolbox uses it as a drift gate — but it is an unauthenticated disclosure of the entire attack surface and should be a deliberate deployment decision. (c) `501` responses distinguish "feature not configured in this deployment" from other failures across 10 operations, which leaks deployment topology to an unauthenticated-adjacent caller. |
| **Toolbox coverage** | Deployment-level; not assertable from a conformance client. The suite treats 501/502/503 as SKIP rather than failure precisely because they are configuration outcomes. |
| **Recommended action** | Add a "Deployment Security Requirements" section covering TLS, `COOKIE_SECURE=true`, CORS origin allow-listing and whether `/openapi.yaml` should be exposed in production. |

### API9:2023 — Improper Inventory Management

| Attribute | Value |
|---|---|
| **Risk** | Shadow endpoints that exist in the deployment but appear in no specification. |
| **Applies to** | Both |
| **Verification** | Spectral lint + router/spec diff |
| **CI automatable?** | Yes |
| **What the platform does** | Publishes 88 client-callable paths across the two gateways, and self-serves each spec for drift checking. |
| **Findings** | (a) **Five Scenario A routes are wired by the router but appear nowhere in its OpenAPI**: `GET /api/v1/statement`, `/api/v1/identities/*`, `/internal/v1/identities/*`, `POST /internal/v1/payments/pvp-legs`, `GET /internal/v1/payments/pvp-credits` (`router.go:149-158`, `240-243`). (b) **Fourteen `/api/v2` operations declare free-form `additionalProperties` objects**, including `swap/cross-currency` and `bridge/lock-mint`; the platform says so explicitly. (c) The internal surface is versioned inconsistently — `/internal/v1/*`, `/internal/v2/*` and an **unversioned** `/internal/amm/*`. (d) `ErrorCodeResponse` is declared in both specs and referenced by nothing. (e) Both specs declare `info.version: 2.3.0` for two **different** path sets, so the version string cannot tell a client which scenario it is talking to; it must probe. |
| **Toolbox coverage** | Spectral lints all three contracts in CI at `--fail-severity=error` (green today). Two further gates are **required by the realignment plan §5.7 and not yet on disk**: `tools/validate_artifact_paths.py`, which fails any mock or vector whose path is not a real contract path+method, and a `workflow_dispatch` job that diffs each gateway's self-served `/openapi.yaml` against the published contracts. Until they land, this row is partially covered. Nothing is modelled on the five undocumented routes. |
| **Recommended action** | Keep the drift gate. Ask that the five wired-but-undocumented routes be either documented or removed. |

### API10:2023 — Unsafe Consumption of APIs

| Attribute | Value |
|---|---|
| **Risk** | The gateway trusts data from an upstream it does not control. |
| **Applies to** | Implementations |
| **Verification** | Code review |
| **CI automatable?** | No |
| **What the platform does** | The gateway consumes Keycloak, Paladin, the Cacti relay, hub RPC, the compliance service and (on commercial-bank gateways) the Central Bank API by proxy. Failures surface as `502` with messages such as "Central Bank API unreachable" and "Hub RPC read failed (ONCHAIN_ERROR); the cause is logged, not returned". |
| **Findings** | The commercial-bank gateway is a **proxy, and that is invisible in the paths**: the same path is served locally on a Central Bank gateway and proxied on a commercial-bank one, where the proxy injects `requester_besu_address`, the CSR, the key and a proof-of-possession signature. What the client must send therefore differs by deployment with no signal in the contract. |
| **Toolbox coverage** | 502 is treated as a deployment outcome (SKIP), never as conformance evidence. The proxy asymmetry is documented in the contract READMEs. |
| **Recommended action** | Expose a capability endpoint so a client can discover whether it is talking to a proxy, and which optional subsystems (Pente, PKI, fiat adapter, swap history) are wired. |

---

## Summary matrix

| # | OWASP risk | Applies to | CI? | Platform control | Toolbox coverage | Priority |
|---|---|---|---|---|---|---|
| 1 | BOLA | Impl | No | Identity from JWT claim (strong) | None — needs 2 credentials | High |
| 2 | Broken authentication | Both | Partial | CookieAuth; **no CSRF** | 12 × 401 + both login flows | High |
| 3 | Object property auth | Both | Partial | Separate request/response types; no `readOnly` | Indirect | Medium |
| 4 | Resource consumption | Impl | No | **None** — no 429, no rate headers | Not assertable | Medium |
| 5 | Function level auth | Both | No | Real, but prose-only | Profile gate only | High |
| 6 | Sensitive business flows | Both | Yes (live) | Quorums, breaker, timelocks | **Strong** | Maintain |
| 7 | SSRF | N/A | N/A | No URL inputs | N/A | N/A |
| 8 | Security misconfiguration | Both | Partial | `COOKIE_SECURE`-dependent | Deployment-level | Medium |
| 9 | Inventory management | Both | Yes | Self-served spec | Spectral live; 2 gates pending | High |
| 10 | Unsafe consumption | Impl | No | 502 taxonomy | SKIP-on-502 | Low |

---

## PR review checklist (for contract changes)

- [ ] Every operation declares `security` explicitly — including `security: []` for a public one. **There is no document-level default to inherit.**
- [ ] No `Authorization: Bearer` is introduced outside the 12 Scenario B `/api/v2` `RequireAnyAuth` routes.
- [ ] **No CSRF header is invented.** The platform has none.
- [ ] Every 4xx/5xx references the shared error schema for its surface (`ErrorResponse` on `/api/v1`, `AmmErrorResponse` on `/api/v2`).
- [ ] Request and response schemas stay separate types; no server-owned field becomes settable.
- [ ] No real credential, key, certificate or internal endpoint appears in an example.
- [ ] Any shape not present in the platform's own OpenAPI carries `x-cbweb3-source: implementation-observed`.
- [ ] New endpoints ship at least one happy-path and one error conformance test, with the correct `mock_safe` / `live_only` tier.
- [ ] HTLC changes preserve the initiator-outlives-responder timelock ordering.
- [ ] Any documented `SHA-256(secret) = hashlock` pair is recomputed — by `tools/verify_hashlocks.py` once it lands (plan §5.7), and meanwhile by `test_hash_lock_is_the_sha256_of_the_secret` in the conformance suite.
