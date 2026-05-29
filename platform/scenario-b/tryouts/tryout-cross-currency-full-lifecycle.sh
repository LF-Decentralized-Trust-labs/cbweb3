#!/usr/bin/env bash
# =============================================================================
# tryouts/tryout-cross-currency-full-lifecycle.sh
#
# Ciclo completo de swap cross-currency BRL → ARS usando as APIs do frontend.
#
# Etapas:
#   1.  CB-A e CB-B fazem login (M2M)
#   2.  CB-A adiciona liquidez W-BRL no pool (sovereign-add)
#   3.  CB-B adiciona liquidez W-ARS no pool (sovereign-add)
#   4.  Bank-A faz login e registra depósito FIAT
#   5.  CB-A aprova o depósito do Bank-A → converte BRL em tCeBM
#   6.  CB-A converte tCeBM → W-BRL no Hub (mint-and-approve)
#   7.  Bank-B faz login e registra depósito FIAT
#   8.  CB-B aprova o depósito do Bank-B → converte ARS em tCeBM
#   9.  CB-B converte tCeBM → W-ARS no Hub (mint-and-approve)
#  10.  Bank-A obtém cotação cross-currency (quote)
#  11.  Bank-A executa swap #1 BRL → ARS (cross-currency)
#  12.  Aguarda processamento e valida saldos
#  13.  Bank-A executa swap #2 BRL → ARS (cross-currency)
#  14.  Aguarda processamento e valida saldos finais
#
# Uso:
#   ./tryouts/tryout-cross-currency-full-lifecycle.sh
#
#   Variáveis de ambiente (opcionais):
#     BANK_A_URL       URL do api-gateway do Bank-A  (padrão: http://localhost:18080)
#     BANK_B_URL       URL do api-gateway do Bank-B  (padrão: http://localhost:28080)
#     CB_A_URL         URL do api-gateway do CB-A    (padrão: http://localhost:38080)
#     CB_B_URL         URL do api-gateway do CB-B    (padrão: http://localhost:60080)
#     AMOUNT_FIAT      Valor FIAT depositado por banco (padrão: 500)
#     SWAP_AMOUNT      tCeBM-ARS desejados por swap   (padrão: 25000000000000000000 = 25 ARS)
#     SKIP_LIQUIDITY   Se "true", pula adição de liquidez (para re-execuções)
#     SKIP_DEPOSITS    Se "true", pula depósitos e mint (para re-execuções)
# =============================================================================

set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuração
# ─────────────────────────────────────────────────────────────────────────────

BANK_A_URL="${BANK_A_URL:-http://localhost:18080}"
BANK_B_URL="${BANK_B_URL:-http://localhost:28080}"
CB_A_URL="${CB_A_URL:-http://localhost:38080}"
CB_B_URL="${CB_B_URL:-http://localhost:60080}"

POOL_PAIR="${POOL_PAIR:-W-BRL-ARS}"
AMOUNT_FIAT="${AMOUNT_FIAT:-500}"
# 60 ARS com 18 casas decimais
SWAP_AMOUNT="${SWAP_AMOUNT:-25000000000000000000}"
# 100 tCeBM com 18 casas decimais (liquidez por CB)
LIQUIDITY_AMOUNT="${LIQUIDITY_AMOUNT:-100000000000000000000}"

SKIP_LIQUIDITY="${SKIP_LIQUIDITY:-false}"
SKIP_DEPOSITS="${SKIP_DEPOSITS:-false}"

# Timeout para aguardar processamento do swap (segundos)
SWAP_WAIT_SECONDS="${SWAP_WAIT_SECONDS:-30}"

# Credenciais Keycloak (M2M)
BANK_A_CLIENT_ID="${BANK_A_CLIENT_ID:-bank-a-client}"
BANK_A_CLIENT_SECRET="${BANK_A_CLIENT_SECRET:-bank-a-local-secret}"
BANK_B_CLIENT_ID="${BANK_B_CLIENT_ID:-bank-b-client}"
BANK_B_CLIENT_SECRET="${BANK_B_CLIENT_SECRET:-bank-b-local-secret}"
CB_A_CLIENT_ID="${CB_A_CLIENT_ID:-central-bank-a-client}"
CB_A_CLIENT_SECRET="${CB_A_CLIENT_SECRET:-central-bank-a-local-secret}"
CB_B_CLIENT_ID="${CB_B_CLIENT_ID:-central-bank-b-client}"
CB_B_CLIENT_SECRET="${CB_B_CLIENT_SECRET:-central-bank-b-local-secret}"

# Tokens JWT por entidade
TOKEN_BANK_A=""
TOKEN_BANK_B=""
TOKEN_CB_A=""
TOKEN_CB_B=""

# ─────────────────────────────────────────────────────────────────────────────
# Cores e helpers
# ─────────────────────────────────────────────────────────────────────────────

BOLD='\033[1m'
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
DIM='\033[2m'
NC='\033[0m'

step_num=0

step() {
    step_num=$((step_num + 1))
    echo ""
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN}  ETAPA ${step_num}: $*${NC}"
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════════════════════${NC}"
}

log() { echo -e "${GREEN}▶${NC} $*"; }
info() { echo -e "${BLUE}ℹ${NC} ${DIM}$*${NC}"; }
warn() { echo -e "${YELLOW}⚠${NC} $*"; }
ok()   { echo -e "${GREEN}✓${NC} $*"; }
fail() { echo -e "${RED}✗${NC} $*"; exit 1; }

# Verifica se a response JSON contém um campo "error" — retorna 1 se tiver erro
has_error() {
    local resp="$1"
    if command -v jq &>/dev/null; then
        [[ "$(echo "$resp" | jq -r '.error // empty')" != "" ]]
    else
        echo "$resp" | grep -q '"error"'
    fi
}

# Aguarda pool ficar ACTIVE com timeout (segundos)
wait_pool_active() {
    local pair="$1" token="$2" timeout_s="${3:-60}" url="$4"
    local elapsed=0
    log "Aguardando pool ${pair} ficar ACTIVE (timeout: ${timeout_s}s)..."
    while [[ $elapsed -lt $timeout_s ]]; do
        local status_resp
        status_resp=$(api_call GET "${url}/api/v2/amm/pool/${pair}/status" "$token")
        local pool_status
        pool_status=$(jfield "$status_resp" "pool_status")
        info "  pool_status=${pool_status} (${elapsed}s)"
        if [[ "$pool_status" == "ACTIVE" ]]; then
            ok "Pool ${pair} está ACTIVE"
            return 0
        fi
        sleep 5
        elapsed=$((elapsed + 5))
    done
    warn "Pool ${pair} não ficou ACTIVE após ${timeout_s}s (status atual: ${pool_status:-?})"
    return 1
}

# Imprime payload formatado antes da chamada
print_request() {
    local method="$1"
    local url="$2"
    local payload="${3:-}"
    echo -e ""
    echo -e "${MAGENTA}  ┌── REQUEST ─────────────────────────────────────────────────────${NC}"
    echo -e "${MAGENTA}  │  ${BOLD}${method} ${url}${NC}"
    if [[ -n "$payload" && "$payload" != "-" ]]; then
        echo -e "${MAGENTA}  │  Payload:${NC}"
        echo "$payload" | python3 -m json.tool 2>/dev/null | sed 's/^/  │    /' || echo "  │    $payload"
    fi
    echo -e "${MAGENTA}  └────────────────────────────────────────────────────────────────${NC}"
}

# Imprime response formatado
print_response() {
    local resp="$1"
    echo -e "${DIM}  ┌── RESPONSE ────────────────────────────────────────────────────${NC}"
    echo "$resp" | python3 -m json.tool 2>/dev/null | sed 's/^/  │  /' || echo "  │  $resp"
    echo -e "${DIM}  └────────────────────────────────────────────────────────────────${NC}"
}

# Extrai campo JSON sem jq (fallback)
jfield() {
    local json="$1" field="$2"
    if command -v jq &>/dev/null; then
        echo "$json" | jq -r ".$field // empty"
    else
        echo "$json" | grep -o "\"$field\":\"[^\"]*\"" | sed "s/\"$field\":\"\([^\"]*\)\"/\1/" | head -1
    fi
}

jfield_num() {
    local json="$1" field="$2"
    if command -v jq &>/dev/null; then
        echo "$json" | jq -r ".$field // empty"
    else
        echo "$json" | grep -o "\"$field\":[0-9.eE+-]*" | sed "s/\"$field\"://" | head -1
    fi
}

# Executa curl e retorna o body.
# Token é enviado como cookie (v1 endpoints) E como Bearer header (v2 endpoints),
# garantindo compatibilidade com ambos os mecanismos de auth.
api_call() {
    local method="$1"
    local url="$2"
    local token="${3:-}"
    local payload="${4:-}"

    local cmd_args=(-s -X "$method" "$url" -H "Content-Type: application/json")
    if [[ -n "$token" ]]; then
        cmd_args+=(-H "Authorization: Bearer $token")
        cmd_args+=(-b "access_token=$token")
    fi
    [[ -n "$payload" && "$payload" != "-" ]] && cmd_args+=(-d "$payload")

    curl "${cmd_args[@]}" || true
}

# ─────────────────────────────────────────────────────────────────────────────
# Login helpers
# ─────────────────────────────────────────────────────────────────────────────

login() {
    local label="$1" url="$2" client_id="$3" secret="$4"
    local payload
    payload=$(printf '{"clientId":"%s","clientSecret":"%s"}' "$client_id" "$secret")

    # Todos os prints vão para stderr para não contaminar o valor capturado por TOKEN=$(login ...)
    print_request "POST" "${url}/api/v1/auth/login" "$payload" >&2
    local resp
    resp=$(api_call POST "${url}/api/v1/auth/login" "" "$payload")
    # Omite accessToken do output (valor JWT muito longo)
    local resp_redacted
    if command -v jq &>/dev/null; then
        resp_redacted=$(echo "$resp" | jq 'if .accessToken then .accessToken = "<JWT omitido>" else . end')
    else
        resp_redacted=$(echo "$resp" | sed 's/"accessToken":"[^"]*"/"accessToken":"<JWT omitido>"/')
    fi
    print_response "$resp_redacted" >&2

    local token
    token=$(jfield "$resp" "accessToken")
    if [[ -z "$token" || "$token" == "null" ]]; then
        fail "Login de ${label} falhou — verifique credenciais e serviço" >&2
    fi
    ok "${label} autenticado" >&2
    # Único echo para stdout: o token puro para ser capturado pelo chamador
    echo "$token"
}

# ─────────────────────────────────────────────────────────────────────────────
# Balanços (helper visual)
# ─────────────────────────────────────────────────────────────────────────────

show_balances() {
    local label="$1"
    echo -e ""
    echo -e "${BOLD}${YELLOW}── Saldos: ${label} ────────────────────────────────────${NC}"

    local bal_a
    print_request "GET" "${BANK_A_URL}/api/v1/token/balance"
    bal_a=$(api_call GET "${BANK_A_URL}/api/v1/token/balance" "$TOKEN_BANK_A")
    print_response "$bal_a"
    info "Bank-A tCeBM-BRL: $(jfield "$bal_a" "balance") wei"

    local bal_b
    print_request "GET" "${BANK_B_URL}/api/v1/token/balance"
    bal_b=$(api_call GET "${BANK_B_URL}/api/v1/token/balance" "$TOKEN_BANK_B")
    print_response "$bal_b"
    info "Bank-B tCeBM-ARS: $(jfield "$bal_b" "balance") wei"
}

# ─────────────────────────────────────────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${CYAN}║   TRYOUT: CICLO COMPLETO CROSS-CURRENCY BRL → ARS               ║${NC}"
echo -e "${BOLD}${CYAN}║   Bank-A (BRL) → Bank-B (ARS) — 2 swaps completos               ║${NC}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════════════════════════╝${NC}"
echo ""
info "Bank-A API:   ${BANK_A_URL}"
info "Bank-B API:   ${BANK_B_URL}"
info "CB-A API:     ${CB_A_URL}"
info "CB-B API:     ${CB_B_URL}"
info "Pool pair:    ${POOL_PAIR}"
info "Swap amount:  ${SWAP_AMOUNT} wei (cada swap)"
info "FIAT depósito:${AMOUNT_FIAT} (cada banco)"

# ─────────────────────────────────────────────────────────────────────────────
# ETAPA 1: Login de todos os atores
# ─────────────────────────────────────────────────────────────────────────────

step "Login de todos os atores (Bank-A, Bank-B, CB-A, CB-B)"

log "Login → Bank-A"
TOKEN_BANK_A=$(login "bank-a" "$BANK_A_URL" "$BANK_A_CLIENT_ID" "$BANK_A_CLIENT_SECRET")

log "Login → Bank-B"
TOKEN_BANK_B=$(login "bank-b" "$BANK_B_URL" "$BANK_B_CLIENT_ID" "$BANK_B_CLIENT_SECRET")

log "Login → CB-A"
TOKEN_CB_A=$(login "central-bank-a" "$CB_A_URL" "$CB_A_CLIENT_ID" "$CB_A_CLIENT_SECRET")

log "Login → CB-B"
TOKEN_CB_B=$(login "central-bank-b" "$CB_B_URL" "$CB_B_CLIENT_ID" "$CB_B_CLIENT_SECRET")

ok "Todos os atores autenticados"

# ─────────────────────────────────────────────────────────────────────────────
# ETAPA 2: CB-A e CB-B adicionam liquidez soberana
# ─────────────────────────────────────────────────────────────────────────────

if [[ "$SKIP_LIQUIDITY" == "true" ]]; then
    warn "SKIP_LIQUIDITY=true — pulando adição de liquidez"
else
    step "CB-A adiciona liquidez W-BRL no pool ${POOL_PAIR}"
    log "CB-A precisa primeiro mintar e aprovar W-BRL no Hub"

    # CB-A: mint-and-approve W-BRL (Token A do pair) para o AMM
    local_payload=$(printf '{"amount":"%s","recipient":"","side":"A"}' "$LIQUIDITY_AMOUNT")
    print_request "POST" "${CB_A_URL}/api/v2/amm/token/mint-and-approve" "$local_payload"
    resp=$(api_call POST "${CB_A_URL}/api/v2/amm/token/mint-and-approve" "$TOKEN_CB_A" "$local_payload")
    print_response "$resp"
    has_error "$resp" && warn "CB-A mint-and-approve retornou erro (continuando)" || ok "CB-A: mint-and-approve W-BRL concluído"

    # CB-A: sovereign-add W-BRL
    local_payload=$(printf '{"pool_pair":"%s","amount":"%s","is_token_a":true}' "$POOL_PAIR" "$LIQUIDITY_AMOUNT")
    print_request "POST" "${CB_A_URL}/api/v2/amm/liquidity/sovereign-add" "$local_payload"
    resp=$(api_call POST "${CB_A_URL}/api/v2/amm/liquidity/sovereign-add" "$TOKEN_CB_A" "$local_payload")
    print_response "$resp"
    has_error "$resp" && warn "CB-A sovereign-add retornou erro — pool pode já ter reserve_a de sessão anterior (continuando)" \
                         || ok "CB-A: liquidez W-BRL adicionada ao pool"

    step "CB-B adiciona liquidez W-ARS no pool ${POOL_PAIR}"
    log "CB-B precisa primeiro mintar e aprovar W-ARS no Hub"

    # CB-B: mint-and-approve W-ARS (Token B do pair) para o AMM
    local_payload=$(printf '{"amount":"%s","recipient":"","side":"B"}' "$LIQUIDITY_AMOUNT")
    print_request "POST" "${CB_B_URL}/api/v2/amm/token/mint-and-approve" "$local_payload"
    resp=$(api_call POST "${CB_B_URL}/api/v2/amm/token/mint-and-approve" "$TOKEN_CB_B" "$local_payload")
    print_response "$resp"
    has_error "$resp" && warn "CB-B mint-and-approve retornou erro (continuando)" || ok "CB-B: mint-and-approve W-ARS concluído"

    # CB-B: sovereign-add W-ARS
    local_payload=$(printf '{"pool_pair":"%s","amount":"%s","is_token_a":false}' "$POOL_PAIR" "$LIQUIDITY_AMOUNT")
    print_request "POST" "${CB_B_URL}/api/v2/amm/liquidity/sovereign-add" "$local_payload"
    resp=$(api_call POST "${CB_B_URL}/api/v2/amm/liquidity/sovereign-add" "$TOKEN_CB_B" "$local_payload")
    print_response "$resp"
    has_error "$resp" && warn "CB-B sovereign-add retornou erro — pool pode já ter reserve_b de sessão anterior (continuando)" \
                         || ok "CB-B: liquidez W-ARS adicionada ao pool"

    # Aguardar pool ficar ACTIVE (timeout 90s)
    step "Aguardar pool ${POOL_PAIR} ficar ACTIVE"
    print_request "GET" "${BANK_A_URL}/api/v2/amm/pool/${POOL_PAIR}/status"
    if ! wait_pool_active "$POOL_PAIR" "$TOKEN_BANK_A" 90 "$BANK_A_URL"; then
        resp=$(api_call GET "${BANK_A_URL}/api/v2/amm/pool/${POOL_PAIR}/status" "$TOKEN_BANK_A")
        print_response "$resp"
        fail "Pool ${POOL_PAIR} não ficou ACTIVE — verifique os CBs e tente novamente com SKIP_LIQUIDITY=true se o pool já está ativo"
    fi
    resp=$(api_call GET "${BANK_A_URL}/api/v2/amm/pool/${POOL_PAIR}/status" "$TOKEN_BANK_A")
    print_response "$resp"
fi

# ─────────────────────────────────────────────────────────────────────────────
# ETAPAS 3-4: Depósito FIAT e tokenização para Bank-A (BRL)
# ─────────────────────────────────────────────────────────────────────────────

if [[ "$SKIP_DEPOSITS" == "true" ]]; then
    warn "SKIP_DEPOSITS=true — pulando depósitos e mint"
else
    step "Bank-A deposita FIAT (${AMOUNT_FIAT} BRL) e recebe tCeBM"

    # Capturar endereço Besu de Bank-A (do token balance endpoint)
    BANK_A_BESU_ADDR=$(jfield "$(api_call GET "${BANK_A_URL}/api/v1/token/balance" "$TOKEN_BANK_A")" "address" 2>/dev/null || echo "")
    if [[ -z "$BANK_A_BESU_ADDR" ]]; then
        # fallback: endereço padrão de dev
        BANK_A_BESU_ADDR="0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef"
    fi
    info "Bank-A BESU address: ${BANK_A_BESU_ADDR}"

    # 4a. Bank-A registra depósito via seu api-gateway (proxy → payment-orchestrator)
    local_payload=$(printf '{"requester_besu_address":"%s","amount":"%s"}' \
        "$BANK_A_BESU_ADDR" "$AMOUNT_FIAT")
    print_request "POST" "${BANK_A_URL}/api/v1/payments/deposits" "$local_payload"
    resp=$(api_call POST "${BANK_A_URL}/api/v1/payments/deposits" "$TOKEN_BANK_A" "$local_payload")
    print_response "$resp"
    DEPOSIT_A_ID=$(jfield "$resp" "deposit_id")
    [[ -z "$DEPOSIT_A_ID" || "$DEPOSIT_A_ID" == "null" ]] && DEPOSIT_A_ID=$(jfield "$resp" "id")
    [[ -z "$DEPOSIT_A_ID" || "$DEPOSIT_A_ID" == "null" ]] && fail "Depósito Bank-A falhou: $resp"
    ok "Bank-A: depósito registrado (ID: ${DEPOSIT_A_ID})"

    # 4b. CB-A aprova o depósito (via seu gateway, rota interna/CB)
    local_payload=$(printf '{"deposit_id":"%s"}' "$DEPOSIT_A_ID")
    print_request "POST" "${CB_A_URL}/api/v1/payments/deposits/approve" "$local_payload"
    resp=$(api_call POST "${CB_A_URL}/api/v1/payments/deposits/approve" "$TOKEN_CB_A" "$local_payload")
    print_response "$resp"
    has_error "$resp" && fail "Aprovação depósito Bank-A falhou: $resp"
    ok "CB-A: depósito aprovado — tCeBM-BRL mintado para Bank-A"

    # 4c. CB-A converte o tCeBM de Bank-A em W-BRL no Hub (mint-and-approve para AMM)
    log "CB-A converte tCeBM de Bank-A → W-BRL no Hub (mint-and-approve)"
    local_payload=$(printf '{"amount":"%s","recipient":"%s","side":"A"}' "$AMOUNT_FIAT" "$BANK_A_BESU_ADDR")
    print_request "POST" "${CB_A_URL}/api/v2/amm/token/mint-and-approve" "$local_payload"
    resp=$(api_call POST "${CB_A_URL}/api/v2/amm/token/mint-and-approve" "$TOKEN_CB_A" "$local_payload")
    print_response "$resp"
    ok "CB-A: W-BRL mintado e aprovado no Hub para Bank-A"

    step "Bank-B deposita FIAT (${AMOUNT_FIAT} ARS) e recebe tCeBM"

    BANK_B_BESU_ADDR=$(jfield "$(api_call GET "${BANK_B_URL}/api/v1/token/balance" "$TOKEN_BANK_B")" "address" 2>/dev/null || echo "")
    if [[ -z "$BANK_B_BESU_ADDR" ]]; then
        BANK_B_BESU_ADDR="0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef"
    fi
    info "Bank-B BESU address: ${BANK_B_BESU_ADDR}"

    # 5a. Bank-B registra depósito
    local_payload=$(printf '{"requester_besu_address":"%s","amount":"%s"}' \
        "$BANK_B_BESU_ADDR" "$AMOUNT_FIAT")
    print_request "POST" "${BANK_B_URL}/api/v1/payments/deposits" "$local_payload"
    resp=$(api_call POST "${BANK_B_URL}/api/v1/payments/deposits" "$TOKEN_BANK_B" "$local_payload")
    print_response "$resp"
    DEPOSIT_B_ID=$(jfield "$resp" "deposit_id")
    [[ -z "$DEPOSIT_B_ID" || "$DEPOSIT_B_ID" == "null" ]] && DEPOSIT_B_ID=$(jfield "$resp" "id")
    [[ -z "$DEPOSIT_B_ID" || "$DEPOSIT_B_ID" == "null" ]] && fail "Depósito Bank-B falhou: $resp"
    ok "Bank-B: depósito registrado (ID: ${DEPOSIT_B_ID})"

    # 5b. CB-B aprova depósito do Bank-B
    local_payload=$(printf '{"deposit_id":"%s"}' "$DEPOSIT_B_ID")
    print_request "POST" "${CB_B_URL}/api/v1/payments/deposits/approve" "$local_payload"
    resp=$(api_call POST "${CB_B_URL}/api/v1/payments/deposits/approve" "$TOKEN_CB_B" "$local_payload")
    print_response "$resp"
    has_error "$resp" && fail "Aprovação depósito Bank-B falhou: $resp"
    ok "CB-B: depósito aprovado — tCeBM-ARS mintado para Bank-B"

    # 5c. CB-B converte tCeBM de Bank-B → W-ARS no Hub
    log "CB-B converte tCeBM de Bank-B → W-ARS no Hub (mint-and-approve)"
    local_payload=$(printf '{"amount":"%s","recipient":"%s","side":"B"}' "$AMOUNT_FIAT" "$BANK_B_BESU_ADDR")
    print_request "POST" "${CB_B_URL}/api/v2/amm/token/mint-and-approve" "$local_payload"
    resp=$(api_call POST "${CB_B_URL}/api/v2/amm/token/mint-and-approve" "$TOKEN_CB_B" "$local_payload")
    print_response "$resp"
    ok "CB-B: W-ARS mintado e aprovado no Hub para Bank-B"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Saldos ANTES dos swaps
# ─────────────────────────────────────────────────────────────────────────────

show_balances "ANTES dos swaps"

# ─────────────────────────────────────────────────────────────────────────────
# ETAPA 5: Swap #1 — Bank-A envia BRL para Bank-B receber ARS
# ─────────────────────────────────────────────────────────────────────────────

step "SWAP #1: Bank-A → Bank-B (${POOL_PAIR})"

# 5a. Cotação
log "Obtendo cotação cross-currency..."
QUOTE_URL="${BANK_A_URL}/api/v2/amm/quote/cross-currency?source_currency=BRL&target_currency=ARS&amount_out=${SWAP_AMOUNT}&max_slippage_pct=0.05"
print_request "GET" "$QUOTE_URL" "-"
quote_resp=$(api_call GET "$QUOTE_URL" "$TOKEN_BANK_A")
print_response "$quote_resp"

has_error "$quote_resp" && fail "Quote swap #1 falhou — pool sem liquidez suficiente? Resposta: $quote_resp"

QUOTE_ID_1=$(jfield "$quote_resp" "quote_id")
AMOUNT_IN_1=$(jfield "$quote_resp" "amount_in")
EFFECTIVE_RATE_1=$(jfield "$quote_resp" "effective_rate")

ok "Cotação obtida — quote_id: ${QUOTE_ID_1}"
info "  amount_in (W-BRL): ${AMOUNT_IN_1} wei"
info "  amount_out (W-ARS): ${SWAP_AMOUNT} wei"
info "  taxa efetiva: ${EFFECTIVE_RATE_1}"

# max_amount_in = amount_in * 1.05 (5% slippage) — aritmética simples via awk
MAX_AMOUNT_IN_1=$(echo "$AMOUNT_IN_1" | awk '{printf "%.0f", $1 * 1.05}' 2>/dev/null || echo "$AMOUNT_IN_1")

# 5b. Executar swap #1
SWAP_PAYLOAD_1=$(printf '{
  "source_currency": "BRL",
  "target_currency": "ARS",
  "pool_pair": "%s",
  "amount_out": "%s",
  "max_amount_in": "%s",
  "beneficiary_bank_id": "bank-b",
  "quote_id": "%s"
}' "$POOL_PAIR" "$SWAP_AMOUNT" "$MAX_AMOUNT_IN_1" "$QUOTE_ID_1")

print_request "POST" "${BANK_A_URL}/api/v2/amm/swap/cross-currency" "$SWAP_PAYLOAD_1"
swap_resp_1=$(api_call POST "${BANK_A_URL}/api/v2/amm/swap/cross-currency" "$TOKEN_BANK_A" "$SWAP_PAYLOAD_1")
print_response "$swap_resp_1"

SWAP_ID_1=$(jfield "$swap_resp_1" "swap_id")
CORRELATION_ID_1=$(jfield "$swap_resp_1" "correlation_id")
has_error "$swap_resp_1" && fail "Swap #1 falhou: $swap_resp_1"
[[ -z "$SWAP_ID_1" || "$SWAP_ID_1" == "null" ]] && fail "Swap #1 não retornou swap_id: $swap_resp_1"
ok "Swap #1 submetido — swap_id: ${SWAP_ID_1}"
info "  correlation_id: ${CORRELATION_ID_1}"

# 5c. Aguardar processamento
log "Aguardando ${SWAP_WAIT_SECONDS}s para processamento do bridge-out..."
sleep "$SWAP_WAIT_SECONDS"

# 5d. Verificar status
print_request "GET" "${BANK_A_URL}/api/v2/amm/swap/cross-currency/${SWAP_ID_1}"
status_resp_1=$(api_call GET "${BANK_A_URL}/api/v2/amm/swap/cross-currency/${SWAP_ID_1}" "$TOKEN_BANK_A")
print_response "$status_resp_1"
SWAP_STATUS_1=$(jfield "$status_resp_1" "status")
ok "Swap #1 status: ${SWAP_STATUS_1}"

# Saldos após swap #1
show_balances "APÓS Swap #1"

# ─────────────────────────────────────────────────────────────────────────────
# ETAPA 6: Swap #2 — Bank-A envia BRL para Bank-B receber ARS (segundo swap)
# ─────────────────────────────────────────────────────────────────────────────

step "SWAP #2: Bank-A → Bank-B (${POOL_PAIR}) — segundo swap"

# 6a. Nova cotação
log "Obtendo nova cotação cross-currency para o swap #2..."
QUOTE_URL="${BANK_A_URL}/api/v2/amm/quote/cross-currency?source_currency=BRL&target_currency=ARS&amount_out=${SWAP_AMOUNT}&max_slippage_pct=0.05"
print_request "GET" "$QUOTE_URL" "-"
quote_resp_2=$(api_call GET "$QUOTE_URL" "$TOKEN_BANK_A")
print_response "$quote_resp_2"

has_error "$quote_resp_2" && fail "Quote swap #2 falhou: $quote_resp_2"

QUOTE_ID_2=$(jfield "$quote_resp_2" "quote_id")
AMOUNT_IN_2=$(jfield "$quote_resp_2" "amount_in")
EFFECTIVE_RATE_2=$(jfield "$quote_resp_2" "effective_rate")

ok "Cotação swap #2 — quote_id: ${QUOTE_ID_2}"
info "  amount_in (W-BRL): ${AMOUNT_IN_2} wei"
info "  taxa efetiva: ${EFFECTIVE_RATE_2}"

MAX_AMOUNT_IN_2=$(echo "$AMOUNT_IN_2" | awk '{printf "%.0f", $1 * 1.05}' 2>/dev/null || echo "$AMOUNT_IN_2")

# 6b. Executar swap #2
SWAP_PAYLOAD_2=$(printf '{
  "source_currency": "BRL",
  "target_currency": "ARS",
  "pool_pair": "%s",
  "amount_out": "%s",
  "max_amount_in": "%s",
  "beneficiary_bank_id": "bank-b",
  "quote_id": "%s"
}' "$POOL_PAIR" "$SWAP_AMOUNT" "$MAX_AMOUNT_IN_2" "$QUOTE_ID_2")

print_request "POST" "${BANK_A_URL}/api/v2/amm/swap/cross-currency" "$SWAP_PAYLOAD_2"
swap_resp_2=$(api_call POST "${BANK_A_URL}/api/v2/amm/swap/cross-currency" "$TOKEN_BANK_A" "$SWAP_PAYLOAD_2")
print_response "$swap_resp_2"

SWAP_ID_2=$(jfield "$swap_resp_2" "swap_id")
CORRELATION_ID_2=$(jfield "$swap_resp_2" "correlation_id")
has_error "$swap_resp_2" && fail "Swap #2 falhou: $swap_resp_2"
[[ -z "$SWAP_ID_2" || "$SWAP_ID_2" == "null" ]] && fail "Swap #2 não retornou swap_id: $swap_resp_2"
ok "Swap #2 submetido — swap_id: ${SWAP_ID_2}"
info "  correlation_id: ${CORRELATION_ID_2}"

# 6c. Aguardar processamento
log "Aguardando ${SWAP_WAIT_SECONDS}s para processamento do bridge-out..."
sleep "$SWAP_WAIT_SECONDS"

# 6d. Verificar status
print_request "GET" "${BANK_A_URL}/api/v2/amm/swap/cross-currency/${SWAP_ID_2}"
status_resp_2=$(api_call GET "${BANK_A_URL}/api/v2/amm/swap/cross-currency/${SWAP_ID_2}" "$TOKEN_BANK_A")
print_response "$status_resp_2"
SWAP_STATUS_2=$(jfield "$status_resp_2" "status")
ok "Swap #2 status: ${SWAP_STATUS_2}"

# ─────────────────────────────────────────────────────────────────────────────
# ETAPA 7: Validação final de saldos
# ─────────────────────────────────────────────────────────────────────────────

step "Validação final de saldos"

show_balances "FINAIS (após 2 swaps)"

log "Saldo tCeBM Bank-A (Spoke-A):"
print_request "GET" "${BANK_A_URL}/api/v1/token/balance" "-"
fiat_a=$(api_call GET "${BANK_A_URL}/api/v1/token/balance" "$TOKEN_BANK_A")
print_response "$fiat_a"

log "Saldo tCeBM Bank-B (Spoke-B):"
print_request "GET" "${BANK_B_URL}/api/v1/token/balance" "-"
fiat_b=$(api_call GET "${BANK_B_URL}/api/v1/token/balance" "$TOKEN_BANK_B")
print_response "$fiat_b"

# ─────────────────────────────────────────────────────────────────────────────
# Resumo
# ─────────────────────────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║                       RESUMO DO TRYOUT                          ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BOLD}Pool:${NC} ${POOL_PAIR}"
echo ""
echo -e "${BOLD}Swap #1${NC}"
echo    "  swap_id:        ${SWAP_ID_1:-N/A}"
echo    "  correlation_id: ${CORRELATION_ID_1:-N/A}"
echo    "  amount_in:      ${AMOUNT_IN_1:-N/A} wei W-BRL"
echo    "  amount_out:     ${SWAP_AMOUNT} wei W-ARS"
echo    "  status:         ${SWAP_STATUS_1:-N/A}"
echo ""
echo -e "${BOLD}Swap #2${NC}"
echo    "  swap_id:        ${SWAP_ID_2:-N/A}"
echo    "  correlation_id: ${CORRELATION_ID_2:-N/A}"
echo    "  amount_in:      ${AMOUNT_IN_2:-N/A} wei W-BRL"
echo    "  amount_out:     ${SWAP_AMOUNT} wei W-ARS"
echo    "  status:         ${SWAP_STATUS_2:-N/A}"
echo ""

if [[ "${SWAP_STATUS_1:-}" == "COMPLETED" && "${SWAP_STATUS_2:-}" == "COMPLETED" ]]; then
    echo -e "${BOLD}${GREEN}✓ AMBOS OS SWAPS COMPLETADOS COM SUCESSO${NC}"
elif [[ "${SWAP_STATUS_1:-}" == "IN_PROGRESS" || "${SWAP_STATUS_2:-}" == "IN_PROGRESS" ]]; then
    warn "Alguns swaps ainda estão em processamento (bridge-out assíncrono)"
    warn "Aumente SWAP_WAIT_SECONDS (atual: ${SWAP_WAIT_SECONDS}s) ou verifique os logs:"
    info "  docker logs backend-payment-orchestrator-central-bank-b | tail -20"
else
    warn "Status dos swaps: Swap#1=${SWAP_STATUS_1:-?} Swap#2=${SWAP_STATUS_2:-?}"
    warn "Verifique os logs do payment-orchestrator CB-B para diagnóstico"
fi

echo ""
info "Logs úteis para diagnóstico:"
info "  docker logs backend-api-gateway-bank-a 2>&1 | tail -20"
info "  docker logs backend-payment-orchestrator-central-bank-b 2>&1 | tail -20"
info "  docker logs cbweb3-cacti-liquidity-relay 2>&1 | tail -20"
echo ""
