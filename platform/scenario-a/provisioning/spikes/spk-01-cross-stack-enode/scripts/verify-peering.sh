#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env-defaults.sh"

rpc_call() {
    local rpc_url="$1"
    local method="$2"
    local params="${3:-[]}"
    curl -sf -X POST "${rpc_url}" \
        -H 'Content-Type: application/json' \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}" \
        | jq -r '.result'
}

case "${MODE}" in
    found)
        echo "[verify-peering] checking block production on found stack"

        BLOCK_HEX=$(rpc_call "http://localhost:${HOST_RPC_BOOT}" "eth_blockNumber")
        if [ -z "${BLOCK_HEX}" ] || [ "${BLOCK_HEX}" = "null" ] || [ "${BLOCK_HEX}" = "0x0" ]; then
            echo "FAIL T2 block-production blockHeight=0"
            exit 1
        fi
        BLOCK_DEC=$((BLOCK_HEX))
        echo "[verify-peering] bootnode block height: ${BLOCK_DEC}"

        VALIDATORS=$(rpc_call "http://localhost:${HOST_RPC_BOOT}" "qbft_getValidatorsByBlockNumber" '["latest"]')
        VALIDATOR_COUNT=$(echo "${VALIDATORS}" | jq 'length')
        echo "[verify-peering] validator count: ${VALIDATOR_COUNT}"

        if [ "${VALIDATOR_COUNT}" != "3" ]; then
            echo "FAIL T2 block-production validators=${VALIDATOR_COUNT} expected=3"
            exit 1
        fi

        echo "PASS T2 block-production blockHeight=${BLOCK_DEC} validators=${VALIDATOR_COUNT}"
        ;;

    join)
        echo "[verify-peering] checking cross-stack peering"

        # T4: check peers on joiner
        PEERS=$(rpc_call "http://localhost:${HOST_RPC_JOIN}" "admin_peers")
        PEER_COUNT=$(echo "${PEERS}" | jq 'length')
        echo "[verify-peering] joiner peer count: ${PEER_COUNT}"

        if [ "${PEER_COUNT}" -lt 1 ]; then
            echo "FAIL T4 cross-stack-peering peers=0"
            exit 1
        fi

        # T4: check block height convergence (within ±5 of found stack)
        FOUND_HEX=$(rpc_call "http://localhost:${HOST_RPC_BOOT}" "eth_blockNumber")
        JOIN_HEX=$(rpc_call "http://localhost:${HOST_RPC_JOIN}" "eth_blockNumber")

        if [ -z "${FOUND_HEX}" ] || [ "${FOUND_HEX}" = "null" ]; then
            echo "FAIL T4 cross-stack-peering UNREACHABLE found_rpc"
            exit 1
        fi
        if [ -z "${JOIN_HEX}" ] || [ "${JOIN_HEX}" = "null" ] || [ "${JOIN_HEX}" = "0x0" ]; then
            echo "FAIL T4 cross-stack-peering join_block_zero"
            exit 1
        fi

        FOUND_DEC=$((FOUND_HEX))
        JOIN_DEC=$((JOIN_HEX))
        LAG=$((FOUND_DEC - JOIN_DEC))
        ABS_LAG=${LAG#-}

        if [ "${ABS_LAG}" -gt 5 ]; then
            echo "FAIL T4 cross-stack-peering BLOCK_LAG height_found=${FOUND_DEC} height_join=${JOIN_DEC}"
            exit 1
        fi

        # T4 (convergence): check that joiner appears in bootnode's admin_peers
        BOOT_PEERS=$(rpc_call "http://localhost:${HOST_RPC_BOOT}" "admin_peers" || echo "[]")
        JOINER_ENODE=$(rpc_call "http://localhost:${HOST_RPC_JOIN}" "admin_nodeInfo" | jq -r '.enode')
        JOINER_PUBKEY=$(echo "${JOINER_ENODE}" | cut -d'@' -f1 | cut -d'/' -f4)

        if [ -n "${JOINER_PUBKEY}" ] && [ "${JOINER_PUBKEY}" != "null" ]; then
            if echo "${BOOT_PEERS}" | jq -e --arg pk "${JOINER_PUBKEY}" '.[] | select(.id == $pk)' > /dev/null 2>&1; then
                echo "[verify-peering] joiner enode found in bootnode admin_peers"
            else
                echo "FAIL T4 cross-stack-peering FOUND_SIDE_MISSING joiner_pubkey=${JOINER_PUBKEY}"
                exit 1
            fi
        fi

        echo "PASS T4 cross-stack-peering peers=${PEER_COUNT} height_found=${FOUND_DEC} height_join=${JOIN_DEC}"
        ;;

    *)
        echo "Usage: $0 <found|join>"
        echo "  found — check block production (T2)"
        echo "  join  — check cross-stack peering (T4)"
        exit 1
        ;;
esac
