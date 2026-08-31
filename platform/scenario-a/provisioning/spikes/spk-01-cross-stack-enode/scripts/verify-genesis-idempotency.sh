#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GENESIS_FILE="${1:-./data/genesis.json}"

GENESIS_ONCE="${SCRIPT_DIR}/genesis-once.sh"

if [ ! -f "${GENESIS_ONCE}" ]; then
    echo "FAIL T1 genesis-idempotency genesis-once.sh not found"
    exit 1
fi

if [ ! -f "${GENESIS_FILE}" ]; then
    echo "FAIL T1 genesis-idempotency genesis file does not exist, run genesis-once.sh first"
    exit 1
fi

CHECKSUM_BEFORE=$(sha256sum "${GENESIS_FILE}" | awk '{print $1}')

OUTPUT=$("${GENESIS_ONCE}" "${GENESIS_FILE}" 2>&1)
EXIT_CODE=$?

if [ "${EXIT_CODE}" -ne 0 ]; then
    echo "FAIL T1 genesis-idempotency genesis-once.sh exited non-zero on second run"
    exit 1
fi

CHECKSUM_AFTER=$(sha256sum "${GENESIS_FILE}" | awk '{print $1}')

if ! echo "${OUTPUT}" | grep -qi "SKIP"; then
    echo "FAIL T1 genesis-idempotency second run did not print SKIP, got: ${OUTPUT}"
    exit 1
fi

if [ "${CHECKSUM_BEFORE}" != "${CHECKSUM_AFTER}" ]; then
    echo "FAIL T1 genesis-idempotency checksum changed: ${CHECKSUM_BEFORE} -> ${CHECKSUM_AFTER}"
    exit 1
fi

echo "PASS T1 genesis-idempotency checksum=${CHECKSUM_AFTER} unchanged"
