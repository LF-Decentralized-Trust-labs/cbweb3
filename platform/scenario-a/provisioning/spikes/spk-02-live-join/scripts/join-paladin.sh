#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/env-defaults.sh"

echo "[join-paladin] starting Paladin Bank-X join sequence..."

# Step 1: Generate TLS cert for Bank-X
echo "[join-paladin] generating Bank-X TLS cert..."
"${SCRIPT_DIR}/generate-paladin-certs.sh"

# Step 2: Render Paladin config for Bank-X
echo "[join-paladin] rendering Bank-X config..."
"${SCRIPT_DIR}/render-paladin-configs.sh" bank-x

# Step 3: Start Paladin Bank-X container
echo "[join-paladin] starting Paladin Bank-X..."
cd "${SPIKE_ROOT}"
docker compose -f compose/stack-join.yml up -d paladin-bank-x

# Step 4: Wait for Bank-X RPC
echo "[join-paladin] waiting for Bank-X RPC on ${HOST_PALADIN_BX_RPC}..."
"${SCRIPT_DIR}/wait-for-paladin.sh" "http://localhost:${HOST_PALADIN_BX_RPC}" 90

# Step 5: Register Bank-X on-chain via cast (spike-local, FR-008 safe)
echo "[join-paladin] registering Bank-X Paladin node on-chain..."
"${SCRIPT_DIR}/register-paladin-nodes.sh" bank-x

# Step 6: Wait for event propagation
echo "[join-paladin] waiting 15s for event propagation..."
sleep 15

echo "[join-paladin] Paladin Bank-X join complete"
echo "[join-paladin] Bank-X RPC: http://localhost:${HOST_PALADIN_BX_RPC}"
echo "[join-paladin] Bank-X gRPC: spk02-paladin-bank-x:9000"
