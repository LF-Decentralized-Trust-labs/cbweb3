#!/usr/bin/env bash
#
# deploy-three.sh — bring up THREE Scenario A spokes via the cbweb3 CLI.
#
# Same flow as deploy-all.sh, extended with a third spoke: once the Colombia
# deploy finishes, the Argentina spoke is founded and its commercial banks join.
#   • Brazil   (spoke-brl): central-bank-brazil    + bank-itau, bank-bradesco
#   • Colombia (spoke-cop): central-bank-colombia  + bank-bancolombia, bank-davivienda
#   • Argentina(spoke-ars): central-bank-argentina + bank-galicia, bank-macro
#
# Each central bank FOUNDS its spoke (Besu+Paladin, contracts, relay registration,
# operational backend + frontend portals) and emits a join bundle; each commercial
# bank JOINS as a full node and brings up its own operational stack.
#
# Idempotent: re-running resumes from the first incomplete step per entity.
#
# Usage:
#   ./deploy-three.sh              # build CLI, start relay, deploy everything
#   ./deploy-three.sh --clean      # wipe docker (containers+volumes) + data dirs first
#   CBWEB3_BIN=/path/to/cbweb3 ./deploy-three.sh   # reuse a prebuilt CLI binary
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # .../scenario-a/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"               # .../scenario-a
export CBWEB3_HOME="${SCENARIO_DIR}"
# All entities share ONE Docker host here, so their per-entity container-alias
# advertisedHosts are not externally routable: force the container-name Paladin
# transport + derived gRPC ports so they do not collide on the fixed peer port 9000.
export CBWEB3_SINGLE_HOST=1

BUNDLES_DIR="${SCRIPT_DIR}/bundles"
# The toolkit emits bundles to <outputDir>/bundles, where outputDir defaults to the
# parent of the manifest's node.dataDir. The sample manifests use a relative
# node.dataDir (cbweb3-data/<entity>), which the CLI resolves against its CWD — so
# data and bundles land under ${PWD}/cbweb3-data here. Override with CBWEB3_OUTPUT_DIR.
DATA_ROOT="${CBWEB3_OUTPUT_DIR:-${PWD}/cbweb3-data}"
DATA_BUNDLES="${DATA_ROOT}/bundles"

log() { printf '\n\033[1;36m[deploy] %s\033[0m\n' "$*"; }

# --- optional clean -----------------------------------------------------------
if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes) and data dirs…"
  docker rm -f $(docker ps -aq) 2>/dev/null || true
  docker volume rm $(docker volume ls -q) 2>/dev/null || true
  # Data lives under the user-owned ${DATA_ROOT} (no privileged /opt path, no sudo).
  rm -rf "${DATA_ROOT}" 2>/dev/null || true
fi

# --- CLI binary ---------------------------------------------------------------
BIN="${CBWEB3_BIN:-}"
if [[ -z "${BIN}" ]]; then
  BIN="${SCRIPT_DIR}/.cbweb3"
  log "building cbweb3 CLI → ${BIN}"
  ( cd "${SCENARIO_DIR}/toolkit" && go build -o "${BIN}" ./cmd/cbweb3 )
fi

# field <json-line> <key> — extract a string value from a flat JSON log line.
field() { printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p"; }

apply() { # <label> <manifest>
  log "$1"
  # The engine streams its step lifecycle as structured JSON to stdout in real time
  # (Go's stdout is unbuffered), then prints the final report at the end. Humanize the
  # lifecycle events as they arrive so the terminal shows live progress, and pass the
  # report (and anything unrecognized) through verbatim. Do NOT swallow output here:
  # a previous version grep-filtered stdout, hiding both progress and errors.
  local rc
  set +e
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
  rc=${PIPESTATUS[0]}
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    log "apply failed for $(basename "$2") (exit ${rc}) — see the report above; aborting."
    exit "${rc}"
  fi
}

copy_bundle() { # <spoke-id>
  # The joining banks reference ../bundles/<spoke>.bundle.yaml (i.e. samples/bundles),
  # which is gitignored and therefore absent on a clean checkout — create it first.
  mkdir -p "${BUNDLES_DIR}"
  cp "${DATA_BUNDLES}/$1.bundle.yaml" "${BUNDLES_DIR}/"
  log "bundle $1 → samples/bundles/"
}

# --- relay (hard prerequisite of the register-relay step) ---------------------
log "starting relay (cacti)…"
bash "${SCENARIO_DIR}/provisioning/scripts/start-cacti.sh" >/dev/null
log "relay healthy at http://localhost:4000"

# --- Brazil spoke -------------------------------------------------------------
apply "Brazil — found central-bank-brazil (spoke-brl)" "${SCRIPT_DIR}/brazil/central-bank-brazil.yaml"
copy_bundle spoke-brl
apply "Brazil — join bank-itau"     "${SCRIPT_DIR}/brazil/bank-itau.yaml"
apply "Brazil — join bank-bradesco" "${SCRIPT_DIR}/brazil/bank-bradesco.yaml"

# --- Colombia spoke -----------------------------------------------------------
# Every founding central bank defaults its Paladin to host port 31648; on a single
# host each additional spoke must use a distinct port. (Besu/backend/frontend ports are
# already derived per-entity from each manifest's Besu RPC port.)
export CBWEB3_PALADIN_CB_URL="http://localhost:31748"
apply "Colombia — found central-bank-colombia (spoke-cop)" "${SCRIPT_DIR}/colombia/central-bank-colombia.yaml"
copy_bundle spoke-cop
apply "Colombia — join bank-bancolombia" "${SCRIPT_DIR}/colombia/bank-bancolombia.yaml"
apply "Colombia — join bank-davivienda"  "${SCRIPT_DIR}/colombia/bank-davivienda.yaml"

# --- Argentina spoke ----------------------------------------------------------
# Third spoke, founded once Colombia finishes. Its Paladin needs yet another
# distinct host port (31648 → 31748 → 31848).
export CBWEB3_PALADIN_CB_URL="http://localhost:31848"
apply "Argentina — found central-bank-argentina (spoke-ars)" "${SCRIPT_DIR}/argentina/central-bank-argentina.yaml"
copy_bundle spoke-ars
apply "Argentina — join bank-galicia" "${SCRIPT_DIR}/argentina/bank-galicia.yaml"
apply "Argentina — join bank-macro"   "${SCRIPT_DIR}/argentina/bank-macro.yaml"

# --- summary ------------------------------------------------------------------
log "all samples deployed. Endpoints (api-gateway /healthz, portals on /):"
cat <<'EOF'

  Brazil (spoke-brl)
    central-bank   api http://localhost:18645   governance http://localhost:25645
                                                 treasury   http://localhost:26645
                                                 supervisor http://localhost:30645
                                                 noc        http://localhost:32645
                                                 launcher   http://localhost:5191
    bank-itau      api http://localhost:18646   portal     http://localhost:25646
                                                 launcher   http://localhost:5192
    bank-bradesco  api http://localhost:18647   portal     http://localhost:25647
                                                 launcher   http://localhost:5193

  Colombia (spoke-cop)
    central-bank   api http://localhost:18745   governance http://localhost:25745
                                                 treasury   http://localhost:26745
                                                 supervisor http://localhost:30745
                                                 noc        http://localhost:32745
                                                 launcher   http://localhost:5197
    bank-bancolombia api http://localhost:18746 portal     http://localhost:25746
                                                 launcher   http://localhost:5198
    bank-davivienda  api http://localhost:18747 portal     http://localhost:25747
                                                 launcher   http://localhost:5199

  Argentina (spoke-ars)
    central-bank   api http://localhost:18845   governance http://localhost:25845
                                                 treasury   http://localhost:26845
                                                 supervisor http://localhost:30845
                                                 noc        http://localhost:32845
                                                 launcher   http://localhost:5194
    bank-galicia   api http://localhost:18846   portal     http://localhost:25846
                                                 launcher   http://localhost:5195
    bank-macro     api http://localhost:18847   portal     http://localhost:25847
                                                 launcher   http://localhost:5196

  The launcher (per entity) is the A/B entry point; it lists that entity's Scenario A
  and B portals. Build the image once: ( cd ../../launcher && ./build.sh ).

  Note: a joining bank completes onboarding via its Governance Portal. On KYC
  approval the CB compliance service both issues the CB-signed certificate and
  whitelists the bank's wallet in the on-chain IdentityRegistry (registerParticipant,
  onlyRole GOVERNANCE_ROLE), signed by the CB governance key (CB_PRIVATE_KEY). No
  manual CLI step is required. Bilateral Pente/FXAgreement remains a governance-gated
  step in the join's soft tail (see toolkit/E2E-STATUS.md).
EOF
