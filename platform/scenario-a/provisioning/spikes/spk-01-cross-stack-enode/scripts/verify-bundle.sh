#!/usr/bin/env bash
set -euo pipefail

BUNDLE_FILE="${1:-}"

if [ -z "${BUNDLE_FILE}" ]; then
    echo "Usage: $0 <bundle-file.yaml>"
    exit 1
fi

if [ ! -f "${BUNDLE_FILE}" ]; then
    echo "FAIL T6 bundle-clean not_found file=${BUNDLE_FILE}"
    exit 1
fi

# Guard: no Docker bridge IPs
if grep -qE '172\.(1[6-9]|2[0-9]|3[01])\.|127\.0\.0\.1' "${BUNDLE_FILE}"; then
    echo "FAIL T6 bundle-clean PRIVATE_IP"
    exit 1
fi

# Guard: no private key material
if grep -qE '\-\-\-\-\-BEGIN.*(PRIVATE|ENCRYPTED).*KEY' "${BUNDLE_FILE}"; then
    echo "FAIL T6 bundle-clean PRIVATE_KEY"
    exit 1
fi

# Guard: required fields present
HAS_ENODE=$(grep -c 'bootnodeEnode:' "${BUNDLE_FILE}" || true)
HAS_RPC=$(grep -c 'endpoint:' "${BUNDLE_FILE}" || true)
HAS_WS=$(grep -c 'wsEndpoint:' "${BUNDLE_FILE}" || true)
HAS_GENESIS=$(grep -c 'genesisHash:' "${BUNDLE_FILE}" || true)

MISSING=""
[ "${HAS_ENODE}" -gt 0 ] || MISSING="${MISSING} bootnodeEnode"
[ "${HAS_RPC}" -gt 0 ] || MISSING="${MISSING} endpoint"
[ "${HAS_WS}" -gt 0 ] || MISSING="${MISSING} wsEndpoint"
[ "${HAS_GENESIS}" -gt 0 ] || MISSING="${MISSING} genesisHash"

if [ -n "${MISSING}" ]; then
    echo "FAIL T6 bundle-clean MISSING_FIELDS:${MISSING}"
    exit 1
fi

echo "PASS T6 bundle-clean fields=ok secrets=none"
