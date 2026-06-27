#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"

source "${SCRIPT_DIR}/env-defaults.sh"

BOOT_RPC="http://localhost:${HOST_RPC_BOOT}"
V1_RPC="http://localhost:${HOST_RPC_V1}"
V2_RPC="http://localhost:${HOST_RPC_V2}"
JOIN_RPC="http://localhost:${HOST_RPC_JOIN}"

rpc_call() {
    local url="$1" method="$2" params="${3:-[]}"
    curl -sf -X POST "${url}" \
        -H 'Content-Type: application/json' \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}"
}

# ── Step 1: Derive joiner EVM address from key3 ─────────────────────────────
# Use Besu's own CLI to derive the address from the private key file.
# This is the only reliable method that produces the correct keccak256-based
# Ethereum address without requiring external crypto libraries.
echo "[vote-in-validator] deriving joiner EVM address from key3..."
JOINER_ADDR=$(docker run --rm \
    --user "$(id -u):$(id -g)" \
    -v "${DATA_DIR}:/data:ro" \
    hyperledger/besu:25.8.0 \
    public-key export-address \
    --node-private-key-file=/data/key3 2>/dev/null | tr -d '\n\r' | grep -oP '0x[a-fA-F0-9]{40}' || true)

if [ -z "${JOINER_ADDR}" ] || [ "${JOINER_ADDR}" = "0x0000000000000000000000000000000000000000" ]; then
    echo "[vote-in-validator] ERROR: could not derive joiner address from key3"
    exit 1
fi
echo "[vote-in-validator] joiner EVM address: ${JOINER_ADDR}"

# ── Step 2: Show current validator set ─────────────────────────────────────
echo "[vote-in-validator] current validator set (before voting):"
rpc_call "${BOOT_RPC}" "qbft_getValidatorsByBlockNumber" '["latest"]' \
    | jq -r '.result[]?' 2>/dev/null | sed 's/^/  /' || true

# ── Step 3: Cast one vote from each of the 3 existing validators ────────────
# QBFT block-header voting: each validator's vote is node-local.
# We need floor(3/2)+1 = 2 votes from distinct validators.
# We call qbft_proposeValidatorVote on each validator's own RPC.

declare -A VALIDATOR_RPCS=(
    ["boot"]="${BOOT_RPC}"
    ["v1"]="${V1_RPC}"
    ["v2"]="${V2_RPC}"
)

for VNAME in boot v1 v2; do
    VRPC="${VALIDATOR_RPCS[${VNAME}]}"
    echo "[vote-in-validator] casting vote from ${VNAME} (${VRPC})..."
    VOTE_RESULT=$(rpc_call "${VRPC}" "qbft_proposeValidatorVote" "[\"${JOINER_ADDR}\",true]" \
        | jq -r '.result // .error.message // "UNKNOWN"' 2>/dev/null) || VOTE_RESULT="ERROR"
    echo "[vote-in-validator]   ${VNAME} result: ${VOTE_RESULT}"
done

# ── Step 4: Poll for activation ─────────────────────────────────────────────
echo "[vote-in-validator] polling for joiner in validator set (max 120s, epochlength=${QBFT_EPOCH_LENGTH} blocks)..."
for i in $(seq 1 60); do
    VALIDATORS=$(rpc_call "${BOOT_RPC}" "qbft_getValidatorsByBlockNumber" '["latest"]' \
        | jq -r '.result[]?' 2>/dev/null | tr '[:upper:]' '[:lower:]') || VALIDATORS=""

    JOINER_LOWER=$(echo "${JOINER_ADDR}" | tr '[:upper:]' '[:lower:]')

    if echo "${VALIDATORS}" | grep -qF "${JOINER_LOWER}"; then
        CURRENT_BLOCK_HEX=$(rpc_call "${BOOT_RPC}" "eth_blockNumber" | jq -r '.result')
        ACTIVATION_BLOCK=$((CURRENT_BLOCK_HEX))
        EPOCH=$((ACTIVATION_BLOCK / QBFT_EPOCH_LENGTH))
        echo ""
        echo "[vote-in-validator] PASS: joiner ${JOINER_ADDR} is now an active validator"
        echo "[vote-in-validator] activation block: ${ACTIVATION_BLOCK} (epoch ${EPOCH}, block % ${QBFT_EPOCH_LENGTH} = $((ACTIVATION_BLOCK % QBFT_EPOCH_LENGTH)))"
        exit 0
    fi

    CURRENT_BLOCK_HEX=$(rpc_call "${BOOT_RPC}" "eth_blockNumber" | jq -r '.result' 2>/dev/null || echo "0x0")
    CURRENT_BLOCK=$((CURRENT_BLOCK_HEX))
    NEXT_EPOCH=$(( (CURRENT_BLOCK / QBFT_EPOCH_LENGTH + 1) * QBFT_EPOCH_LENGTH ))
    printf "[vote-in-validator] poll %d/60: block=%d next_epoch_at=%d\n" "${i}" "${CURRENT_BLOCK}" "${NEXT_EPOCH}"

    sleep 2
done

echo ""
echo "[vote-in-validator] FAIL: joiner did not become active within 120s"
exit 1
