#!/usr/bin/env bash
# log.sh — structured JSON logging + run-id / artifact-dir helpers for the
# R1-12.3 zero-config performance harness.
#
# Source this from the driver; do not execute directly.
#
# Emits one JSON object per line to stdout (Constitution VI — Observability):
#   {"ts":"<ISO-8601>","level":"info","service":"perf-harness","msg":"...","<k>":<v>...}
#
# Exposes:
#   log_info  MSG [k=v ...]
#   log_warn  MSG [k=v ...]
#   log_error MSG [k=v ...]
#   perf_run_id              -> echoes a stable run id for this process
#   perf_artifact_dir        -> echoes (and mkdir -p's) the artifact dir for this run

set -o pipefail

# ISO-8601 UTC timestamp with seconds precision.
_perf_ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }

# _perf_log LEVEL MSG [k=v ...] — build a JSON line with jq so values are escaped.
_perf_log() {
  local level="$1" msg="$2"
  shift 2 || true
  local jq_filter='{ts:$ts, level:$level, service:"perf-harness", msg:$msg}'
  local -a jq_args=(--arg ts "$(_perf_ts)" --arg level "$level" --arg msg "$msg")
  local kv k v
  for kv in "$@"; do
    k="${kv%%=*}"
    v="${kv#*=}"
    jq_args+=(--arg "$k" "$v")
    jq_filter="${jq_filter} + {\"${k}\": \$${k}}"
  done
  if command -v jq >/dev/null 2>&1; then
    jq -nc "${jq_args[@]}" "$jq_filter"
  else
    # Fallback: jq missing (should not happen — it is a declared dep).
    echo "{\"ts\":\"$(_perf_ts)\",\"level\":\"$level\",\"service\":\"perf-harness\",\"msg\":\"$msg\"}"
  fi
}

log_info() { _perf_log info "$@"; }
log_warn() { _perf_log warn "$@" >&2; }
log_error() { _perf_log error "$@" >&2; }

# Stable run id for the lifetime of the process (UTC compact + pid).
perf_run_id() {
  if [ -z "${PERF_RUN_ID:-}" ]; then
    PERF_RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
    export PERF_RUN_ID
  fi
  echo "$PERF_RUN_ID"
}

# Artifact directory for this run (created on demand).
perf_artifact_dir() {
  local base="${PERF_ARTIFACT_BASE:-tests/performance/.artifacts}"
  local dir="${base}/$(perf_run_id)"
  mkdir -p "$dir"
  echo "$dir"
}
