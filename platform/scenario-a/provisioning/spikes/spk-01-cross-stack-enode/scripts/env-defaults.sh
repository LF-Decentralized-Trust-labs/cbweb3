#!/usr/bin/env bash
# env-defaults.sh — Default ports and identifiers for the SP01 spike.
# Source this file before using port variables.

export HOST_P2P_BOOT="${HOST_P2P_BOOT:-31303}"
export HOST_RPC_BOOT="${HOST_RPC_BOOT:-8645}"
export HOST_WS_BOOT="${HOST_WS_BOOT:-8655}"

export HOST_P2P_V1="${HOST_P2P_V1:-31304}"
export HOST_P2P_V2="${HOST_P2P_V2:-31305}"

export HOST_P2P_JOIN="${HOST_P2P_JOIN:-31306}"
export HOST_RPC_JOIN="${HOST_RPC_JOIN:-8646}"
export HOST_WS_JOIN="${HOST_WS_JOIN:-8656}"

export SPOKE_ID="${SPOKE_ID:-spoke-spk01}"
