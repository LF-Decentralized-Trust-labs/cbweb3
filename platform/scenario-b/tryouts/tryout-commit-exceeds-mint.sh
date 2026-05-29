#!/usr/bin/env bash
# tryout-commit-exceeds-mint.sh
#
# Reproduces the reported scenario: commit with amount greater than Hub lock-mint.
#
#   CB-A: lock-mint 90k → commit 90k  (should accept)
#   CB-B: lock-mint 70k → commit 90k  (should reject with INSUFFICIENT_BALANCE)
#
# Without balance validation on POST /api/v2/amm/liquidity/commit, the pair would match,
# side A deposits into the pool and side B fails on-chain — reserve_a > 0, reserve_b = 0.
#
# Prerequisites:
#   - Scenario B stack: make scenario-b.up
#   - CB api-gateways rebuilt with balance check fix
#   - curl, jq
#
# Usage (defaults = 90k / 70k mint, 90k commit, 18 decimals):
#   ./tryouts/tryout-commit-exceeds-mint.sh
#
# Variables:
#   CB_A_URL, CB_B_URL, CB_A_ENV, CB_B_ENV, POOL_PAIR
#   LOCK_MINT_A   — wei minted on CB-A bridge (default: 90000000000000000000000)
#   LOCK_MINT_B   — wei minted on CB-B bridge (default: 70000000000000000000000)
#   COMMIT_AMOUNT — wei sent in both commits (default: 90000000000000000000000)
#   CLEANUP_PENDING — if "true" (default), cancel PENDING commits in DB (DELETE API)
#   CLEANUP_ON_CHAIN — if "true" (default), cancel PENDING LCR slots via cast (requires cast)
#   HUB_RPC_URL, LCR_ADDRESS — override; otherwise read from backend/config/.env.infra.central-bank-a
#   POLL_BRIDGE_SECS, POLL_POOL_SECS — polling timeouts

set -uo pipefail

CB_A_URL="${CB_A_URL:-http://localhost:38080}"
CB_B_URL="${CB_B_URL:-http://localhost:60080}"
CB_A_ENV="${CB_A_ENV:-backend/config/.env.infra.central-bank-a}"
CB_B_ENV="${CB_B_ENV:-backend/config/.env.infra.central-bank-b}"
POOL_PAIR="${POOL_PAIR:-W-BRL-ARS}"

# 90_000 and 70_000 tokens (18 decimals)
LOCK_MINT_A="${LOCK_MINT_A:-90000000000000000000000}"
LOCK_MINT_B="${LOCK_MINT_B:-70000000000000000000000}"
COMMIT_AMOUNT="${COMMIT_AMOUNT:-90000000000000000000000}"

CLEANUP_PENDING="${CLEANUP_PENDING:-true}"
CLEANUP_ON_CHAIN="${CLEANUP_ON_CHAIN:-true}"
HUB_RPC_URL="${HUB_RPC_URL:-http://localhost:8645}"
POLL_BRIDGE_SECS="${POLL_BRIDGE_SECS:-120}"
POLL_POOL_SECS="${POLL_POOL_SECS:-45}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}✓ $*${NC}"; }
info() { echo -e "${YELLOW}  $*${NC}" >&2; }
fail() { echo -e "${RED}✗ $*${NC}"; exit 1; }
warn() { echo -e "${YELLOW}⚠ $*${NC}"; }

# Extract JSON object from response (ignore accidental log lines).
json_field() {
  local json="$1" field="$2"
  echo "$json" | awk '/^\{/ { line=$0 } END { print line }' | jq -r "$field" 2>/dev/null || true
}
sep()  { echo "────────────────────────────────────────────────────────"; }

_HTTP_CODE_FILE=$(mktemp)
trap 'rm -f "${_HTTP_CODE_FILE}"' EXIT

_http_code() { cat "${_HTTP_CODE_FILE}" 2>/dev/null || echo "?"; }

require_cmd() { command -v "$1" &>/dev/null || fail "Required command missing: $1"; }
require_cmd curl
require_cmd jq

load_env() {
  local envfile="$1"
  if [[ -f "$envfile" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$envfile"
    set +a
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
  local response http_code
  response=$(curl -s -S -w '\n__HTTP_CODE__%{http_code}' \
    -X POST "${url}" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "${body}" 2>/dev/null || echo "__HTTP_CODE__000")
  http_code=$(echo "$response" | grep '__HTTP_CODE__' | sed 's/__HTTP_CODE__//')
  response=$(echo "$response" | grep -v '__HTTP_CODE__')
  echo "$http_code" > "${_HTTP_CODE_FILE}"
  echo "$response"
}

api_get() {
  local url="$1" token="$2"
  curl -sf "${url}" -H "Authorization: Bearer ${token}"
}

poll_bridge_active() {
  local base_url="$1" token="$2" label="$3"
  local elapsed=0 interval=5
  info "${label}: waiting for bridge_state=ACTIVE (up to ${POLL_BRIDGE_SECS}s)…"
  while (( elapsed < POLL_BRIDGE_SECS )); do
    local st
    st=$(api_get "${base_url}/api/v2/bridge/positions?state=ACTIVE" "$token" 2>/dev/null \
      | jq -r '.positions[0].bridge_state // ""' || true)
    if [[ "$st" == "ACTIVE" ]]; then
      pass "${label}: bridge ACTIVE (${elapsed}s)"
      return 0
    fi
    sleep "$interval"
    elapsed=$((elapsed + interval))
  done
  fail "${label}: bridge did not become ACTIVE within ${POLL_BRIDGE_SECS}s"
}

lock_mint() {
  local base_url="$1" token="$2" amount="$3" label="$4"
  info "${label}: POST /api/v2/bridge/lock-mint amount=${amount}"
  local body resp pos_id code
  body=$(jq -nc --arg amt "$amount" '{amount: $amt}')
  resp=$(api_post "${base_url}/api/v2/bridge/lock-mint" "$token" "$body")
  code=$(_http_code)
  if [[ "$code" != "200" && "$code" != "201" ]]; then
    fail "${label}: lock-mint HTTP ${code}: $(echo "$resp" | jq -c . 2>/dev/null || echo "$resp")"
  fi
  pos_id=$(echo "$resp" | jq -r '.position_id // empty')
  [[ -n "$pos_id" ]] || fail "${label}: lock-mint missing position_id"
  pass "${label}: lock-mint OK position_id=${pos_id}"
}

commit_liquidity() {
  local base_url="$1" token="$2" amount="$3" label="$4"
  info "${label}: POST /api/v2/amm/liquidity/commit amount=${amount}"
  local body
  body=$(jq -nc --arg pair "$POOL_PAIR" --arg amt "$amount" '{pool_pair: $pair, amount: $amt}')
  api_post "${base_url}/api/v2/amm/liquidity/commit" "$token" "$body"
}

cancel_pending_commits() {
  local base_url="$1" token="$2" label="$3"
  info "${label}: clearing PENDING commits (DB) for ${POOL_PAIR}…"
  local list
  list=$(api_get "${base_url}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}&status=PENDING" "$token" 2>/dev/null || echo '{}')
  echo "$list" | jq -r '.commits[]? | [.CommitID // .commit_id // "", .ProviderID // .provider_id // ""] | @tsv' \
  | while IFS=$'\t' read -r cid provider; do
    [[ -z "$cid" || -z "$provider" ]] && continue
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
      "${base_url}/api/v2/amm/liquidity/commits/${cid}?provider_id=${provider}" \
      -H "Authorization: Bearer ${token}" || echo "000")
    info "${label}: DELETE commit ${cid} (provider=${provider}) → HTTP ${code}"
  done
}

read_pool_reserves() {
  local base_url="$1" token="$2"
  api_get "${base_url}/api/v2/amm/pool/${POOL_PAIR}/status" "$token" 2>/dev/null \
    | jq -r '[.reserve_a // "0", .reserve_b // "0", .pool_status // "UNKNOWN"] | @tsv' || echo "0	0	UNKNOWN"
}

human_tokens() {
  # Display approximate whole tokens (18 decimals) for logs.
  local wei="$1"
  python3 -c "w=int('${wei}'); print(f'{w/10**18:.0f} tokens (wei={w})')" 2>/dev/null \
    || echo "${wei} wei"
}

env_var_from_file() {
  local file="$1" key="$2"
  grep -m1 "^${key}=" "$file" 2>/dev/null | cut -d= -f2- || true
}

is_zero_bytes32() {
  local v="${1,,}"
  [[ -z "$v" || "$v" == "0x0000000000000000000000000000000000000000000000000000000000000000" || "$v" == "0x0" ]]
}

# Cancel PENDING slot on LiquidityCommitRegistry (on-chain). Required between tryout runs —
# DELETE on API only marks EXPIRED in DB, does not free the on-chain slot.
cancel_on_chain_lcr_side() {
  local lcr="$1" side="$2" private_key="$3" label="$4"
  [[ -n "$private_key" ]] || { warn "${label}: private key missing — skipping on-chain cleanup"; return 0; }
  local pending
  pending=$(cast call "$lcr" "getPendingCommit(string,uint8)(bytes32)" "$POOL_PAIR" "$side" \
    --rpc-url "$HUB_RPC_URL" 2>/dev/null || true)
  if is_zero_bytes32 "$pending"; then
    info "${label}: LCR slot side=${side} free"
    return 0
  fi
  info "${label}: cancelCommit on-chain ${pending} (side=${side})…"
  if cast send "$lcr" "cancelCommit(bytes32)" "$pending" \
    --private-key "$private_key" --rpc-url "$HUB_RPC_URL" >/dev/null 2>&1; then
    pass "${label}: slot side=${side} freed on-chain"
  else
    warn "${label}: cancelCommit failed for ${pending} — on-chain commit may block the test"
  fi
}

cleanup_on_chain_lcr() {
  command -v cast &>/dev/null || {
    warn "cast not found — skipping on-chain cleanup (export CLEANUP_ON_CHAIN=false to suppress)"
    return 0
  }
  local lcr="${LCR_ADDRESS:-$(env_var_from_file "$CB_A_ENV" LIQUIDITY_COMMIT_REGISTRY_ADDRESS)}"
  [[ -n "$lcr" ]] || { warn "LIQUIDITY_COMMIT_REGISTRY_ADDRESS not set — skipping on-chain cleanup"; return 0; }
  local pk_a pk_b
  pk_a=$(env_var_from_file "$CB_A_ENV" CB_PRIVATE_KEY)
  pk_b=$(env_var_from_file "$CB_B_ENV" CB_PRIVATE_KEY)
  info "LCR=${lcr} hub_rpc=${HUB_RPC_URL}"
  cancel_on_chain_lcr_side "$lcr" 0 "$pk_a" "CB-A"
  cancel_on_chain_lcr_side "$lcr" 1 "$pk_b" "CB-B"
}

# ─── Start ───────────────────────────────────────────────────────────────────
sep
echo "Tryout: commit amount greater than lock-mint"
sep
info "Pair: ${POOL_PAIR}"
info "CB-A lock-mint:  $(human_tokens "$LOCK_MINT_A")"
info "CB-B lock-mint:  $(human_tokens "$LOCK_MINT_B")"
info "Commit (both):   $(human_tokens "$COMMIT_AMOUNT")"
sep

load_env "$CB_A_ENV"
KC_CLIENT_A="${KC_CLIENT_ID:?KC_CLIENT_ID missing in ${CB_A_ENV}}"
KC_SECRET_A="${KC_CLIENT_SECRET:?KC_CLIENT_SECRET missing in ${CB_A_ENV}}"
CB_A_BANK_ID="${BANK_CODE:-central-bank-a}"
TOKEN_A=$(cb_api_token "${CB_A_URL}" "${KC_CLIENT_A}" "${KC_SECRET_A}")
[[ -n "$TOKEN_A" ]] || fail "CB-A authentication failed"

load_env "$CB_B_ENV"
KC_CLIENT_B="${KC_CLIENT_ID:?KC_CLIENT_ID missing in ${CB_B_ENV}}"
KC_SECRET_B="${KC_CLIENT_SECRET:?KC_CLIENT_SECRET missing in ${CB_B_ENV}}"
CB_B_BANK_ID="${BANK_CODE:-central-bank-b}"
TOKEN_B=$(cb_api_token "${CB_B_URL}" "${KC_CLIENT_B}" "${KC_SECRET_B}")
[[ -n "$TOKEN_B" ]] || fail "CB-B authentication failed"
pass "CB-A and CB-B authenticated"

sep
echo "Pre-cleanup (previous runs)"
if [[ "$CLEANUP_ON_CHAIN" == "true" ]]; then
  cleanup_on_chain_lcr
fi
if [[ "$CLEANUP_PENDING" == "true" ]]; then
  cancel_pending_commits "$CB_A_URL" "$TOKEN_A" "CB-A"
  cancel_pending_commits "$CB_B_URL" "$TOKEN_B" "CB-B"
fi

# ─── Phase 1: CB-A lock-mint + commit (happy path) ─────────────────────────────
sep
echo "Phase 1 — CB-A: lock-mint and commit with same amount"
lock_mint "$CB_A_URL" "$TOKEN_A" "$LOCK_MINT_A" "CB-A"
poll_bridge_active "$CB_A_URL" "$TOKEN_A" "CB-A"

RESP_A=$(commit_liquidity "$CB_A_URL" "$TOKEN_A" "$COMMIT_AMOUNT" "CB-A")
CODE_A=$(_http_code)
if [[ "$CODE_A" != "200" && "$CODE_A" != "201" ]]; then
  err_code=$(json_field "$RESP_A" '.code // .error_code // empty')
  if [[ "$err_code" == "ON_CHAIN_COMMIT_FAILED" ]]; then
    fail "CB-A commit failed on-chain (HTTP ${CODE_A}). Likely PENDING commit on LCR — run with CLEANUP_ON_CHAIN=true and cast installed, or cancel manually on contract. Response: $(json_field "$RESP_A" '.error // .')"
  fi
  fail "CB-A commit should accept (HTTP ${CODE_A}): $(echo "$RESP_A" | awk '/^\{/{print}' | jq -c . 2>/dev/null || echo "$RESP_A")"
fi
COMMIT_A_ID=$(json_field "$RESP_A" '.commit_id // .CommitID // empty')
[[ -n "$COMMIT_A_ID" ]] || fail "CB-A commit HTTP ${CODE_A} missing commit_id: $(echo "$RESP_A" | tr -d '\n')"
pass "CB-A commit accepted commit_id=${COMMIT_A_ID} (HTTP ${CODE_A})"

# ─── Phase 2: CB-B smaller lock-mint + larger commit (should fail) ───────────────
sep
echo "Phase 2 — CB-B: lock-mint smaller than commit amount"
lock_mint "$CB_B_URL" "$TOKEN_B" "$LOCK_MINT_B" "CB-B"
poll_bridge_active "$CB_B_URL" "$TOKEN_B" "CB-B"

RESP_B=$(commit_liquidity "$CB_B_URL" "$TOKEN_B" "$COMMIT_AMOUNT" "CB-B")
CODE_B=$(_http_code)
ERR_CODE=$(json_field "$RESP_B" '.code // .error_code // empty')

TEST_OK=0
if [[ "$CODE_B" == "422" && "$ERR_CODE" == "INSUFFICIENT_BALANCE" ]]; then
  pass "CB-B commit rejected as expected (HTTP 422, code=INSUFFICIENT_BALANCE)"
  TEST_OK=1
elif [[ "$CODE_B" == "200" || "$CODE_B" == "201" ]]; then
  warn "BUG REPRODUCED: CB-B commit was ACCEPTED (HTTP ${CODE_B}) despite smaller lock-mint"
  warn "Response: $(echo "$RESP_B" | jq -c . 2>/dev/null || echo "$RESP_B")"
  TEST_OK=0
else
  warn "Unexpected CB-B commit response: HTTP ${CODE_B} code=${ERR_CODE:-?}"
  warn "Body: $(echo "$RESP_B" | jq -c . 2>/dev/null || echo "$RESP_B")"
  TEST_OK=0
fi

# ─── Phase 3: pool must not become asymmetric (A in pool, B zero) ────────────
sep
echo "Phase 3 — Checking pool reserves (waiting up to ${POLL_POOL_SECS}s)"
sleep 5
elapsed=0
RESERVE_A="0"
RESERVE_B="0"
POOL_STATUS="UNKNOWN"
while (( elapsed <= POLL_POOL_SECS )); do
  read -r RESERVE_A RESERVE_B POOL_STATUS < <(read_pool_reserves "$CB_A_URL" "$TOKEN_A")
  info "pool_status=${POOL_STATUS} reserve_a=${RESERVE_A} reserve_b=${RESERVE_B} (+${elapsed}s)"
  if [[ "$RESERVE_A" != "0" && "$RESERVE_B" == "0" ]]; then
    break
  fi
  if [[ "$RESERVE_A" != "0" && "$RESERVE_B" != "0" ]]; then
    break
  fi
  sleep 5
  elapsed=$((elapsed + 5))
done

sep
echo "Result"
if [[ "$TEST_OK" -eq 1 ]]; then
  if [[ "$RESERVE_A" != "0" && "$RESERVE_B" == "0" ]]; then
    warn "Commit validation OK, but pool is asymmetric (reserve_a=${RESERVE_A}, reserve_b=0)"
    warn "May be from a previous run — check EXECUTED / RECONCILIATION_REQUIRED commits"
    exit 1
  fi
  pass "Test complete: CB-B commit blocked before match; no partial pool deposit detected"
  info "reserve_a=${RESERVE_A} reserve_b=${RESERVE_B} pool_status=${POOL_STATUS}"
  exit 0
else
  if [[ "$RESERVE_A" != "0" && "$RESERVE_B" == "0" ]]; then
    fail "Asymmetric pool confirmed (reserve_a=${RESERVE_A}, reserve_b=0) — same symptom as original bug"
  fi
  fail "Test failed: expected HTTP 422 INSUFFICIENT_BALANCE on CB-B commit"
fi
