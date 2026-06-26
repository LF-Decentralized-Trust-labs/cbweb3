#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
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

# Step 2: Extract bootnode public key (pass to compose via env for validators to use)
BOOTNODE_PUBKEY=$(cat "${DATA_DIR}/key0.pub" 2>/dev/null | tr -d '\n\r' | sed 's/^0x//')
if [ -z "${BOOTNODE_PUBKEY}" ] || [ "${#BOOTNODE_PUBKEY}" -lt 64 ]; then
    echo "[start-found] ERROR: could not extract bootnode public key from ${DATA_DIR}/key0.pub"
    exit 1
fi
export BOOTNODE_PUBKEY
echo "[start-found] bootnode pubkey: ${BOOTNODE_PUBKEY}"

# Step 3: Start the full found stack (bootnode + validators)
# Validators resolve "besu-boot" via DNS inside the container (getent hosts) — no `docker inspect`.
cd "${SPIKE_ROOT}"
docker compose -f compose/stack-found.yml up -d

# Step 4: Wait for blocks
echo "[start-found] waiting for block production..."
"${SCRIPT_DIR}/wait-for-blocks.sh" "http://localhost:${HOST_RPC_BOOT}" 60
