#!/usr/bin/env bash
# =============================================================================
# debug-cb-liquidity-injection.sh
# Step-by-step diagnostic for sovereign liquidity injection flow (Scenario B)
#
# 008-fix-cb-liquidity UPDATE: Uses simplified API
#   - Lock&Mint: payload {"amount": "..."} only (API derives owner_bank_id, spoke_network, native_asset, mirrored_asset)
#   - Commit: payload {"pool_pair": "...", "amount": "..."} only (API derives provider_id, side, w_token_address)
#
# GOAL: Run each flow step and capture full responses
#       to identify where a failure occurs.
#
# FLOW COVERED:
#   [0] Pre-checks: services up, Keycloak reachable, containers running
#   [1] Authentication: Keycloak tokens for CB-A and CB-B
#   [2] Mint Hub W-tCeBM: mint tokens on each CB signer + approve AMM
#   [3] Bridge Lock&Mint: lock native asset + mint mirrored asset on Hub
#   [4] Poll bridge_state=ACTIVE: wait for Cacti Relayer confirmation
#   [5] On-chain commit: bilateral registration on LiquidityCommitRegistry
#   [6] Poll commit status=EXECUTED: wait for CommitMatched watcher
#   [7] Pool verification: confirm pool_status=ACTIVE and LP count (each CB keeps positions locally)
#   [8] Anti-G5-cross guard: ensure legacy path returns 403
#   [9] Diagnostic summary
#
# USAGE:
#   bash tryouts/debug-cb-liquidity-injection.sh
#
# ENVIRONMENT VARIABLES (all have working defaults for local dev):
#   CB_A_URL              CB-A API Gateway URL (default: http://localhost:38080)
#   CB_B_URL              CB-B API Gateway URL (default: http://localhost:60080)
#   CB_A_ENV              CB-A .env file (default: backend/config/.env.infra.central-bank-a)
#   CB_B_ENV              CB-B .env file (default: backend/config/.env.infra.central-bank-b)
#   POOL_PAIR             Pool pair (default: W-BRL-ARS)
#   AMOUNT_A              CB-A liquidity amount in wei (default: 100000000000000000000000)
#   AMOUNT_B              CB-B liquidity amount in wei (default: 200000000000000000000000)
#   CB_B_HUB_SIGNER       CB-B Ethereum address on Hub (for anti-G5 check)
#   SOVEREIGN_AMM_ADDR    Sovereign AMM contract on Hub (for cast logs)
#   HUB_RPC_URL           Hub Besu RPC (default: http://localhost:8645)
#   SKIP_MINT             If "true" (default), skip Phase 2 — with BesuRelayerExecutor mint is automatic
#   SKIP_LOCK_MINT        If "true", skip Phase 3 and use existing bridge positions
#   SKIP_COMMIT           If "true", skip Phase 5 and use existing commits
#   VERBOSE               If "true", print full HTTP responses at each step
# =============================================================================

set -uo pipefail
# NOTE: we intentionally omit 'set -e' — continue on failures
#          to capture as much diagnostic data as possible.

# ─── Cores e helpers de output ────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[1;34m'; NC='\033[0m'
BOLD='\033[1m'

ok()      { echo -e "${GREEN}${BOLD}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}${BOLD}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}${BOLD}[ERROR]${NC} $*"; ERROS+=("$*"); }
info()    { echo -e "${BLUE}  →${NC} $*"; }
titulo()  { echo ""; echo -e "${BOLD}════════════════════════════════════════════════════${NC}"; echo -e "${BOLD}  $*${NC}"; echo -e "${BOLD}════════════════════════════════════════════════════${NC}"; }
sep()     { echo -e "${BLUE}────────────────────────────────────────────────────${NC}"; }
raw()     {
  # Print raw response when VERBOSE=true — always to stderr to avoid polluting $(...) captures
  if [[ "${VERBOSE:-false}" == "true" ]]; then
    echo -e "${YELLOW}  [RAW] $*${NC}" >&2
  fi
}

# Accumulate errors for final summary
ERROS=()

# Temp file to propagate HTTP code from subshells (api_post/api_get)
# run in subshell via $(); HTTP code is persisted to disk.
_HTTP_CODE_FILE=$(mktemp)
trap 'rm -f "${_HTTP_CODE_FILE}"' EXIT

# Read HTTP code from last api_post/api_get call.
# Required because these functions run in subshell ($()) and cannot export vars.
_http_code() { cat "${_HTTP_CODE_FILE}" 2>/dev/null || echo "?"; }

# ─── Default configuration ──────────────────────────────────────────────────────
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
HUB_RPC_URL="${HUB_RPC_URL:-http://localhost:8645}"
SOVEREIGN_AMM_ADDR="${SOVEREIGN_AMM_ADDR:-}"
CB_B_HUB_SIGNER="${CB_B_HUB_SIGNER:-}"
SKIP_MINT="${SKIP_MINT:-true}"
SKIP_LOCK_MINT="${SKIP_LOCK_MINT:-false}"
SKIP_COMMIT="${SKIP_COMMIT:-false}"
VERBOSE="${VERBOSE:-false}"

# ─── Utility functions ──────────────────────────────────────────────────────

# Load .env variables without overwriting already-set variables
load_env_file() {
  local arquivo="$1"
  if [[ -f "$arquivo" ]]; then
    # Export all variables from file
    set -a
    # shellcheck disable=SC1090
    source "$arquivo"
    set +a
    info "Env file loaded: $arquivo"
  else
    warn "Arquivo de env not found: $arquivo — using environment variables or defaults"
  fi
}

# Obtain JWT from Keycloak via client_credentials
get_keycloak_token() {
  local url="$1"     # URL base do realm Keycloak
  local client="$2"  # client_id
  local secret="$3"  # client_secret

  # Keycloak token endpoint: <base_url>/protocol/openid-connect/token
  local resposta
  resposta=$(curl -sf \
    "${url}/protocol/openid-connect/token" \
    -d "grant_type=client_credentials" \
    -d "client_id=${client}" \
    -d "client_secret=${secret}" \
    2>/dev/null || echo "")

  if [[ -z "$resposta" ]]; then
    echo ""
    return 1
  fi

  # Extract access_token from JSON response
  echo "$resposta" | jq -r '.access_token // empty'
}

# Authenticated POST returning full JSON
api_post() {
  local label="$1"   # Step description (for logs)
  local url="$2"     # Full endpoint URL
  local token="$3"   # Token Bearer
  local body="$4"    # JSON body

  info "POST ${url}" >&2
  raw "Body: $body"

  # curl -s: silent; -S: show errors; -w: capture HTTP status
  local http_code resposta
  resposta=$(curl -s -S -w '\n__HTTP_CODE__%{http_code}' \
    -X POST "${url}" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "${body}" \
    2>/dev/null || echo "__HTTP_CODE__000")

  http_code=$(echo "$resposta" | grep '__HTTP_CODE__' | sed 's/__HTTP_CODE__//')
  resposta=$(echo "$resposta" | grep -v '__HTTP_CODE__')

  info "HTTP $http_code ← $label" >&2
  raw "$resposta"
  echo "$resposta"
  # Persist HTTP code to file for parent shell (subshell cannot export vars)
  echo "$http_code" > "${_HTTP_CODE_FILE}"
}

# Authenticated GET returning full JSON
api_get() {
  local label="$1"
  local url="$2"
  local token="$3"

  info "GET ${url}" >&2

  local http_code resposta
  resposta=$(curl -s -S -w '\n__HTTP_CODE__%{http_code}' \
    "${url}" \
    -H "Authorization: Bearer ${token}" \
    2>/dev/null || echo "__HTTP_CODE__000")

  http_code=$(echo "$resposta" | grep '__HTTP_CODE__' | sed 's/__HTTP_CODE__//')
  resposta=$(echo "$resposta" | grep -v '__HTTP_CODE__')

  info "HTTP $http_code ← $label" >&2
  raw "$resposta"
  echo "$resposta"
  echo "$http_code" > "${_HTTP_CODE_FILE}"
}

# =============================================================================
# PHASE 0 — PRE-CHECKS: dependencies and services
# =============================================================================
titulo "PHASE 0 — Pre-checks: dependencies and services"

# Check required commands are available
info "Checking required tools (curl, jq, docker)..."
for cmd in curl jq; do
  if command -v "$cmd" &>/dev/null; then
    ok "Comando available: $cmd"
  else
    error "Comando not found: $cmd — instale antes de continuar"
  fi
done

# docker is optional — container check only
if command -v docker &>/dev/null; then
  ok "Comando available: docker"
  DOCKER_OK=true
else
  warn "docker not found — skipping container check"
  DOCKER_OK=false
fi

# cast (Foundry) is optional — used for on-chain verification
if command -v cast &>/dev/null; then
  ok "Comando available: cast (Foundry) — on-chain verification available"
  CAST_OK=true
else
  warn "cast not found — on-chain verification de eventos will be skipped"
  CAST_OK=false
fi

sep

# Verificar se os containers do Scenario B are running
if [[ "$DOCKER_OK" == "true" ]]; then
  info "Checking Scenario B containers..."

  # List running Scenario B containers
  CONTAINERS_UP=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E 'central-bank|bank-[abcd]|cacti|keycloak|postgres|besu' | sort || echo "")

  if [[ -n "$CONTAINERS_UP" ]]; then
    ok "Containers running:"
    echo "$CONTAINERS_UP" | while read -r c; do info "  container: $c"; done
  else
    warn "No Scenario B container detected — stack may not be up"
    warn "Run: make up.central-banks  (or the Scenario B make target)"
  fi

  sep

  # Verificar especificamente o container do Cacti Relayer
  # Cacti is critical: without it, o bridge NUNCA chega a ACTIVE
  info "Checking Cacti Relayer container..."
  CACTI_CONTAINER=$(docker ps --format '{{.Names}}\t{{.Status}}' 2>/dev/null | grep -i 'cacti\|relayer' || echo "")
  if [[ -n "$CACTI_CONTAINER" ]]; then
    ok "Cacti Relayer container found:"
    echo "$CACTI_CONTAINER" | while IFS=$'\t' read -r nome status; do
      info "  nome: $nome | status: $status"
    done
  else
    error "Cacti Relayer NOT found running — bridge NUNCA will reach ACTIVE without it"
    error "Check: docker ps | grep -i cacti"
  fi
fi

sep

# Verificar acessibilidade dos endpoints principais
info "Checking API endpoint reachability..."

for endpoint_label in "CB-A|${CB_A_URL}/health" "CB-B|${CB_B_URL}/health"; do
  label=$(echo "$endpoint_label" | cut -d'|' -f1)
  url=$(echo "$endpoint_label" | cut -d'|' -f2)
  # Short-timeout health check; accept 200, 204 or 404 (endpoint may not exist)
  http=$(curl -s -o /dev/null -w '%{http_code}' --connect-timeout 3 --max-time 5 "$url" 2>/dev/null || echo "000")
  if [[ "$http" =~ ^(200|204|404)$ ]]; then
    ok "$label reachable: HTTP $http em $url"
  else
    error "$label unreachable (HTTP $http) — URL: $url"
    error "  Check se o API Gateway do $label is running"
  fi
done

# =============================================================================
# PHASE 1 — Authentication: obtain Keycloak tokens for CB-A and CB-B
# =============================================================================
titulo "PHASE 1 — Authentication no Keycloak"

info "Loading CB-A credentials from: $CB_A_ENV"
load_env_file "$CB_A_ENV"

# After sourcing env, KC_BASE_PATH, KC_REALM, KC_CLIENT_ID,
# KC_CLIENT_SECRET and BANK_CODE are available (if the file exists)
KC_URL_A="${KC_ISSUER_URL:-${KC_BASE_PATH:-http://localhost:8081}/realms/${KC_REALM:-central-bank-a}}"
KC_CLIENT_A="${KC_CLIENT_ID:-central-bank-a-client}"
KC_SECRET_A="${KC_CLIENT_SECRET:-}"
CB_A_BANK_ID="${BANK_CODE:-central-bank-a}"
# Prefer sovereign W-token (registered in IdentityRegistry with CB-A as issuer);
# falls back to cooperative HUB_TOKEN_A_ADDRESS if sovereign not yet deployed.
W_TOKEN_A_ADDR="${SOVEREIGN_HUB_TOKEN_A_ADDRESS:-${HUB_TOKEN_A_ADDRESS:-}}"

if [[ -z "$KC_SECRET_A" ]]; then
  error "KC_CLIENT_SECRET not found for CB-A — authentication will fail"
  error "  Check o arquivo: $CB_A_ENV"
fi

info "CB-A → Realm: $KC_URL_A | Client: $KC_CLIENT_A | bank_id: $CB_A_BANK_ID"
info "CB-A → W-Token Hub address: ${W_TOKEN_A_ADDR:-<not configured>}"

TOKEN_A=$(get_keycloak_token "$KC_URL_A" "$KC_CLIENT_A" "$KC_SECRET_A" 2>/dev/null || echo "")
if [[ -n "$TOKEN_A" ]]; then
  # Show first 40 token chars (to confirm valid JWT)
  ok "CB-A authenticated — token: ${TOKEN_A:0:40}..."
else
  error "CB-A: failed to obtain Keycloak token"
  error "  URL: $KC_URL_A"
  error "  Client: $KC_CLIENT_A"
  error "  Check Keycloak is up and client_secret is correct"
fi

sep

info "Carregando credenciais do CB-B de: $CB_B_ENV"
load_env_file "$CB_B_ENV"

KC_URL_B="${KC_ISSUER_URL:-${KC_BASE_PATH:-http://localhost:8081}/realms/${KC_REALM:-central-bank-b}}"
KC_CLIENT_B="${KC_CLIENT_ID:-central-bank-b-client}"
KC_SECRET_B="${KC_CLIENT_SECRET:-}"
CB_B_BANK_ID="${BANK_CODE:-central-bank-b}"
# Prefer sovereign W-token (registered in IdentityRegistry with CB-B as issuer);
# falls back to cooperative HUB_TOKEN_B_ADDRESS if sovereign not yet deployed.
W_TOKEN_B_ADDR="${SOVEREIGN_HUB_TOKEN_B_ADDRESS:-${HUB_TOKEN_B_ADDRESS:-}}"

if [[ -z "$KC_SECRET_B" ]]; then
  error "KC_CLIENT_SECRET not found for CB-B"
fi

info "CB-B → Realm: $KC_URL_B | Client: $KC_CLIENT_B | bank_id: $CB_B_BANK_ID"
info "CB-B → W-Token Hub address: ${W_TOKEN_B_ADDR:-<not configured>}"

TOKEN_B=$(get_keycloak_token "$KC_URL_B" "$KC_CLIENT_B" "$KC_SECRET_B" 2>/dev/null || echo "")
if [[ -n "$TOKEN_B" ]]; then
  ok "CB-B authenticated — token: ${TOKEN_B:0:40}..."
else
  error "CB-B: falha ao obter token Keycloak"
  error "  URL: $KC_URL_B"
  error "  Client: $KC_CLIENT_B"
fi

# Se does not haveos tokens, cannot continuar as fases seguintes
if [[ -z "$TOKEN_A" || -z "$TOKEN_B" ]]; then
  warn "Missing tokens — next phases may fail. Continuing for diagnosis..."
fi

# =============================================================================
# PHASE 2 — MINT HUB W-tCeBM: diagnostic / manual bootstrap (optional)
# =============================================================================
titulo "PHASE 2 — Mint Hub W-tCeBM tokens nos signers (opcional)"

# NOTE:
# With BesuRelayerExecutor active, W-tCeBM is minted on-chain on the Hub
# automatically during lock-mint processing (Phase 3). This phase
# NOT required in the default flow.
#
# Use SKIP_MINT=false SOMENTE para:
#   - Diagnostic when BesuRelayerExecutor is not configured correctly
#   - Bootstrap manual em ambientes onde o relay still is not up
#   - Recreate CB balance after burn without re-bridge
#
# O endpoint POST /api/v2/amm/token/mint-and-approve:
#   1. Mint `amount` W-tCeBM directly to CB Hub signer
#   2. Approve SovereignAMM contract to spend those tokens
#
# In production with BesuRelayerExecutor: keep SKIP_MINT=true (default).

if [[ "${SKIP_MINT:-true}" == "true" ]]; then
  warn "SKIP_MINT=true — skipping Phase 2 (BesuRelayerExecutor mints automatically on bridge)"
else
  # ── CB-A: mint-and-approve ───────────────────────────────────────────────
  info "CB-A: mint ${AMOUNT_A} W-tCeBM_BRL → signer Hub + approve SovereignAMM"
  MINT_A_RESP=$(api_post "CB-A mint-and-approve" \
    "${CB_A_URL}/api/v2/amm/token/mint-and-approve" \
    "${TOKEN_A:-}" \
    "{\"amount\":\"${AMOUNT_A}\"}") 
  MINT_A_HTTP=$(_http_code)

  if [[ "$MINT_A_HTTP" == "200" || "$MINT_A_HTTP" == "201" ]]; then
    ok "CB-A: mint-and-approve OK (HTTP $MINT_A_HTTP)"
    echo "$MINT_A_RESP" | jq . 2>/dev/null || true
  else
    error "CB-A: mint-and-approve FAILED (HTTP $MINT_A_HTTP)"
    echo "$MINT_A_RESP" | jq . 2>/dev/null || echo "$MINT_A_RESP"
    error "  Check se o signer tem CENTRAL_BANK_ROLE no contrato W-tCeBM_BRL"
  fi

  sep

  # ── CB-B: mint-and-approve ───────────────────────────────────────────────
  info "CB-B: mint ${AMOUNT_B} W-tCeBM_ARS → signer Hub + approve SovereignAMM"
  MINT_B_RESP=$(api_post "CB-B mint-and-approve" \
    "${CB_B_URL}/api/v2/amm/token/mint-and-approve" \
    "${TOKEN_B:-}" \
    "{\"amount\":\"${AMOUNT_B}\"}") 
  MINT_B_HTTP=$(_http_code)

  if [[ "$MINT_B_HTTP" == "200" || "$MINT_B_HTTP" == "201" ]]; then
    ok "CB-B: mint-and-approve OK (HTTP $MINT_B_HTTP)"
    echo "$MINT_B_RESP" | jq . 2>/dev/null || true
  else
    error "CB-B: mint-and-approve FALHOU (HTTP $MINT_B_HTTP)"
    echo "$MINT_B_RESP" | jq . 2>/dev/null || echo "$MINT_B_RESP"
    error "  Check se o signer tem CENTRAL_BANK_ROLE no contrato W-tCeBM_ARS"
  fi
fi

# =============================================================================
# PHASE 3 — BRIDGE LOCK&MINT: lock native asset + mint on Hub
# =============================================================================
titulo "PHASE 3 — Bridge Lock&Mint (ambos os CBs)"

# WHAT HAPPENS HERE:
# 1. O CB-A chama POST /api/v2/bridge/lock-mint
# 2. O API Gateway instrui o SpokeBridge a BLOQUEAR (lock) o ativo nativo no spoke
# 3. O Cacti Relayer detecta o evento de lock no contrato SpokeBridge
# 4. O Relayer instrui o Hub a MINTAR (mint) o token espelhado (W-tCeBM_BRL)
# 5. A resposta inclui um position_id to track progress via polling
#
# ERROS COMUNS:
# - HTTP 422 BRIDGE_POSITION_NOT_ACTIVE: bridge still processando (normal, aguardar polling)
# - HTTP 400/500: incorrect parameters or internal service error
# - Timeout no polling: Cacti Relayer offline ou spoke desconectado do Hub

POS_A_ID=""
POS_B_ID=""

if [[ "${SKIP_LOCK_MINT}" == "true" ]]; then
  warn "SKIP_LOCK_MINT=true — skipping Phase 3 (Bridge Lock&Mint)"
  warn "  Ensure existing bridge positions bridge ACTIVE antes de continuing"
else

  info "Checking current bridge state BEFORE lock-mint..."
  info "This helps determine if any bridge position is already in progress"

  BRIDGES_A_ANTES=$(api_get "CB-A bridges existentes" \
    "${CB_A_URL}/api/v2/bridge/positions" \
    "${TOKEN_A:-}" 2>/dev/null || echo "{}")
  qtd_A=$(echo "$BRIDGES_A_ANTES" | jq -r '.positions // [] | length' 2>/dev/null || echo "0")
  info "CB-A: $qtd_A existing bridge position(s)"
  echo "$BRIDGES_A_ANTES" | jq '.positions[]? | {position_id, bridge_state, spoke_network, native_asset, amount}' 2>/dev/null || true

  BRIDGES_B_ANTES=$(api_get "CB-B bridges existentes" \
    "${CB_B_URL}/api/v2/bridge/positions" \
    "${TOKEN_B:-}" 2>/dev/null || echo "{}")
  qtd_B=$(echo "$BRIDGES_B_ANTES" | jq -r '.positions // [] | length' 2>/dev/null || echo "0")
  info "CB-B: $qtd_B position(s) bridge existente(s)"

  echo "$BRIDGES_B_ANTES" | jq '.positions[]? | {position_id, bridge_state, spoke_network, native_asset, amount}' 2>/dev/null || true

  sep

  # ── CB-A: Lock&Mint ──────────────────────────────────────────────────────
  info "CB-A: iniciando Lock&Mint (API simplificada - resolve owner_bank_id, spoke_network, native_asset, mirrored_asset)"
  info "  Quantidade: ${AMOUNT_A} (API deriva campos de JWT + config)"

  LOCK_A_BODY=$(jq -nc \
    --arg amount "$AMOUNT_A" \
    '{"amount":$amount}')

  LOCK_A_RESP=$(api_post "CB-A lock-mint" \
    "${CB_A_URL}/api/v2/bridge/lock-mint" \
    "${TOKEN_A:-}" \
    "$LOCK_A_BODY")

  POS_A_ID=$(echo "$LOCK_A_RESP" | jq -r '.position_id // empty' 2>/dev/null || echo "")

  if [[ -n "$POS_A_ID" ]]; then
    ok "CB-A: lock-mint iniciado — position_id: $POS_A_ID"
    echo "$LOCK_A_RESP" | jq '{position_id, bridge_state, spoke_network, native_asset, mirrored_asset, amount}' 2>/dev/null || true
  else
    error "CB-A: lock-mint FAILED — full response:"
    echo "$LOCK_A_RESP" | jq . 2>/dev/null || echo "$LOCK_A_RESP"
    error "  HTTP code: $(_http_code)"
    error "  Check: CB-A spoke balance, SpokeBridge deployed, permissions"
  fi

  sep

  # ── CB-B: Lock&Mint ──────────────────────────────────────────────────────
  info "CB-B: iniciando Lock&Mint (API simplificada - resolve owner_bank_id, spoke_network, native_asset, mirrored_asset)"
  info "  Quantidade: ${AMOUNT_B} (API deriva campos de JWT + config)"

  LOCK_B_BODY=$(jq -nc \
    --arg amount "$AMOUNT_B" \
    '{"amount":$amount}')

  LOCK_B_RESP=$(api_post "CB-B lock-mint" \
    "${CB_B_URL}/api/v2/bridge/lock-mint" \
    "${TOKEN_B:-}" \
    "$LOCK_B_BODY")

  POS_B_ID=$(echo "$LOCK_B_RESP" | jq -r '.position_id // empty' 2>/dev/null || echo "")

  if [[ -n "$POS_B_ID" ]]; then
    ok "CB-B: lock-mint iniciado — position_id: $POS_B_ID"
    echo "$LOCK_B_RESP" | jq '{position_id, bridge_state, spoke_network, native_asset, mirrored_asset, amount}' 2>/dev/null || true
  else
    error "CB-B: lock-mint FALHOU — resposta completa:"
    echo "$LOCK_B_RESP" | jq . 2>/dev/null || echo "$LOCK_B_RESP"
    error "  HTTP code: $(_http_code)"
  fi

fi # fim SKIP_LOCK_MINT

# =============================================================================
# PHASE 4 — POLLING BRIDGE: wait for bridge_state=ACTIVE
# =============================================================================
titulo "PHASE 4 — Polling bridge_state=ACTIVE (Cacti Relayer)"

# NOTE:
# After lock-mint, Cacti Relayer must:
#   1. Detectar o evento Lock emitido pelo contrato SpokeBridge
#   2. Sign and send mint transaction on Hub
#   3. Wait for on-chain confirmation
# This may take de 2s a 60s depending on network latency.
# Canonical timeout: 120s. Polling interval: 5s.
#
# IF BRIDGE DOES NOT REACH ACTIVE:
#   - Cacti Relayer offline (container parado ou com erro)
#   - Spoke desconectado do Hub (problema de rede entre containers)
#   - SpokeBridge contract not deployed correctly
#   - Saldo insuficiente na conta signer do Relayer

poll_bridge_active() {
  local cb_url="$1"
  local token="$2"
  local label="$3"
  local timeout=120
  local intervalo=5
  local elapsed=0
  local bridge_state=""

  info "$label: polling bridge_state a cada ${intervalo}s (timeout: ${timeout}s)"
  info "$label: URL: ${cb_url}/api/v2/bridge/positions?state=ACTIVE"

  while [[ $elapsed -lt $timeout ]]; do
    # Query all positions com state=ACTIVE
    local resposta
    resposta=$(curl -sf \
      "${cb_url}/api/v2/bridge/positions?state=ACTIVE" \
      -H "Authorization: Bearer ${token}" \
      2>/dev/null || echo "{}")

    # Capture first position state (should be ACTIVE)
    bridge_state=$(echo "$resposta" | jq -r '.positions[0].bridge_state // "N/A"' 2>/dev/null || echo "N/A")
    local pos_count
    pos_count=$(echo "$resposta" | jq '.positions // [] | length' 2>/dev/null || echo "0")

    info "$label: ${elapsed}s — ACTIVE positions found: $pos_count | estado[0]: $bridge_state"

    if [[ "$bridge_state" == "ACTIVE" ]]; then
      ok "$label: bridge ACTIVE after ${elapsed}s"
      # Show active position details
      echo "$resposta" | jq '.positions[0] | {position_id, bridge_state, native_asset, mirrored_asset, amount, spoke_network}' 2>/dev/null || true
      return 0
    fi

    # Every 30s, fetch ALL positions for additional diagnosis
    if [[ $((elapsed % 30)) -eq 0 && $elapsed -gt 0 ]]; then
      warn "$label: ${elapsed}s without ACTIVE — checking all positions (no filter)..."
      local todas
      todas=$(curl -sf "${cb_url}/api/v2/bridge/positions" \
        -H "Authorization: Bearer ${token}" 2>/dev/null || echo "{}")
      echo "$todas" | jq '.positions[]? | {position_id, bridge_state, native_asset, created_at}' 2>/dev/null || true
    fi

    sleep "$intervalo"
    elapsed=$((elapsed + intervalo))
  done

  # Timeout reached — detailed diagnosis
  error "$label: TIMEOUT after ${timeout}s without bridge ACTIVE"
  error "  Possible causes:"
  error "  1. Cacti Relayer parado ou com error — check: docker logs <cacti_container>"
  error "  2. Spoke not connected to Hub — check Docker network"
  error "  3. Conta signer do Relayer no balance no Hub — check saldo ETH"
  error "  4. SpokeBridge contract not deployed — execute: make contracts.deploy"

  warn "$label: querying all bridge positions for diagnosis..."
  local todas_final
  todas_final=$(curl -sf "${cb_url}/api/v2/bridge/positions" \
    -H "Authorization: Bearer ${token}" 2>/dev/null || echo "{}")
  echo "$todas_final" | jq '.positions[]? | {position_id, bridge_state, native_asset, error_message}' 2>/dev/null || true

  return 1
}

BRIDGE_A_OK=false
BRIDGE_B_OK=false

if poll_bridge_active "$CB_A_URL" "${TOKEN_A:-}" "CB-A"; then
  BRIDGE_A_OK=true
else
  warn "CB-A: continuing without bridge ACTIVE for full diagnosis"
fi

sep

if poll_bridge_active "$CB_B_URL" "${TOKEN_B:-}" "CB-B"; then
  BRIDGE_B_OK=true
else
  warn "CB-B: continuing without bridge ACTIVE for full diagnosis"
fi

# =============================================================================
# PHASE 4 — COMMIT ON-CHAIN: registro bilateral no LiquidityCommitRegistry
# =============================================================================
titulo "PHASE 5 — On-chain commit (LiquidityCommitRegistry)"

# COMMIT-REVEAL PROTOCOL COMMIT-REVEAL:
#
# O commit funciona como um protocolo de duas fases bilateral:
#
# CB-A commit (side A):
#   → status: PENDING   (aguardando contraparte)
#   → pool: EMPTY       (no funds moved yet)
#
# CB-B commit (side B):
#   → status: EXECUTED  (automatic on-chain match!)
#   → pool: ACTIVE      (LP positions created for both CBs)
#
# CRITICAL RULES:
# - Same CB CANNOT commit both sides → HTTP 409 SAME_PROVIDER_BOTH_SIDES
# - Commits expire in 72h → pool returns to EMPTY without counterparty
# - A bridge DEVE estar ACTIVE antes do commit → HTTP 422 BRIDGE_POSITION_NOT_ACTIVE
# - w_token_address must be mirrored Hub token address (W-tCeBM_BRL etc)

COMMIT_A_ID=""
COMMIT_B_ID=""

if [[ "${SKIP_COMMIT}" == "true" ]]; then
  warn "SKIP_COMMIT=true — skipping Phase 5 (commit on-chain)"
else

  info "Verificando estado atual dos commits for pair $POOL_PAIR..."

  # Renovar tokens antes do commit (podem ter expirado durante polling de bridge)
  info "Renovando tokens de acesso antes do commit..."
  TOKEN_A=$(get_keycloak_token "$KC_URL_A" "$KC_CLIENT_A" "$KC_SECRET_A" 2>/dev/null || echo "")
  TOKEN_B=$(get_keycloak_token "$KC_URL_B" "$KC_CLIENT_B" "$KC_SECRET_B" 2>/dev/null || echo "")
  [[ -n "$TOKEN_A" ]] && ok "CB-A: token renovado" || error "CB-A: falha ao renovar token"
  [[ -n "$TOKEN_B" ]] && ok "CB-B: token renovado" || error "CB-B: falha ao renovar token"

  COMMITS_A_ANTES=$(api_get "CB-A commits existentes" \
    "${CB_A_URL}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}" \
    "${TOKEN_A:-}" 2>/dev/null || echo "{}")
  echo "$COMMITS_A_ANTES" | jq '.commits[]? | {commit_id, status, side, provider_id, created_at, expires_at}' 2>/dev/null || true

  sep

  if [[ "$BRIDGE_A_OK" != "true" ]]; then
    warn "CB-A: bridge is NOT ACTIVE — commit will likely return BRIDGE_POSITION_NOT_ACTIVE (HTTP 422)"
    warn "  But we still try to see the exact error..."
  fi

  # ── CB-A: Commit side A ──────────────────────────────────────────────────
  info "CB-A: registrando commit (API simplificada - resolve provider_id, side, w_token_address)"
  info "  pool_pair: $POOL_PAIR | amount: $AMOUNT_A"
  info "  (API deriva: provider_id do JWT, side do BANK_CODE, w_token_address da config)"

  COMMIT_A_BODY=$(jq -nc \
    --arg pair "$POOL_PAIR" \
    --arg amt  "$AMOUNT_A" \
    '{"pool_pair":$pair,"amount":$amt}')

  COMMIT_A_RESP=$(api_post "CB-A commit side A" \
    "${CB_A_URL}/api/v2/amm/liquidity/commit" \
    "${TOKEN_A:-}" \
    "$COMMIT_A_BODY")

  COMMIT_A_ID=$(echo "$COMMIT_A_RESP" | jq -r '.commit_id // empty' 2>/dev/null || echo "")
  COMMIT_A_STATUS=$(echo "$COMMIT_A_RESP" | jq -r '.status // empty' 2>/dev/null || echo "")
  COMMIT_A_HTTP=$(_http_code)

  if [[ -n "$COMMIT_A_ID" ]]; then
    ok "CB-A: commit registrado — commit_id: $COMMIT_A_ID | status: $COMMIT_A_STATUS"
    if [[ "$COMMIT_A_STATUS" == "PENDING" ]]; then
      ok "CB-A: status PENDING — correct, aguardando commit do CB-B"
    else
      warn "CB-A: status inexpected: $COMMIT_A_STATUS (expected: PENDING)"
    fi
    echo "$COMMIT_A_RESP" | jq '{commit_id, on_chain_commit_id, status, side, pool_pair, expires_at}' 2>/dev/null || true
  else
    error "CB-A: commit FALHOU (HTTP $COMMIT_A_HTTP) — resposta completa:"
    echo "$COMMIT_A_RESP" | jq . 2>/dev/null || echo "$COMMIT_A_RESP"

    # Diagnosis based on HTTP code
    case "$COMMIT_A_HTTP" in
      422)
        error "  HTTP 422 → provavelmente BRIDGE_POSITION_NOT_ACTIVE"
        error "  Causa: a bridge do CB-A still is not ACTIVE"
        error "  Action: wait for Cacti to process lock-mint (Phase 3)"
        ;;
      409)
        error "  HTTP 409 → provavelmente SAME_PROVIDER_BOTH_SIDES"
        error "  Cause: this CB already has an active commit on both sides"
        ;;
      403)
        error "  HTTP 403 → CROSS_CB_MINT_PROHIBITED or insufficient permission"
        ;;
      *)
        error "  HTTP $COMMIT_A_HTTP → check logs do API Gateway do CB-A"
        ;;
    esac
  fi

  sep

  # ── CB-B: Commit side B ──────────────────────────────────────────────────
  # IMPORTANTE: O commit do CB-B DEVE acontecer depois do commit do CB-A
  # Quando o CB-B commita o lado B, o contrato detecta que ambos os lados existem
  # and emits CommitMatched, captured by Cacti Watcher

  info "CB-B: registrando commit (API simplificada - resolve provider_id, side, w_token_address)"
  info "  pool_pair: $POOL_PAIR | amount: $AMOUNT_B"
  info "  (API deriva: provider_id do JWT, side do BANK_CODE, w_token_address da config)"

  COMMIT_B_BODY=$(jq -nc \
    --arg pair "$POOL_PAIR" \
    --arg amt  "$AMOUNT_B" \
    '{"pool_pair":$pair,"amount":$amt}')

  COMMIT_B_RESP=$(api_post "CB-B commit side B" \
    "${CB_B_URL}/api/v2/amm/liquidity/commit" \
    "${TOKEN_B:-}" \
    "$COMMIT_B_BODY")

  COMMIT_B_ID=$(echo "$COMMIT_B_RESP" | jq -r '.commit_id // empty' 2>/dev/null || echo "")
  COMMIT_B_STATUS=$(echo "$COMMIT_B_RESP" | jq -r '.status // empty' 2>/dev/null || echo "")
  COMMIT_B_HTTP=$(_http_code)

  if [[ -n "$COMMIT_B_ID" ]]; then
    ok "CB-B: commit registrado — commit_id: $COMMIT_B_ID | status: $COMMIT_B_STATUS"
    if [[ "$COMMIT_B_STATUS" == "EXECUTED" ]]; then
      ok "CB-B: status EXECUTED — auto-match occurred! Pool deve estar ACTIVE"
    elif [[ "$COMMIT_B_STATUS" == "PENDING" ]]; then
      warn "CB-B: status PENDING — match did NOT occur automatically"
      warn "  Possible cause: CB-A commit was missing or expired"
      warn "  Ou: commit do CB-A era do mesmo provider_id (rule violation)"
    else
      warn "CB-B: status inexpected: $COMMIT_B_STATUS"
    fi
    echo "$COMMIT_B_RESP" | jq '{commit_id, on_chain_commit_id, status, side, pool_pair, expires_at}' 2>/dev/null || true
  else
    error "CB-B: commit FALHOU (HTTP $COMMIT_B_HTTP) — resposta completa:"
    echo "$COMMIT_B_RESP" | jq . 2>/dev/null || echo "$COMMIT_B_RESP"

    case "$COMMIT_B_HTTP" in
      422)
        error "  HTTP 422 → BRIDGE_POSITION_NOT_ACTIVE"
        error "  Causa: bridge do CB-B still is not ACTIVE"
        ;;
      409)
        error "  HTTP 409 → SAME_PROVIDER_BOTH_SIDES"
        ;;
      403)
        error "  HTTP 403 → CROSS_CB_MINT_PROHIBITED or insufficient permission"
        ;;
      *)
        error "  HTTP $COMMIT_B_HTTP → check logs do API Gateway do CB-B"
        ;;
    esac
  fi

  # Record timestamp after both commits — used to measure watcher latency
  T_APOS_COMMITS=$(date +%s)
  info "Latency timer started at: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"

fi # fim SKIP_COMMIT

# =============================================================================
# PHASE 5 — POLLING COMMIT EXECUTED: aguardar watcher CommitMatched
# =============================================================================
titulo "PHASE 6 — Polling commit status=EXECUTED (Cacti CommitMatchedWatcher)"

# NOTE:
# CommitMatchedWatcher is a Cacti TypeScript process that:
#   1. Escuta o evento CommitMatched emitido pelo LiquidityCommitRegistry no Hub
#   2. Ao detectar o evento, chama execute-matched-commit no backend
#   3. Backend atualiza ambos os commits para EXECUTED e cria LiquidityPositions
#
# SE FICAR PRESO EM PENDING:
#   - CommitMatchedWatcher is not rodando (container Cacti parado?)
#   - CommitMatched event was not emitted (verificar logs on-chain)
#   - Backend failed to process o execute-matched-commit (error interno)
#
# Canonical timeout de UI: 60s (warning after 30s)

poll_commit_executed() {
  local cb_url="$1"
  local token="$2"
  local label="$3"
  local timeout=90    # use 90s here for broader diagnosis
  local intervalo=3
  local elapsed=0
  local aviso_30s_emitido=false

  info "$label: aguardando commit EXECUTED via polling a cada ${intervalo}s (timeout: ${timeout}s)"

  while [[ $elapsed -lt $timeout ]]; do
    local resposta
    resposta=$(curl -sf \
      "${cb_url}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}" \
      -H "Authorization: Bearer ${token}" \
      2>/dev/null || echo "{}")

    # Captura status do primeiro commit encontrado para este pool_pair
    # Nota: o endpoint retorna campos PascalCase (Status, CommitID) pois o struct Go does not have json tags
    local status_atual
    status_atual=$(echo "$resposta" | jq -r '.commits[0].Status // "N/A"' 2>/dev/null || echo "N/A")
    local total_commits
    total_commits=$(echo "$resposta" | jq '.commits // [] | length' 2>/dev/null || echo "0")

    info "$label: ${elapsed}s — commits encontrados: $total_commits | status[0]: $status_atual"

    # High latency warning after 30s (per spec)
    if [[ $elapsed -ge 30 && "$aviso_30s_emitido" == "false" ]]; then
      warn "$label: >30s sem EXECUTED — high latency (p95 expected: ≤30s)"
      warn "  Check logs do Cacti CommitMatchedWatcher"
      aviso_30s_emitido=true
    fi

    # Verifica se ALGUM commit chegou a EXECUTED (campos PascalCase — sem json tags no struct Go)
    local executed_count
    executed_count=$(echo "$resposta" | jq '[.commits[]? | select(.Status=="EXECUTED")] | length' 2>/dev/null || echo "0")

    if [[ "$executed_count" -gt "0" ]]; then
      local latencia=$((elapsed - 0))
      if [[ -n "${T_APOS_COMMITS:-}" ]]; then
        latencia=$(( $(date +%s) - T_APOS_COMMITS ))
      fi
      ok "$label: commit EXECUTED after ${latencia}s from bilateral registration"
      # Exibe detalhes dos commits executados
      echo "$resposta" | jq '[.commits[]? | select(.Status=="EXECUTED")] | .[] | {CommitID, OnChainCommitID, Status, Side, ProviderID}' 2>/dev/null || true
      return 0
    fi

    # A cada 30s shows all commits for diagnosis
    if [[ $((elapsed % 30)) -eq 0 && $elapsed -gt 0 ]]; then
      warn "$label: estado atual de todos os commits:"
      echo "$resposta" | jq '.commits[]? | {CommitID, Status, Side, ProviderID, CreatedAt, ExpiresAt}' 2>/dev/null || true
    fi

    sleep "$intervalo"
    elapsed=$((elapsed + intervalo))
  done

  error "$label: TIMEOUT after ${timeout}s — commit never reached EXECUTED"
  error "  CRITICAL DIAGNOSIS — check:"
  error "  1. CommitMatchedWatcher running?"
  error "     docker logs <cacti_container> 2>&1 | grep -i 'CommitMatched\\|watcher\\|error'"
  error "  2. Evento CommitMatched emitido on-chain?"
  if [[ -n "${SOVEREIGN_AMM_ADDR:-}" && "$CAST_OK" == "true" ]]; then
    info "  Verificando eventos on-chain com cast..."
    cast logs \
      --rpc-url "$HUB_RPC_URL" \
      --address "${SOVEREIGN_AMM_ADDR}" \
      "CommitMatched(bytes32,bytes32)" 2>/dev/null | tail -20 || \
      warn "  Nenhum evento CommitMatched encontrado no contrato"
  else
    error "  Para verificar on-chain: instale cast (Foundry) e defina SOVEREIGN_AMM_ADDR"
  fi
  error "  3. Final state dos commits:"
  curl -sf "${cb_url}/api/v2/amm/liquidity/commits?pool_pair=${POOL_PAIR}" \
    -H "Authorization: Bearer ${token}" 2>/dev/null | \
    jq '.commits[]? | {CommitID, Status, Side, ProviderID}' 2>/dev/null || true

  return 1
}

info "Aguardando CB-A commit chegar a EXECUTED..."
poll_commit_executed "$CB_A_URL" "${TOKEN_A:-}" "CB-A" || true

sep

info "Aguardando CB-B commit chegar a EXECUTED..."
poll_commit_executed "$CB_B_URL" "${TOKEN_B:-}" "CB-B" || true

# =============================================================================
# PHASE 6 — POOL VERIFICATION: confirm pool_status=ACTIVE e LP positions
# =============================================================================
titulo "PHASE 7 — Pool status and LP positions verification"

# NOTE:
# After commits are EXECUTED, pool should have:
#   - pool_status: ACTIVE
#   - lp_positions: two positions, one per CB (provider_id)
# Somente com pool ACTIVE os swaps comerciais ficam habilitados.
#
# IF POOL DOES NOT STAY ACTIVE:
#   - Commits not EXECUTED (check Phase 5)
#   - execute-matched-commit falhou internamente (ver logs backend)
#   - Problema no contrato AMM ao adicionar liquidez

info "Consultando pool status via CB-A (pool_pair: $POOL_PAIR)..."
POOL_STATUS_RESP=$(api_get "Pool status" \
  "${CB_A_URL}/api/v2/amm/pool/${POOL_PAIR}/status" \
  "${TOKEN_A:-}")

POOL_STATUS_ATUAL=$(echo "$POOL_STATUS_RESP" | jq -r '.pool_status // "N/A"' 2>/dev/null || echo "N/A")
LP_COUNT=$(echo "$POOL_STATUS_RESP" | jq '.total_lp_count // 0' 2>/dev/null || echo "0")

echo ""
info "═══ RESULTADO DO POOL ═══"
info "  pool_pair: $POOL_PAIR"
info "  pool_status: $POOL_STATUS_ATUAL"
info "  total_lp_count (CB-A local): $LP_COUNT"

case "$POOL_STATUS_ATUAL" in
  ACTIVE)
    ok "Pool ATIVO — liquidez injetada com sucesso!"
    ok "Commercial swaps are now enabled for pair $POOL_PAIR"
    ;;
  PENDING_COUNTERPART)
    error "Pool em PENDING_COUNTERPART — apenas um lado do commit foi executado"
    error "  CB-A committed but CB-B has not completed yet (ou vice-versa)"
    ;;
  EMPTY)
    error "Pool EMPTY — nenhum commit foi executado"
    error "  Check as Fases 4 e 5"
    ;;
  N/A)
    error "Could not query pool status (HTTP $(_http_code))"
    ;;
  *)
    warn "Pool status inexpected: $POOL_STATUS_ATUAL"
    ;;
esac

sep

# Nota: O endpoint /api/v2/amm/pool/:pair/status retorna apenas total_lp_count,
# not the full list de LP positions. Each CB keeps its own positions locally.
if [[ "$LP_COUNT" -gt "0" ]]; then
  ok "CB-A: $LP_COUNT LP position(s) registrada(s) localmente"
  ok "SC-001: LP position do CB-A criada com sucesso (provider_id: $CB_A_BANK_ID)"
  info "  (CB-B has its own LP position in its database)"
else
  error "CB-A: Nenhuma LP position encontrada — execute-matched-commit pode ter falhado"
  error "  Check logs do backend: docker logs backend-api-gateway-central-bank-a"
fi

# Check CB-B as well
info "Verificando LP count do CB-B..."
POOL_STATUS_B_RESP=$(api_get "Pool status CB-B" \
  "${CB_B_URL}/api/v2/amm/pool/${POOL_PAIR}/status" \
  "${TOKEN_B:-}")
LP_COUNT_B=$(echo "$POOL_STATUS_B_RESP" | jq '.total_lp_count // 0' 2>/dev/null || echo "0")

if [[ "$LP_COUNT_B" -gt "0" ]]; then
  ok "CB-B: $LP_COUNT_B LP position(s) registrada(s) localmente"
  ok "SC-001: LP position do CB-B criada com sucesso (provider_id: $CB_B_BANK_ID)"
else
  error "CB-B: Nenhuma LP position encontrada"
fi

sep

# Listar LP positions detalhadas (novo endpoint 008-fix-cb-liquidity)
info "Consultando LP positions detalhadas via novo endpoint GET /liquidity/positions..."
info "CB-A: GET /api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}"
LP_POSITIONS_A=$(api_get "CB-A LP positions" \
  "${CB_A_URL}/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" \
  "${TOKEN_A:-}" 2>/dev/null || echo '{}')
LP_A_COUNT=$(echo "$LP_POSITIONS_A" | jq '.count // 0' 2>/dev/null || echo "0")
if [[ "$LP_A_COUNT" -gt "0" ]]; then
  ok "CB-A: $LP_A_COUNT position(s) retornada(s) via /liquidity/positions"
  echo "$LP_POSITIONS_A" | jq '.positions[]? | {lp_id, provider_bank_id, deposit_side, token_a_contributed, token_b_contributed, added_at}' 2>/dev/null || true
else
  warn "CB-A: Nenhuma position retornada via /liquidity/positions"
fi

info "CB-B: GET /api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}"
LP_POSITIONS_B=$(api_get "CB-B LP positions" \
  "${CB_B_URL}/api/v2/amm/liquidity/positions?pool_pair=${POOL_PAIR}" \
  "${TOKEN_B:-}" 2>/dev/null || echo '{}')
LP_B_COUNT=$(echo "$LP_POSITIONS_B" | jq '.count // 0' 2>/dev/null || echo "0")
if [[ "$LP_B_COUNT" -gt "0" ]]; then
  ok "CB-B: $LP_B_COUNT position(s) retornada(s) via /liquidity/positions"
  echo "$LP_POSITIONS_B" | jq '.positions[]? | {lp_id, provider_bank_id, deposit_side, token_a_contributed, token_b_contributed, added_at}' 2>/dev/null || true
else
  warn "CB-B: Nenhuma position retornada via /liquidity/positions"
fi

sep

# Optional on-chain verification (requires cast and SOVEREIGN_AMM_ADDR)
if [[ "$CAST_OK" == "true" && -n "${SOVEREIGN_AMM_ADDR:-}" ]]; then
  info "On-chain verification: querying LogSingleSidedLiquidityAdded events on Hub..."
  cast logs \
    --rpc-url "$HUB_RPC_URL" \
    --address "$SOVEREIGN_AMM_ADDR" \
    "LogSingleSidedLiquidityAdded(address,bool,uint256)" 2>/dev/null | tail -30 && \
    ok "Eventos on-chain encontrados — check msg.sender acima" || \
    warn "Nenhum evento LogSingleSidedLiquidityAdded encontrado no contrato"
else
  info "On-chain verification skipped — set SOVEREIGN_AMM_ADDR and install cast to enable"
fi

# =============================================================================
# PHASE 7 — GUARD ANTI-G5-CROSS: verificar bloqueio do fluxo legado
# =============================================================================
titulo "PHASE 8 — Anti-G5-cross guard (CROSS_CB_MINT_PROHIBITED)"

# NOTE:
# O Scenario B bloqueia o fluxo legado (G5) onde um CB fazia mint-and-approve
# directly to another CB's Ethereum address (cross-CB minting).
# API Gateway checks via IdentityRegistry if recipient is a CB signer
# registrado no Hub. Se for, retorna HTTP 403 CROSS_CB_MINT_PROHIBITED.
#
# This guard is critical for monetary sovereignty — each CB may only
# interact with its own tokens.

if [[ -n "${CB_B_HUB_SIGNER:-}" ]]; then
  info "Testando guard anti-G5-cross..."
  info "  CB-A tenta mint-and-approve com recipient = CB-B Ethereum address ($CB_B_HUB_SIGNER)"
  info "  Result ESPERADO: HTTP 403 CROSS_CB_MINT_PROHIBITED"

  SC002_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${CB_A_URL}/api/v2/amm/token/mint-and-approve" \
    -H "Authorization: Bearer ${TOKEN_A:-}" \
    -H "Content-Type: application/json" \
    -d "{\"amount\":\"1000000000000000000\",\"recipient\":\"${CB_B_HUB_SIGNER}\"}" \
    2>/dev/null || echo "000")

  if [[ "$SC002_CODE" == "403" ]]; then
    ok "SC-002: Guard ATIVO — HTTP 403 CROSS_CB_MINT_PROHIBITED retornado correctly"
  elif [[ "$SC002_CODE" == "200" || "$SC002_CODE" == "201" ]]; then
    error "SC-002: Guard AUSENTE — mint-and-approve com CB signer como recipient retornou HTTP $SC002_CODE"
    error "  THIS IS A BUG — legacy G5-cross flow is NOT blocked!"
    error "  Check: backend/services/api-gateway — handler de mint-and-approve"
  else
    warn "SC-002: HTTP $SC002_CODE — check se o endpoint mint-and-approve existe"
  fi
else
  warn "SC-002: Anti-G5 guard NOT tested — defina CB_B_HUB_SIGNER=<cb_b_ethereum_address>"
  warn "  Exemplo: export CB_B_HUB_SIGNER=0xf17f52151EbEF6C7334FAD080c5704D77216b732"
fi

# =============================================================================
# PHASE 8 — DIAGNOSTIC SUMMARY
# =============================================================================
titulo "PHASE 9 — Diagnostic summary"

echo ""
echo -e "${BOLD}Final state:${NC}"
echo ""

# Linha de resumo de cada fase
check_item() {
  local label="$1"
  local condicao="$2"
  if [[ "$condicao" == "true" ]]; then
    echo -e "  ${GREEN}✓${NC} $label"
  else
    echo -e "  ${RED}✗${NC} $label"
  fi
}

check_item "PHASE 1: Authentication CB-A" "$([[ -n "${TOKEN_A:-}" ]] && echo true || echo false)"
check_item "PHASE 1: Authentication CB-B" "$([[ -n "${TOKEN_B:-}" ]] && echo true || echo false)"
check_item "PHASE 4: Bridge CB-A ACTIVE" "$([[ "$BRIDGE_A_OK" == "true" ]] && echo true || echo false)"
check_item "PHASE 4: Bridge CB-B ACTIVE" "$([[ "$BRIDGE_B_OK" == "true" ]] && echo true || echo false)"
check_item "PHASE 5: Commit CB-A registrado" "$([[ -n "${COMMIT_A_ID:-}" ]] && echo true || echo false)"
check_item "PHASE 5: Commit CB-B registrado" "$([[ -n "${COMMIT_B_ID:-}" ]] && echo true || echo false)"
check_item "PHASE 7: Pool ACTIVE" "$([[ "${POOL_STATUS_ATUAL:-}" == "ACTIVE" ]] && echo true || echo false)"
check_item "PHASE 7: LP position CB-A" "$([[ "${LP_COUNT:-0}" -gt "0" ]] && echo true || echo false)"
check_item "PHASE 7: LP position CB-B" "$([[ "${LP_COUNT_B:-0}" -gt "0" ]] && echo true || echo false)"

echo ""

if [[ "${#ERROS[@]}" -gt 0 ]]; then
  echo -e "${RED}${BOLD}Errors detected (${#ERROS[@]}):${NC}"
  for e in "${ERROS[@]}"; do
    echo -e "  ${RED}•${NC} $e"
  done
  echo ""
  echo -e "${YELLOW}Suggested next steps:${NC}"
  echo "  1. Identify the first phase with ✗ in the summary above"
  echo "  2. Check os logs do container correspondente:"
  echo "     docker logs <container> --tail=50 --follow"
  echo "  3. Para o Cacti Relayer: docker logs <cacti_container> 2>&1 | grep -i error"
  echo "  4. Execute com VERBOSE=true para ver respostas HTTP completas:"
  echo "     VERBOSE=true bash tryouts/debug-cb-liquidity-injection.sh"
else
  ok "No errors detected — full liquidity injection flow completed successfully!"
fi

echo ""
echo -e "${BOLD}Variables used in this run:${NC}"
echo "  CB_A_URL:          $CB_A_URL"
echo "  CB_B_URL:          $CB_B_URL"
echo "  CB_A_ENV:          $CB_A_ENV"
echo "  CB_B_ENV:          $CB_B_ENV"
echo "  CB_A_BANK_ID:      ${CB_A_BANK_ID:-?}"
echo "  CB_B_BANK_ID:      ${CB_B_BANK_ID:-?}"
echo "  POOL_PAIR:         $POOL_PAIR"
echo "  AMOUNT_A:          $AMOUNT_A"
echo "  AMOUNT_B:          $AMOUNT_B"
echo "  W_TOKEN_A_ADDR:    ${W_TOKEN_A_ADDR:-<not configured>}"
echo "  W_TOKEN_B_ADDR:    ${W_TOKEN_B_ADDR:-<not configured>}"
echo "  SPOKE_A_ASSET:     $SPOKE_A_ASSET"
echo "  SPOKE_B_ASSET:     $SPOKE_B_ASSET"
echo "  HUB_RPC_URL:       $HUB_RPC_URL"
echo "  SOVEREIGN_AMM_ADDR:${SOVEREIGN_AMM_ADDR:-<not configured>}"
echo "  CB_B_HUB_SIGNER:   ${CB_B_HUB_SIGNER:-<not configured>}"
echo ""
