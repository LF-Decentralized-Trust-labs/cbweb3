#!/usr/bin/env bash
# sync-contracts.sh — Extracts deployed contract addresses from Foundry broadcast
# output and updates .env.infra.* files used by backend microservices.
#
# Spoke-A (chain 1338): IdentityRegistry, tCeBM (domestic token), HTLC, SpokeBridge
#   All three entities (bank-a, bank-c, central-bank-a) share the same spoke-a
#   Besu network, so the same contract addresses are written to all three env files.
#
# Spoke-B (chain 1339): same contracts; written to bank-b, bank-d, central-bank-b.
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

SPOKE_A_CHAIN_ID="${SPOKE_A_CHAIN_ID:-1338}"

SPOKE_A_BROADCAST="${ROOT_DIR}/contracts/broadcast/CBWeb3Spoke.s.sol/${SPOKE_A_CHAIN_ID}/run-latest.json"

ENV_BANK_A="${ROOT_DIR}/backend/config/.env.infra.bank-a"
ENV_BANK_B="${ROOT_DIR}/backend/config/.env.infra.bank-b"
ENV_CENTRAL_BANK_A="${ROOT_DIR}/backend/config/.env.infra.central-bank-a"

ENV_BANK_C="${ROOT_DIR}/backend/config/.env.infra.bank-c"
ENV_BANK_D="${ROOT_DIR}/backend/config/.env.infra.bank-d"
ENV_CENTRAL_BANK_B="${ROOT_DIR}/backend/config/.env.infra.central-bank-b"

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
# SPOKE-A (chain 1338) — shared by bank-a, bank-c, central-bank-a
# ============================================================
if [[ -f "${SPOKE_A_BROADCAST}" ]]; then
  echo "--- Spoke-A (chain ${SPOKE_A_CHAIN_ID}) ---"

  SA_IDENTITY_REGISTRY=$(extract_address "${SPOKE_A_BROADCAST}" "IdentityRegistry")
  SA_TOKEN=$(extract_address "${SPOKE_A_BROADCAST}" "TokenizedCentralBankMoney" 1)
  SA_HTLC=$(extract_address "${SPOKE_A_BROADCAST}" "HashTimeLockedContract")
  SA_SPOKE_BRIDGE=$(extract_address "${SPOKE_A_BROADCAST}" "SpokeBridge")
  SA_FIAT=$(extract_address "${SPOKE_A_BROADCAST}" "FiatCentralBankMoney")

  MISSING=()
  [[ -z "${SA_IDENTITY_REGISTRY}" ]] && MISSING+=("IdentityRegistry")
  [[ -z "${SA_TOKEN}" ]]             && MISSING+=("Token")
  [[ -z "${SA_HTLC}" ]]              && MISSING+=("HTLC")
  [[ -z "${SA_SPOKE_BRIDGE}" ]]      && MISSING+=("SpokeBridge")

  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "ERROR: Spoke-A — missing addresses: ${MISSING[*]}" >&2
    exit 1
  fi

  [[ -z "${SA_FIAT}" ]] && echo "  WARN: FiatCentralBankMoney not found in spoke-a broadcast — skipping FIAT_TOKEN_ADDRESS." >&2

  echo "  IdentityRegistry : ${SA_IDENTITY_REGISTRY}"
  echo "  Token            : ${SA_TOKEN}"
  echo "  HTLC             : ${SA_HTLC}"
  echo "  Spoke Bridge     : ${SA_SPOKE_BRIDGE}"
  [[ -n "${SA_FIAT}" ]] && echo "  Fiat Token       : ${SA_FIAT}"

  # Write same addresses to all three entity env files
  for ENV_FILE in "${ENV_BANK_A}" "${ENV_BANK_C}" "${ENV_CENTRAL_BANK_A}"; do
    if [[ -f "${ENV_FILE}" ]]; then
      upsert_env "${ENV_FILE}" "PARTICIPANT_REGISTRY_ADDRESS" "${SA_IDENTITY_REGISTRY}"
      upsert_env "${ENV_FILE}" "TOKEN_ADDRESS"                "${SA_TOKEN}"
      upsert_env "${ENV_FILE}" "HTLC_ADDRESS"                 "${SA_HTLC}"
      upsert_env "${ENV_FILE}" "SPOKE_BRIDGE_ADDRESS"         "${SA_SPOKE_BRIDGE}"
      [[ -n "${SA_FIAT}" ]] && upsert_env "${ENV_FILE}" "FIAT_TOKEN_ADDRESS" "${SA_FIAT}"
      UPDATED=$((UPDATED + 1))
      echo "  Updated: ${ENV_FILE}"
    else
      echo "  WARN: ${ENV_FILE} not found, skipping." >&2
    fi
  done
else
  echo "WARN: Spoke-A broadcast not found: ${SPOKE_A_BROADCAST} — skipping spoke-a sync." >&2
fi

# ============================================================
# SPOKE-B (chain 1339) — shared by bank-b, bank-d, central-bank-b
# ============================================================
SPOKE_B_CHAIN_ID="${SPOKE_B_CHAIN_ID:-1339}"

SPOKE_B_BROADCAST="${ROOT_DIR}/contracts/broadcast/CBWeb3Spoke.s.sol/${SPOKE_B_CHAIN_ID}/run-latest.json"

if [[ -f "${SPOKE_B_BROADCAST}" ]]; then
  echo "--- Spoke-B (chain ${SPOKE_B_CHAIN_ID}) ---"

  SB_IDENTITY_REGISTRY=$(extract_address "${SPOKE_B_BROADCAST}" "IdentityRegistry")
  SB_TOKEN=$(extract_address "${SPOKE_B_BROADCAST}" "TokenizedCentralBankMoney" 1)
  SB_HTLC=$(extract_address "${SPOKE_B_BROADCAST}" "HashTimeLockedContract")
  SB_SPOKE_BRIDGE=$(extract_address "${SPOKE_B_BROADCAST}" "SpokeBridge")
  SB_FIAT=$(extract_address "${SPOKE_B_BROADCAST}" "FiatCentralBankMoney")

  MISSING_B=()
  [[ -z "${SB_IDENTITY_REGISTRY}" ]] && MISSING_B+=("IdentityRegistry")
  [[ -z "${SB_TOKEN}" ]]             && MISSING_B+=("Token")
  [[ -z "${SB_HTLC}" ]]              && MISSING_B+=("HTLC")
  [[ -z "${SB_SPOKE_BRIDGE}" ]]      && MISSING_B+=("SpokeBridge")

  if [[ ${#MISSING_B[@]} -gt 0 ]]; then
    echo "ERROR: Spoke-B — missing addresses: ${MISSING_B[*]}" >&2
    exit 1
  fi

  [[ -z "${SB_FIAT}" ]] && echo "  WARN: FiatCentralBankMoney not found in spoke-b broadcast — skipping FIAT_TOKEN_ADDRESS." >&2

  echo "  IdentityRegistry : ${SB_IDENTITY_REGISTRY}"
  echo "  Token            : ${SB_TOKEN}"
  echo "  HTLC             : ${SB_HTLC}"
  echo "  Spoke Bridge     : ${SB_SPOKE_BRIDGE}"
  [[ -n "${SB_FIAT}" ]] && echo "  Fiat Token       : ${SB_FIAT}"

  for ENV_FILE in "${ENV_BANK_B}" "${ENV_BANK_D}" "${ENV_CENTRAL_BANK_B}"; do
    if [[ -f "${ENV_FILE}" ]]; then
      upsert_env "${ENV_FILE}" "PARTICIPANT_REGISTRY_ADDRESS" "${SB_IDENTITY_REGISTRY}"
      upsert_env "${ENV_FILE}" "TOKEN_ADDRESS"                "${SB_TOKEN}"
      upsert_env "${ENV_FILE}" "HTLC_ADDRESS"                 "${SB_HTLC}"
      upsert_env "${ENV_FILE}" "SPOKE_BRIDGE_ADDRESS"         "${SB_SPOKE_BRIDGE}"
      [[ -n "${SB_FIAT}" ]] && upsert_env "${ENV_FILE}" "FIAT_TOKEN_ADDRESS" "${SB_FIAT}"
      UPDATED=$((UPDATED + 1))
      echo "  Updated: ${ENV_FILE}"
    else
      echo "  WARN: ${ENV_FILE} not found, skipping." >&2
    fi
  done
else
  echo "WARN: Spoke-B broadcast not found: ${SPOKE_B_BROADCAST} — skipping spoke-b sync." >&2
fi

# ============================================================
# CB_PRIVATE_KEY — Propagate governance admin key to central bank envs only
# ============================================================
CONTRACTS_ENV="${ROOT_DIR}/contracts/.env"
if [[ -f "${CONTRACTS_ENV}" ]]; then
  ADMIN_KEY=$(grep -E '^ADMIN_PRIVATE_KEY=' "${CONTRACTS_ENV}" | head -1 | cut -d= -f2-)
  ADMIN_KEY="${ADMIN_KEY#0x}"
  if [[ -n "${ADMIN_KEY}" ]]; then
    echo "--- CB_PRIVATE_KEY (governance admin) ---"
    for CB_ENV in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_B}"; do
      if [[ -f "${CB_ENV}" ]]; then
        upsert_env "${CB_ENV}" "CB_PRIVATE_KEY" "${ADMIN_KEY}"
        echo "  Updated CB_PRIVATE_KEY in: ${CB_ENV}"
      fi
    done
  else
    echo "WARN: ADMIN_PRIVATE_KEY is empty in ${CONTRACTS_ENV} — skipping CB_PRIVATE_KEY." >&2
  fi
else
  echo "WARN: ${CONTRACTS_ENV} not found — skipping CB_PRIVATE_KEY propagation." >&2
fi

# ============================================================
# CACTI HTLC relay — write SPOKE_*_HTLC_ADDRESS for the Cacti compose
# ============================================================
CACTI_ENV="${ROOT_DIR}/interop/hub-and-spoke/cacti/.env"
CACTI_UPDATED=0

if [[ -n "${SA_HTLC:-}" || -n "${SB_HTLC:-}" ]]; then
  echo "--- Cacti HTLC relay env ---"
  {
    echo "# Auto-generated by sync-contracts.sh — do not edit manually"
    [[ -n "${SA_HTLC:-}" ]] && echo "SPOKE_A_HTLC_ADDRESS=${SA_HTLC}"
    [[ -n "${SB_HTLC:-}" ]] && echo "SPOKE_B_HTLC_ADDRESS=${SB_HTLC}"
  } > "${CACTI_ENV}"
  echo "  Updated: ${CACTI_ENV}"
  CACTI_UPDATED=1
else
  echo "WARN: No HTLC addresses extracted — skipping Cacti env update." >&2
fi

# --- Summary ---
echo ""
echo "=========================================="
echo "  CONTRACT ADDRESSES SYNCED"
echo "=========================================="
echo "  Files updated: $((UPDATED + CACTI_UPDATED))"
[[ ${CACTI_UPDATED} -eq 1 ]] && echo "  Cacti env   : ${CACTI_ENV}"
echo "=========================================="
