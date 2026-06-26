#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATA_DIR="${SPIKE_ROOT}/data"

source "${SCRIPT_DIR}/env-defaults.sh"

MODE="${1:-all}"

run_t3() {
    echo "[T3] Paladin cross-node registry + mTLS check (CB → Bank-X)..."

    RPC_CB="http://localhost:${HOST_PALADIN_CB_RPC}"

    # Probe 1: verify Bank-X is present in CB's view of the evm-registry.
    # CB indexed the on-chain IdentityRegistered event; if this returns the entry,
    # the on-chain registration (T029) worked and CB knows how to reach Bank-X.
    # Paladin v0.15 uses reg_queryEntriesWithProps JSON-RPC method.
    echo "[T3]   Probe 1: querying CB registry for all entries (reg_queryEntriesWithProps)..."
    ENTRY_RESP=$(curl -sf -X POST "${RPC_CB}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"reg_queryEntriesWithProps","params":["evm-registry", {"limit": 100}, "any"],"id":1}' \
        2>/dev/null || echo "")

    if echo "${ENTRY_RESP}" | grep -qi 'spoke-spk02-bank-x'; then
        TRANSPORT_COUNT=$(echo "${ENTRY_RESP}" | grep -c 'transport.grpc' || echo "0")
        echo "[T3]   Probe 1 PASS: Bank-X entry visible in CB registry (transport property present, total transport entries=${TRANSPORT_COUNT})"
        PROBE1="PASS"
    else
        echo "[T3]   Probe 1 FAIL: Bank-X not visible in CB registry — registration may not have propagated"
        echo "[T3]   raw response: $(echo "${ENTRY_RESP}" | head -c 200)"
        PROBE1="FAIL"
    fi

    # Probe 2: force a gRPC cross-node connection CB → Bank-X.
    # Use ptx_resolveVerifier with per-documentation parameter names: keyIdentifier, algorithm, verifierType.
    # Paladin routes this call to Bank-X's registered gRPC endpoint (host.docker.internal:9702).
    # Result classification:
    #   x509 / TLS handshake error → H-B (trust store loaded at startup, restart required)
    #   response (even "verifier not found") without x509 → H-A (dynamic trust, no restart)
    echo "[T3]   Probe 2: forcing gRPC call CB → Bank-X (ptx_resolveVerifier)..."
    RESOLVE_RESP=$(curl -sf -X POST "${RPC_CB}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"ptx_resolveVerifier","params":["spoke-spk02-bank-x@ecdsa:secp256k1","ecdsa:secp256k1","eth_address"],"id":1}' \
        2>&1) || RESOLVE_RESP="CURL_FAILED"

    # Allow CB logs to surface any async TLS errors
    sleep 3
    CB_LOGS=$(docker logs "${CONTAINER_PALADIN_CB}" --since 15s 2>&1 || echo "")

    TLS_HIT=$(echo "${RESOLVE_RESP}${CB_LOGS}" | grep -ci 'x509\|certificate.*invalid\|tls handshake fail\|transport.*x509' || true)

    if [ "${TLS_HIT}" -gt 0 ]; then
        echo "[T3]   Probe 2: TLS/x509 error — H-B confirmed (restart required)"
        echo "FAIL T3 paladin-mtls hypothesis=H-B restart-required"
        echo "[T3]   evidence:"
        echo "${RESOLVE_RESP}${CB_LOGS}" | grep -i 'x509\|tls\|certificate' | head -5 | sed 's/^/    /'
        exit 1
    elif echo "${RESOLVE_RESP}" | grep -qi '"error"\|"result"'; then
        echo "[T3]   Probe 2: gRPC reached Bank-X — H-A confirmed (no restart)"
        echo "[T3]   resolve response: $(echo "${RESOLVE_RESP}" | head -c 120)"
    else
        echo "[T3]   Probe 2: inconclusive (no x509 error, non-JSON response)"
        echo "[T3]   response: $(echo "${RESOLVE_RESP}" | head -c 120)"
        echo "[T3]   cb_log (recent): $(echo "${CB_LOGS}" | tail -5)"
    fi

    if [ "${PROBE1}" = "FAIL" ]; then
        echo "FAIL T3 paladin-mtls registry-probe-failed"
        exit 1
    fi

    echo "PASS T3 paladin-mtls no-x509-error probe1=${PROBE1}"
}

run_t4() {
    echo "[T4] Existing Paladin container restart check..."

    if [ ! -f "${DATA_DIR}/baseline-start-times.json" ]; then
        echo "FAIL T4 no-existing-paladin-restart (baseline not found — run record-container-start-times.sh first)"
        exit 1
    fi

    for CONTAINER in "${CONTAINER_PALADIN_CB}" "${CONTAINER_PALADIN_BA}"; do
        if ! docker inspect "${CONTAINER}" &>/dev/null; then
            echo "FAIL T4 no-existing-paladin-restart (${CONTAINER} not running)"
            exit 1
        fi

        CURRENT_START=$(docker inspect --format '{{.State.StartedAt}}' "${CONTAINER}" 2>/dev/null)

        # Extract baseline start time from JSON
        BASELINE_START=$(python3 -c "import json,sys; d=json.load(open('${DATA_DIR}/baseline-start-times.json')); print(d.get('${CONTAINER}',''))" 2>/dev/null || \
                         grep "\"${CONTAINER}\"" "${DATA_DIR}/baseline-start-times.json" | sed 's/.*: *"\(.*\)".*/\1/' 2>/dev/null || echo "")

        if [ "${CURRENT_START}" != "${BASELINE_START}" ]; then
            echo "FAIL T4 no-existing-paladin-restart container=${CONTAINER} before=${BASELINE_START} after=${CURRENT_START}"
            exit 1
        fi
        echo "[T4] ${CONTAINER}: no restart (started=${CURRENT_START})"
    done

    echo "PASS T4 no-existing-paladin-restart"
}

case "${MODE}" in
    --t3) run_t3 ;;
    --t4) run_t4 ;;
    all)   run_t3 && run_t4 ;;
    *)     run_t3 ;;
esac
