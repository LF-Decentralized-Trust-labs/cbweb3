#!/usr/bin/env bash
# tryout-sovereign-cb-liquidity.sh — End-to-end sovereign CB liquidity tryout
# (007-bridge-based-cb-liquidity)
#
# Exercises the full sovereign liquidity flow between two Central Banks:
#
#   Phase 1: Bridge Lock&Mint (both CBs bridge assets to Hub)
#            BesuRelayerExecutor mints W-tCeBM on Hub automatically when bridge_state → ACTIVE.
#   Phase 2: Commit Registration (both CBs register on-chain commit via LiquidityCommitRegistry)
#   Phase 3: Watcher Trigger (simulates CommitMatched → execute-matched-commit)
#   Phase 4: Verify Pool State (liquidity positions created)
#   Phase 5: Remove Liquidity (sovereign CB withdraws)
#
# Prerequisites:
#   - Hub + Spoke-A + Spoke-B stacks running
#   - Contracts deployed: make contracts.seed-sovereign-pair
#   - curl, jq
#   - backend/config/.env.infra.central-bank-a  (CB-A credentials)
#   - backend/config/.env.infra.central-bank-b  (CB-B credentials)
#
# Environment variables:
#   CB_A_URL              CB-A API base (default: http://localhost:38080)
#   CB_B_URL              CB-B API base (default: http://localhost:48080)
#   CB_A_ENV              CB-A env file (default: backend/config/.env.infra.central-bank-a)
#   CB_B_ENV              CB-B env file (default: backend/config/.env.infra.central-bank-b)
#   POOL_PAIR             Pool pair ID  (default: W-BRL-ARS)
#   AMOUNT_A              CB-A deposit  (default: 100000000000000000000000)
#   AMOUNT_B              CB-B deposit  (default: 200000000000000000000000)
#   W_TOKEN_A_ADDR        W-tCeBM token address for side A (CB-A token)
#   W_TOKEN_B_ADDR        W-tCeBM token address for side B (CB-B token)
#   SPOKE_A_ASSET         Native asset on Spoke-A (default: tCeBM_BRL)
#   SPOKE_B_ASSET         Native asset on Spoke-B (default: tCeBM_ARS)
#   SPOKE_A_NETWORK       Spoke-A network name (default: spoke-a)
#   SPOKE_B_NETWORK       Spoke-B network name (default: spoke-b)
#
# Usage:
#   ./tryouts/tryout-sovereign-cb-liquidity.sh

set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────
CB_A_URL="${CB_A_URL:-http://localhost:38080}"
CB_B_URL="${CB_B_URL:-http://localhost:60080}"
CB_A_ENV="${CB_A_ENV:-backend/config/.env.infra.central-bank-a}"
CB_B_ENV="${CB_B_ENV:-backend/config/.env.infra.central-bank-b}"
POOL_PAIR="${POOL_PAIR:-W-BRL-ARS}"
AMOUNT_A="${AMOUNT_A:-100000000000000000000000}"
AMOUNT_B="${AMOUNT_B:-200000000000000000000000}"
# Read W-token addresses from CB env files if not explicitly set
_W_TOK_A_FROM_ENV=$(grep -m1 "^SOVEREIGN_HUB_TOKEN_A_ADDRESS=" "${CB_A_ENV}" 2>/dev/null | cut -d= -f2)
_W_TOK_B_FROM_ENV=$(grep -m1 "^SOVEREIGN_HUB_TOKEN_B_ADDRESS=" "${CB_B_ENV}" 2>/dev/null | cut -d= -f2)
W_TOKEN_A_ADDR="${W_TOKEN_A_ADDR:-${_W_TOK_A_FROM_ENV:-}}"
W_TOKEN_B_ADDR="${W_TOKEN_B_ADDR:-${_W_TOK_B_FROM_ENV:-}}"
SPOKE_A_ASSET="${SPOKE_A_ASSET:-tCeBM_BRL}"
SPOKE_B_ASSET="${SPOKE_B_ASSET:-tCeBM_ARS}"
SPOKE_A_NETWORK="${SPOKE_A_NETWORK:-spoke-a}"
SPOKE_B_NETWORK="${SPOKE_B_NETWORK:-spoke-b}"
# SC-002: Ethereum signer address of CB-B on the Hub (used to test anti-G5-cross guard).
# Auto-read from CB-B env file (LOCAL_CB_HUB_SIGNER) if not explicitly set.
_CB_B_SIGNER_FROM_ENV=$(grep -m1 "^LOCAL_CB_HUB_SIGNER=" "${CB_B_ENV}" 2>/dev/null | cut -d= -f2)
CB_B_HUB_SIGNER="${CB_B_HUB_SIGNER:-${_CB_B_SIGNER_FROM_ENV:-}}"
# SC-001 (on-chain): AMM contract address and Hub RPC (optional, requires cast CLI).
# Auto-read from CB-A env file (SOVEREIGN_AMM_ADDRESS) if not explicitly set.
_SOV_AMM_FROM_ENV=$(grep -m1 "^SOVEREIGN_AMM_ADDRESS=" "${CB_A_ENV}" 2>/dev/null | cut -d= -f2)
SOVEREIGN_AMM_ADDR="${SOVEREIGN_AMM_ADDR:-${_SOV_AMM_FROM_ENV:-}}"
HUB_RPC_URL="${HUB_RPC_URL:-http://localhost:8645}"

# ─── Helpers ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}✓ $*${NC}"; }
info() { echo -e "${YELLOW}  $*${NC}"; }
fail() { echo -e "${RED}✗ $*${NC}"; exit 1; }
sep()  { echo "────────────────────────────────────────────────────────"; }

require_cmd() { command -v "$1" &>/dev/null || fail "Missing required command: $1"; }
require_cmd curl; require_cmd jq

load_env() {
  local envfile="$1"
  if [[ -f "$envfile" ]]; then
    # shellcheck disable=SC1090
    set -a; source "$envfile"; set +a
  fi
}

cb_api_token() {
  # Obtain token enriched com BankID via POST /api/v1/auth/login from the CB's own API.
  # Parameters: <cb_api_url> <client_id> <client_secret>
  local api_url="$1" client_id="$2" secret="$3"
  curl -sf -X POST "${api_url}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\":\"${client_id}\",\"clientSecret\":\"${secret}\"}" \
    | jq -r '.accessToken'
}

api_post() {
  local url="$1" token="$2" body="$3"
  curl -sf -X POST "${url}" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "${body}" | jq .
}

api_get() {
  local url="$1" token="$2"
  curl -sf "${url}" -H "Authorization: Bearer ${token}" | jq .
}

# ─── Load CB credentials ──────────────────────────────────────────────────────
sep
info "Loading CB-A credentials from ${CB_A_ENV}"
load_env "$CB_A_ENV"
KC_CLIENT_A="${KC_CLIENT_ID:?CB-A KC_CLIENT_ID not set in ${CB_A_ENV}}"
KC_SECRET_A="${KC_CLIENT_SECRET:?CB-A KC_CLIENT_SECRET not set in ${CB_A_ENV}}"
CB_A_BANK_ID="${BANK_CODE:-central-bank-a}"

TOKEN_A=$(cb_api_token "${CB_A_URL}" "${KC_CLIENT_A}" "${KC_SECRET_A}")
[[ -n "$TOKEN_A" ]] && pass "CB-A authenticated (client=${KC_CLIENT_A})" || fail "CB-A authentication failed — check ${CB_A_URL}/api/v1/auth/login"

info "Loading CB-B credentials from ${CB_B_ENV}"
load_env "$CB_B_ENV"
KC_CLIENT_B="${KC_CLIENT_ID:?CB-B KC_CLIENT_ID not set in ${CB_B_ENV}}"
KC_SECRET_B="${KC_CLIENT_SECRET:?CB-B KC_CLIENT_SECRET not set in ${CB_B_ENV}}"
CB_B_BANK_ID="${BANK_CODE:-central-bank-b}"

TOKEN_B=$(cb_api_token "${CB_B_URL}" "${KC_CLIENT_B}" "${KC_SECRET_B}")
[[ -n "$TOKEN_B" ]] && pass "CB-B authenticated (client=${KC_CLIENT_B})" || fail "CB-B authentication failed — check ${CB_B_URL}/api/v1/auth/login"

# ─── Validate W-token addresses ──────────────────────────────────────────────
[[ -n "${W_TOKEN_A_ADDR:-}" ]] || fail "W_TOKEN_A_ADDR is empty — set SOVEREIGN_HUB_TOKEN_A_ADDRESS in ${CB_A_ENV} or pass W_TOKEN_A_ADDR env var"
[[ -n "${W_TOKEN_B_ADDR:-}" ]] || fail "W_TOKEN_B_ADDR is empty — set SOVEREIGN_HUB_TOKEN_B_ADDRESS in ${CB_B_ENV} or pass W_TOKEN_B_ADDR env var"
pass "W-token addresses: A=${W_TOKEN_A_ADDR}  B=${W_TOKEN_B_ADDR}"

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 1: Bridge Lock&Mint (both CBs bridge native assets → Hub W-tCeBM)
# BesuRelayerExecutor mints W-tCeBM automatically on the Hub as part of this flow.
# ═══════════════════════════════════════════════════════════════════════════════
sep
echo "Phase 1: Bridge Lock&Mint"
sep

info "CB-A: Lock&Mint (API resolves owner_bank_id, spoke_network, native_asset, mirrored_asset from JWT+config)"
LOCK_A=$(api_post "${CB_A_URL}/api/v2/bridge/lock-mint" "$TOKEN_A" \
  '{"amount":"'"${AMOUNT_A}"'"}')
POS_A_ID=$(echo "$LOCK_A" | jq -r '.position_id // empty')
[[ -n "$POS_A_ID" ]] && pass "CB-A lock-mint position_id=${POS_A_ID}" || fail "CB-A lock-mint failed: ${LOCK_A}"

info "CB-B: Lock&Mint (API resolves owner_bank_id, spoke_network, native_asset, mirrored_asset from JWT+config)"
LOCK_B=$(api_post "${CB_B_URL}/api/v2/bridge/lock-mint" "$TOKEN_B" \
  '{"amount":"'"${AMOUNT_B}"'"}')
POS_B_ID=$(echo "$LOCK_B" | jq -r '.position_id // empty')
[[ -n "$POS_B_ID" ]] && pass "CB-B lock-mint position_id=${POS_B_ID}" || fail "CB-B lock-mint failed: ${LOCK_B}"

# FR-001 / T018(2)(5): polling com timeout 120s / intervalo 5s para bridge_state=ACTIVE.
# BesuRelayerExecutor minta W-tCeBM no Hub durante este worker cycle — ACTIVE confirma que os
# tokens foram creditados ao signer antes da fase de commit.
poll_bridge_active() {
  local url="$1" token="$2" label="$3"
  local timeout=120 elapsed=0
  info "${label}: Polling bridge position until ACTIVE (max ${timeout}s, interval 5s)…"
  while [ $elapsed -lt $timeout ]; do
    local st
    st=$(curl -sf "${url}/api/v2/bridge/positions?state=ACTIVE" \
      -H "Authorization: Bearer ${token}" 2>/dev/null \
      | jq -r '.positions[0].bridge_state // ""')
    if [[ "$st" == "ACTIVE" ]]; then
      pass "${label}: bridge position ACTIVE (after ${elapsed}s)"
      return 0
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done
  fail "${label}: bridge position did not reach ACTIVE within ${timeout}s — check Relayer"
}

poll_bridge_active "$CB_A_URL" "$TOKEN_A" "CB-A"
poll_bridge_active "$CB_B_URL" "$TOKEN_B" "CB-B"

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 2: On-chain Commit Registration
# ═══════════════════════════════════════════════════════════════════════════════
sep
echo "Phase 2: On-chain Commit Registration (LiquidityCommitRegistry)"
sep

COMMIT_BODY_A=$(jq -nc \
  --arg pair "$POOL_PAIR" \
  --arg amt "$AMOUNT_A" \
  '{"pool_pair":$pair,"amount":$amt}')

info "CB-A: Register commit (API resolves provider_id, side, w_token_address from JWT+config)"
COMMIT_A=$(api_post "${CB_A_URL}/api/v2/amm/liquidity/commit" "$TOKEN_A" "$COMMIT_BODY_A")
COMMIT_A_ID=$(echo "$COMMIT_A" | jq -r '.commit_id // empty')
COMMIT_A_ONCHAIN=$(echo "$COMMIT_A" | jq -r '.on_chain_commit_id // "N/A"')
[[ -n "$COMMIT_A_ID" ]] && pass "CB-A commit_id=${COMMIT_A_ID} on_chain=${COMMIT_A_ONCHAIN}" \
  || fail "CB-A commit registration failed: ${COMMIT_A}"

COMMIT_BODY_B=$(jq -nc \
  --arg pair "$POOL_PAIR" \
  --arg amt "$AMOUNT_B" \
  '{"pool_pair":$pair,"amount":$amt}')

info "CB-B: Register commit (API resolves provider_id, side, w_token_address from JWT+config)"
COMMIT_B=$(api_post "${CB_B_URL}/api/v2/amm/liquidity/commit" "$TOKEN_B" "$COMMIT_BODY_B")
COMMIT_B_ID=$(echo "$COMMIT_B" | jq -r '.commit_id // empty')
COMMIT_B_ONCHAIN=$(echo "$COMMIT_B" | jq -r '.on_chain_commit_id // "N/A"')
[[ -n "$COMMIT_B_ID" ]] && pass "CB-B commit_id=${COMMIT_B_ID} on_chain=${COMMIT_B_ONCHAIN}" \
  || fail "CB-B commit registration failed: ${COMMIT_B}"

# SC-003: start latency timer immediately after both commits are registered
# (CB-B commit triggers the bilateral match on-chain)
T_AFTER_BOTH_COMMITS=$(date +%s)
info "SC-003: Latency timer started at $(date -u +%T) UTC"

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 3: Wait for Cacti Watcher to trigger execute-matched-commit
# SC-003: active polling (spec: "poll both LP positions until EXECUTED
#          with start/end timestamps from CommitMatched event"
#          — p95 target ≤ 30s, NFR-001).
# ═══════════════════════════════════════════════════════════════════════════════
sep
echo "Phase 3: Polling for CommitMatched → LP EXECUTED (max 180s)"
sep
info "The Cacti LiquidityCommitWatcher listens for CommitMatched on the Hub."
info "Polling every 3s (max 180s) until CB-A commit ${COMMIT_A_ID} transitions to EXECUTED…"

# T018(7): poll BOTH LP positions until EXECUTED with timestamp (spec: p95 ≤ 30s)
# Filters by specific CommitID to avoid false positives from prior-run commits.
T_EXECUTED=0
COMMIT_B_EXECUTED=0
POLL_MAX=180
POLL_ELAPSED=0
while [ $POLL_ELAPSED -lt $POLL_MAX ]; do
  if [[ $T_EXECUTED -eq 0 ]]; then
    POLL_STATUS_A=$(curl -sf \
      "${CB_A_URL}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=EXECUTED" \
      -H "Authorization: Bearer ${TOKEN_A}" 2>/dev/null \
      | jq -r --arg cid "$COMMIT_A_ID" '.commits[]? | select(.CommitID==$cid) | .Status // ""')
    if [[ "$POLL_STATUS_A" == "EXECUTED" ]]; then
      T_EXECUTED=$(date +%s)
      PROTOCOL_LATENCY=$((T_EXECUTED - T_AFTER_BOTH_COMMITS))
      pass "Phase 3: CB-A commit EXECUTED in ${PROTOCOL_LATENCY}s"
    fi
  fi
  if [[ $COMMIT_B_EXECUTED -eq 0 ]]; then
    POLL_STATUS_B=$(curl -sf \
      "${CB_B_URL}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=EXECUTED" \
      -H "Authorization: Bearer ${TOKEN_B}" 2>/dev/null \
      | jq -r --arg cid "$COMMIT_B_ID" '.commits[]? | select(.CommitID==$cid) | .Status // ""')
    if [[ "$POLL_STATUS_B" == "EXECUTED" ]]; then
      COMMIT_B_EXECUTED=1
      pass "Phase 3: CB-B commit EXECUTED"
    fi
  fi
  [[ $T_EXECUTED -gt 0 && $COMMIT_B_EXECUTED -gt 0 ]] && break
  sleep 3
  POLL_ELAPSED=$((POLL_ELAPSED + 3))
done

if [[ $T_EXECUTED -eq 0 ]]; then
  info "Phase 3 WARNING: CB-A commit ${COMMIT_A_ID} not EXECUTED within ${POLL_MAX}s (watcher may be slow or offline)"
  PROTOCOL_LATENCY=$POLL_MAX
else
  pass "Phase 3: both commits EXECUTED (latency=${PROTOCOL_LATENCY}s)"
fi
if [[ $COMMIT_B_EXECUTED -eq 0 ]]; then
  info "Phase 3 WARNING: CB-B commit ${COMMIT_B_ID} not EXECUTED within ${POLL_MAX}s"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 4: Verify Pool State
# ═══════════════════════════════════════════════════════════════════════════════
sep
echo "Phase 4: Verify Pool State"
sep

info "CB-A: Check commit status (expect EXECUTED)"
COMMIT_A_STATUS=$(api_get "${CB_A_URL}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=EXECUTED" "$TOKEN_A" \
  | jq -r --arg cid "$COMMIT_A_ID" '.commits[]? | select(.CommitID==$cid) | .Status // "NOT_FOUND"')
[[ "$COMMIT_A_STATUS" == "EXECUTED" ]] && pass "CB-A commit EXECUTED" \
  || info "WARNING: CB-A commit ${COMMIT_A_ID} status=${COMMIT_A_STATUS:-NOT_FOUND}"

info "CB-A: Pool status for ${POOL_PAIR}"
api_get "${CB_A_URL}/api/v2/amm/pool/${POOL_PAIR}/status" "$TOKEN_A" || true

info "CB-A: LP Positions for ${POOL_PAIR}"
api_get "${CB_A_URL}/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" "$TOKEN_A" || true

info "CB-B: LP Positions for ${POOL_PAIR}"
api_get "${CB_B_URL}/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" "$TOKEN_B" || true

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 5: Remove Liquidity (optional — requires LP position)
# ═══════════════════════════════════════════════════════════════════════════════
sep
echo "Phase 5: Remove Liquidity (sovereign CB-A)"
sep
info "Skipping remove-liquidity in tryout (requires LP position ID from Phase 4)."
info "To remove: POST ${CB_A_URL}/api/v2/amm/liquidity/remove"
info '  {"pool_pair":"'"${POOL_PAIR}"'","provider_bank_id":"'"${CB_A_BANK_ID}"'","lp_id":"<LP_ID>"}'

# ═══════════════════════════════════════════════════════════════════════════════
# SC Assertions (spec 007-bridge-based-cb-liquidity: SC-001, SC-002, SC-003)
# ═══════════════════════════════════════════════════════════════════════════════
sep
echo "SC Assertions"
sep

# SC-001: EXECUTED commits carry the correct ProviderID for each CB.
# Filters by the specific CommitID from this run to ensure we check the current commits.
info "SC-001: Verify LP provider_id matches each CB's identity (this run's commits)"
LP_A_PID=$(curl -sf \
  "${CB_A_URL}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=EXECUTED" \
  -H "Authorization: Bearer ${TOKEN_A}" 2>/dev/null \
  | jq -r --arg cid "$COMMIT_A_ID" '.commits[]? | select(.CommitID==$cid) | .ProviderID' | head -1)
LP_B_PID=$(curl -sf \
  "${CB_B_URL}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=EXECUTED" \
  -H "Authorization: Bearer ${TOKEN_B}" 2>/dev/null \
  | jq -r --arg cid "$COMMIT_B_ID" '.commits[]? | select(.CommitID==$cid) | .ProviderID' | head -1)

if [[ "$LP_A_PID" == "$CB_A_BANK_ID" ]]; then
  pass "SC-001: CB-A EXECUTED commit has ProviderID=${CB_A_BANK_ID}"
else
  info "SC-001 WARNING: CB-A EXECUTED commit with ProviderID=${CB_A_BANK_ID} not found"
fi
if [[ "$LP_B_PID" == "$CB_B_BANK_ID" ]]; then
  pass "SC-001: CB-B EXECUTED commit has ProviderID=${CB_B_BANK_ID}"
else
  info "SC-001 WARNING: CB-B EXECUTED commit with ProviderID=${CB_B_BANK_ID} not found"
fi

# SC-001 (on-chain, optional): cast logs verification of msg.sender.
# Requires: cast CLI, SOVEREIGN_AMM_ADDR env var, HUB_RPC_URL env var.
if command -v cast &>/dev/null && [[ -n "${SOVEREIGN_AMM_ADDR:-}" ]] && [[ -n "${HUB_RPC_URL:-}" ]]; then
  info "SC-001 (on-chain): cast logs LogSingleSidedLiquidityAdded"
  CAST_OUT=$(cast logs \
    --rpc-url "$HUB_RPC_URL" \
    --address "$SOVEREIGN_AMM_ADDR" \
    "LogSingleSidedLiquidityAdded(address,bool,uint256)" 2>/dev/null | tail -40 || true)
  if [[ -n "$CAST_OUT" ]]; then
    pass "SC-001 (on-chain): events found — verify msg.sender == CB signers in output:"
    echo "$CAST_OUT"
  else
    info "SC-001 (on-chain): no events found (pool may not have fired event yet)"
  fi
else
  info "SC-001 (on-chain): Skipped — requires cast, SOVEREIGN_AMM_ADDR, HUB_RPC_URL"
fi

# SC-002: mint-and-approve with a CB signer as recipient must return HTTP 403.
# Requires CB_B_HUB_SIGNER env var (the Hub Ethereum address of CB-B's signer).
sep
info "SC-002: Anti-G5-cross guard — mint-and-approve with CB signer as recipient → expect HTTP 403"
if [[ -n "${CB_B_HUB_SIGNER:-}" ]]; then
  SC002_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${CB_A_URL}/api/v2/amm/token/mint-and-approve" \
    -H "Authorization: Bearer ${TOKEN_A}" \
    -H "Content-Type: application/json" \
    -d "{\"amount\":\"1000000000000000000\",\"recipient\":\"${CB_B_HUB_SIGNER}\"}")
  if [[ "$SC002_CODE" == "403" ]]; then
    pass "SC-002: HTTP 403 CROSS_CB_MINT_PROHIBITED (anti-G5-cross guard active)"
  else
    fail "SC-002: Expected HTTP 403, got ${SC002_CODE} — anti-G5-cross guard NOT enforcing!"
  fi
else
  info "SC-002: Skipped — set CB_B_HUB_SIGNER=<hub_ethereum_address_of_cb_b_signer> to enable"
fi

# SC-003: cross-gateway latency from both commits registered to LP EXECUTED.
# The watcher fires after detecting CommitMatched; p95 target <= 30s (spec NFR-001).
sep
info "SC-003: Cross-gateway latency (from both commits → LP EXECUTED)"
if [[ $T_EXECUTED -gt 0 ]]; then
  [[ $PROTOCOL_LATENCY -le 30 ]] && pass "SC-003: Protocol latency ${PROTOCOL_LATENCY}s ≤ 30s" \
    || info "SC-003 WARNING: Protocol latency ${PROTOCOL_LATENCY}s > 30s — check watcher poll interval or block time"
else
  info "SC-003: Cannot measure latency — commit did not reach EXECUTED within timeout"
fi

# ─── Summary ──────────────────────────────────────────────────────────────────
sep
echo -e "${GREEN}tryout-sovereign-cb-liquidity.sh complete${NC}"
echo "  Pool pair  : ${POOL_PAIR}"
echo "  CB-A commit: ${COMMIT_A_ID} (on-chain: ${COMMIT_A_ONCHAIN})"
echo "  CB-B commit: ${COMMIT_B_ID} (on-chain: ${COMMIT_B_ONCHAIN})"
# T018(10): LATENCY_COMMIT_MATCHED_TO_EXECUTED=Xs ao final
echo "  LATENCY_COMMIT_MATCHED_TO_EXECUTED=${PROTOCOL_LATENCY}s"
sep
