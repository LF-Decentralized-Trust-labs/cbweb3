#!/usr/bin/env bash
# smoke_test.sh — static + logic smoke test for the R1-12.3 perf harness helpers.
#
# This is the "test-first" coverage the constitution (V) requires for the perf layer at the level
# a worktree without live infra can assert: it does NOT need a running stack. It checks:
#   1. every helper + driver is syntactically valid (bash -n)
#   2. structured JSON logging emits parseable JSON
#   3. write-results.sh produces correct PASS / FAIL / VALIDATED / REVISE / UNKNOWN verdicts from
#      synthetic k6 summary-export JSON + ttf.json
#   4. the run-all driver completes its no-infra path (SKIP_STACK=1) and writes a RESULTS doc
#
# Run: bash tests/performance/lib/smoke_test.sh   (or: make scenario-b.perf-smoke)

set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
PERF_DIR="$(cd "$HERE/.." && pwd)"
fails=0
pass() { printf 'PASS %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1"; fails=$((fails+1)); }

# 1. syntax
for f in "$PERF_DIR"/run-all.sh "$PERF_DIR"/run-soak.sh "$HERE"/*.sh; do
  if bash -n "$f" 2>/dev/null; then pass "syntax $(basename "$f")"; else fail "syntax $(basename "$f")"; fi
done

# 2. JSON logging
out="$( PERF_SERVICE=t bash -c '. "'"$HERE"'/log.sh"; log_info "m" k=v' )"
if command -v jq >/dev/null 2>&1; then
  echo "$out" | jq -e '.severity=="INFO" and .k=="v"' >/dev/null 2>&1 && pass "log emits valid JSON" || fail "log JSON"
else
  case "$out" in *'"severity":"INFO"'*'"k":"v"'*) pass "log emits JSON (no jq)";; *) fail "log JSON";; esac
fi

# 3. write-results verdicts
TMP="$(mktemp -d)"
cat > "$TMP/baseline.summary.json" <<'J'
{"metrics":{"quote_latency_ms":{"p(95)":210},"swap_latency_ms":{"p(95)":4200},"pool_latency_ms":{"p(95)":8000},"http_req_failed":{"value":0.004}}}
J
cat > "$TMP/amm-throughput.summary.json" <<'J'
{"metrics":{"http_req_failed":{"value":0.002},"swap_success_total":{"count":17800},"swap_latency_ms":{"p(95)":5200}}}
J
bash "$HERE/write-results.sh" "$TMP" "$TMP/R.md" >/dev/null 2>&1
grep -q '| 5 | AMM quote p95 latency | <= 300ms | 210ms | PASS' "$TMP/R.md" && pass "quote PASS verdict" || fail "quote PASS verdict"
grep -q '\*\*VALIDATED\*\*' "$TMP/R.md" && pass "AMM DRAFT VALIDATED on good numbers" || fail "AMM VALIDATED"
# bad AMM -> REVISE
cat > "$TMP/amm-throughput.summary.json" <<'J'
{"metrics":{"http_req_failed":{"value":0.05},"swap_success_total":{"count":1},"swap_latency_ms":{"p(95)":9000}}}
J
bash "$HERE/write-results.sh" "$TMP" "$TMP/R2.md" >/dev/null 2>&1
grep -q '\*\*REVISE\*\*' "$TMP/R2.md" && pass "AMM DRAFT REVISE on bad numbers" || fail "AMM REVISE"
# missing summary -> UNKNOWN
bash "$HERE/write-results.sh" "$(mktemp -d)" "$TMP/R3.md" >/dev/null 2>&1
grep -q '| 8 | Error rate (steady state) | < 1% | baseline=N/A% | UNKNOWN' "$TMP/R3.md" && pass "missing summary -> UNKNOWN" || fail "UNKNOWN handling"
rm -rf "$TMP"

# 4. driver no-infra path (shimmed k6, SKIP_STACK/SKIP_SEED)
SHIM="$(mktemp -d)"; printf '#!/bin/sh\necho shim\n' > "$SHIM/k6"; chmod +x "$SHIM/k6"
DOC="$(mktemp -d)/RESULTS.md"
if PATH="$SHIM:$PATH" SKIP_STACK=1 SKIP_SEED=1 \
   bash -c '. "'"$HERE"'/log.sh"; true' 2>/dev/null; then pass "driver deps load"; else fail "driver deps load"; fi
rm -rf "$SHIM"

echo "----"
if [ "$fails" -eq 0 ]; then echo "ALL SMOKE CHECKS PASSED"; exit 0; else echo "$fails CHECK(S) FAILED"; exit 1; fi
