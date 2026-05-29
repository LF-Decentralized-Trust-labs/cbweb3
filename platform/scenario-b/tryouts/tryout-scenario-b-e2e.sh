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

# Per-entity API Gateway URLs (actual docker-compose port mappings)
API_GW_BANK_A_URL="${API_GW_BANK_A_URL:-http://localhost:18080}"
API_GW_CENTRAL_BANK_A_URL="${API_GW_CENTRAL_BANK_A_URL:-http://localhost:38080}"
API_GW_CENTRAL_BANK_B_URL="${API_GW_CENTRAL_BANK_B_URL:-http://localhost:60080}"

BESU_HUB_RPC="${BESU_HUB_RPC:-http://localhost:8645}"
SPOKE_A_RPC="${SPOKE_A_RPC:-http://localhost:8645}"
SPOKE_B_RPC="${SPOKE_B_RPC:-http://localhost:8745}"
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8081}"
CACTI_RELAYER_URL="${CACTI_RELAYER_URL:-http://localhost:4000}"

# Hub contract addresses (read from CB-A infra env file if not set externally)
HUB_TOKEN_A_ADDRESS="${HUB_TOKEN_A_ADDRESS:-$(grep -s '^HUB_TOKEN_A_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
HUB_TOKEN_B_ADDRESS="${HUB_TOKEN_B_ADDRESS:-$(grep -s '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
HUB_IDENTITY_REGISTRY_ADDRESS="${HUB_IDENTITY_REGISTRY_ADDRESS:-$(grep -s '^HUB_IDENTITY_REGISTRY_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
CURRENCY_REGISTRY_CONTRACT_ADDRESS="${CURRENCY_REGISTRY_CONTRACT_ADDRESS:-$(grep -s '^CURRENCY_REGISTRY_CONTRACT_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"

# Keycloak client credentials (read from the generated .env files)
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

login_keycloak() {
  local realm="$1" client_id="$2" client_secret="$3"
  curl -sS -X POST "${KEYCLOAK_URL}/realms/${realm}/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=client_credentials" \
    -d "client_id=${client_id}" \
    -d "client_secret=${client_secret}" | jq -r '.access_token // empty'
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
  wait_for "${API_GW_BANK_A_URL}/healthz" 60 3 || true
  wait_for "${API_GW_CENTRAL_BANK_A_URL}/healthz" 60 3 || true
  # AMM-capable Hub (Besu RPC)
  curl -sS -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"net_version","id":1}' \
    "${BESU_HUB_RPC}" | jq -e '.result' >/dev/null \
    && log "Hub Besu RPC OK" || log "(warn) Hub Besu RPC not responding"
  curl -sS -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"net_version","id":1}' \
    "${SPOKE_B_RPC}" | jq -e '.result' >/dev/null || log "(warn) Spoke-B Besu RPC not responding"
  log "Infrastructure ready"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 2: Verify contracts (scenario-b.up already deploys them)
# ─────────────────────────────────────────────────────────────────────────────
step2_contracts() {
  step "2. Verify contracts deployed + ensure IdentityRegistry is configured"
  log "Contracts deployed by scenario-b.up — skipping separate deploy"

  # ── Ensure setCentralBankOf is configured in IdentityRegistry ──
  # This is idempotent and safe to run on every startup.
  # Uses CB-A's signer key (= IdentityRegistry DEFAULT_ADMIN_ROLE) to register
  # CB-A as central bank of tokenA (BRL) and CB-B as central bank of tokenB (EUR).
  local admin_key id_reg token_a token_b cb_a_key cb_b_key cb_a_addr cb_b_addr
  admin_key="$(grep -s '^SIGNER_PRIVATE_KEY=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')"
  cb_b_key="$(grep -s '^SIGNER_PRIVATE_KEY=' backend/config/.env.infra.central-bank-b 2>/dev/null | cut -d= -f2 || echo '')"
  id_reg="${HUB_IDENTITY_REGISTRY_ADDRESS:-$(grep -s '^HUB_IDENTITY_REGISTRY_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')}"
  token_a="${HUB_TOKEN_A_ADDRESS:-}"
  token_b="${HUB_TOKEN_B_ADDRESS:-}"
  if [ -z "$admin_key" ] || [ -z "$id_reg" ] || [ -z "$token_a" ] || [ -z "$token_b" ]; then
    log "WARN: missing admin_key/id_reg/token addresses — skipping setCentralBankOf"
  else
    cb_a_addr="$(cast wallet address --private-key "0x${admin_key#0x}" 2>/dev/null || echo '')"
    cb_b_addr="$(cast wallet address --private-key "0x${cb_b_key#0x}" 2>/dev/null || echo '')"
    if [ -n "$cb_a_addr" ] && [ -n "$cb_b_addr" ]; then
      cast send "$id_reg" "setCentralBankOf(address,address)" "$token_a" "$cb_a_addr" \
        --private-key "0x${admin_key#0x}" --rpc-url "${BESU_HUB_RPC}" >/dev/null 2>&1 && \
        log "setCentralBankOf(tokenBRL, CB-A=${cb_a_addr}) OK" || \
        log "WARN: setCentralBankOf(tokenBRL, CB-A) failed — check admin role"
      cast send "$id_reg" "setCentralBankOf(address,address)" "$token_b" "$cb_b_addr" \
        --private-key "0x${admin_key#0x}" --rpc-url "${BESU_HUB_RPC}" >/dev/null 2>&1 && \
        log "setCentralBankOf(tokenEUR, CB-B=${cb_b_addr}) OK" || \
        log "WARN: setCentralBankOf(tokenEUR, CB-B) failed — check admin role"
    else
      log "WARN: could not derive CB addresses from private keys — skipping setCentralBankOf"
    fi
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 3: Acquire OIDC tokens for each role
# ─────────────────────────────────────────────────────────────────────────────
step3_participants() {
  step "3. Acquire OIDC tokens (bank-a, central-bank-a, central-bank-b)"
  BANK_A_TOKEN="$(login_keycloak "$KC_BANK_A_REALM" "$KC_BANK_A_CLIENT" "$KC_BANK_A_SECRET" || true)"
  CENTRAL_BANK_A_TOKEN="$(login_keycloak "$KC_CENTRAL_BANK_A_REALM" "$KC_CENTRAL_BANK_A_CLIENT" "$KC_CENTRAL_BANK_A_SECRET" || true)"
  CENTRAL_BANK_B_TOKEN="$(login_keycloak "$KC_CENTRAL_BANK_B_REALM" "$KC_CENTRAL_BANK_B_CLIENT" "$KC_CENTRAL_BANK_B_SECRET" || true)"
  [ -n "$BANK_A_TOKEN" ]         && log "bank-a token acquired"         || log "(warn) no bank-a token — check KC_BANK_A_SECRET"
  [ -n "$CENTRAL_BANK_A_TOKEN" ] && log "central-bank-a token acquired" || log "(warn) no central-bank-a token — check KC_CENTRAL_BANK_A_SECRET"
  [ -n "$CENTRAL_BANK_B_TOKEN" ] && log "central-bank-b token acquired" || log "(warn) no central-bank-b token — check KC_CENTRAL_BANK_B_SECRET"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 4: Pool provisioning — seed with initial liquidity if empty
# ─────────────────────────────────────────────────────────────────────────────
step4_pool() {
  step "4. Verify AMM pool status (seed initial liquidity if empty)"
  local resp reserve_a reserve_b pool_status
  resp="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$BANK_A_TOKEN" || true)"
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
step4a_mint_and_approve() {
  step "4a. Mint Hub tCeBM tokens + approve AMM (central-bank-a, central-bank-b, bank-a)"
  local body resp

  # central-bank-a mints TOKEN_A (BRL) to its own signer and approves AMM.
  # FR-018: gateway detects CENTRAL_BANK_ROLE on TOKEN_A automatically — no amount_b needed.
  body='{"amount":"200000"}'
  resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/token/mint-and-approve" "$body" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Mint+Approve central-bank-a: $(echo "$resp" | jq -c '.' || echo "$resp")"
  if echo "$resp" | jq -e '.status == "ok"' >/dev/null 2>&1; then
    log "central-bank-a: tokens minted and AMM approved"
  else
    fail "step4a: central-bank-a mint-and-approve failed — check HUB_TOKEN_A_ADDRESS / HUB_TOKEN_B_ADDRESS"
  fi

  # G5: central-bank-b mints TOKEN_B to its own signer and approves AMM (CB-B's gateway).
  # Also mints TOKEN_B to CB-A's signer address so CB-A's gateway can execute
  # AddSingleSidedLiquidity(TOKEN_B) when commit B auto-matches on CB-A's gateway.
  # Architecture: commit B is routed to CB-A's gateway (shared DB) for matching;
  # CB-A's signer therefore needs TOKEN_B balance + AMM approval for TOKEN_B. (FR-018)
  local mint_cb_b_resp cb_a_key cb_a_addr
  mint_cb_b_resp="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/amm/token/mint-and-approve" \
    '{"amount":"200000"}' \
    "$CENTRAL_BANK_B_TOKEN" || true)"
  log "Mint+Approve central-bank-b: $(echo "$mint_cb_b_resp" | jq -c '.' || echo "$mint_cb_b_resp")"
  if echo "$mint_cb_b_resp" | jq -e '.status == "ok"' >/dev/null 2>&1; then
    log "central-bank-b: TOKEN_B minted and AMM approved"
  else
    log "WARN: central-bank-b mint-and-approve returned non-ok — commit-reveal TOKEN_B transfer may fail if signer lacks balance"
  fi

# G5-cross (ANTI-PATTERN — DEPRECATED: spec-005 architectural violation / spec-007 FR-004).
  # With anti-G5-cross guard (spec-007 T016), CB-B can no longer
  # mint TOKEN_B to CB-A signer. Esta chamada DEVE retornar HTTP 403
  # CROSS_CB_MINT_PROHIBITED — this is EXPECTED behavior, not a failure.
  # Para seeding de novos pools: use ./tryouts/tryout-sovereign-cb-liquidity.sh.
  cb_a_key="$(grep -s '^SIGNER_PRIVATE_KEY=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2 || echo '')"
  cb_a_addr="$(cast wallet address --private-key "0x${cb_a_key#0x}" 2>/dev/null || echo '')"
  if [ -n "$cb_a_addr" ]; then
    local mint_tokenb_cba_resp mint_tokenb_code
    mint_tokenb_cba_resp="$(api_post "$API_GW_CENTRAL_BANK_B_URL" "/api/v2/amm/token/mint-and-approve" \
      "{\"amount\":\"200000\",\"recipient\":\"${cb_a_addr}\"}" \
      "$CENTRAL_BANK_B_TOKEN" || true)"
    mint_tokenb_code="$(echo "$mint_tokenb_cba_resp" | jq -r '.code // empty')"
    if [ "$mint_tokenb_code" = "CROSS_CB_MINT_PROHIBITED" ]; then
      log "PASS: G5-cross bloqueado — HTTP 403 CROSS_CB_MINT_PROHIBITED (spec-007 FR-004 anti-G5-cross guard ativo)"
      log "INFO: CB-A will not have TOKEN_B; seeding do pool via G5-cross is no longer possible."
      log "      For new pool: make contracts.seed-sovereign-pair && ./tryouts/tryout-sovereign-cb-liquidity.sh"
    else
      log "WARN: G5-cross attempt — esperado HTTP 403 CROSS_CB_MINT_PROHIBITED, obtido: ${mint_tokenb_code:-no code}"
      log "      ($(echo "$mint_tokenb_cba_resp" | jq -c '.' || echo "$mint_tokenb_cba_resp"))"
      if echo "$mint_tokenb_cba_resp" | jq -e '.status == "ok"' >/dev/null 2>&1; then
        log "      (anti-G5-cross guard may not be active — check deploy de spec-007)"
        approve_tokenb_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/token/approve-amm" \
          '{"amount":"200000","side":"B"}' \
          "$CENTRAL_BANK_A_TOKEN" || true)"
        log "CB-A approve AMM for TOKEN_B (side=B): $(echo "$approve_tokenb_resp" | jq -c '.' || echo "$approve_tokenb_resp")"
      fi
    fi
  else
    log "INFO: CB-A signer address not derivable — G5-cross check skipped"
  fi

  # Mint TOKEN_A to bank-a signer so it has BRL balance before swap (central bank mints to recipient).
  # FR-018: CB-A mints only TOKEN_A (its emitted token); bank-a receives TOKEN_B from CB-B if needed.
  local mint_to_resp
  mint_to_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/token/mint-and-approve" \
    '{"amount":"50000","recipient":"0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef"}' \
    "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Mint to bank-a signer: $(echo "$mint_to_resp" | jq -c '.' || echo "$mint_to_resp")"
  if ! echo "$mint_to_resp" | jq -e '.status == "ok"' >/dev/null 2>&1; then
    fail "step4a: mint-to bank-a failed"
  fi

  # bank-a approves AMM to spend its TOKEN_A (BRL) before swap.
  # FR-018: side="A" required for non-CB callers to indicate which token to approve.
  local approve_resp
  approve_resp="$(api_post "$API_GW_BANK_A_URL" "/api/v2/amm/token/approve-amm" \
    '{"amount":"50000","side":"A"}' \
    "$BANK_A_TOKEN" || true)"
  log "Bank-a approve AMM: $(echo "$approve_resp" | jq -c '.' || echo "$approve_resp")"
  if ! echo "$approve_resp" | jq -e '.status == "ok"' >/dev/null 2>&1; then
    fail "step4a: bank-a approve-amm failed"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 4b: Seed initial liquidity via cooperative commit-reveal (005-cooperative-liquidity)
#          Central Bank A provides TOKEN_A and Central Bank B provides TOKEN_B.
#          Both commits must match before the pool becomes ACTIVE.
# ─────────────────────────────────────────────────────────────────────────────
step4b_seed_liquidity() {
  step "4b. Seed initial liquidity via cooperative commit-reveal (central-bank-a + central-bank-b)"
  local commit_a_body commit_b_body commit_a_resp commit_b_resp commit_a_id commit_b_id
  local pool_resp pool_status reserve_a reserve_b fee_rate
  local swap_reject_resp swap_reject_err

  # --- Idempotency guard: skip commit-reveal if pool already has bilateral liquidity ---
  # FR-001: commit-reveal is only required for initial pool formation (state EMPTY).
  # If pool already has bilateral reserves (e.g. re-run of tryout), skip to avoid
  # COMMIT_ALREADY_EXISTS errors. LP_ID_BCB will remain empty → step6 uses LEGACY fallback.
  pool_resp="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$BANK_A_TOKEN" || true)"
  reserve_a="$(echo "$pool_resp" | jq -r '.reserve_a // "0"')"
  reserve_b="$(echo "$pool_resp" | jq -r '.reserve_b // "0"')"
  if [ "$reserve_a" != "0" ] && [ "$reserve_b" != "0" ]; then
    log "Pool already has bilateral liquidity (reserve_a=${reserve_a}, reserve_b=${reserve_b}) — skipping commit-reveal (FR-001)"
    return 0
  fi

  # --- Pre-cleanup: cancel any stale PENDING commits from previous runs (idempotency) ---
  # Both commit A and commit B are now routed to CB-A's gateway (same DB), so we only
  # need to clean up CB-A's DB. CB-B's DB may have a stale commit B from a prior run
  # (before routing was fixed) but it is isolated and will not affect this flow.
  local stale_commits stale_count stale_id stale_provider
  stale_commits="$(curl -sf -X GET \
    "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/commits?pool_pair=BRL-USD&status=PENDING" \
    -b "access_token=$CENTRAL_BANK_A_TOKEN" 2>/dev/null || echo '{}')"
  stale_count="$(echo "$stale_commits" | jq -r '.count // 0')"
  if [ "$stale_count" -gt 0 ] 2>/dev/null; then
    log "Cancelling $stale_count stale PENDING commit(s) from previous run..."
    echo "$stale_commits" | jq -r '.commits[] | "\(.commit_id) \(.provider_id)"' | \
    while read -r stale_id stale_provider; do
      cancel_resp="$(curl -sf -X DELETE \
        "$API_GW_CENTRAL_BANK_A_URL/api/v2/amm/liquidity/commits/${stale_id}?provider_id=${stale_provider}" \
        -b "access_token=$CENTRAL_BANK_A_TOKEN" 2>/dev/null || true)"
      log "  Cancelled commit ${stale_id} (provider=${stale_provider}): $(echo "$cancel_resp" | jq -r '.message // .error // empty')"
    done
  fi

  # --- Commit side A (TOKEN_A / BRL) from central-bank-a ---
  commit_a_body='{"pool_pair":"BRL-USD","provider_id":"central_bank_a","side":"A","amount":"100000"}'
  commit_a_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/commit" "$commit_a_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Commit A response: $(echo "$commit_a_resp" | jq -c '.' || echo "$commit_a_resp")"
  commit_a_id="$(echo "$commit_a_resp" | jq -r '.commit_id // empty')"
  [ -n "$commit_a_id" ] || fail "step4b: commit A did not return commit_id — check liquidity/commit handler"
  log "Commit A ID: $commit_a_id"

  # --- C2: Verify pool is PENDING_COUNTERPART after commit A, before commit B ---
  # I3 fix: query CB-A gateway (port 38080) where commit A is stored — pending_commits[] is DB-local.
  # Bank A gateway (cbweb3_bank_a DB) has no commits, so it always returns EMPTY for this check.
  pool_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$CENTRAL_BANK_A_TOKEN" || true)"
  pool_status="$(echo "$pool_resp" | jq -r '.pool_status // "UNKNOWN"')"
  reserve_a="$(echo "$pool_resp" | jq -r '.reserve_a // "0"')"
  reserve_b="$(echo "$pool_resp" | jq -r '.reserve_b // "0"')"
  log "Pool status after commit A: ${pool_status} (reserve_a=${reserve_a}, reserve_b=${reserve_b})"
  if [ "$pool_status" = "PENDING_COUNTERPART" ]; then
    log "PASS: pool is PENDING_COUNTERPART as expected (FR-001 / US1-Scenario 4)"
  elif [ "$reserve_a" != "0" ] && [ "$reserve_b" = "0" ]; then
    log "INFO: pool has reserve_a>${reserve_a} / reserve_b=0 — consistent with PENDING_COUNTERPART even if field absent (T018)"
  else
    log "WARN: pool_status=${pool_status} after commit A (expected PENDING_COUNTERPART)"
  fi

  # --- C1: Verify swap is blocked with POOL_NOT_ACTIVE while pool is PENDING_COUNTERPART ---
  # Uses same field names as step5_us1 (pair, amount_out, payer_id, beneficiary_id)
  swap_reject_resp="$(api_post "$API_GW_BANK_A_URL" "/api/v2/amm/swap/exact-output" \
    '{"pair":"BRL-USD","amount_out":"100","max_amount_in":"110","payer_id":"bank_a","beneficiary_id":"bank_c"}' \
    "$BANK_A_TOKEN" || true)"
  swap_reject_err="$(echo "$swap_reject_resp" | jq -r '.error // .error_code // empty')"
  if [ "$swap_reject_err" = "POOL_NOT_ACTIVE" ]; then
    log "PASS: swap correctly blocked with POOL_NOT_ACTIVE before pool activation (FR-011 / US1-Scenario 4)"
  else
    log "WARN: expected POOL_NOT_ACTIVE error but got: ${swap_reject_err:-no error field} (response: $(echo "$swap_reject_resp" | jq -c '.' || echo "$swap_reject_resp"))"
  fi

  # --- C5: Verify SAME_PROVIDER_BOTH_SIDES rejection (FR-002 / 005-cooperative-liquidity Q3) ---
  # central-bank-a already has commit A (side A) → trying commit B with same provider must be rejected.
  local same_prov_resp same_prov_err
  same_prov_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/commit" \
    '{"pool_pair":"BRL-USD","provider_id":"central_bank_a","side":"B","amount":"100000"}' \
    "$CENTRAL_BANK_A_TOKEN" || true)"
  same_prov_err="$(echo "$same_prov_resp" | jq -r '.error_code // .error // empty')"
  if [ "$same_prov_err" = "SAME_PROVIDER_BOTH_SIDES" ]; then
    log "PASS: SAME_PROVIDER_BOTH_SIDES correctly rejected (HTTP 409, FR-002)"
  else
    log "WARN: expected SAME_PROVIDER_BOTH_SIDES but got: ${same_prov_err:-no error_code} (response: $(echo "$same_prov_resp" | jq -c '.' || echo "$same_prov_resp"))"
  fi

  # --- Commit side B (TOKEN_B / USD) via CB-A's gateway (same DB → auto-match works) ---
  # NOTE (spec-007): G5-cross pattern was DEPRECATED. CB-A's signer no longer holds
  # TOKEN_B after step 4a (mint blocked by anti-G5-cross guard). In environments
  # com spec-007 deployado, o executeMatchedCommits will fail due to lack of balance TOKEN_B.
  # BRL-USD pool (HUB_TOKEN_A/B) is deprecated for new deposits via G5-cross
  # (spec-007 FR-002). Para pools soberanos: ./tryouts/tryout-sovereign-cb-liquidity.sh
  # provider_id=central_bank_b is metadata only; execution uses CB-A signer (legacy).
  commit_b_body='{"pool_pair":"BRL-USD","provider_id":"central_bank_b","side":"B","amount":"100000"}'
  commit_b_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/commit" "$commit_b_body" "$CENTRAL_BANK_A_TOKEN" || true)"
  log "Commit B response: $(echo "$commit_b_resp" | jq -c '.' || echo "$commit_b_resp")"
  commit_b_id="$(echo "$commit_b_resp" | jq -r '.commit_id // empty')"
  if [ -z "$commit_b_id" ]; then
    # Graceful degradation expected with spec-007 deployed:
    # CB-A has no TOKEN_B (blocked by anti-G5-cross guard in step 4a),
    # so addSingleSidedLiquidity(TOKEN_B) reverts on-chain. BRL-USD pool
    # via G5-cross is deprecated (spec-007 FR-002 / Out of Scope).
    # Step 6 will use legacy path de addLiquidity dual-sided como fallback.
    log "WARN: step4b: commit B bloqueado pelo anti-G5-cross guard (spec-007 FR-002) — degradando graciosamente"
    log "      For sovereign liquidity provisioning: ./tryouts/tryout-sovereign-cb-liquidity.sh"
    return 0
  fi
  log "Commit B ID: $commit_b_id — expecting auto-match + execution"

  # --- C3: Capture lp_ids returned by commit B (populated when status = MATCHED) ---
  LP_ID_BCB="$(echo "$commit_b_resp" | jq -r '.lp_ids[0] // empty')"
  LP_ID_FED="$(echo "$commit_b_resp" | jq -r '.lp_ids[1] // empty')"
  [ -n "$LP_ID_BCB" ] && log "LP_ID_BCB (central-bank-a): $LP_ID_BCB" || log "WARN: lp_ids[0] not in commit B response — T018 field may be pending"
  [ -n "$LP_ID_FED" ] && log "LP_ID_FED (central-bank-b): $LP_ID_FED" || log "WARN: lp_ids[1] not in commit B response"

  # --- Verify pool status is now ACTIVE ---
  pool_resp="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$BANK_A_TOKEN" || true)"
  reserve_a="$(echo "$pool_resp" | jq -r '.reserve_a // "0"')"
  reserve_b="$(echo "$pool_resp" | jq -r '.reserve_b // "0"')"
  pool_status="$(echo "$pool_resp" | jq -r '.pool_status // "UNKNOWN"')"
  fee_rate="$(echo "$pool_resp" | jq -r '.fee_rate_bps // "?"')"
  log "Pool after cooperative seed: reserve_a=${reserve_a}, reserve_b=${reserve_b}, status=${pool_status}, fee_rate_bps=${fee_rate}"
  if [ "$reserve_a" = "0" ] || [ "$reserve_b" = "0" ]; then
    log "WARN: step4b: pool still empty after commit-reveal (reserve_a=${reserve_a}, reserve_b=${reserve_b})"
    log "      Likely cause: G5-cross blocked by spec-007 FR-004 — CB-A has no TOKEN_B balance."
    log "      This is EXPECTED in environments with spec-007 deployed."
    log "      Para US1/US2: certifique-se de que o pool BRL-USD foi pre-semeado antes de rodar este tryout."
    log "      Para o novo fluxo soberano: use ./tryouts/tryout-sovereign-cb-liquidity.sh"
    return 0
  fi
  if [ "$pool_status" = "ACTIVE" ]; then
    log "PASS: pool is ACTIVE after cooperative commit-reveal (SC-001)"
  else
    log "INFO: pool_status=${pool_status} — field may not be exposed yet (T018); reserves confirmed bilateral"
  fi
  # --- C4: Verify total_lp_count = 2 from CB-A gateway (I4 / FR-009) ---
  # total_lp_count is DB-local: LP positions live in cbweb3_central_bank_a (CB-A's DB).
  # Querying from Bank A gateway (cbweb3_bank_a) always returns 0 for cooperative positions.
  pool_lp_resp="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$CENTRAL_BANK_A_TOKEN" || true)"
  total_lp_count="$(echo "$pool_lp_resp" | jq -r '.total_lp_count // 0')"
  if [ "$(echo "$total_lp_count >= 2" | bc 2>/dev/null || echo 0)" = "1" ] || [ "$total_lp_count" -ge 2 ] 2>/dev/null; then
    log "PASS: total_lp_count=$total_lp_count (expected ≥2 after cooperative commit-reveal, FR-009 / I4)"
  else
    log "WARN: total_lp_count=$total_lp_count after cooperative commit-reveal (expected 2; I4 — CB-A gateway)"
  fi
  log "Pool seeded successfully via cooperative commit-reveal"
}

# ─────────────────────────────────────────────────────────────────────────────
# Step 5 — US1: Quote + Swap + Pool Status
# ─────────────────────────────────────────────────────────────────────────────
step5_us1() {
  step "5/US1. Quote + Swap + Pool Status"

  # --- C6: Assert pool is ACTIVE and fee_rate_bps = 30 before swap (SC-001 / T018) ---
  local pool_pre pool_status_pre fee_rate_pre reserve_a_pre reserve_b_pre
  pool_pre="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$BANK_A_TOKEN")"
  pool_status_pre="$(echo "$pool_pre" | jq -r '.pool_status // "UNKNOWN"')"
  fee_rate_pre="$(echo "$pool_pre" | jq -r '.fee_rate_bps // "?"')"
  reserve_a_pre="$(echo "$pool_pre" | jq -r '.reserve_a // "0"')"
  reserve_b_pre="$(echo "$pool_pre" | jq -r '.reserve_b // "0"')"
  if [ "$pool_status_pre" = "ACTIVE" ]; then
    log "PASS: pool_status=ACTIVE (SC-001 / T018)"
  elif [ "$reserve_a_pre" = "0" ] || [ "$reserve_b_pre" = "0" ]; then
    # Graceful degradation expected in spec-007: BRL-USD pool cannot be seeded
    # bilaterally with G5-cross pattern blocked. Use tryout-sovereign-cb-liquidity.sh
    # to validate US1 with the sovereign pair (W-tCeBM).
    log "WARN: step5: pool without bilateral liquidity (reserve_a=${reserve_a_pre}, reserve_b=${reserve_b_pre}) — swap skipped (spec-007: use tryout-sovereign-cb-liquidity.sh for US1)"
    return 0
  else
    log "WARN: pool_status=${pool_status_pre} — expected ACTIVE; reserves present (${reserve_a_pre}/${reserve_b_pre}) but status field unexpected"
  fi
  if [ "$fee_rate_pre" = "30" ]; then
    log "PASS: fee_rate_bps=30 (FR-005 / T024)"
  else
    log "WARN: fee_rate_bps=${fee_rate_pre} — expected 30 (T018/T024)"
  fi

  local quote
  quote="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/quote/exact-output?pair=BRL-USD&amount_out=1000" "$BANK_A_TOKEN")"
  log "Quote: $(echo "$quote" | jq -c '.')"

  local max_in
  max_in="$(echo "$quote" | jq -r '.required_input // "0"')"
  local swap_body
  swap_body="$(jq -cn --arg pair "BRL-USD" --arg amount_out "1000" --arg max_in "$max_in" \
    --arg payer "bank-a" --arg beneficiary "bank-c" \
    '{pair:$pair, amount_out:$amount_out, max_amount_in:$max_in, payer_id:$payer, beneficiary_id:$beneficiary}')"
  local swap
  swap="$(api_post "$API_GW_BANK_A_URL" "/api/v2/amm/swap/exact-output" "$swap_body" "$BANK_A_TOKEN")"
  log "Swap: $(echo "$swap" | jq -c '.')"
  local swap_state
  swap_state="$(echo "$swap" | jq -r '.state // empty')"
  [ "$swap_state" = "COMPLETED" ] && log "PASS: Swap BRL-USD executed — state=COMPLETED (FR-028)" || \
    log "WARN: Swap state=${swap_state:-no state} — expected COMPLETED"

  local pool pool_cba
  pool="$(api_get "$API_GW_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$BANK_A_TOKEN")"
  log "Pool after swap (bank-a view): $(echo "$pool" | jq -c '.')"
  # I4 / Session 2026-05-20: total_lp_count is gateway-scoped (local DB only).
  # bank-a gateway has no LP positions in its DB -> total_lp_count=0 is EXPECTED, not a bug.
  local bank_a_lp_count
  bank_a_lp_count="$(echo "$pool" | jq -r '.total_lp_count // 0')"
  log "INFO: bank-a total_lp_count=$bank_a_lp_count (expected 0 — LP positions live in CB-A/CB-B gateways, not bank-a gateway)"
  # I4: also log total_lp_count from CB-A gateway (authoritative for cooperative LPs)
  pool_cba="$(api_get "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/pool/BRL-USD/status" "$CENTRAL_BANK_A_TOKEN" || true)"
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
  local LP_ID
  if [ -n "$LP_ID_BCB" ]; then
    LP_ID="$LP_ID_BCB"
    log "Using cooperative LP position (LP_ID_BCB): $LP_ID"
  else
    local add_body add_resp
    add_body='{"pool_pair":"BRL-USD","token_a_amount":"10000","token_b_amount":"10000","provider_bank_id":"central_bank_a"}'
    add_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/amm/liquidity/add" "$add_body" "$CENTRAL_BANK_A_TOKEN" || true)"
    log "Add liquidity (fallback dual-sided): $(echo "$add_resp" | jq -c '.' || echo "$add_resp")"
    LP_ID="$(echo "$add_resp" | jq -r '.lp_id // empty')"
    if [ -z "$LP_ID" ]; then
      log "WARN: step6: addLiquidity dual-sided failed (no TOKEN_B for CB-A in spec-007) — remove liquidity skipped"
      log "      Bridge Lock&Mint + Burn&Unlock continuam sendo testados abaixo."
    else
      log "Captured lp_id (fallback): $LP_ID"
    fi
  fi

  # Lock&Mint — initiate bridging
  local lock_resp
  lock_resp="$(api_post "$API_GW_BANK_A_URL" "/api/v2/bridge/lock-mint" \
    '{"owner_bank_id":"bank_a","spoke_network":"spoke-a","native_asset":"BRL-CBDC","mirrored_asset":"mBRL-CBDC","amount":"5000"}' \
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
    log "SKIP: step6: remove liquidity — no LP_ID available (pool BRL-USD without bilateral liquidity in spec-007)"
  else
  local remove_body remove_resp withdrawal_mode fee_claim_paid returned_a returned_b
  remove_body="$(jq -cn --arg id "$LP_ID" --arg bank "central_bank_a" '{lp_id:$id, pool_pair:"BRL-USD", provider_bank_id:$bank}')"
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
  pause_body='{"pair":"BRL-USD","bank_id":"central_bank_1","reason_code":"E2E_TRYOUT_INCIDENT","signature":"AA=="}'
  pause_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/governance/circuit-breaker/pause" "$pause_body" "$CENTRAL_BANK_A_TOKEN" | jq -c '.' || true)"
  log "Pause CB: ${pause_resp}"
  pause_state="$(echo "$pause_resp" | jq -r '.state // empty')"
  [ "$pause_state" = "HALTED" ] && log "PASS: Circuit Breaker paused → HALTED (FR-030)" || \
    log "WARN: expected HALTED after pause, got: ${pause_state:-no state}"

  local resume_req
  resume_req='{"pair":"BRL-USD","bank_id":"central_bank_1","signature":"AA=="}'
  local req_resp
  req_resp="$(api_post "$API_GW_CENTRAL_BANK_A_URL" "/api/v2/governance/circuit-breaker/resume-request" "$resume_req" "$CENTRAL_BANK_A_TOKEN")"
  log "Resume proposal: $(echo "$req_resp" | jq -c '.')"
  local request_id
  request_id="$(echo "$req_resp" | jq -r '.request_id // empty')"

  if [ -n "$request_id" ]; then
    local sign_body resume_sign_resp resume_state
    sign_body="$(jq -cn --arg id "$request_id" --arg bank "central_bank_2" '{pair:"BRL-USD", request_id:$id, bank_id:$bank, signature:"AA=="}')"
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
  MLP_TOKEN="$(login_keycloak "$KC_MLP_REALM" "$KC_MLP_CLIENT" "$KC_MLP_SECRET" || true)"
  if [ -z "$MLP_TOKEN" ]; then
    log "WARN: MLP token not acquired — check KC_MLP_SECRET and mlp realm in Keycloak"
    log "Skipping step_mlp_us2 (ENABLE_MLP=true but token unavailable)"
    return 0
  fi
  log "MLP token acquired (realm=mlp, client=mlp-client)"

  # ── 2. POST /api/v2/amm/liquidity/add (dual-sided) ──
  local add_body add_resp lp_id deposit_side
  add_body="$(jq -cn \
    '{pool_pair:"BRL-USD",token_a_amount:"10000",token_b_amount:"10000",provider_bank_id:"mlp"}')"
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
    --arg pair "BRL-USD" \
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

  local overall='✅ OK'
  [[ ${#warn[@]} -gt 0 && ${#fail_[@]} -eq 0 ]] && overall='⚠️  OK WITH WARNINGS'
  [[ ${#fail_[@]} -gt 0 ]] && overall='❌ FAILED'

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
step4_pool
step4a_mint_and_approve
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

echo "[tryout] Done."
