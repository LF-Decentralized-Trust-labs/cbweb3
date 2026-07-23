# Scenario B — Postman collection

`cbweb3-scenario-b.postman_collection.json` is a Postman v2.1.0 collection that
mirrors the **delivered** Scenario B API Gateway surface (International Hub).

- **Source of truth:** `../../backend/services/api-gateway/docs/openapi.yaml`
  (OpenAPI 3.0.3, `info.version: 2.3.0`), the same spec the gateway embeds and
  serves at `GET /openapi.yaml` and `GET /docs`. See
  `docs/deliverables/D5-endpoint-specification.md`.
- **Surface:** 97 requests grouped into folders by OpenAPI tag, covering the
  `/api/v1` core surface plus the `/api/v2` AMM, bridge, liquidity, governance,
  oversight, currency/pair registry and internal relay operations.
- **Version consistency:** the collection is derived directly from the served
  v2.3.0 spec, so it never drifts from what the gateway actually exposes.

## Regenerate

```bash
# from scenario-b/
make scenario-b.gen-postman
# or directly:
bash apis/postman/generate.sh
```

The generator runs fully offline via the pinned `openapi-to-postmanv2` CLI (no
external CDN or hosted conversion service). Re-run it after any change to the
served OpenAPI spec.
