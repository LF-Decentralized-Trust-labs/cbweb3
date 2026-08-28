#!/usr/bin/env bash
# run-all.sh — zero-config orchestrator for the R1-12.3 Scenario A perf suite.
#
# Invoked by `make scenario-a.perf-all` (from scenario-a/). Needs NO arguments:
# it stands up the stack if needed, mints the auth token(s), funds the sender,
# runs the threshold benchmarks (baseline + 50 TPS HTLC + 15 TPS Zeto), performs
# on-chain TTF correlation, captures --summary-export evidence, and writes a
# populated RESULTS-<UTC>.md.  The 12h soak is a SEPARATE opt-in target.
#
# Everything is overridable by env var but nothing is REQUIRED:
#   API_GW_URL     originator bank API gateway  (derived from the toolkit manifests)
#   CB_GW_URL      central-bank-a API gateway   (derived from the toolkit manifests)
#   BANK_ENV       bank-a infra env file       (default backend/config/.env.infra.bank-a)
#   CB_ENV         central-bank-a infra env    (default backend/config/.env.infra.central-bank-a)
#   RECEIVER       HTLC receiver identity      (default funded_operator@spoke-a-bank-c)
#   RPC_URL        spoke-a bank-a Besu RPC     (default http://localhost:8646)
#   DURATION       per-benchmark duration      (default 10m; baseline uses BASELINE_DURATION)
#   TRANSFER_TPS / ZETO_TPS                     (defaults 50 / 15)
#   PERF_DRY_RUN=1 validate orchestration with NO infra (no stack/k6/rpc calls)
#   PERF_SKIP_STACK=1 / PERF_SKIP_FUND=1        skip those phases (stack already proven)
#   PERF_SOAK=1    run the 12h soak instead of the threshold suite (opt-in path)
#
# Exit non-zero if any benchmark's built-in k6 threshold gate is breached.

set -u
set -o pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/log.sh
# Dry-run guard: the dry run is the CI-safe smoke for this orchestrator — it contacts nothing
# and must not need a stack, a manifest or yq on the box. Give the endpoint variables an
# unroutable placeholder so the required-value checks downstream are satisfied without
# reintroducing a real port that a live run could silently fall back to. `.invalid` is
# reserved by RFC 2606 and can never resolve.
if [ "${PERF_DRY_RUN:-0}" = "1" ]; then
  : "${API_GW_URL:=http://dry-run.invalid}"
  : "${CB_GW_URL:=http://dry-run.invalid}"
  : "${API_GW_CENTRAL_BANK_A_URL:=http://dry-run.invalid}"
  : "${API_GW_CENTRAL_BANK_B_URL:=http://dry-run.invalid}"
  : "${API_GW_BANK_B_URL:=http://dry-run.invalid}"
  : "${BANK_D_GW_URL:=http://dry-run.invalid}"
  : "${PERF_CUSTODIAN_GW_URL:=http://dry-run.invalid}"
fi

. "${HERE}/lib/log.sh"
. "${HERE}/lib/auth.sh"
. "${HERE}/lib/stack.sh"
. "${HERE}/lib/fund.sh"
. "${HERE}/lib/ttf.sh"
. "${HERE}/lib/results.sh"
. "${HERE}/lib/provision.sh"

# ── Config (all defaulted) ───────────────────────────────────────────────────
# No port defaults: the retired deploy/local topology published these, and defaulting to
# one made the harness probe a gateway nothing serves (DEF-022). scenario-a.perf-* derive
# the real endpoints from the toolkit manifests (tests/integration/toolkit-env.sh).
: "${API_GW_URL:?API_GW_URL is required — run through 'make scenario-a.perf-all', or derive it with tests/integration/toolkit-env.sh}"
CB_GW_URL="${CB_GW_URL:-${API_GW_CENTRAL_BANK_A_URL:?CB_GW_URL or API_GW_CENTRAL_BANK_A_URL is required}}"
BANK_ENV="${BANK_ENV:-backend/config/.env.infra.bank-a}"
CB_ENV="${CB_ENV:-backend/config/.env.infra.central-bank-a}"
RECEIVER="${RECEIVER:-funded_operator@spoke-a-bank-c}"
RPC_URL="${RPC_URL:-http://localhost:8646}"
DURATION="${DURATION:-10m}"
BASELINE_DURATION="${BASELINE_DURATION:-2m}"
TTF_DURATION="${TTF_DURATION:-2m}"
TTF_TPS="${TTF_TPS:-10}"
TRANSFER_TPS="${TRANSFER_TPS:-50}"
ZETO_TPS="${ZETO_TPS:-15}"
PERF_DRY_RUN="${PERF_DRY_RUN:-0}"

# Full happy-path (FX + cross-spoke HTLC settlement) benchmark — the Scenario A
# analogue of Scenario B's end-to-end cross-currency measurement. Relay-bound, so a
# few concurrent flows (HAPPY_VUS) rather than a high TPS gate.
# The custodian leg. PERF_CUSTODIAN_GW_URL is what the make target exports from the
# manifests; BANK_D_GW_URL stays accepted so an explicit override still works.
BANK_D_GW_URL="${BANK_D_GW_URL:-${PERF_CUSTODIAN_GW_URL:?BANK_D_GW_URL or PERF_CUSTODIAN_GW_URL is required for the happy-path leg}}"
BANK_D_ENV="${BANK_D_ENV:-backend/config/.env.infra.bank-d}"
# Sequential by default: the cross-spoke path is relay-bound + single-signer per
# bank (one EVM operator key), so 1 flow gives the clean per-lifecycle D6 timing
# with 100% completion. Raise HAPPY_VUS for the burst/stress profiles — concurrency
# >1 deliberately surfaces the single-signer (nonce-serialised) bottleneck.
HAPPY_VUS="${HAPPY_VUS:-1}"
HAPPY_DURATION="${HAPPY_DURATION:-3m}"
# PERF_ONLY_HAPPY=1 → run ONLY the happy-path benchmark (skip baseline/transfer/zeto/TTF).
PERF_ONLY_HAPPY="${PERF_ONLY_HAPPY:-0}"
RUN_COMPONENTS=1
[ "$PERF_ONLY_HAPPY" = "1" ] && RUN_COMPONENTS=0

export PERF_DRY_RUN API_GW_URL

# The dry run is the CI-safe smoke for this orchestrator, so it keeps its output out of the
# tree: docs/performance/ holds published measured runs, and a no-infra run has no numbers to
# contribute. Both the artefact dir and the RESULTS file go to a temp dir instead.
if [ "$PERF_DRY_RUN" = "1" ]; then
  PERF_ARTIFACT_BASE="$(mktemp -d)/artifacts"
  export PERF_ARTIFACT_BASE
fi

RUN_ID="$(perf_run_id)"
ART="$(perf_artifact_dir)"
if [ "$PERF_DRY_RUN" = "1" ]; then
  RESULTS_DIR="$(mktemp -d)/results"
else
  RESULTS_DIR="docs/performance"
fi
RESULTS_OUT="${RESULTS_DIR}/RESULTS-$(date -u +%Y-%m-%dT%H%M%SZ).md"
FAILED=0

log_info "perf-all starting" run_id="$RUN_ID" artifact_dir="$ART" \
  api_gw="$API_GW_URL" cb_gw="$CB_GW_URL" receiver="$RECEIVER" mode="${PERF_SOAK:+soak}${PERF_SOAK:-threshold}"

# ── Preflight: k6 ─────────────────────────────────────────────────────────────
if [ "$PERF_DRY_RUN" != "1" ] && ! command -v k6 >/dev/null 2>&1; then
  log_error "k6 is required (https://k6.io)"; exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  log_error "jq is required"; exit 1
fi

# k6_run SCRIPT SUMMARY_OUT [extra env assignments...] — run k6 with summary export.
# Returns k6's exit code (non-zero if a threshold gate is breached).
k6_run() {
  local script="$1" summary="$2"; shift 2
  if [ "$PERF_DRY_RUN" = "1" ]; then
    log_info "dry-run: would run k6" script="$script" summary="$summary" env="$*"
    printf '{"metrics":{}}' > "$summary"   # placeholder so results.sh is exercised
    return 0
  fi
  log_info "running k6" script="$script" summary="$summary"
  env "$@" API_GW_URL="$API_GW_URL" \
    k6 run --summary-export="$summary" "$script"
}

# ── Phase 1: stack ────────────────────────────────────────────────────────────
# The suite now includes the cross-spoke happy-path benchmark, so by default it
# runs against the FULL stack (both spokes + custodian bank-d + relay). Only when
# the happy path is explicitly skipped (component-only run) does spoke-a suffice.
if [ "${PERF_SKIP_STACK:-0}" = "1" ]; then
  log_info "PERF_SKIP_STACK=1 — assuming stack is up"
elif [ "${PERF_SKIP_HAPPY:-0}" = "1" ]; then
  perf_ensure_stack "$API_GW_URL" || { log_error "stack unavailable"; exit 1; }
else
  perf_ensure_full_stack "$API_GW_URL" "$BANK_D_GW_URL" || { log_error "full stack unavailable"; exit 1; }
fi

# ── Phase 2: auth ─────────────────────────────────────────────────────────────
# The happy-path benchmark needs BOTH the originator (bank-a) and custodian (bank-d)
# tokens; the component benchmarks use only bank-a.
BANK_TOKEN=""
CB_TOKEN=""
CUSTODIAN_TOKEN=""
if [ "$PERF_DRY_RUN" = "1" ]; then
  log_info "dry-run: skipping auth mint"
  BANK_TOKEN="dry-run-token"; CB_TOKEN="dry-run-token"; CUSTODIAN_TOKEN="dry-run-token"
else
  BANK_TOKEN="$(perf_mint_token "$API_GW_URL" "$BANK_ENV")" || { log_error "bank-a auth failed"; exit 1; }
  CB_TOKEN="$(perf_mint_token "$CB_GW_URL" "$CB_ENV")"   || { log_error "central-bank-a auth failed"; exit 1; }
  CUSTODIAN_TOKEN="$(perf_mint_token "$BANK_D_GW_URL" "$BANK_D_ENV")" \
    || log_warn "bank-d (custodian) auth failed — happy-path benchmark will be skipped"
fi
export AUTH_TOKEN="$BANK_TOKEN"

# ── Phase 3: fund the sender ──────────────────────────────────────────────────
if [ "${PERF_SKIP_FUND:-0}" = "1" ]; then
  log_info "PERF_SKIP_FUND=1 — skipping funding"
else
  perf_fund_sender "$API_GW_URL" "$CB_GW_URL" "$BANK_TOKEN" "$CB_TOKEN" \
    || log_warn "funding step failed — continuing (run may hit funding limits)"
fi

# ── Phase 3b: provision happy-path actors (fund originator + custodian) ─────────
# The FX-settlement happy path locks real tCeBM on both legs, so the originator
# (bank-a) and custodian (bank-d) operators must hold reserves. Idempotent-enough.
provision_settlement_actors || log_warn "provision step incomplete — happy-path benchmark may revert"

# ── meta.json (for results.sh) ────────────────────────────────────────────────
k6ver="n/a"
if command -v k6 >/dev/null 2>&1; then k6ver="$(k6 version 2>/dev/null | head -1)"; fi
jq -nc --arg gw "$API_GW_URL" --arg rcv "$RECEIVER" --arg k6 "$k6ver" \
  --arg env "local devnet (spoke-a, Besu QBFT)" \
  '{api_gw_url:$gw, receiver:$rcv, k6_version:$k6, environment:$env}' > "${ART}/meta.json"

# ── SOAK path (opt-in) ────────────────────────────────────────────────────────
if [ "${PERF_SOAK:-0}" = "1" ]; then
  log_info "SOAK mode — running 12h soak (opt-in)" duration="${DURATION}"
  k6_run "${HERE}/k6/soak.js" "${ART}/soak.summary.json" \
    AUTH_TOKEN="$BANK_TOKEN" RECEIVER="$RECEIVER" DURATION="${SOAK_DURATION:-12h}" \
    || { log_error "soak gate breached"; FAILED=1; }
  log_info "soak complete — capture out-of-band leak/crash evidence (README §5)" artifact_dir="$ART"
  exit "$FAILED"
fi

# ── Component benchmarks (baseline + isolated HTLC transfer + Zeto) ───────────
# Skipped when PERF_ONLY_HAPPY=1 (the standalone happy-path target).
if [ "$RUN_COMPONENTS" = "1" ]; then

# ── Phase 4: baseline (thresholds 4,5,6) ─────────────────────────────────────
k6_run "${HERE}/scenario-a-perf.js" "${ART}/baseline.summary.json" \
  AUTH_TOKEN="$BANK_TOKEN" RECEIVER="$RECEIVER" DURATION="$BASELINE_DURATION" \
  || { log_error "baseline gate breached"; FAILED=1; }

# ── Phase 5: 50 TPS HTLC transfer (threshold 1) + TTF id capture ─────────────
# Run the main throughput benchmark for the achieved-TPS / error gates.
k6_run "${HERE}/k6/htlc-transfer-throughput.js" "${ART}/transfer.summary.json" \
  AUTH_TOKEN="$BANK_TOKEN" RECEIVER="$RECEIVER" \
  TRANSFER_TPS="$TRANSFER_TPS" DURATION="$DURATION" \
  || { log_error "transfer gate breached"; FAILED=1; }

# ── Phase 6: TTF correlation (threshold 3) ───────────────────────────────────
# Separate short PRINT_IDS run feeds contract_id+t0 lines to the TTF post-processor.
TTF_IDS="${ART}/ttf-ids.log"
if [ "$PERF_DRY_RUN" = "1" ]; then
  log_info "dry-run: skipping TTF id-capture + correlation"
  jq -nc '{p50_s:null,p95_s:null,samples:0,gate_pass:null,note:"dry-run"}' > "${ART}/ttf.summary.json"
else
  HTLC_ADDRESS="$(grep -E '^HTLC_ADDRESS=' "$BANK_ENV" 2>/dev/null | head -1 | cut -d= -f2-)"
  log_info "TTF: id-capture run" tps="$TTF_TPS" duration="$TTF_DURATION" htlc_address="${HTLC_ADDRESS:-<empty>}"
  env AUTH_TOKEN="$BANK_TOKEN" RECEIVER="$RECEIVER" API_GW_URL="$API_GW_URL" \
    TRANSFER_TPS="$TTF_TPS" DURATION="$TTF_DURATION" PRINT_IDS=1 \
    k6 run "${HERE}/k6/htlc-transfer-throughput.js" 2>&1 | grep '^CONTRACT_ID' > "$TTF_IDS" || true
  if [ -s "$TTF_IDS" ]; then
    perf_ttf_compute "$TTF_IDS" "$RPC_URL" "$HTLC_ADDRESS" "${ART}/ttf.summary.json" \
      || log_warn "TTF correlation incomplete"
  else
    log_warn "TTF: no contract_ids captured — skipping correlation"
    jq -nc '{p50_s:null,p95_s:null,samples:0,gate_pass:null,note:"no ids captured"}' > "${ART}/ttf.summary.json"
  fi
fi

# ── Phase 7: 15 TPS Zeto escrow (threshold 2) ────────────────────────────────
k6_run "${HERE}/k6/zeto-escrow-throughput.js" "${ART}/zeto.summary.json" \
  AUTH_TOKEN="$BANK_TOKEN" ZETO_TPS="$ZETO_TPS" DURATION="$DURATION" \
  || { log_error "zeto gate breached"; FAILED=1; }

fi  # RUN_COMPONENTS

# ── Phase 8: full happy-path settlement (FX + cross-spoke HTLC) ───────────────
# The headline measurement — drives the SAME flow as the integration happy path
# end to end (propose → accept → dual-leg lock → settle → relay settlement) and
# captures end-to-end settlement latency + completion rate. Relay-bound, so it runs
# a few concurrent flows (HAPPY_VUS) rather than a TPS gate.
if [ "${PERF_SKIP_HAPPY:-0}" = "1" ]; then
  log_info "PERF_SKIP_HAPPY=1 — skipping happy-path benchmark"
elif [ "$PERF_DRY_RUN" != "1" ] && [ -z "$CUSTODIAN_TOKEN" ]; then
  log_warn "no custodian (bank-d) token — skipping happy-path benchmark"
  jq -nc '{metrics:{},note:"skipped: no custodian token"}' > "${ART}/happy-path.summary.json"
else
  k6_run "${HERE}/k6/fx-settlement-throughput.js" "${ART}/happy-path.summary.json" \
    ORIGINATOR_TOKEN="$BANK_TOKEN" CUSTODIAN_TOKEN="$CUSTODIAN_TOKEN" \
    API_GW_BANK_D_URL="$BANK_D_GW_URL" HAPPY_VUS="$HAPPY_VUS" DURATION="$HAPPY_DURATION" \
    || { log_error "happy-path gate breached"; FAILED=1; }
fi

# ── Phase 9: write the results doc ────────────────────────────────────────────
mkdir -p "$RESULTS_DIR"
perf_write_results "$RESULTS_OUT" "$ART"

log_info "perf-all complete" results="$RESULTS_OUT" artifact_dir="$ART" failed="$FAILED"
if [ "$FAILED" != "0" ]; then
  log_error "one or more threshold gates breached — see $RESULTS_OUT"
fi
exit "$FAILED"
