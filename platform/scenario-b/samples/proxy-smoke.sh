#!/usr/bin/env bash
#
# proxy-smoke.sh — single-host reverse-proxy smoke test (Scenario B).
#
# Brings up the hub + ONE spoke (central-bank-brazil, found-spoke) with the per-host
# Caddy reverse proxy ENABLED on the CB, so its portals + api-gateway are reached
# PATH-based on :80/:443 (no per-portal host ports). Exercises the proxy end-to-end on
# a single machine — routing, path stripping, cookie scoping, TLS, launcher at root.
#
# Why one spoke: the proxy is one container per host with a single site + one route
# fragment per scenario. Do NOT run this together with deploy-all.sh on the same host.
#
# The browser reaches the proxy via `cb-brazil.localtest.me` (resolves to 127.0.0.1 —
# no /etc/hosts edit; browser must run on THIS machine). Run this with the SAME
# frontendHost as scenario-a/samples/proxy-smoke.sh and one proxy serves /a/* and /b/*.
#
# Usage:
#   ./proxy-smoke.sh                 # build, start relay, deploy hub + CB (self-signed TLS)
#   ./proxy-smoke.sh --clean         # wipe docker (containers+volumes+networks) + work dir
#   PROXY_TLS_MODE=off ./proxy-smoke.sh   # plain HTTP on :80 (no cert warning)
#   CBWEB3B_BIN=/path/to/cbweb3b ./proxy-smoke.sh   # reuse a prebuilt CLI binary
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"       # .../scenario-b/samples/proxy-smoke
SAMPLES_DIR="${SCRIPT_DIR}"
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"                  # .../scenario-b
REPO_ROOT="$(cd "${SCENARIO_DIR}/.." && pwd)"                   # repo root

export PROXY_TLS_MODE="${PROXY_TLS_MODE:-internal}"
HOST="cb-brazil.localtest.me"

log() { printf '\n\033[1;36m[proxy-smoke] %s\033[0m\n' "$*"; }

cd "${SCRIPT_DIR}"   # relative node.dataDir + emitted bundles resolve under proxy-smoke/

if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes + networks) and work dir…"
  docker rm -f "$(docker ps -aq)" 2>/dev/null || true
  docker volume rm "$(docker volume ls -q)" 2>/dev/null || true
  docker network prune -f 2>/dev/null || true
  docker rm -f cbweb3-proxy 2>/dev/null || true
  rm -rf "${SCRIPT_DIR}/cbweb3-data" "${SCRIPT_DIR}/bundles" 2>/dev/null || true
fi

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
BIN="${CBWEB3B_BIN:-}"
if [[ -z "${BIN}" ]]; then
  BIN="${SAMPLES_DIR}/.cbweb3b"
  log "building cbweb3b CLI → ${BIN}"
  ( cd "${SCENARIO_DIR}/toolkit" && go build -o "${BIN}" ./cmd/cbweb3b )
fi

# --- contract dependencies (Soldeer) — one-time, idempotent --------------------
if [[ ! -d "${SCENARIO_DIR}/contracts/dependencies" ]]; then
  log "installing contract dependencies (forge soldeer install)…"
  ( cd "${SCENARIO_DIR}/contracts" && forge soldeer install )
fi

apply() { # <label> <manifest> [extra flags...]
  local label="$1" manifest="$2"; shift 2
  log "${label}"
  "${BIN}" apply -f "${manifest}" -o yaml --repo-root "${REPO_ROOT}" --out-dir "${SCRIPT_DIR}" "$@"
}

# --- relay (external; hard prerequisite of register-relay-spoke) --------------
log "starting Cacti relay (external)…"
bash "${SCENARIO_DIR}/provisioning/scripts/start-cacti.sh"

# --- hub then CB (proxy enabled on the CB) ------------------------------------
apply "Hub — found-hub hub-cbweb3" "${SCENARIO_DIR}/samples/hub/hub-cbweb3.yaml"
apply "Brazil — found-spoke central-bank-brazil behind the proxy" \
  "${SCRIPT_DIR}/central-bank-brazil.yaml" --spoke-rpc http://localhost:33645

# --- summary ------------------------------------------------------------------
scheme="https"; [[ "${PROXY_TLS_MODE}" == "off" ]] && scheme="http"
log "up. Reach the CB spoke through the proxy on this host (no ports):"
cat <<EOF

  ${scheme}://${HOST}/              -> launcher (A/B landing page)
  ${scheme}://${HOST}/b/governance/ -> Governance
  ${scheme}://${HOST}/b/treasury/   -> Treasury
  ${scheme}://${HOST}/b/supervisor/ -> Supervisor
  ${scheme}://${HOST}/b/api/v1/     -> api-gateway

  '${HOST}' resolves to 127.0.0.1, so open these in a browser ON THIS MACHINE.
  The hub stays port-based (it is infra, not proxied): api http://localhost:41845.
EOF
if [[ "${PROXY_TLS_MODE}" != "off" ]]; then
  cat <<'EOF'
  TLS is 'internal' (Caddy local CA), so the browser warns once — accept it, or trust
  the root: docker cp cbweb3-proxy:/data/caddy/pki/authorities/local/root.crt .
  For a warning-free HTTP run instead: PROXY_TLS_MODE=off ./proxy-smoke.sh
  Run scenario-a/samples/proxy-smoke.sh with the same host to serve /a/* on this proxy too.
EOF
fi
