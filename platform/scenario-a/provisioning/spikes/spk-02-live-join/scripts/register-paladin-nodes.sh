#!/usr/bin/env bash
# register-paladin-nodes.sh — Register spoke-spk02 Paladin nodes in the on-chain
# IdentityRegistry using Foundry's cast CLI.
#
# Registers CB, Bank-A (on found stack) and optionally Bank-X (on join stack).
# Usage:
#   bash register-paladin-nodes.sh [all|found|bank-x]
#   all     (default) register CB + Bank-A + Bank-X
#   found   register only CB + Bank-A
#   bank-x  register only Bank-X
#
# Prerequisites: genesis-once.sh run, IdentityRegistry deployed, TLS certs generated.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"
MODE="${1:-all}"

source "${SCRIPT_DIR}/env-defaults.sh"

BESU_RPC="http://localhost:${HOST_RPC_BOOT}"

# Funded operator key — sourced from env-defaults.sh (research context only, from genesis alloc).
PRIVATE_KEY="${OPERATOR_PRIVATE_KEY}"
OWNER_ADDR=$(cast wallet address "${PRIVATE_KEY}")

# Read deployed registry address
ADDRS_FILE="${DATA_DIR}/.deployed-addrs.env"
if [ ! -f "${ADDRS_FILE}" ]; then
    echo "[register-paladin-nodes] ERROR: ${ADDRS_FILE} not found — run start-found.sh first"
    exit 1
fi
REGISTRY_ADDR=$(grep 'REGISTRY_CONTRACT_ADDRESS' "${ADDRS_FILE}" | cut -d= -f2 | tr -d '[:space:]')
if [ -z "${REGISTRY_ADDR}" ]; then
    echo "[register-paladin-nodes] ERROR: REGISTRY_CONTRACT_ADDRESS not in ${ADDRS_FILE}"
    exit 1
fi
echo "[register-paladin-nodes] registry: ${REGISTRY_ADDR}"
echo "[register-paladin-nodes] owner:    ${OWNER_ADDR}"

ZERO_HASH="0x0000000000000000000000000000000000000000000000000000000000000000"

register_node() {
    local node_key="$1"     # cb | bank-a | bank-x
    local node_name="$2"    # PALADIN_{CB,BA,BX}_NODE_NAME from env-defaults.sh
    local grpc_host="$3"    # hostname for gRPC endpoint
    local grpc_port="$4"    # host-mapped gRPC port
    local cert_file="${DATA_DIR}/paladin/${node_key}/tls.crt"

    if [ ! -f "${cert_file}" ]; then
        echo "[register-paladin-nodes] ERROR: cert not found at ${cert_file} — run generate-paladin-certs.sh first"
        exit 1
    fi

    echo ""
    echo "[register-paladin-nodes] === registering '${node_name}' ==="

    # Step 1: registerIdentity(zeroHash, name, ownerAddr)
    echo "[register-paladin-nodes]   calling registerIdentity..."
    RECEIPT=$(cast send \
        --rpc-url "${BESU_RPC}" \
        --private-key "${PRIVATE_KEY}" \
        --legacy \
        --gas-limit 300000 \
        --timeout 600 \
        --confirmations 0 \
        "${REGISTRY_ADDR}" \
        "registerIdentity(bytes32,string,address)" \
        "${ZERO_HASH}" "${node_name}" "${OWNER_ADDR}" \
        --json)

    # Extract identityHash from the IdentityRegistered event data.
    # Event ABI: IdentityRegistered(bytes32 parentHash, bytes32 identityHash, string name, address owner)
    # All params are non-indexed → packed into log data.
    # identityHash is the 2nd bytes32 in data: chars [2+64..2+64+64) of the 0x-prefixed hex string.
    LOG_DATA=$(echo "${RECEIPT}" | jq -r '.logs[0].data // ""')
    if [ -z "${LOG_DATA}" ] || [ "${LOG_DATA}" = "null" ]; then
        echo "[register-paladin-nodes]   WARN: no event log in receipt — registration may have failed"
        echo "[register-paladin-nodes]   receipt: $(echo "${RECEIPT}" | jq -r '.status')"
        return 1
    fi
    IDENTITY_HASH="0x${LOG_DATA:66:64}"
    echo "[register-paladin-nodes]   registered: identityHash=${IDENTITY_HASH}"

    # Step 2: setIdentityProperty(identityHash, "transport.grpc", transportJSON)
    # Transport JSON follows the Paladin evm-registry schema: endpoint + issuers (TLS cert PEM).
    TRANSPORT_JSON=$(jq -nc \
        --arg endpoint "dns:///${grpc_host}:${grpc_port}" \
        --arg issuers "$(cat "${cert_file}")" \
        '{endpoint: $endpoint, issuers: $issuers}')

    echo "[register-paladin-nodes]   calling setIdentityProperty (transport.grpc)..."
    sleep 3
    TX_RECEIPT=$(cast send \
        --rpc-url "${BESU_RPC}" \
        --private-key "${PRIVATE_KEY}" \
        --legacy \
        --gas-limit 2000000 \
        --timeout 600 \
        --confirmations 0 \
        "${REGISTRY_ADDR}" \
        "setIdentityProperty(bytes32,string,string)" \
        "${IDENTITY_HASH}" "transport.grpc" "${TRANSPORT_JSON}" \
        --json)
    TX_STATUS=$(echo "${TX_RECEIPT}" | jq -r '.status // "unknown"')
    if [ "${TX_STATUS}" != "0x1" ]; then
        echo "[register-paladin-nodes] ERROR: setIdentityProperty tx failed with status=${TX_STATUS}"
        exit 1
    fi

    echo "[register-paladin-nodes]   published transport: endpoint=dns:///${grpc_host}:${grpc_port}"
    echo "[register-paladin-nodes]   cert: ${cert_file} ($(wc -c < "${cert_file}") bytes)"

    # Persist the identity hash for downstream scripts
    echo "${node_name}_IDENTITY_HASH=${IDENTITY_HASH}" >> "${ADDRS_FILE}"
    echo "[register-paladin-nodes]   saved ${node_name}_IDENTITY_HASH to ${ADDRS_FILE}"
}

case "${MODE}" in
    found|all)
        # CB and Bank-A are on spk02_found_net — they communicate via Docker DNS (internal).
        register_node "cb"     "${PALADIN_CB_NODE_NAME}" "${CONTAINER_PALADIN_CB}" "9000"
        sleep 5
        register_node "bank-a" "${PALADIN_BA_NODE_NAME}" "${CONTAINER_PALADIN_BA}" "9000"
        ;;
esac

case "${MODE}" in
    bank-x|all)
        # Bank-X is on spk02_join_net — CB must reach it via host.docker.internal + mapped port.
        # The gRPC host-port mapping is HOST_PALADIN_BX_GRPC (9702) → container 9000.
        register_node "bank-x" "${PALADIN_BX_NODE_NAME}" "host.docker.internal" "${HOST_PALADIN_BX_GRPC}"
        ;;
esac

echo ""
echo "[register-paladin-nodes] === Registration complete for mode: ${MODE} ==="
