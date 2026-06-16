#!/usr/bin/env bash
# ttf.sh — measure end-to-end Time-To-Finality (threshold 4, <5s) by on-chain correlation.
#
# A black-box HTTP probe cannot measure TTF: lock-mint returns 201 immediately and the relay
# (interop/hub-and-spoke/cacti) finalises asynchronously across two Besu networks. This helper
# does the correlation the README §4 describes:
#
#   t0 = client send time  — emitted by k6 as `POSITION_ID <position_id> <epoch_ms>`
#        (bridge-transfer-throughput.js with PRINT_IDS=1).
#   t1 = on-chain finality  — the moment the bridge position reaches bridge_state=ACTIVE, which
#        the gateway only reports AFTER the relay observed the Spoke lock event and completed the
#        Hub mint (constitution III lock->mint). Polled via GET /api/v2/bridge/positions.
#        The Cacti relay log (`cbweb3-cacti-liquidity-relay`) carries the matching Minted event
#        and is the documented cross-check (see docs/performance/README.md §4).
#
#   TTF = t1 - t0. We report p50/p95 across the batch. Gate: p95 < 5000 ms.
#
# Functions:
#   ttf_run TOKEN TPS SECS OUTDIR  -> runs a short transfer batch, correlates, writes
#                                     $OUTDIR/ttf.json with {p50_ms,p95_ms,samples,...}
#
# Env:
#   API_GW_URL   commercial-bank gateway (default http://localhost:18080)
#   TTF_TPS      transfer rate for the TTF batch (default 10 — moderate, finality-friendly)
#   TTF_SECS     batch duration seconds (default 60)
#   TTF_POLL_TIMEOUT  per-position finality wait, seconds (default 30)

: "${API_GW_URL:=http://localhost:18080}"

# _positions_active_map TOKEN -> echoes "<position_id> <epoch_ms_now>" for every ACTIVE position.
# We stamp "now" because the API does not expose a finalised-at timestamp; polling cadence (1s)
# bounds the t1 error, which is recorded in the methodology.
_positions_active() {
  token="$1"
  curl -s --max-time 20 -H "Authorization: Bearer $token" -H 'Accept: application/json' \
    "$API_GW_URL/api/v2/bridge/positions?state=ACTIVE" 2>/dev/null \
    | grep -o '"position_id":"[^"]*"' | sed 's/"position_id":"//;s/"//'
}

# ttf_run TOKEN TPS SECS OUTDIR
ttf_run() {
  token="$1"; tps="${2:-${TTF_TPS:-10}}"; secs="${3:-${TTF_SECS:-60}}"; outdir="$4"
  poll_timeout="${TTF_POLL_TIMEOUT:-30}"
  require_cmd k6; require_cmd awk; require_cmd curl
  mkdir -p "$outdir"
  perfdir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

  log_info "running TTF correlation batch" tps="$tps" secs="${secs}s"
  ids_file="$outdir/ttf-ids.txt"
  # Capture POSITION_ID <id> <t0_ms> lines from the k6 run.
  API_GW_URL="$API_GW_URL" AUTH_TOKEN="$token" \
    TRANSFER_TPS="$tps" DURATION="${secs}s" PRINT_IDS=1 \
    k6 run "$perfdir/k6/bridge-transfer-throughput.js" 2>&1 \
    | grep -E '^.*POSITION_ID ' | sed 's/.*POSITION_ID /POSITION_ID /' > "$ids_file" || true

  count="$(grep -c POSITION_ID "$ids_file" 2>/dev/null || echo 0)"
  if [ "$count" = "0" ]; then
    log_warn "no position ids captured — cannot compute TTF (was the transfer path healthy?)"
    printf '{"p50_ms":null,"p95_ms":null,"samples":0,"note":"no position ids"}\n' > "$outdir/ttf.json"
    return 0
  fi
  log_info "captured position ids; polling for on-chain finality" samples="$count"

  ttf_samples="$outdir/ttf-samples.txt"; : > "$ttf_samples"
  # For each (id,t0) wait until it appears ACTIVE, stamp t1, record TTF ms.
  while read -r _tag id t0; do
    [ -n "$id" ] || continue
    deadline=$(( $(date +%s) + poll_timeout ))
    while :; do
      if _positions_active "$token" | grep -qx "$id"; then
        t1=$(( $(date +%s) * 1000 ))
        printf '%s\n' "$(( t1 - t0 ))" >> "$ttf_samples"
        break
      fi
      [ "$(date +%s)" -lt "$deadline" ] || { log_warn "position not ACTIVE within timeout" id="$id" timeout="${poll_timeout}s"; break; }
      sleep 1
    done
  done < "$ids_file"

  # Compute p50/p95 in awk.
  awk '
    { v[NR]=$1 }
    END {
      n=NR
      if (n==0) { print "{\"p50_ms\":null,\"p95_ms\":null,\"samples\":0}"; exit }
      # insertion sort (small n)
      for (i=2;i<=n;i++){k=v[i];j=i-1;while(j>0&&v[j]>k){v[j+1]=v[j];j--}v[j+1]=k}
      p50=v[int((n*0.50)+0.999)]; p95=v[int((n*0.95)+0.999)]
      if(p50=="")p50=v[n]; if(p95=="")p95=v[n]
      printf "{\"p50_ms\":%d,\"p95_ms\":%d,\"samples\":%d,\"max_ms\":%d}\n", p50, p95, n, v[n]
    }
  ' "$ttf_samples" > "$outdir/ttf.json"

  log_info "TTF computed" result="$(cat "$outdir/ttf.json")"
}
