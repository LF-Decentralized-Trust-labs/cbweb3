#!/usr/bin/env bash
# tryout-diagnostico-cb-liquidity.sh
# Detailed diagnostic for sovereign liquidity injection flow (CB-A and CB-B)
# Goal: quickly identify where the flow may stall or fail

set -euo pipefail

# ─── Variable configuration ────────────────────────────────────────────────────
CB_A_URL="${CB_A_URL:-http://localhost:38080}"
CB_B_URL="${CB_B_URL:-http://localhost:60080}"
CB_A_ENV="${CB_A_ENV:-backend/config/.env.infra.central-bank-a}"
CB_B_ENV="${CB_B_ENV:-backend/config/.env.infra.central-bank-b}"
POOL_PAIR="${POOL_PAIR:-W-BRL-ARS}"
AMOUNT_A="${AMOUNT_A:-100000000000000000000000}"
AMOUNT_B="${AMOUNT_B:-200000000000000000000000}"
SPOKE_A_ASSET="${SPOKE_A_ASSET:-tCeBM_BRL}"
SPOKE_B_ASSET="${SPOKE_B_ASSET:-tCeBM_ARS}"
SPOKE_A_NETWORK="${SPOKE_A_NETWORK:-spoke-a}"
SPOKE_B_NETWORK="${SPOKE_B_NETWORK:-spoke-b}"

# Read token addresses from .env
_W_TOK_A_FROM_ENV=$(grep -m1 "^SOVEREIGN_HUB_TOKEN_A_ADDRESS=" "${CB_A_ENV}" 2>/dev/null | cut -d= -f2)
_W_TOK_B_FROM_ENV=$(grep -m1 "^SOVEREIGN_HUB_TOKEN_B_ADDRESS=" "${CB_B_ENV}" 2>/dev/null | cut -d= -f2)
W_TOKEN_A_ADDR="${W_TOKEN_A_ADDR:-${_W_TOK_A_FROM_ENV:-}}"
W_TOKEN_B_ADDR="${W_TOKEN_B_ADDR:-${_W_TOK_B_FROM_ENV:-}}"

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
    set -a; source "$envfile"; set +a
  fi
}

cb_api_token() {
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

# ─── 1. Authentication ─────────────────────────────────────────────────────────
sep
info "Loading CB-A credentials from ${CB_A_ENV}"
load_env "$CB_A_ENV"
KC_CLIENT_A="${KC_CLIENT_ID:?CB-A KC_CLIENT_ID not set}"
KC_SECRET_A="${KC_CLIENT_SECRET:?CB-A KC_CLIENT_SECRET not set}"
CB_A_BANK_ID="${BANK_CODE:-central-bank-a}"

TOKEN_A=$(cb_api_token "${CB_A_URL}" "${KC_CLIENT_A}" "${KC_SECRET_A}")
[[ -n "$TOKEN_A" ]] && pass "CB-A authenticated" || fail "CB-A authentication failed"

info "Loading CB-B credentials from ${CB_B_ENV}"
load_env "$CB_B_ENV"
KC_CLIENT_B="${KC_CLIENT_ID:?CB-B KC_CLIENT_ID not set}"
KC_SECRET_B="${KC_CLIENT_SECRET:?CB-B KC_CLIENT_SECRET not set}"
CB_B_BANK_ID="${BANK_CODE:-central-bank-b}"

TOKEN_B=$(cb_api_token "${CB_B_URL}" "${KC_CLIENT_B}" "${KC_SECRET_B}")
[[ -n "$TOKEN_B" ]] && pass "CB-B authenticated" || fail "CB-B authentication failed"

# ─── 2. Validate token addresses ───────────────────────────────────────────────
[[ -n "${W_TOKEN_A_ADDR:-}" ]] || fail "W_TOKEN_A_ADDR empty — check SOVEREIGN_HUB_TOKEN_A_ADDRESS"
[[ -n "${W_TOKEN_B_ADDR:-}" ]] || fail "W_TOKEN_B_ADDR empty — check SOVEREIGN_HUB_TOKEN_B_ADDRESS"
pass "W-token addresses: A=${W_TOKEN_A_ADDR}  B=${W_TOKEN_B_ADDR}"

# ─── 3. Bridge Lock&Mint ──────────────────────────────────────────────────────
sep
info "CB-A: Lock&Mint (API resolves owner_bank_id, spoke_network, native_asset, mirrored_asset from JWT+config)"
PAYLOAD_A_LOCK='{"amount":"'"${AMOUNT_A}"'"}'
info "  Simplified Payload (POST /api/v2/bridge/lock-mint):"
echo "$PAYLOAD_A_LOCK" | jq . | sed 's/^/    /'
LOCK_A=$(api_post "${CB_A_URL}/api/v2/bridge/lock-mint" "$TOKEN_A" "$PAYLOAD_A_LOCK")
POS_A_ID=$(echo "$LOCK_A" | jq -r '.position_id // empty')
[[ -n "$POS_A_ID" ]] && pass "CB-A lock-mint position_id=${POS_A_ID}" || fail "CB-A lock-mint failed: ${LOCK_A}"

info "CB-B: Lock&Mint (API resolves owner_bank_id, spoke_network, native_asset, mirrored_asset from JWT+config)"
PAYLOAD_B_LOCK='{"amount":"'"${AMOUNT_B}"'"}'
info "  Simplified Payload (POST /api/v2/bridge/lock-mint):"
echo "$PAYLOAD_B_LOCK" | jq . | sed 's/^/    /'
LOCK_B=$(api_post "${CB_B_URL}/api/v2/bridge/lock-mint" "$TOKEN_B" "$PAYLOAD_B_LOCK")
POS_B_ID=$(echo "$LOCK_B" | jq -r '.position_id // empty')
[[ -n "$POS_B_ID" ]] && pass "CB-B lock-mint position_id=${POS_B_ID}" || fail "CB-B lock-mint failed: ${LOCK_B}"

# ─── 4. Poll until bridge_state=ACTIVE ──────────────────────────────────────
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

# ─── 5. On-chain Commit ──────────────────────────────────────────────────────
sep
info "CB-A: Commit (API resolves provider_id, side, w_token_address from JWT+config)"
COMMIT_BODY_A=$(jq -nc \
  --arg pair "$POOL_PAIR" \
  --arg amt "$AMOUNT_A" \
  '{"pool_pair":$pair,"amount":$amt}')
info "  Simplified Payload (POST /api/v2/amm/liquidity/commit):"
echo "$COMMIT_BODY_A" | jq . | sed 's/^/    /'
COMMIT_A=$(api_post "${CB_A_URL}/api/v2/amm/liquidity/commit" "$TOKEN_A" "$COMMIT_BODY_A")
COMMIT_A_ID=$(echo "$COMMIT_A" | jq -r '.commit_id // empty')
[[ -n "$COMMIT_A_ID" ]] && pass "CB-A commit_id=${COMMIT_A_ID}" || fail "CB-A commit failed: ${COMMIT_A}"

info "CB-B: Commit (API resolves provider_id, side, w_token_address from JWT+config)"
COMMIT_BODY_B=$(jq -nc \
  --arg pair "$POOL_PAIR" \
  --arg amt "$AMOUNT_B" \
  '{"pool_pair":$pair,"amount":$amt}')
info "  Simplified Payload (POST /api/v2/amm/liquidity/commit):"
echo "$COMMIT_BODY_B" | jq . | sed 's/^/    /'
COMMIT_B=$(api_post "${CB_B_URL}/api/v2/amm/liquidity/commit" "$TOKEN_B" "$COMMIT_BODY_B")
COMMIT_B_ID=$(echo "$COMMIT_B" | jq -r '.commit_id // empty')
[[ -n "$COMMIT_B_ID" ]] && pass "CB-B commit_id=${COMMIT_B_ID}" || fail "CB-B commit failed: ${COMMIT_B}"

# ─── 6. Poll until EXECUTED ─────────────────────────────────────────────────
poll_commit_executed() {
  local url="$1" token="$2" commit_id="$3" label="$4"
  local timeout=180 elapsed=0
  info "${label}: Polling commit until EXECUTED (max ${timeout}s, interval 3s)…"
  while [ $elapsed -lt $timeout ]; do
    local st
    st=$(curl -sf "${url}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=EXECUTED" \
      -H "Authorization: Bearer ${token}" 2>/dev/null \
      | jq -r --arg cid "$commit_id" '.commits[]? | select(.CommitID==$cid) | .Status // ""')
    if [[ "$st" == "EXECUTED" ]]; then
      pass "${label}: commit EXECUTED (after ${elapsed}s)"
      return 0
    fi
    sleep 3
    elapsed=$((elapsed + 3))
  done
  fail "${label}: commit did not reach EXECUTED within ${timeout}s — check watcher"
}

poll_commit_executed "$CB_A_URL" "$TOKEN_A" "$COMMIT_A_ID" "CB-A"
poll_commit_executed "$CB_B_URL" "$TOKEN_B" "$COMMIT_B_ID" "CB-B"

# ─── 7. Pool verification ──────────────────────────────────────────────────
sep
info "Checking pool status via CB-A"
POOL_STATUS=$(api_get "${CB_A_URL}/api/v2/amm/pool/${POOL_PAIR}/status" "$TOKEN_A")
echo "$POOL_STATUS"

info "CB-A: Query LP positions (GET /api/v2/amm/liquidity/positions)"
LP_POSITIONS_A=$(api_get "${CB_A_URL}/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" "$TOKEN_A")
echo "$LP_POSITIONS_A" | jq '{pool_pair, count, positions: [.positions[]? | {lp_id, provider_bank_id, deposit_side, token_a_contributed, token_b_contributed}]}'

info "CB-B: Query LP positions (GET /api/v2/amm/liquidity/positions)"
LP_POSITIONS_B=$(api_get "${CB_B_URL}/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" "$TOKEN_B")
echo "$LP_POSITIONS_B" | jq '{pool_pair, count, positions: [.positions[]? | {lp_id, provider_bank_id, deposit_side, token_a_contributed, token_b_contributed}]}'

info "Diagnostic complete!"
