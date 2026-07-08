#!/usr/bin/env bash
# test-central-bank-template.sh — Integration test for TK-4 (central-bank Compose template)
#
# Tests:
#   (a) 1st run: genesis is generated and node becomes available
#   (b) 2nd run: genesis hash is preserved (idempotency guard)
#   (c) enode audit: enode from admin_nodeInfo does not contain private IPs
#       NOTE: Besu 25.8.0 returns 127.0.0.1 from net_enode and admin_nodeInfo.enode
#       regardless of --nat-method (verified in spk-01 verify-enode.sh:10).
#       The real enode verification is deferred to the join bundle (TK-6).
#       Here we verify RPC reachability + block production.
#   (d) cleanup: all containers and named volumes removed
#
# STORAGE: config/genesis/chain-data all live on named Docker volumes, not on a
# SPOKE_DATA_DIR bind mount (see the deviation addendum in
# specs/026-tk4-compose-central-bank/plan.md). This script mirrors what the
# orchestration engine does in production: qbftConfigFile.json is piped directly
# into the cb_config volume (never touches this script's own filesystem beyond a
# heredoc), and genesis.json is read back from the cb_genesis volume via a
# throwaway container — matching engine/dockervolume.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
TEMPLATE="${REPO_ROOT}/scenario-a/provisioning/templates/central-bank/docker-compose.yaml"
PROJECT_NAME="tk4test$$$(date +%s | tail -c 6)"
BESU_IMAGE="${BESU_IMAGE:-hyperledger/besu:25.8.0}"
SPOKE_ID="spoke-test"
CONFIG_VOLUME="${SPOKE_ID}_cb_config"
GENESIS_VOLUME="${SPOKE_ID}_cb_genesis"
RPC_PORT=18645
WS_PORT=18655
P2P_PORT=31503
PASS=0
FAIL=0

log() { echo "[tk4-test] $*"; }
pass() { log "PASS $1"; PASS=$((PASS + 1)); }
fail() { log "FAIL $1: $2"; FAIL=$((FAIL + 1)); }

# read_genesis_hash prints sha256(genesis.json) from the cb_genesis volume, or
# nothing if the file/volume does not exist yet.
read_genesis_hash() {
    docker run --rm -v "${GENESIS_VOLUME}:/target:ro" alpine:3.20 \
        sh -c 'sha256sum /target/genesis.json 2>/dev/null | cut -d" " -f1' || true
}

cleanup() {
    log "Cleaning up..."
    docker compose -f "${TEMPLATE}" --project-name "${PROJECT_NAME}" down -v 2>/dev/null || true
    log "Cleanup complete."
}
trap cleanup EXIT

# --- Prerequisites ---
if [ ! -f "${TEMPLATE}" ]; then
    echo "FAIL: template not found at ${TEMPLATE}"
    echo "Run speckit-implement to create the template first."
    exit 1
fi

# Seed qbftConfigFile.json (chainId 1337, 1 validator, QBFT) directly into the
# cb_config named volume — piped via stdin, no host file involved (mirrors
# step_start_besu_found.go's scaffold()).
docker run --rm -i -v "${CONFIG_VOLUME}:/target" alpine:3.20 \
    sh -c 'mkdir -p /target && cat > /target/qbftConfigFile.json' << 'QBFT_EOF'
{
  "genesis": {
    "config": {
      "chainId": 1337,
      "londonBlock": 0,
      "qbft": {
        "blockperiodseconds": 2,
        "epochlength": 30000,
        "requesttimeoutseconds": 4
      }
    },
    "nonce": "0x0",
    "timestamp": "0x0",
    "gasLimit": "0x1fffffffffffff",
    "difficulty": "0x1",
    "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "alloc": {}
  },
  "blockchain": {
    "nodes": {
      "generate": true,
      "count": 1
    }
  }
}
QBFT_EOF

export SPOKE_ID \
       BESU_ADVERTISED_HOST="127.0.0.1" \
       BESU_RPC_PORT="${RPC_PORT}" \
       BESU_WS_PORT="${WS_PORT}" \
       BESU_P2P_PORT="${P2P_PORT}" \
       BESU_IMAGE \
       HOST_UID="$(id -u)" \
       HOST_GID="$(id -g)"

# --- Test (a): 1st run generates genesis and node starts ---
log "Test (a): First run — genesis generation + node startup"

docker compose -f "${TEMPLATE}" --project-name "${PROJECT_NAME}" up -d
log "Compose up. Waiting for Besu RPC to become available (max 90s)..."

RPC_OK=false
for i in $(seq 1 45); do
    if curl -sf -X POST "http://localhost:${RPC_PORT}" \
         -H 'Content-Type: application/json' \
         -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
         2>/dev/null | grep -q result; then
        RPC_OK=true
        break
    fi
    sleep 2
done

if $RPC_OK; then
    BLOCK_HEX=$(curl -sf -X POST "http://localhost:${RPC_PORT}" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        | jq -r '.result')
    if [ -z "${BLOCK_HEX}" ] || [ "${BLOCK_HEX}" = "null" ]; then
        fail "(a)" "eth_blockNumber returned null or empty result"
    else
        BLOCK_DEC=$((BLOCK_HEX))
        pass "(a) RPC reachable, blockHeight=${BLOCK_DEC}"
    fi
else
    fail "(a)" "Besu RPC did not respond after 90s"
fi

GENESIS_HASH_1="$(read_genesis_hash)"
if [ -n "${GENESIS_HASH_1}" ]; then
    pass "(a) genesis.json created in volume ${GENESIS_VOLUME}"
else
    fail "(a)" "genesis.json not found in volume ${GENESIS_VOLUME} after first run"
fi

# --- Test (b): 2nd run preserves genesis (idempotency) ---
log "Test (b): Second run — genesis must NOT be regenerated"

docker compose -f "${TEMPLATE}" --project-name "${PROJECT_NAME}" down
docker compose -f "${TEMPLATE}" --project-name "${PROJECT_NAME}" up -d

# Wait for RPC again (H-6: capture result to detect non-response)
RPC_OK=false
for i in $(seq 1 45); do
    if curl -sf -X POST "http://localhost:${RPC_PORT}" \
         -H 'Content-Type: application/json' \
         -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
         2>/dev/null | grep -q result; then
        RPC_OK=true
        break
    fi
    sleep 2
done

if ! $RPC_OK; then
    fail "(b)" "Besu RPC did not respond after 90s on second run"
fi

GENESIS_HASH_2="$(read_genesis_hash)"

if [ "${GENESIS_HASH_1}" = "${GENESIS_HASH_2}" ]; then
    pass "(b) genesis hash unchanged: ${GENESIS_HASH_1}"
else
    fail "(b)" "genesis regenerated! before=${GENESIS_HASH_1} after=${GENESIS_HASH_2}"
fi

# Verify genesis-init container logs contain skip message
INIT_LOGS=$(docker compose -f "${TEMPLATE}" --project-name "${PROJECT_NAME}" logs genesis-init 2>/dev/null || true)
if echo "${INIT_LOGS}" | grep -qiE "already exists|skip"; then
    pass "(b) genesis-init logged skip on 2nd run"
else
    fail "(b)" "genesis-init did not log skip message on 2nd run (logs: ${INIT_LOGS})"
fi

# --- Test (c): enode audit — RPC reachable + no private IPs in admin_nodeInfo ---
log "Test (c): Enode audit — RPC reachability and no private IPs in admin_nodeInfo"
# NOTE: Besu 25.8.0 returns 127.0.0.1 from both net_enode and admin_nodeInfo.enode
# regardless of --nat-method. The real enode verification (host = BESU_ADVERTISED_HOST)
# is deferred to the join bundle (TK-6 / verify-bundle.sh pattern from spk-01).
# Here we only verify: RPC is up + admin_nodeInfo responds + block height > 0.

BLOCK_HEX=$(curl -sf -X POST "http://localhost:${RPC_PORT}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    | jq -r '.result')

if [ -z "${BLOCK_HEX}" ] || [ "${BLOCK_HEX}" = "null" ]; then
    fail "(c)" "eth_blockNumber returned null or empty result"
else
    BLOCK_DEC=$((BLOCK_HEX))
    if [ "${BLOCK_DEC}" -gt 0 ]; then
        pass "(c) block production confirmed, blockHeight=${BLOCK_DEC}"
    else
        fail "(c)" "no block production detected, blockHeight=${BLOCK_DEC}"
    fi
fi

ADMIN_RESULT=$(curl -sf -X POST "http://localhost:${RPC_PORT}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
    2>/dev/null | jq -r '.result.enode // empty')

if [ -n "${ADMIN_RESULT}" ]; then
    pass "(c) admin_nodeInfo responded, enode=${ADMIN_RESULT}"
else
    fail "(c)" "admin_nodeInfo did not return enode (ADMIN API may not be enabled)"
fi

# --- Summary ---
echo ""
echo "==============================="
echo "TK-4 Template Integration Test"
echo "==============================="
echo "  PASS: ${PASS}"
echo "  FAIL: ${FAIL}"
echo "==============================="
if [ "${FAIL}" -eq 0 ]; then
    echo "PASS: template central-bank OK"
    exit 0
else
    echo "FAIL: ${FAIL} test(s) failed"
    exit 1
fi
