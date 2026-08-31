#!/usr/bin/env bash
set -euo pipefail

RPC_URL="${1:-http://localhost:8645}"
MAX_ATTEMPTS="${2:-30}"
DELAY_SECONDS=2

echo "[wait-for-blocks] waiting for blocks on ${RPC_URL} (max ${MAX_ATTEMPTS} attempts)"

for i in $(seq 1 "${MAX_ATTEMPTS}"); do
    BLOCK_HEX=$(curl -sf -X POST "${RPC_URL}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        | jq -r '.result' 2>/dev/null) || true

    if [ -n "${BLOCK_HEX}" ] && [ "${BLOCK_HEX}" != "null" ] && [ "${BLOCK_HEX}" != "0x0" ]; then
        BLOCK_DEC=$((BLOCK_HEX))
        echo "[wait-for-blocks] attempt ${i}/${MAX_ATTEMPTS} RPC=${RPC_URL} blockHeight=${BLOCK_DEC}"
        echo "[wait-for-blocks] blocks found on ${RPC_URL}, block=${BLOCK_DEC}"
        exit 0
    fi

    echo "[wait-for-blocks] attempt ${i}/${MAX_ATTEMPTS} RPC=${RPC_URL} blockHeight=0"
    sleep "${DELAY_SECONDS}"
done

echo "[wait-for-blocks] TIMEOUT after ${MAX_ATTEMPTS} attempts on ${RPC_URL}"
exit 1
