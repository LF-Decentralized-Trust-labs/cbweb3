#!/usr/bin/env bash
# stack.sh — detect (and optionally provision) the Scenario B stack for the perf harness.
#
# Reuse first: if a stack is already serving at API_GW_URL we use it and touch nothing.
#
# When nothing is reachable, the harness does NOT provision by default. It used to run
# `make scenario-b.up`, a fixed local topology that the legacy-path retirement deleted, and
# the toolkit is not a drop-in replacement for it: `deploy-all.sh` provisions every entity
# and, with --clean, wipes docker containers, volumes and data dirs host-wide. A benchmark
# that quietly did that could destroy an environment someone else was using. So provisioning
# is opt-in, and the default is to stop with an actionable message.
#
# Set PERF_ALLOW_PROVISION=1 to let the harness stand a stack up through the toolkit. It runs
# `samples/deploy-all.sh` WITHOUT --clean, so an existing environment is converged, never
# wiped. Wiping stays a deliberate act the operator performs.
#
# Endpoints are not hardcoded here: the make targets derive them from the toolkit manifests
# via tests/integration/toolkit-env.sh, the same source the integration suite uses.
#
# Functions:
#   stack_gateway_ready URL TIMEOUT_SECS  -> 0 if healthy within timeout
#   stack_ensure_up                       -> reuse if up; else provision (opt-in) or fail clearly
#
# Env:
#   API_GW_URL                 commercial-bank gateway (derived from the manifests)
#   API_GW_CENTRAL_BANK_A_URL  CB-A gateway (for profile/seed); derived likewise
#   SKIP_STACK                 "1" to never start a stack (dry-run / CI parse)
#   PERF_ALLOW_PROVISION       "1" to allow toolkit provisioning when nothing is reachable
#   PERF_SAMPLES_DIR           dir holding deploy-all.sh (default: scenario-b/samples)

# shellcheck source=./log.sh
# No legacy default: the make targets export API_GW_URL from the toolkit manifests
# (tests/integration/toolkit-env.sh). A hardcoded 18080 pointed at the retired topology.
: "${API_GW_URL:?API_GW_URL is required — derive it with tests/integration/toolkit-env.sh}"
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

  if [ "${PERF_ALLOW_PROVISION:-0}" != "1" ]; then
    # Not a missing-make-target error, which is what this used to produce and which told
    # the operator nothing about what to do.
    log_error "no stack answering at the gateway" url="$API_GW_URL"
    log_error "the perf harness does not provision by default: the toolkit deploys every entity on this host"
    log_error "stand one up yourself:  cd scenario-b/samples && ./deploy-all.sh"
    log_error "or re-run with PERF_ALLOW_PROVISION=1 to let the harness do it (never passes --clean)"
    log_fatal "no stack to benchmark" url="$API_GW_URL"
  fi

  require_cmd bash
  samples="${PERF_SAMPLES_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../samples" && pwd)}"
  [ -x "${samples}/deploy-all.sh" ] || log_fatal "deploy-all.sh not found or not executable" dir="$samples"
  # No --clean. Converging an existing environment is recoverable; wiping one is not, and a
  # benchmark is the wrong place to make that call.
  log_info "PERF_ALLOW_PROVISION=1 — provisioning through the toolkit" dir="$samples"
  ( cd "$samples" && ./deploy-all.sh ) || log_fatal "toolkit provisioning failed" dir="$samples"
  log_info "waiting for gateway readiness (up to 10m)"
  stack_wait_ready "$API_GW_URL" 600 || log_fatal "gateway not ready after provisioning" url="$API_GW_URL"
  log_info "stack is up and ready" url="$API_GW_URL"
}
