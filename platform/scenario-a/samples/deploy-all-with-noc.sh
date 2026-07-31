#!/usr/bin/env bash
#
# deploy-all-with-noc.sh — deploy-all.sh PLUS a functional NOC (Network Operations
# Center) for Scenario A. No reverse proxy (port-based, same as deploy-all.sh).
#
# It runs the normal deploy-all.sh flow (founds + joins), then stands up the NOC
# DATA PLANE for the Brazil spoke via an `observe`-mode apply:
#   • central-bank-brazil's found ALREADY built the NOC portal (http://localhost:32645)
#   • this adds Postgres + noc-backend on :28645 + the founding CB's noc-agent
#
# Result: a working NOC — log in at http://localhost:32645 with the ROLE_NOC_ADMIN
# user provisioned by central-bank-brazil's found (admin@brasil.noc.gov).
#
# Single-host scope: the NOC backend port (28645) is fixed (one NOC per host), so
# this stands up ONE NOC (Brazil). Colombia's NOC portal (:32745) points at the same
# :28645 backend; a second, independent NOC needs another host (see deploy-lnet/).
#
# Usage:
#   ./deploy-all-with-noc.sh            # deploy everything + Brazil NOC
#   ./deploy-all-with-noc.sh --clean    # wipe docker + data dirs first, then deploy
#   CBWEB3_BIN=/path/to/cbweb3 ./deploy-all-with-noc.sh   # reuse a prebuilt CLI
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # .../scenario-a/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"               # .../scenario-a
export CBWEB3_HOME="${SCENARIO_DIR}"
export CBWEB3_SINGLE_HOST=1

log() { printf '\n\033[1;36m[deploy+noc] %s\033[0m\n' "$*"; }

# --- 1) base deploy (founds + joins) -----------------------------------------
# Delegates to the canonical script so this variant never drifts from it. The
# NOC bundle is emitted (best-effort) by each CB's found during this run.
log "running base deploy-all.sh…"
bash "${SCRIPT_DIR}/deploy-all.sh" "$@"

# --- 2) NOC data plane for the Brazil spoke ----------------------------------
BIN="${CBWEB3_BIN:-${SCRIPT_DIR}/.cbweb3}"
# Mirror deploy-all.sh's bundle location: <CBWEB3_OUTPUT_DIR|${PWD}/cbweb3-data>/bundles.
DATA_BUNDLES="${CBWEB3_OUTPUT_DIR:-${PWD}/cbweb3-data}/bundles"
NOC_BUNDLE="${DATA_BUNDLES}/spoke-brl.noc.bundle.yaml"

if [[ ! -f "${NOC_BUNDLE}" ]]; then
  log "ERROR: NOC bundle not found at ${NOC_BUNDLE}"
  log "central-bank-brazil's found should have emitted it — check the base deploy output."
  exit 1
fi

# The observe manifest references ../bundles/spoke-brl.noc.bundle.yaml; place a copy
# alongside the join bundles deploy-all.sh already drops in samples/bundles/.
mkdir -p "${SCRIPT_DIR}/bundles"
cp "${NOC_BUNDLE}" "${SCRIPT_DIR}/bundles/"
log "NOC bundle spoke-brl → samples/bundles/"

log "observe — NOC data plane for spoke-brl (Postgres + noc-backend :28645 + agent)"
"${BIN}" apply -f "${SCRIPT_DIR}/brazil/noc-brazil.yaml" -o yaml

# --- summary ------------------------------------------------------------------
log "NOC deployed for Scenario A."
cat <<'EOF'

  NOC (spoke-brl)
    portal   http://localhost:32645   (built by central-bank-brazil's found)
    backend  http://localhost:28645/api/v1
    login    admin@brasil.noc.gov  (ROLE_NOC_ADMIN; password from central-bank-brazil.yaml)

  Colombia's NOC portal (http://localhost:32745) points at the SAME :28645 backend
  (one NOC per host). Run a second, independent NOC on another host — see deploy-lnet/.
EOF
