#!/usr/bin/env bash
set -euo pipefail

RPC_URL="${1:-http://localhost:9648}"
MAX_ATTEMPTS="${2:-90}"
DELAY_SECONDS=2

echo "[wait-for-paladin] waiting for Paladin RPC on ${RPC_URL} (max ${MAX_ATTEMPTS} attempts)"

for i in $(seq 1 "${MAX_ATTEMPTS}"); do
    if curl -sS --max-time 2 \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","id":1,"method":"ptx_getTransaction","params":["dummy"]}' \
        "${RPC_URL}" 2>/dev/null | grep -q 'PD020704'; then
        echo "[wait-for-paladin] Paladin RPC ready on ${RPC_URL}"
        exit 0
    fi
    echo "[wait-for-paladin] attempt ${i}/${MAX_ATTEMPTS}: not ready yet"
    sleep "${DELAY_SECONDS}"
done

echo "[wait-for-paladin] TIMEOUT after ${MAX_ATTEMPTS} attempts on ${RPC_URL}"
exit 1
