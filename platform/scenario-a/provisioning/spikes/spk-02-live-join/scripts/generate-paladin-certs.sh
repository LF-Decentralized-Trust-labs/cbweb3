#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CERTS_DIR="${SPIKE_ROOT}/data/paladin"

NODES=("cb" "bank-a" "bank-x")

generate_cert() {
    local node="$1"
    local cert_dir="${CERTS_DIR}/${node}"
    local cert_file="${cert_dir}/tls.crt"
    local key_file="${cert_dir}/tls.key"

    if [ -f "${cert_file}" ] && [ -f "${key_file}" ]; then
        echo "[generate-paladin-certs] SKIP: cert already exists for ${node}"
        return 0
    fi

    mkdir -p "${cert_dir}"

    echo "[generate-paladin-certs] generating self-signed P-256 cert for ${node}..."
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
        -keyout "${key_file}" \
        -out "${cert_file}" \
        -days 3650 -nodes \
        -subj "/CN=spk02-paladin-${node}" \
        -addext "subjectAltName=DNS:spk02-paladin-${node},DNS:localhost" \
        2>/dev/null

    echo "[generate-paladin-certs] CREATED: cert for ${node}"
}

for node in "${NODES[@]}"; do
    generate_cert "${node}"
done

echo "[generate-paladin-certs] all certs generated"
