#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"

source "${SCRIPT_DIR}/env-defaults.sh"

echo "[T5] Besu validator restart check..."

if [ ! -f "${DATA_DIR}/baseline-start-times.json" ]; then
    echo "FAIL T5 no-besu-validator-restart (baseline not found)"
    exit 1
fi

ALL_OK=true

for CONTAINER in "${CONTAINER_BESU_BOOT}" "${CONTAINER_BESU_V1}" "${CONTAINER_BESU_V2}" "${CONTAINER_PALADIN_CB}" "${CONTAINER_PALADIN_BA}"; do
    if ! docker inspect "${CONTAINER}" &>/dev/null; then
        echo "[T5] WARNING: ${CONTAINER} not running (skip)"
        continue
    fi

    CURRENT_START=$(docker inspect --format '{{.State.StartedAt}}' "${CONTAINER}" 2>/dev/null)

    BASELINE_START=$(grep "\"${CONTAINER}\"" "${DATA_DIR}/baseline-start-times.json" | sed 's/.*: *"\(.*\)".*/\1/' 2>/dev/null || echo "")

    if [ -z "${BASELINE_START}" ]; then
        echo "[T5] WARNING: no baseline for ${CONTAINER} (skip)"
        continue
    fi

    if [ "${CURRENT_START}" != "${BASELINE_START}" ]; then
        echo "[T5] RESTART DETECTED: ${CONTAINER} before=${BASELINE_START} after=${CURRENT_START}"
        ALL_OK=false
    else
        echo "[T5] ${CONTAINER}: no restart (started=${CURRENT_START})"
    fi
done

if [ "${ALL_OK}" = true ]; then
    echo "PASS T5 no-besu-validator-restart"
else
    echo "FAIL T5 restart-detected"
    exit 1
fi
