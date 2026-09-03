#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/env-defaults.sh"

echo "[rolling-restart] Starting rolling restart of Paladin CB and Bank-A..."

restart_node() {
    local node="$1"
    local service_name="$2"
    local compose_file="$3"
    local rpc_port="$4"
    local start_time
    local end_time
    local duration

    echo "[rolling-restart] restarting ${node}..."
    start_time=$(date +%s)

    cd "${SPIKE_ROOT}"
    docker compose -f "${compose_file}" restart "${service_name}"

    echo "[rolling-restart] waiting for ${node} RPC on port ${rpc_port}..."
    "${SCRIPT_DIR}/wait-for-rpc.sh" "http://localhost:${rpc_port}" 60

    end_time=$(date +%s)
    duration=$((end_time - start_time))

    echo "[rolling-restart] RESTART ${node} start=${start_time} end=${end_time} duration=${duration}s healthy=true"
}

# Step 1: Restart CB Paladin
restart_node "cb" "paladin-cb" "compose/stack-found.yml" "${HOST_PALADIN_CB_RPC}"

# Step 2: Restart Bank-A Paladin
restart_node "bank-a" "paladin-bank-a" "compose/stack-found.yml" "${HOST_PALADIN_BA_RPC}"

echo "[rolling-restart] rolling restart complete"
echo "[rolling-restart] total duration: $((RESTART_CB_DURATION + RESTART_BA_DURATION))s"
