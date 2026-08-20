# Prism Mock Server Configuration

Reference configurations for running [Prism](https://stoplight.io/open-source/prism)
against the CBWeb3 Toolbox contracts.

The Toolbox publishes **three** contracts and Prism mocks **one document per process**, so
a full sandbox is three instances. The port convention below is used by the tutorials, the
conformance suite, `.env.example` and CI — keep it if you can.

| Port | Contract | Scenario | Paths / operations |
|---|---|---|---|
| **4010** | `Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml` | A — FX agreement + HTLC | 28 / 32 |
| **4011** | `Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml` | B — bridge + Hub AMM | 52 / 59 |
| **4012** | `Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml` | shared authentication | 8 / 8 |

> **Paths are complete.** Every contract path carries its own `/api/v1` or `/api/v2`
> prefix and the `servers:` entries are bare origins, so Prism serves exactly the path a
> real gateway serves. `CBWEB3_BASE_URL` is a bare origin too — never append `/api/v1`.

---

## Default (recommended for tutorials)

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010 &
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011 &
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012 &
```

Start only what you need: tutorial 01 needs 4010, tutorial 03 needs 4011, and 4012 is only
needed to see the login flow.

This serves the OpenAPI examples as static responses. Every request to a given operation
returns the same example, regardless of the request body.

---

## Authenticating against a mock

The whole `/api/v1` surface is guarded by an `access_token` **HttpOnly cookie**, not by a
bearer token. Prism validates only the **presence** of the cookie credential, never its
value, so any string works:

```bash
curl -s http://localhost:4010/api/v1/token/balance \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI' | jq .
```

Omit the cookie and Prism returns a genuine `401` — a useful negative assertion.

Sixteen operations are public and need no cookie at all: four in the auth contract
(`GET /healthz`, `login`, `wallet/bind`, `refresh`) and twelve in the Scenario B discovery
surface.

---

## Dynamic mode (random schema-valid responses)

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --dynamic
```

Generates random values that conform to the response schemas. Useful for stress-testing
client-side parsing, but **not** suitable for the tutorials, which expect the documented
example values.

---

## Request validation (`--errors`)

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --errors
```

Makes Prism reject a request that does not satisfy the contract instead of serving a
best-effort example. This is the mode that checks **your client** sends what the contract
requires — the closest a mock gets to a compliance gate.

---

## Selecting a specific response or example

Static mode returns the first `2xx` example. Use the `Prefer` header to pick another:

```bash
# Force the 409 invalid-state-transition branch (the gateway's real code; the
# platform document's 412 is unreachable — see Toolbox/DIVERGENCES.md row A4)
curl -s -H 'Prefer: code=409' \
  -H 'Cookie: access_token=SYNTHETIC_COOKIE_CI' \
  -X POST http://localhost:4010/api/v1/payments/fx/agreements/trade-a1b2c3d4/accept | jq .

# Force the PKI nonce branch of login — what a commercial bank actually receives
curl -s -H 'Prefer: example=pkiNonceFlow' \
  -H 'Content-Type: application/json' \
  -X POST http://localhost:4012/api/v1/auth/login \
  -d '{"clientId":"bank-a","clientSecret":"secret-a"}' | jq .
# → {"nonce":"9f2c7a1de4b83056af11c2d9e7b40a3c"}
```

`POST /api/v1/auth/login` is the one operation where the response branch genuinely matters:
the same status `200` carries either an `AuthResponse` (direct flow) or a bare `{"nonce": …}`
(PKI flow).

---

## With CORS (for browser-based frontends)

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --cors
```

Note that an `HttpOnly` cookie is set by the gateway, not by JavaScript. A browser client
must use `credentials: 'include'` and be served from an origin the gateway accepts.

---

## Debug mode (verbose logging)

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --verboseLevel debug
```

Shows full request/response details in the terminal, including why a request was rejected.

---

## Running alongside the conformance tests

**Terminal 1** — start the three mocks:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010 &
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011 &
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012 &
```

**Terminal 2** — run the tier that a stateless mock can actually satisfy:

```bash
cd Toolbox/conformance
CBWEB3_AUTH_MODE=mock CBWEB3_PROFILE=mock \
  pytest tests/ -m "mock_safe and happy_path" -v
```

`mock_safe` tests make a single request and assert status and shape. `live_only` tests
assert state transitions, balance deltas and residue reconciliation; they are skipped
automatically in `mock` mode because Prism has no state. This is the exact command CI runs.

---

## Environment variables

`.env.example` holds the same conventions:

```bash
set -a; source Toolbox/sandbox/sample-configs/.env.example; set +a
cd Toolbox/conformance
pytest tests/ -m "mock_safe and happy_path" -v
```

`CBWEB3_AUTH_TOKEN` no longer exists anywhere in this repository. The credential is a
cookie, and the conformance suite carries it in a `requests.Session` jar.

---

## Limitations worth knowing before you trust a green run

- **No state.** Create → get → accept sequences are meaningless against Prism.
- **No credential validation.** Presence of a cookie is checked; validity, roles and expiry
  are not.
- **No deployment awareness.** A real gateway registers the *create* half of the reserve
  lifecycle only on a commercial-bank gateway and the *approve* half only on a Central Bank
  gateway. One Prism process serves both.

See [../devnet-guide/mock-server-setup.md](../devnet-guide/mock-server-setup.md) for the
longer discussion.
