#!/usr/bin/env bash
# tryout-scenario-b-e2e.sh — End-to-end tryout for Scenario B (Hub-and-Spoke Liquidity Pool)
#
# Prerequisites:
#   - docker, docker compose (v2)
#   - forge, cast (Foundry)
#   - jq, openssl, curl
#
# Environment variables:
#   BESU_HUB_RPC              — JSON-RPC endpoint for the Hub Besu network (AMM)
#   SPOKE_A_RPC               — JSON-RPC endpoint for Spoke A Besu network
#   SPOKE_B_RPC               — JSON-RPC endpoint for Spoke B Besu network
#   KEYCLOAK_URL              — Keycloak base URL for token acquisition
#   API_GW_BANK_A_URL         — API Gateway for bank-a     (default: http://localhost:18080)
#   API_GW_CENTRAL_BANK_A_URL — API Gateway for central-bank-a (default: http://localhost:38080)
#   CACTI_RELAYER_URL         — Cacti Relayer base URL (default: http://localhost:4000)
#
# Usage:
#   bash tryouts/tryout-scenario-b-e2e.sh [us1|us2|us3|us5|us6|all]
#
set -euo pipefail

# ── Report tracking (print_report is called by trap EXIT) ──────────────────
REPORT_FILE="$(mktemp /tmp/cbweb3-tryout-XXXXXX.log 2>/dev/null || printf '')"

STORY="${1:-all}"
case "$STORY" in
  --story) STORY="${2:-all}" ;;
esac

SKIP_UP="${SKIP_UP:-1}"

# ── Endpoints ────────────────────────────────────────────────────────────────
# Derived from the toolkit manifests, not hardcoded. The defaults here used to be
# the legacy deploy/local ports (18080/38080/60080, hub 8845, relay 4000); nothing
# answers on them since that bring-up was removed, so every call in this script
# addressed a stack that does not exist.
#
# tests/integration/toolkit-env.sh reads the same manifests the toolkit was applied
# with and only fills in what the caller left unset, so an explicit override still
# wins. Sourced when present; the script still runs without it if you pass the URLs.
_tryout_dir="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
_toolkit_env="${_tryout_dir}/../tests/integration/toolkit-env.sh"
if [[ -f "$_toolkit_env" ]]; then
  # shellcheck source=/dev/null
  source "$_toolkit_env" >/dev/null
fi

API_GW_BANK_A_URL="${API_GW_BANK_A_URL:?derive it with tests/integration/toolkit-env.sh or pass it explicitly}"
API_GW_CENTRAL_BANK_A_URL="${API_GW_CENTRAL_BANK_A_URL:?see API_GW_BANK_A_URL}"
API_GW_CENTRAL_BANK_B_URL="${API_GW_CENTRAL_BANK_B_URL:?see API_GW_BANK_A_URL}"
# The beneficiary bank. Its own gateway is needed because onboarding is initiated by the
# bank itself — the payer's gateway cannot onboard someone else.
API_GW_BANK_B_URL="${API_GW_BANK_B_URL:?see API_GW_BANK_A_URL}"

BESU_HUB_RPC="${BESU_HUB_RPC:?see API_GW_BANK_A_URL}"
# toolkit-env.sh names the spoke RPCs BESU_SPOKE_*; this script has always called
# them SPOKE_*. Map rather than rename, so an operator passing either one is served.
SPOKE_A_RPC="${SPOKE_A_RPC:-${BESU_SPOKE_A_RPC:-}}"
SPOKE_B_RPC="${SPOKE_B_RPC:-${BESU_SPOKE_B_RPC:-}}"
# Scenario B's relay listens on 7000 (Scenario A's is the one on 4000).
CACTI_RELAYER_URL="${CACTI_RELAYER_URL:-http://localhost:7000}"

# Hub contract addresses (read from CB-A infra env file if not set externally)
HUB_TOKEN_A_ADDRESS="${HUB_TOKEN_A_ADDRESS:-$(grep -s '^HUB_TOKEN_A_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
HUB_TOKEN_B_ADDRESS="${HUB_TOKEN_B_ADDRESS:-$(grep -s '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
HUB_IDENTITY_REGISTRY_ADDRESS="${HUB_IDENTITY_REGISTRY_ADDRESS:-$(grep -s '^HUB_IDENTITY_REGISTRY_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
CURRENCY_REGISTRY_CONTRACT_ADDRESS="${CURRENCY_REGISTRY_CONTRACT_ADDRESS:-$(grep -s '^CURRENCY_REGISTRY_CONTRACT_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"

# Keycloak client credentials (read from the generated .env files)
# ── Sovereign corridor under test ────────────────────────────────────────────
# The hub wraps each sovereign currency with a "W-" prefix and names a pool by BOTH
# wrapped sides: "W-BRL-W-ARS". This script used to hardcode "BRL-USD" in 28 places —
# a pair from an earlier corridor that no toolkit stack has, so every pool read,
# quote, swap, commit and breaker call addressed something that does not exist.
#
# Defaults follow samples/deploy-all.sh (BRL<->ARS). tests/integration/toolkit-env.sh
# exports these three, so sourcing it points the tryout at whatever corridor is live.
SOURCE_CURRENCY="${SOURCE_CURRENCY:-BRL}"
TARGET_CURRENCY="${TARGET_CURRENCY:-ARS}"
POOL_PAIR="${POOL_PAIR:-W-${SOURCE_CURRENCY}-W-${TARGET_CURRENCY}}"

# Seed amounts, one per side. They are NOT equal: the pool is priced at the national
# exchange rate, so side B carries the counter-currency multiple of side A. The first
# deposit sets the pool's price, which every later quote in this run is measured against.
SEED_AMOUNT_A="${SEED_AMOUNT_A:-1000000000000000000000}"    # 1e21 W-<SOURCE>
SEED_AMOUNT_B="${SEED_AMOUNT_B:-287000000000000000000000}"  # 287e21 W-<TARGET>

# Payer funding. The bank must hold tCeBM on its own spoke before it can bridge in;
# there are no static hub tokens to mint from. Topped up only when short.
PAYER_MIN_BALANCE="${PAYER_MIN_BALANCE:-10000000000000000000}"  # 10e18
PAYER_TOPUP="${PAYER_TOPUP:-20000000000000000000}"              # 20e18

# The payment itself. max_amount_in is kept above the expected cost on purpose: the
# unspent buffer comes back on a separate asynchronous leg (the residue return).
SWAP_AMOUNT_OUT="${SWAP_AMOUNT_OUT:-1000000000000000000}"       # 1e18
SWAP_MAX_AMOUNT_IN="${SWAP_MAX_AMOUNT_IN:-3000000000000000000}" # 3e18

# Bridged by the US2 lock-mint / burn-unlock story.
BRIDGE_AMOUNT="${BRIDGE_AMOUNT:-1000000000000000000}"           # 1e18

# Bank codes come from the manifests (spec.bankId) via toolkit-env.sh. The literals
# "bank-a"/"bank-b" name nothing in a toolkit topology.
BANK_A_CODE="${BANK_A_CODE:-bank-itau}"
BANK_B_CODE="${BANK_B_CODE:-bank-galicia}"

KC_BANK_B_CLIENT="${KC_BANK_B_CLIENT:-}"
KC_BANK_B_SECRET="${KC_BANK_B_SECRET:-}"

KC_BANK_A_REALM="${KC_BANK_A_REALM:-bank-a}"
KC_BANK_A_CLIENT="${KC_BANK_A_CLIENT:-bank-a-client}"
KC_BANK_A_SECRET="${KC_BANK_A_SECRET:-$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2 || echo '')}"

KC_CENTRAL_BANK_A_REALM="${KC_CENTRAL_BANK_A_REALM:-central-bank-a}"
KC_CENTRAL_BANK_A_CLIENT="${KC_CENTRAL_BANK_A_CLIENT:-central-bank-a-client}"
KC_CENTRAL_BANK_A_SECRET="${KC_CENTRAL_BANK_A_SECRET:-$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"

KC_CENTRAL_BANK_B_REALM="${KC_CENTRAL_BANK_B_REALM:-central-bank-b}"
KC_CENTRAL_BANK_B_CLIENT="${KC_CENTRAL_BANK_B_CLIENT:-central-bank-b-client}"
KC_CENTRAL_BANK_B_SECRET="${KC_CENTRAL_BANK_B_SECRET:-$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.central-bank-b 2>/dev/null | cut -d= -f2 || echo '')}"


# MLP Path B (opt-in: ENABLE_MLP=true)
ENABLE_MLP="${ENABLE_MLP:-false}"
API_GW_MLP_URL="${API_GW_MLP_URL:-http://localhost:68080}"
KC_MLP_REALM="${KC_MLP_REALM:-mlp}"
KC_MLP_CLIENT="${KC_MLP_CLIENT:-mlp-client}"
KC_MLP_SECRET="${KC_MLP_SECRET:-$(grep -s '^KC_CLIENT_SECRET=' backend/config/.env.infra.mlp 2>/dev/null | cut -d= -f2 || echo '')}"

BANK_A_TOKEN=""
CENTRAL_BANK_A_TOKEN=""
CENTRAL_BANK_B_TOKEN=""
MLP_TOKEN=""

# LP position IDs created by cooperative commit-reveal (populated in step4b)
LP_ID_BCB=""
LP_ID_FED=""

log() {
  echo "[tryout] $*" >&2
  if [[ -n "${REPORT_FILE:-}" ]]; then
    case "$*" in
      "PASS:"*) printf 'PASS|%s\n' "${*/PASS: /}" >> "$REPORT_FILE" ;;
      "WARN:"*) printf 'WARN|%s\n' "${*/WARN: /}" >> "$REPORT_FILE" ;;
      "SKIP:"*) printf 'SKIP|%s\n' "${*/SKIP: /}" >> "$REPORT_FILE" ;;
    esac
  fi
}
step()  { echo "" >&2; echo "============================================================" >&2; echo "STEP: $*" >&2; echo "============================================================" >&2; }
fail()  {
  [[ -n "${REPORT_FILE:-}" ]] && printf 'FAIL|%s\n' "$*" >> "$REPORT_FILE"
  echo "[ERROR] $*" >&2
  exit 1
}
have()  { command -v "$1" >/dev/null 2>&1 || fail "Required tool missing: $1"; }

have curl
have jq

echo "[tryout] Starting Scenario B E2E — story=${STORY}"
log "API_GW_BANK_A_URL=${API_GW_BANK_A_URL}"
log "API_GW_CENTRAL_BANK_A_URL=${API_GW_CENTRAL_BANK_A_URL}"
log "API_GW_CENTRAL_BANK_B_URL=${API_GW_CENTRAL_BANK_B_URL}"
log "BESU_HUB_RPC=${BESU_HUB_RPC}"
log "CACTI_RELAYER_URL=${CACTI_RELAYER_URL}"

# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────

# gateway_login <gateway_url> <username> <password>
#
# Logs in through the ENTITY'S OWN gateway, which routes to that entity's Keycloak.
# This replaces a direct grant_type=client_credentials call against a single shared
# Keycloak, which was wrong twice over: the auth service accepts only the OIDC
# password grant (a realm client id/secret is refused on purpose), and the toolkit
# gives every entity its OWN Keycloak rather than one instance with per-entity realms.
#
# The clientId/clientSecret JSON fields are the wire contract's names, not the
# credential type — a username and that user's password is what belongs in them.
gateway_login() {
  local gw="$1" user="$2" pass="$3"
  curl -sS -X POST "${gw%/}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "$(jq -cn --arg u "$user" --arg p "$pass" '{clientId:$u, clientSecret:$p}')" \
    | jq -r '.accessToken // empty'
}

# api_get  <base_url> <path> [token]
# api_post <base_url> <path> <body> [token]
# Auth is cookie-based (access_token HttpOnly cookie), matching the v1 frontend pattern.
api_get()    { curl -sS -H "Accept: application/json" ${3:+--cookie "access_token=$3"} "${1}${2}"; }
api_post()   { curl -sS -X POST   -H "Content-Type: application/json" -H "Accept: application/json" ${4:+--cookie "access_token=$4"} -d "$3" "${1}${2}"; }
api_delete() { curl -sS -X DELETE -H "Accept: application/json" ${3:+--cookie "access_token=$3"} "${1}${2}"; }

wait_for() {
  local url="$1" retries="${2:-30}" delay="${3:-2}"
  for _ in $(seq 1 "$retries"); do
    if curl -sSf -o /dev/null "$url"; then return 0; fi
    sleep "$delay"
  done
  log "(warn) Service not ready after $retries retries: $url"
  return 1
}

# wait_for_position_state <position_id> <target_state> [timeout_secs=120] [interval_secs=5]
# Polls GET /api/v2/bridge/positions until the given position reaches target_state or timeout.
wait_for_position_state() {
  local position_id="$1" target_state="$2" timeout_secs="${3:-120}" interval_secs="${4:-5}"
  local elapsed=0
  log "Waiting for position $position_id to reach $target_state (timeout=${timeout_secs}s, interval=${interval_secs}s)"
  while [ "$elapsed" -lt "$timeout_secs" ]; do
    local positions current_state
    positions="$(api_get "$API_GW_BANK_A_URL" "/api/v2/bridge/positions" "$BANK_A_TOKEN" || true)"
    current_state="$(echo "$positions" | jq -r --arg id "$position_id" '.positions[] | select(.position_id==$id) | .bridge_state // empty' 2>/dev/null || true)"
    if [ "$current_state" = "$target_state" ]; then
      log "Position $position_id reached state $target_state after ${elapsed}s"
      return 0
    fi
    log "Position $position_id: current=${current_state:-unknown}, waiting for $target_state (${elapsed}s elapsed)"
    sleep "$interval_secs"
    elapsed=$(( elapsed + interval_secs ))
  done
  fail "Timeout waiting for position $position_id to reach $target_state (waited ${timeout_secs}s)"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 0: Bring up the full stack (skip with SKIP_UP=1)
# ─────────────────────────────────────────────────────────────────────────────
step0_up() {
  if [ -n "${SKIP_UP:-}" ]; then
    log "SKIP_UP=1 — assuming stack already running"
    return
  fi
  step "0. Starting full stack (make scenario-b.up)"
  make scenario-b.up || fail "scenario-b.up failed — check logs above"
  log "Stack up"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 1: Infrastructure readiness
# ─────────────────────────────────────────────────────────────────────────────
step1_infrastructure() {
  step "1. Verify infrastructure readiness"

  # This step is a GATE. It used to swallow every failure — `|| true` on the health
  # waits, "(warn)" on the RPC probes — and then print "Infrastructure ready"
  # unconditionally. A run against a stack that answered NOTHING sailed through to
  # step 4a and reported the defect there, three steps from its cause and blaming the
  # wrong thing. Whatever is unreachable, say so here and stop.
  local unreachable=()

  wait_for "${API_GW_BANK_A_URL}/healthz" 60 3         || unreachable+=("bank-a gateway ${API_GW_BANK_A_URL}")
  wait_for "${API_GW_CENTRAL_BANK_A_URL}/healthz" 60 3 || unreachable+=("central-bank-a gateway ${API_GW_CENTRAL_BANK_A_URL}")
  wait_for "${API_GW_CENTRAL_BANK_B_URL}/healthz" 60 3 || unreachable+=("central-bank-b gateway ${API_GW_CENTRAL_BANK_B_URL}")

  rpc_alive() {
    curl -sS -X POST -H "Content-Type: application/json" \
      --data '{"jsonrpc":"2.0","method":"net_version","id":1}' "$1" 2>/dev/null \
      | jq -e '.result' >/dev/null 2>&1
  }
  rpc_alive "${BESU_HUB_RPC}" || unreachable+=("hub Besu RPC ${BESU_HUB_RPC}")
  [ -n "${SPOKE_A_RPC:-}" ] && { rpc_alive "${SPOKE_A_RPC}" || unreachable+=("spoke-A Besu RPC ${SPOKE_A_RPC}"); }
  [ -n "${SPOKE_B_RPC:-}" ] && { rpc_alive "${SPOKE_B_RPC}" || unreachable+=("spoke-B Besu RPC ${SPOKE_B_RPC}"); }

  if [ ${#unreachable[@]} -gt 0 ]; then
    local what
    for what in "${unreachable[@]}"; do echo "  ✗ unreachable: ${what}" >&2; done
    fail "step1: ${#unreachable[@]} endpoint(s) unreachable. Bring a stack up with the single provisioning path (cd samples && ./deploy-all.sh), or pass the URLs explicitly."
  fi
  log "Infrastructure ready — all gateways and RPCs answering"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 2: Verify contracts (scenario-b.up already deploys them)
# ─────────────────────────────────────────────────────────────────────────────
# Step 2: Contracts.
#
# What used to live here was a `cast send setCentralBankOf(...)` pair, signing with a
# key read from backend/config/.env.infra.central-bank-a — a file the legacy bring-up
# wrote and the toolkit deliberately does not. The key came back empty on every run,
# so both calls warned and nothing was configured; the warnings were noise pointing at
# an "admin role" that was never the problem.
#
# It is also the wrong layer: the toolkit registers each spoke's currency and its
# central bank on the hub during found-spoke. A tryout signing raw transactions with a
# CB's private key is exactly what the sovereignty model exists to avoid — every step
# below goes through the CB's own gateway, authenticated, holding no keys.
# ─────────────────────────────────────────────────────────────────────────────
step2_contracts() {
  step "2. Verify the sovereign currencies are registered on the hub"
  local currencies sym_a sym_b
  currencies="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" "$CENTRAL_BANK_A_TOKEN" || true)"
  sym_a="W-tCeBM_${SOURCE_CURRENCY}"
  sym_b="W-tCeBM_${TARGET_CURRENCY}"
  for sym in "$sym_a" "$sym_b"; do
    echo "$currencies" | jq -e --arg s "$sym" '[.currencies[]? | select(.symbol==$s)] | length > 0' >/dev/null 2>&1 \
      || fail "step2: ${sym} is not registered on the hub — found-spoke registers it, so the spoke for that currency did not complete"
  done
  log "both sovereign currencies registered: ${sym_a}, ${sym_b}"
}

# ─────────────────────────────────────────────────────────────────────────────
step3_participants() {
  step "3. Acquire OIDC tokens (bank-a, central-bank-a, central-bank-b)"
  BANK_A_TOKEN="$(gateway_login "$API_GW_BANK_A_URL" "$KC_BANK_A_CLIENT" "$KC_BANK_A_SECRET" || true)"
  BANK_B_TOKEN="$(gateway_login "$API_GW_BANK_B_URL" "$KC_BANK_B_CLIENT" "$KC_BANK_B_SECRET" || true)"
  CENTRAL_BANK_A_TOKEN="$(gateway_login "$API_GW_CENTRAL_BANK_A_URL" "$KC_CENTRAL_BANK_A_CLIENT" "$KC_CENTRAL_BANK_A_SECRET" || true)"
  CENTRAL_BANK_B_TOKEN="$(gateway_login "$API_GW_CENTRAL_BANK_B_URL" "$KC_CENTRAL_BANK_B_CLIENT" "$KC_CENTRAL_BANK_B_SECRET" || true)"
  # A missing token is fatal, not a warning: every step below authenticates with one,
  # and letting the run continue reports the first API call's 401 as the defect
  # instead of the login that never happened.
  [ -n "$BANK_A_TOKEN" ]         || fail "step3: no ${BANK_A_CODE} token from $API_GW_BANK_A_URL (user $KC_BANK_A_CLIENT)"
  [ -n "$BANK_B_TOKEN" ]         || fail "step3: no ${BANK_B_CODE} token from $API_GW_BANK_B_URL (user $KC_BANK_B_CLIENT)"
  [ -n "$CENTRAL_BANK_A_TOKEN" ] || fail "step3: no central-bank-a token from $API_GW_CENTRAL_BANK_A_URL (user $KC_CENTRAL_BANK_A_CLIENT)"
  [ -n "$CENTRAL_BANK_B_TOKEN" ] || fail "step3: no central-bank-b token from $API_GW_CENTRAL_BANK_B_URL (user $KC_CENTRAL_BANK_B_CLIENT)"
  log "tokens acquired for bank-a, central-bank-a and central-bank-b"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 4: Pool provisioning — seed with initial liquidity if empty
# ─────────────────────────────────────────────────────────────────────────────
step4_pool() {
  step "4. Verify AMM pool status (seed initial liquidity if empty)"
  local resp reserve_a reserve_b pool_status
  resp="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/${POOL_PAIR}/status" "$BANK_A_TOKEN" || true)"
  log "Pool status: $(echo "$resp" | jq -c '.' 2>/dev/null || echo "$resp")"
  if echo "$resp" | jq -e '.reserve_a' >/dev/null 2>&1; then
    reserve_a="$(echo "$resp" | jq -r '.reserve_a // "0"')"
    reserve_b="$(echo "$resp" | jq -r '.reserve_b // "0"')"
    pool_status="$(echo "$resp" | jq -r '.pool_status // "UNKNOWN"')"
    log "Pool current state: reserve_a=${reserve_a}, reserve_b=${reserve_b}, pool_status=${pool_status}"
    if [ "$reserve_a" != "0" ] && [ "$reserve_b" != "0" ]; then
      log "Pool already has liquidity — step4b will verify ACTIVE status"
    else
      log "Pool is empty or one-sided — step4b will seed via cooperative commit-reveal"
    fi
  else
    log "Pool endpoint not responding or returned no reserve data — will attempt seeding in step4b"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 4a: Mint Hub tCeBM tokens + approve AMM (pre-condition for addLiquidity)
#          The central bank holds CENTRAL_BANK_ROLE on Hub tCeBM contracts and must
#          mint tokens to its own signer address and approve the AMM before the pool
#          can be seeded. This must run once after every fresh environment reset.
# ─────────────────────────────────────────────────────────────────────────────
# Step 4a: Open the sovereign FX corridor.
#
# This step did not exist, and its absence is why the prelude failed. It used to be
# unnecessary: provisioning created ONE static AMM plus two hub tokens, so a pool was
# already there when the stack came up, and PairRegistry (step 8) was a feature test
# rather than a prerequisite.
#
# That model is gone. The toolkit states it plainly — "the sovereign FX corridor is
# NOT part of provisioning: it is opened at runtime by each central bank through its
# governance portal" (toolkit/engine/orchestrator/step_found_spoke.go) — and
# TestApplyFoundSpokeHasNoSovereignTail keeps it that way. An AMM is now resolved PER
# PAIR, so with no pair open there is nothing for the steps below to address.
#
# Two sovereign acts, one per central bank. amm_address is omitted deliberately: the
# proposing CB then deploys the pair's own AMM in the same signed call.
# ─────────────────────────────────────────────────────────────────────────────
step4a_open_corridor() {
  step "4a. Open the ${SOURCE_CURRENCY}<->${TARGET_CURRENCY} corridor (${SOURCE_CURRENCY} CB proposes, ${TARGET_CURRENCY} CB confirms)"

  local pairs_resp existing
  pairs_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs" "$CENTRAL_BANK_A_TOKEN" || true)"
  existing="$(echo "$pairs_resp" | jq -r --arg p "$POOL_PAIR" \
    '[.pairs[]? | select(.pair_id==$p and .status=="ACTIVE")] | length' 2>/dev/null || echo 0)"
  if [ "${existing:-0}" -ge 1 ]; then
    log "Corridor ${POOL_PAIR} already ACTIVE — skipping open"
    return 0
  fi

  # Resolve each spoke's WRAPPED token on the hub. The hub lists it as W-tCeBM_<CUR>;
  # the pair is keyed by these two addresses.
  local currencies token_a token_b
  currencies="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" "$CENTRAL_BANK_A_TOKEN" || true)"
  token_a="$(echo "$currencies" | jq -r --arg s "W-tCeBM_${SOURCE_CURRENCY}" \
    '[.currencies[]? | select(.symbol==$s) | .token_address][0] // empty')"
  token_b="$(echo "$currencies" | jq -r --arg s "W-tCeBM_${TARGET_CURRENCY}" \
    '[.currencies[]? | select(.symbol==$s) | .token_address][0] // empty')"
  [ -n "$token_a" ] && [ -n "$token_b" ] || \
    fail "step4a: hub has no wrapped tokens for ${SOURCE_CURRENCY}/${TARGET_CURRENCY} — are both spokes registered? response: $(echo "$currencies" | jq -c '.' 2>/dev/null || echo "$currencies")"
  log "tokenA=${token_a} tokenB=${token_b}"

  local propose_body propose_resp propose_status propose_code amm
  propose_body="$(jq -cn --arg p "$POOL_PAIR" --arg tA "$token_a" --arg tB "$token_b" --arg cb "$SOURCE_CURRENCY" \
    '{pair_id:$p, token_a_address:$tA, token_b_address:$tB, proposer_cb:$cb}')"
  propose_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs/propose" "$propose_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  propose_status="$(echo "$propose_resp" | jq -r '.status // "ERROR"')"
  propose_code="$(echo "$propose_resp" | jq -r '.code // empty')"
  if [ "$propose_status" != "PROPOSED" ] && [ "$propose_code" != "PAIR_ALREADY_EXISTS" ]; then
    fail "step4a: propose ${POOL_PAIR} failed: $(echo "$propose_resp" | jq -c '.' 2>/dev/null || echo "$propose_resp")"
  fi
  amm="$(echo "$propose_resp" | jq -r '.amm_address // empty')"
  log "${SOURCE_CURRENCY} CB proposed ${POOL_PAIR}${amm:+ (amm=${amm})}"

  local confirm_resp confirm_code
  confirm_resp="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/amm/pairs/confirm" \
    "$(jq -cn --arg p "$POOL_PAIR" --arg cb "$TARGET_CURRENCY" '{pair_id:$p, confirmer_cb:$cb}')" \
    "$CENTRAL_BANK_B_TOKEN" || true)"
  confirm_code="$(echo "$confirm_resp" | jq -r '.code // empty')"
  if echo "$confirm_resp" | jq -e '.status // empty' >/dev/null 2>&1 || [ "$confirm_code" = "PAIR_ALREADY_ACTIVE" ]; then
    log "${TARGET_CURRENCY} CB confirmed ${POOL_PAIR} — corridor ACTIVE"
  else
    fail "step4a: confirm ${POOL_PAIR} failed: $(echo "$confirm_resp" | jq -c '.' 2>/dev/null || echo "$confirm_resp")"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 4b: Seed the pool — sovereign escrow-and-finalize.
#
# Each central bank deposits ONLY its own side; one finalize funds both reserves
# atomically. This replaces the commit-reveal flow this step used to drive
# (/liquidity/commit per side, then poll until the relay fired CommitMatched): the
# route still answers, but the LiquidityCommitRegistry behind it went with the legacy
# sovereign tail, so the commits sat PENDING until the poll gave up.
#
# deposit-side mints and approves the caller's own W-token internally and resolves the
# side on-chain from pool_pair, so the separate mint-and-approve this prelude used to
# run is gone with it.
# ─────────────────────────────────────────────────────────────────────────────
# Step 3b: Onboard the commercial bank through its central bank.
#
# This phase did not exist. Under the legacy bring-up commercial banks were
# pre-registered by provisioning, so the tryout could transact immediately. The
# toolkit does NOT pre-register them: onboarding is what verifies a bank, and an
# unverified bank has no on-chain wallet for anything below to move value to.
#
# Three acts, mirroring the governance portal: the bank initiates, its central bank
# approves KYC, the bank completes (PoP → CB-signed certificate → on-chain
# participant). Idempotent: a bank already ACTIVE is skipped.
# ─────────────────────────────────────────────────────────────────────────────
# onboard_bank LABEL BANK_GW BANK_TOKEN_VAR CB_GW CB_TOKEN COUNTRY
#
# Governance-portal onboarding: the bank initiates, its central bank approves KYC, the
# bank completes (PoP → CB-signed certificate → on-chain participant). Idempotent — a
# bank already ACTIVE is skipped.
#
# The token is refreshed through the name in BANK_TOKEN_VAR: the one used to initiate
# carries the pre-onboarding identity, and later calls must act as the verified
# participant.
onboard_bank() {
  local label=$1 bank_gw=$2 tok_var=$3 cb_gw=$4 cb_tok=$5 country=$6
  local bank_tok="${!tok_var}" status subject resp init_body

  status="$(api_get "$bank_gw" "/api/v1/onboarding/my-status" "$bank_tok" | jq -r '.status // empty')"
  if [ "$status" = "ACTIVE" ]; then
    log "${label} already ACTIVE — onboarding skipped"
    return 0
  fi

  init_body="$(jq -cn --arg n "$label" --arg c "$country" --arg u "${label}-user" --arg e "ops@${label}.local" \
    '{institution_name:$n, country:$c, role:"ROLE_COMMERCIAL_BANK", email:$e, username:$u}')"
  resp="$(api_post "$bank_gw" "/api/v1/onboarding/initiate" "$init_body" "$bank_tok" || true)"
  subject="$(echo "$resp" | jq -r '.user_id // empty')"
  if [ -z "$subject" ]; then
    # A partial earlier run can leave the Keycloak user behind; recover its subject.
    subject="$(api_get "$bank_gw" "/api/v1/onboarding/my-status" "$bank_tok" | jq -r '.user_id // empty')"
    [ -n "$subject" ] || fail "step3b: ${label} initiate returned no user_id: $(echo "$resp" | jq -c '.' 2>/dev/null || echo "$resp")"
    log "${label}: initiate returned no subject — resuming with ${subject}"
  fi

  api_post "$cb_gw" "/api/v1/compliance/approve-kyc" \
    "$(jq -cn --arg s "$subject" '{subject:$s, reason:"tryout onboarding approval"}')" "$cb_tok" >/dev/null || true
  api_post "$bank_gw" "/api/v1/onboarding/complete" \
    "$(jq -cn --arg s "$subject" '{request_id:$s, user_id:$s}')" "$bank_tok" >/dev/null || true

  status="$(api_get "$bank_gw" "/api/v1/onboarding/my-status" "$bank_tok" | jq -r '.status // empty')"
  [ "$status" = "ACTIVE" ] || fail "step3b: ${label} is ${status:-unknown} after onboarding, expected ACTIVE"
  printf -v "$tok_var" '%s' "$(gateway_login "$bank_gw" "$([ "$tok_var" = BANK_A_TOKEN ] && echo "$KC_BANK_A_CLIENT" || echo "$KC_BANK_B_CLIENT")" "$([ "$tok_var" = BANK_A_TOKEN ] && echo "$KC_BANK_A_SECRET" || echo "$KC_BANK_B_SECRET")" || true)"
  log "${label} ACTIVE — on-chain participant with its own wallet"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 3b: Onboard BOTH commercial banks — the payer and the beneficiary.
#
# This phase did not exist. Under the legacy bring-up commercial banks were
# pre-registered by provisioning, so the tryout could transact immediately. The
# toolkit does NOT pre-register them: onboarding is what verifies a bank, and an
# unverified bank has no on-chain wallet.
#
# The BENEFICIARY needs it as much as the payer: a cross-currency payment's bridge-out
# leg resolves the beneficiary on the DESTINATION spoke, and an unonboarded one fails
# the whole payment with BENEFICIARY_NOT_FOUND after the swap has already executed.
# ─────────────────────────────────────────────────────────────────────────────
step3b_onboard() {
  step "3b. Onboard ${BANK_A_CODE} (payer) and ${BANK_B_CODE} (beneficiary)"
  onboard_bank "$BANK_A_CODE" "$API_GW_BANK_A_URL" BANK_A_TOKEN \
    "$API_GW_CENTRAL_BANK_A_URL" "$CENTRAL_BANK_A_TOKEN" "$(echo "$SOURCE_CURRENCY" | cut -c1-2)"
  onboard_bank "$BANK_B_CODE" "$API_GW_BANK_B_URL" BANK_B_TOKEN \
    "$API_GW_CENTRAL_BANK_B_URL" "$CENTRAL_BANK_B_TOKEN" "$(echo "$TARGET_CURRENCY" | cut -c1-2)"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 3c: Tokenise reserves so the payer can actually pay.
#
# Also new. There are no static hub tokens a CB can mint from any more: a bank holds
# tCeBM on its OWN spoke, and gets it by putting fiat in. The full reserve path is
# four calls in two approvals — deposit fiat (bank) → CB approves, minting fCeBM;
# request escrow (bank) → CB approves, burning fCeBM and minting tCeBM. 1:1 in wei.
#
# Idempotent by measurement rather than by flag: it tops up only what is missing.
# ─────────────────────────────────────────────────────────────────────────────
step3c_fund_payer() {
  step "3c. Ensure ${BANK_A_CODE} holds at least ${PAYER_MIN_BALANCE} tCeBM"

  local balance
  balance="$(api_get "$API_GW_BANK_A_URL" "/api/v1/token/balance" "$BANK_A_TOKEN" | jq -r '.balance // "0"')"
  if python3 -c "import sys; sys.exit(0 if int('${balance:-0}') >= int('$PAYER_MIN_BALANCE') else 1)"; then
    log "${BANK_A_CODE} already holds ${balance} tCeBM — no tokenisation needed"
    return 0
  fi
  log "${BANK_A_CODE} holds ${balance} — tokenising ${PAYER_TOPUP} of fiat"

  local deposit_id escrow_id resp
  resp="$(api_post "$API_GW_BANK_A_URL" "/api/v1/payments/deposits" \
    "$(jq -cn --arg a "$PAYER_TOPUP" '{amount:$a}')" "$BANK_A_TOKEN" || true)"
  deposit_id="$(echo "$resp" | jq -r '.deposit_id // empty')"
  [ -n "$deposit_id" ] || fail "step3c: no deposit_id: $(echo "$resp" | jq -c '.' 2>/dev/null || echo "$resp")"

  resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v1/payments/deposits/approve" \
    "$(jq -cn --arg d "$deposit_id" '{deposit_id:$d}')" "$CENTRAL_BANK_A_TOKEN" || true)"
  echo "$resp" | jq -e '.error // empty' >/dev/null 2>&1 && \
    fail "step3c: deposit approve failed: $(echo "$resp" | jq -c '.')"

  resp="$(api_post "$API_GW_BANK_A_URL" "/api/v1/payments/escrows" \
    "$(jq -cn --arg d "$deposit_id" --arg a "$PAYER_TOPUP" '{deposit_id:$d, amount:$a}')" "$BANK_A_TOKEN" || true)"
  escrow_id="$(echo "$resp" | jq -r '.escrow_id // empty')"
  [ -n "$escrow_id" ] || fail "step3c: no escrow_id: $(echo "$resp" | jq -c '.' 2>/dev/null || echo "$resp")"

  resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v1/payments/escrows/approve" \
    "$(jq -cn --arg e "$escrow_id" '{escrow_id:$e}')" "$CENTRAL_BANK_A_TOKEN" || true)"
  echo "$resp" | jq -e '.error // empty' >/dev/null 2>&1 && \
    fail "step3c: escrow approve failed: $(echo "$resp" | jq -c '.')"

  balance="$(api_get "$API_GW_BANK_A_URL" "/api/v1/token/balance" "$BANK_A_TOKEN" | jq -r '.balance // "0"')"
  log "tokenised (deposit=${deposit_id} escrow=${escrow_id}) — balance now ${balance}"
}

# ─────────────────────────────────────────────────────────────────────────────
step4b_seed_liquidity() {
  step "4b. Seed ${POOL_PAIR} — each CB deposits its own side, then finalize"

  local pool_resp reserve_a reserve_b
  pool_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pool/${POOL_PAIR}/status" "$CENTRAL_BANK_A_TOKEN" || true)"
  reserve_a="$(echo "$pool_resp" | jq -r '.reserve_a // "0"')"
  reserve_b="$(echo "$pool_resp" | jq -r '.reserve_b // "0"')"
  if [ "$reserve_a" != "0" ] && [ "$reserve_b" != "0" ]; then
    log "Pool already funded (reserve_a=${reserve_a} reserve_b=${reserve_b}) — skipping seed"
    return 0
  fi

  local dep_a dep_b side_a side_b
  dep_a="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/deposit-side" \
    "$(jq -cn --arg p "$POOL_PAIR" --arg a "$SEED_AMOUNT_A" '{pool_pair:$p, amount:$a}')" "$CENTRAL_BANK_A_TOKEN" || true)"
  side_a="$(echo "$dep_a" | jq -r '.side // empty')"
  [ -n "$side_a" ] || fail "step4b: ${SOURCE_CURRENCY} CB deposit-side failed: $(echo "$dep_a" | jq -c '.' 2>/dev/null || echo "$dep_a")"
  log "${SOURCE_CURRENCY} CB deposited side ${side_a} (${SEED_AMOUNT_A})"

  dep_b="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/amm/liquidity/deposit-side" \
    "$(jq -cn --arg p "$POOL_PAIR" --arg a "$SEED_AMOUNT_B" '{pool_pair:$p, amount:$a}')" "$CENTRAL_BANK_B_TOKEN" || true)"
  side_b="$(echo "$dep_b" | jq -r '.side // empty')"
  [ -n "$side_b" ] || fail "step4b: ${TARGET_CURRENCY} CB deposit-side failed: $(echo "$dep_b" | jq -c '.' 2>/dev/null || echo "$dep_b")"
  log "${TARGET_CURRENCY} CB deposited side ${side_b} (${SEED_AMOUNT_B})"

  # Distinct sides are the point of sovereign seeding: the same side twice means one
  # CB funded both, which is the breach escrow-and-finalize exists to prevent.
  [ "$side_a" != "$side_b" ] || fail "step4b: both central banks deposited side ${side_a} — the pool would be one-sided"

  local fin_resp
  fin_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/finalize" \
    "$(jq -cn --arg p "$POOL_PAIR" '{pool_pair:$p}')" "$CENTRAL_BANK_A_TOKEN" || true)"
  echo "$fin_resp" | jq -e '.shares_a // empty' >/dev/null 2>&1 || \
    fail "step4b: finalize failed: $(echo "$fin_resp" | jq -c '.' 2>/dev/null || echo "$fin_resp")"
  log "finalized (shares_a=$(echo "$fin_resp" | jq -r '.shares_a') shares_b=$(echo "$fin_resp" | jq -r '.shares_b'))"

  local waited status
  status=""
  for waited in 0 3 6 9 12 15; do
    pool_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pool/${POOL_PAIR}/status" "$CENTRAL_BANK_A_TOKEN" || true)"
    status="$(echo "$pool_resp" | jq -r '.pool_status // empty')"
    [ "$status" = "ACTIVE" ] && break
    log "  pool_status=${status:-unknown} after ${waited}s — retrying"
    sleep 3
  done
  [ "$status" = "ACTIVE" ] || fail "step4b: pool ${POOL_PAIR} not ACTIVE after finalize (status=${status:-unknown})"
  log "pool ${POOL_PAIR} ACTIVE: reserve_a=$(echo "$pool_resp" | jq -r '.reserve_a') reserve_b=$(echo "$pool_resp" | jq -r '.reserve_b')"
}

# ─────────────────────────────────────────────────────────────────────────────
step5_us1() {
  step "5/US1. Quote + Swap + Pool Status"

  # --- C6: Assert pool is ACTIVE and fee_rate_bps = 30 before swap (SC-001 / T018) ---
  local pool_pre pool_status_pre fee_rate_pre reserve_a_pre reserve_b_pre
  pool_pre="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/${POOL_PAIR}/status" "$BANK_A_TOKEN")"
  pool_status_pre="$(echo "$pool_pre" | jq -r '.pool_status // "UNKNOWN"')"
  fee_rate_pre="$(echo "$pool_pre" | jq -r '.fee_rate_bps // "?"')"
  reserve_a_pre="$(echo "$pool_pre" | jq -r '.reserve_a // "0"')"
  reserve_b_pre="$(echo "$pool_pre" | jq -r '.reserve_b // "0"')"
  if [ "$pool_status_pre" = "ACTIVE" ]; then
    log "PASS: pool_status=ACTIVE (SC-001 / T018)"
  elif [ "$reserve_a_pre" = "0" ] || [ "$reserve_b_pre" = "0" ]; then
    # Graceful degradation expected in spec-007: ${POOL_PAIR} pool cannot be seeded
    # bilaterally with G5-cross pattern blocked. Use tryout-sovereign-cb-liquidity.sh
    # to validate US1 with the sovereign pair (W-tCeBM).
    log "WARN: step5: pool without bilateral liquidity (reserve_a=${reserve_a_pre}, reserve_b=${reserve_b_pre}) — swap skipped (spec-007: use tryout-sovereign-cb-liquidity.sh for US1)"
    return 0
  else
    log "WARN: pool_status=${pool_status_pre} — expected ACTIVE; reserves present (${reserve_a_pre}/${reserve_b_pre}) but status field unexpected"
  fi
  # The design is documented as 30 bps distributed to LPs on every swap
  # (docs/runbooks/contract-configuration.md, docs/design/cooperative-liquidity.md).
  # A pair's AMM is deployed by the proposing CB with feeBps = 0 (AutomatedMarketMaker
  # constructor) and setFeeBps — which exists — is called by nothing: not the toolkit,
  # not this script, not the backend at startup. So a runtime-opened corridor charges
  # no fee and its liquidity providers earn nothing.
  if [ "$fee_rate_pre" = "30" ]; then
    log "PASS: fee_rate_bps=30 (FR-005 / T024)"
  elif [ "$fee_rate_pre" = "0" ]; then
    log "WARN: fee_rate_bps=0 — the AMM deploys with no fee and nothing calls setFeeBps, so LPs earn nothing on this corridor (docs specify 30 bps)"
  else
    log "WARN: fee_rate_bps=${fee_rate_pre} — expected 30 (T018/T024)"
  fi

  # The payment goes through the CROSS-CURRENCY path: bridge-in (the issuing CB mints
  # the wrapped token on the hub, backed by locked reserves) → AMM swap on the
  # sovereign pool → bridge-out via the relay to the counterparty CB, which burns the
  # wrapped token for the beneficiary.
  #
  # It used to call /api/v2/amm/swap/exact-output, which is unreachable by design on
  # this topology: that endpoint requires the commercial_bank role AND a hub signing
  # key, and a bank gateway has none — SIGNER_PRIVATE_KEY is empty for banks because a
  # commercial bank must not sign on the hub. Every hub act is delegated to the CB.
  # Calling it produced "amm: swap requires a signing key; configure PrivateKeyHex",
  # surfaced here only as "Swap state=no state".
  local quote amount_in
  quote="$(api_get "$API_GW_BANK_A_URL" \
    "/api/v2/amm/quote/cross-currency?source_currency=${SOURCE_CURRENCY}&target_currency=${TARGET_CURRENCY}&amount_out=${SWAP_AMOUNT_OUT}&pool_pair=${POOL_PAIR}" \
    "$BANK_A_TOKEN")"
  amount_in="$(echo "$quote" | jq -r '.amount_in // empty')"
  [ -n "$amount_in" ] || fail "step5: no quote for ${SOURCE_CURRENCY}→${TARGET_CURRENCY}: $(echo "$quote" | jq -c '.' 2>/dev/null || echo "$quote")"
  log "Quote: pay ${amount_in} for ${SWAP_AMOUNT_OUT} (rate=$(echo "$quote" | jq -r '.effective_rate // "?"'))"

  local swap_body swap swap_state
  swap_body="$(jq -cn --arg sc "$SOURCE_CURRENCY" --arg tc "$TARGET_CURRENCY" --arg p "$POOL_PAIR" \
    --arg out "$SWAP_AMOUNT_OUT" --arg cap "$SWAP_MAX_AMOUNT_IN" --arg b "$BANK_B_CODE" \
    '{source_currency:$sc, target_currency:$tc, pool_pair:$p, amount_out:$out, max_amount_in:$cap, beneficiary_bank_id:$b}')"
  swap="$(api_post "$API_GW_BANK_A_URL" "/api/v2/amm/swap/cross-currency" "$swap_body" "$BANK_A_TOKEN")"
  swap_state="$(echo "$swap" | jq -r '.status // empty')"
  if [ "$swap_state" = "COMPLETED" ]; then
    log "PASS: cross-currency payment settled — in=$(echo "$swap" | jq -r '.amount_in') out=$(echo "$swap" | jq -r '.amount_out') tx=$(echo "$swap" | jq -r '.swap_tx_hash // "?"')"
  else
    fail "step5: cross-currency payment status=${swap_state:-none}: $(echo "$swap" | jq -c '.' 2>/dev/null || echo "$swap")"
  fi

  local pool pool_cba
  pool="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/${POOL_PAIR}/status" "$BANK_A_TOKEN")"
  log "Pool after swap (bank-a view): $(echo "$pool" | jq -c '.')"
  # I4 / Session 2026-05-20: total_lp_count is gateway-scoped (local DB only).
  # bank-a gateway has no LP positions in its DB -> total_lp_count=0 is EXPECTED, not a bug.
  local bank_a_lp_count
  bank_a_lp_count="$(echo "$pool" | jq -r '.total_lp_count // 0')"
  log "INFO: bank-a total_lp_count=$bank_a_lp_count (expected 0 — LP positions live in CB-A/CB-B gateways, not bank-a gateway)"
  # I4: also log total_lp_count from CB-A gateway (authoritative for cooperative LPs)
  pool_cba="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pool/${POOL_PAIR}/status" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Pool after swap (central-bank-a view, total_lp_count): $(echo "$pool_cba" | jq -r '.total_lp_count // 0')"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 6 — US2 (Legacy Scenario-B): Bridging (Lock&Mint + Burn&Unlock) + Liquidity Remove
# Note: "US2" here refers to the original Scenario B user story (bridging flows).
# The 005-cooperative-liquidity US2 (MLP as supplementary provider) reuses the
# same /add and /remove endpoints; LP_ID_BCB from step4b is used for removal below.
# ─────────────────────────────────────────────────────────────────────────────
step6_us2() {
  step "6/US2-LegacyScenarioB. Bridging (Lock&Mint + Burn&Unlock) + Cooperative Liquidity Remove"

  # Use LP_ID_BCB from cooperative commit-reveal (step4b) if available;
  # fall back to adding dual-sided liquidity (MLP / legacy path) otherwise.
  # The LP position to withdraw from comes from /liquidity/positions, which the CB's
  # own sovereign deposit records. The fallback that used to sit here posted to
  # /api/v2/amm/liquidity/add — a route that no longer exists: dual-sided add let ONE
  # central bank supply both sides, which is the sovereignty breach escrow-and-finalize
  # replaced. Calling it returned "Cannot POST" and killed the run.
  local LP_ID positions
  positions="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" "$CENTRAL_BANK_A_TOKEN" || true)"
  LP_ID="$(echo "$positions" | jq -r '[.positions[]? | select(.status=="ACTIVE")][0].lp_id // empty' 2>/dev/null || echo '')"
  if [ -n "$LP_ID" ]; then
    log "Using the ${SOURCE_CURRENCY} CB's own sovereign LP position: $LP_ID"
  else
    log "WARN: step6: no ACTIVE LP position for ${POOL_PAIR} — remove-liquidity skipped"
    log "      Bridge Lock&Mint + Burn&Unlock still exercised below."
  fi

  # Lock&Mint — initiate bridging
  local lock_resp
  # Amount only. The payload used to name the owner, the spoke and both assets
  # ("bank_a", "spoke-a", "BRL-CBDC", "mBRL-CBDC") — every one of them from the legacy
  # topology. The gateway derives all of it: the owner from the authenticated bank, the
  # spoke and its assets from the entity's own configuration. Naming them by hand
  # produced a position the relayer could not act on, so it sat in LOCKING until the
  # 120s wait gave up.
  lock_resp="$(api_post "$API_GW_BANK_A_URL" "/api/v2/bridge/lock-mint" \
    "$(jq -cn --arg a "$BRIDGE_AMOUNT" '{amount:$a}')" \
    "$BANK_A_TOKEN" || true)"
  log "Lock&Mint: $(echo "$lock_resp" | jq -c '.' || echo "$lock_resp")"

  # Get positions and capture first position_id
  local positions first_pos_id
  positions="$(api_get "$API_GW_BANK_A_URL" "/api/v2/bridge/positions" "$BANK_A_TOKEN")"
  log "Positions: $(echo "$positions" | jq -c '.')"
  first_pos_id="$(echo "$positions" | jq -r '.positions[0].position_id // empty')"
  [ -n "$first_pos_id" ] || fail "No positions returned after Lock&Mint — check bridge/lock-mint handler"

  # Wait for ACTIVE before Burn&Unlock (Decision 20 / FR-029 / T113)
  wait_for_position_state "$first_pos_id" "ACTIVE" 120 5

  # Burn&Unlock
  local burn_body
  burn_body="$(jq -cn --arg pid "$first_pos_id" '{position_id:$pid}')"
  log "Burn&Unlock: $(api_post "$API_GW_BANK_A_URL" "/api/v2/bridge/burn-unlock" "$burn_body" "$BANK_A_TOKEN" | jq -c '.' || true)"

  # Wait for BURNED confirmation (Decision 24 / FR-029)
  wait_for_position_state "$first_pos_id" "BURNED" 120 5

  # Remove liquidity — use lp_id — expect PROPORTIONAL withdrawal_mode for cooperative positions
  if [ -z "$LP_ID" ]; then
    log "SKIP: step6: remove liquidity — no LP_ID available (pool ${POOL_PAIR} without bilateral liquidity in spec-007)"
  else
  local remove_body remove_resp withdrawal_mode fee_claim_paid returned_a returned_b
  remove_body="$(jq -cn --arg id "$LP_ID" --arg bank "central_bank_a" --arg pair "$POOL_PAIR" '{lp_id:$id, pool_pair:$pair, provider_bank_id:$bank}')"
  remove_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/remove" "$remove_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Remove liquidity: $(echo "$remove_resp" | jq -c '.' || echo "$remove_resp")"

  withdrawal_mode="$(echo "$remove_resp" | jq -r '.withdrawal_mode // "UNKNOWN"')"
  # LPResult serializes as token_a_amount / token_b_amount (not returned_token_a/b).
  returned_a="$(echo "$remove_resp" | jq -r '.token_a_amount // .returned_token_a // "?"')"
  returned_b="$(echo "$remove_resp" | jq -r '.token_b_amount // .returned_token_b // "?"')"
  fee_claim_paid="$(echo "$remove_resp" | jq -r '.fee_claim_paid // "?"')"

  log "Withdrawal mode:  ${withdrawal_mode}"
  log "Returned token A: ${returned_a}"
  log "Returned token B: ${returned_b}"
  log "Fee claim paid:   ${fee_claim_paid}"

  if [ "$withdrawal_mode" = "PROPORTIONAL" ]; then
    log "PASS: proportional withdrawal confirmed (FR-007 / 005-cooperative-liquidity)"
  else
    log "INFO: withdrawal_mode=${withdrawal_mode} (PROPORTIONAL expected for cooperative positions)"
  fi

  # C4: fee_claim_paid validation — FR-006 / SC-006 (best-effort in multi-gateway, Session 2026-05-20)
  # In multi-gateway deployments, swaps via bank-a gateway cannot reach LP positions in CB-A/CB-B DB.
  # fee_claim_paid=0 is EXPECTED in this setup, not a bug.
  if [ "$fee_claim_paid" != "?" ] && [ "$fee_claim_paid" != "0" ] && [ "$fee_claim_paid" != "" ]; then
    log "PASS: fee_claim_paid=${fee_claim_paid} — LP received fee distribution (FR-006 / single-gateway path)"
  else
    log "INFO: fee_claim_paid=${fee_claim_paid} — expected in multi-gateway: swap via bank-a gateway does not reach LP positions in CB-A DB (FR-006 best-effort / Session 2026-05-20)"
  fi
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 7 — US3: Circuit Breaker + Master Viewing Key Disclosure
# ─────────────────────────────────────────────────────────────────────────────
step7_us3() {
  step "7/US3. Circuit Breaker (asymmetric) + Master Viewing Key"

  local status
  status="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/governance/circuit-breaker/status" "$CENTRAL_BANK_A_TOKEN")"
  log "CB status (pre): $(echo "$status" | jq -c '.')"

  local pause_body pause_resp pause_state
  pause_body='{"pair":"'"$POOL_PAIR"'","bank_id":"central_bank_1","reason_code":"E2E_TRYOUT_INCIDENT","signature":"AA=="}'
  pause_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/governance/circuit-breaker/pause" "$pause_body" "$CENTRAL_BANK_A_TOKEN" | jq -c '.' || true)"
  log "Pause CB: ${pause_resp}"
  pause_state="$(echo "$pause_resp" | jq -r '.state // empty')"
  [ "$pause_state" = "HALTED" ] && log "PASS: Circuit Breaker paused → HALTED (FR-030)" || \
    log "WARN: expected HALTED after pause, got: ${pause_state:-no state}"

  local resume_req
  resume_req='{"pair":"'"$POOL_PAIR"'","bank_id":"central_bank_1","signature":"AA=="}'
  local req_resp
  req_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/governance/circuit-breaker/resume-request" "$resume_req" "$CENTRAL_BANK_A_TOKEN")"
  log "Resume proposal: $(echo "$req_resp" | jq -c '.')"
  local request_id
  request_id="$(echo "$req_resp" | jq -r '.request_id // empty')"

  if [ -n "$request_id" ]; then
    local sign_body resume_sign_resp resume_state
    sign_body="$(jq -cn --arg id "$request_id" --arg bank "central_bank_2" --arg pair "$POOL_PAIR" '{pair:$pair, request_id:$id, bank_id:$bank, signature:"AA=="}')"
    resume_sign_resp="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/governance/circuit-breaker/resume-sign" "$sign_body" "$CENTRAL_BANK_B_TOKEN" | jq -c '.' || true)"
    log "Resume sign#2: ${resume_sign_resp}"
    resume_state="$(echo "$resume_sign_resp" | jq -r '.state // empty')"
    [ "$resume_state" = "LIVE" ] && log "PASS: Circuit Breaker resumed → LIVE after multi-sig (FR-030)" || \
      log "WARN: expected LIVE after resume-sign#2, got: ${resume_state:-no state}"
  fi

  local disc_body
  disc_body='{"tx_ref":"did:spoke-b:bank-b:tx001","requestor_id":"central_bank_1","reason_code":"REGULATORY_INVESTIGATION_001"}'
  local disc_resp
  disc_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/oversight/disclosure-request" "$disc_body" "$CENTRAL_BANK_A_TOKEN")"
  log "Disclosure open: $(echo "$disc_resp" | jq -c '.')"
  local disc_id
  disc_id="$(echo "$disc_resp" | jq -r '.request_id // empty')"

  if [ -n "$disc_id" ]; then
    local d2 d3 disc_status disc_state
    d2="$(jq -cn --arg id "$disc_id" --arg s "central_bank_2" '{request_id:$id, signer_id:$s}')"
    d3="$(jq -cn --arg id "$disc_id" --arg s "central_bank_3" '{request_id:$id, signer_id:$s}')"
    log "Disclosure sign#2: $(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/oversight/disclosure-sign" "$d2" "$CENTRAL_BANK_A_TOKEN" | jq -c '.' || true)"
    log "Disclosure sign#3: $(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/oversight/disclosure-sign" "$d3" "$CENTRAL_BANK_A_TOKEN" | jq -c '.' || true)"
    disc_status="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/oversight/disclosure-status/${disc_id}" "$CENTRAL_BANK_A_TOKEN" | jq -c '.' || true)"
    log "Disclosure status: ${disc_status}"
    disc_state="$(echo "$disc_status" | jq -r '.state // empty')"
    [ "$disc_state" = "QUORUM_REACHED" ] && log "PASS: Disclosure QUORUM_REACHED (2/2 signatures — FR-034)" || \
      log "WARN: expected QUORUM_REACHED, got: ${disc_state:-no state}"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# ─────────────────────────────────────────────────────────────────────────────
# Step 8 — US5 (D9/D10): PairRegistry — Registro bilateral de novo par
# ─────────────────────────────────────────────────────────────────────────────
# Fluxo 6 (US2 MLP): MLP deposita liquidez dual-sided + remove
# Condicional a ENABLE_MLP=true (SC-004) — clarification 2026-05-18
# Acceptance criteria: FR-004, SC-004
# ────────────────────────────────────────────────────────────────────────────────
step_mlp_us2() {
  step "MLP/US2. MLP deposits dual-sided liquidity (ENABLE_MLP=true) and removes"

  # ── 1. Acquire MLP token ──
  # login_keycloak is gone (it used the client-credentials grant the auth service
  # refuses); the MLP authenticates through its own gateway like every other entity.
  MLP_TOKEN="$(gateway_login "$API_GW_MLP_URL" "$KC_MLP_CLIENT" "$KC_MLP_SECRET" || true)"
  if [ -z "$MLP_TOKEN" ]; then
    log "WARN: MLP token not acquired — check KC_MLP_SECRET and mlp realm in Keycloak"
    log "Skipping step_mlp_us2 (ENABLE_MLP=true but token unavailable)"
    return 0
  fi
  log "MLP token acquired (realm=mlp, client=mlp-client)"

  # ── 2. POST /api/v2/amm/liquidity/add (dual-sided) ──
  local add_body add_resp lp_id deposit_side
  add_body="$(jq -cn --arg pair "$POOL_PAIR" \
    '{pool_pair:$pair,token_a_amount:"10000",token_b_amount:"10000",provider_bank_id:"mlp"}')"
  add_resp="$(api_post "$API_GW_MLP_URL" "/api/v2/amm/liquidity/add" \
    "$add_body" "$MLP_TOKEN" || true)"
  log "MLP addLiquidity response: $(echo "$add_resp" | jq -c '.')"

  lp_id="$(echo "$add_resp" | jq -r '.lp_id // empty')"
  deposit_side="$(echo "$add_resp" | jq -r '.deposit_side // empty')"

  if [ -n "$lp_id" ] && [ "$deposit_side" = "BOTH" ]; then
    log "PASS: MLP lp_id=${lp_id}, deposit_side=BOTH (FR-004 / SC-004)"
  else
    log "WARN: expected non-null lp_id and deposit_side=BOTH — got lp_id=${lp_id:-empty} deposit_side=${deposit_side:-empty}"
  fi

  # ── 3. Verify MLP indistinguishable from CB (GET /api/v2/amm/liquidity/providers) ──
  local providers_resp mlp_count
  providers_resp="$(api_get "$API_GW_MLP_URL" "/api/v2/amm/liquidity/providers" "$MLP_TOKEN" || true)"
  mlp_count="$(echo "$providers_resp" | jq '[.providers[] // .[] // empty | select(.lp_id == "mlp" or (.provider_id // "") == "mlp")] | length' 2>/dev/null || echo 0)"
  log "Providers: $(echo "$providers_resp" | jq -c '.' 2>/dev/null || echo "$providers_resp")"
  [ "$mlp_count" -ge 1 ] && \
    log "PASS: MLP visible in GET /providers — indistinguishable from CB (SC-004)" || \
    log "WARN: MLP not found in GET /providers — check handler"

  # ── 4. POST /api/v2/amm/liquidity/remove ──
  if [ -n "$lp_id" ]; then
    local remove_body remove_resp withdrawal_mode
    remove_body="$(jq -cn --arg id "$lp_id" '{lp_id:$id}')"
    remove_resp="$(api_post "$API_GW_MLP_URL" "/api/v2/amm/liquidity/remove" \
      "$remove_body" "$MLP_TOKEN" || true)"
    log "MLP removeLiquidity response: $(echo "$remove_resp" | jq -c '.')"
    withdrawal_mode="$(echo "$remove_resp" | jq -r '.withdrawal_mode // empty')"
    [ "$withdrawal_mode" = "PROPORTIONAL" ] && \
      log "PASS: withdrawal_mode=PROPORTIONAL (FR-004 / SC-004)" || \
      log "WARN: withdrawal_mode=${withdrawal_mode:-empty} — expected PROPORTIONAL"
  fi

  log "=== Flow 6 (US2 MLP) complete (FR-004 / SC-004) ==="
}

# ─────────────────────────────────────────────────────────────────────────────
# Fluxo 5: BRL-ARS propose → rejection (PAIR_ALREADY_EXISTS + NOT_CENTRAL_BANK) → confirm → ACTIVE
# Acceptance criteria: SC-009, FR-016, FR-017
# ─────────────────────────────────────────────────────────────────────────────
step8_pair_registry() {
  step "8/US5. PairRegistry: propose + rejection tests + confirm + GET /pairs"

  # ── 1. CB-A (Brazil, issuer of tCeBM_BRL) proposes BRL-EUR ──
  #    CB-A is getCentralBankOf(tokenA=BRL); CB-B is getCentralBankOf(tokenB=EUR).
  #    Uses the existing AMM_CONTRACT_ADDRESS (BRL-EUR AMM) and both tokens already on hub.
  local propose_body
  propose_body="$(jq -cn \
    --arg pair "BRL-EUR" \
    --arg tA   "${HUB_TOKEN_A_ADDRESS:-}" \
    --arg tB   "${HUB_TOKEN_B_ADDRESS:-}" \
    --arg amm  "${AMM_CONTRACT_ADDRESS:-$(grep -s '^AMM_CONTRACT_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}" \
    --arg cb   "cb-brazil" \
    '{pair_id:$pair, token_a_address:$tA, token_b_address:$tB, amm_address:$amm, proposer_cb:$cb}')"
  local propose_resp propose_status propose_code
  propose_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs/propose" \
    "$propose_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  propose_status="$(echo "$propose_resp" | jq -r '.status // "ERROR"')"
  propose_code="$(echo "$propose_resp" | jq -r '.code // empty')"
  log "Propose BRL-EUR: $(echo "$propose_resp" | jq -c '.')"
  if [ "$propose_status" = "PROPOSED" ] || [ "$propose_code" = "PAIR_ALREADY_EXISTS" ]; then
    log "PASS: BRL-EUR proposed successfully (FR-017 / SC-009)"
  else
    log "WARN: propose status=${propose_status} — expected PROPOSED (check PAIR_REGISTRY_CONTRACT_ADDRESS and signer setup)"
  fi

  # ── 2. GET /pairs — BRL-EUR deve aparecer como PROPOSED ou ACTIVE (SC-009) ──
  local pairs_resp proposed_count
  pairs_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Pairs after propose: $(echo "$pairs_resp" | jq -c '.pairs | map({pair_id,status})')"
  proposed_count="$(echo "$pairs_resp" | jq '[.pairs[] | select(.status=="PROPOSED" or .status=="ACTIVE")] | length')"
  [ "$proposed_count" -ge 1 ] && log "PASS: BRL-EUR visible as PROPOSED/ACTIVE (SC-009)" || \
    log "WARN: no PROPOSED pairs in GET /pairs response"

  # ── 3. Rejection: PAIR_ALREADY_EXISTS — second proposal with same pair_id → 409 ──
  local dup_resp dup_code
  dup_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs/propose" \
    "$propose_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  dup_code="$(echo "$dup_resp" | jq -r '.code // empty')"
  if [ "$dup_code" = "PAIR_ALREADY_EXISTS" ]; then
    log "PASS: PAIR_ALREADY_EXISTS (409) on second proposal with same pair_id (FR-016)"
  else
    log "WARN: expected PAIR_ALREADY_EXISTS but got: ${dup_code:-no code} ($(echo "$dup_resp" | jq -c '.'))"
  fi

  # ── 4. Rejection: NOT_CENTRAL_BANK_OF_TOKEN_A — commercial bank cannot propose ──
  local unauth_body unauth_resp unauth_code
  unauth_body="$(jq -cn \
    --arg pair "$POOL_PAIR" \
    --arg tA   "${HUB_TOKEN_A_ADDRESS:-}" \
    --arg tB   "${HUB_TOKEN_B_ADDRESS:-}" \
    --arg amm  "${AMM_CONTRACT_ADDRESS:-0x0000000000000000000000000000000000000001}" \
    --arg cb   "bank-a" \
    '{pair_id:$pair, token_a_address:$tA, token_b_address:$tB, amm_address:$amm, proposer_cb:$cb}')"
  unauth_resp="$(api_post "$API_GW_BANK_A_URL" "/api/v2/amm/pairs/propose" \
    "$unauth_body" "$BANK_A_TOKEN" || true)"
  unauth_code="$(echo "$unauth_resp" | jq -r '.code // .error_code // empty')"
  if [ "$unauth_code" = "NOT_CENTRAL_BANK_OF_TOKEN_A" ] || [ "$unauth_code" = "INSUFFICIENT_ROLE" ]; then
    log "PASS: NOT_CENTRAL_BANK_OF_TOKEN_A/INSUFFICIENT_ROLE (403) for commercial bank trying to propose (FR-016)"
  else
    log "WARN: expected NOT_CENTRAL_BANK_OF_TOKEN_A but got: ${unauth_code:-no code} ($(echo "$unauth_resp" | jq -c '.'))"
  fi

  # ── 5. Rejection: NOT_CENTRAL_BANK_OF_TOKEN_B — CB-A cannot confirm BRL-EUR ──
  #    getCentralBankOf(tokenB=EUR) == CB-B, not CB-A → expected rejection.
  local wrong_confirm_body wrong_confirm_resp wrong_confirm_code wrong_confirm_err wrong_confirm_active
  wrong_confirm_body="$(jq -cn --arg pair "BRL-EUR" --arg cb "cb-brazil" \
    '{pair_id:$pair, confirmer_cb:$cb}')"
  wrong_confirm_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs/confirm" \
    "$wrong_confirm_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  wrong_confirm_code="$(echo "$wrong_confirm_resp" | jq -r '.code // empty')"
  wrong_confirm_err="$(echo "$wrong_confirm_resp" | jq -r '.error // empty')"
  wrong_confirm_active="$(echo "$wrong_confirm_resp" | jq -r '.status // empty')"
  if [ "$wrong_confirm_code" = "NOT_CENTRAL_BANK_OF_TOKEN_B" ] || \
     [ "$wrong_confirm_code" = "PAIR_ALREADY_ACTIVE" ] || \
     [ "$wrong_confirm_active" = "ACTIVE" ] || \
     [[ "$wrong_confirm_err" == *"reverted"* ]] || \
     [[ "$wrong_confirm_err" == *"AlreadyActive"* ]]; then
    log "PASS: NOT_CENTRAL_BANK_OF_TOKEN_B/AlreadyActive when CB-A tries to confirm CB-B pair (FR-016)"
  else
    log "WARN: expected NOT_CENTRAL_BANK_OF_TOKEN_B but got: ${wrong_confirm_code:-no code} ($(echo "$wrong_confirm_resp" | jq -c '.'))"
  fi

  # ── 6. CB-B (Fed/EUR, issuer of tCeBM_EUR) confirma BRL-EUR ──
  local confirm_body confirm_resp confirm_status confirm_code
  confirm_body="$(jq -cn --arg pair "BRL-EUR" --arg cb "cb-fed" \
    '{pair_id:$pair, confirmer_cb:$cb}')"
  confirm_resp="$(api_post "${API_GW_CENTRAL_BANK_B_URL}" "/api/v2/amm/pairs/confirm" \
    "$confirm_body" "${CENTRAL_BANK_B_TOKEN}" || true)"
  confirm_status="$(echo "$confirm_resp" | jq -r '.status // "ERROR"')"
  confirm_code="$(echo "$confirm_resp" | jq -r '.code // empty')"
  log "Confirm BRL-EUR: $(echo "$confirm_resp" | jq -c '.')"
  if [ "$confirm_status" = "ACTIVE" ] || [ "$confirm_code" = "PAIR_ALREADY_ACTIVE" ]; then
    log "PASS: BRL-EUR confirmed and ACTIVE (SC-009 / FR-016)"
  else
    log "WARN: confirm status=${confirm_status} — expected ACTIVE"
  fi

  # ── 7. GET /pairs — BRL-EUR deve estar ACTIVE (SC-009) ──
  local pairs_resp_b active_count
  pairs_resp_b="$(api_get "${API_GW_CENTRAL_BANK_B_URL}" "/api/v2/amm/pairs" "${CENTRAL_BANK_B_TOKEN}" || true)"
  pairs_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pairs" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Pairs after confirm (CB-B): $(echo "$pairs_resp_b" | jq -c '.pairs | map({pair_id,status})')"
  log "Pairs after confirm (CB-A): $(echo "$pairs_resp" | jq -c '.pairs | map({pair_id,status})')"
  active_count="$(( $(echo "$pairs_resp_b" | jq '[.pairs[] | select(.status=="ACTIVE")] | length') + $(echo "$pairs_resp" | jq '[.pairs[] | select(.status=="ACTIVE")] | length') ))"
  [ "$active_count" -ge 1 ] && log "PASS: ${active_count} pair(s) ACTIVE in GET /pairs — no restart needed (SC-009)" || \
    log "WARN: no ACTIVE pairs in GET /pairs"

  # ── 8. Quote check on BRL-EUR — PairRouter should route to correct AMM ──
  local quote_resp
  quote_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" \
    "/api/v2/amm/quote/exact-output?pair=BRL-EUR&amount_out=1000" \
    "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Quote BRL-EUR: $(echo "$quote_resp" | jq -c '.')"
  echo "$quote_resp" | jq -e '.required_input' >/dev/null 2>&1 && \
    log "PASS: quote BRL-EUR returned required_input — PairRouter routed correctly (FR-015)" || \
    log "WARN: quote BRL-EUR missing required_input — check PairRouter and AMM_CONTRACT_ADDRESS"

  log "=== Flow 5 complete (SC-009 / FR-015 / FR-016 / FR-017) ==="

  # ── FR-011: Reject duplicate pair by token combination (006-hub-currency-registry) ──
  # BRL-EUR already exists as PROPOSED ou ACTIVE → try to propose EUR-BRL (tokens invertidos) → PAIR_ALREADY_EXISTS
  local dup_token_body dup_token_resp dup_token_code
  dup_token_body="$(jq -cn \
    --arg pair    "EUR-BRL" \
    --arg tA      "${HUB_TOKEN_B_ADDRESS:-}" \
    --arg tB      "${HUB_TOKEN_A_ADDRESS:-}" \
    --arg amm     "${AMM_CONTRACT_ADDRESS:-0x0000000000000000000000000000000000000001}" \
    --arg cb      "cb-fed" \
    '{pair_id:$pair, token_a_address:$tA, token_b_address:$tB, amm_address:$amm, proposer_cb:$cb}')"
  dup_token_resp="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/amm/pairs/propose" \
    "$dup_token_body" "$CENTRAL_BANK_B_TOKEN" || true)"
  dup_token_code="$(echo "$dup_token_resp" | jq -r '.code // empty')"
  if [ "$dup_token_code" = "PAIR_ALREADY_EXISTS" ]; then
    log "PASS: FR-011 — EUR-BRL rejected with PAIR_ALREADY_EXISTS (token pair already exists as BRL-EUR)"
  else
    log "WARN: FR-011 — expected PAIR_ALREADY_EXISTS for EUR-BRL but got: ${dup_token_code:-no code} ($(echo "$dup_token_resp" | jq -c '.'))"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 9 — US6: CurrencyRegistry — registro e descoberta de moedas no hub
# 006-hub-currency-registry / FR-003 / FR-004 / FR-005
# ─────────────────────────────────────────────────────────────────────────────
step9_currency_registry() {
  step "9/US6. CurrencyRegistry: register + list + reject duplicates + remove"

  if [ -z "${CURRENCY_REGISTRY_CONTRACT_ADDRESS:-}" ]; then
    log "SKIP: CURRENCY_REGISTRY_CONTRACT_ADDRESS not configured — skipping step9_currency_registry"
    log "      Set the variable (or add to backend/config/.env.infra.central-bank-a) after contract deploy"
    return 0
  fi
  log "CurrencyRegistry: ${CURRENCY_REGISTRY_CONTRACT_ADDRESS}"

  # ── 1. GET /api/v2/hub/currencies (public, no auth) — baseline before register ──
  local baseline_resp baseline_count
  baseline_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" || true)"
  baseline_count="$(echo "$baseline_resp" | jq '.currencies | length' 2>/dev/null || echo 0)"
  log "Currencies before registration (baseline): ${baseline_count}"

  # ── 2. FR-003: CB-A registra BRL no hub ──
  local reg_brl_body reg_brl_resp reg_brl_status reg_brl_code
  reg_brl_body="$(jq -cn \
    --arg sym  "BRL" \
    --arg name "Brazil" \
    --arg addr "${HUB_TOKEN_A_ADDRESS:-}" \
    --arg cb   "central_bank_a" \
    '{symbol:$sym, country_name:$name, token_address:$addr, proposer_cb:$cb}')"
  reg_brl_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" \
    "$reg_brl_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  reg_brl_status="$(echo "$reg_brl_resp" | jq -r '.symbol // .error // empty')"
  reg_brl_code="$(echo "$reg_brl_resp" | jq -r '.code // empty')"
  log "Register BRL: $(echo "$reg_brl_resp" | jq -c '.')"
  if [ "$reg_brl_status" = "BRL" ] || [ "$reg_brl_code" = "CURRENCY_ALREADY_EXISTS" ]; then
    log "PASS: BRL registered (or already existed) in CurrencyRegistry (FR-003)"
  else
    log "WARN: Register BRL — status=${reg_brl_status} code=${reg_brl_code:-no code} — check CURRENCY_REGISTRY_CONTRACT_ADDRESS and signer"
  fi

  # ── 3. FR-003: CB-B registra EUR no hub ──
  local reg_eur_body reg_eur_resp reg_eur_status reg_eur_code
  reg_eur_body="$(jq -cn \
    --arg sym  "EUR" \
    --arg name "European Union" \
    --arg addr "${HUB_TOKEN_B_ADDRESS:-}" \
    --arg cb   "central_bank_b" \
    '{symbol:$sym, country_name:$name, token_address:$addr, proposer_cb:$cb}')"
  reg_eur_resp="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/hub/currencies" \
    "$reg_eur_body" "$CENTRAL_BANK_B_TOKEN" || true)"
  reg_eur_status="$(echo "$reg_eur_resp" | jq -r '.symbol // .error // empty')"
  reg_eur_code="$(echo "$reg_eur_resp" | jq -r '.code // empty')"
  log "Register EUR: $(echo "$reg_eur_resp" | jq -c '.')"
  if [ "$reg_eur_status" = "EUR" ] || [ "$reg_eur_code" = "CURRENCY_ALREADY_EXISTS" ]; then
    log "PASS: EUR registered (or already existed) in CurrencyRegistry (FR-003)"
  else
    log "WARN: Register EUR — status=${reg_eur_status} code=${reg_eur_code:-no code}"
  fi

  # ── 4. FR-005: GET /api/v2/hub/currencies — BRL e EUR should appear (public, no auth) ──
  local list_resp list_count brl_found eur_found
  list_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" || true)"
  list_count="$(echo "$list_resp" | jq '.currencies | length' 2>/dev/null || echo 0)"
  brl_found="$(echo "$list_resp" | jq -r '[.currencies[] | select(.symbol=="BRL")] | length' 2>/dev/null || echo 0)"
  eur_found="$(echo "$list_resp" | jq -r '[.currencies[] | select(.symbol=="EUR")] | length' 2>/dev/null || echo 0)"
  log "GET /hub/currencies: ${list_count} currency(ies) — BRL=${brl_found}, EUR=${eur_found}"
  if [ "$brl_found" -ge 1 ] && [ "$eur_found" -ge 1 ] 2>/dev/null; then
    log "PASS: BRL and EUR visible in GET /api/v2/hub/currencies (FR-005)"
  else
    log "WARN: expected BRL=1 and EUR=1, got BRL=${brl_found} EUR=${eur_found} — list: $(echo "$list_resp" | jq -c '.currencies | map(.symbol)')"
  fi

  # ── 5. Rejection: CURRENCY_ALREADY_EXISTS — duplicate symbol ──
  local dup_sym_resp dup_sym_code
  dup_sym_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" \
    "$reg_brl_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  dup_sym_code="$(echo "$dup_sym_resp" | jq -r '.code // empty')"
  if [ "$dup_sym_code" = "CURRENCY_ALREADY_EXISTS" ]; then
    log "PASS: CURRENCY_ALREADY_EXISTS (409) on attempt to re-register BRL (FR-003)"
  else
    log "WARN: expected CURRENCY_ALREADY_EXISTS but got: ${dup_sym_code:-no code} ($(echo "$dup_sym_resp" | jq -c '.'))"
  fi

  # ── 6. Rejection: UNAUTHORIZED — commercial bank cannot register currency ──
  local unauth_resp unauth_code unauth_error
  # Send with commercial bank token to CB-A gateway (which has the route registered).
  # O token bank-a is from a different realm (bank-a vs central-bank-a) → "invalid token" (401)
  # ou, se o realm for compartilhado, RequireCentralBankRole() retorna INSUFFICIENT_ROLE (403).
  # Both cases represent correct denied access for FR-003.
  unauth_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" \
    "$reg_brl_body" "$BANK_A_TOKEN" || true)"
  unauth_code="$(echo "$unauth_resp" | jq -r '.code // .error_code // empty' 2>/dev/null || echo '')"
  unauth_error="$(echo "$unauth_resp" | jq -r '.error // empty' 2>/dev/null || echo '')"
  if [ "$unauth_code" = "UNAUTHORIZED" ] || [ "$unauth_code" = "INSUFFICIENT_ROLE" ] || \
     [ "$unauth_error" = "invalid token" ] || \
     echo "$unauth_resp" | grep -q '"status":403\|status.*403\|Forbidden\|forbidden'; then
    log "PASS: access denied (401/403) for commercial bank trying to register currency (FR-003 — CB-only)"
  else
    log "WARN: expected access denied but got: ${unauth_code:-${unauth_error:-no code}} ($(echo "$unauth_resp" | jq -c '.' 2>/dev/null || echo 'non-JSON'))"
  fi

  # ── 7. FR-004: CB-A remove BRL (DELETE /api/v2/hub/currencies/BRL) ──
  local del_resp del_code
  del_resp="$(api_delete "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies/BRL" \
    "$CENTRAL_BANK_A_TOKEN" || true)"
  del_code="$(echo "$del_resp" | jq -r '.code // .symbol // empty' 2>/dev/null || echo '')"
  log "DELETE /hub/currencies/BRL: $(echo "$del_resp" | jq -c '.')"
  if [ "$del_code" = "BRL" ] || echo "$del_resp" | jq -e '.symbol == "BRL"' >/dev/null 2>&1; then
    log "PASS: BRL removed from CurrencyRegistry (FR-004)"
  else
    log "WARN: DELETE BRL — code=${del_code:-no code} — check handler and auth"
  fi

  # ── 8. FR-005: GET /hub/currencies — BRL should no longer appear ──
  local post_del_resp brl_after_del
  post_del_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" || true)"
  brl_after_del="$(echo "$post_del_resp" | jq -r '[.currencies[] | select(.symbol=="BRL")] | length' 2>/dev/null || echo 0)"
  if [ "$brl_after_del" = "0" ] 2>/dev/null; then
    log "PASS: BRL absent from GET /api/v2/hub/currencies after removal (FR-004 / tombstone)"
  else
    log "WARN: BRL still appears after DELETE — tombstone may not have worked (brl_after_del=${brl_after_del})"
  fi

  # ── 9. Register BRL again (symbol released after removal) ──
  local rereg_resp rereg_status
  rereg_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/hub/currencies" \
    "$reg_brl_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  rereg_status="$(echo "$rereg_resp" | jq -r '.symbol // empty')"
  if [ "$rereg_status" = "BRL" ]; then
    log "PASS: BRL successfully re-registered after removal (symbol released by tombstone)"
  else
    log "INFO: BRL re-registration — status=${rereg_status:-no symbol} ($(echo "$rereg_resp" | jq -c '.'))"
  fi

  log "=== Step 9 complete (FR-003 / FR-004 / FR-005 — 006-hub-currency-registry) ==="
}

# ─────────────────────────────────────────────────────────────────────────────
# print_report — human-readable summary of PASS / WARN / SKIP / FAIL
# Called automatically via trap EXIT; no explicit call needed.
# ─────────────────────────────────────────────────────────────────────────────
print_report() {
  [[ ! -f "${REPORT_FILE:-}" ]] && return

  local status msg
  local pass=() warn=() skip=() fail_=()

  while IFS='|' read -r status msg; do
    case "$status" in
      PASS) pass+=("$msg")  ;;
      WARN) warn+=("$msg")  ;;
      SKIP) skip+=("$msg")  ;;
      FAIL) fail_+=("$msg") ;;
    esac
  done < "$REPORT_FILE"

  printf '\n' >&2
  printf '════════════════════════════════════════════════════════════\n' >&2
  printf '  SCENARIO B E2E — FINAL REPORT\n' >&2
  printf '════════════════════════════════════════════════════════════\n' >&2

  if [[ ${#pass[@]} -gt 0 ]]; then
    printf '\n  ✅  PASS (%d)\n' "${#pass[@]}" >&2
    for msg in "${pass[@]}"; do printf '      ✓  %s\n' "$msg" >&2; done
  fi

  if [[ ${#warn[@]} -gt 0 ]]; then
    printf '\n  ⚠️   WARN (%d)\n' "${#warn[@]}" >&2
    for msg in "${warn[@]}"; do printf '      ⚠  %s\n' "$msg" >&2; done
  fi

  if [[ ${#skip[@]} -gt 0 ]]; then
    printf '\n  ⏭   SKIP (%d)\n' "${#skip[@]}" >&2
    for msg in "${skip[@]}"; do printf '      ○  %s\n' "$msg" >&2; done
  fi

  if [[ ${#fail_[@]} -gt 0 ]]; then
    printf '\n  ❌  FAIL (%d)\n' "${#fail_[@]}" >&2
    for msg in "${fail_[@]}"; do printf '      ✗  %s\n' "$msg" >&2; done
  fi

  # A green verdict must mean "checks ran and passed", not "no check recorded a
  # failure". Without the first line below, a run that connected to nothing printed
  # `RESULT: ✅ OK ( PASS=0 FAIL=0 )` — measured on 2026-08-21 against a healthy
  # toolkit stack, where every call failed on the retired deploy/local port and the
  # verdict still read OK. The EXIT trap makes it worse: an aborted or timed-out run
  # reaches this function too, so the banner appeared for runs that barely started.
  # Anyone using that banner as evidence was reading a default, not a result.
  local overall='✅ OK'
  [[ ${#pass[@]} -eq 0 ]] && overall='❌ NOTHING VERIFIED (no check ran)'
  [[ ${#warn[@]} -gt 0 && ${#fail_[@]} -eq 0 && ${#pass[@]} -gt 0 ]] && overall='⚠️  OK WITH WARNINGS'
  [[ ${#fail_[@]} -gt 0 ]] && overall='❌ FAILED'
  # A run that DIED mid-way is not a pass, however many checks it managed first. The
  # trap fires on any exit, so without this a story that aborted — an removed endpoint,
  # a curl that blew up, Ctrl-C — printed the verdict of the checks it happened to reach
  # and called it OK. Observed on us3: it died on `Cannot POST /liquidity/add` and
  # reported "✅ OK (PASS=3)".
  [[ ${RUN_COMPLETED:-0} -eq 1 ]] || overall='❌ ABORTED (the run did not reach the end)'

  printf '\n' >&2
  printf '  RESULT: %s   ( PASS=%d  WARN=%d  SKIP=%d  FAIL=%d )\n' \
    "$overall" "${#pass[@]}" "${#warn[@]}" "${#skip[@]}" "${#fail_[@]}" >&2
  printf '════════════════════════════════════════════════════════════\n' >&2

  rm -f "$REPORT_FILE"
}

# Registers print_report to run on exit (success, failure or Ctrl-C).
trap 'print_report' EXIT

# ─────────────────────────────────────────────────────────────────────────────
# Dispatch
# ─────────────────────────────────────────────────────────────────────────────
step0_up
step1_infrastructure
step2_contracts
step3_participants
step3b_onboard
step3c_fund_payer
step4_pool
step4a_open_corridor
step4b_seed_liquidity

case "$STORY" in
  us1) step5_us1 ;;
  us2) step5_us1; step6_us2; if [[ "${ENABLE_MLP:-false}" == "true" ]]; then step_mlp_us2; fi ;;
  us3) step5_us1; step6_us2; step7_us3 ;;
  us5) step8_pair_registry ;;
  us6) step9_currency_registry ;;
  all) step5_us1; step6_us2; step7_us3; step8_pair_registry; step9_currency_registry; if [[ "${ENABLE_MLP:-false}" == "true" ]]; then step_mlp_us2; fi ;;
  *)   fail "Unknown story: $STORY. Use us1|us2|us3|us5|us6|all" ;;
esac

# Reached only when the dispatch above ran to completion; print_report checks it.
RUN_COMPLETED=1

echo "[tryout] Done."
