#!/usr/bin/env bash
#
# proxy-smoke.sh — single-host reverse-proxy smoke test (Scenario A).
#
# Brings up ONE entity (central-bank-brazil, mode: found) with the per-host Caddy
# reverse proxy ENABLED, so the portals + api-gateway are reached PATH-based on
# :80/:443 (no per-portal host ports). This exercises the proxy end-to-end on a single
# machine — routing, path stripping, cookie scoping, TLS, and the launcher at the root.
#
# Why one entity: the proxy is one container per host with a single site + one route
# fragment per scenario, so it maps to one entity per host (the multi-host model). Do
# NOT run this together with deploy-all.sh (many entities) on the same host.
#
# The browser reaches the proxy via `cb-brazil.localtest.me`, a public name that
# resolves to 127.0.0.1 — no /etc/hosts edit needed, as long as the browser runs on
# THIS machine. To also cover Scenario B behind the SAME proxy, run
# scenario-b/samples/proxy-smoke.sh (same frontendHost) — one proxy then serves /a/*
# and /b/*.
#
# Usage:
#   ./proxy-smoke.sh                 # build, start relay, deploy with self-signed TLS
#   ./proxy-smoke.sh --clean         # wipe docker (containers+volumes) + data first
#   PROXY_TLS_MODE=off ./proxy-smoke.sh   # plain HTTP on :80 (no cert warning)
#   CBWEB3_BIN=/path/to/cbweb3 ./proxy-smoke.sh   # reuse a prebuilt CLI binary
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"       # .../scenario-a/samples/proxy-smoke
SAMPLES_DIR="${SCRIPT_DIR}"                                      # (script lives in samples/)
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"                  # .../scenario-a
REPO_ROOT="$(cd "${SCENARIO_DIR}/.." && pwd)"                   # repo root
export CBWEB3_HOME="${SCENARIO_DIR}"

# TLS mode for the proxy step (see proxy/README.md). Default: internal (Caddy local CA,
# self-signed — one-time browser warning). `off` serves plain HTTP on :80 (no warning).
export PROXY_TLS_MODE="${PROXY_TLS_MODE:-internal}"

WORK_DIR="${SCRIPT_DIR}/work"       # relative node.dataDir resolves here (isolated state)
HOST="cb-brazil.localtest.me"

log() { printf '\n\033[1;36m[proxy-smoke] %s\033[0m\n' "$*"; }

if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes) and work dir…"
  docker rm -f "$(docker ps -aq)" 2>/dev/null || true
  docker volume rm "$(docker volume ls -q)" 2>/dev/null || true
  docker rm -f cbweb3-proxy 2>/dev/null || true
  rm -rf "${WORK_DIR}" 2>/dev/null || true
fi
mkdir -p "${WORK_DIR}"

# --- images (generic, built once) ---------------------------------------------
if ! docker image inspect cbweb3/proxy:local >/dev/null 2>&1; then
  log "building reverse-proxy image (cbweb3/proxy:local)…"
  ( cd "${REPO_ROOT}/proxy" && ./build.sh )
fi
if ! docker image inspect cbweb3/launcher:local >/dev/null 2>&1; then
  log "building launcher image (cbweb3/launcher:local)…"
  ( cd "${REPO_ROOT}/launcher" && ./build.sh )
fi

# --- CLI binary ---------------------------------------------------------------
BIN="${CBWEB3_BIN:-}"
if [[ -z "${BIN}" ]]; then
  BIN="${SAMPLES_DIR}/.cbweb3"
  log "building cbweb3 CLI → ${BIN}"
  ( cd "${SCENARIO_DIR}/toolkit" && go build -o "${BIN}" ./cmd/cbweb3 )
fi

field() { printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p"; }

apply() { # <label> <manifest>
  log "$1"
  local rc; set +e
  "${BIN}" apply -f "$2" -o yaml | while IFS= read -r line; do
    case "$line" in
      *'"action":"step_started"'*)   printf '    -> %s\n'      "$(field "$line" step)" ;;
      *'"action":"step_completed"'*) printf '    [ok]   %s\n'  "$(field "$line" step)" ;;
      *'"action":"step_skipped"'*)   printf '    [skip] %s\n'  "$(field "$line" step)" ;;
      *'"action":"step_detail"'*)    printf '       .. %s\n'   "$(field "$line" detail)" ;;
      *'"action":"step_failed"'*)    printf '    [FAIL] %s: %s\n' "$(field "$line" step)" "$(field "$line" error)" ;;
      *) printf '%s\n' "$line" ;;
    esac
  done
  rc=${PIPESTATUS[0]}; set -e
  [[ "${rc}" -eq 0 ]] || { log "apply failed (exit ${rc}) — see report above"; exit "${rc}"; }
}

# --- relay (hard prerequisite of register-relay) ------------------------------
log "starting relay (cacti)…"
bash "${SCENARIO_DIR}/provisioning/scripts/start-cacti.sh" >/dev/null
log "relay healthy at http://localhost:4000"

# --- deploy (run from work dir so relative dataDir isolates state) ------------
( cd "${WORK_DIR}" && apply "found central-bank-brazil behind the proxy" \
    "${SCRIPT_DIR}/central-bank-brazil.yaml" )

# --- summary ------------------------------------------------------------------
scheme="https"; [[ "${PROXY_TLS_MODE}" == "off" ]] && scheme="http"
log "up. Reach everything through the proxy on this host (no ports):"
cat <<EOF

  ${scheme}://${HOST}/              -> launcher (A/B landing page)
  ${scheme}://${HOST}/a/governance/ -> Governance
  ${scheme}://${HOST}/a/treasury/   -> Treasury
  ${scheme}://${HOST}/a/supervisor/ -> Supervisor
  ${scheme}://${HOST}/a/api/v1/     -> api-gateway

  '${HOST}' resolves to 127.0.0.1, so open these in a browser ON THIS MACHINE.
EOF
if [[ "${PROXY_TLS_MODE}" != "off" ]]; then
  cat <<'EOF'
  TLS is 'internal' (Caddy local CA), so the browser warns once — accept it, or trust
  the root: docker cp cbweb3-proxy:/data/caddy/pki/authorities/local/root.crt .
  For a warning-free HTTP run instead: PROXY_TLS_MODE=off ./proxy-smoke.sh
  To also serve Scenario B behind this same proxy: run scenario-b/samples/proxy-smoke.sh
EOF
fi
