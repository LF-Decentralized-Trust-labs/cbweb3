#!/usr/bin/env bash
#
# deploy-three.sh — bring up the full Scenario B sample via the cbweb3b CLI.
#
# Same flow as deploy-all.sh, extended with a third spoke (Colombia) that joins
# the hub WITHOUT opening a sovereign corridor (a plain found-spoke):
#   • hub       (found-hub) : hub-cbweb3
#   • Brazil    (spoke-brl) : central-bank-brazil    + bank-itau, bank-bradesco     [pair BRL<->ARS: proposer]
#   • Argentina (spoke-ars) : central-bank-argentina + bank-galicia, bank-macro     [pair BRL<->ARS: confirmer]
#   • Colombia  (spoke-cop) : central-bank-colombia  + bank-bancolombia, bank-davivienda  [no pair]
#
# Idempotent: re-running resumes from the first incomplete step per entity.
#
# Usage:
#   ./deploy-three.sh              # build CLI, deploy all three spokes
#   ./deploy-three.sh --clean      # wipe docker (containers+volumes) + data dirs first
#   CBWEB3B_BIN=/path/to/cbweb3b ./deploy-three.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"    # .../scenario-b/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"                # .../scenario-b
REPO_ROOT="$(cd "${SCENARIO_DIR}/.." && pwd)"                 # repo root (--repo-root)

cd "${SCRIPT_DIR}"

log() { printf '\n\033[1;36m[deploy] %s\033[0m\n' "$*"; }

if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes) and data dirs…"
  docker rm -f $(docker ps -aq) 2>/dev/null || true
  docker volume rm $(docker volume ls -q) 2>/dev/null || true
  rm -rf "${SCRIPT_DIR}/cbweb3-data" "${SCRIPT_DIR}/bundles" 2>/dev/null || true
fi

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

apply() {
  local label="$1" manifest="$2"; shift 2
  log "${label}"
  "${BIN}" apply -f "${manifest}" -o yaml \
    --repo-root "${REPO_ROOT}" \
    --out-dir "${SCRIPT_DIR}" \
    "$@"
}

# --- hub (starts the shared relay) --------------------------------------------
apply "Hub — found-hub hub-cbweb3" "${SCRIPT_DIR}/hub/hub-cbweb3.yaml"

# --- Brazil spoke (proposes the BRL<->ARS pair) -------------------------------
apply "Brazil — found-spoke central-bank-brazil (spoke-brl)" \
  "${SCRIPT_DIR}/brazil/central-bank-brazil.yaml" --spoke-rpc http://localhost:8645
apply "Brazil — join bank-itau"     "${SCRIPT_DIR}/brazil/bank-itau.yaml"     --spoke-rpc http://localhost:8646
apply "Brazil — join bank-bradesco" "${SCRIPT_DIR}/brazil/bank-bradesco.yaml" --spoke-rpc http://localhost:8647

# --- Argentina spoke (confirms the pair -> ACTIVE) ----------------------------
apply "Argentina — found-spoke central-bank-argentina (spoke-ars)" \
  "${SCRIPT_DIR}/argentina/central-bank-argentina.yaml" --spoke-rpc http://localhost:8745
apply "Argentina — join bank-galicia" "${SCRIPT_DIR}/argentina/bank-galicia.yaml" --spoke-rpc http://localhost:8746
apply "Argentina — join bank-macro"   "${SCRIPT_DIR}/argentina/bank-macro.yaml"   --spoke-rpc http://localhost:8747

# --- Colombia spoke (no sovereign pair) ---------------------------------------
apply "Colombia — found-spoke central-bank-colombia (spoke-cop)" \
  "${SCRIPT_DIR}/colombia/central-bank-colombia.yaml" --spoke-rpc http://localhost:8945
apply "Colombia — join bank-bancolombia" "${SCRIPT_DIR}/colombia/bank-bancolombia.yaml" --spoke-rpc http://localhost:8946
apply "Colombia — join bank-davivienda"  "${SCRIPT_DIR}/colombia/bank-davivienda.yaml"  --spoke-rpc http://localhost:8947

log "done. RPC ports: hub 8845 | BR 8645-8647 | AR 8745-8747 | CO 8945-8947"
log "bundles under samples/bundles/ ; per-entity state under samples/cbweb3-data/"
