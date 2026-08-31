#!/usr/bin/env bash
# test-commercial-bank-template.sh — Integration test for TK-8 (commercial-bank Compose template)
#
# The commercial-bank node is a JOINER: it has no genesis-init (genesis comes from
# the join bundle, written by TK-9) and REQUIRES BOOTNODE_ENODE. A full sync test
# would need a live bootnode; instead this test validates the template's structural
# guarantees via `docker compose config` (no containers started):
#
#   (a) template renders with all required variables set
#   (b) there is NO genesis-init service (genesis is supplied, never generated)
#   (c) BOOTNODE_ENODE is required — config fails when it is absent
#   (d) the genesis directory is mounted read-only (never written by the node)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
TEMPLATE="${REPO_ROOT}/scenario-a/provisioning/templates/commercial-bank/docker-compose.yaml"
PASS=0
FAIL=0

log() { echo "[tk8-test] $*"; }
pass() { log "PASS $1"; PASS=$((PASS + 1)); }
fail() { log "FAIL $1: $2"; FAIL=$((FAIL + 1)); }

if [ ! -f "${TEMPLATE}" ]; then
    echo "FAIL: template not found at ${TEMPLATE}"
    exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
    log "SKIP: docker not available — cannot validate compose config"
    exit 0
fi

# Common required variables (a joiner needs BOOTNODE_ENODE).
base_env() {
    export SPOKE_ID="spoke-test" \
           BANK_ID="commercial-bank-alpha" \
           BESU_IMAGE="hyperledger/besu:25.8.0" \
           BESU_ADVERTISED_HOST="cbweb3-spoke-test-besu.commercial-bank-alpha" \
           BESU_RPC_PORT="18746" \
           BESU_WS_PORT="18756" \
           BESU_P2P_PORT="31603"
}

# --- Test (a): renders with BOOTNODE_ENODE set ---
log "Test (a): template renders with all required variables"
base_env
export BOOTNODE_ENODE="enode://abc123@cbweb3-spoke-test-besu.central-bank:31303"
if CONFIG_OUT="$(docker compose -f "${TEMPLATE}" config 2>&1)"; then
    pass "(a) docker compose config succeeded"
else
    fail "(a)" "docker compose config failed: ${CONFIG_OUT}"
fi

# --- Test (b): NO genesis-init service ---
log "Test (b): template must not contain a genesis-init service"
if echo "${CONFIG_OUT:-}" | grep -q "genesis-init"; then
    fail "(b)" "genesis-init service present — commercial bank must not generate genesis"
else
    pass "(b) no genesis-init service (genesis supplied by bundle)"
fi

# --- Test (c): BOOTNODE_ENODE is required ---
log "Test (c): BOOTNODE_ENODE must be required"
base_env
unset BOOTNODE_ENODE
if docker compose -f "${TEMPLATE}" config >/dev/null 2>&1; then
    fail "(c)" "config succeeded without BOOTNODE_ENODE — it must be required for a joiner"
else
    pass "(c) config fails without BOOTNODE_ENODE (required for mode:join)"
fi

# --- Test (d): genesis dir mounted read-only ---
log "Test (d): genesis directory mounted read-only"
base_env
export BOOTNODE_ENODE="enode://abc123@cbweb3-spoke-test-besu.central-bank:31303"
CONFIG_OUT="$(docker compose -f "${TEMPLATE}" config 2>&1)"
if echo "${CONFIG_OUT}" | grep -qE "genesis.*:ro|read_only: true"; then
    pass "(d) genesis mounted read-only"
else
    fail "(d)" "genesis mount is not read-only"
fi

# --- Summary ---
echo ""
echo "==============================="
echo "TK-8 Template Integration Test"
echo "==============================="
echo "  PASS: ${PASS}"
echo "  FAIL: ${FAIL}"
echo "==============================="
if [ "${FAIL}" -eq 0 ]; then
    echo "PASS: template commercial-bank OK"
    exit 0
else
    echo "FAIL: ${FAIL} test(s) failed"
    exit 1
fi
