#!/usr/bin/env bash
set -euo pipefail

GENESIS_FILE="${1:-./data/genesis.json}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONFIG_FILE="${SPIKE_ROOT}/compose/genesis-config.json"
DATA_DIR="${SPIKE_ROOT}/data"
TEMP_DIR="${DATA_DIR}/networkFiles"

if [ -f "${GENESIS_FILE}" ]; then
    echo "[genesis-once] SKIP: genesis already exists at ${GENESIS_FILE}"
    exit 0
fi

echo "[genesis-once] generating genesis from config: ${CONFIG_FILE}"

mkdir -p "${DATA_DIR}"

rm -rf "${TEMP_DIR}"

docker run --rm \
    --user "$(id -u):$(id -g)" \
    -v "${SPIKE_ROOT}:/work" \
    hyperledger/besu:25.8.0 \
    operator generate-blockchain-config \
    --config-file=/work/compose/genesis-config.json \
    --to=/work/data/networkFiles \
    --private-key-file-name=key

cp "${TEMP_DIR}/genesis.json" "${GENESIS_FILE}"

IDX=0
for NODE_DIR in $(ls -d "${TEMP_DIR}/keys/"*/); do
    NODE_DIR="${NODE_DIR%/}"
    cp "${NODE_DIR}/key" "${DATA_DIR}/key${IDX}"
    cp "${NODE_DIR}/key.pub" "${DATA_DIR}/key${IDX}.pub"
    IDX=$((IDX + 1))
done

rm -rf "${TEMP_DIR}"

echo "[genesis-once] CREATED: ${GENESIS_FILE} (4 node keys generated)"
