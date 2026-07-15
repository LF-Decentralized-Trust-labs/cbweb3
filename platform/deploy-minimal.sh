#!/usr/bin/env bash
#
# deploy-minimal.sh — smallest A+B footprint on ONE host to validate the shared launcher.
#
# Brings up, per scenario, ONLY the Brazil spoke and one commercial bank (bank-itau),
# using EACH scenario's own toolkit CLI (no legacy Makefiles):
#
#   Scenario A (Enhanced Correspondent Banking):
#     • central-bank-brazil  (found, spoke-brl)   + bank-itau (join)
#
#   Scenario B (International Hub):
#     • hub-cbweb3           (found-hub — REQUIRED: the spoke reads its hub bundle)
#     • central-bank-brazil  (found-spoke, spoke-brl) + bank-itau (join)
#
# Why this validates the launcher: the launcher is one container PER ENTITY, shared
# between A and B (same spec.launcherPort in both). So after this run:
#     http://localhost:5191  → Brazil CB launcher (lists BOTH A and B portals)
#     http://localhost:5192  → bank-itau launcher (lists BOTH A and B portals)
#
# Coexistence is collision-free by construction:
#     • Besu/derived host ports: Scenario A ≤ 32847, Scenario B ≥ 33645 (+25000 shift)
#     • Cacti relay: Scenario A on 4000, Scenario B on 7000
#     • Containers: cbweb3-* / spoke-brl-* (A) vs sc-b-cbweb3-* (B)
#     • Scenario B deploys no Paladin, so A's Paladin (31648) never clashes
#
# Idempotent: re-running resumes from the first incomplete step per entity.
#
# Usage:
#   ./deploy-minimal.sh            # build CLIs, start relays, deploy the minimal A+B set
#   ./deploy-minimal.sh --clean    # wipe ALL docker state + sample data dirs first
#
# Note on resources: this still starts ~45-50 containers (many JVM). On a RAM-tight
# host, run with --clean so no unrelated stack is competing for memory.
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
A_DIR="${ROOT}/scenario-a"
B_DIR="${ROOT}/scenario-b"
A_SAMPLES="${A_DIR}/samples"
B_SAMPLES="${B_DIR}/samples"

log() { printf '\n\033[1;36m[minimal] %s\033[0m\n' "$*"; }

# --- optional clean -----------------------------------------------------------
if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes + networks) and sample data dirs…"
  docker rm -f $(docker ps -aq) 2>/dev/null || true
  docker volume rm $(docker volume ls -q) 2>/dev/null || true
  docker network prune -f 2>/dev/null || true
  rm -rf "${A_SAMPLES}/cbweb3-data" "${A_SAMPLES}/bundles" \
         "${B_SAMPLES}/cbweb3-data" "${B_SAMPLES}/bundles" 2>/dev/null || true
fi

# --- shared launcher image (built once; both scenarios' launcher steps reuse it) --
if ! docker image inspect cbweb3/launcher:local >/dev/null 2>&1; then
  log "building shared launcher image (cbweb3/launcher:local)…"
  ( cd "${ROOT}/launcher" && ./build.sh )
fi

# --- contract dependencies (Soldeer), one-time and idempotent per scenario --------
for c in "${A_DIR}/contracts" "${B_DIR}/contracts"; do
  if [[ -d "${c}" && ! -d "${c}/dependencies" ]]; then
    log "installing contract dependencies in ${c}…"
    ( cd "${c}" && forge soldeer install )
  fi
done

# --- toolkit CLIs (one per scenario) ---------------------------------------------
A_BIN="${A_SAMPLES}/.cbweb3"
B_BIN="${B_SAMPLES}/.cbweb3b"
log "building scenario-a CLI → ${A_BIN}"
( cd "${A_DIR}/toolkit" && go build -o "${A_BIN}" ./cmd/cbweb3 )
log "building scenario-b CLI → ${B_BIN}"
( cd "${B_DIR}/toolkit" && go build -o "${B_BIN}" ./cmd/cbweb3b )

# ============================== SCENARIO A ======================================
# The CLI resolves the manifest's relative node.dataDir against its CWD, so run from
# the samples dir (data + bundles land under samples/cbweb3-data).
log "SCENARIO A — starting Cacti relay (:4000, hard prerequisite of register-relay)"
bash "${A_DIR}/provisioning/scripts/start-cacti.sh" >/dev/null

cd "${A_SAMPLES}"
export CBWEB3_HOME="${A_DIR}"

log "A: found central-bank-brazil (spoke-brl)"
"${A_BIN}" apply -f "${A_SAMPLES}/brazil/central-bank-brazil.yaml" -o yaml

# The joining bank reads ../bundles/spoke-brl.bundle.yaml (samples/bundles), which the
# founder emits under samples/cbweb3-data/bundles — copy it across first.
log "A: publishing spoke-brl join bundle → samples/bundles/"
mkdir -p "${A_SAMPLES}/bundles"
cp "${A_SAMPLES}/cbweb3-data/bundles/spoke-brl.bundle.yaml" "${A_SAMPLES}/bundles/"

log "A: join bank-itau"
"${A_BIN}" apply -f "${A_SAMPLES}/brazil/bank-itau.yaml" -o yaml

unset CBWEB3_HOME

# ============================== SCENARIO B ======================================
# Scenario B emits bundles straight into samples/bundles via --out-dir, so no copy
# step is needed. found-hub MUST run first: the spoke manifest reads the hub bundle.
log "SCENARIO B — starting Cacti relay (:7000)"
bash "${B_DIR}/provisioning/scripts/start-cacti.sh" >/dev/null

cd "${B_SAMPLES}"
B_FLAGS=(-o yaml --repo-root "${ROOT}" --out-dir "${B_SAMPLES}")

log "B: found-hub (hub-cbweb3)"
"${B_BIN}" apply -f "${B_SAMPLES}/hub/hub-cbweb3.yaml" "${B_FLAGS[@]}"

log "B: found central-bank-brazil (spoke-brl)"
"${B_BIN}" apply -f "${B_SAMPLES}/brazil/central-bank-brazil.yaml" "${B_FLAGS[@]}" \
  --spoke-rpc http://localhost:33645

log "B: join bank-itau"
"${B_BIN}" apply -f "${B_SAMPLES}/brazil/bank-itau.yaml" "${B_FLAGS[@]}" \
  --spoke-rpc http://localhost:33646

# --- summary --------------------------------------------------------------------
log "minimal A+B deployed. Shared launcher per entity (lists both scenarios' portals):"
cat <<'EOF'

  Brazil Central Bank
    LAUNCHER (A/B)   http://localhost:5191
    Scenario A       api http://localhost:18645   governance http://localhost:25645
    Scenario B       api http://localhost:41645   governance http://localhost:42645

  bank-itau (Brazil)
    LAUNCHER (A/B)   http://localhost:5192
    Scenario A       api http://localhost:18646   portal http://localhost:25646
    Scenario B       api http://localhost:41646   portal http://localhost:42646

  Open a launcher URL: it shows a "Scenario A" and a "Scenario B" group of portal
  buttons for that entity, served by ONE launcher container per host.
EOF
