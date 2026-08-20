# Authentication & Session — Interface Contract

| | |
|---|---|
| **File** | `openapi_auth_v2.3.0.yaml` |
| **Scenario** | Both — Scenario A (single-ledger / PvP) and Scenario B (hub-and-spoke / AMM) |
| **Platform version** | CBWeb3 API Gateway **v2.3.0** |
| **Operations** | 8 (`/healthz` + 7 `/api/v1/auth/*`) |
| **Status** | Normative. Derived from the delivered platform gateway. |

## What this contract covers

The session-bootstrap surface every other CBWeb3 Toolbox contract depends on:

| Operation | Method + path |
|---|---|
| `healthCheck` | `GET /healthz` |
| `login` | `POST /api/v1/auth/login` |
| `walletBind` | `POST /api/v1/auth/wallet/bind` |
| `refreshToken` | `POST /api/v1/auth/refresh` |
| `logout` | `POST /api/v1/auth/logout` |
| `getMe` | `GET /api/v1/auth/me` |
| `pkiLogin` | `POST /api/v1/auth/pki-login` |
| `changeClientSecret` | `POST /api/v1/auth/client-secret/change` |

## Why it is published once, not twice

The authentication block of the platform's own OpenAPI document is **byte-identical**
between the Scenario A and Scenario B gateways — paths, schemas and the shared error
responses alike. Publishing it once guarantees the two scenario contracts cannot drift on
the one thing every integration starts with.

`operationId` values match the platform's own operation IDs exactly, so a client generated
from this contract and one generated from the gateway's live `/openapi.yaml` will name the
same methods.

## The one thing to get right: commercial banks log in twice

`POST /api/v1/auth/login` behaves in two completely different ways depending on the
caller's role, and **the server decides** — the client cannot choose.

* **Governance / supervisor / NOC** → tokens and cookies come back immediately.
* **Commercial bank / treasury** → the response is `{"nonce": "<hex>"}` with **no tokens
  and no cookies**. You must sign that nonce with the private key matching your
  Central-Bank-issued participant certificate and complete login at
  `POST /api/v1/auth/wallet/bind`.

A fixture that stops after `/auth/login` will never hold a session for the actor an
external integrator is most likely building for.

## Using it with Prism

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012

# Direct-flow login (Prism returns the first named example)
curl -s -X POST http://127.0.0.1:4012/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"clientId":"bank-a","clientSecret":"secret-a"}'

# Ask Prism for the PKI nonce example instead
curl -s -X POST http://127.0.0.1:4012/api/v1/auth/login \
  -H 'Content-Type: application/json' -H 'Prefer: example=pkiNonceFlow' \
  -d '{"clientId":"bank-a","clientSecret":"secret-a"}'

curl -s http://127.0.0.1:4012/api/v1/auth/me
```

Add `--errors` to make Prism enforce the contract (request validation) rather than
returning a best-effort example.

Validate before committing changes:

```bash
npx @stoplight/spectral-cli lint --ruleset .spectral.yml --fail-severity=error \
  Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml
```

## Known divergences from the platform's own OpenAPI document

These are places where the delivered gateway does **not** behave the way its published
document says. The contract documents observed behaviour and marks each one inline with a
`Spec divergence:` paragraph.

| # | Where | Platform document says | Delivered gateway does |
|---|---|---|---|
| 1 | `POST /auth/login` 200 | Body typed solely as `AuthResponse` | PKI flow returns `{"nonce": "<hex>"}`, which `AuthResponse` cannot represent. A strict validator built from the platform document rejects a *correct* commercial-bank login. Modelled here as a `oneOf`. |
| 2 | `POST /auth/login` | 400, 401 only | Can also return **503** `authentication service unavailable`; Scenario A and Scenario B classify identity-service failures differently, so the same bad input can give 401 on one and 503 on the other. |
| 3 | `POST /auth/wallet/bind` | 400, 401, 501 | Can also return **403** and **500**. |
| 4 | `POST /auth/refresh` | Body required, `refreshToken` required | Handler falls back to the `refresh_token` cookie; 400 only when neither is present. The contract mirrors the document here, and notes the fallback. |
| 5 | `GET /auth/me` | Single `MeResponse` | Scenario A substitutes the gateway's `BANK_CODE` env var into `bankId` when the claim is absent; Scenario B does not. A test asserting `bankId` absence passes on B and fails on A. |

## Deliberately out of scope

* **Internal relay endpoints** (`/internal/*`) — authenticated by a relay credential, not a
  user session, and never exposed to community integrators.
* **Onboarding, compliance and governance endpoints**, which also live on the
  `/api/v1` surface but belong to other Toolbox domains.
* **CSRF and idempotency headers.** Neither exists in the delivered gateway. The contract
  deliberately declares no CSRF token and no `Idempotency-Key`; adding either to a client
  would be inventing a mechanism the platform does not implement. `X-Correlation-Id` is
  server-generated and overrides any client value, so it cannot be used as an idempotency
  key.
