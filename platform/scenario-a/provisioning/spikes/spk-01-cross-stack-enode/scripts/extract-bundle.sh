#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"
BUNDLES_DIR="${SPIKE_ROOT}/bundles"

source "${SCRIPT_DIR}/env-defaults.sh"

# Step 1: Retrieve enode from admin_nodeInfo
ENODE=$(curl -sf -X POST "http://localhost:${HOST_RPC_BOOT}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
    | jq -r '.result.enode')

if [ -z "${ENODE}" ] || [ "${ENODE}" = "null" ]; then
    echo "[FAIL] extract-bundle: could not retrieve enode from RPC at port ${HOST_RPC_BOOT}"
    exit 1
fi

# Step 2: Guard — enode must not contain private Docker IP
if echo "${ENODE}" | grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1'; then
    echo "[FAIL] extract-bundle: enode contains private Docker IP: ${ENODE}"
    echo "[FAIL] Ensure bootnode is started with --nat-method=DOCKER and host port is published"
    exit 1
fi

# Step 3: Compute genesis hash
GENESIS_HASH=$(sha256sum "${DATA_DIR}/genesis.json" | awk '{print $1}')
echo "[extract-bundle] genesis hash: ${GENESIS_HASH}"

# Step 4: Read CA certificate if present
CA_CERT=""
if [ -f "${DATA_DIR}/ca.crt" ]; then
    CA_CERT=$(cat "${DATA_DIR}/ca.crt")
fi

# Step 5: Build and write bundle
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
    bootnodeEnode: "${ENODE}"
  rpc:
    endpoint: "http://host.docker.internal:${HOST_RPC_BOOT}"
    wsEndpoint: "ws://host.docker.internal:${HOST_WS_BOOT}"
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

# Step 6: Self-validate
if ! "${SCRIPT_DIR}/verify-bundle.sh" "${BUNDLE_FILE}"; then
    echo "[FAIL] extract-bundle: bundle validation failed"
    exit 1
fi

echo "[extract-bundle] bundle is clean and ready for join"
