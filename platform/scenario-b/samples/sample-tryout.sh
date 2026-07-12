#!/usr/bin/env bash
#
# sample-tryout.sh — end-to-end walkthrough of the deployed Scenario B sample via
# REAL api-gateway REST calls (no portal clicking). Drives the sovereign FX
# corridor lifecycle and a cross-currency SWAP:
#   login (CBs + a commercial bank) → verify registered currencies → open the
#   BRL↔ARS corridor (the hub deploys the sovereign AMM + registers the pair) →
#   central bank adds cooperative liquidity → wait pool ACTIVE → the commercial
#   bank quotes and executes a W-BRL → W-ARS swap on the sovereign pool.
#
# Everything resolves the sovereign AMM DYNAMICALLY per pool_pair from the
# on-chain PairRegistry, so a corridor opened at runtime works with no config.
#
# Prereqs: the sample is up (./deploy-all.sh) and python3 is on PATH.
# Auth: POST /api/v1/auth/login (clientId/clientSecret = the seeded Keycloak
# user's username/password); the returned accessToken is reused as the
# access_token cookie. Any failed HTTP call prints in RED and stops.
set -uo pipefail

# ── entity api-gateways (host port = besu RPC port + 8000) ──────────────────────
HUB="http://localhost:16845"          # hub-cbweb3
BR_CB="http://localhost:16645"        # central-bank-brazil   (issues W-tCeBM_BRL)
AR_CB="http://localhost:16745"        # central-bank-argentina (issues W-tCeBM_ARS)
ITAU="http://localhost:16646"         # bank-itau (Brazil) — the swapping bank

# ── seeded login credentials (username / password) ─────────────────────────────
CB_USER="cb-admin";   CB_PASS="cb-admin-local"        # ROLE central_bank + governance
BANK_USER="bank-admin"; BANK_PASS="bank-admin-local"  # ROLE commercial_bank

# ── corridor parameters ─────────────────────────────────────────────────────────
CUR_A="BRL"; CUR_B="ARS"
POOL="W-${CUR_A}-W-${CUR_B}"                  # pair id convention: W-{source}-W-{target}
RELAY_SECRET="cbweb3-relay-shared-secret"     # X-Relay-Auth for the hub M2M endpoints
LIQ="1000000000000000000000"                  # 1000 tokens per side (18 decimals)
SWAP_OUT="10000000000000000000"               # want 10 W-ARS
SWAP_MAX_IN="11000000000000000000"            # accept up to 11 W-BRL in
SWAP_FUND="100000000000000000000"             # 100 W-BRL funded to the bank for the swap

# ── output helpers ─────────────────────────────────────────────────────────────
RED=$'\033[31m'; GREEN=$'\033[32m'; BOLD=$'\033[1m'; DIM=$'\033[2m'; RST=$'\033[0m'
# Progress goes to stderr so command-substitution ($(...)) captures only data.
STEP=0
step() { STEP=$((STEP+1)); printf '\n%s══ STEP %s: %s%s\n' "$BOLD" "$STEP" "$*" "$RST" >&2; }
info() { printf '   %s%s%s\n' "$DIM" "$*" "$RST" >&2; }
ok()   { printf '   %s✓ %s%s\n' "$GREEN" "$*" "$RST" >&2; }
die()  { printf '%s✗ %s%s\n' "$RED" "$*" "$RST" >&2; exit 1; }

# jget FIELD — read a (optionally dotted) field from JSON on stdin
jget() {
  python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception:
    print(""); sys.exit(0)
for k in sys.argv[1].split("."):
    d = d.get(k, "") if isinstance(d, dict) else ""
print(d if d is not None else "")
' "$1"
}

BODY=""; CODE=""

# try METHOD URL TOKEN [BODY] [HEADER] — curl; sets BODY+CODE; never stops on non-2xx.
try() {
  local method=$1 url=$2 token=$3 body=${4:-} header=${5:-}
  local args=(-sS -m 180 -w $'\n%{http_code}' -X "$method" -H 'Content-Type: application/json')
  [[ -n $token ]]  && args+=(-b "access_token=$token")
  [[ -n $body ]]   && args+=(-d "$body")
  [[ -n $header ]] && args+=(-H "$header")
  local out; out=$(curl "${args[@]}" "$url") || die "curl failed: $method $url"
  CODE=${out##*$'\n'}; BODY=${out%$'\n'*}
  info "$method $url -> HTTP $CODE"
}

# call METHOD URL TOKEN [BODY] [HEADER] — try() + RED-and-stop on any non-2xx.
call() {
  try "$@"
  [[ -n $CODE && $CODE -ge 200 && $CODE -lt 300 ]] || die "HTTP $CODE from $1 $2
    response: $BODY"
}

# login GATEWAY USER PASS — echoes accessToken.
login() {
  local out tok
  out=$(curl -sS -m 30 -X POST "$1/api/v1/auth/login" -H 'Content-Type: application/json' \
        -d "{\"clientId\":\"$2\",\"clientSecret\":\"$3\"}") || die "login curl failed for $2"
  tok=$(printf '%s' "$out" | jget accessToken)
  [[ -n $tok ]] || die "login failed for $2: $out"
  printf '%s' "$tok"
}

# pool_status TOKEN — echoes the pool status string (EMPTY/ACTIVE/…) for $POOL.
pool_status() { try GET "$BR_CB/api/v2/amm/pool/$POOL/status" "$1"; printf '%s' "$BODY" | jget pool_status; }

printf '%s%s cbweb3 Scenario B — sample tryout (cross-currency swap) %s\n' "$BOLD" "════════" "$RST"

# ═══════════════════════════════ LOGIN ══════════════════════════════════════════
step "Login — Brazil CB, Argentina CB, and bank-itau"
BR_TOK=$(login "$BR_CB" "$CB_USER" "$CB_PASS");   ok "logged in at Brazil CB"
AR_TOK=$(login "$AR_CB" "$CB_USER" "$CB_PASS");   ok "logged in at Argentina CB"
ITAU_TOK=$(login "$ITAU" "$BANK_USER" "$BANK_PASS"); ok "logged in as bank-itau (commercial_bank)"

# ═══════════════════════════════ CURRENCIES ═════════════════════════════════════
# W-tokens are deployed + registered at found-spoke by the hub compliance service,
# so they already exist here — no runtime currency step.
step "Verify the sovereign currencies are registered on the hub"
call GET "$BR_CB/api/v2/hub/currencies" "$BR_TOK"
info "currencies: $BODY"
printf '%s' "$BODY" | grep -q "W-tCeBM_${CUR_A}" || die "W-tCeBM_${CUR_A} not registered"
printf '%s' "$BODY" | grep -q "W-tCeBM_${CUR_B}" || die "W-tCeBM_${CUR_B} not registered"
ok "W-tCeBM_${CUR_A} and W-tCeBM_${CUR_B} are registered"

# ═══════════════════════════════ OPEN CORRIDOR ══════════════════════════════════
# The hub deploys the sovereign-pair AMM over the two registered W-tokens and
# registers the pair (proposePair + confirmPair). CBs never touch the hub chain;
# the hub governance signer performs the on-chain acts. Idempotent by pair id.
step "Open the ${CUR_A}↔${CUR_B} corridor (hub deploys the sovereign AMM + registers the pair)"
call POST "$HUB/internal/v1/spokes/register-pair" "" \
  "{\"currency_a\":\"$CUR_A\",\"currency_b\":\"$CUR_B\",\"pair_id\":\"$POOL\"}" \
  "X-Relay-Auth: $RELAY_SECRET"
AMM=$(printf '%s' "$BODY" | jget amm_address)
ok "corridor $POOL ready (amm=$AMM already_registered=$(printf '%s' "$BODY" | jget already_registered))"

# ═══════════════════════════════ LIQUIDITY ══════════════════════════════════════
# In this local sample the central bank issues both sovereign W-tokens, so it
# seeds both sides of the pool. mint-and-approve + addLiquidity resolve the pair's
# tokens + AMM dynamically from pool_pair.
step "Central bank seeds cooperative liquidity into $POOL (both sides)"
for side in A B; do
  call POST "$BR_CB/api/v2/amm/token/mint-and-approve" "$BR_TOK" \
    "{\"pool_pair\":\"$POOL\",\"amount\":\"$LIQ\",\"side\":\"$side\"}"
  ok "minted + approved side $side ($LIQ)"
done
call POST "$BR_CB/api/v2/amm/liquidity/add" "$BR_TOK" \
  "{\"pool_pair\":\"$POOL\",\"provider_bank_id\":\"central-bank\",\"token_a_amount\":\"$LIQ\",\"token_b_amount\":\"$LIQ\"}"
ok "liquidity added (lp_id=$(printf '%s' "$BODY" | jget lp_id))"

step "Wait for the pool to be ACTIVE"
ST=""
for i in 1 2 3 4 5; do
  ST=$(pool_status "$BR_TOK")
  info "attempt $i: pool_status=$ST"
  [[ $ST == ACTIVE ]] && break
  sleep 3
done
[[ $ST == ACTIVE ]] || die "pool $POOL not ACTIVE (status=$ST)"
call GET "$BR_CB/api/v2/amm/pool/$POOL/status" "$BR_TOK"
ok "pool ACTIVE — reserves A=$(printf '%s' "$BODY" | jget reserve_a) B=$(printf '%s' "$BODY" | jget reserve_b)"

# ═══════════════════════════════ FUND THE BANK ══════════════════════════════════
# The commercial bank pays W-${CUR_A} into the swap. In production it obtains it by
# bridging in (lock ${CUR_A} on its spoke → mint W-${CUR_A} on the hub via its CB).
# In this AMM-focused sample the issuing CB mints W-${CUR_A} to the swapper and
# approves the pool's AMM, standing in for the bridge-in leg.
step "Fund bank-itau with W-${CUR_A} for the swap (stands in for bridge-in)"
call POST "$BR_CB/api/v2/amm/token/mint-and-approve" "$BR_TOK" \
  "{\"pool_pair\":\"$POOL\",\"amount\":\"$SWAP_FUND\",\"side\":\"A\"}"
ok "funded + approved $SWAP_FUND W-${CUR_A}"

# ═══════════════════════════════ QUOTE + SWAP ═══════════════════════════════════
step "bank-itau quotes ${CUR_A} → ${CUR_B} (exact output $SWAP_OUT W-${CUR_B})"
call GET "$ITAU/api/v2/amm/quote/cross-currency?source_currency=${CUR_A}&target_currency=${CUR_B}&amount_out=${SWAP_OUT}&max_slippage_pct=0.05" "$ITAU_TOK"
QIN=$(printf '%s' "$BODY" | jget amount_in)
ok "quote: pay $QIN W-${CUR_A} for $SWAP_OUT W-${CUR_B} (rate=$(printf '%s' "$BODY" | jget effective_rate))"

step "bank-itau executes the swap on the sovereign pool"
call POST "$ITAU/api/v2/amm/swap/exact-output" "$ITAU_TOK" \
  "{\"pair\":\"$POOL\",\"amount_out\":\"$SWAP_OUT\",\"max_amount_in\":\"$SWAP_MAX_IN\",\"payer_id\":\"bank-itau\",\"beneficiary_id\":\"bank-itau\"}"
ok "swap $(printf '%s' "$BODY" | jget state): amount_in=$(printf '%s' "$BODY" | jget amount_in) tx=$(printf '%s' "$BODY" | jget tx_hash)"

step "Confirm the pool reserves moved (constant-product swap)"
call GET "$BR_CB/api/v2/amm/pool/$POOL/status" "$BR_TOK"
ok "reserves now A=$(printf '%s' "$BODY" | jget reserve_a) B=$(printf '%s' "$BODY" | jget reserve_b)"

printf '\n%s✓ tryout complete — %s corridor opened via the hub, liquidity seeded, and a cross-currency swap settled on the sovereign AMM%s\n' "$GREEN$BOLD" "$POOL" "$RST"
