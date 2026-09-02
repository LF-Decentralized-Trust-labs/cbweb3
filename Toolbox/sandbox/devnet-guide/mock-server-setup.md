# Mock Server Setup

How to run local mock API servers for the CBWeb3 interface contracts using Prism.

---

## What is Prism?

[Prism](https://stoplight.io/open-source/prism) is an open-source HTTP mock server that
reads an OpenAPI document and serves the examples defined in it as real HTTP responses.
That lets you build and test a CBWeb3 client with no backend infrastructure at all.

---

## One instance per contract

The Toolbox publishes **three** self-contained contracts, and Prism mocks one document per
process. The convention — used by the tutorials, the conformance suite and CI — is:

| Port | Contract | Scenario |
|---|---|---|
| **4010** | `Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml` | A — FX agreement + HTLC |
| **4011** | `Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml` | B — bridge + Hub AMM |
| **4012** | `Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml` | shared authentication |

From the repository root:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml   --port 4010 &
npx @stoplight/prism-cli mock Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml   --port 4011 &
npx @stoplight/prism-cli mock Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml --port 4012 &
```

Start only what you need. Tutorial 01 needs 4010 (and 4012 if you want to see the login
flow); tutorial 03 needs 4011.

Starting the auth contract prints:

```
[CLI] ℹ  info      GET        http://127.0.0.1:4012/healthz
[CLI] ℹ  info      POST       http://127.0.0.1:4012/api/v1/auth/login
[CLI] ℹ  info      POST       http://127.0.0.1:4012/api/v1/auth/wallet/bind
[CLI] ℹ  info      POST       http://127.0.0.1:4012/api/v1/auth/refresh
[CLI] ℹ  info      POST       http://127.0.0.1:4012/api/v1/auth/logout
[CLI] ℹ  info      GET        http://127.0.0.1:4012/api/v1/auth/me
[CLI] ℹ  info      POST       http://127.0.0.1:4012/api/v1/auth/pki-login
[CLI] ℹ  info      POST       http://127.0.0.1:4012/api/v1/auth/client-secret/change
[CLI] ▶  start     Prism is listening on http://127.0.0.1:4012
```

The PvP contract prints 28 paths and the AMM contract 52; both are long, so the banner is
not reproduced here.

> **The paths are complete.** Every contract path carries its own `/api/v1` or `/api/v2`
> prefix, and the `servers:` entries are bare origins with no base path. Prism therefore
> serves exactly the path a real gateway serves. `CBWEB3_BASE_URL` is a bare origin too —
> do not append `/api/v1` to it.

---

## Authentication against a mock

The whole `/api/v1` surface is authenticated by an `access_token` **HttpOnly cookie**, not
by a bearer token. Prism validates only the **presence** of that cookie, never its value:

```bash
# Authenticated — Prism sees a cookie credential and serves the example
curl -s http://localhost:4010/api/v1/payments/escrows \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Cookie: access_token=SYNTHETIC_COOKIE_CI" \
  -d '{"amount":"5000000"}' | jq .
# → 201 {"escrow_id":"esc-a1b2c3d4","status":"PENDING"}

# No cookie — a genuine 401, and a useful negative assertion
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:4010/api/v1/payments/escrows \
  -X POST -H "Content-Type: application/json" -d '{"amount":"5000000"}'
# → 401
```

Twelve Scenario B operations and four auth operations are **public** and need no cookie at
all:

```bash
curl -s http://localhost:4011/api/v2/amm/hub-config | jq .
curl -s http://localhost:4012/healthz | jq .
```

---

## Configuration options

### Port

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 8080
```

If you move a port, move `CBWEB3_BASE_URL` with it.

### Dynamic mode

By default Prism returns the examples defined in the contract (static mode). `--dynamic`
generates random schema-valid values instead:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --dynamic
```

> Static mode is what the tutorials assume: it returns the exact documented values.

### Request validation

`--errors` makes Prism reject a request that does not satisfy the contract instead of
serving a best-effort example. This is the useful mode for checking that **your client**
sends what the contract requires:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --errors
```

### Verbose logging

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml --port 4010 --verboseLevel debug
```

---

## Limitations

- **No state.** Prism is stateless. Creating an FX agreement does not make it retrievable;
  each request is answered from the contract's examples. "Create → get → accept" sequences
  are meaningless against a mock, which is why the conformance suite splits its tests into
  `mock_safe` (shape and status only) and `live_only` (real state transitions).
- **No credential validation.** Prism checks that a cookie credential is present, never that
  it is valid. It does not verify signatures, roles or expiry.
- **No role or deployment awareness.** A real gateway serves the reserve-lifecycle *create*
  half only on a commercial-bank gateway and the *approve* half only on a Central Bank
  gateway; the mock serves both from one process.
- **One example per response.** In static mode Prism returns the first `2xx` example. Use
  the `Prefer` header to select something else.

### Selecting a specific response or example

```bash
# Force the 401 branch
curl -s http://localhost:4010/api/v1/htlc/settle \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Prefer: code=401" \
  -d '{"contract_id":"htlc-a1b2c3d4","secret":"hello"}' | jq .

# Force a specific named example — here, the PKI nonce branch of login,
# which is what a commercial bank actually receives
curl -s http://localhost:4012/api/v1/auth/login \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Prefer: example=pkiNonceFlow" \
  -d '{"clientId":"bank-a","clientSecret":"secret-a"}' | jq .
# → {"nonce":"9f2c7a1de4b83056af11c2d9e7b40a3c"}
```

---

## Next steps

- [Tutorial 1: Scenario A PvP settlement against a mock](../tutorials/01-pvp-settlement-mock.md)
- [Tutorial 3: Scenario B hub swap against a mock](../tutorials/03-hub-swap-mock.md)
- [Settlement flow walkthrough](flow-walkthrough.md) — what each call actually does
