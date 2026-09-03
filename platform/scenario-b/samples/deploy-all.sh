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
# spec.relay.endpoint (http://localhost:7000). found-hub founds the hub (Besu +
# base contracts). Each central bank FOUNDS its spoke (CB is the sole QBFT
# validator), registers on the hub, and dynamically registers its spoke with the
# relay (POST /api/v1/spokes). Each commercial bank JOINS as a non-validating full
# node. The sovereign FX corridor (e.g. BRL<->ARS) is NOT opened here: each CB
# opens it at runtime from its governance portal (propose/confirm pair +
# cooperative liquidity), so provisioning never handles sovereign signing keys.
#
# Idempotent: re-running resumes from the first incomplete step per entity
# (per-entity state under cbweb3-data/<entity>).
#
# JWT validation (R2-H-6): each api-gateway auth service enforces both the token
# issuer (iss, derived from the entity/hub Keycloak URL + realm) and audience
# (aud). The toolkit provisions an oidc-audience-mapper on the spoke-backend /
# hub-backend login clients so tokens carry aud=cbweb3-backend, and wires
# KEYCLOAK_AUDIENCE to match — nothing to set here. NOC backends run
# NOC_SKIP_AUTH=true (unchanged).
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

# Per-entity signatures ENFORCED on the internal routes for this sample: the shared
# INTERNAL_RELAY_AUTH_SECRET is identical in every entity, so it cannot attribute a call, and every
# caller here signs (bank gateways, the payment proxy, the transfer-limit client and the Cacti relay).
#
# Safe to enforce from a clean deploy because of two properties: a gateway with this set and no pinned
# peer REFUSES TO START rather than answer 401 to everything, and an unknown key-id triggers one
# rate-limited registry reload before rejection — which is what makes a bank verifiable the moment it
# finishes onboarding instead of at the next periodic sweep.
#
# Override with RELAY_REQUIRE_SIGNATURE=  (empty) to reproduce the pre-enforcement behaviour.
export RELAY_REQUIRE_SIGNATURE="${RELAY_REQUIRE_SIGNATURE-true}"

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

# The relay was started BEFORE the hub (it is a hard prerequisite of register-relay-spoke), so it
# booted before found-hub generated its signing identity — a signer is read once, at construction.
# Restart it now so it picks the key up; without this it forwards the bridge-out leg unsigned, which
# the central banks reject once RELAY_REQUIRE_SIGNATURE is on.
log "restarting the Cacti relay so it picks up its signing identity…"
docker restart cbweb3-cacti-liquidity-relay >/dev/null 2>&1 || \
  log "WARNING: could not restart the relay — it will forward unsigned until restarted"

# --- Brazil spoke (spoke-brl) -------------------------------------------------
apply "Brazil — found-spoke central-bank-brazil (spoke-brl)" \
  "${SCRIPT_DIR}/brazil/central-bank-brazil.yaml" --spoke-rpc http://localhost:33645
apply "Brazil — join bank-itau"     "${SCRIPT_DIR}/brazil/bank-itau.yaml"     --spoke-rpc http://localhost:33646
apply "Brazil — join bank-bradesco" "${SCRIPT_DIR}/brazil/bank-bradesco.yaml" --spoke-rpc http://localhost:33647

# --- Argentina spoke (spoke-ars) ----------------------------------------------
apply "Argentina — found-spoke central-bank-argentina (spoke-ars)" \
  "${SCRIPT_DIR}/argentina/central-bank-argentina.yaml" --spoke-rpc http://localhost:33745
apply "Argentina — join bank-galicia" "${SCRIPT_DIR}/argentina/bank-galicia.yaml" --spoke-rpc http://localhost:33746
apply "Argentina — join bank-macro"   "${SCRIPT_DIR}/argentina/bank-macro.yaml"   --spoke-rpc http://localhost:33747

log "done. Node RPC ports: hub 33845 | brazil 33645/33646/33647 | argentina 33745/33746/33747"
log "bundles emitted under samples/bundles/ ; per-entity state under samples/cbweb3-data/"

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

  The launcher (per entity) is the A/B entry point; it lists that entity's Scenario A
  and B portals. Build the image once: ( cd ../../launcher && ./build.sh ).
EOF

cat <<'EOF'

  Sovereign currencies (W-tCeBM_BRL, W-tCeBM_ARS): already deployed and registered
  on the hub by found-spoke (via the hub compliance service) — no runtime step.

  Sovereign FX corridor (BRL<->ARS): opened at RUNTIME from the CB governance portal,
  not by this script. Once the stacks are up, each central bank uses its portal
  (Cooperative Liquidity wizard) or the v2 API to propose/confirm the pair and add
  liquidity — CB-role, authenticated, no raw keys:
    POST /api/v2/amm/pairs/propose · /api/v2/amm/pairs/confirm
    POST /api/v2/amm/liquidity/deposit-side  (each CB, its own side)
    POST /api/v2/amm/liquidity/finalize      (funds both reserves atomically)
  (currencies are listable at GET /api/v2/hub/currencies)
EOF
