# Prism Mock Server Configuration

Reference configurations for running Prism against the CBWeb3 Toolbox contracts.

---

## Default (recommended for tutorials)

```bash
npx @stoplight/prism-cli mock \
  Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml \
  --port 4010
```

This serves the OpenAPI examples as static responses. Every request to a given endpoint returns the same example response, regardless of the request body.

---

## Dynamic mode (random schema-valid responses)

```bash
npx @stoplight/prism-cli mock \
  Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml \
  --port 4010 \
  --dynamic
```

Generates random values that conform to the response schemas. Useful for stress-testing client-side parsing, but not suitable for the tutorials (which expect specific values).

---

## With CORS (for browser-based frontends)

```bash
npx @stoplight/prism-cli mock \
  Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml \
  --port 4010 \
  --cors
```

---

## Debug mode (verbose logging)

```bash
npx @stoplight/prism-cli mock \
  Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml \
  --port 4010 \
  --verboseLevel debug
```

Shows full request/response details in the terminal. Useful for troubleshooting.

---

## Running alongside conformance tests

The CI pipeline and conformance tests expect Prism on port `4010` (the default `CBWEB3_BASE_URL`).

**Terminal 1:**
```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010
```

**Terminal 2:**
```bash
cd Toolbox/conformance
pytest -m happy_path -v
```

---

## Environment variables

If you use the `.env.example` file, the conformance tests will automatically pick up `CBWEB3_BASE_URL`:

```bash
source Toolbox/sandbox/sample-configs/.env.example
cd Toolbox/conformance
pytest -v
```
