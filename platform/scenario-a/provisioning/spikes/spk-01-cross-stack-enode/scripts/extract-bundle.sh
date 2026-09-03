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

# Step 2: Discover host IP via resolve-host-ip.sh (hostname -I on Linux, route on macOS).
# We use the real LAN IP instead of host.docker.internal because Besu 25.8.0 returns
# 127.0.0.1 in admin_nodeInfo.enode even with --nat-method=DOCKER, making the RPC-reported
# enode unusable. The LAN IP is reachable from any Docker container on the same host
# without requiring extra_hosts, and avoids the Linux-specific host.docker.internal quirk.
# Override: export HOST_IP=<addr> before calling this script.
if ! HOST_IP=$("${SCRIPT_DIR}/resolve-host-ip.sh"); then
    echo "[FAIL] extract-bundle: could not determine host IP (set HOST_IP explicitly)"
    exit 1
fi
echo "[extract-bundle] host IP: ${HOST_IP}"

# Step 3: Reconstruct enode using admin_nodeInfo pubkey + real host IP + published P2P port.
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

# Step 5: Read CA certificate if present
CA_CERT=""
if [ -f "${DATA_DIR}/ca.crt" ]; then
    CA_CERT=$(cat "${DATA_DIR}/ca.crt")
fi

# Step 6: Build and write bundle
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
  pki:
    trustAnchor: "${CA_CERT}"
  contracts:
    identityRegistry: ""
    fxAgreement: ""
    zetoFactory: ""
BUNDLEEOF

echo "[extract-bundle] bundle written to ${BUNDLE_FILE}"

# Step 7: Self-validate
if ! "${SCRIPT_DIR}/verify-bundle.sh" "${BUNDLE_FILE}"; then
    echo "[FAIL] extract-bundle: bundle validation failed"
    exit 1
fi

echo "[extract-bundle] bundle is clean and ready for join"
