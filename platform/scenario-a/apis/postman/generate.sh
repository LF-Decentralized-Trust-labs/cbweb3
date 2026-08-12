#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Regenerate (or verify) the Scenario A Postman collection from the served OpenAPI spec.
#
# The served/embedded spec (backend/services/api-gateway/docs/openapi.yaml) is the
# single source of truth — the same bytes the gateway serves at GET /openapi.yaml and
# GET /docs (see docs/deliverables/D5-endpoint-specification.md).
#
# Modes:
#   ./generate.sh              lint the spec, regenerate the collection, write it
#   ./generate.sh --check      lint the spec, regenerate into a temp dir, fail on drift
#   ./generate.sh --lint-only  lint the spec only
#
# Both tools are pinned to an exact version: openapi-to-postmanv2 is a code generator,
# and a floating range silently changes the committed artifact (the previous `@6` had
# already moved 6.3.1 -> 6.3.3 between two reviews of this branch). npx fetches them
# from the npm registry on first use and serves them from the local cache afterwards,
# so the first run needs network access — it is not an air-gapped build step.
#
# Determinism: the converter assigns a fresh UUID to every item and fakes example
# response bodies from the schemas, neither of which is reproducible. Normalisation
# below drops both, which leaves one residual source of churn — for enum-typed fields
# without an explicit `example:` the converter picks a member at random. So --check
# does NOT byte-compare; it compares the request surface (folder, name, method, path,
# query keys, auth), which is exactly what drifts when the spec gains or loses an
# endpoint, and is fully deterministic.
set -euo pipefail

CONVERTER_VERSION="6.3.3"   # openapi-to-postmanv2 — exact pin, see above
REDOCLY_VERSION="1.34.5"    # @redocly/cli — exact pin

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC="$HERE/../../backend/services/api-gateway/docs/openapi.yaml"
OUT="$HERE/cbweb3-scenario-a.postman_collection.json"
NAME="CBWeb3 API Gateway — Scenario A — Enhanced Correspondent Banking"

MODE="generate"
case "${1:-}" in
  "") ;;
  --check) MODE="check" ;;
  --lint-only) MODE="lint" ;;
  *) echo "usage: $(basename "$0") [--check|--lint-only]" >&2; exit 2 ;;
esac

# Structural OpenAPI 3.0 compliance (parse failures, duplicated keys, broken $ref,
# undeclared security). Runs in every mode: a collection generated from a spec that
# does not lint is not worth diffing.
npx --yes "@redocly/cli@${REDOCLY_VERSION}" lint --extends minimal "$SPEC"
[[ "$MODE" == "lint" ]] && exit 0

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

npx --yes "openapi-to-postmanv2@${CONVERTER_VERSION}" \
  -s "$SPEC" \
  -o "$TMP/raw.json" \
  -p \
  -O folderStrategy=Tags,requestNameSource=Fallback

# Normalise: strip the per-item UUIDs and the faked example responses, and label the
# collection per scenario (both gateways share info.title).
python3 - "$TMP/raw.json" "$TMP/collection.json" "$NAME" <<'PY'
import json, sys

src, dst, name = sys.argv[1], sys.argv[2], sys.argv[3]
d = json.load(open(src))

def strip(items):
    for it in items:
        it.pop("id", None)
        it.pop("_postman_id", None)
        if "item" in it:
            strip(it["item"])
        else:
            it.pop("response", None)   # converter-faked, non-reproducible

strip(d["item"])
d["info"].pop("_postman_id", None)
d["info"]["name"] = name
json.dump(d, open(dst, "w"), indent=2)
open(dst, "a").write("\n")
PY

# Request surface: what a drift check has to notice, and nothing that varies per run.
signature() {
  python3 - "$1" <<'PY'
import json, sys

d = json.load(open(sys.argv[1]))
rows = []

def walk(items, folder=""):
    for it in items:
        if "item" in it:
            walk(it["item"], f"{folder}/{it.get('name','')}")
        else:
            r = it.get("request", {})
            u = r.get("url", {})
            path = "/" + "/".join(u.get("path", []))
            query = ",".join(sorted(q.get("key", "") for q in u.get("query", [])))
            auth = (r.get("auth") or {}).get("type", "")
            rows.append(f"{folder}|{it.get('name','')}|{r.get('method','')}|{path}|{query}|{auth}")

walk(d["item"])
print("\n".join(sorted(rows)))
PY
}

if [[ "$MODE" == "check" ]]; then
  if [[ ! -f "$OUT" ]]; then
    echo "FAIL: $OUT does not exist — run $(basename "$0") to generate it" >&2
    exit 1
  fi
  if diff -u <(signature "$OUT") <(signature "$TMP/collection.json") > "$TMP/drift.diff"; then
    echo "OK: the committed Scenario A collection matches the served spec."
  else
    echo "FAIL: the committed collection has drifted from the served OpenAPI spec." >&2
    echo "      Run 'bash apis/postman/generate.sh' and commit the result." >&2
    cat "$TMP/drift.diff" >&2
    exit 1
  fi
  exit 0
fi

cp "$TMP/collection.json" "$OUT"
echo "Regenerated $OUT -> $NAME"
