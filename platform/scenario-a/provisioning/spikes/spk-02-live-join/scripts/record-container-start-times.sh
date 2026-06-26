#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"

source "${SCRIPT_DIR}/env-defaults.sh"

echo "[record-container-start-times] capturing baseline start times..."

mkdir -p "${DATA_DIR}"

declare -A STARTS
declare -a CONTAINERS=(
    "${CONTAINER_BESU_BOOT}"
    "${CONTAINER_BESU_V1}"
    "${CONTAINER_BESU_V2}"
    "${CONTAINER_PALADIN_CB}"
    "${CONTAINER_PALADIN_BA}"
)

for CONTAINER in "${CONTAINERS[@]}"; do
    if docker inspect "${CONTAINER}" &>/dev/null; then
        STARTED_AT=$(docker inspect --format '{{.State.StartedAt}}' "${CONTAINER}" 2>/dev/null)
        echo "[record-container-start-times] ${CONTAINER}: ${STARTED_AT}"
        STARTS["${CONTAINER}"]="${STARTED_AT}"
    else
        echo "[record-container-start-times] WARNING: ${CONTAINER} not found (skipping)"
    fi
done

# Write results
echo "{" > "${DATA_DIR}/baseline-start-times.json"
FIRST=true
for CONTAINER in "${!STARTS[@]}"; do
    if [ "${FIRST}" = true ]; then
        FIRST=false
    else
        echo "," >> "${DATA_DIR}/baseline-start-times.json"
    fi
    printf '  "%s": "%s"' "${CONTAINER}" "${STARTS[$CONTAINER]}" >> "${DATA_DIR}/baseline-start-times.json"
done
echo "" >> "${DATA_DIR}/baseline-start-times.json"
echo "}" >> "${DATA_DIR}/baseline-start-times.json"

# Write captured-at timestamp
CAPTURED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "\"${CAPTURED_AT}\"" > "${DATA_DIR}/baseline-captured-at.json" 2>/dev/null || \
    echo "${CAPTURED_AT}" > "${DATA_DIR}/baseline-captured-at.json"

echo "[record-container-start-times] baseline written to ${DATA_DIR}/baseline-start-times.json"
