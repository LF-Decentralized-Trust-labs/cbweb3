#!/usr/bin/env bash
# results.sh — render a measured RESULTS-<UTC>.md from RESULTS-TEMPLATE.md plus
# the k6 --summary-export JSONs and the TTF correlation summary.
#
# The template ships with empty "Measured" cells (do-not-fabricate policy). This
# helper reads the machine-readable evidence captured during the run and writes
# a populated results file next to the template, with each threshold marked
# PASS/FAIL against its gate. Source after log.sh.
#
# Functions:
#   perf_jq_metric        SUMMARY_JSON JQ_PATH          -> numeric metric or "n/a"
#   perf_pf               MEASURED OP GATE              -> "PASS"|"FAIL"|"n/a"
#   perf_write_results    OUT_MD ARTIFACT_DIR TEMPLATE  -> populated results file
#
# Requires jq.  All inputs are optional/defensive: a missing summary yields
# "n/a" and an "n/a" verdict rather than a crash, so a partial run still
# produces a coherent (clearly-incomplete) report.

# perf_jq_metric SUMMARY_JSON JQ_PATH
# Echoes the numeric value at JQ_PATH in a k6 summary-export file (rounded to 4
# significant decimals to keep small error rates intact), or "n/a" if the
# file/key is missing.
perf_jq_metric() {
  local file="$1" path="$2" v
  [ -f "$file" ] || { printf 'n/a'; return 0; }
  v=$(jq -r "($path) // empty | if type==\"number\" then (.*10000|round/10000) else . end" "$file" 2>/dev/null)
  [ -n "$v" ] && printf '%s' "$v" || printf 'n/a'
}

# perf_pf MEASURED OP GATE   (OP is "<" or ">")
# Echoes PASS/FAIL/n-a comparing a measured number against its gate.
perf_pf() {
  local measured="$1" op="$2" gate="$3"
  case "$measured" in ''|n/a|null) printf 'n/a'; return 0 ;; esac
  awk -v m="$measured" -v g="$gate" -v op="$op" \
    'BEGIN{ ok = (op=="<") ? (m<g) : (m>g); print ok ? "PASS" : "FAIL" }'
}

# perf_write_results OUT_MD ARTIFACT_DIR [TEMPLATE]
# Reads the per-scenario summaries under ARTIFACT_DIR and emits a populated
# results markdown to OUT_MD.  Expected evidence file names (best-effort):
#   baseline.summary.json   transfer.summary.json   zeto.summary.json
#   ttf.summary.json        meta.json
perf_write_results() {
  local out="$1" dir="$2"
  local base="${dir}/baseline.summary.json"
  local xfer="${dir}/transfer.summary.json"
  local zeto="${dir}/zeto.summary.json"
  local ttf="${dir}/ttf.summary.json"
  local meta="${dir}/meta.json"

  # --- pull measured values -------------------------------------------------
  # k6 summary-export schema: .metrics.<name>.{values}.<stat>
  local read_p95 fx_p95 write_p95 err_rate
  # k6 --summary-export schema (v2.x): stats live DIRECTLY under .metrics.<name>
  # (e.g. ["p(95)"], .med for median≈p50). Rate metrics expose .value; Counters
  # expose .count and .rate. (There is NO .values wrapper — that is the in-process
  # end-of-test summary object, not the exported JSON.)
  read_p95=$(perf_jq_metric "$base" '.metrics.htlc_search_latency_ms["p(95)"]')
  fx_p95=$(perf_jq_metric "$base" '.metrics.fx_list_latency_ms["p(95)"]')
  write_p95=$(perf_jq_metric "$base" '.metrics.htlc_lock_latency_ms["p(95)"]')
  err_rate=$(perf_jq_metric "$base" '.metrics.http_req_failed.value')

  # transfer (50 TPS): achieved rate = locks / duration(s); error rate
  local xfer_count xfer_rate xfer_err xfer_dur xfer_tps
  xfer_count=$(perf_jq_metric "$xfer" '.metrics.htlc_locked_total.count')
  xfer_rate=$(perf_jq_metric "$xfer" '.metrics.htlc_lock_rate.value')
  xfer_err=$(perf_jq_metric "$xfer" '.metrics.http_req_failed.value')
  # iterations is a Counter: .rate is iterations/second
  xfer_tps=$(perf_jq_metric "$xfer" '.metrics.iterations.rate')

  local zeto_count zeto_tps zeto_err
  zeto_count=$(perf_jq_metric "$zeto" '.metrics.zeto_escrow_submitted_total.count')
  zeto_tps=$(perf_jq_metric "$zeto" '.metrics.iterations.rate')
  zeto_err=$(perf_jq_metric "$zeto" '.metrics.http_req_failed.value')

  local ttf_p50 ttf_p95 ttf_n ttf_pass
  if [ -f "$ttf" ]; then
    ttf_p50=$(jq -r '.p50_s // "n/a"' "$ttf" 2>/dev/null)
    ttf_p95=$(jq -r '.p95_s // "n/a"' "$ttf" 2>/dev/null)
    ttf_n=$(jq -r '.samples // 0' "$ttf" 2>/dev/null)
    ttf_pass=$(jq -r 'if .gate_pass==true then "PASS" elif .gate_pass==false then "FAIL" else "n/a" end' "$ttf" 2>/dev/null)
  else
    ttf_p50="n/a"; ttf_p95="n/a"; ttf_n=0; ttf_pass="n/a"
  fi

  # baseline p50 (D6 read p50<200ms, write p50<800ms). k6 exports the median as
  # .med (= p50); it does not export an explicit "p(50)" by default.
  local read_p50 fx_p50 write_p50
  read_p50=$(perf_jq_metric "$base" '.metrics.htlc_search_latency_ms.med')
  fx_p50=$(perf_jq_metric "$base" '.metrics.fx_list_latency_ms.med')
  write_p50=$(perf_jq_metric "$base" '.metrics.htlc_lock_latency_ms.med')

  # happy path — Scenario A HTLC settlement lifecycle (D6 timing constraints)
  local happy="${dir}/happy-path.summary.json"
  local life_p50 life_p95 relay_p95 apisync_p95 settle_rate settle_n
  life_p50=$(perf_jq_metric "$happy" '.metrics.fx_settlement_latency_ms.med')
  life_p95=$(perf_jq_metric "$happy" '.metrics.fx_settlement_latency_ms["p(95)"]')
  relay_p95=$(perf_jq_metric "$happy" '.metrics.fx_relay_propagation_ms["p(95)"]')
  apisync_p95=$(perf_jq_metric "$happy" '.metrics.api_sync_ms["p(95)"]')
  settle_rate=$(perf_jq_metric "$happy" '.metrics.fx_settlement_rate.value')
  settle_n=$(perf_jq_metric "$happy" '.metrics.fx_settlement_success_total.count')

  # --- verdicts -------------------------------------------------------------
  # error rate as percent for display
  local err_pct read_pf fx_pf write_pf err_pf xfer_pf zeto_pf xfer_err_pf
  err_pct=$(awk -v r="$err_rate" 'BEGIN{ if(r=="n/a"||r==""){print "n/a"} else {printf "%.2f", r*100} }')
  read_pf=$(perf_pf "$read_p95" "<" 500)
  fx_pf=$(perf_pf "$fx_p95" "<" 500)
  write_pf=$(perf_pf "$write_p95" "<" 1500)
  err_pf=$(perf_pf "$err_rate" "<" 0.01)
  xfer_pf=$(perf_pf "$xfer_tps" ">" 49.5)   # 50 TPS target, allow 1% admission slack
  zeto_pf=$(perf_pf "$zeto_tps" ">" 14.85)  # 15 TPS target

  # read p95 threshold (4) is PASS only if BOTH read endpoints pass
  local read4_pf="n/a"
  if [ "$read_pf" != "n/a" ] && [ "$fx_pf" != "n/a" ]; then
    if [ "$read_pf" = "PASS" ] && [ "$fx_pf" = "PASS" ]; then read4_pf="PASS"; else read4_pf="FAIL"; fi
  fi

  # D6 p50 verdicts
  local read_p50_pf fx_p50_pf read_p50_4_pf write_p50_pf
  read_p50_pf=$(perf_pf "$read_p50" "<" 200)
  fx_p50_pf=$(perf_pf "$fx_p50" "<" 200)
  read_p50_4_pf="n/a"
  if [ "$read_p50_pf" != "n/a" ] && [ "$fx_p50_pf" != "n/a" ]; then
    if [ "$read_p50_pf" = "PASS" ] && [ "$fx_p50_pf" = "PASS" ]; then read_p50_4_pf="PASS"; else read_p50_4_pf="FAIL"; fi
  fi
  write_p50_pf=$(perf_pf "$write_p50" "<" 800)

  # D6 Scenario-A HTLC settlement lifecycle verdicts
  local life_pf relay_pf apisync_pf settle_pf settle_pct
  life_pf=$(perf_pf "$life_p95" "<" 60000)     # full lifecycle ≤ 60s
  relay_pf=$(perf_pf "$relay_p95" "<" 15000)   # relayer cross-chain ≤ 15s
  apisync_pf=$(perf_pf "$apisync_p95" "<" 30000) # synchronous API response ≤ 30s
  settle_pf=$(perf_pf "$settle_rate" ">" 0.9)
  settle_pct=$(awk -v r="$settle_rate" 'BEGIN{ if(r=="n/a"||r==""){print "n/a"} else {printf "%.1f", r*100} }')

  local ts commit env_note k6ver gw rcv
  ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
  if [ -f "$meta" ]; then
    env_note=$(jq -r '.environment // "local devnet"' "$meta" 2>/dev/null)
    k6ver=$(jq -r '.k6_version // "n/a"' "$meta" 2>/dev/null)
    gw=$(jq -r '.api_gw_url // "n/a"' "$meta" 2>/dev/null)
    rcv=$(jq -r '.receiver // "n/a"' "$meta" 2>/dev/null)
  else
    env_note="local devnet"; k6ver="n/a"; gw="n/a"; rcv="n/a"
  fi

  log_info "writing results file" out="$out" artifact_dir="$dir"

  cat > "$out" <<EOF
# Scenario A — Measured Performance Results (R1-12.3)

> Auto-generated by \`make scenario-a.perf-all\` (lib/results.sh).
> Raw evidence: \`${dir}\` (k6 \`--summary-export\` JSONs + TTF correlation).
> A measured p95 exceeding its gate by **>20%** MUST block merge (Decision 13).

## Run metadata

| Field | Value |
|-------|-------|
| Date (ISO-8601) | ${ts} |
| Operator | $(whoami 2>/dev/null || echo "n/a") |
| Git commit | ${commit} |
| Environment | ${env_note} |
| Besu version / consensus | 25.8.0 / QBFT |
| Stack provisioned via | \`samples/deploy-all.sh\` (the toolkit; the harness reuses it) |
| k6 version | ${k6ver} |
| API gateway (bank-a) | ${gw} |
| RECEIVER identity used | ${rcv} |
| Raw evidence | ${dir}/{baseline,transfer,zeto,ttf,happy-path}.summary.json |

## Threshold results

| # | Threshold | Target | Measured | Pass/Fail | Notes |
|---|-----------|--------|----------|-----------|-------|
| 1 | HTLC token-transfer throughput | 50 TPS sustained | ${xfer_tps} TPS (${xfer_count} locks) | ${xfer_pf} | iteration rate; lock-success rate ${xfer_rate} |
| 2 | Zeto (privacy) escrow throughput | 15 TPS sustained | ${zeto_tps} TPS (${zeto_count} escrows) | ${zeto_pf} | escrow-request admission rate |
| 3 | Time-To-Finality per spoke (TTF) | p95 < 5s | p50 ${ttf_p50} / p95 ${ttf_p95} s | ${ttf_pass} | HTLCLocked event correlation, n=${ttf_n} (README §4) |
| 4 | API read latency | p50<200 / p95<500 ms | p50 search ${read_p50}/fx ${fx_p50}; p95 search ${read_p95}/fx ${fx_p95} ms | p50 ${read_p50_4_pf} / p95 ${read4_pf} | both read endpoints |
| 5 | API write latency (admission) | p50<800 / p95<1500 ms | p50 ${write_p50}; p95 ${write_p95} ms | p50 ${write_p50_pf} / p95 ${write_pf} | \`htlc_lock_latency_ms\` |
| 6 | Error rate (steady state) | < 1% | ${err_pct} % | ${err_pf} | baseline \`http_req_failed\`; transfer ${xfer_err}, zeto ${zeto_err} |
| 7 | Resource stability (12h soak) | no leak / crash | (run \`make scenario-a.perf-soak-all\`) | n/a | opt-in; not part of perf-all |

## Scenario A — HTLC settlement lifecycle (D6 timing constraints)

End-to-end correspondent-banking settlement (FX propose → custodian accept → dual-leg
HTLC lock → secret reveal → both spokes SETTLED), driven by the happy-path benchmark
(\`k6/fx-settlement-throughput.js\`). State is polled every 1s (no fixed sleeps), per D6.

| D6 constraint | Target | Measured | Pass/Fail | Notes |
|---|--------|----------|-----------|-------|
| Full HTLC lifecycle (end to end) | ≤ 60s | p50 ${life_p50} / p95 ${life_p95} ms | ${life_pf} | ${settle_n} settlements completed |
| Synchronous API response | ≤ 30s | p95 ${apisync_p95} ms | ${apisync_pf} | propose/accept/lock/settle calls (\`api_sync_ms\`) |
| Relayer cross-chain propagation | ≤ 15s | p95 ${relay_p95} ms | ${relay_pf} | secret carry → custodian SETTLED (reported) |
| Settlement completion rate | > 90% | ${settle_pct} % | ${settle_pf} | relay-bound; raise \`HAPPY_VUS\` for burst/stress |
| Transaction mining (per spoke) | ≤ 10s | see TTF (row 3) | ${ttf_pass} | QBFT single-block finality |

## Per-scenario evidence

- API latency baseline: \`${base}\`
- 50 TPS HTLC transfer: \`${xfer}\`
- 15 TPS Zeto escrow: \`${zeto}\`
- Time-To-Finality correlation: \`${ttf}\`
- Full happy-path settlement (D6 lifecycle): \`${happy}\`

## Overall verdict

$(perf_overall_verdict "$xfer_pf" "$zeto_pf" "$ttf_pass" "$read4_pf" "$write_pf" "$err_pf" \
    "$read_p50_4_pf" "$write_p50_pf" "$life_pf" "$apisync_pf" "$settle_pf")
EOF

  log_info "results written" out="$out"
}

# perf_overall_verdict PF...  -> a one-line verdict; FAIL if any arg is FAIL.
perf_overall_verdict() {
  local any_fail=0 any_na=0 pf
  for pf in "$@"; do
    case "$pf" in FAIL) any_fail=1 ;; n/a) any_na=1 ;; esac
  done
  if [ "$any_fail" = "1" ]; then
    echo "- Run status: **One or more thresholds FAIL** — blocks merge per Decision 13."
  elif [ "$any_na" = "1" ]; then
    echo "- Run status: **Incomplete** — some thresholds could not be measured (see n/a cells); not a clean PASS."
  else
    echo "- Run status: **All measured thresholds PASS** (soak is separate; run \`make scenario-a.perf-soak-all\`)."
  fi
}
