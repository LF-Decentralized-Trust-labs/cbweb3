#!/usr/bin/env bash
#
# deploy-three.sh — bring up the full Scenario B sample via the cbweb3b CLI.
#
# Same flow as deploy-all.sh, extended with a third spoke (Colombia):
#   • hub       (found-hub) : hub-cbweb3
#   • Brazil    (spoke-brl) : central-bank-brazil    + bank-itau, bank-bradesco
#   • Argentina (spoke-ars) : central-bank-argentina + bank-galicia, bank-macro
#   • Colombia  (spoke-cop) : central-bank-colombia  + bank-bancolombia, bank-davivienda
#
# The Cacti relay is deployed EXTERNALLY (provisioning/scripts/start-cacti.sh)
# before any apply — its address reaches the toolkit via each manifest's
# spec.relay.endpoint (http://localhost:7000). The sovereign FX corridor
# (BRL<->ARS) is NOT opened here: each CB opens it at runtime from its governance
# portal (propose/confirm pair + cooperative liquidity), so provisioning never
# handles sovereign signing keys. Colombia simply never opens a corridor.
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
  docker network prune -f 2>/dev/null || true   # free address pools (one net per entity)
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

# --- relay (external; hard prerequisite of register-relay-spoke) --------------
# Deployed outside the toolkit and reached via each manifest's spec.relay.endpoint.
log "starting Cacti relay (external)…"
bash "${SCENARIO_DIR}/provisioning/scripts/start-cacti.sh"

# --- hub -----------------------------------------------------------------------
apply "Hub — found-hub hub-cbweb3" "${SCRIPT_DIR}/hub/hub-cbweb3.yaml"

# --- Brazil spoke (proposes the BRL<->ARS pair) -------------------------------
apply "Brazil — found-spoke central-bank-brazil (spoke-brl)" \
  "${SCRIPT_DIR}/brazil/central-bank-brazil.yaml" --spoke-rpc http://localhost:33645
apply "Brazil — join bank-itau"     "${SCRIPT_DIR}/brazil/bank-itau.yaml"     --spoke-rpc http://localhost:33646
apply "Brazil — join bank-bradesco" "${SCRIPT_DIR}/brazil/bank-bradesco.yaml" --spoke-rpc http://localhost:33647

# --- Argentina spoke (confirms the pair -> ACTIVE) ----------------------------
apply "Argentina — found-spoke central-bank-argentina (spoke-ars)" \
  "${SCRIPT_DIR}/argentina/central-bank-argentina.yaml" --spoke-rpc http://localhost:33745
apply "Argentina — join bank-galicia" "${SCRIPT_DIR}/argentina/bank-galicia.yaml" --spoke-rpc http://localhost:33746
apply "Argentina — join bank-macro"   "${SCRIPT_DIR}/argentina/bank-macro.yaml"   --spoke-rpc http://localhost:33747

# --- Colombia spoke (no sovereign pair) ---------------------------------------
apply "Colombia — found-spoke central-bank-colombia (spoke-cop)" \
  "${SCRIPT_DIR}/colombia/central-bank-colombia.yaml" --spoke-rpc http://localhost:33945
apply "Colombia — join bank-bancolombia" "${SCRIPT_DIR}/colombia/bank-bancolombia.yaml" --spoke-rpc http://localhost:33946
apply "Colombia — join bank-davivienda"  "${SCRIPT_DIR}/colombia/bank-davivienda.yaml"  --spoke-rpc http://localhost:33947

log "done. RPC ports: hub 8845 | BR 8645-8647 | AR 8745-8747 | CO 8945-8947"
log "bundles under samples/bundles/ ; per-entity state under samples/cbweb3-data/"

log "all stacks up. Endpoints (api-gateway + operator portals, on the host):"
cat <<'EOF'

  Hub (hub-cbweb3)
    hub            api http://localhost:41845   governance http://localhost:42845

  Brazil (spoke-brl)
    central-bank   api http://localhost:41645   governance http://localhost:42645
                                                 treasury   http://localhost:46645
                                                 supervisor http://localhost:47645
                                                 noc        http://localhost:45645
                                                 launcher   http://localhost:5191
    bank-itau      api http://localhost:41646   portal     http://localhost:42646
                                                 launcher   http://localhost:5192
    bank-bradesco  api http://localhost:41647   portal     http://localhost:42647
                                                 launcher   http://localhost:5193

  Argentina (spoke-ars)
    central-bank   api http://localhost:41745   governance http://localhost:42745
                                                 treasury   http://localhost:46745
                                                 supervisor http://localhost:47745
                                                 noc        http://localhost:45745
                                                 launcher   http://localhost:5194
    bank-galicia   api http://localhost:41746   portal     http://localhost:42746
                                                 launcher   http://localhost:5195
    bank-macro     api http://localhost:41747   portal     http://localhost:42747
                                                 launcher   http://localhost:5196

  Colombia (spoke-cop)
    central-bank   api http://localhost:41945   governance http://localhost:42945
                                                 treasury   http://localhost:46945
                                                 supervisor http://localhost:47945
                                                 noc        http://localhost:45945
                                                 launcher   http://localhost:5197
    bank-bancolombia api http://localhost:41946 portal     http://localhost:42946
                                                 launcher   http://localhost:5198
    bank-davivienda  api http://localhost:41947 portal     http://localhost:42947
                                                 launcher   http://localhost:5199

  The launcher (per entity) is the A/B entry point; it lists that entity's Scenario A
  and B portals. Build the image once: ( cd ../../launcher && ./build.sh ).
EOF

cat <<'EOF'

  Sovereign currencies (W-tCeBM_BRL, W-tCeBM_ARS, W-tCeBM_COP): already deployed
  and registered on the hub by found-spoke (via the hub compliance service) — no
  runtime step.

  Sovereign FX corridor (BRL<->ARS): opened at RUNTIME from the CB governance portal,
  not by this script. Once the stacks are up, each central bank uses its portal
  (Cooperative Liquidity wizard) or the v2 API to propose/confirm the pair and add
  liquidity — CB-role, authenticated, no raw keys:
    POST /api/v2/amm/pairs/propose · /api/v2/amm/pairs/confirm
    POST /api/v2/amm/liquidity/add
  (currencies are listable at GET /api/v2/hub/currencies)
  Colombia (spoke-cop) joins the hub but opens no corridor.
EOF
