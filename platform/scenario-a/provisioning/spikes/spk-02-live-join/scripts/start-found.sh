#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SCENARIO_ROOT="$(cd "${SPIKE_ROOT}/../../.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"

source "${SCRIPT_DIR}/env-defaults.sh"

# Port availability preflight
for PORT in "${HOST_P2P_BOOT}" "${HOST_RPC_BOOT}" "${HOST_WS_BOOT}"; do
    if ss -tlnp 2>/dev/null | grep -q ":${PORT} "; then
        echo "[start-found] ERROR: port ${PORT} already in use — cannot start"
        exit 1
    fi
    if command -v lsof &>/dev/null && lsof -i ":${PORT}" -sTCP:LISTEN &>/dev/null; then
        echo "[start-found] ERROR: port ${PORT} already in use — cannot start"
        exit 1
    fi
done

# Step 1: Generate genesis (idempotent)
echo "[start-found] ensuring genesis exists"
"${SCRIPT_DIR}/genesis-once.sh" "${DATA_DIR}/genesis.json"

# Step 2: Extract bootnode public key
BOOTNODE_PUBKEY=$(cat "${DATA_DIR}/key0.pub" 2>/dev/null | tr -d '\n\r' | sed 's/^0x//')
if [ -z "${BOOTNODE_PUBKEY}" ] || [ "${#BOOTNODE_PUBKEY}" -lt 64 ]; then
    echo "[start-found] ERROR: could not extract bootnode public key from ${DATA_DIR}/key0.pub"
    exit 1
fi
export BOOTNODE_PUBKEY
echo "[start-found] bootnode pubkey: ${BOOTNODE_PUBKEY}"

# Step 3: Start Besu found stack (3 validators)
echo "[start-found] starting Besu found stack..."
cd "${SPIKE_ROOT}"
docker compose -f compose/stack-found.yml up -d besu-boot besu-v1 besu-v2

# Step 4: Wait for RPC to be ready then wait for block production
echo "[start-found] waiting for Besu RPC on port ${HOST_RPC_BOOT}..."
"${SCRIPT_DIR}/wait-for-rpc.sh" "http://localhost:${HOST_RPC_BOOT}" 60

echo "[start-found] waiting for block production..."
"${SCRIPT_DIR}/wait-for-blocks.sh" "http://localhost:${HOST_RPC_BOOT}" 30

# Step 5: Deploy IdentityRegistry contract using existing Go test
echo "[start-found] deploying IdentityRegistry contract..."
PALADIN_SCRIPTS="${SCENARIO_ROOT}/deploy/local/paladin/scripts"

# Ensure the ARTIFACTS_DIR points to the contracts directory
CONTRACTS_DIR="${SCENARIO_ROOT}/deploy/local/paladin/contracts"
if [ ! -d "${CONTRACTS_DIR}" ]; then
    CONTRACTS_DIR="${SCENARIO_ROOT}/../../../scenario-a/deploy/local/paladin/contracts"
fi

REGISTRY_DEPLOY_OUTPUT=$(cd "${PALADIN_SCRIPTS}" && \
    SPOKE=spoke-a \
    BESU_RPC_URL="http://localhost:${HOST_RPC_BOOT}" \
    ARTIFACTS_DIR="${CONTRACTS_DIR}" \
    go test ./... -run TestDeployEVMRegistry -v -count=1 -timeout 5m 2>&1)
echo "${REGISTRY_DEPLOY_OUTPUT}"

REGISTRY_ADDR=$(echo "${REGISTRY_DEPLOY_OUTPUT}" | grep -oP 'REGISTRY_CONTRACT_ADDRESS=\K0x[a-fA-F0-9]{40}' | tail -1)
if [ -z "${REGISTRY_ADDR}" ]; then
    echo "[start-found] WARNING: could not parse REGISTRY_CONTRACT_ADDRESS from test output"
    echo "[start-found] attempting to read from spoke-a .deployed-addrs.env..."
    REGISTRY_ADDR=$(grep -oP 'REGISTRY_CONTRACT_ADDRESS=\K0x[a-fA-F0-9]{40}' "${SCENARIO_ROOT}/deploy/local/paladin/spoke-a/.deployed-addrs.env" 2>/dev/null | tail -1 || true)
fi

if [ -z "${REGISTRY_ADDR}" ]; then
    echo "[start-found] ERROR: could not determine REGISTRY_CONTRACT_ADDRESS"
    exit 1
fi

echo "REGISTRY_CONTRACT_ADDRESS=${REGISTRY_ADDR}" > "${DATA_DIR}/.deployed-addrs.env"
echo "[start-found] REGISTRY_CONTRACT_ADDRESS=${REGISTRY_ADDR}"

# Step 6: Generate Paladin TLS certs
echo "[start-found] generating Paladin TLS certs..."
"${SCRIPT_DIR}/generate-paladin-certs.sh"

# Step 7: Render Paladin configs for CB and Bank-A
echo "[start-found] rendering Paladin configs..."
"${SCRIPT_DIR}/render-paladin-configs.sh" cb
"${SCRIPT_DIR}/render-paladin-configs.sh" bank-a

# Step 8: Register CB + Bank-A nodes on-chain
echo "[start-found] registering CB and Bank-A Paladin nodes on-chain..."
"${SCRIPT_DIR}/register-paladin-nodes.sh" found
echo "[start-found] === Besu found stack is running ==="
echo "[start-found] Besu RPC: http://localhost:${HOST_RPC_BOOT}"
echo "[start-found] Registry: ${REGISTRY_ADDR}"

# Step 9: Start Paladin CB and Bank-A
echo "[start-found] starting Paladin CB and Bank-A..."
docker compose -f compose/stack-found.yml up -d paladin-cb paladin-bank-a

# Step 10: Wait for Paladin CB RPC
echo "[start-found] waiting for Paladin CB RPC on ${HOST_PALADIN_CB_RPC}..."
"${SCRIPT_DIR}/wait-for-paladin.sh" "http://localhost:${HOST_PALADIN_CB_RPC}" 90

echo "[start-found] === Full found stack is running ==="
echo "[start-found] Besu RPC:     http://localhost:${HOST_RPC_BOOT}"
echo "[start-found] Paladin CB:   http://localhost:${HOST_PALADIN_CB_RPC}"
echo "[start-found] Paladin BankA: http://localhost:${HOST_PALADIN_BA_RPC}"
