# Authentication and Session — Test Vectors

**10 vectors** (6 happy-path, 4 error) against
[`../../contracts/auth/openapi_auth_v2.3.0.yaml`](../../contracts/auth/openapi_auth_v2.3.0.yaml).

This contract is **shared by both scenarios** — verified byte-identical between the Scenario A
and Scenario B gateway specs. Every integration starts here, so getting it right removes the
only drift risk on the part every implementer touches first.

## What the group covers

| Vector | Flow |
|---|---|
| `auth-hp-01` | Direct login → tokens + `Set-Cookie` |
| `auth-hp-02` | PKI login step 1 → **nonce only**, no tokens, no cookies |
| `auth-hp-03` | PKI login step 2 → `wallet/bind` → cookies issued |
| `auth-hp-04` | `GET /auth/me` → roles and `bankId` |
| `auth-hp-05` | Refresh with an **empty body**, falling back to the cookie |
| `auth-hp-06` | Logout → both cookies expired |
| `auth-err-01` | No cookie → `401` on a guarded route |
| `auth-err-02` | Login without `clientSecret` → `400`, not `401` |
| `auth-err-03` | Wallet bind with a signature that does not verify → `401` |
| `auth-err-04` | Refresh with neither body token nor cookie → `400` |

## The three things that surprise implementers

**1. There is no `Authorization: Bearer` on `/api/v1`.** The credential is an HttpOnly cookie
named `access_token`, `SameSite=Strict`, `Path=/`, with `Secure` following the gateway's
`COOKIE_SECURE` variable. Use a session-scoped HTTP client with a persistent cookie jar.
`BearerAuth` exists only in the Scenario B contract, only as an alternative on twelve `/api/v2`
routes, and **not** on the two cross-currency swap endpoints.

**2. `POST /api/v1/auth/login` behaves two completely different ways and the server decides.**
A governance, supervisor or NOC participant gets tokens and cookies. A commercial bank or
treasury participant gets `{"nonce": "<hex>"}` and nothing else, and must sign the nonce with
its P-256 participant key and call `POST /api/v1/auth/wallet/bind`. The client cannot request a
flow; it must handle both responses from the same call.

**3. There is no CSRF mechanism anywhere.** No `X-CSRF-Token`, no double-submit cookie, no
synchroniser token, no Origin or Referer check — an exhaustive grep of both platform specs and
the gateway source finds nothing. `SameSite=Strict` is the sole mitigation. These vectors send
no CSRF header, and the absence is recorded as a finding in
`../../conformance/spec/security_checklist.md` rather than papered over.

## Load-bearing upstream defect, mirrored not fixed

`auth-hp-02` exercises the PKI nonce response. The platform types the `200` on `/auth/login` as
`AuthResponse`, whose required properties are `accessToken`, `expiresIn` and `tokenType` and
which has **no `nonce` property**. A strict OpenAPI response validator therefore fails a
perfectly correct PKI login. The contract models the nonce branch as a documented `oneOf`
carrying `x-cbweb3-source: implementation-observed`. Do not "fix" it by relaxing the assertion —
the point is that the defect is visible.

## Gaps recorded, not asserted

* **`GET /healthz` has no vector.** `vector.schema.json` constrains `input.path` to
  `^/api/v\d+/`, and `/healthz` is unprefixed — the only endpoint on the entire platform that
  requires no credential on both scenarios. It is covered in the conformance suite instead.
* **Undeclared status codes.** The auth contract documents `503`, `403` and `500` in prose only
  on some operations. A strict response validator will fail on an auth `503`. No vector asserts
  one, because none is declared.
* **`POST /api/v1/auth/pki-login` and `POST /api/v1/auth/client-secret/change` have no vectors.**
  Both are Commercial-Bank-gateway proxy operations whose preconditions (a participant
  certificate on disk from a completed onboarding, and knowledge of a current secret) cannot be
  established from the published surface alone. `pki-login`'s `412` is the *only* `412` the
  Scenario A gateway can actually emit — see `../pvp/README.md`, defect FX-409.
