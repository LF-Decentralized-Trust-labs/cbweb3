#!/usr/bin/env bash
set -euo pipefail

RPC_URL="${1:-http://localhost:8645}"

ENODE=$(curl -sf -X POST "${RPC_URL}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
    | jq -r '.result.enode')

if [ -z "${ENODE}" ] || [ "${ENODE}" = "null" ]; then
    echo "FAIL T3 enode-audit could not retrieve enode from ${RPC_URL}"
    exit 1
fi

if echo "${ENODE}" | grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1'; then
    echo "FAIL T3 enode-audit enode=${ENODE} CONTAINS_PRIVATE_IP"
    exit 1
fi

echo "PASS T3 enode-audit enode=${ENODE}"
