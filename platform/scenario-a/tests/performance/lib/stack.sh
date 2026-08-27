#!/usr/bin/env bash
# stack.sh — detect (and optionally provision) the Scenario A stack for the perf harness.
#
# "Up" = the originator bank's API gateway answers on the URL it is given.
#
# Reuse first, and when nothing is reachable do NOT provision by default. This used to run
# `make spoke-a` / `make spoke-all`, a fixed local topology the legacy-path retirement
# deleted. The toolkit is not a drop-in replacement: `deploy-all.sh` provisions every entity
# and, with --clean, wipes docker containers, volumes and data dirs host-wide. A benchmark
# that quietly did that could destroy an environment someone else was using.
#
# Set PERF_ALLOW_PROVISION=1 to let the harness stand a stack up through the toolkit. It runs
# `samples/deploy-all.sh` WITHOUT --clean, so an existing environment is converged, never
# wiped. Wiping stays a deliberate act the operator performs.
#
# The distinction the old targets encoded is preserved: most benchmarks need only the
# originator spoke, while the FX-settlement happy path spans both spokes plus the Cacti
# relay. That is now a difference in which gateways are PROBED, not in which target is run —
# the toolkit deploys the whole sample topology in one go.
#
# Functions:
#   perf_gw_healthy       GW_URL              -> 0 if gateway answers, else 1
#   perf_wait_gw          GW_URL SECS         -> poll until healthy or timeout
#   perf_ensure_stack     GW_URL              -> ensure the originator gateway is reachable
#   perf_ensure_full_stack ORIG_GW CUST_GW    -> ensure both gateways are reachable

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
# Ensures the originator gateway is reachable, provisioning only if the operator opted in
# reachable. Honours PERF_DRY_RUN (skips the actual `make`).
perf_ensure_stack() {
  local gw="$1"
  if perf_gw_healthy "$gw"; then
    log_info "stack already up — reusing" gw="$gw"
    return 0
  fi
  if [ "${PERF_DRY_RUN:-0}" = "1" ]; then
    log_info "dry-run: would provision through the toolkit (opt-in)" gw="$gw"
    return 0
  fi
  perf_provision_or_explain "$gw" || return 1
  perf_wait_gw "$gw" "${PERF_STACK_TIMEOUT:-600}"
}

# perf_ensure_full_stack ORIGINATOR_GW CUSTODIAN_GW
# Ensures the FULL cross-spoke stack (both spokes + custodian bank-d + Cacti relay)
# Required by the FX-settlement happy-path benchmark, which
# spans spoke-a (originator) and spoke-b (custodian) with the relay between them.
# Reuses a stack that is already fully up (both gateways answer). Honours PERF_DRY_RUN.
perf_ensure_full_stack() {
  local ogw="$1" cgw="$2"
  if perf_gw_healthy "$ogw" && perf_gw_healthy "$cgw"; then
    log_info "full cross-spoke stack already up — reusing" originator="$ogw" custodian="$cgw"
    return 0
  fi
  if [ "${PERF_DRY_RUN:-0}" = "1" ]; then
    log_info "dry-run: would provision through the toolkit (opt-in)" originator="$ogw" custodian="$cgw"
    return 0
  fi
  perf_provision_or_explain "$ogw" || return 1
  perf_wait_gw "$ogw" "${PERF_STACK_TIMEOUT:-900}" || return 1
  perf_wait_gw "$cgw" "${PERF_STACK_TIMEOUT:-900}"
}

# perf_provision_or_explain GW_URL
# Shared by both ensure functions: provision through the toolkit when the operator opted in,
# otherwise stop with a message that says what to do. The old code produced a missing-make-
# target error, which told the reader nothing about the cause or the remedy.
perf_provision_or_explain() {
  local gw="$1" samples
  if [ "${PERF_ALLOW_PROVISION:-0}" != "1" ]; then
    log_error "no stack answering at the gateway" gw="$gw"
    log_error "the perf harness does not provision by default: the toolkit deploys every entity on this host"
    log_error "stand one up yourself:  cd scenario-a/samples && ./deploy-all.sh"
    log_error "or re-run with PERF_ALLOW_PROVISION=1 to let the harness do it (never passes --clean)"
    return 1
  fi
  samples="${PERF_SAMPLES_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../samples" && pwd)}"
  if [ ! -x "${samples}/deploy-all.sh" ]; then
    log_error "deploy-all.sh not found or not executable" dir="$samples"
    return 1
  fi
  # No --clean: converging an existing environment is recoverable, wiping one is not.
  log_info "PERF_ALLOW_PROVISION=1 — provisioning through the toolkit" dir="$samples"
  ( cd "$samples" && ./deploy-all.sh ) || { log_error "toolkit provisioning failed" dir="$samples"; return 1; }
}
