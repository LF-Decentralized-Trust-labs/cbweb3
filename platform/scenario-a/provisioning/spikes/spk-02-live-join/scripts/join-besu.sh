#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/env-defaults.sh"

# Port availability preflight
for PORT in "${HOST_P2P_JOIN}" "${HOST_RPC_JOIN}" "${HOST_WS_JOIN}"; do
    if ss -tlnp 2>/dev/null | grep -q ":${PORT} "; then
        echo "[join-besu] ERROR: port ${PORT} already in use — cannot start"
        exit 1
    fi
    if command -v lsof &>/dev/null && lsof -i ":${PORT}" -sTCP:LISTEN &>/dev/null; then
        echo "[join-besu] ERROR: port ${PORT} already in use — cannot start"
        exit 1
    fi
done

# Step 1: Read BOOTNODE_ENODE from bundle
BUNDLE_FILE="${SPIKE_ROOT}/bundles/${SPOKE_ID}.bundle.yaml"

if [ ! -f "${BUNDLE_FILE}" ]; then
    echo "[join-besu] ERROR: bundle file not found at ${BUNDLE_FILE}"
    echo "[join-besu] Run 'make spk02.bundle' first"
    exit 1
fi

BOOTNODE_ENODE=$(grep "bootnodeEnode:" "${BUNDLE_FILE}" | head -1 | sed 's/.*bootnodeEnode: *"\(.*\)"/\1/' | sed 's/.*bootnodeEnode: *//' | tr -d '"' | tr -d ' ')
if [ -z "${BOOTNODE_ENODE}" ] || [ "${BOOTNODE_ENODE}" = "null" ]; then
    echo "[join-besu] ERROR: could not extract bootnodeEnode from bundle"
    exit 1
fi

export BOOTNODE_ENODE
echo "[join-besu] using bootnode enode: ${BOOTNODE_ENODE}"

# Step 2: Start the join stack
cd "${SPIKE_ROOT}"
docker compose -f compose/stack-join.yml up -d besu-joiner

# Step 3: Wait for joiner RPC and block production
echo "[join-besu] waiting for joiner RPC on port ${HOST_RPC_JOIN}..."
"${SCRIPT_DIR}/wait-for-rpc.sh" "http://localhost:${HOST_RPC_JOIN}" 60

echo "[join-besu] waiting for joiner to sync blocks..."
"${SCRIPT_DIR}/wait-for-blocks.sh" "http://localhost:${HOST_RPC_JOIN}" 45

# Step 4: Compare block heights
BOOT_HEIGHT_HEX=$(curl -sf -X POST "http://localhost:${HOST_RPC_BOOT}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    | jq -r '.result') || true

JOIN_HEIGHT_HEX=$(curl -sf -X POST "http://localhost:${HOST_RPC_JOIN}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    | jq -r '.result') || true

BOOT_HEIGHT=$((BOOT_HEIGHT_HEX))
JOIN_HEIGHT=$((JOIN_HEIGHT_HEX))
echo "[join-besu] bootnode height=${BOOT_HEIGHT} joiner height=${JOIN_HEIGHT}"

# Loop until heights converge or timeout
for i in $(seq 1 30); do
    JOIN_HEIGHT_HEX=$(curl -sf -X POST "http://localhost:${HOST_RPC_JOIN}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        | jq -r '.result') || true
    BOOT_HEIGHT_HEX=$(curl -sf -X POST "http://localhost:${HOST_RPC_BOOT}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        | jq -r '.result') || true

    JOIN_HEIGHT=$((JOIN_HEIGHT_HEX))
    BOOT_HEIGHT=$((BOOT_HEIGHT_HEX))
    DIFF=$((BOOT_HEIGHT - JOIN_HEIGHT))
    DIFF_ABS=${DIFF#-}

    echo "[join-besu] attempt ${i}/30: boot=${BOOT_HEIGHT} joiner=${JOIN_HEIGHT} diff=${DIFF}"

    if [ "${DIFF_ABS}" -le 5 ]; then
        echo "[join-besu] heights converged (diff=${DIFF_ABS} ≤ 5)"
        echo "[join-besu] Joiner is synced and ready for voting"
        exit 0
    fi
    sleep 2
done

echo "[join-besu] ERROR: heights did not converge within 60s (boot=${BOOT_HEIGHT} joiner=${JOIN_HEIGHT})"
exit 1
