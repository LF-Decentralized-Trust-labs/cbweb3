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

# Resolved in the LiquidityCommitRegistry section below; declared here so all
# downstream sections (Cacti env) can reference it unconditionally.
LCR_ADDRESS=""

ENV_BANK_A="${ROOT_DIR}/backend/config/.env.infra.bank-a"
ENV_BANK_B="${ROOT_DIR}/backend/config/.env.infra.bank-b"
ENV_CENTRAL_BANK_A="${ROOT_DIR}/backend/config/.env.infra.central-bank-a"

ENV_BANK_C="${ROOT_DIR}/backend/config/.env.infra.bank-c"
ENV_BANK_D="${ROOT_DIR}/backend/config/.env.infra.bank-d"
ENV_CENTRAL_BANK_B="${ROOT_DIR}/backend/config/.env.infra.central-bank-b"

# .example files mirror the real files and must be kept in sync so they
# serve as accurate templates after every redeploy.
ENV_BANK_A_EXAMPLE="${ROOT_DIR}/backend/config/.env.infra.bank-a.example"
ENV_BANK_B_EXAMPLE="${ROOT_DIR}/backend/config/.env.infra.bank-b.example"
ENV_BANK_C_EXAMPLE="${ROOT_DIR}/backend/config/.env.infra.bank-c.example"
ENV_BANK_D_EXAMPLE="${ROOT_DIR}/backend/config/.env.infra.bank-d.example"
ENV_CENTRAL_BANK_A_EXAMPLE="${ROOT_DIR}/backend/config/.env.infra.central-bank-a.example"
ENV_CENTRAL_BANK_B_EXAMPLE="${ROOT_DIR}/backend/config/.env.infra.central-bank-b.example"

# --- Preflight checks ---
if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not installed." >&2
  exit 1
fi

# ============================================================
# Bootstrap: create .env.infra.* from .example if missing.
# This allows a clean redeploy without requiring manual setup.
# ============================================================
echo "--- Bootstrap env files from .example (if missing) ---"
BOOTSTRAP_PAIRS=(
  "${ROOT_DIR}/backend/config/.env.infra.bank-a.example:${ENV_BANK_A}"
  "${ROOT_DIR}/backend/config/.env.infra.bank-b.example:${ENV_BANK_B}"
  "${ROOT_DIR}/backend/config/.env.infra.central-bank-a.example:${ENV_CENTRAL_BANK_A}"
  "${ROOT_DIR}/backend/config/.env.infra.central-bank-b.example:${ENV_CENTRAL_BANK_B}"
)
# Include optional entities (bank-c, bank-d) only when their example files exist
for OPTIONAL in \
  "${ROOT_DIR}/backend/config/.env.infra.bank-c.example:${ENV_BANK_C}" \
  "${ROOT_DIR}/backend/config/.env.infra.bank-d.example:${ENV_BANK_D}"; do
  SRC="${OPTIONAL%%:*}"
  [[ -f "${SRC}" ]] && BOOTSTRAP_PAIRS+=("${OPTIONAL}")
done

for PAIR in "${BOOTSTRAP_PAIRS[@]}"; do
  SRC="${PAIR%%:*}"
  DST="${PAIR##*:}"
  if [[ ! -f "${DST}" && -f "${SRC}" ]]; then
    cp "${SRC}" "${DST}"
    echo "  Created: ${DST} (from $(basename ${SRC}))"
  fi
done

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
  SA_FX_AGREEMENT=$(extract_address "${SPOKE_A_BROADCAST}" "FXAgreement")
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
  [[ -z "${SA_FX_AGREEMENT}" ]] && echo "  WARN: FXAgreement not found in spoke-a broadcast — skipping FX_AGREEMENT_ADDRESS." >&2

  echo "  IdentityRegistry : ${SA_IDENTITY_REGISTRY}"
  echo "  Token            : ${SA_TOKEN}"
  echo "  HTLC             : ${SA_HTLC}"
  [[ -n "${SA_FX_AGREEMENT}" ]] && echo "  FX Agreement     : ${SA_FX_AGREEMENT}"
  echo "  Spoke Bridge     : ${SA_SPOKE_BRIDGE}"
  [[ -n "${SA_FIAT}" ]] && echo "  Fiat Token       : ${SA_FIAT}"

  # Write same addresses to all three entity env files (and their .example mirrors)
  for ENV_FILE in "${ENV_BANK_A}" "${ENV_BANK_A_EXAMPLE}" \
                  "${ENV_BANK_C}" "${ENV_BANK_C_EXAMPLE}" \
                  "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}"; do
    if [[ -f "${ENV_FILE}" ]]; then
      upsert_env "${ENV_FILE}" "PARTICIPANT_REGISTRY_ADDRESS" "${SA_IDENTITY_REGISTRY}"
      upsert_env "${ENV_FILE}" "TOKEN_ADDRESS"                "${SA_TOKEN}"
      upsert_env "${ENV_FILE}" "HTLC_ADDRESS"                 "${SA_HTLC}"
      [[ -n "${SA_FX_AGREEMENT}" ]] && upsert_env "${ENV_FILE}" "FX_AGREEMENT_ADDRESS" "${SA_FX_AGREEMENT}"
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
  SB_FX_AGREEMENT=$(extract_address "${SPOKE_B_BROADCAST}" "FXAgreement")
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
  [[ -z "${SB_FX_AGREEMENT}" ]] && echo "  WARN: FXAgreement not found in spoke-b broadcast — skipping FX_AGREEMENT_ADDRESS." >&2

  echo "  IdentityRegistry : ${SB_IDENTITY_REGISTRY}"
  echo "  Token            : ${SB_TOKEN}"
  echo "  HTLC             : ${SB_HTLC}"
  [[ -n "${SB_FX_AGREEMENT}" ]] && echo "  FX Agreement     : ${SB_FX_AGREEMENT}"
  echo "  Spoke Bridge     : ${SB_SPOKE_BRIDGE}"
  [[ -n "${SB_FIAT}" ]] && echo "  Fiat Token       : ${SB_FIAT}"

  for ENV_FILE in "${ENV_BANK_B}" "${ENV_BANK_B_EXAMPLE}" \
                  "${ENV_BANK_D}" "${ENV_BANK_D_EXAMPLE}" \
                  "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
    if [[ -f "${ENV_FILE}" ]]; then
      upsert_env "${ENV_FILE}" "PARTICIPANT_REGISTRY_ADDRESS" "${SB_IDENTITY_REGISTRY}"
      upsert_env "${ENV_FILE}" "TOKEN_ADDRESS"                "${SB_TOKEN}"
      upsert_env "${ENV_FILE}" "HTLC_ADDRESS"                 "${SB_HTLC}"
      [[ -n "${SB_FX_AGREEMENT}" ]] && upsert_env "${ENV_FILE}" "FX_AGREEMENT_ADDRESS" "${SB_FX_AGREEMENT}"
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
# HUB (AMM) — Spoke-A plays Hub role in local dev (chain 1338)
# The AMM broadcast lives under AutomatedMarketMaker.s.sol/<chainId>.
# We also accept CBWeb3Hub.s.sol/<chainId> as a fallback.
# ============================================================
HUB_CHAIN_ID="${HUB_CHAIN_ID:-${SPOKE_A_CHAIN_ID}}"
HUB_AMM_BROADCAST="${ROOT_DIR}/contracts/broadcast/AutomatedMarketMaker.s.sol/${HUB_CHAIN_ID}/run-latest.json"
HUB_FALLBACK_BROADCAST="${ROOT_DIR}/contracts/broadcast/CBWeb3Hub.s.sol/${HUB_CHAIN_ID}/run-latest.json"

HUB_BROADCAST=""
if [[ -f "${HUB_AMM_BROADCAST}" ]]; then
  HUB_BROADCAST="${HUB_AMM_BROADCAST}"
elif [[ -f "${HUB_FALLBACK_BROADCAST}" ]]; then
  HUB_BROADCAST="${HUB_FALLBACK_BROADCAST}"
fi

if [[ -n "${HUB_BROADCAST}" ]]; then
  echo "--- Hub AMM (chain ${HUB_CHAIN_ID}) ---"
  HUB_AMM=$(extract_address "${HUB_BROADCAST}" "AutomatedMarketMaker")
  HUB_IDENTITY_REGISTRY=$(extract_address "${HUB_BROADCAST}" "IdentityRegistry")
  HUB_TOKEN_A=$(extract_address "${HUB_BROADCAST}" "TokenizedCentralBankMoney" 1)
  HUB_TOKEN_B=$(extract_address "${HUB_BROADCAST}" "TokenizedCentralBankMoney" 2)
  if [[ -n "${HUB_AMM}" ]]; then
    echo "  AMM              : ${HUB_AMM}"
    [[ -n "${HUB_IDENTITY_REGISTRY}" ]] && echo "  Hub IdentityReg  : ${HUB_IDENTITY_REGISTRY}"
    # Write to ALL entity env files (and .example mirrors) — every api-gateway needs the Hub AMM address.
    for ENV_FILE in "${ENV_BANK_A}" "${ENV_BANK_A_EXAMPLE}" \
                    "${ENV_BANK_B}" "${ENV_BANK_B_EXAMPLE}" \
                    "${ENV_BANK_C}" "${ENV_BANK_C_EXAMPLE}" \
                    "${ENV_BANK_D}" "${ENV_BANK_D_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "AMM_CONTRACT_ADDRESS" "${HUB_AMM}"
        upsert_env "${ENV_FILE}" "HUB_CHAIN_ID"         "${HUB_CHAIN_ID}"
        [[ -n "${HUB_IDENTITY_REGISTRY}" ]] && upsert_env "${ENV_FILE}" "HUB_IDENTITY_REGISTRY_ADDRESS" "${HUB_IDENTITY_REGISTRY}"
        echo "  Updated AMM in: ${ENV_FILE}"
      fi
    done
    UPDATED=$((UPDATED + 1))
  else
    echo "  WARN: AutomatedMarketMaker not found in Hub broadcast — skipping AMM sync." >&2
  fi
  if [[ -n "${HUB_TOKEN_A}" && -n "${HUB_TOKEN_B}" ]]; then
    echo "  HUB_TOKEN_A      : ${HUB_TOKEN_A}"
    echo "  HUB_TOKEN_B      : ${HUB_TOKEN_B}"
    for ENV_FILE in "${ENV_BANK_A}" "${ENV_BANK_A_EXAMPLE}" \
                    "${ENV_BANK_B}" "${ENV_BANK_B_EXAMPLE}" \
                    "${ENV_BANK_C}" "${ENV_BANK_C_EXAMPLE}" \
                    "${ENV_BANK_D}" "${ENV_BANK_D_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "HUB_TOKEN_A_ADDRESS" "${HUB_TOKEN_A}"
        upsert_env "${ENV_FILE}" "HUB_TOKEN_B_ADDRESS" "${HUB_TOKEN_B}"
        echo "  Updated Hub tokens in: ${ENV_FILE}"
      fi
    done
  else
    echo "  WARN: Hub tCeBM token addresses not found — skipping HUB_TOKEN_A/B sync." >&2
  fi

  # CurrencyRegistry (006-hub-currency-registry) — written only to CB env files.
  # Commercial banks do not call CurrencyRegistry endpoints directly.
  HUB_CURRENCY_REGISTRY=$(extract_address "${HUB_BROADCAST}" "CurrencyRegistry")
  if [[ -n "${HUB_CURRENCY_REGISTRY}" ]]; then
    echo "  CurrencyRegistry : ${HUB_CURRENCY_REGISTRY}"
    for ENV_FILE in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "CURRENCY_REGISTRY_CONTRACT_ADDRESS" "${HUB_CURRENCY_REGISTRY}"
        echo "  Updated CurrencyRegistry in: ${ENV_FILE}"
      fi
    done
    UPDATED=$((UPDATED + 1))
  else
    echo "  INFO: CurrencyRegistry not found in Hub broadcast — skipping (deploy CBWeb3Hub.s.sol to include it)." >&2
  fi

  # PairRegistry (005-cooperative-liquidity) — written to all env files.
  HUB_PAIR_REGISTRY=$(extract_address "${HUB_BROADCAST}" "PairRegistry")
  if [[ -n "${HUB_PAIR_REGISTRY}" ]]; then
    echo "  PairRegistry     : ${HUB_PAIR_REGISTRY}"
    for ENV_FILE in "${ENV_BANK_A}" "${ENV_BANK_A_EXAMPLE}" \
                    "${ENV_BANK_B}" "${ENV_BANK_B_EXAMPLE}" \
                    "${ENV_BANK_C}" "${ENV_BANK_C_EXAMPLE}" \
                    "${ENV_BANK_D}" "${ENV_BANK_D_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "PAIR_REGISTRY_CONTRACT_ADDRESS" "${HUB_PAIR_REGISTRY}"
        echo "  Updated PairRegistry in: ${ENV_FILE}"
      fi
    done
    UPDATED=$((UPDATED + 1))
  else
    echo "  INFO: PairRegistry not found in Hub broadcast — skipping (deploy CBWeb3Hub.s.sol to include it)." >&2
  fi
else
  echo "WARN: Hub AMM broadcast not found (tried chain ${HUB_CHAIN_ID}) — skipping AMM sync." >&2
fi

# ============================================================
# CB_PRIVATE_KEY — Propagate governance admin key to central bank envs only
# ============================================================
CONTRACTS_ENV="${ROOT_DIR}/contracts/.env"
if [[ -f "${CONTRACTS_ENV}" ]]; then
  ADMIN_KEY=$(grep -E '^ADMIN_PRIVATE_KEY=' "${CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- || true)
  ADMIN_KEY="${ADMIN_KEY#0x}"
  if [[ -n "${ADMIN_KEY}" ]]; then
    echo "--- CB_PRIVATE_KEY + SIGNER_PRIVATE_KEY (governance admin / tCeBM signer) ---"
    for CB_ENV in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                  "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${CB_ENV}" ]]; then
        upsert_env "${CB_ENV}" "CB_PRIVATE_KEY" "${ADMIN_KEY}"
        upsert_env "${CB_ENV}" "SIGNER_PRIVATE_KEY" "${ADMIN_KEY}"
        echo "  Updated CB_PRIVATE_KEY + SIGNER_PRIVATE_KEY in: ${CB_ENV}"
      fi
    done
  else
    echo "WARN: ADMIN_PRIVATE_KEY is empty in ${CONTRACTS_ENV} — skipping CB_PRIVATE_KEY." >&2
  fi
else
  echo "WARN: ${CONTRACTS_ENV} not found — skipping CB_PRIVATE_KEY propagation." >&2
fi

# ============================================================
# LiquidityCommitRegistry — 007-bridge-based-cb-liquidity
# Extract address from SeedNewSovereignPair broadcast if it exists;
# otherwise auto-deploy using forge create on the Hub chain.
# Writes LIQUIDITY_COMMIT_REGISTRY_ADDRESS to CB env files and Cacti relay env.
# ============================================================
SOVEREIGN_PAIR_BROADCAST="${ROOT_DIR}/contracts/broadcast/SeedNewSovereignPair.s.sol/${HUB_CHAIN_ID:-${SPOKE_A_CHAIN_ID}}/run-latest.json"

echo "--- LiquidityCommitRegistry (007) ---"
if [[ -f "${SOVEREIGN_PAIR_BROADCAST}" ]]; then
  LCR_ADDRESS=$(jq -r '[.transactions[] | select(.contractName == "LiquidityCommitRegistry")] | .[0].contractAddress // empty' "${SOVEREIGN_PAIR_BROADCAST}" 2>/dev/null || true)
  [[ -n "${LCR_ADDRESS}" ]] && echo "  Found in SeedNewSovereignPair broadcast: ${LCR_ADDRESS}"
fi

if [[ -z "${LCR_ADDRESS}" ]]; then
  echo "  Not found in broadcast — attempting auto-deploy..."
  _HUB_RPC="${SPOKE_A_RPC_URL:-http://127.0.0.1:8645}"
  _CONTRACTS_ENV="${ROOT_DIR}/contracts/.env"
  _ADMIN_KEY=$(grep -E '^ADMIN_PRIVATE_KEY=' "${_CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '[:space:]' || true)
  _HUB_IR="${HUB_IDENTITY_REGISTRY:-}"

  if command -v forge &>/dev/null && [[ -n "${_ADMIN_KEY}" && -n "${_HUB_IR}" ]]; then
    _LCR_JSON=$(cd "${ROOT_DIR}/contracts" && \
      forge create src/LiquidityCommitRegistry.sol:LiquidityCommitRegistry \
        --rpc-url "${_HUB_RPC}" \
        --private-key "${_ADMIN_KEY}" \
        --constructor-args "${_HUB_IR}" \
        --json 2>/dev/null || echo "{}")
    LCR_ADDRESS=$(echo "${_LCR_JSON}" | jq -r '.deployedTo // empty' 2>/dev/null || true)
    [[ -n "${LCR_ADDRESS}" ]] && echo "  Deployed LiquidityCommitRegistry: ${LCR_ADDRESS}" || \
      echo "  WARN: forge create returned no address — check RPC and key." >&2
  else
    ! command -v forge &>/dev/null  && echo "  WARN: forge not found — cannot auto-deploy LCR." >&2
    [[ -z "${_ADMIN_KEY}" ]]        && echo "  WARN: ADMIN_PRIVATE_KEY missing in contracts/.env." >&2
    [[ -z "${_HUB_IR}" ]]          && echo "  WARN: HUB_IDENTITY_REGISTRY unknown — run hub deploy first." >&2
  fi
fi

if [[ -n "${LCR_ADDRESS}" ]]; then
  for ENV_FILE in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                  "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
    if [[ -f "${ENV_FILE}" ]]; then
      upsert_env "${ENV_FILE}" "LIQUIDITY_COMMIT_REGISTRY_ADDRESS" "${LCR_ADDRESS}"
      echo "  Updated LIQUIDITY_COMMIT_REGISTRY_ADDRESS in: ${ENV_FILE}"
    fi
  done
  UPDATED=$((UPDATED + 1))
else
  echo "  WARN: LiquidityCommitRegistry address unresolved — skipping env sync." >&2
fi

# ============================================================
# Sovereign W-tCeBM tokens (007-bridge-based-cb-liquidity)
# Extract the two new W-tCeBM token addresses deployed by SeedNewSovereignPair
# and the sovereign AMM address. Written to CB env files so the api-gateway
# and tryout scripts can use the correct w_token_address in registerCommit.
#
#   SOVEREIGN_HUB_TOKEN_A_ADDRESS → central-bank-a env only (CB-A issuer)
#   SOVEREIGN_HUB_TOKEN_B_ADDRESS → central-bank-b env only (CB-B issuer)
#   SOVEREIGN_AMM_ADDRESS         → both CB env files
# ============================================================
if [[ -f "${SOVEREIGN_PAIR_BROADCAST}" ]]; then
  echo "--- Sovereign W-tCeBM tokens (007) ---"
  SOV_TOKEN_A=$(jq -r \
    '[.transactions[] | select(.transactionType=="CREATE" and .contractName=="TokenizedCentralBankMoney")] | .[0].contractAddress // empty' \
    "${SOVEREIGN_PAIR_BROADCAST}" 2>/dev/null || true)
  SOV_TOKEN_B=$(jq -r \
    '[.transactions[] | select(.transactionType=="CREATE" and .contractName=="TokenizedCentralBankMoney")] | .[1].contractAddress // empty' \
    "${SOVEREIGN_PAIR_BROADCAST}" 2>/dev/null || true)
  SOV_AMM=$(jq -r \
    '[.transactions[] | select(.transactionType=="CREATE" and .contractName=="AutomatedMarketMaker")] | .[0].contractAddress // empty' \
    "${SOVEREIGN_PAIR_BROADCAST}" 2>/dev/null || true)

  if [[ -n "${SOV_TOKEN_A}" ]]; then
    echo "  SOVEREIGN_HUB_TOKEN_A_ADDRESS : ${SOV_TOKEN_A} (CB-A issuer)"
    for ENV_FILE in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "SOVEREIGN_HUB_TOKEN_A_ADDRESS" "${SOV_TOKEN_A}"
        upsert_env "${ENV_FILE}" "W_TOKEN_ADDRESS" "${SOV_TOKEN_A}"
        echo "  Updated SOVEREIGN_HUB_TOKEN_A_ADDRESS + W_TOKEN_ADDRESS in: ${ENV_FILE}"
      fi
    done
    # Propagate W_TOKEN_ADDRESS to all spoke-a commercial banks (bank-a, bank-c) and their .example mirrors.
    for BANK_ENV in "${ENV_BANK_A}" "${ENV_BANK_A_EXAMPLE}" "${ENV_BANK_C}" "${ENV_BANK_C_EXAMPLE}"; do
      if [[ -f "${BANK_ENV}" ]]; then
        upsert_env "${BANK_ENV}" "W_TOKEN_ADDRESS" "${SOV_TOKEN_A}"
        echo "  Updated W_TOKEN_ADDRESS in: ${BANK_ENV}"
      fi
    done
  else
    echo "  WARN: Sovereign Token A not found in SeedNewSovereignPair broadcast — getCentralBankOf will fail." >&2
  fi

  if [[ -n "${SOV_TOKEN_B}" ]]; then
    echo "  SOVEREIGN_HUB_TOKEN_B_ADDRESS : ${SOV_TOKEN_B} (CB-B issuer)"
    for ENV_FILE in "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "SOVEREIGN_HUB_TOKEN_B_ADDRESS" "${SOV_TOKEN_B}"
        upsert_env "${ENV_FILE}" "W_TOKEN_ADDRESS" "${SOV_TOKEN_B}"
        echo "  Updated SOVEREIGN_HUB_TOKEN_B_ADDRESS + W_TOKEN_ADDRESS in: ${ENV_FILE}"
      fi
    done
    # Propagate W_TOKEN_ADDRESS to all spoke-b commercial banks (bank-b, bank-d) and their .example mirrors.
    for BANK_ENV in "${ENV_BANK_B}" "${ENV_BANK_B_EXAMPLE}" "${ENV_BANK_D}" "${ENV_BANK_D_EXAMPLE}"; do
      if [[ -f "${BANK_ENV}" ]]; then
        upsert_env "${BANK_ENV}" "W_TOKEN_ADDRESS" "${SOV_TOKEN_B}"
        echo "  Updated W_TOKEN_ADDRESS in: ${BANK_ENV}"
      fi
    done
  else
    echo "  WARN: Sovereign Token B not found in SeedNewSovereignPair broadcast — getCentralBankOf will fail." >&2
  fi

  if [[ -n "${SOV_AMM}" ]]; then
    echo "  SOVEREIGN_AMM_ADDRESS         : ${SOV_AMM}"
    # SOVEREIGN_PAIR_ID defaults to W-BRL-ARS; override via env when seeding a different pair.
    _SOV_PAIR_ID="${SOVEREIGN_PAIR_ID:-W-BRL-ARS}"
    _SOV_PAIR_AMM_MAP="{\"${_SOV_PAIR_ID}\":\"${SOV_AMM}\"}"
    for ENV_FILE in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}" \
                    "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
      if [[ -f "${ENV_FILE}" ]]; then
        upsert_env "${ENV_FILE}" "SOVEREIGN_AMM_ADDRESS" "${SOV_AMM}"
        upsert_env "${ENV_FILE}" "SOVEREIGN_PAIR_AMM_MAP" "${_SOV_PAIR_AMM_MAP}"
        echo "  Updated SOVEREIGN_AMM_ADDRESS + SOVEREIGN_PAIR_AMM_MAP in: ${ENV_FILE}"
      fi
    done
  fi
else
  echo "INFO: SeedNewSovereignPair broadcast not found — skipping sovereign token address sync." >&2
fi

# ============================================================
# LOCAL_CB_HUB_SIGNER — derive each CB gateway's Hub signer address
# from the private keys declared in contracts/.env.
# Written to .env.infra.central-bank-{a,b} so the api-gateway container
# knows which signer to match when executing a matched commit (FR-003 / T012).
# ============================================================
if command -v cast &>/dev/null; then
  echo "--- LOCAL_CB_HUB_SIGNER ---"
  _CONTRACTS_ENV="${ROOT_DIR}/contracts/.env"

  # CB-A signer comes from CENTRAL_BANK_PRIVATE_KEY (same as ADMIN_PRIVATE_KEY in local dev)
  _CBA_KEY=$(grep -E '^CENTRAL_BANK_PRIVATE_KEY=' "${_CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '[:space:]' || true)
  if [[ -n "${_CBA_KEY}" ]]; then
    _CBA_SIGNER=$(cast wallet address --private-key "${_CBA_KEY}" 2>/dev/null || true)
    if [[ -n "${_CBA_SIGNER}" ]]; then
      for ENV_FILE in "${ENV_CENTRAL_BANK_A}" "${ENV_CENTRAL_BANK_A_EXAMPLE}"; do
        [[ -f "${ENV_FILE}" ]] && upsert_env "${ENV_FILE}" "LOCAL_CB_HUB_SIGNER" "${_CBA_SIGNER}"
      done
      echo "  CB-A LOCAL_CB_HUB_SIGNER=${_CBA_SIGNER}"
    fi
  fi

  # CB-B signer comes from CENTRAL_BANK_B_PRIVATE_KEY
  _CBB_KEY=$(grep -E '^CENTRAL_BANK_B_PRIVATE_KEY=' "${_CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '[:space:]' || true)
  if [[ -n "${_CBB_KEY}" ]]; then
    _CBB_SIGNER=$(cast wallet address --private-key "${_CBB_KEY}" 2>/dev/null || true)
    if [[ -n "${_CBB_SIGNER}" ]]; then
      for ENV_FILE in "${ENV_CENTRAL_BANK_B}" "${ENV_CENTRAL_BANK_B_EXAMPLE}"; do
        if [[ -f "${ENV_FILE}" ]]; then
          upsert_env "${ENV_FILE}" "LOCAL_CB_HUB_SIGNER" "${_CBB_SIGNER}"
          # HUB_MINT_RECIPIENT must equal LOCAL_CB_HUB_SIGNER so that the W-tCeBM minted
          # by the payment-orchestrator relayer lands in the same wallet the api-gateway
          # pre-flight checks before a sovereign pool commit.
          upsert_env "${ENV_FILE}" "HUB_MINT_RECIPIENT" "${_CBB_SIGNER}"
        fi
      done
      echo "  CB-B LOCAL_CB_HUB_SIGNER=${_CBB_SIGNER} HUB_MINT_RECIPIENT=${_CBB_SIGNER}"
    fi
  fi

  if [[ -z "${_CBA_KEY:-}" && -z "${_CBB_KEY:-}" ]]; then
    echo "  WARN: CENTRAL_BANK_PRIVATE_KEY / CENTRAL_BANK_B_PRIVATE_KEY not set in contracts/.env" >&2
  fi
else
  echo "WARN: cast not found — skipping LOCAL_CB_HUB_SIGNER computation." >&2
fi

# ============================================================
# CB_TOKEN_RECIPIENT_ADDRESS — populate commercial bank envs
# with their corresponding Central Bank's Besu address (for Redeem flow).
# Commercial banks need to know where to send tokens when requesting de-tokenization.
#   bank-a, bank-c → CB-A address
#   bank-b, bank-d → CB-B address
# ============================================================
if command -v cast &>/dev/null; then
  echo "--- CB_TOKEN_RECIPIENT_ADDRESS (commercial banks) ---"
  _CONTRACTS_ENV="${ROOT_DIR}/contracts/.env"

  # CB-A address for bank-a and bank-c
  _CBA_KEY=$(grep -E '^CENTRAL_BANK_PRIVATE_KEY=' "${_CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '[:space:]' || true)
  if [[ -n "${_CBA_KEY}" ]]; then
    _CBA_ADDRESS=$(cast wallet address --private-key "${_CBA_KEY}" 2>/dev/null || true)
    if [[ -n "${_CBA_ADDRESS}" ]]; then
      for BANK_ENV in "${ENV_BANK_A}" "${ENV_BANK_A_EXAMPLE}" "${ENV_BANK_C}" "${ENV_BANK_C_EXAMPLE}"; do
        if [[ -f "${BANK_ENV}" ]]; then
          upsert_env "${BANK_ENV}" "CB_TOKEN_RECIPIENT_ADDRESS" "${_CBA_ADDRESS}"
          echo "  $(basename ${BANK_ENV}): CB_TOKEN_RECIPIENT_ADDRESS=${_CBA_ADDRESS} (CB-A)"
        fi
      done
    fi
  fi

  # CB-B address for bank-b and bank-d
  _CBB_KEY=$(grep -E '^CENTRAL_BANK_B_PRIVATE_KEY=' "${_CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '[:space:]' || true)
  if [[ -n "${_CBB_KEY}" ]]; then
    _CBB_ADDRESS=$(cast wallet address --private-key "${_CBB_KEY}" 2>/dev/null || true)
    if [[ -n "${_CBB_ADDRESS}" ]]; then
      for BANK_ENV in "${ENV_BANK_B}" "${ENV_BANK_B_EXAMPLE}" "${ENV_BANK_D}" "${ENV_BANK_D_EXAMPLE}"; do
        if [[ -f "${BANK_ENV}" ]]; then
          upsert_env "${BANK_ENV}" "CB_TOKEN_RECIPIENT_ADDRESS" "${_CBB_ADDRESS}"
          echo "  $(basename ${BANK_ENV}): CB_TOKEN_RECIPIENT_ADDRESS=${_CBB_ADDRESS} (CB-B)"
        fi
      done
    fi
  fi

  if [[ -z "${_CBA_KEY:-}" && -z "${_CBB_KEY:-}" ]]; then
    echo "  WARN: No CB private keys found in contracts/.env — skipping CB_TOKEN_RECIPIENT_ADDRESS." >&2
  fi
else
  echo "WARN: cast not found — skipping CB_TOKEN_RECIPIENT_ADDRESS computation." >&2
fi

# ============================================================
# CACTI relay env — upsert all relay variables (idempotent)
# Uses upsert_env so the file is never fully overwritten; each key is
# updated in-place or appended, preserving manually added entries.
# ============================================================
CACTI_ENV="${ROOT_DIR}/interop/hub-and-spoke/cacti/.env"
CACTI_UPDATED=0

if [[ -n "${SA_HTLC:-}" || -n "${SB_HTLC:-}" || -n "${LCR_ADDRESS:-}" ]]; then
  echo "--- Cacti relay env ---"
  # Create file with header only when it does not exist yet
  if [[ ! -f "${CACTI_ENV}" ]]; then
    echo "# Auto-generated by sync-contracts.sh — do not edit manually" > "${CACTI_ENV}"
  fi

  [[ -n "${SA_HTLC:-}" ]]         && upsert_env "${CACTI_ENV}" "SPOKE_A_HTLC_ADDRESS"           "${SA_HTLC}"
  [[ -n "${SB_HTLC:-}" ]]         && upsert_env "${CACTI_ENV}" "SPOKE_B_HTLC_ADDRESS"           "${SB_HTLC}"
  [[ -n "${SA_FX_AGREEMENT:-}" ]] && upsert_env "${CACTI_ENV}" "SPOKE_A_FX_AGREEMENT_ADDRESS"  "${SA_FX_AGREEMENT}"
  [[ -n "${SB_FX_AGREEMENT:-}" ]] && upsert_env "${CACTI_ENV}" "SPOKE_B_FX_AGREEMENT_ADDRESS"  "${SB_FX_AGREEMENT}"

  # 007-bridge-based-cb-liquidity: LCR address for the CommitMatched watcher
  if [[ -n "${LCR_ADDRESS:-}" ]]; then
    upsert_env "${CACTI_ENV}" "LIQUIDITY_COMMIT_REGISTRY_ADDRESS" "${LCR_ADDRESS}"
  fi

  # Internal gateway URLs the watcher POSTs to on CommitMatched.
  # CB-A api-gateway is on port 38080; CB-B on port 60080 (host-gateway networking).
  _GATEWAY_URLS="${GATEWAY_INTERNAL_URLS:-http://host.docker.internal:38080,http://host.docker.internal:60080}"
  upsert_env "${CACTI_ENV}" "GATEWAY_INTERNAL_URLS" "${_GATEWAY_URLS}"

  # 009-commercial-cross-currency-swap: CB-B gateway URL for CrossCurrencySwapRelay.
  # The relay forwards bridge-out notifications from CB-A to CB-B's internal endpoint.
  _CB_B_GW_URL="${CB_B_GATEWAY_URL:-http://host.docker.internal:60080}"
  upsert_env "${CACTI_ENV}" "CB_B_GATEWAY_URL" "${_CB_B_GW_URL}"

  CACTI_UPDATED=1
  echo "  Updated: ${CACTI_ENV}"
else
  echo "WARN: No Cacti relay vars to sync — skipping." >&2
fi

# ============================================================
# Grant CENTRAL_BANK_ROLE on sovereign W-tCeBM tokens to ADMIN address
#
# SeedNewSovereignPair grants CENTRAL_BANK_ROLE only to CENTRAL_BANK_B_PRIVATE_KEY
# (0xf17f52...), but the payment-orchestrator uses SIGNER_PRIVATE_KEY = ADMIN_PRIVATE_KEY
# (0x627306...) for hub mints. Without this grant, every hub mint reverts.
# The ADMIN already has DEFAULT_ADMIN_ROLE on both tokens so it can self-grant.
#
# Runs only when sovereign token addresses are known (i.e., after seed-sovereign-pair).
# On the first sync (before seed), SOV_TOKEN_A/B are empty — skipped silently.
# ============================================================
if command -v cast &>/dev/null && [[ -f "${CONTRACTS_ENV}" ]]; then
  _ADMIN_KEY_GRANT=$(grep -E '^ADMIN_PRIVATE_KEY=' "${CONTRACTS_ENV}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '[:space:]' || true)
  _ADMIN_KEY_GRANT="${_ADMIN_KEY_GRANT#0x}"
  # Resolve Hub RPC: prefer env var exported by make, fall back to localhost
  _GRANT_RPC="${SPOKE_A_RPC_URL:-${BESU_HUB_RPC:-http://127.0.0.1:8645}}"
  _ROLE=$(cast keccak "CENTRAL_BANK_ROLE" 2>/dev/null || true)

  if [[ -n "${_ADMIN_KEY_GRANT}" && -n "${_ROLE}" ]]; then
    _ADMIN_ADDR=$(cast wallet address --private-key "${_ADMIN_KEY_GRANT}" 2>/dev/null || true)

    # Collect all hub tokens that need CENTRAL_BANK_ROLE for the ADMIN signer:
    #   - Sovereign W-tCeBM tokens (deployed by SeedNewSovereignPair, only present on 2nd sync)
    #   - Hub tCeBM tokens (deployed by CBWeb3Hub, present on both syncs)
    _GRANT_TOKENS=()
    [[ -n "${SOV_TOKEN_A:-}" ]] && _GRANT_TOKENS+=("${SOV_TOKEN_A}")
    [[ -n "${SOV_TOKEN_B:-}" ]] && _GRANT_TOKENS+=("${SOV_TOKEN_B}")
    [[ -n "${HUB_TOKEN_A:-}" ]] && _GRANT_TOKENS+=("${HUB_TOKEN_A}")
    [[ -n "${HUB_TOKEN_B:-}" ]] && _GRANT_TOKENS+=("${HUB_TOKEN_B}")

    if [[ ${#_GRANT_TOKENS[@]} -gt 0 ]]; then
      echo "--- CENTRAL_BANK_ROLE grants on hub tokens for ADMIN signer (${_ADMIN_ADDR}) ---"
      for _TOKEN in "${_GRANT_TOKENS[@]}"; do
        _HAS_ROLE=$(cast call "${_TOKEN}" "hasRole(bytes32,address)(bool)" "${_ROLE}" "${_ADMIN_ADDR}" \
          --rpc-url "${_GRANT_RPC}" 2>/dev/null || echo "error")
        if [[ "${_HAS_ROLE}" == "false" ]]; then
          echo "  Granting on ${_TOKEN}..."
          cast send "${_TOKEN}" "grantRole(bytes32,address)" "${_ROLE}" "${_ADMIN_ADDR}" \
            --private-key "${_ADMIN_KEY_GRANT}" --rpc-url "${_GRANT_RPC}" \
            --json 2>/dev/null | python3 -c \
              "import sys,json; d=json.load(sys.stdin); print('  TX:', d.get('transactionHash','?'), 'status:', d.get('status','?'))" \
            2>/dev/null || echo "  WARN: grant failed — check RPC and key" >&2
        elif [[ "${_HAS_ROLE}" == "true" ]]; then
          echo "  Already granted on ${_TOKEN} — skip"
        fi
      done
    fi
  fi
fi

# --- Summary ---
echo ""
echo "=========================================="
echo "  CONTRACT ADDRESSES SYNCED"
echo "=========================================="
echo "  Files updated: $((UPDATED + CACTI_UPDATED))"
[[ ${CACTI_UPDATED} -eq 1 ]] && echo "  Cacti env   : ${CACTI_ENV}"
echo "=========================================="
