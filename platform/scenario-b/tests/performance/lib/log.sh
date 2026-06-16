#!/usr/bin/env bash
# log.sh — structured JSON logging for the R1-12.3 perf harness (constitution VI).
#
# Every helper sources this and emits one JSON object per line to stdout with:
#   ts (ISO-8601 UTC), service, severity, msg [, extra fields]
# Silent failures are prohibited; use log_fatal to abort loudly.
#
# Usage:
#   . "$(dirname "$0")/lib/log.sh"
#   PERF_SERVICE=auth.sh
#   log_info "minting token" bank=bank-a
#   log_fatal "no stack" url="$API_GW_URL"

PERF_SERVICE="${PERF_SERVICE:-perf}"

_log_ts() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }

# _log SEVERITY MSG [k=v ...]
_log() {
  severity="$1"; shift
  msg="$1"; shift
  extra=""
  for kv in "$@"; do
    key="${kv%%=*}"
    val="${kv#*=}"
    # JSON-escape backslashes and double quotes in the value.
    val="${val//\\/\\\\}"
    val="${val//\"/\\\"}"
    extra="$extra,\"$key\":\"$val\""
  done
  # Escape the message too.
  msg="${msg//\\/\\\\}"
  msg="${msg//\"/\\\"}"
  printf '{"ts":"%s","service":"%s","severity":"%s","msg":"%s"%s}\n' \
    "$(_log_ts)" "$PERF_SERVICE" "$severity" "$msg" "$extra"
}

log_info()  { _log INFO  "$@"; }
log_warn()  { _log WARN  "$@" >&2; }
log_error() { _log ERROR "$@" >&2; }

# log_fatal MSG [k=v ...] — log at ERROR and exit 1.
log_fatal() { _log ERROR "$@" >&2; exit 1; }

# require_cmd CMD — fail loudly if a binary is missing.
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || log_fatal "required command not found" cmd="$1"
}
