#!/usr/bin/env bash
# run-soak.sh — opt-in 12-hour soak driver (threshold 9). NOT part of perf-all / CI.
#
# Runs the k6 soak load generator AND the out-of-band metrics collector in parallel over the same
# window, capturing the leak/drift evidence README §5 requires (RSS, goroutines, pg connections,
# container restarts, AMM x*y=k before/after). Surfaced as `make scenario-b.perf-soak`.
#
# Knobs: DURATION (12h), API_GW_URL (3000), PAIR (BRL-USD), SOAK_METRICS_INTERVAL (300s).

set -u
PERF_DIR="$(cd "$(dirname "$0")" && pwd)"
PERF_SERVICE="perf-soak"
. "$PERF_DIR/lib/log.sh"
. "$PERF_DIR/lib/stack.sh"
. "$PERF_DIR/lib/auth.sh"
. "$PERF_DIR/lib/profile.sh"
. "$PERF_DIR/lib/seed.sh"
. "$PERF_DIR/lib/metrics.sh"

: "${API_GW_URL:=http://localhost:3000}"
: "${API_GW_CENTRAL_BANK_A_URL:=http://localhost:38080}"
: "${PAIR:=BRL-USD}"
: "${DURATION:=12h}"
: "${SOAK_METRICS_INTERVAL:=300}"

require_cmd k6; require_cmd curl; require_cmd awk
PERF_PAIR="$PAIR"

RESULTS_DIR="$PERF_DIR/results/soak-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"
log_info "soak results directory" dir="$RESULTS_DIR"

_dur_secs() { d="$1"; case "$d" in *h) echo $(( ${d%h}*3600 ));; *m) echo $(( ${d%m}*60 ));; *s) echo "${d%s}";; *) echo "$d";; esac; }
DUR_SECS="$(_dur_secs "$DURATION")"

stack_ensure_up
COMM_TOKEN="$(auth_commercial_bank_token)"
CB_TOKEN="$(auth_central_bank_token)"
profile_apply "$API_GW_CENTRAL_BANK_A_URL" "$CB_TOKEN"
seed_ensure_depth "$PAIR" "$COMM_TOKEN" 3 "$DUR_SECS" 1000

# Invariant: x*y=k BEFORE.
log_info "recording AMM invariant (before)"
metrics_snapshot "$RESULTS_DIR"

# Metrics collector in background for the full window.
( metrics_collect_loop "$RESULTS_DIR" "$SOAK_METRICS_INTERVAL" "$DUR_SECS" ) &
METRICS_PID=$!
log_info "metrics collector started" pid="$METRICS_PID"

log_info "starting 12h k6 soak load" duration="$DURATION"
API_GW_URL="$API_GW_URL" AUTH_TOKEN="$COMM_TOKEN" PAIR="$PAIR" DURATION="$DURATION" \
  k6 run --summary-export="$RESULTS_DIR/soak.summary.json" "$PERF_DIR/k6/soak.js" \
  > "$RESULTS_DIR/soak.log" 2>&1
SOAK_RC=$?

wait "$METRICS_PID" 2>/dev/null || true
log_info "recording AMM invariant (after)"
metrics_snapshot "$RESULTS_DIR"

log_info "soak complete — inspect metrics for RSS/connection trends + x*y=k drift" \
  rc="$SOAK_RC" metrics="$RESULTS_DIR/soak-metrics.jsonl" summary="$RESULTS_DIR/soak.summary.json"
