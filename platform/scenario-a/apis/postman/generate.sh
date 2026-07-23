#!/usr/bin/env bash
# Regenerate the Scenario A Postman collection from the served OpenAPI spec.
#
# The served/embedded spec (backend/services/api-gateway/docs/openapi.yaml) is
# the single source of truth (see docs/deliverables/D5-endpoint-specification.md).
# This script keeps the Postman collection in sync with the delivered API
# surface. No external CDN or hosted service is used — conversion runs locally
# via the pinned openapi-to-postmanv2 CLI.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC="$HERE/../../backend/services/api-gateway/docs/openapi.yaml"
OUT="$HERE/cbweb3-scenario-a.postman_collection.json"
NAME="CBWeb3 API Gateway — Scenario A — Enhanced Correspondent Banking"

npx --yes openapi-to-postmanv2@6 \
  -s "$SPEC" \
  -o "$OUT" \
  -p \
  -O folderStrategy=Tags,requestNameSource=Fallback

# Label the collection per scenario (both gateways share info.title).
python3 - "$OUT" "$NAME" <<'PY'
import json, sys
path, name = sys.argv[1], sys.argv[2]
d = json.load(open(path))
d["info"]["name"] = name
json.dump(d, open(path, "w"), indent=2)
open(path, "a").write("\n")
print("Regenerated", path, "->", name)
PY
