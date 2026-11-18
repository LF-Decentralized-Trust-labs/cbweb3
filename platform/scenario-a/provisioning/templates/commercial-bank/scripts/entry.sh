#!/bin/sh
# entry.sh — Besu entrypoint for the central-bank Compose template.
#
# When BOOTNODE_ENODE is set, Besu 25.8.0 requires an IP address in --bootnodes,
# not a hostname (ADR-001 rejected alternatives). This script resolves the hostname
# via Docker DNS (getent hosts) and reconstructs the enode with the resolved IP
# before exec'ing Besu.
#
# When BOOTNODE_ENODE is empty (mode: found — this node IS the bootnode),
# the script starts Besu without --bootnodes.
set -eu

BESU_ADVERTISED_HOST="${BESU_ADVERTISED_HOST:-}"
BESU_NAT_PROFILE="${BESU_NAT_PROFILE:-DOCKER}"
BESU_P2P_PORT="${BESU_P2P_PORT:-30303}"
BESU_RPC_PORT_INTERNAL="${BESU_RPC_PORT_INTERNAL:-8545}"
BESU_WS_PORT_INTERNAL="${BESU_WS_PORT_INTERNAL:-8546}"
BESU_LOGGING="${BESU_LOGGING:-INFO}"
BOOTNODE_ENODE="${BOOTNODE_ENODE:-}"
# Ceiling on concurrent JSON-RPC WebSocket connections. Besu's own default is 80,
# and an implicit default of 80 connections was exhausted in production: once every
# slot was taken, Besu refused new WebSocket clients, including the local Paladin
# node. The value is now explicit and tunable; 256 leaves room above the handful of
# legitimate clients (Paladin, relay, NOC, services) so a leak is caught by the
# besu_rpc_active_ws_connection_count gauge well before it becomes an outage. An
# empty value (compose forwards ${VAR:-}) falls back to the default.
BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS="${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS:-256}"

# Reject anything but an integer in [1, 2147483647] here, by name. Besu parses the
# option as a Java int: a typo or a larger value makes it fail option parsing inside
# a restart loop, and 0 starts a node that refuses every WebSocket client, which is
# the outage this setting exists to prevent.
case "${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}" in
    [1-9]*[!0-9]* | [!1-9]*)
        echo "[entry] ERROR: BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS must be a positive integer, got '${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}'"
        exit 1
        ;;
esac
# The length check keeps the numeric comparison within the shell's integer range.
if [ "${#BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}" -gt 10 ] || [ "${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}" -gt 2147483647 ]; then
    echo "[entry] ERROR: BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS must be at most 2147483647, got '${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}'"
    exit 1
fi

# Build NAT flags based on profile
# ADR-001 three-profile matrix:
#   DOCKER  → --nat-method=DOCKER (local cross-compose; auto-discovers host IP via Docker NAT table)
#   NONE    → --nat-method=NONE --p2p-host=<BESU_ADVERTISED_HOST> (intra-compose or prod)
case "${BESU_NAT_PROFILE}" in
    DOCKER)
        # ADR-001 D1: DOCKER NAT profile auto-discovers the host IP via Docker NAT table.
        # --p2p-host is intentionally omitted here (FR-006 deviation): Besu 25.8.0 returns
        # 127.0.0.1 for net_enode regardless of --nat-method or --p2p-host, so the declared
        # enode for the join bundle is extracted via admin_nodeInfo (not net_enode). The
        # BESU_ADVERTISED_HOST value is passed to TK-5 for join bundle construction; it does
        # not influence the wire-protocol enode in DOCKER mode.
        NAT_FLAGS="--nat-method=DOCKER"
        ;;
    NONE)
        if [ -z "${BESU_ADVERTISED_HOST}" ]; then
            echo "[entry] ERROR: BESU_ADVERTISED_HOST must be set when BESU_NAT_PROFILE=NONE"
            exit 1
        fi
        NAT_FLAGS="--nat-method=NONE --p2p-host=${BESU_ADVERTISED_HOST}"
        ;;
    *)
        echo "[entry] ERROR: unknown BESU_NAT_PROFILE=${BESU_NAT_PROFILE} (expected DOCKER or NONE)"
        exit 1
        ;;
esac

# Build --bootnodes flag
# Besu 25.8.0 rejects non-IP values in --bootnodes (ADR-001).
# If BOOTNODE_ENODE contains a DNS hostname, resolve it to an IP via getent hosts.
BOOTNODE_FLAG=""
if [ -n "${BOOTNODE_ENODE}" ]; then
    # Extract host from enode URI: enode://PUBKEY@HOST:PORT
    BOOT_HOST=$(echo "${BOOTNODE_ENODE}" | sed 's|enode://[^@]*@\([^:]*\):.*|\1|')
    BOOT_PORT=$(echo "${BOOTNODE_ENODE}" | sed 's|.*:\([0-9]*\)$|\1|')
    BOOT_PUBKEY=$(echo "${BOOTNODE_ENODE}" | sed 's|enode://\([^@]*\)@.*|\1|')

    # Check if already an IP address
    if echo "${BOOT_HOST}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
        BOOTNODE_FLAG="--bootnodes=${BOOTNODE_ENODE}"
    else
        echo "[entry] BOOTNODE_ENODE host is a DNS name (${BOOT_HOST}), resolving via getent hosts..."
        BOOT_IP=""
        _attempt=1
        while [ "${_attempt}" -le 30 ]; do
            BOOT_IP=$(getent hosts "${BOOT_HOST}" 2>/dev/null | awk '{print $1}' | head -1)
            if [ -n "${BOOT_IP}" ]; then
                echo "[entry] Resolved ${BOOT_HOST} -> ${BOOT_IP}"
                break
            fi
            echo "[entry] attempt ${_attempt}/30: ${BOOT_HOST} not yet resolvable, retrying..."
            sleep 2
            _attempt=$((_attempt + 1))
        done
        if [ -z "${BOOT_IP}" ]; then
            echo "[entry] FATAL: could not resolve ${BOOT_HOST} after 30 attempts"
            exit 1
        fi
        BOOTNODE_FLAG="--bootnodes=enode://${BOOT_PUBKEY}@${BOOT_IP}:${BOOT_PORT}"
    fi
    echo "[entry] Using bootnode: ${BOOTNODE_FLAG#--bootnodes=}"
fi

echo "[entry] NAT profile: ${BESU_NAT_PROFILE} | flags: ${NAT_FLAGS}"
echo "[entry] WebSocket max active connections: ${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}"

# --revert-reason-enabled returns the revert data of a failed call instead of an
# empty body. Paladin cannot decode what Besu does not send: without the flag a
# reverting private transition surfaces only as "PD012214: Unable to decode revert
# data (no revert data available)", and the real custom error — PenteInvalidEndorser
# in the FX incident of 2026-09-03 — has to be recovered by replaying the
# transaction through eth_call by hand. Besu holds the revert data for the duration
# of the call, which is why it ships off; on these node sizes that cost is worth
# paying to keep a private revert diagnosable.
#
# NAT_FLAGS and BOOTNODE_FLAG are intentionally unquoted: word splitting is the
# mechanism that passes multi-word flags (e.g. "--nat-method=NONE --p2p-host=X")
# as separate Besu arguments. When BOOTNODE_FLAG is empty, the unquoted expansion
# produces zero arguments (not an empty-string argument), which is correct.
exec /opt/besu/bin/besu \
    --data-path=/opt/besu/data \
    --genesis-file=/opt/besu/genesis/genesis.json \
    --min-gas-price=0 \
    --revert-reason-enabled=true \
    --rpc-http-enabled=true \
    --rpc-http-api=ETH,NET,QBFT,ADMIN \
    --rpc-http-host=0.0.0.0 \
    --rpc-http-port="${BESU_RPC_PORT_INTERNAL}" \
    --rpc-ws-enabled=true \
    --rpc-ws-api=ETH,NET,QBFT,ADMIN \
    --rpc-ws-host=0.0.0.0 \
    --rpc-ws-port="${BESU_WS_PORT_INTERNAL}" \
    --rpc-ws-max-active-connections="${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS}" \
    --p2p-port="${BESU_P2P_PORT}" \
    --host-allowlist="${BESU_HOST_ALLOWLIST:-*}" \
    --rpc-http-cors-origins="${BESU_CORS_ORIGINS:-all}" \
    --logging="${BESU_LOGGING}" \
    ${NAT_FLAGS} \
    ${BOOTNODE_FLAG}
