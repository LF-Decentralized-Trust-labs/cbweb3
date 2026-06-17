#!/usr/bin/env bash
# run-all.sh — zero-config driver for the full R1-12.3 performance suite (excluding the 12h soak).
#
# One command, walk away. Surfaced as `make scenario-b.perf-all`. With NO arguments and NO manual
# steps it:
#   1. ensures the Scenario B stack is up (reuse if already serving)            [stack.sh]
#   2. mints commercial-bank + central-bank JWTs via the gateway login          [auth.sh]
#   3. applies the perf profile: clears transfer limits, asserts breaker active [profile.sh]
#   4. ensures the AMM pair has 30-TPS-sized depth and is ACTIVE                 [seed.sh]
#   5. runs every threshold benchmark with --summary-export JSON capture        [k6 scripts]
#        - latency baseline (thresholds 5,6,7,8)
#        - AMM 30 TPS throughput (threshold 3, DRAFT — validated/revised here)
#        - 50 TPS transfer (threshold 1)
#        - 15 TPS Zeto/privacy transfer (threshold 2)
#   6. measures end-to-end TTF by on-chain correlation (threshold 4)            [ttf.sh]
#   7. writes measured numbers + PASS/FAIL into docs/performance/RESULTS.md
#
# All steps log structured JSON to stdout. The 12h soak is a SEPARATE opt-in target.
#
# Knobs (all optional — sensible defaults make it zero-config):
#   API_GW_URL (3000), API_GW_CENTRAL_BANK_A_URL (38080)
#   PAIR (W-BRL-ARS — the sovereign pair scenario-b.up seeds), DURATION (3m for the throughput
#         runs — shorter than the 10m manual default
#         so the unattended suite finishes; override DURATION=10m for a publication run)
#   SWAP_TPS (30), TRANSFER_TPS (50), ZETO_TPS (15)
#   SKIP_STACK / SKIP_SEED (set by the dry-run smoke test)

set -u
PERF_DIR="$(cd "$(dirname "$0")" && pwd)"
PERF_SERVICE="perf-all"
# shellcheck source=lib/log.sh
. "$PERF_DIR/lib/log.sh"
# shellcheck source=lib/stack.sh
. "$PERF_DIR/lib/stack.sh"
# shellcheck source=lib/auth.sh
. "$PERF_DIR/lib/auth.sh"
# shellcheck source=lib/profile.sh
. "$PERF_DIR/lib/profile.sh"
# shellcheck source=lib/seed.sh
. "$PERF_DIR/lib/seed.sh"
# shellcheck source=lib/ttf.sh
. "$PERF_DIR/lib/ttf.sh"

: "${API_GW_URL:=http://localhost:18080}"
: "${API_GW_CENTRAL_BANK_A_URL:=http://localhost:38080}"
: "${PAIR:=W-BRL-ARS}"
: "${DURATION:=3m}"
: "${SWAP_TPS:=30}"
: "${TRANSFER_TPS:=50}"
: "${ZETO_TPS:=15}"

require_cmd k6
require_cmd curl
require_cmd awk

RESULTS_DIR="$PERF_DIR/results/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RESULTS_DIR"
log_info "results directory" dir="$RESULTS_DIR"

DOC_OUT="$PERF_DIR/../../docs/performance/RESULTS.md"
# Always emit a report from whatever summaries exist — even on Ctrl-C or an early
# failure — so a partial run is never lost. write-results.sh tolerates missing phases
# (they render as n/a). The trap fires once on normal EXIT or on INT/TERM.
_finalize() {
  trap - EXIT INT TERM
  log_info "finalising — writing results from captured summaries" dir="$RESULTS_DIR"
  "$PERF_DIR/lib/write-results.sh" "$RESULTS_DIR" "$DOC_OUT" \
    || log_warn "results writer reported an issue"
  log_info "perf-all results written" results="$RESULTS_DIR" doc="docs/performance/RESULTS.md"
}
trap '_finalize; exit 130' INT TERM
trap '_finalize' EXIT

# duration in seconds for depth/ttf math (accept Nm or Ns).
_dur_secs() {
  d="$1"
  case "$d" in
    *m) echo $(( ${d%m} * 60 )) ;;
    *s) echo "${d%s}" ;;
    *h) echo $(( ${d%h} * 3600 )) ;;
    *) echo "$d" ;;
  esac
}
DUR_SECS="$(_dur_secs "$DURATION")"

# ── 1. stack ────────────────────────────────────────────────────────────────
stack_ensure_up

# ── 2. auth ─────────────────────────────────────────────────────────────────
CB_TOKEN=""; COMM_TOKEN=""
if [ "${SKIP_STACK:-0}" = "1" ]; then
  log_warn "SKIP_STACK=1 — skipping live auth (dry-run)"
else
  COMM_TOKEN="$(auth_commercial_bank_token)"
  CB_TOKEN="$(auth_central_bank_token)"
  log_info "tokens minted" commercial_len="${#COMM_TOKEN}" central_len="${#CB_TOKEN}"
fi

# ── 3. profile (limits + circuit breaker) ─────────────────────────────────────
if [ -n "$CB_TOKEN" ]; then
  profile_apply "$API_GW_CENTRAL_BANK_A_URL" "$CB_TOKEN"
else
  log_warn "no central-bank token — skipping perf-profile (limits/breaker) step"
fi

# ── 4. seed depth ─────────────────────────────────────────────────────────────
seed_ensure_depth "$PAIR" "$COMM_TOKEN" "$SWAP_TPS" "$DUR_SECS" 1000

# ── 5. benchmarks (with --summary-export) ──────────────────────────────────────
run_k6() { # NAME SCRIPT EXTRA_ENV...
  name="$1"; script="$2"; shift 2
  out="$RESULTS_DIR/${name}.summary.json"
  log_info "running benchmark" name="$name" duration="$DURATION"
  # shellcheck disable=SC2086
  env API_GW_URL="$API_GW_URL" AUTH_TOKEN="$COMM_TOKEN" "$@" \
    k6 run --summary-export="$out" "$PERF_DIR/$script" \
    > "$RESULTS_DIR/${name}.log" 2>&1
  rc=$?
  if [ "$rc" -ne 0 ]; then
    log_warn "benchmark exited non-zero (threshold breach or error) — captured for results" name="$name" rc="$rc"
  fi
  echo "$out"
}

if [ "${SKIP_STACK:-0}" = "1" ]; then
  log_warn "SKIP_STACK=1 — dry-run: skipping live benchmark execution + TTF"
else
  BASELINE_OUT="$(run_k6 baseline scenario-b-perf.js PAIR="$PAIR" DURATION="$DURATION" LOAD_MODEL=vus)"
  AMM_OUT="$(run_k6 amm-throughput scenario-b-perf.js PAIR="$PAIR" DURATION="$DURATION" LOAD_MODEL=rate SWAP_TPS="$SWAP_TPS" QUOTE_TPS=$(( SWAP_TPS * 2 )))"
  TRANSFER_OUT="$(run_k6 transfer k6/bridge-transfer-throughput.js DURATION="$DURATION" TRANSFER_TPS="$TRANSFER_TPS" TOKEN_KIND=noto)"
  ZETO_OUT="$(run_k6 zeto k6/bridge-transfer-throughput.js DURATION="$DURATION" TRANSFER_TPS="$ZETO_TPS" TOKEN_KIND=zeto)"

  # ── 6. TTF ──────────────────────────────────────────────────────────────────
  ttf_run "$COMM_TOKEN" "${TTF_TPS:-10}" "${TTF_SECS:-60}" "$RESULTS_DIR"
fi

# ── 7. write RESULTS.md ────────────────────────────────────────────────────────
# Handled by the _finalize EXIT trap (defined above) so it also runs on interruption.
