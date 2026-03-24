#!/usr/bin/env bash
# sync-contracts.sh — Extracts deployed contract addresses from Foundry broadcast
# output and updates .env.infra.* files used by backend microservices.
#
# Hub  (chain 1337): IdentityRegistry, tCeBM_BRL, tCeBM_EUR, HTLC, AMM, FXAgreement, ManualOracle
# Spoke-A (chain 1338): IdentityRegistry, tCeBM (domestic token), HTLC, SpokeBridge
# Spoke-B (chain 1339): IdentityRegistry, tCeBM (domestic token), HTLC, SpokeBridge
#
# Requires: jq

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# GNU sed (Linux) uses `sed -i`; BSD sed (macOS) requires `sed -i ''`
if [[ "$(uname)" == "Darwin" ]]; then
  SED_INPLACE=(sed -i '')
else
  SED_INPLACE=(sed -i)
fi

HUB_CHAIN_ID="${HUB_CHAIN_ID:-1337}"
SPOKE_A_CHAIN_ID="${SPOKE_A_CHAIN_ID:-1338}"
SPOKE_B_CHAIN_ID="${SPOKE_B_CHAIN_ID:-1339}"

HUB_BROADCAST="${ROOT_DIR}/contracts/broadcast/CBWeb3Hub.s.sol/${HUB_CHAIN_ID}/run-latest.json"
SPOKE_A_BROADCAST="${ROOT_DIR}/contracts/broadcast/CBWeb3Spoke.s.sol/${SPOKE_A_CHAIN_ID}/run-latest.json"
SPOKE_B_BROADCAST="${ROOT_DIR}/contracts/broadcast/CBWeb3Spoke.s.sol/${SPOKE_B_CHAIN_ID}/run-latest.json"

ENV_HUB="${ROOT_DIR}/backend/config/.env.infra.hub"
ENV_SPOKE_A="${ROOT_DIR}/backend/config/.env.infra.spoke-a"
ENV_SPOKE_B="${ROOT_DIR}/backend/config/.env.infra.spoke-b"

# --- Preflight checks ---
if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not installed." >&2
  exit 1
fi

# --- Extract address helper ---
extract_address() {
  local file="$1"
  local contract_name="$2"
  local occurrence="${3:-1}"
  jq -r --arg name "$contract_name" --argjson idx "$occurrence" \
    '[.transactions[] | select(.contractName == $name)] | .[$idx - 1].contractAddress // empty' \
    "${file}"
}

# --- Helper: upsert a KEY=VALUE in a .env file ---
upsert_env() {
  local file="$1" key="$2" value="$3"
  if grep -qE "^#?${key}=" "${file}" 2>/dev/null; then
    "${SED_INPLACE[@]}" "s|^#*${key}=.*|${key}=${value}|" "${file}"
  else
    echo "${key}=${value}" >> "${file}"
  fi
}

UPDATED=0

# ============================================================
# HUB (chain 1337)
# ============================================================
if [[ -f "${HUB_BROADCAST}" ]]; then
  echo "--- Hub (chain ${HUB_CHAIN_ID}) ---"

  HUB_IDENTITY_REGISTRY=$(extract_address "${HUB_BROADCAST}" "IdentityRegistry")
  HUB_TOKEN_BRL=$(extract_address "${HUB_BROADCAST}" "TokenizedCentralBankMoney" 1)
  HUB_TOKEN_EUR=$(extract_address "${HUB_BROADCAST}" "TokenizedCentralBankMoney" 2)
  HUB_HTLC=$(extract_address "${HUB_BROADCAST}" "HashTimeLockedContract")
  HUB_AMM=$(extract_address "${HUB_BROADCAST}" "AutomatedMarketMaker")
  HUB_FX_AGREEMENT=$(extract_address "${HUB_BROADCAST}" "FXAgreement")
  HUB_ORACLE=$(extract_address "${HUB_BROADCAST}" "ManualOracle")

  MISSING=()
  [[ -z "${HUB_IDENTITY_REGISTRY}" ]] && MISSING+=("IdentityRegistry")
  [[ -z "${HUB_TOKEN_BRL}" ]]         && MISSING+=("tCeBM_BRL")
  [[ -z "${HUB_TOKEN_EUR}" ]]         && MISSING+=("tCeBM_EUR")
  [[ -z "${HUB_HTLC}" ]]              && MISSING+=("HTLC")
  [[ -z "${HUB_AMM}" ]]               && MISSING+=("AMM")
  [[ -z "${HUB_FX_AGREEMENT}" ]]      && MISSING+=("FXAgreement")
  [[ -z "${HUB_ORACLE}" ]]            && MISSING+=("ManualOracle")

  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "ERROR: Hub — missing addresses: ${MISSING[*]}" >&2
    exit 1
  fi

  if [[ -f "${ENV_HUB}" ]]; then
    upsert_env "${ENV_HUB}" "PARTICIPANT_REGISTRY_ADDRESS" "${HUB_IDENTITY_REGISTRY}"
    upsert_env "${ENV_HUB}" "TOKEN_BRL_ADDRESS"            "${HUB_TOKEN_BRL}"
    upsert_env "${ENV_HUB}" "TOKEN_EUR_ADDRESS"            "${HUB_TOKEN_EUR}"
    upsert_env "${ENV_HUB}" "HTLC_ADDRESS"                 "${HUB_HTLC}"
    upsert_env "${ENV_HUB}" "AMM_ADDRESS"                  "${HUB_AMM}"
    upsert_env "${ENV_HUB}" "FX_AGREEMENT_ADDRESS"         "${HUB_FX_AGREEMENT}"
    upsert_env "${ENV_HUB}" "ORACLE_ADDRESS"               "${HUB_ORACLE}"
    UPDATED=$((UPDATED + 1))
    echo "  Updated: ${ENV_HUB}"
  else
    echo "  WARN: ${ENV_HUB} not found, skipping." >&2
  fi

  echo "  IdentityRegistry : ${HUB_IDENTITY_REGISTRY}"
  echo "  tCeBM_BRL        : ${HUB_TOKEN_BRL}"
  echo "  tCeBM_EUR        : ${HUB_TOKEN_EUR}"
  echo "  HTLC             : ${HUB_HTLC}"
  echo "  AMM              : ${HUB_AMM}"
  echo "  FX Agreement     : ${HUB_FX_AGREEMENT}"
  echo "  Manual Oracle    : ${HUB_ORACLE}"
else
  echo "WARN: Hub broadcast not found: ${HUB_BROADCAST} — skipping hub sync." >&2
fi

# ============================================================
# SPOKE-A (chain 1338)
# ============================================================
if [[ -f "${SPOKE_A_BROADCAST}" ]]; then
  echo "--- Spoke-A (chain ${SPOKE_A_CHAIN_ID}) ---"

  SA_IDENTITY_REGISTRY=$(extract_address "${SPOKE_A_BROADCAST}" "IdentityRegistry")
  SA_TOKEN=$(extract_address "${SPOKE_A_BROADCAST}" "TokenizedCentralBankMoney" 1)
  SA_HTLC=$(extract_address "${SPOKE_A_BROADCAST}" "HashTimeLockedContract")
  SA_SPOKE_BRIDGE=$(extract_address "${SPOKE_A_BROADCAST}" "SpokeBridge")

  MISSING=()
  [[ -z "${SA_IDENTITY_REGISTRY}" ]] && MISSING+=("IdentityRegistry")
  [[ -z "${SA_TOKEN}" ]]             && MISSING+=("Token")
  [[ -z "${SA_HTLC}" ]]              && MISSING+=("HTLC")
  [[ -z "${SA_SPOKE_BRIDGE}" ]]      && MISSING+=("SpokeBridge")

  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "ERROR: Spoke-A — missing addresses: ${MISSING[*]}" >&2
    exit 1
  fi

  if [[ -f "${ENV_SPOKE_A}" ]]; then
    upsert_env "${ENV_SPOKE_A}" "PARTICIPANT_REGISTRY_ADDRESS" "${SA_IDENTITY_REGISTRY}"
    upsert_env "${ENV_SPOKE_A}" "TOKEN_ADDRESS"                "${SA_TOKEN}"
    upsert_env "${ENV_SPOKE_A}" "HTLC_ADDRESS"                 "${SA_HTLC}"
    upsert_env "${ENV_SPOKE_A}" "SPOKE_BRIDGE_ADDRESS"         "${SA_SPOKE_BRIDGE}"
    UPDATED=$((UPDATED + 1))
    echo "  Updated: ${ENV_SPOKE_A}"
  else
    echo "  WARN: ${ENV_SPOKE_A} not found, skipping." >&2
  fi

  echo "  IdentityRegistry : ${SA_IDENTITY_REGISTRY}"
  echo "  Token            : ${SA_TOKEN}"
  echo "  HTLC             : ${SA_HTLC}"
  echo "  Spoke Bridge     : ${SA_SPOKE_BRIDGE}"
else
  echo "WARN: Spoke-A broadcast not found: ${SPOKE_A_BROADCAST} — skipping spoke-a sync." >&2
fi

# ============================================================
# SPOKE-B (chain 1339)
# ============================================================
if [[ -f "${SPOKE_B_BROADCAST}" ]]; then
  echo "--- Spoke-B (chain ${SPOKE_B_CHAIN_ID}) ---"

  SB_IDENTITY_REGISTRY=$(extract_address "${SPOKE_B_BROADCAST}" "IdentityRegistry")
  SB_TOKEN=$(extract_address "${SPOKE_B_BROADCAST}" "TokenizedCentralBankMoney" 1)
  SB_HTLC=$(extract_address "${SPOKE_B_BROADCAST}" "HashTimeLockedContract")
  SB_SPOKE_BRIDGE=$(extract_address "${SPOKE_B_BROADCAST}" "SpokeBridge")

  MISSING=()
  [[ -z "${SB_IDENTITY_REGISTRY}" ]] && MISSING+=("IdentityRegistry")
  [[ -z "${SB_TOKEN}" ]]             && MISSING+=("Token")
  [[ -z "${SB_HTLC}" ]]              && MISSING+=("HTLC")
  [[ -z "${SB_SPOKE_BRIDGE}" ]]      && MISSING+=("SpokeBridge")

  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "ERROR: Spoke-B — missing addresses: ${MISSING[*]}" >&2
    exit 1
  fi

  if [[ -f "${ENV_SPOKE_B}" ]]; then
    upsert_env "${ENV_SPOKE_B}" "PARTICIPANT_REGISTRY_ADDRESS" "${SB_IDENTITY_REGISTRY}"
    upsert_env "${ENV_SPOKE_B}" "TOKEN_ADDRESS"                "${SB_TOKEN}"
    upsert_env "${ENV_SPOKE_B}" "HTLC_ADDRESS"                 "${SB_HTLC}"
    upsert_env "${ENV_SPOKE_B}" "SPOKE_BRIDGE_ADDRESS"         "${SB_SPOKE_BRIDGE}"
    UPDATED=$((UPDATED + 1))
    echo "  Updated: ${ENV_SPOKE_B}"
  else
    echo "  WARN: ${ENV_SPOKE_B} not found, skipping." >&2
  fi

  echo "  IdentityRegistry : ${SB_IDENTITY_REGISTRY}"
  echo "  Token            : ${SB_TOKEN}"
  echo "  HTLC             : ${SB_HTLC}"
  echo "  Spoke Bridge     : ${SB_SPOKE_BRIDGE}"
else
  echo "WARN: Spoke-B broadcast not found: ${SPOKE_B_BROADCAST} — skipping spoke-b sync." >&2
fi

# --- Summary ---
echo ""
echo "=========================================="
echo "  CONTRACT ADDRESSES SYNCED"
echo "=========================================="
echo "  Files updated: ${UPDATED}"
echo "=========================================="
