# Scenario B — Postman collection

`cbweb3-scenario-b.postman_collection.json` is a Postman v2.1.0 collection generated
from the **delivered** Scenario B API Gateway surface (International Hub).

- **Source of truth:** `../../backend/services/api-gateway/docs/openapi.yaml`
  (OpenAPI 3.0.3, `info.version: 2.3.0`) — the same bytes the gateway embeds and serves
  at `GET /openapi.yaml` and `GET /docs`. See
  `docs/deliverables/D5-endpoint-specification.md`.
- **Surface:** 110 requests grouped into folders by OpenAPI tag, covering the `/api/v1`
  core surface plus the `/api/v2` AMM, bridge, liquidity (sovereign escrow-and-finalize),
  governance, oversight, currency/pair registry, hub reconciliation and internal relay
  operations.
- **Generated, not hand-edited.** Edit the OpenAPI spec and regenerate; direct edits to
  the JSON are overwritten on the next run and are not covered by the drift check.

## Regenerate

```bash
# from scenario-b/
bash apis/postman/generate.sh              # lint + regenerate + write
bash apis/postman/generate.sh --check      # lint + fail if the collection has drifted
bash apis/postman/generate.sh --lint-only  # lint the served spec only
```

`make scenario-b.gen-postman`, `scenario-b.check-postman` and `scenario-b.validate-openapi`
are thin aliases for the three modes; the script is the supported entry point.

The tool versions are pinned exactly in the script (`openapi-to-postmanv2`,
`@redocly/cli`) — a floating range silently rewrites the committed artifact. `npx`
downloads them from the npm registry on first use and reuses the local cache
afterwards, so the first run needs network access.

## What the drift check compares

The converter assigns a fresh UUID per item and fakes example response bodies from the
schemas; the generator strips both, so the committed file stays stable across runs. One
source of churn survives: for enum-typed fields with no explicit `example:`, the
converter picks a member at random. `--check` therefore compares the **request surface**
— folder, request name, method, path, query keys and auth type — which is what actually
changes when the spec gains or loses an endpoint, and is fully deterministic. CI runs
this check on every change to the spec or the collection
(`.github/workflows/api-artifacts.yml`).
