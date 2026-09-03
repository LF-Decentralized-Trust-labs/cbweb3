#!/usr/bin/env bash
set -euo pipefail

NODE="${1:-}"
if [ -z "${NODE}" ]; then
    echo "Usage: $0 <cb|bank-a|bank-x>"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"
ADDRS_FILE="${DATA_DIR}/.deployed-addrs.env"

if [ ! -f "${ADDRS_FILE}" ]; then
    echo "[render-paladin-configs] ERROR: ${ADDRS_FILE} not found — run deploy-registry first"
    exit 1
fi

# Extract REGISTRY_CONTRACT_ADDRESS from the env file (avoid 'source' because
# identity hash lines contain hyphens, which are invalid shell variable names).
REGISTRY_CONTRACT_ADDRESS=$(grep '^REGISTRY_CONTRACT_ADDRESS=' "${ADDRS_FILE}" | cut -d= -f2- | tr -d '[:space:]')
if [ -z "${REGISTRY_CONTRACT_ADDRESS}" ]; then
    echo "[render-paladin-configs] ERROR: REGISTRY_CONTRACT_ADDRESS not set in ${ADDRS_FILE}"
    exit 1
fi

TMPL_FILE="${DATA_DIR}/paladin/${NODE}/config.yaml.tmpl"
OUTPUT_FILE="${DATA_DIR}/paladin/${NODE}/config.yaml"

if [ ! -f "${TMPL_FILE}" ]; then
    echo "[render-paladin-configs] ERROR: template not found: ${TMPL_FILE}"
    exit 1
fi

export REGISTRY_CONTRACT_ADDRESS

envsubst '${REGISTRY_CONTRACT_ADDRESS}' < "${TMPL_FILE}" > "${OUTPUT_FILE}"

echo "[render-paladin-configs] rendered ${OUTPUT_FILE} for node=${NODE}"
echo "[render-paladin-configs] registry address: ${REGISTRY_CONTRACT_ADDRESS}"
