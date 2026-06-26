#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"
BUNDLES_DIR="${SPIKE_ROOT}/bundles"

source "${SCRIPT_DIR}/env-defaults.sh"

# Step 1: Retrieve node info from admin_nodeInfo
NODE_INFO=$(curl -sf -X POST "http://localhost:${HOST_RPC_BOOT}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}')

NODE_ID=$(echo "${NODE_INFO}" | jq -r '.result.id')
NODE_ENODE=$(echo "${NODE_INFO}" | jq -r '.result.enode')

if [ -z "${NODE_ID}" ] || [ "${NODE_ID}" = "null" ]; then
    echo "[FAIL] extract-bundle: could not retrieve node id from RPC at port ${HOST_RPC_BOOT}"
    exit 1
fi

# Step 2: Discover host IP
if ! HOST_IP=$("${SCRIPT_DIR}/resolve-host-ip.sh"); then
    echo "[FAIL] extract-bundle: could not determine host IP (set HOST_IP explicitly)"
    exit 1
fi
echo "[extract-bundle] host IP: ${HOST_IP}"

# Step 3: Reconstruct enode with real host IP
STABLE_ENODE="enode://${NODE_ID}@${HOST_IP}:${HOST_P2P_BOOT}"
echo "[extract-bundle] reconstructed enode: ${STABLE_ENODE}"

# Guard — enode must not contain private Docker IP
if echo "${STABLE_ENODE}" | grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1'; then
    echo "[FAIL] extract-bundle: enode contains private Docker IP: ${STABLE_ENODE}"
    exit 1
fi

# Step 4: Compute genesis hash
GENESIS_HASH=$(sha256sum "${DATA_DIR}/genesis.json" | awk '{print $1}')
echo "[extract-bundle] genesis hash: ${GENESIS_HASH}"

# Step 5: Read registry contract address
REGISTRY_ADDR=""
if [ -f "${DATA_DIR}/.deployed-addrs.env" ]; then
    REGISTRY_ADDR=$(grep -oP 'REGISTRY_CONTRACT_ADDRESS=\K.+' "${DATA_DIR}/.deployed-addrs.env" | tr -d '\n\r' | xargs)
fi

# Step 6: Read CA cert if present
CA_CERT=""
if [ -f "${DATA_DIR}/ca.crt" ]; then
    CA_CERT=$(cat "${DATA_DIR}/ca.crt")
fi

# Step 7: Build and write bundle
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
BUNDLE_FILE="${BUNDLES_DIR}/${SPOKE_ID}.bundle.yaml"

mkdir -p "${BUNDLES_DIR}"

cat > "${BUNDLE_FILE}" << BUNDLEEOF
apiVersion: cbweb3/v1
kind: JoinBundle
metadata:
  spokeId: ${SPOKE_ID}
  createdAt: "${TIMESTAMP}"
spec:
  p2p:
    bootnodeEnode: "${STABLE_ENODE}"
  rpc:
    endpoint: "http://${HOST_IP}:${HOST_RPC_BOOT}"
    wsEndpoint: "ws://${HOST_IP}:${HOST_WS_BOOT}"
  chain:
    genesisHash: "${GENESIS_HASH}"
    epochlength: 30
  pki:
    trustAnchor: "${CA_CERT}"
  contracts:
    identityRegistry: "${REGISTRY_ADDR}"
BUNDLEEOF

echo "[extract-bundle] bundle written to ${BUNDLE_FILE}"
echo "[extract-bundle] bundle is ready for join"
