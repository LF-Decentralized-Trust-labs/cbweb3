#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ADR_FILE="${SPIKE_ROOT}/docs/adr-002-live-validator-join.md"

if [ ! -f "${ADR_FILE}" ]; then
    echo "FAIL T6 adr-evidence-incomplete (ADR file not found at ${ADR_FILE})"
    exit 1
fi

echo "[T6] ADR evidence table completeness check..."

MISSING=""
for TEST_ID in T1 T2 T3 T4 T5 T6; do
    if ! grep -q "| *${TEST_ID}" "${ADR_FILE}"; then
        MISSING="${MISSING} ${TEST_ID}"
    fi
done

if [ -n "${MISSING}" ]; then
    echo "FAIL T6 adr-evidence-incomplete missing=${MISSING}"
    exit 1
fi

# Check that no PENDING stubs remain in the evidence table
if grep -q '\[PENDING\]' "${ADR_FILE}"; then
    echo "FAIL T6 adr-evidence-incomplete (PENDING stubs found)"
    exit 1
fi

ROW_COUNT=$(grep -c "| *T[0-9]" "${ADR_FILE}" || echo "0")
echo "PASS T6 adr-evidence-complete rows=${ROW_COUNT}"
