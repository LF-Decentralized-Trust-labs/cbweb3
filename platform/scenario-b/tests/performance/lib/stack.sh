#!/usr/bin/env bash
# stack.sh — detect / bring up the Scenario B stack for the perf harness.
#
# Zero-config rule: if a stack is already serving at API_GW_URL we reuse it; only when
# nothing is reachable do we run `make scenario-b.up` (idempotent). The driver passes
#
# BROKEN since the legacy path was retired: `scenario-b.up` and the PERF_UP_TARGET default
# `scenario-b.up-perf` no longer exist. Stand the stack up with `cd samples && ./deploy-all.sh`
# before running the benchmarks. Tracked as DEF-022.
# SKIP_STACK=1 in dry-run / static-check mode so this never touches infra.
#
# Functions:
#   stack_gateway_ready URL TIMEOUT_SECS  -> 0 if healthy within timeout
#   stack_ensure_up                       -> reuse if up, else `make scenario-b.up`
#
# Env:
#   API_GW_URL                 commercial-bank gateway (default http://localhost:18080)
#   API_GW_CENTRAL_BANK_A_URL  CB-A gateway (for profile/seed); default http://localhost:38080
#   SKIP_STACK                 "1" to never start a stack (dry-run / CI parse)
#   PERF_MAKE_DIR              dir to run `make` from (default: scenario-b root)

# shellcheck source=./log.sh
: "${API_GW_URL:=http://localhost:18080}"
: "${SKIP_STACK:=0}"

# stack_gateway_ready URL [TIMEOUT_SECS]
stack_gateway_ready() {
  url="$1"; timeout="${2:-2}"
  # Gateways expose a health/readiness path; fall back to root. A 2xx/3xx/401 all mean
  # "process is serving" — 401 just means auth is required, which is fine for liveness.
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time "$timeout" "$url/health" 2>/dev/null || echo 000)"
  case "$code" in
    2??|3??|4??) return 0 ;;
  esac
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time "$timeout" "$url" 2>/dev/null || echo 000)"
  case "$code" in
    2??|3??|4??) return 0 ;;
    *) return 1 ;;
  esac
}

# stack_wait_ready URL TIMEOUT_SECS — poll until ready or timeout.
stack_wait_ready() {
  url="$1"; deadline=$(( $(date +%s) + ${2:-300} ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if stack_gateway_ready "$url" 3; then return 0; fi
    sleep 5
  done
  return 1
}

stack_ensure_up() {
  if [ "$SKIP_STACK" = "1" ]; then
    log_info "SKIP_STACK=1 — not touching infra (assuming caller-managed stack)"
    return 0
  fi
  if stack_gateway_ready "$API_GW_URL" 3; then
    log_info "stack already serving — reusing" url="$API_GW_URL"
    return 0
  fi
  require_cmd make
  makedir="${PERF_MAKE_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
  # Default to the perf-lean bring-up (full settlement stack minus the NOC portal, which
  # is a monitoring frontend not on the perf path). Override with PERF_UP_TARGET if needed.
  up_target="${PERF_UP_TARGET:-scenario-b.up-perf}"
  log_info "no stack at gateway — bringing it up" url="$API_GW_URL" dir="$makedir" target="$up_target"
  ( cd "$makedir" && make "$up_target" ) || log_fatal "stack bring-up failed" target="$up_target"
  log_info "waiting for gateway readiness (up to 10m)"
  stack_wait_ready "$API_GW_URL" 600 || log_fatal "gateway not ready after scenario-b.up" url="$API_GW_URL"
  log_info "stack is up and ready" url="$API_GW_URL"
}
