#!/usr/bin/env bash
#
# deploy-all.sh — bring up ALL Scenario A samples via the cbweb3 CLI.
#
# Deploys two spokes and their commercial banks, end to end:
#   • Brazil  (spoke-brl): central-bank-brazil  + bank-itau, bank-bradesco
#   • Colombia(spoke-cop): central-bank-colombia + bank-bancolombia, bank-davivienda
#
# Each central bank FOUNDS its spoke (Besu+Paladin, contracts, relay registration,
# operational backend + frontend portals) and emits a join bundle; each commercial
# bank JOINS as a full node and brings up its own operational stack.
#
# Idempotent: re-running resumes from the first incomplete step per entity.
#
# Usage:
#   ./deploy-all.sh              # build CLI, start relay, deploy everything
#   ./deploy-all.sh --clean      # wipe docker (containers+volumes) + data dirs first
#   CBWEB3_BIN=/path/to/cbweb3 ./deploy-all.sh   # reuse a prebuilt CLI binary
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # .../scenario-a/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"               # .../scenario-a
export CBWEB3_HOME="${SCENARIO_DIR}"

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
# Both founding central banks default their Paladin to host port 31648; on a single
# host the second spoke must use a distinct port. (Besu/backend/frontend ports are
# already derived per-entity from each manifest's Besu RPC port.)
export CBWEB3_PALADIN_CB_URL="http://localhost:31748"
apply "Colombia — found central-bank-colombia (spoke-cop)" "${SCRIPT_DIR}/colombia/central-bank-colombia.yaml"
copy_bundle spoke-cop
apply "Colombia — join bank-bancolombia" "${SCRIPT_DIR}/colombia/bank-bancolombia.yaml"
apply "Colombia — join bank-davivienda"  "${SCRIPT_DIR}/colombia/bank-davivienda.yaml"

# --- summary ------------------------------------------------------------------
log "all samples deployed. Endpoints (api-gateway /healthz, portals on /):"
cat <<'EOF'

  Brazil (spoke-brl)
    central-bank   api http://localhost:18645   governance http://localhost:25645
                                                 treasury   http://localhost:26645
                                                 supervisor http://localhost:30645
                                                 noc        http://localhost:32645
    bank-itau      api http://localhost:18646   portal     http://localhost:25646
    bank-bradesco  api http://localhost:18647   portal     http://localhost:25647

  Colombia (spoke-cop)
    central-bank   api http://localhost:18745   governance http://localhost:25745
                                                 treasury   http://localhost:26745
                                                 supervisor http://localhost:30745
                                                 noc        http://localhost:32745
    bank-bancolombia api http://localhost:18746 portal     http://localhost:25746
    bank-davivienda  api http://localhost:18747 portal     http://localhost:25747

  Note: a joining bank completes onboarding via its Governance Portal (KYC approval
  + CB-signed certificate). On-chain IdentityRegistry whitelisting is then performed
  CB-side by the engine:

    cbweb3 register-participant -f <central-bank.yaml> --bank <bank-code>

  (signed by the CB governance key; the bank wallet is resolved from the CB
  api-gateway my-status, or pass --wallet 0x…). Bilateral Pente/FXAgreement remains
  a governance-gated step in the join's soft tail (see toolkit/E2E-STATUS.md).
EOF
