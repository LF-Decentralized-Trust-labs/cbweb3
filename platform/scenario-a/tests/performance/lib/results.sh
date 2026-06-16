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
  read_p95=$(perf_jq_metric "$base" '.metrics.htlc_search_latency_ms.values["p(95)"]')
  fx_p95=$(perf_jq_metric "$base" '.metrics.fx_list_latency_ms.values["p(95)"]')
  write_p95=$(perf_jq_metric "$base" '.metrics.htlc_lock_latency_ms.values["p(95)"]')
  err_rate=$(perf_jq_metric "$base" '.metrics.http_req_failed.values.rate')

  # transfer (50 TPS): achieved rate = locks / duration(s); error rate
  local xfer_count xfer_rate xfer_err xfer_dur xfer_tps
  xfer_count=$(perf_jq_metric "$xfer" '.metrics.htlc_locked_total.values.count')
  xfer_rate=$(perf_jq_metric "$xfer" '.metrics.htlc_lock_rate.values.rate')
  xfer_err=$(perf_jq_metric "$xfer" '.metrics.http_req_failed.values.rate')
  # k6 records iteration throughput on .metrics.iterations.values.rate (per second)
  xfer_tps=$(perf_jq_metric "$xfer" '.metrics.iterations.values.rate')

  local zeto_count zeto_tps zeto_err
  zeto_count=$(perf_jq_metric "$zeto" '.metrics.zeto_escrow_submitted_total.values.count')
  zeto_tps=$(perf_jq_metric "$zeto" '.metrics.iterations.values.rate')
  zeto_err=$(perf_jq_metric "$zeto" '.metrics.http_req_failed.values.rate')

  local ttf_p50 ttf_p95 ttf_n ttf_pass
  if [ -f "$ttf" ]; then
    ttf_p50=$(jq -r '.p50_s // "n/a"' "$ttf" 2>/dev/null)
    ttf_p95=$(jq -r '.p95_s // "n/a"' "$ttf" 2>/dev/null)
    ttf_n=$(jq -r '.samples // 0' "$ttf" 2>/dev/null)
    ttf_pass=$(jq -r 'if .gate_pass==true then "PASS" elif .gate_pass==false then "FAIL" else "n/a" end' "$ttf" 2>/dev/null)
  else
    ttf_p50="n/a"; ttf_p95="n/a"; ttf_n=0; ttf_pass="n/a"
  fi

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
| Stack brought up via | \`make spoke-a\` (auto, lib/stack.sh) |
| k6 version | ${k6ver} |
| API gateway (bank-a) | ${gw} |
| RECEIVER identity used | ${rcv} |
| Raw evidence | ${dir}/{baseline,transfer,zeto,ttf}.summary.json |

## Threshold results

| # | Threshold | Target | Measured | Pass/Fail | Notes |
|---|-----------|--------|----------|-----------|-------|
| 1 | HTLC token-transfer throughput | 50 TPS sustained | ${xfer_tps} TPS (${xfer_count} locks) | ${xfer_pf} | iteration rate; lock-success rate ${xfer_rate} |
| 2 | Zeto (privacy) escrow throughput | 15 TPS sustained | ${zeto_tps} TPS (${zeto_count} escrows) | ${zeto_pf} | escrow-request admission rate |
| 3 | Time-To-Finality per spoke (TTF) | p95 < 5s | p50 ${ttf_p50} / p95 ${ttf_p95} s | ${ttf_pass} | HTLCLocked event correlation, n=${ttf_n} (README §4) |
| 4 | API read p95 latency | < 500ms | search ${read_p95} / fx ${fx_p95} ms | ${read4_pf} | both read endpoints must pass |
| 5 | API write p95 latency | < 1500ms | ${write_p95} ms | ${write_pf} | \`htlc_lock_latency_ms\` (admission only) |
| 6 | Error rate (steady state) | < 1% | ${err_pct} % | ${err_pf} | baseline \`http_req_failed\`; transfer ${xfer_err}, zeto ${zeto_err} |
| 7 | Resource stability (12h soak) | no leak / crash | (run \`make scenario-a.perf-soak-all\`) | n/a | opt-in; not part of perf-all |

## Per-scenario evidence

- API latency baseline: \`${base}\`
- 50 TPS HTLC transfer: \`${xfer}\`
- 15 TPS Zeto escrow: \`${zeto}\`
- Time-To-Finality correlation: \`${ttf}\`

## Overall verdict

$(perf_overall_verdict "$xfer_pf" "$zeto_pf" "$ttf_pass" "$read4_pf" "$write_pf" "$err_pf")
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
