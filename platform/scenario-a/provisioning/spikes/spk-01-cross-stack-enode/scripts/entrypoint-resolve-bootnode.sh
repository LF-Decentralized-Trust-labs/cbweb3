#!/bin/bash
# entrypoint-resolve-bootnode.sh
# Resolves BOOTNODE_DNS to its IP at container startup and passes it to Besu.
# No `docker inspect` — pure DNS resolution inside the container.

set -e

BOOTNODE_DNS="${BOOTNODE_DNS:-besu-boot}"
BOOTNODE_PUBKEY="${BOOTNODE_PUBKEY:-}"
BOOTNODE_PORT="${BOOTNODE_PORT:-30303}"
MAX_RETRIES=30
RETRY_DELAY=2

echo "[entrypoint] resolving ${BOOTNODE_DNS} to IP..."

for i in $(seq 1 "${MAX_RETRIES}"); do
    BOOTNODE_IP=$(getent hosts "${BOOTNODE_DNS}" 2>/dev/null | awk '{print $1}')
    if [ -n "${BOOTNODE_IP}" ]; then
        echo "[entrypoint] resolved ${BOOTNODE_DNS} -> ${BOOTNODE_IP}"
        break
    fi
    echo "[entrypoint] attempt ${i}/${MAX_RETRIES}: ${BOOTNODE_DNS} not resolved yet, retrying in ${RETRY_DELAY}s..."
    sleep "${RETRY_DELAY}"
done

if [ -z "${BOOTNODE_IP}" ]; then
    echo "[entrypoint] ERROR: failed to resolve ${BOOTNODE_DNS} after ${MAX_RETRIES} attempts"
    exit 1
fi

# Reconstruct the bootnode enode with the resolved IP
BOOTNODE_ENODE="enode://${BOOTNODE_PUBKEY}@${BOOTNODE_IP}:${BOOTNODE_PORT}"
echo "[entrypoint] bootnode enode: ${BOOTNODE_ENODE}"

# Exec Besu with all original args, appending --bootnodes
exec /opt/besu/bin/besu \
    --bootnodes="${BOOTNODE_ENODE}" \
    "$@"
