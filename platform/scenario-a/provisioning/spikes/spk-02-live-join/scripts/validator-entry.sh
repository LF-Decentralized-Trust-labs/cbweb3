#!/bin/bash
# validator-entry.sh
# Besu's --bootnodes flag requires an IP address, not a hostname.
# This script resolves besu-boot via Docker DNS (getent hosts) and
# constructs the bootnode enode with the resolved IP before exec'ing Besu.
set -e

BOOTNODE_PUBKEY="${BOOTNODE_PUBKEY:-}"
BOOT_DNS="${BOOT_DNS:-besu-boot}"
BOOT_PORT="${BOOT_PORT:-30303}"

if [ -z "${BOOTNODE_PUBKEY}" ]; then
    echo "[validator-entry] FATAL: BOOTNODE_PUBKEY not set"
    exit 1
fi

echo "[validator-entry] resolving ${BOOT_DNS} via Docker DNS..."
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

RPC_ARGS=""
if [ "${ENABLE_RPC:-false}" = "true" ]; then
    RPC_ARGS="--rpc-http-enabled=true --rpc-http-port=8545 --rpc-http-api=ETH,NET,QBFT,ADMIN --rpc-http-host=0.0.0.0 --rpc-http-cors-origins=all"
else
    RPC_ARGS="--rpc-http-enabled=false --rpc-ws-enabled=false"
fi

# shellcheck disable=SC2086
exec /opt/besu/bin/besu \
    --nat-method=NONE \
    --bootnodes="${BOOTNODE_ENODE}" \
    --p2p-port=30303 \
    --host-allowlist=* \
    --genesis-file=/opt/besu/genesis/genesis.json \
    --data-path=/opt/besu/data \
    --node-private-key-file=/opt/besu/data/key \
    --min-gas-price=0 \
    ${RPC_ARGS}
