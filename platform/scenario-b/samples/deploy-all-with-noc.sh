#!/usr/bin/env bash
#
# deploy-all-with-noc.sh — deploy-all.sh PLUS a functional NOC (Network Operations
# Center) for Scenario B. No reverse proxy (port-based, same as deploy-all.sh).
#
# It runs the normal deploy-all.sh flow (hub + spokes + joins), then stands up the
# full NOC control plane for the Brazil spoke via an `observe`-mode apply:
#   • Postgres + noc-backend on :8090 + noc-portal on :3030 (own docker network)
#   • central-bank-brazil's found-spoke already runs the spoke noc-agent (pushes here)
#
# Result: a working NOC — log in at http://localhost:3030 with the NOC_ADMIN user
# provisioned by central-bank-brazil's found-spoke (admin@brasil.noc.gov). That user
# was added to brazil/central-bank-brazil.yaml so the portal has a login.
#
# Single-host scope: the NOC ports (8090/3030) are fixed (one NOC per host), so this
# stands up ONE NOC (Brazil). A separate hub/Argentina NOC needs another host (see
# deploy-lnet/). All entities' noc-agents default to host.docker.internal:8090, so
# they co-report here, but only the registered spoke (spoke-brl) is shown.
#
# Usage:
#   ./deploy-all-with-noc.sh            # deploy hub + Brazil + Argentina + Brazil NOC
#   ./deploy-all-with-noc.sh --clean    # wipe docker + data dirs first, then deploy
#   CBWEB3B_BIN=/path/to/cbweb3b ./deploy-all-with-noc.sh   # reuse a prebuilt CLI
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"    # .../scenario-b/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"                # .../scenario-b
REPO_ROOT="$(cd "${SCENARIO_DIR}/.." && pwd)"                 # repo root (--repo-root)

cd "${SCRIPT_DIR}"   # match deploy-all.sh: relative dataDir + state under samples/

log() { printf '\n\033[1;36m[deploy+noc] %s\033[0m\n' "$*"; }

# --- 1) base deploy (hub + spokes + joins) -----------------------------------
# Delegates to the canonical script so this variant never drifts from it. Each
# CB's found-spoke emits its NOC bundle under samples/bundles/ during this run.
log "running base deploy-all.sh…"
bash "${SCRIPT_DIR}/deploy-all.sh" "$@"

# --- 2) NOC control plane for the Brazil spoke -------------------------------
BIN="${CBWEB3B_BIN:-${SCRIPT_DIR}/.cbweb3b}"
NOC_BUNDLE="${SCRIPT_DIR}/bundles/spoke-brl.noc.bundle.yaml"

if [[ ! -f "${NOC_BUNDLE}" ]]; then
  log "ERROR: NOC bundle not found at ${NOC_BUNDLE}"
  log "central-bank-brazil's found-spoke should have emitted it — check the base deploy output."
  exit 1
fi

log "observe — NOC control plane for spoke-brl (Postgres + noc-backend :8090 + noc-portal :3030)"
"${BIN}" apply -f "${SCRIPT_DIR}/brazil/noc-brazil.yaml" -o yaml \
  --repo-root "${REPO_ROOT}" \
  --out-dir "${SCRIPT_DIR}"

# --- summary ------------------------------------------------------------------
log "NOC deployed for Scenario B."
cat <<'EOF'

  NOC (spoke-brl)
    portal   http://localhost:3030
    backend  http://localhost:8090/api/v1
    login    admin@brasil.noc.gov  (NOC_ADMIN; password from central-bank-brazil.yaml)

  One NOC per host (fixed :8090/:3030). The hub + Argentina noc-agents also push to
  :8090, but only the registered spoke (spoke-brl) is shown. Run their own NOCs on
  separate hosts — see deploy-lnet/.
EOF
