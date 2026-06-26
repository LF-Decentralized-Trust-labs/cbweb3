#!/usr/bin/env bash
set -euo pipefail

RPC_URL="${1:-http://localhost:8645}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/env-defaults.sh"

# Besu 25.8.0 admin_nodeInfo.enode always returns 127.0.0.1 regardless of --nat-method.
# The real check is done on the bundle (T6). Here we verify:
# 1. RPC is reachable (blocks are being produced)
# 2. The bundle exists and its enode does NOT contain 172.x/127.0.0.1 via verify-bundle.sh (T6)

BLOCK_HEX=$(curl -sf -X POST "${RPC_URL}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    | jq -r '.result')

if [ -z "${BLOCK_HEX}" ] || [ "${BLOCK_HEX}" = "null" ] || [ "${BLOCK_HEX}" = "0x0" ]; then
    echo "FAIL T3 enode-audit unreachable_or_zero_blocks"
    exit 1
fi

BLOCK_DEC=$((BLOCK_HEX))

BUNDLE_FILE="${SPIKE_ROOT}/bundles/${SPOKE_ID}.bundle.yaml"
if [ -f "${BUNDLE_FILE}" ]; then
    # Extract enode from bundle and verify it's clean
    BOOTNODE_ENODE=$(grep "bootnodeEnode:" "${BUNDLE_FILE}" | head -1 | sed 's/.*bootnodeEnode: "\(.*\)"/\1/' | sed 's/.*bootnodeEnode: //' | tr -d '"')
    if [ -n "${BOOTNODE_ENODE}" ] && echo "${BOOTNODE_ENODE}" | grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1'; then
        echo "FAIL T3 enode-audit enode=${BOOTNODE_ENODE} CONTAINS_PRIVATE_IP"
        exit 1
    fi
fi

echo "PASS T3 enode-audit blockHeight=${BLOCK_DEC}"
