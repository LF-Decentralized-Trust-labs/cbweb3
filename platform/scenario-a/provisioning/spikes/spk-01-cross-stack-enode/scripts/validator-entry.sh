#!/bin/bash
# validator-entry.sh
# Resolves besu-boot via DNS inside the container (NOT `docker inspect`) and starts Besu.
set -e

BOOTNODE_PUBKEY="${BOOTNODE_PUBKEY:-}"
BOOT_DNS="${BOOT_DNS:-besu-boot}"
BOOT_PORT="${BOOT_PORT:-30303}"

if [ -z "${BOOTNODE_PUBKEY}" ]; then
    echo "[validator-entry] FATAL: BOOTNODE_PUBKEY not set"
    exit 1
fi

echo "[validator-entry] resolving ${BOOT_DNS} via DNS..."
for i in $(seq 1 30); do
    BOOT_IP=$(getent hosts "${BOOT_DNS}" 2>/dev/null | awk '{print $1}')
    if [ -n "${BOOT_IP}" ]; then
        echo "[validator-entry] ${BOOT_DNS} resolved to ${BOOT_IP}"
        break
    fi
    echo "[validator-entry] attempt ${i}/30: ${BOOT_DNS} not yet resolvable"
    sleep 2
done

if [ -z "${BOOT_IP}" ]; then
    echo "[validator-entry] FATAL: could not resolve ${BOOT_DNS} after 30 attempts"
    exit 1
fi

BOOTNODE_ENODE="enode://${BOOTNODE_PUBKEY}@${BOOT_IP}:${BOOT_PORT}"
echo "[validator-entry] bootnodes: ${BOOTNODE_ENODE}"

exec /opt/besu/bin/besu \
    --nat-method=NONE \
    --bootnodes="${BOOTNODE_ENODE}" \
    --p2p-port=30303 \
    --rpc-http-enabled=false \
    --rpc-ws-enabled=false \
    --host-allowlist=* \
    --genesis-file=/opt/besu/genesis/genesis.json \
    --data-path=/opt/besu/data \
    --node-private-key-file=/opt/besu/data/key \
    --min-gas-price=0
