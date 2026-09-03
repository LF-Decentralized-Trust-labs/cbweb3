#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/env-defaults.sh"

# Port availability preflight for join stack
for PORT in "${HOST_P2P_JOIN}" "${HOST_RPC_JOIN}" "${HOST_WS_JOIN}"; do
    if ss -tlnp 2>/dev/null | grep -q ":${PORT} "; then
        echo "[start-join] ERROR: port ${PORT} already in use — cannot start"
        exit 1
    fi
    if command -v lsof &>/dev/null && lsof -i ":${PORT}" -sTCP:LISTEN &>/dev/null; then
        echo "[start-join] ERROR: port ${PORT} already in use — cannot start"
        exit 1
    fi
done

# Step 1: Read BOOTNODE_ENODE from bundle
BUNDLE_FILE="${SPIKE_ROOT}/bundles/${SPOKE_ID}.bundle.yaml"

if [ ! -f "${BUNDLE_FILE}" ]; then
    echo "[start-join] ERROR: bundle file not found at ${BUNDLE_FILE}"
    echo "[start-join] Run 'make spk01.bundle' first"
    exit 1
fi

# Parse bootnodeEnode from bundle using grep+sed (no external YAML parser required)
BOOTNODE_ENODE=$(grep "bootnodeEnode:" "${BUNDLE_FILE}" | head -1 | sed 's/.*bootnodeEnode: *"\(.*\)"/\1/' | sed 's/.*bootnodeEnode: *//' | tr -d '"' | tr -d ' ')

if [ -z "${BOOTNODE_ENODE}" ] || [ "${BOOTNODE_ENODE}" = "null" ]; then
    echo "[start-join] ERROR: could not extract bootnodeEnode from bundle"
    exit 1
fi

export BOOTNODE_ENODE
echo "[start-join] using bootnode enode: ${BOOTNODE_ENODE}"

# Step 2: Start the join stack
cd "${SPIKE_ROOT}"
docker compose -f compose/stack-join.yml up -d

# Step 3: Wait for blocks to sync
echo "[start-join] waiting for joiner to sync blocks (90s timeout)..."
"${SCRIPT_DIR}/wait-for-blocks.sh" "http://localhost:${HOST_RPC_JOIN}" 45
