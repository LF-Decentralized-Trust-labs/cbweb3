# `auth/` — obtaining a CBWeb3 session

Contract: [`../../contracts/auth/openapi_auth_v2.3.0.yaml`](../../contracts/auth/openapi_auth_v2.3.0.yaml)
· Scenario: **shared** — the authentication block is byte-identical between the Scenario A and
Scenario B gateways.

Nothing else in this kit works until one of these flows has produced an `access_token` cookie.

## The flow, in order

| # | File | Call | What it establishes |
|---|---|---|---|
| 1 | [`01_login_direct.json`](01_login_direct.json) | `POST /api/v1/auth/login` → 200 | **Direct flow.** A BCCR governance operator logs in and is done: tokens in the body, `Set-Cookie` installs `access_token` and `refresh_token`. |
| 2 | [`02_login_pki_nonce.json`](02_login_pki_nonce.json) | `POST /api/v1/auth/login` → 200 | **PKI flow, step 1.** The *same endpoint*, called by a commercial bank, answers `{"nonce": "<hex>"}` — no tokens, **no cookies**. Not authenticated yet. |
| 3 | [`03_wallet_bind.json`](03_wallet_bind.json) | `POST /api/v1/auth/wallet/bind` → 200 | **PKI flow, step 2.** The bank signs the nonce with the private key matching its CB-issued participant certificate; only now are the cookies set. |
| 4 | [`04_me.json`](04_me.json) | `GET /api/v1/auth/me` → 200 | The claims the gateway derived — above all `bankId`, the identity every write is attributed to. |
| 5 | [`05_error_no_cookie.json`](05_error_no_cookie.json) | `GET /api/v1/auth/me` → 401 | The negative assertion: no cookie, no session. |

## The three things that catch people out

**One endpoint, two behaviours, and the client does not choose.** `POST /api/v1/auth/login`
answers with tokens for `ROLE_GOVERNANCE`, `ROLE_SUPERVISOR`, `ROLE_NOC` and
`ROLE_GOVERNANCE_OFFICER`, and with a bare nonce for `ROLE_COMMERCIAL_BANK` and `ROLE_TREASURY`.
The server decides from the participant record. **Commercial banks — the actors an external
integrator is most likely building for — always perform the two-step dance**, so a fixture or
client that stops after `/auth/login` never holds a session. Fixtures 1 and 2 are the same
request shape against the same path with entirely different results; that is not a mistake.

**The platform's own spec cannot express the nonce response.** It types the 200 on `/auth/login`
solely as `AuthResponse`, which has no `nonce` property and marks `accessToken`, `expiresIn` and
`tokenType` required — so a strict response validator built from the platform document **fails a
perfectly correct commercial-bank login**. The contract models both shapes as a documented
`oneOf`, with the nonce shape marked `x-cbweb3-source: implementation-observed`.

**There is no CSRF token and no idempotency key.** An exhaustive search of the delivered gateway
finds no `X-CSRF-Token`, no CSRF endpoint, no double-submit cookie and no `Origin`/`Referer`
check; `SameSite=Strict` is the sole mitigation. Equally there is no `Idempotency-Key` and no
`If-Match`/`ETag` — and `X-Correlation-Id`, which every response here carries, is
*server*-generated and overrides any client value, so it cannot serve as one.

## Not mockable here

`GET /healthz` is the only endpoint on the whole platform callable with no credentials at all,
and every integration harness should poll it first. It has no fixture because
`mock.schema.json` constrains `request.path` to `^/api/v\d+/` and `/healthz` carries no version
prefix. It answers `{"status": "ok"}`.

Undocumented status codes an implementer should still expect, described in the contract in prose
because the platform declares none of them: **503** from `/auth/login` when the identity service
is unreachable (and note the two scenarios classify identity failures differently, so identical
bad credentials can yield 401 on one and 503 on the other), and **403** / **500** from
`/auth/wallet/bind`.

## Synthetic material

`accessToken`, `refreshToken`, the cookie value, `nonce_signature_hex` and `cert_pem` are all
literal placeholders. They are not JWTs, not a DER signature and not a certificate — this kit
ships no key material of any kind, and the tokens are deliberately not even well-formed so that
nobody can mistake one for a working credential.
