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

# NAT_FLAGS and BOOTNODE_FLAG are intentionally unquoted: word splitting is the
# mechanism that passes multi-word flags (e.g. "--nat-method=NONE --p2p-host=X")
# as separate Besu arguments. When BOOTNODE_FLAG is empty, the unquoted expansion
# produces zero arguments (not an empty-string argument), which is correct.
exec /opt/besu/bin/besu \
    --data-path=/opt/besu/data \
    --genesis-file=/opt/besu/genesis/genesis.json \
    --min-gas-price=0 \
    --rpc-http-enabled=true \
    --rpc-http-api=ETH,NET,QBFT,ADMIN \
    --rpc-http-host=0.0.0.0 \
    --rpc-http-port="${BESU_RPC_PORT_INTERNAL}" \
    --rpc-ws-enabled=true \
    --rpc-ws-api=ETH,NET,QBFT,ADMIN \
    --rpc-ws-host=0.0.0.0 \
    --rpc-ws-port="${BESU_WS_PORT_INTERNAL}" \
    --p2p-port="${BESU_P2P_PORT}" \
    --host-allowlist="${BESU_HOST_ALLOWLIST:-*}" \
    --rpc-http-cors-origins="${BESU_CORS_ORIGINS:-all}" \
    --logging="${BESU_LOGGING}" \
    ${NAT_FLAGS} \
    ${BOOTNODE_FLAG}
