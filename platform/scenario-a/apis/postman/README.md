# Scenario A — Postman collection

`cbweb3-scenario-a.postman_collection.json` is a Postman v2.1.0 collection that
mirrors the **delivered** Scenario A API Gateway surface (Enhanced
Correspondent Banking).

- **Source of truth:** `../../backend/services/api-gateway/docs/openapi.yaml`
  (OpenAPI 3.0.3, `info.version: 2.3.0`), the same spec the gateway embeds and
  serves at `GET /openapi.yaml` and `GET /docs`. See
  `docs/deliverables/D5-endpoint-specification.md`.
- **Surface:** 83 requests grouped into folders by OpenAPI tag.
- **Version consistency:** the collection is derived directly from the served
  v2.3.0 spec, so it never drifts from what the gateway actually exposes.

## Regenerate

```bash
# from scenario-a/
bash apis/postman/generate.sh
```

The generator runs fully offline via the pinned `openapi-to-postmanv2` CLI (no
external CDN or hosted conversion service). Re-run it after any change to the
served OpenAPI spec.
