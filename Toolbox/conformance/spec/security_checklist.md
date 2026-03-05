# Security Checklist — CBWeb3 Toolbox (OWASP API Top 10)

> Version: 0.1.0 | Effective: 2026-03-05
> Based on: [OWASP API Security Top 10 (2023)](https://owasp.org/API-Security/)

---

## Purpose

This checklist adapts the OWASP API Security Top 10 to CBWeb3's context (tokenized Central Bank Money, HTLC-based settlement, privacy-preserving transfers). It classifies each risk by:

- **Applicability:** Does this risk apply to the Toolbox artifacts, to implementations, or both?
- **Verification method:** Can it be checked automatically (CI), manually (PR review), or only in a live deployment?
- **Current coverage:** What the Toolbox already provides for this risk

---

## Checklist

### API1:2023 — Broken Object Level Authorization (BOLA)

| Attribute | Value |
|-----------|-------|
| **Risk** | An attacker accesses another participant's FX agreement or HTLC contract by manipulating the `agreementId` or `contractId` parameter |
| **Applies to** | Implementations (not Toolbox artifacts directly) |
| **Verification** | Manual / deployment testing |
| **CI automatable?** | No |
| **Current Toolbox coverage** | Test vectors use fixed IDs. Implementations MUST validate that the authenticated caller is a party to the agreement/contract |
| **Recommended action** | Add BOLA-specific test vectors in v0.2.0 (e.g., attempt to access agreement as non-participant → expect 403) |

### API2:2023 — Broken Authentication

| Attribute | Value |
|-----------|-------|
| **Risk** | Missing or weak JWT validation allows unauthorized access to settlement endpoints |
| **Applies to** | Implementations |
| **Verification** | Manual / deployment testing |
| **CI automatable?** | Partially — Spectral can verify that `securitySchemes` are defined and applied globally |
| **Current Toolbox coverage** | OpenAPI spec defines `BearerAuth` (JWT) as a global security requirement. Mocks use synthetic tokens (`SYNTHETIC_TOKEN_CB001`) |
| **Recommended action** | Add conformance tests for: missing token → 401, expired token → 401, malformed token → 401 |

### API3:2023 — Broken Object Property Level Authorization

| Attribute | Value |
|-----------|-------|
| **Risk** | Caller modifies read-only fields (e.g., `status`, `transactionHash`, `blockNumber`) via request body |
| **Applies to** | Both (contracts should define readOnly; implementations should enforce it) |
| **Verification** | PR review + Spectral rule |
| **CI automatable?** | Yes — custom Spectral rule to verify readOnly annotations on response-only fields |
| **Current Toolbox coverage** | Response schemas (`FxAgreementResponse`, `HtlcResponse`) contain fields that should be readOnly but are not explicitly annotated |
| **Recommended action** | Add `readOnly: true` to `status`, `transactionHash`, `blockNumber`, `createdAt`, `acceptedAt` in the OpenAPI spec |

### API4:2023 — Unrestricted Resource Consumption

| Attribute | Value |
|-----------|-------|
| **Risk** | No rate limiting allows an attacker to flood settlement endpoints, causing denial of service or excessive gas consumption |
| **Applies to** | Implementations |
| **Verification** | Manual / deployment testing |
| **CI automatable?** | Partially — Spectral rule to check for `X-RateLimit-*` response headers in the spec |
| **Current Toolbox coverage** | Not addressed. The OpenAPI spec does not define rate limit headers |
| **Recommended action** | Document recommended rate limits in the contract README. Add `X-RateLimit-Limit` and `X-RateLimit-Remaining` headers to response definitions in v0.2.0 |

### API5:2023 — Broken Function Level Authorization

| Attribute | Value |
|-----------|-------|
| **Risk** | The initiator calls `/fx/agreement/{id}/accept` (should be counterparty-only) or the counterparty calls `/htlc/settle` before the initiator reveals the secret |
| **Applies to** | Implementations |
| **Verification** | Manual / deployment testing |
| **CI automatable?** | No |
| **Current Toolbox coverage** | Test vector `fx-hp-02` notes that "only the counterpartyAddress should be allowed to accept." No automated check exists |
| **Recommended action** | Add role-based test vectors: initiator-attempts-accept → 403, non-participant-attempts-lock → 403 |

### API6:2023 — Unrestricted Access to Sensitive Business Flows

| Attribute | Value |
|-----------|-------|
| **Risk** | Double-settlement, settle-after-refund, or lock without accepted agreement bypass business rules |
| **Applies to** | Both (contracts define state machine; implementations enforce it) |
| **Verification** | Automated via test vectors |
| **CI automatable?** | Yes (Level 2 and Level 3 conformance tests) |
| **Current Toolbox coverage** | **Good.** Test vectors cover: settle expired HTLC (`htlc-err-02`), refund before expiry (`htlc-err-03`), refund already settled (`htlc-err-04`), lock on non-accepted agreement (`htlc-edge-01`), accept already-accepted agreement (`fx-err-02`) |
| **Recommended action** | Maintain coverage as new endpoints are added |

### API7:2023 — Server Side Request Forgery (SSRF)

| Attribute | Value |
|-----------|-------|
| **Risk** | The API processes a user-supplied URL and makes a server-side request to it |
| **Applies to** | N/A for Toolbox v0.1 |
| **Verification** | N/A |
| **CI automatable?** | N/A |
| **Current Toolbox coverage** | No endpoints accept URLs as input. No callback/webhook mechanism exists |
| **Recommended action** | Re-evaluate if webhook endpoints are added in v0.2.0 |

### API8:2023 — Security Misconfiguration

| Attribute | Value |
|-----------|-------|
| **Risk** | Missing CORS policy, HTTPS enforcement, security headers, or overly permissive configurations |
| **Applies to** | Both (contracts should document; implementations should enforce) |
| **Verification** | PR review + Spectral |
| **CI automatable?** | Partially — Spectral can verify security schemes are defined |
| **Current Toolbox coverage** | OpenAPI spec defines HTTPS server URL and BearerAuth security scheme. No CORS or security header documentation |
| **Recommended action** | Add a "Deployment Security Requirements" section to the contract README covering CORS, CSP, HSTS, and X-Content-Type-Options |

### API9:2023 — Improper Inventory Management

| Attribute | Value |
|-----------|-------|
| **Risk** | "Shadow APIs" — endpoints that exist in the deployment but are not documented in the OpenAPI spec |
| **Applies to** | Both |
| **Verification** | PR review + Spectral lint |
| **CI automatable?** | Yes — Spectral lint ensures the spec is complete and valid |
| **Current Toolbox coverage** | **Good.** The PvP contract documents all 7 endpoints with full schemas. Spectral CI gate ensures the spec stays valid |
| **Recommended action** | Maintain Spectral lint as a mandatory CI gate. Add a custom rule to flag undocumented response codes |

### API10:2023 — Unsafe Consumption of APIs

| Attribute | Value |
|-----------|-------|
| **Risk** | If CBWeb3 consumes external APIs (e.g., FX rate feeds, KYC providers), it must validate inputs from those external sources |
| **Applies to** | Implementations |
| **Verification** | Manual / code review |
| **CI automatable?** | No |
| **Current Toolbox coverage** | Not applicable to the Toolbox itself. The PvP contract is self-contained |
| **Recommended action** | Add guidance in the conformance spec for implementations that integrate external data sources |

---

## Summary matrix

| # | OWASP Risk | Applies to | CI? | Current coverage | Priority |
|---|-----------|-----------|-----|-----------------|----------|
| 1 | BOLA | Impl | No | Low | High |
| 2 | Broken Auth | Impl | Partial | Medium | High |
| 3 | Object Property Auth | Both | Yes | Low | Medium |
| 4 | Resource Consumption | Impl | Partial | None | Medium |
| 5 | Function Level Auth | Impl | No | Low | High |
| 6 | Sensitive Business Flows | Both | Yes | **High** | Maintain |
| 7 | SSRF | N/A | N/A | N/A | N/A |
| 8 | Security Misconfiguration | Both | Partial | Low | Medium |
| 9 | Inventory Management | Both | Yes | **High** | Maintain |
| 10 | Unsafe Consumption | Impl | No | N/A | Low |

---

## PR review checklist (for contract changes)

When reviewing a PR that modifies interface contracts, verify:

- [ ] All endpoints define `security` requirements (or explicitly document why they don't)
- [ ] All error responses (4xx, 5xx) include `$ref: "#/components/schemas/Error"`
- [ ] Response-only fields are annotated with `readOnly: true`
- [ ] No real credentials, keys, or internal endpoints in examples
- [ ] State machine transitions are documented (which status → which status)
- [ ] New endpoints have corresponding test vectors covering at least 1 happy path and 1 error case
- [ ] HTLC-related endpoints document the hashLock/timeLock security assumptions
