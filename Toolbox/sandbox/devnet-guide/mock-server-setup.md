# Mock Server Setup

How to run a local mock API server for the CBWeb3 PvP Settlement contract using Prism.

---

## What is Prism?

[Prism](https://stoplight.io/open-source/prism) is an open-source HTTP mock server that reads an OpenAPI spec and serves the examples defined in it as real HTTP responses. This lets you develop and test against the CBWeb3 API without any backend infrastructure.

---

## Quick start

From the repository root:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010
```

You should see output like:

```
[CLI] ...  info      POST       http://127.0.0.1:4010/fx/agreement
[CLI] ...  info      POST       http://127.0.0.1:4010/fx/agreement/{id}/accept
[CLI] ...  info      GET        http://127.0.0.1:4010/fx/agreement/{id}
[CLI] ...  info      POST       http://127.0.0.1:4010/htlc/lock
[CLI] ...  info      POST       http://127.0.0.1:4010/htlc/settle
[CLI] ...  info      POST       http://127.0.0.1:4010/htlc/refund
[CLI] ...  info      GET        http://127.0.0.1:4010/htlc/status
[CLI] ...  start     Prism is listening on http://127.0.0.1:4010
```

> **Note:** Prism serves the paths as defined in the OpenAPI spec (e.g., `/fx/agreement`), without the server base path `/api/v1`. When targeting a real implementation, include the base path in `CBWEB3_BASE_URL`.

The mock server is now running at `http://localhost:4010`.

---

## Test it

```bash
curl -s http://localhost:4010/fx/agreement \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token" \
  -d '{"sourceCurrency":"tCeBM-A","targetCurrency":"tCeBM-B","sourceAmount":"1000000","exchangeRate":"0.058","counterpartyAddress":"0xCB002_CountryB"}' \
  | jq .
```

You should get a `201` response with an `agreementId`.

---

## Configuration options

### Port

Change the port with `--port`:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 8080
```

### Dynamic mode

By default, Prism returns the examples defined in the OpenAPI spec (static mode). Use `--dynamic` to generate random responses that conform to the schemas:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010 --dynamic
```

> **Note:** Static mode is recommended for the tutorials, as it returns the exact values documented in the mocks.

### Verbose logging

Add `--verboseLevel debug` for detailed request/response logging:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010 --verboseLevel debug
```

---

## Limitations

- **No state management:** Prism is stateless. Each request is independent. Creating an agreement in step 1 doesn't affect step 2. The mock always returns the same example response.
- **No authentication validation:** Prism accepts any Bearer token. It does not validate JWTs.
- **No error simulation by default:** In static mode, Prism returns the first `2xx` example. To get error responses, use request headers: `Prefer: code=400` or `Prefer: code=404`.

### Getting error responses

```bash
# Force a 400 response
curl -s http://localhost:4010/fx/agreement \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token" \
  -H "Prefer: code=400" \
  -d '{}' \
  | jq .

# Force a 404 response
curl -s http://localhost:4010/fx/agreement/nonexistent \
  -X GET \
  -H "Authorization: Bearer test-token" \
  -H "Prefer: code=404" \
  | jq .
```

---

## Next steps

- Follow [Tutorial 1: PvP Settlement against Mock Server](../tutorials/01-pvp-settlement-mock.md)
- Learn about the [PvP flow architecture](flow-walkthrough.md)
