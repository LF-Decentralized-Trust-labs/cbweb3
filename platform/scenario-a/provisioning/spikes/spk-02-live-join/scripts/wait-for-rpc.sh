#!/usr/bin/env bash
set -euo pipefail

RPC_URL="${1:-http://localhost:9645}"
MAX_ATTEMPTS="${2:-60}"
DELAY_SECONDS=2

echo "[wait-for-rpc] waiting for RPC on ${RPC_URL} (max ${MAX_ATTEMPTS} attempts)"

for i in $(seq 1 "${MAX_ATTEMPTS}"); do
    if curl -sf -X POST "${RPC_URL}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        >/dev/null 2>&1; then
        echo "[wait-for-rpc] RPC ready on ${RPC_URL}"
        exit 0
    fi
    echo "[wait-for-rpc] attempt ${i}/${MAX_ATTEMPTS}: not ready yet"
    sleep "${DELAY_SECONDS}"
done

echo "[wait-for-rpc] TIMEOUT after ${MAX_ATTEMPTS} attempts on ${RPC_URL}"
exit 1
