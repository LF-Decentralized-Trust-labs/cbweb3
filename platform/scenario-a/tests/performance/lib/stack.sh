#!/usr/bin/env bash
# stack.sh — detect whether the Scenario A stack is up; stand it up if not.
#
# "Up" = the bank-a API gateway answers on $PERF_GW_URL. If it does not, run
# `make spoke-a` from scenario-a/ (the full Besu QBFT + Paladin + backend stack).
# spoke-a is sufficient for every threshold benchmark (transfer, zeto, baseline,
# TTF); the cross-spoke (spoke-b) and Cacti relay legs are only needed for the
# optional cross-spoke TTF note, so the default does NOT bring up the whole world.
#
# Functions:
#   perf_gw_healthy   GW_URL        -> 0 if gateway answers, else 1
#   perf_wait_gw      GW_URL SECS   -> poll until healthy or timeout
#   perf_ensure_stack GW_URL        -> ensure the gateway is reachable (auto up)

# perf_gw_healthy GW_URL
# A reachable gateway is enough; /auth/login responds even without creds (400),
# so any HTTP status < 500 from the API root counts as "process is serving".
perf_gw_healthy() {
  local gw="$1" code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 \
    "${gw}/api/v1/auth/login" -X POST -H 'Content-Type: application/json' -d '{}' 2>/dev/null) || return 1
  case "$code" in
    000) return 1 ;;          # no connection
    5*)  return 1 ;;          # serving but unhealthy
    *)   return 0 ;;          # 2xx/3xx/4xx — process is up and routing
  esac
}

# perf_wait_gw GW_URL TIMEOUT_SECS
perf_wait_gw() {
  local gw="$1" timeout="${2:-300}" waited=0
  while [ "$waited" -lt "$timeout" ]; do
    if perf_gw_healthy "$gw"; then
      log_info "gateway healthy" gw="$gw" waited_s="$waited"
      return 0
    fi
    sleep 5
    waited=$((waited + 5))
  done
  log_error "gateway did not become healthy" gw="$gw" timeout_s="$timeout"
  return 1
}

# perf_ensure_stack GW_URL
# Brings the stack up via `make spoke-a` only if the gateway is not already
# reachable. Honours PERF_DRY_RUN (skips the actual `make`).
perf_ensure_stack() {
  local gw="$1"
  if perf_gw_healthy "$gw"; then
    log_info "stack already up — reusing" gw="$gw"
    return 0
  fi
  log_info "stack not detected — bringing up spoke-a" gw="$gw"
  if [ "${PERF_DRY_RUN:-0}" = "1" ]; then
    log_info "dry-run: would run 'make spoke-a'"
    return 0
  fi
  if ! make spoke-a; then
    log_error "make spoke-a failed"
    return 1
  fi
  perf_wait_gw "$gw" "${PERF_STACK_TIMEOUT:-600}"
}
