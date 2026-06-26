#!/usr/bin/env bash
set -euo pipefail

FOUND_RPC="${SPOKE_FOUND_RPC:-http://host.docker.internal:8645}"
JOIN_RPC="${SPOKE_JOIN_RPC:-http://host.docker.internal:8646}"

rpc_block() {
    local url="$1"
    curl -sf -X POST "${url}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        | jq -r '.result'
}

echo "[verify-cacti-rpc] testing RPC reachability from isolated network"
echo "[verify-cacti-rpc] found spoke RPC: ${FOUND_RPC}"
echo "[verify-cacti-rpc] join spoke RPC: ${JOIN_RPC}"

FOUND_HEX=$(rpc_block "${FOUND_RPC}")
if [ -z "${FOUND_HEX}" ] || [ "${FOUND_HEX}" = "null" ] || [ "${FOUND_HEX}" = "0x0" ]; then
    echo "FAIL T5 cacti-rpc-isolation UNREACHABLE url=${FOUND_RPC}"
    exit 1
fi
FOUND_DEC=$((FOUND_HEX))

JOIN_HEX=$(rpc_block "${JOIN_RPC}")
if [ -z "${JOIN_HEX}" ] || [ "${JOIN_HEX}" = "null" ] || [ "${JOIN_HEX}" = "0x0" ]; then
    echo "FAIL T5 cacti-rpc-isolation UNREACHABLE url=${JOIN_RPC}"
    exit 1
fi
JOIN_DEC=$((JOIN_HEX))

echo "PASS T5 cacti-rpc-isolation found=${FOUND_DEC} join=${JOIN_DEC} network=isolated"
