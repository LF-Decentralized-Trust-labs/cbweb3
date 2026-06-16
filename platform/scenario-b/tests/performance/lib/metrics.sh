#!/usr/bin/env bash
# metrics.sh — out-of-band soak evidence collector (threshold 9: no leaks / no drift over 12h).
#
# The k6 soak only generates load; the leak/drift *evidence* comes from periodic host/container
# observation. This helper snapshots, on an interval, the signals README §5 lists:
#   - container RSS + Go goroutines (docker stats; /metrics if exposed)
#   - Postgres pg_stat_activity connection count (leak signal)
#   - Besu / relay container restart count (node-health signal)
#   - AMM constant-product invariant x*y=k for the pool (correctness/drift signal)
#
# A sustained upward RSS/goroutine/connection trend, any container restart, or a *decrease* in
# x*y across the window are failures. Output is a JSONL time series at $OUTDIR/soak-metrics.jsonl
# plus a before/after invariant pair, for attachment to the results report.
#
# Functions:
#   metrics_snapshot OUTDIR        — one snapshot line (JSONL) appended to soak-metrics.jsonl
#   metrics_collect_loop OUTDIR INTERVAL_SECS TOTAL_SECS  — snapshot every INTERVAL until TOTAL
#
# Env:
#   API_GW_URL   gateway for pool status (default http://localhost:18080)
#   PERF_PAIR    pool pair for the invariant (default W-BRL-ARS)
#   PG_CONTAINER postgres container name (default cbweb3-postgres)

: "${API_GW_URL:=http://localhost:18080}"
: "${PERF_PAIR:=W-BRL-ARS}"
: "${PG_CONTAINER:=cbweb3-postgres}"

# _pool_reserves -> "<reserve_a> <reserve_b>"
_pool_reserves() {
  body="$(curl -s --max-time 20 -H 'Accept: application/json' \
    "$API_GW_URL/api/v2/amm/pool/$PERF_PAIR/status" 2>/dev/null || true)"
  ra="$(printf '%s' "$body" | grep -o '"reserve_a":"[^"]*"' | sed 's/.*://;s/"//g')"
  rb="$(printf '%s' "$body" | grep -o '"reserve_b":"[^"]*"' | sed 's/.*://;s/"//g')"
  printf '%s %s' "${ra:-0}" "${rb:-0}"
}

# _pg_connections -> active connection count, or empty if not reachable.
_pg_connections() {
  command -v docker >/dev/null 2>&1 || return 0
  docker exec "$PG_CONTAINER" psql -U postgres -tAc \
    'SELECT count(*) FROM pg_stat_activity;' 2>/dev/null | tr -d '[:space:]'
}

# metrics_snapshot OUTDIR
metrics_snapshot() {
  outdir="$1"; mkdir -p "$outdir"
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  reserves="$(_pool_reserves)"; ra="${reserves%% *}"; rb="${reserves##* }"
  pg="$(_pg_connections)"

  # docker stats (RSS) + restart counts for cbweb3 containers, compacted to JSON arrays.
  stats="[]"; restarts="[]"
  if command -v docker >/dev/null 2>&1; then
    stats="$(docker stats --no-stream --format '{"name":"{{.Name}}","mem":"{{.MemUsage}}"}' \
      $(docker ps --format '{{.Names}}' | grep -E 'cbweb3|backend-' || true) 2>/dev/null \
      | paste -sd, - | sed 's/^/[/;s/$/]/')"
    [ -z "$stats" ] || [ "$stats" = "[]" ] || true
    restarts="$(docker ps -a --format '{{.Names}}' | grep -E 'cbweb3|backend-' \
      | while read -r c; do
          rc="$(docker inspect -f '{{.RestartCount}}' "$c" 2>/dev/null || echo 0)"
          printf '{"name":"%s","restarts":%s}' "$c" "${rc:-0}"
        done | paste -sd, - | sed 's/^/[/;s/$/]/')"
  fi
  [ -n "$stats" ] || stats="[]"
  [ -n "$restarts" ] || restarts="[]"

  printf '{"ts":"%s","reserve_a":"%s","reserve_b":"%s","pg_connections":"%s","containers":%s,"restarts":%s}\n' \
    "$ts" "$ra" "$rb" "${pg:-unknown}" "$stats" "$restarts" >> "$outdir/soak-metrics.jsonl"
  log_info "soak snapshot recorded" reserve_a="$ra" reserve_b="$rb" pg_connections="${pg:-unknown}"
}

# metrics_collect_loop OUTDIR INTERVAL_SECS TOTAL_SECS
metrics_collect_loop() {
  outdir="$1"; interval="${2:-300}"; total="${3:-43200}"
  deadline=$(( $(date +%s) + total ))
  log_info "starting soak metrics loop" interval="${interval}s" total="${total}s"
  while [ "$(date +%s)" -lt "$deadline" ]; do
    metrics_snapshot "$outdir"
    sleep "$interval"
  done
  metrics_snapshot "$outdir"
  log_info "soak metrics loop complete" out="$outdir/soak-metrics.jsonl"
}
