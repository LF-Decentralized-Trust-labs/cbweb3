#!/usr/bin/env bash
#
# sample-tryout.sh — end-to-end walkthrough of the deployed Scenario B sample via
# REAL api-gateway REST calls (no portal clicking). Drives the sovereign FX
# corridor lifecycle and a full cross-currency SWAP (bridge-in → AMM → bridge-out):
#   login (CBs + a commercial bank) → verify registered currencies → open the
#   BRL↔ARS corridor (the hub deploys the sovereign AMM + registers the pair) →
#   each central bank deposits its own side of liquidity → finalize → wait pool
#   ACTIVE → bank-itau tokenises reserves (deposit → escrow → CB approve = tCeBM) →
#   the commercial bank runs an end-to-end cross-currency swap: CB-A relayer mints
#   W-BRL (bridge-in, backed by the tokenised reserves), the sovereign AMM swaps to
#   W-ARS, and CB-B's relayer burns W-ARS (bridge-out) for the beneficiary.
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
HUB="http://localhost:41845"          # hub-cbweb3
BR_CB="http://localhost:41645"        # central-bank-brazil   (issues W-tCeBM_BRL)
AR_CB="http://localhost:41745"        # central-bank-argentina (issues W-tCeBM_ARS)
ITAU="http://localhost:41646"         # bank-itau (Brazil) — the swapping bank
MACRO="http://localhost:41747"        # bank-macro (Argentina) — the beneficiary bank

# ── seeded login credentials (username / password) — per-entity, from each
# manifest's spec.adminUsers (scenario-a naming). The CB governance operator carries
# the central_bank + ROLE_GOVERNANCE roles (drives the sovereign AMM + KYC); the bank
# operator carries commercial_bank + ROLE_COMMERCIAL_BANK.
BR_CB_USER="admin@brasil.governance.gov";     BR_CB_PASS="brasil-governance-local"
AR_CB_USER="admin@argentina.governance.gov";  AR_CB_PASS="argentina-governance-local"
ITAU_USER="admin@itau.brasil.com";            ITAU_PASS="itau-bank-local"
MACRO_USER="admin@macro.argentina.com";       MACRO_PASS="macro-bank-local"

# ── corridor parameters ─────────────────────────────────────────────────────────
CUR_A="BRL"; CUR_B="ARS"
POOL="W-${CUR_A}-W-${CUR_B}"                  # pair id convention: W-{source}-W-{target}
RELAY_SECRET="cbweb3-relay-shared-secret"     # X-Relay-Auth for the hub M2M endpoints
LIQ="1000000000000000000000"                  # 1000 tokens per side (18 decimals)
# Cross-currency swap: exact 5 W-ARS out for the ARS beneficiary, accept up to 6 W-BRL.
CC_OUT="5000000000000000000"                  # want 5 W-ARS delivered on Spoke-B
CC_MAX_IN="6000000000000000000"               # accept up to 6 W-BRL in

# ── beneficiary (Argentina) ──────────────────────────────────────────────────────
# The ARS bank that receives the swapped W-ARS. It onboards through the governance
# portal at its own central bank (CB-B) — the onboarding derives its distinct on-chain
# wallet — so the bridge-out can resolve it as an ACTIVE participant.
BENEF_BANK="bank-macro"

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

# relogin — re-mint every access token from its username/password. The Keycloak
# access token lifespan is ~300s, but this script's on-chain phases (onboarding,
# liquidity seeding, and the cross-currency swap — which alone allows up to 180s)
# can out-live a token minted at login, causing a mid-run 401 "invalid token".
# A browser portal refreshes the token silently; a curl script has no such loop,
# so we re-login explicitly at the start of each long phase.
relogin() {
  BR_TOK=$(login "$BR_CB" "$BR_CB_USER" "$BR_CB_PASS")
  AR_TOK=$(login "$AR_CB" "$AR_CB_USER" "$AR_CB_PASS")
  ITAU_TOK=$(login "$ITAU" "$ITAU_USER" "$ITAU_PASS")
  MACRO_TOK=$(login "$MACRO" "$MACRO_USER" "$MACRO_PASS")
}

# pool_status TOKEN — echoes the pool status string (EMPTY/ACTIVE/…) for $POOL.
pool_status() { try GET "$BR_CB/api/v2/amm/pool/$POOL/status" "$1"; printf '%s' "$BODY" | jget pool_status; }

# onboard LABEL BANK_URL BANK_TOK CB_URL CB_TOK INSTITUTION COUNTRY EMAIL USERNAME
# Drives the governance-portal onboarding (mirrors scenario-a): the bank initiates
# (the api-gateway smart proxy injects the CSR + KMS key), the CB approves KYC, and
# the bank completes (PoP signature → CB-signed cert → on-chain participant). The bank
# ends ACTIVE with its own distinct on-chain wallet. Idempotent: skips if already ACTIVE.
onboard() {
  local label=$1 bank_url=$2 bank_tok=$3 cb_url=$4 cb_tok=$5 inst=$6 country=$7 email=$8 user=$9 subj
  try GET "$bank_url/api/v1/onboarding/my-status" "$bank_tok"
  [[ $(printf '%s' "$BODY" | jget status) == ACTIVE ]] && { ok "$label already ACTIVE — skipping onboarding"; return 0; }
  # initiate; on 409 (Keycloak user already exists from a partial run) recover the subject.
  try POST "$bank_url/api/v1/onboarding/initiate" "$bank_tok" \
    "{\"institution_name\":\"$inst\",\"country\":\"$country\",\"role\":\"ROLE_COMMERCIAL_BANK\",\"email\":\"$email\",\"username\":\"$user\"}"
  if [[ -n $CODE && $CODE -ge 200 && $CODE -lt 300 ]]; then
    subj=$(printf '%s' "$BODY" | jget user_id)
  elif [[ $CODE -eq 409 ]]; then
    try GET "$bank_url/api/v1/onboarding/my-status" "$bank_tok"; subj=$(printf '%s' "$BODY" | jget user_id)
  else
    die "$label initiate HTTP $CODE: $BODY"
  fi
  [[ -n $subj ]] || die "$label onboarding: no subject/user_id"
  ok "$label credential requested (subject=$subj, wallet=$(printf '%s' "$BODY" | jget wallet_address))"
  # CB governance approves KYC (route lives under /compliance/ in scenario-b).
  call POST "$cb_url/api/v1/compliance/approve-kyc" "$cb_tok" "{\"subject\":\"$subj\",\"reason\":\"sample-tryout onboarding approval\"}"
  ok "$label KYC approved by its central bank"
  # Bank completes: PoP signature → CB signs the CSR + registers the participant on-chain.
  call POST "$bank_url/api/v1/onboarding/complete" "$bank_tok" "{\"request_id\":\"$subj\",\"user_id\":\"$subj\"}"
  ok "$label onboarding COMPLETE — participant ACTIVE with its own wallet"
}

printf '%s%s cbweb3 Scenario B — sample tryout (cross-currency swap) %s\n' "$BOLD" "════════" "$RST"

# ═══════════════════════════════ LOGIN ══════════════════════════════════════════
step "Login — Brazil CB, Argentina CB, and bank-itau"
BR_TOK=$(login "$BR_CB" "$BR_CB_USER" "$BR_CB_PASS");   ok "logged in at Brazil CB (governance)"
AR_TOK=$(login "$AR_CB" "$AR_CB_USER" "$AR_CB_PASS");   ok "logged in at Argentina CB (governance)"
ITAU_TOK=$(login "$ITAU" "$ITAU_USER" "$ITAU_PASS");    ok "logged in as bank-itau (commercial_bank, Brazil)"
MACRO_TOK=$(login "$MACRO" "$MACRO_USER" "$MACRO_PASS"); ok "logged in as bank-macro (commercial_bank, Argentina)"

# ═══════════════════════════════ ONBOARDING ═════════════════════════════════════
# Each commercial bank onboards through its central bank's governance portal before
# transacting (mirrors scenario-a): initiate → CB approve-kyc → complete → ACTIVE.
# bank-itau is the swap initiator (Brazil); bank-macro is the beneficiary (Argentina)
# whose ACTIVE participant record lets the bridge-out resolve its on-chain wallet.
step "Onboard the commercial banks through their central banks' governance portals"
onboard "bank-itau"  "$ITAU"  "$ITAU_TOK"  "$BR_CB" "$BR_TOK" "Banco Itau"  "BR" "ops@itau.br"      "bank-itau-user"
relogin  # itau's on-chain onboarding (approve-kyc + complete) may have aged the tokens
onboard "bank-macro" "$MACRO" "$MACRO_TOK" "$AR_CB" "$AR_TOK" "Banco Macro" "AR" "ops@macro.ar"     "bank-macro-user"

# ═══════════════════════════════ CURRENCIES ═════════════════════════════════════
# W-tokens are deployed + registered at found-spoke by the hub compliance service,
# so they already exist here — no runtime currency step.
step "Verify the sovereign currencies are registered on the hub"
relogin
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
# Sovereign escrow-and-finalize seeding: each central bank deposits ONLY its own side
# of the pool, then a single finalize funds both reserves atomically. This replaces the
# legacy dual-sided /liquidity/add (removed — one CB seeding both sides breached
# sovereignty). deposit-side mints + approves the caller CB's own W-token internally and
# resolves the side on-chain from pool_pair, so no separate mint-and-approve step is needed.
step "Each central bank sovereignly deposits its own side into $POOL"
relogin
call POST "$BR_CB/api/v2/amm/liquidity/deposit-side" "$BR_TOK" \
  "{\"pool_pair\":\"$POOL\",\"amount\":\"$LIQ\"}"
ok "Brazil CB deposited side $(printf '%s' "$BODY" | jget side) ($LIQ)"
call POST "$AR_CB/api/v2/amm/liquidity/deposit-side" "$AR_TOK" \
  "{\"pool_pair\":\"$POOL\",\"amount\":\"$LIQ\"}"
ok "Argentina CB deposited side $(printf '%s' "$BODY" | jget side) ($LIQ)"

step "Finalize the pool once both sides are escrowed (funds reserves atomically)"
call POST "$BR_CB/api/v2/amm/liquidity/finalize" "$BR_TOK" \
  "{\"pool_pair\":\"$POOL\"}"
ok "liquidity finalized (shares_a=$(printf '%s' "$BODY" | jget shares_a) shares_b=$(printf '%s' "$BODY" | jget shares_b))"

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

# ═══════════════════════════ RESERVE TOKENISATION ═══════════════════════════════
# The cross-currency bridge-in locks the payer bank's tCeBM to back the minted W-token
# — it does NOT mint tCeBM on the fly (reserve backing is enforced). So bank-itau must
# hold >= max_amount_in tCeBM_${CUR_A} BEFORE swapping. It gets there via the escrow
# flow, all amounts 1:1 (18-decimal wei) and approvals synchronous:
#   deposit fiat (bank) → CB approves = mint fCeBM → request escrow (bank) → CB approves
#   = burn fCeBM + mint tCeBM. Steps run on the bank gateway (ITAU) and are approved on
#   the central bank gateway (BR_CB governance operator).
step "bank-itau tokenises reserves — register a fiat deposit ($CC_MAX_IN)"
relogin
call POST "$ITAU/api/v1/payments/deposits" "$ITAU_TOK" "{\"amount\":\"$CC_MAX_IN\"}"
DEP_ID=$(printf '%s' "$BODY" | jget deposit_id)
[[ -n $DEP_ID ]] || die "no deposit_id in response: $BODY"
ok "deposit registered (deposit_id=$DEP_ID)"

call POST "$BR_CB/api/v1/payments/deposits/approve" "$BR_TOK" "{\"deposit_id\":\"$DEP_ID\"}"
ok "Brazil CB approved the deposit — fCeBM minted (tx=$(printf '%s' "$BODY" | jget fiat_mint_tx_hash))"

step "bank-itau escrows the fCeBM for tokenisation, and the CB approves (burn fCeBM → mint tCeBM)"
call POST "$ITAU/api/v1/payments/escrows" "$ITAU_TOK" "{\"deposit_id\":\"$DEP_ID\",\"amount\":\"$CC_MAX_IN\"}"
ESC_ID=$(printf '%s' "$BODY" | jget escrow_id)
[[ -n $ESC_ID ]] || die "no escrow_id in response: $BODY"
ok "escrow requested (escrow_id=$ESC_ID)"

call POST "$BR_CB/api/v1/payments/escrows/approve" "$BR_TOK" "{\"escrow_id\":\"$ESC_ID\"}"
ok "Brazil CB approved the escrow — tCeBM minted (burn=$(printf '%s' "$BODY" | jget burn_tx_hash) mint=$(printf '%s' "$BODY" | jget mint_tx_hash))"

step "Verify bank-itau now holds enough tCeBM_${CUR_A} to back the bridge-in"
call GET "$ITAU/api/v1/token/balance" "$ITAU_TOK"
BAL=$(printf '%s' "$BODY" | jget balance)
python3 -c "import sys; sys.exit(0 if int('${BAL:-0}') >= int('$CC_MAX_IN') else 1)" \
  || die "bank-itau tCeBM balance $BAL < required $CC_MAX_IN — reserve tokenisation did not settle"
ok "bank-itau holds $BAL $(printf '%s' "$BODY" | jget symbol) (>= $CC_MAX_IN required)"

# ═══════════════════════════ CROSS-CURRENCY SWAP (BRIDGE) ════════════════════════
# The full lifecycle, orchestrated by bank-itau's gateway:
#   Step 1 Bridge-In  — delegated to CB-A; CB-A's relayer mints W-${CUR_A} on the hub.
#   Step 2 AMM Swap   — W-${CUR_A} → W-${CUR_B} on the sovereign pool.
#   Step 3 Bridge-Out — via the Cacti relay to CB-B; CB-B's relayer burns W-${CUR_B}
#                       and releases to the beneficiary on Spoke-B (release skipped in
#                       hub-only local mode).
step "bank-itau quotes ${CUR_A} → ${CUR_B} (exact output $CC_OUT W-${CUR_B})"
relogin  # the swap that follows may take up to 180s — start it with a fresh token
call GET "$ITAU/api/v2/amm/quote/cross-currency?source_currency=${CUR_A}&target_currency=${CUR_B}&amount_out=${CC_OUT}&max_slippage_pct=0.05" "$ITAU_TOK"
ok "quote: pay $(printf '%s' "$BODY" | jget amount_in) W-${CUR_A} for $CC_OUT W-${CUR_B} (rate=$(printf '%s' "$BODY" | jget effective_rate))"

step "bank-itau executes the end-to-end cross-currency swap (bridge-in → swap → bridge-out)"
call POST "$ITAU/api/v2/amm/swap/cross-currency" "$ITAU_TOK" \
  "{\"source_currency\":\"$CUR_A\",\"target_currency\":\"$CUR_B\",\"pool_pair\":\"$POOL\",\"amount_out\":\"$CC_OUT\",\"max_amount_in\":\"$CC_MAX_IN\",\"beneficiary_bank_id\":\"$BENEF_BANK\"}"
SWAP_STATUS=$(printf '%s' "$BODY" | jget status)
[[ $SWAP_STATUS == COMPLETED ]] || die "cross-currency swap not COMPLETED (status=$SWAP_STATUS): $BODY"
ok "swap COMPLETED: in=$(printf '%s' "$BODY" | jget amount_in) out=$(printf '%s' "$BODY" | jget amount_out)"
ok "  bridge-in position=$(printf '%s' "$BODY" | jget bridge_in_position_id)"
ok "  hub AMM swap tx=$(printf '%s' "$BODY" | jget swap_tx_hash)"
ok "  bridge-out position=$(printf '%s' "$BODY" | jget bridge_out_position_id)"

step "Confirm the pool reserves moved (constant-product swap)"
call GET "$BR_CB/api/v2/amm/pool/$POOL/status" "$BR_TOK"
ok "reserves now A=$(printf '%s' "$BODY" | jget reserve_a) B=$(printf '%s' "$BODY" | jget reserve_b)"

# ═══════════════════════════ REDEEM (DE-TOKENISATION) ════════════════════════════
# Redeem is the exact inverse of the escrow tokenisation: on CB approval it BURNS the
# bank's tCeBM and mints the equivalent fCeBM (fiat) back — so a redeem must DECREASE
# the bank's tCeBM balance. This section is a regression guard for the reported bug
# where redeem MINTED tCeBM instead (balance went UP). It is self-contained: bank-itau
# tokenises a fresh amount, we snapshot its tCeBM balance, redeem part of it, and assert
# the balance dropped by exactly the redeemed amount.
REDEEM_FUND="4000000000000000000"   # tokenise 4 tCeBM of fresh headroom to redeem from
REDEEM_AMT="1000000000000000000"    # de-tokenise (redeem) 1 tCeBM back to fiat

step "bank-itau tokenises fresh reserves to redeem ($REDEEM_FUND)"
relogin
call POST "$ITAU/api/v1/payments/deposits" "$ITAU_TOK" "{\"amount\":\"$REDEEM_FUND\"}"
R_DEP_ID=$(printf '%s' "$BODY" | jget deposit_id)
[[ -n $R_DEP_ID ]] || die "no deposit_id for redeem funding: $BODY"
call POST "$BR_CB/api/v1/payments/deposits/approve" "$BR_TOK" "{\"deposit_id\":\"$R_DEP_ID\"}"
call POST "$ITAU/api/v1/payments/escrows" "$ITAU_TOK" "{\"deposit_id\":\"$R_DEP_ID\",\"amount\":\"$REDEEM_FUND\"}"
R_ESC_ID=$(printf '%s' "$BODY" | jget escrow_id)
[[ -n $R_ESC_ID ]] || die "no escrow_id for redeem funding: $BODY"
call POST "$BR_CB/api/v1/payments/escrows/approve" "$BR_TOK" "{\"escrow_id\":\"$R_ESC_ID\"}"
ok "fresh tCeBM tokenised (deposit_id=$R_DEP_ID escrow_id=$R_ESC_ID)"

step "Snapshot bank-itau tCeBM balance before the redeem"
call GET "$ITAU/api/v1/token/balance" "$ITAU_TOK"
BAL_BEFORE=$(printf '%s' "$BODY" | jget balance)
[[ -n $BAL_BEFORE ]] || die "no tCeBM balance before redeem: $BODY"
ok "tCeBM balance before redeem = $BAL_BEFORE"

step "bank-itau requests a redeem (de-tokenisation) of $REDEEM_AMT tCeBM → fiat"
call POST "$ITAU/api/v1/payments/redeems" "$ITAU_TOK" "{\"amount\":\"$REDEEM_AMT\"}"
REDEEM_ID=$(printf '%s' "$BODY" | jget redeem_id)
[[ -n $REDEEM_ID ]] || die "no redeem_id in response: $BODY"
ok "redeem requested (redeem_id=$REDEEM_ID)"

step "Brazil CB approves the redeem (burns tCeBM → mints fCeBM/fiat back)"
call POST "$BR_CB/api/v1/payments/redeems/approve" "$BR_TOK" "{\"redeem_id\":\"$REDEEM_ID\"}"
ok "redeem approved (fiat_mint_tx=$(printf '%s' "$BODY" | jget mint_tx_hash))"

step "Verify the redeem BURNED tCeBM (balance went DOWN, not up)"
call GET "$ITAU/api/v1/token/balance" "$ITAU_TOK"
BAL_AFTER=$(printf '%s' "$BODY" | jget balance)
[[ -n $BAL_AFTER ]] || die "no tCeBM balance after redeem: $BODY"
info "tCeBM balance after redeem = $BAL_AFTER"
python3 -c "import sys; sys.exit(0 if int('${BAL_AFTER:-0}') == int('${BAL_BEFORE:-0}') - int('$REDEEM_AMT') else 1)" \
  || die "redeem did not burn tCeBM correctly: before=$BAL_BEFORE after=$BAL_AFTER (redeem must DECREASE tCeBM by $REDEEM_AMT, not increase it)"
ok "redeem correctly burned $REDEEM_AMT tCeBM: $BAL_BEFORE → $BAL_AFTER (converted to fiat)"

printf '\n%s✓ tryout complete — %s corridor opened via the hub, liquidity seeded, an end-to-end cross-currency swap (bridge-in → AMM → bridge-out) settled across both sovereign networks, and a redeem de-tokenised tCeBM back to fiat%s\n' "$GREEN$BOLD" "$POOL" "$RST"
