#!/usr/bin/env bash
#
# deploy-all.sh — bring up the core Scenario B sample via the cbweb3b CLI.
#
# Topology: one neutral hub + two sovereign spokes forming the BRL<->ARS corridor:
#   • hub       (found-hub)   : hub-cbweb3         — base contracts + relay + NOC
#   • Brazil    (spoke-brl)   : central-bank-brazil    + bank-itau, bank-bradesco
#   • Argentina (spoke-ars)   : central-bank-argentina + bank-galicia, bank-macro
#
# The Cacti relay is deployed EXTERNALLY (provisioning/scripts/start-cacti.sh)
# before any apply — its address reaches the toolkit via each manifest's
# spec.relay.endpoint (http://localhost:4000). found-hub founds the hub (Besu +
# base contracts). Each central bank FOUNDS its spoke (CB is the sole QBFT
# validator), registers on the hub, and dynamically registers its spoke with the
# relay (POST /api/v1/spokes). Each commercial bank JOINS as a non-validating full
# node. The sovereign FX corridor (spec.pair, e.g. BRL<->ARS) is NOT opened here:
# each CB opens it at runtime from its governance portal (propose/confirm pair +
# cooperative liquidity), so provisioning never handles sovereign signing keys.
#
# Idempotent: re-running resumes from the first incomplete step per entity
# (per-entity state under cbweb3-data/<entity>).
#
# Usage:
#   ./deploy-all.sh              # build CLI, deploy hub + Brazil + Argentina
#   ./deploy-all.sh --clean      # wipe docker (containers+volumes) + data dirs first
#   CBWEB3B_BIN=/path/to/cbweb3b ./deploy-all.sh   # reuse a prebuilt CLI binary
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"    # .../scenario-b/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"                # .../scenario-b
REPO_ROOT="$(cd "${SCENARIO_DIR}/.." && pwd)"                 # repo root (--repo-root)

cd "${SCRIPT_DIR}"   # relative node.dataDir -> samples/cbweb3-data/<entity>

log() { printf '\n\033[1;36m[deploy] %s\033[0m\n' "$*"; }

# --- optional clean -----------------------------------------------------------
if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes) and data dirs…"
  docker rm -f $(docker ps -aq) 2>/dev/null || true
  docker volume rm $(docker volume ls -q) 2>/dev/null || true
  docker network prune -f 2>/dev/null || true   # free address pools (one net per entity)
  rm -rf "${SCRIPT_DIR}/cbweb3-data" "${SCRIPT_DIR}/bundles" 2>/dev/null || true
fi

# --- CLI binary ---------------------------------------------------------------
BIN="${CBWEB3B_BIN:-}"
if [[ -z "${BIN}" ]]; then
  BIN="${SCRIPT_DIR}/.cbweb3b"
  log "building cbweb3b CLI → ${BIN}"
  ( cd "${SCENARIO_DIR}/toolkit" && go build -o "${BIN}" ./cmd/cbweb3b )
fi

# --- contract dependencies (Soldeer) — one-time, idempotent --------------------
# build-contracts runs `forge build`, which needs contracts/dependencies/ present.
if [[ ! -d "${SCENARIO_DIR}/contracts/dependencies" ]]; then
  log "installing contract dependencies (forge soldeer install)…"
  ( cd "${SCENARIO_DIR}/contracts" && forge soldeer install )
fi

# apply <label> <manifest> [extra cbweb3b flags...]
# Bundles are emitted under --out-dir=samples (i.e. samples/bundles/), which the
# manifests reference as ../bundles/. Aborts on a non-zero exit (report shown).
apply() {
  local label="$1" manifest="$2"; shift 2
  log "${label}"
  "${BIN}" apply -f "${manifest}" -o yaml \
    --repo-root "${REPO_ROOT}" \
    --out-dir "${SCRIPT_DIR}" \
    "$@"
}

# --- relay (external; hard prerequisite of register-relay-spoke) --------------
# Deployed outside the toolkit and reached via each manifest's spec.relay.endpoint.
log "starting Cacti relay (external)…"
bash "${SCENARIO_DIR}/provisioning/scripts/start-cacti.sh"

# --- hub -----------------------------------------------------------------------
apply "Hub — found-hub hub-cbweb3" "${SCRIPT_DIR}/hub/hub-cbweb3.yaml"

# --- Brazil spoke (spoke-brl) -------------------------------------------------
apply "Brazil — found-spoke central-bank-brazil (spoke-brl)" \
  "${SCRIPT_DIR}/brazil/central-bank-brazil.yaml" --spoke-rpc http://localhost:8645
apply "Brazil — join bank-itau"     "${SCRIPT_DIR}/brazil/bank-itau.yaml"     --spoke-rpc http://localhost:8646
apply "Brazil — join bank-bradesco" "${SCRIPT_DIR}/brazil/bank-bradesco.yaml" --spoke-rpc http://localhost:8647

# --- Argentina spoke (spoke-ars) ----------------------------------------------
apply "Argentina — found-spoke central-bank-argentina (spoke-ars)" \
  "${SCRIPT_DIR}/argentina/central-bank-argentina.yaml" --spoke-rpc http://localhost:8745
apply "Argentina — join bank-galicia" "${SCRIPT_DIR}/argentina/bank-galicia.yaml" --spoke-rpc http://localhost:8746
apply "Argentina — join bank-macro"   "${SCRIPT_DIR}/argentina/bank-macro.yaml"   --spoke-rpc http://localhost:8747

log "done. Node RPC ports: hub 8845 | brazil 8645/8646/8647 | argentina 8745/8746/8747"
log "bundles emitted under samples/bundles/ ; per-entity state under samples/cbweb3-data/"
cat <<'EOF'

  Sovereign corridor (BRL<->ARS): opened at RUNTIME from the CB governance portal,
  not by this script. Once the stacks are up, each central bank uses its portal
  (Cooperative Liquidity wizard) or the v2 API to register its currency, propose/
  confirm the pair, and add liquidity — CB-role, authenticated, no raw keys:
    POST /api/v2/hub/currencies · /api/v2/amm/pairs/propose · /confirm
    POST /api/v2/amm/liquidity/add

  Verify block height per node:
    for p in 8845 8645 8646 8647 8745 8746 8747; do
      echo -n "port $p: "
      curl -s -X POST "http://localhost:$p" -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result
    done
EOF
