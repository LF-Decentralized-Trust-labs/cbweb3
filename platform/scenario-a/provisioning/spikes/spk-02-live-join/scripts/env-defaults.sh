#!/usr/bin/env bash
# env-defaults.sh — Default ports and identifiers for the SP02 spike.
# Source this file before using port variables.
# All ports in 9xxx range to avoid collisions with SP01 (8xxx).

# === Besu P2P ports ===
export HOST_P2P_BOOT="${HOST_P2P_BOOT:-32303}"
export HOST_P2P_V1="${HOST_P2P_V1:-32304}"
export HOST_P2P_V2="${HOST_P2P_V2:-32305}"
export HOST_P2P_JOIN="${HOST_P2P_JOIN:-32306}"

# === Besu RPC/WS ports ===
export HOST_RPC_BOOT="${HOST_RPC_BOOT:-9645}"
export HOST_WS_BOOT="${HOST_WS_BOOT:-9655}"
export HOST_RPC_V1="${HOST_RPC_V1:-9643}"
export HOST_RPC_V2="${HOST_RPC_V2:-9644}"
export HOST_RPC_JOIN="${HOST_RPC_JOIN:-9646}"
export HOST_WS_JOIN="${HOST_WS_JOIN:-9656}"

# === Paladin RPC ports ===
export HOST_PALADIN_CB_RPC="${HOST_PALADIN_CB_RPC:-9648}"
export HOST_PALADIN_BA_RPC="${HOST_PALADIN_BA_RPC:-9649}"
export HOST_PALADIN_BX_RPC="${HOST_PALADIN_BX_RPC:-9650}"

# === Paladin gRPC ports ===
export HOST_PALADIN_CB_GRPC="${HOST_PALADIN_CB_GRPC:-9700}"
export HOST_PALADIN_BA_GRPC="${HOST_PALADIN_BA_GRPC:-9701}"
export HOST_PALADIN_BX_GRPC="${HOST_PALADIN_BX_GRPC:-9702}"

# === Container names ===
export CONTAINER_BESU_BOOT="${CONTAINER_BESU_BOOT:-spk02-besu-boot}"
export CONTAINER_BESU_V1="${CONTAINER_BESU_V1:-spk02-besu-v1}"
export CONTAINER_BESU_V2="${CONTAINER_BESU_V2:-spk02-besu-v2}"
export CONTAINER_BESU_JOIN="${CONTAINER_BESU_JOIN:-spk02-besu-joiner}"
export CONTAINER_PALADIN_CB="${CONTAINER_PALADIN_CB:-spk02-paladin-cb}"
export CONTAINER_PALADIN_BA="${CONTAINER_PALADIN_BA:-spk02-paladin-bank-a}"
export CONTAINER_PALADIN_BX="${CONTAINER_PALADIN_BX:-spk02-paladin-bank-x}"

export SPOKE_ID="${SPOKE_ID:-spoke-spk02}"

# === QBFT parameters (must match compose/genesis-config.json) ===
export QBFT_EPOCH_LENGTH="${QBFT_EPOCH_LENGTH:-30}"

# === Paladin node names (on-chain identity, derived from SPOKE_ID) ===
export PALADIN_CB_NODE_NAME="${PALADIN_CB_NODE_NAME:-${SPOKE_ID}-cb}"
export PALADIN_BA_NODE_NAME="${PALADIN_BA_NODE_NAME:-${SPOKE_ID}-bank-a}"
export PALADIN_BX_NODE_NAME="${PALADIN_BX_NODE_NAME:-${SPOKE_ID}-bank-x}"

# === Operator funded account (genesis alloc — research context only) ===
export OPERATOR_PRIVATE_KEY="${OPERATOR_PRIVATE_KEY:-0xc87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3}"

# === Toolchain PATH ===
# Foundry's `cast` (used by register-paladin-nodes.sh) is typically installed
# user-local in ~/.foundry/bin, which the Make-inherited PATH may not include.
export PATH="${HOME}/.foundry/bin:${PATH}"
