#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/env-defaults.sh"

BOOT_RPC="http://localhost:${HOST_RPC_BOOT}"
JOIN_RPC="http://localhost:${HOST_RPC_JOIN}"
MODE="${1:-all}"

run_t1() {
    echo "[T1] QBFT validator set check..."

    VALIDATORS=$(curl -sf -X POST "${BOOT_RPC}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"qbft_getValidatorsByBlockNumber","params":["latest"],"id":1}' \
        | jq -r '.result[]?' 2>/dev/null || echo "")

    VALIDATOR_COUNT=$(echo "${VALIDATORS}" | grep -c '^0x' || echo "0")

    # Get joiner address
    JOINER_ADDR=""
    if curl -sf -X POST "${JOIN_RPC}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_coinbase","params":[],"id":1}' \
        >/dev/null 2>&1; then
        JOINER_ADDR=$(curl -sf -X POST "${JOIN_RPC}" \
            -H 'Content-Type: application/json' \
            -d '{"jsonrpc":"2.0","method":"eth_coinbase","params":[],"id":1}' \
            | jq -r '.result')
    fi

    JOINER_LOWER=$(echo "${JOINER_ADDR}" | tr '[:upper:]' '[:lower:]')
    JOINER_IN_SET=$(echo "${VALIDATORS}" | tr '[:upper:]' '[:lower:]' | grep -c "${JOINER_LOWER}" || echo "0")

    if [ "${VALIDATOR_COUNT}" -eq 4 ] && [ "${JOINER_IN_SET}" -gt 0 ]; then
        echo "PASS T1 qbft-validator-set validators=4 joiner=${JOINER_ADDR}"
    elif [ "${VALIDATOR_COUNT}" -eq 4 ]; then
        echo "FAIL T1 qbft-validator-set validators=4 but joiner=${JOINER_ADDR} not found in set"
        exit 1
    else
        echo "FAIL T1 qbft-validator-set validators=${VALIDATOR_COUNT} expected=4"
        exit 1
    fi
}

run_t2() {
    echo "[T2] Block continuity check (20 most recent blocks)..."

    BLOCK_PERIOD=2

    # Get current chain head
    CURRENT_HEX=$(curl -sf -X POST "${BOOT_RPC}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        | jq -r '.result' 2>/dev/null || echo "0x0")
    CURRENT_BLOCK=$((CURRENT_HEX))

    if [ "${CURRENT_BLOCK}" -lt 1 ]; then
        echo "FAIL T2 block-continuity chain has no blocks yet"
        exit 1
    fi

    # Read the 20 most-recent blocks relative to chain head
    START_BLOCK=$(( CURRENT_BLOCK >= 19 ? CURRENT_BLOCK - 19 : 0 ))
    echo "[T2]   chain head: block ${CURRENT_BLOCK}, checking [${START_BLOCK}..${CURRENT_BLOCK}]"

    MAX_GAP=0
    PREV_TS=0
    MISSING=0

    for i in $(seq "${START_BLOCK}" "${CURRENT_BLOCK}"); do
        RAW=$(curl -sf -X POST "${BOOT_RPC}" \
            -H 'Content-Type: application/json' \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBlockByNumber\",\"params\":[\"0x$(printf '%x' ${i})\",false],\"id\":1}" \
            | jq -r '.result.timestamp // empty' 2>/dev/null) || RAW=""

        if [ -z "${RAW}" ]; then
            echo "[T2]   WARNING: block ${i} missing from response"
            MISSING=$((MISSING + 1))
            continue
        fi

        BLOCK_TS=0
        if [[ "${RAW}" == 0x* ]]; then
            BLOCK_TS=$((RAW))
        else
            BLOCK_TS="${RAW}"
        fi

        if [ "${PREV_TS}" -gt 0 ]; then
            GAP=$((BLOCK_TS - PREV_TS))
            [ "${GAP}" -gt "${MAX_GAP}" ] && MAX_GAP="${GAP}"
        fi
        PREV_TS="${BLOCK_TS}"
    done

    # Maximum healthy gap = block period + request timeout (which is 4s per genesis-config).
    # With 4 validators the worst-case round takes one timeout if a validator is slow.
    THRESHOLD=$((BLOCK_PERIOD + 4))

    if [ "${MISSING}" -gt 0 ]; then
        echo "FAIL T2 block-continuity missing_blocks=${MISSING}"
        exit 1
    elif [ "${MAX_GAP}" -le "${THRESHOLD}" ]; then
        echo "PASS T2 block-continuity max_gap=${MAX_GAP}s threshold=${THRESHOLD}s head=${CURRENT_BLOCK}"
    else
        echo "FAIL T2 block-continuity max_gap=${MAX_GAP}s threshold=${THRESHOLD}s head=${CURRENT_BLOCK}"
        exit 1
    fi
}

case "${MODE}" in
    --t2) run_t2 ;;
    all)   run_t1 && run_t2 ;;
    *)     run_t1 ;;
esac
