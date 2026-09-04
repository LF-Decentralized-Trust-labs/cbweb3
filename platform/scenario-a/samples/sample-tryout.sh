#!/usr/bin/env bash
#
# sample-tryout.sh — end-to-end walkthrough of the deployed sample stack via REAL
# api-gateway REST calls (no portal clicking). Drives: onboarding (Brazil + Costa Rica),
# reserve issuance, tokenization, balances, a cross-spoke FX agreement, relay, accept,
# and the PvP leg.
#
# Prereqs: the samples are up (./deploy-all.sh) and python3 is on PATH.
# Auth model: each "login" calls the api-gateway's real POST /api/v1/auth/login
# (grant_type=password under the hood — the clientId/clientSecret JSON fields are the
# user's username/password) and reuses the returned accessToken as the `access_token`
# cookie for subsequent calls.
#
# Any failed HTTP call prints the error in RED and stops immediately.

set -uo pipefail

# ── entity api-gateways ────────────────────────────────────────────────────────
BR_CB="http://localhost:18645"          # central-bank-brazil
ITAU="http://localhost:18646"           # bank-itau
BRADESCO="http://localhost:18647"       # bank-bradesco
# Costa Rica carries the LNET naming convention on purpose — a hyphenated spoke id
# and bare bank ids (cb1, cb2). Code that recovers a bank or spoke id by splitting a
# node name on "-" is right for spoke-brl-bank-itau and wrong for
# spoke-costa-rica-cb1, and this leg is what makes that reachable from a local run.
#
# IF YOU EVER RENAME THIS COUNTRY BACK: rename the IDENTIFIERS first, then the
# country. "BANCOLOMBIA" contains "COLOMBIA", so replacing the country first turns
# it into `BANCOSTA RICA="..."` — which `bash -n` ACCEPTS, because it parses as a
# command plus an argument, so the script breaks only at runtime. Anchored regexes
# do not save you either: "_" is a word character, so \bNAME\b misses NAME_TOK and
# \bNAME misses ID_NAME.
CR_CB="http://localhost:18945"          # central-bank-costa-rica
CB1="http://localhost:18946"    # cb1 (Costa Rica, bankId nu)
CB2="http://localhost:18947"     # cb2 (Costa Rica, bankId nu)

# ── login credentials (username / password; passed to /auth/login) ─────────────
# Brazil CB
# approve-kyc is gated on ROLE_ADMISSION (spec 042), which the governance user does not
# hold — with a governance token it answers 403 {"required_role":["ROLE_ADMISSION"]}.
BR_ADM_USER="admin@brasil.admission.gov";   BR_ADM_PASS="brasil-admission-local"
BR_TRE_USER="admin@brasil.treasury.gov";    BR_TRE_PASS="brasil-treasury-local"
# Costa Rica CB
CR_ADM_USER="admin@costarica.admission.gov";  CR_ADM_PASS="costarica-admission-local"
CR_TRE_USER="admin@costarica.treasury.gov";   CR_TRE_PASS="costarica-treasury-local"
# Banks
ITAU_USER="admin@itau.brasil.com";               ITAU_PASS="itau-bank-local"
BRADESCO_USER="admin@bradesco.brasil.com";       BRADESCO_PASS="bradesco-bank-local"
CB1_USER="admin@cb1.costarica.com"; CB1_PASS="cb1-bank-local"
CB2_USER="admin@cb2.costarica.com";   CB2_PASS="cb2-bank-local"

# ── on-chain Paladin identities used in the FX agreement routing ────────────────
ID_CB1="funded_operator@spoke-costa-rica-cb1"
ID_BR_CB="funded_operator@spoke-brl-cb"
ID_CB2="funded_operator@spoke-costa-rica-cb2"
ID_BRADESCO="funded_operator@spoke-brl-bank-bradesco"

# ── output helpers ─────────────────────────────────────────────────────────────
RED=$'\033[31m'; GREEN=$'\033[32m'; BOLD=$'\033[1m'; DIM=$'\033[2m'; RST=$'\033[0m'
STEP=0
step() { STEP=$((STEP+1)); printf '\n%s══ STEP %s: %s%s\n' "$BOLD" "$STEP" "$*" "$RST"; }
info() { printf '   %s%s%s\n' "$DIM" "$*" "$RST"; }
ok()   { printf '   %s✓ %s%s\n' "$GREEN" "$*" "$RST"; }
die()  { printf '%s✗ %s%s\n' "$RED" "$*" "$RST" >&2; exit 1; }

# jget FIELD  — read a (optionally dotted) field from JSON on stdin
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

BODY=""  # last response body (set by try()/call())
CODE=""  # last HTTP status code (set by try()/call())

# XSRF token per access token, filled by login() and read by try().
#
# The gateway mints an XSRF-TOKEN cookie at login, bound to that access token, and
# CSRF (middleware/csrf.go) fires on any mutating request carrying the access_token
# cookie — which is exactly what try() sends. The header must be present, equal to
# the cookie, and a valid binding for that session, so the value has to come from
# the server; it cannot be invented here.
#
# Kept in FILES, not a shell array: every call site invokes login in a command
# substitution (`ITAU_TOK=$(login ...)`), which runs in a subshell, so an array
# assignment inside login is discarded when that subshell exits and the parent
# reads back an empty token. cksum (POSIX) rather than md5sum, which macOS spells
# differently.
XSRF_DIR=$(mktemp -d)
trap 'rm -rf "$XSRF_DIR"' EXIT
xsrf_key() { printf '%s' "$1" | cksum | cut -d' ' -f1; }

# try METHOD URL TOKEN [BODY] — curl with cookie auth; sets BODY + CODE and NEVER
# stops the script (only a curl transport failure aborts). Use when a non-2xx is a
# valid, handled outcome (e.g. probing for already-onboarded state).
try() {
  local method=$1 url=$2 token=$3 body=${4:-}
  local args=(-sS -m 120 -w $'\n%{http_code}' -X "$method" -H 'Content-Type: application/json')
  if [[ -n $token ]]; then
    local xsrf="" xf="$XSRF_DIR/$(xsrf_key "$token")"
    [[ -f $xf ]] && xsrf=$(cat "$xf")
    if [[ -n $xsrf ]]; then
      # One -b: repeated flags do not merge into a single Cookie header.
      args+=(-b "access_token=$token; XSRF-TOKEN=$xsrf" -H "X-XSRF-TOKEN: $xsrf")
    else
      args+=(-b "access_token=$token")
    fi
  fi
  [[ -n $body ]]  && args+=(-d "$body")
  local out
  out=$(curl "${args[@]}" "$url") || die "curl failed: $method $url"
  CODE=${out##*$'\n'}
  BODY=${out%$'\n'*}
  info "$method $url -> HTTP $CODE"
}

# call METHOD URL TOKEN [BODY] — try() + RED-and-stop on any non-2xx.
# The JSON body is passed inline at each call site.
call() {
  try "$@"
  if [[ -z $CODE || $CODE -lt 200 || $CODE -ge 300 ]]; then
    die "HTTP $CODE from $1 $2
    response: $BODY"
  fi
}

# login GATEWAY_URL USERNAME PASSWORD — echoes an accessToken via the gateway's real
# POST /api/v1/auth/login (password grant; clientId/clientSecret = username/password).
login() {
  local out tok jar
  jar=$(mktemp)
  out=$(curl -sS -m 30 -c "$jar" -X POST "$1/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"clientId\":\"$2\",\"clientSecret\":\"$3\"}") \
    || { rm -f "$jar"; die "login curl failed for $2"; }
  tok=$(printf '%s' "$out" | jget accessToken)
  [[ -n $tok ]] || { rm -f "$jar"; die "login failed for $2: $out"; }
  # Netscape cookie jar: domain flag path secure expiry NAME VALUE.
  awk '$6=="XSRF-TOKEN"{print $7}' "$jar" | tail -1 > "$XSRF_DIR/$(xsrf_key "$tok")"
  rm -f "$jar"
  printf '%s' "$tok"
}

# onboard BANK_URL BANK_TOK CB_URL ADM_TOK INSTITUTION COUNTRY EMAIL USERNAME
# Runs initiate (bank) -> approve-kyc (admission operator). Exports ONB_USER_ID.
#
# Idempotent — safe to re-run against a warm stack:
#   * already onboarded (ACTIVE/KYC_APPROVED/APPROVED) -> skip initiate + approve;
#   * initiate returns 409 (Keycloak user already exists, approval never completed)
#     -> recover user_id from my-status and proceed straight to approval.
onboard() {
  local bank_url=$1 bank_tok=$2 cb_url=$3 adm_tok=$4 inst=$5 country=$6 email=$7 user=$8 st

  # Already fully onboarded? Nothing to do.
  try GET "$bank_url/api/v1/onboarding/my-status" "$bank_tok"
  st=$(printf '%s' "$BODY" | jget status)
  case "$st" in
    ACTIVE|KYC_APPROVED|APPROVED)
      ONB_USER_ID=$(printf '%s' "$BODY" | jget user_id)
      ok "$inst already onboarded ($st) — skipping initiate + approve"
      return 0;;
  esac

  # Request onboarding. On a warm stack the initiate may 409 because the Keycloak
  # user already exists from a prior partial run; recover and continue to approval.
  try POST "$bank_url/api/v1/onboarding/initiate" "$bank_tok" \
    "{\"institution_name\":\"$inst\",\"country\":\"$country\",\"role\":\"ROLE_COMMERCIAL_BANK\",\"email\":\"$email\",\"username\":\"$user\"}"
  if [[ $CODE -ge 200 && $CODE -lt 300 ]]; then
    ONB_USER_ID=$(printf '%s' "$BODY" | jget user_id)
    [[ -n $ONB_USER_ID ]] || die "onboarding initiate returned no user_id: $BODY"
    ok "$inst onboarding requested (request_id=$(printf '%s' "$BODY" | jget request_id) user_id=$ONB_USER_ID)"
  elif [[ $CODE -eq 409 ]]; then
    try GET "$bank_url/api/v1/onboarding/my-status" "$bank_tok"
    ONB_USER_ID=$(printf '%s' "$BODY" | jget user_id)
    [[ -n $ONB_USER_ID ]] || die "$inst initiate returned 409 but my-status has no user_id: $BODY"
    ok "$inst onboarding already requested — recovered user_id=$ONB_USER_ID, proceeding to approval"
  else
    die "HTTP $CODE from POST $bank_url/api/v1/onboarding/initiate
    response: $BODY"
  fi

  call POST "$cb_url/api/v1/governance/approve-kyc" "$adm_tok" \
    "{\"subject\":\"$ONB_USER_ID\",\"reason\":\"tryout auto-approval\"}"
  ok "$inst KYC approved (tx_hash=$(printf '%s' "$BODY" | jget tx_hash))"
}

# poll_onboarding LABEL BANK_URL BANK_TOK — 3 tries over 5s; ok on ACTIVE/KYC_APPROVED.
# Uses the bank-side my-status (resolves its own bank_code from the cookie/config).
poll_onboarding() {
  local label=$1 url=$2 tok=$3 st
  for i in 1 2 3; do
    call GET "$url/api/v1/onboarding/my-status" "$tok"
    st=$(printf '%s' "$BODY" | jget status)
    info "[$label] attempt $i: status=$st"
    case "$st" in ACTIVE|KYC_APPROVED|APPROVED) ok "[$label] onboarding OK ($st)"; return 0;; esac
    [[ $i -lt 3 ]] && sleep 5
  done
  die "[$label] onboarding not OK after 3 attempts (last status=$st)"
}

# issue BANK_URL BANK_TOK CB_URL TRE_TOK AMOUNT LABEL — reserve issuance in THREE steps:
#   1) request deposit (bank), 2) approve (treasury — only flips status to APPROVED),
#   3) fiat-exchange (treasury — the step that actually MINTS fCeBM on Besu).
# Step 3 is mandatory: approve alone writes nothing on-chain, so skipping it leaves the
# bank with a zero fCeBM balance and the later tokenization burn reverts.
issue() {
  local bank_url=$1 bank_tok=$2 cb_url=$3 tre_tok=$4 amount=$5 label=$6 id
  call POST "$bank_url/api/v1/payments/deposits" "$bank_tok" "{\"amount\":\"$amount\"}"
  id=$(printf '%s' "$BODY" | jget deposit_id)
  [[ -n $id ]] || die "[$label] no deposit_id: $BODY"
  ok "[$label] deposit requested (deposit_id=$id, amount=$amount)"
  call POST "$cb_url/api/v1/payments/deposits/approve" "$tre_tok" "{\"deposit_id\":\"$id\"}"
  ok "[$label] deposit approved by treasury"
  call POST "$cb_url/api/v1/payments/deposits/fiat-exchange" "$tre_tok" "{\"deposit_id\":\"$id\"}"
  ok "[$label] fiat exchange executed — fCeBM minted (mint_tx=$(printf '%s' "$BODY" | jget mint_tx_hash))"
}

# tokenize BANK_URL BANK_TOK CB_URL TRE_TOK AMOUNT LABEL — escrow (fCeBM->tCeBM) + treasury approve.
tokenize() {
  local bank_url=$1 bank_tok=$2 cb_url=$3 tre_tok=$4 amount=$5 label=$6 id
  call POST "$bank_url/api/v1/payments/escrows" "$bank_tok" "{\"amount\":\"$amount\"}"
  id=$(printf '%s' "$BODY" | jget escrow_id)
  [[ -n $id ]] || die "[$label] no escrow_id: $BODY"
  ok "[$label] tokenization requested (escrow_id=$id, amount=$amount)"
  call POST "$cb_url/api/v1/payments/escrows/approve" "$tre_tok" "{\"escrow_id\":\"$id\"}"
  ok "[$label] tokenization approved (burn=$(printf '%s' "$BODY" | jget burn_tx_hash) mint=$(printf '%s' "$BODY" | jget mint_tx_hash))"
}

printf '%s%s cbweb3 Scenario A — sample tryout %s\n' "$BOLD" "════════" "$RST"

# ═══════════════════════════════ ONBOARDING — BRAZIL ═══════════════════════════
step "Login Itaú + request onboarding"
ITAU_TOK=$(login "$ITAU" "$ITAU_USER" "$ITAU_PASS"); ok "logged in as Itaú"
BR_ADM_TOK=$(login "$BR_CB" "$BR_ADM_USER" "$BR_ADM_PASS")
onboard "$ITAU" "$ITAU_TOK" "$BR_CB" "$BR_ADM_TOK" "Itau" BR admin@itau.brasil.com itau_admin

step "Login Bradesco + request onboarding"
BRADESCO_TOK=$(login "$BRADESCO" "$BRADESCO_USER" "$BRADESCO_PASS"); ok "logged in as Bradesco"
# Refresh the admission token: it was minted before Itaú's onboard and the access
# token lifespan is 300s, so reusing it here can out-live the token on a slow stack
# (approve-kyc would then 401 "invalid token"). Same pattern as the treasury refresh below.
BR_ADM_TOK=$(login "$BR_CB" "$BR_ADM_USER" "$BR_ADM_PASS")
onboard "$BRADESCO" "$BRADESCO_TOK" "$BR_CB" "$BR_ADM_TOK" "Bradesco" BR admin@bradesco.brasil.com bradesco_admin

step "Brazil admission approved both (done inline above); verify onboarding status"
poll_onboarding Itau "$ITAU" "$ITAU_TOK"
poll_onboarding Bradesco "$BRADESCO" "$BRADESCO_TOK"

# ═══════════════════════════════ ONBOARDING — COSTA RICA ═════════════════════════
step "Login cb1 + request onboarding"
CB1_TOK=$(login "$CB1" "$CB1_USER" "$CB1_PASS"); ok "logged in as cb1"
CR_ADM_TOK=$(login "$CR_CB" "$CR_ADM_USER" "$CR_ADM_PASS")
onboard "$CB1" "$CB1_TOK" "$CR_CB" "$CR_ADM_TOK" "cb1" CR admin@cb1.costarica.com cb1_admin

step "Login cb2 + request onboarding"
CB2_TOK=$(login "$CB2" "$CB2_USER" "$CB2_PASS"); ok "logged in as cb2"
# Refresh the admission token (see the Bradesco note): reused across two banks it
# can out-live its 300s lifespan on a slow stack.
CR_ADM_TOK=$(login "$CR_CB" "$CR_ADM_USER" "$CR_ADM_PASS")
onboard "$CB2" "$CB2_TOK" "$CR_CB" "$CR_ADM_TOK" "cb2" CR admin@cb2.costarica.com cb2_admin

step "Costa Rica admission approved both (done inline above); verify onboarding status"
poll_onboarding cb1 "$CB1" "$CB1_TOK"
poll_onboarding cb2 "$CB2" "$CB2_TOK"

# ═══════════════════════════════ ISSUANCE (reserve / fCeBM) ════════════════════
step "Issue 1000000 BRL to Itaú (treasury Brazil approves)"
BR_TRE_TOK=$(login "$BR_CB" "$BR_TRE_USER" "$BR_TRE_PASS")
issue "$ITAU" "$ITAU_TOK" "$BR_CB" "$BR_TRE_TOK" 1000000 "Itau/BRL"

step "Issue 500000000 CRC to cb1 (treasury Costa Rica approves)"
CR_TRE_TOK=$(login "$CR_CB" "$CR_TRE_USER" "$CR_TRE_PASS")
issue "$CB1" "$CB1_TOK" "$CR_CB" "$CR_TRE_TOK" 500000000 "cb1/CRC"

# ═══════════════════════════════ TOKENIZATION (tCeBM / Zeto) ═══════════════════
step "Tokenize 50000 BRL with Itaú (treasury Brazil approves)"
tokenize "$ITAU" "$ITAU_TOK" "$BR_CB" "$BR_TRE_TOK" 50000 "Itau/BRL"

step "Tokenize 250000 CRC with cb1 (treasury Costa Rica approves)"
tokenize "$CB1" "$CB1_TOK" "$CR_CB" "$CR_TRE_TOK" 250000 "cb1/CRC"

# ═══════════════════════════════ BALANCES ══════════════════════════════════════
step "Check balances for both commercial banks"
call GET "$ITAU/api/v1/token/fiat-balance" "$ITAU_TOK";        ok "Itaú fiat (fCeBM) balance: $(printf '%s' "$BODY" | jget balance)"
call GET "$ITAU/api/v1/token/balance" "$ITAU_TOK";             ok "Itaú token (tCeBM) balance: $(printf '%s' "$BODY" | jget balance)"
call GET "$CB1/api/v1/token/fiat-balance" "$CB1_TOK"; ok "cb1 fiat (fCeBM) balance: $(printf '%s' "$BODY" | jget balance)"
call GET "$CB1/api/v1/token/balance" "$CB1_TOK";      ok "cb1 token (tCeBM) balance: $(printf '%s' "$BODY" | jget balance)"

# ═══════════════════════════════ FX AGREEMENT (propose) ════════════════════════
step "Itaú creates cross-spoke FX agreement (BRL 1000 -> CRC 5000)"
EXPIRY=$(( $(date +%s) + 86400 ))
call POST "$ITAU/api/v1/payments/fx/agreements" "$ITAU_TOK" "{
  \"counterparty_b\":\"$ID_CB1\",
  \"settlement_agent\":\"$ID_BR_CB\",
  \"custodian\":\"$ID_CB1\",
  \"beneficiary\":\"$ID_CB2\",
  \"source_spoke_id\":\"spoke-brl\",
  \"dest_spoke_id\":\"spoke-costa-rica\",
  \"source_receiver\":\"$ID_BRADESCO\",
  \"dest_receiver\":\"$ID_CB2\",
  \"origin_amount\":\"1000\",
  \"counter_amount\":\"5000\",
  \"origin_currency\":\"BRL\",
  \"counter_currency\":\"CRC\",
  \"rate\":\"5\",
  \"expiry_date\":$EXPIRY
}"
TRADE_ID=$(printf '%s' "$BODY" | jget trade_id)
[[ -n $TRADE_ID ]] || die "no trade_id returned from propose: $BODY"
ok "agreement created: trade_id=$TRADE_ID tx_hash=$(printf '%s' "$BODY" | jget tx_hash)"

# ═══════════════════════════════ RELAY (checkpoint) ═══════════════════════════
# The agreement relays cross-spoke asynchronously (source Pente commit -> Cacti relay
# -> dest-chain -> cb1's FX indexer projection), so a fixed sleep races the
# relay. Poll the destination fetch every 5s for up to 60s instead; a 404 just retries.
step "cb1 fetches the relayed agreement (poll every 5s, up to 60s)"
FX_STATE=""
for ((waited=0; waited<=60; waited+=5)); do
  try GET "$CB1/api/v1/payments/fx/agreements/$TRADE_ID" "$CB1_TOK"
  if [[ $CODE -eq 200 ]]; then
    FX_STATE=$(printf '%s' "$BODY" | jget agreement.state)
    [[ -n $FX_STATE ]] && { ok "cb1 sees agreement $TRADE_ID (state=$FX_STATE) after ${waited}s"; break; }
  fi
  [[ $waited -lt 60 ]] && info "[relay] not visible yet after ${waited}s (HTTP $CODE) — retrying in 5s" && sleep 5
done
[[ -n $FX_STATE ]] || die "cb1 could not see the relayed agreement $TRADE_ID within 60s (last HTTP $CODE): $BODY"

# ═══════════════════════════════ ACCEPTANCE (counterparty B) ══════════════════
# cb1 is counterparty_b, so it accepts the agreement directly (empty body →
# on_behalf=false). The accept response returns only trade_id + tx_hash; the ACCEPTED
# state is projected asynchronously, so confirm it by polling the fetch afterwards.
step "cb1 accepts the FX agreement"
call POST "$CB1/api/v1/payments/fx/agreements/$TRADE_ID/accept" "$CB1_TOK"
ok "cb1 submitted acceptance for $TRADE_ID (tx_hash=$(printf '%s' "$BODY" | jget tx_hash))"

step "Confirm the agreement transitions out of $FX_STATE (poll every 5s, up to 30s)"
ACCEPT_STATE="$FX_STATE"
for ((waited=0; waited<=30; waited+=5)); do
  try GET "$CB1/api/v1/payments/fx/agreements/$TRADE_ID" "$CB1_TOK"
  s=$(printf '%s' "$BODY" | jget agreement.state)
  if [[ -n $s && $s != "$FX_STATE" ]]; then
    ACCEPT_STATE=$s; ok "agreement $TRADE_ID is now state=$ACCEPT_STATE (was $FX_STATE) after ${waited}s"; break
  fi
  [[ $waited -lt 30 ]] && info "[accept] state still '$s' after ${waited}s — retrying in 5s" && sleep 5
done
[[ $ACCEPT_STATE != "$FX_STATE" ]] || info "[accept] state unchanged within 30s (accept tx already submitted; state may still be projecting) — last state=$ACCEPT_STATE"

# ═══════════════════════════════ SETTLEMENT (dual-layer HTLC) ══════════════════
# Acceptance must reach the SOURCE spoke before Itaú will lock: LockHTLC gates on the
# local FX agreement being ACCEPTED (non-strict skips the gate only when the record is
# absent; here it exists on the source, so it must be accepted first).
step "Wait for the acceptance to reach the source (Itaú sees ACCEPTED; poll every 5s, up to 60s)"
SRC_STATE=""
for ((waited=0; waited<=60; waited+=5)); do
  try GET "$ITAU/api/v1/payments/fx/agreements/$TRADE_ID" "$ITAU_TOK"
  SRC_STATE=$(printf '%s' "$BODY" | jget agreement.state)
  [[ $SRC_STATE == *ACCEPTED* ]] && { ok "Itaú sees agreement $TRADE_ID ACCEPTED after ${waited}s"; break; }
  [[ $waited -lt 60 ]] && info "[settle] source state '$SRC_STATE' (HTTP $CODE) after ${waited}s — retrying in 5s" && sleep 5
done
[[ $SRC_STATE == *ACCEPTED* ]] || die "source agreement $TRADE_ID not ACCEPTED within 60s (last state '$SRC_STATE') — Itaú lock would be rejected"

# 1) Source leg — Itaú locks the origin amount (BRL) to the source receiver (Bradesco).
#    This mints the secret + hashLock: tokens are escrowed privately on Zeto with a public
#    HTLC coordination record on the source Besu chain.
step "Itaú locks the source leg (1000 BRL → Bradesco)"
call POST "$ITAU/api/v1/htlc/lock" "$ITAU_TOK" \
  "{\"receiver\":\"$ID_BRADESCO\",\"amount\":\"1000\",\"agreement_id\":\"$TRADE_ID\"}"
CONTRACT_ID=$(printf '%s' "$BODY" | jget contract_id)
HASH_LOCK=$(printf '%s' "$BODY" | jget hash_lock)
SECRET=$(printf '%s' "$BODY" | jget secret)
[[ -n $CONTRACT_ID && -n $HASH_LOCK && -n $SECRET ]] || die "source lock returned no contract_id/hash_lock/secret: $BODY"
ok "source locked (contract_id=$CONTRACT_ID hash_lock=$HASH_LOCK htlc_tx=$(printf '%s' "$BODY" | jget htlc_tx_hash))"

step "Check the source lock status"
call GET "$ITAU/api/v1/htlc/status/$CONTRACT_ID" "$ITAU_TOK"
ok "source HTLC state=$(printf '%s' "$BODY" | jget state) counterparty_locked=$(printf '%s' "$BODY" | jget counterparty_locked)"

# 2) Destination leg — cb1 locks the counter amount (CRC) to the destination
#    receiver (cb2) using the SAME hashLock, so a single secret unlocks both legs.
step "cb1 locks the destination leg with the same hash (5000 CRC → cb2)"
call POST "$CB1/api/v1/htlc/lock-with-hash" "$CB1_TOK" \
  "{\"hash_lock\":\"$HASH_LOCK\",\"receiver\":\"$ID_CB2\",\"amount\":\"5000\",\"agreement_id\":\"$TRADE_ID\"}"
DEST_CONTRACT_ID=$(printf '%s' "$BODY" | jget contract_id)
[[ -n $DEST_CONTRACT_ID ]] || die "destination lock returned no contract_id: $BODY"
ok "destination locked (contract_id=$DEST_CONTRACT_ID htlc_tx=$(printf '%s' "$BODY" | jget htlc_tx_hash))"

# 2b) Wait for the relay to confirm the destination lock back on the SOURCE. In cross-spoke
#     mode SettleHTLC is gated on counterparty_locked (set by the relay's lock-event handler);
#     settling before that returns "counterparty spoke has not yet locked its leg".
step "Wait for the relay to confirm cb1's leg on the source (counterparty_locked; poll every 5s, up to 60s)"
CP_LOCKED=""
for ((waited=0; waited<=60; waited+=5)); do
  try GET "$ITAU/api/v1/htlc/status/$CONTRACT_ID" "$ITAU_TOK"
  CP_LOCKED=$(printf '%s' "$BODY" | jget counterparty_locked)
  [[ $CP_LOCKED == [Tt]rue ]] && { ok "relay confirmed counterparty lock after ${waited}s — safe to settle"; break; }
  [[ $waited -lt 60 ]] && info "[settle] counterparty_locked=$CP_LOCKED after ${waited}s — waiting for relay, retrying in 5s" && sleep 5
done
[[ $CP_LOCKED == [Tt]rue ]] || die "relay did not confirm the counterparty lock within 60s (counterparty_locked=$CP_LOCKED) — settle would be rejected; check relay logs"

# 3) Settle — Itaú reveals the secret on the source leg. The relay observes the reveal and
#    settles the destination leg on spoke-costa-rica, completing the atomic cross-spoke swap.
step "Itaú settles the source leg by revealing the secret (relayed to spoke-costa-rica)"
call POST "$ITAU/api/v1/htlc/settle" "$ITAU_TOK" \
  "{\"contract_id\":\"$CONTRACT_ID\",\"secret\":\"$SECRET\"}"
ok "source settled (htlc_tx=$(printf '%s' "$BODY" | jget htlc_tx_hash) zeto_tx=$(printf '%s' "$BODY" | jget zeto_tx_hash))"

step "Confirm the destination leg settles via the relay (poll every 5s, up to 60s)"
DEST_STATE=""
for ((waited=0; waited<=60; waited+=5)); do
  try GET "$CB1/api/v1/htlc/status/$DEST_CONTRACT_ID" "$CB1_TOK"
  DEST_STATE=$(printf '%s' "$BODY" | jget state)
  [[ $DEST_STATE == *SETTLED* ]] && { ok "destination leg SETTLED after ${waited}s — atomic swap complete"; break; }
  [[ $waited -lt 60 ]] && info "[settle] destination state '$DEST_STATE' after ${waited}s — waiting for relay, retrying in 5s" && sleep 5
done
[[ $DEST_STATE == *SETTLED* ]] || info "[settle] destination not SETTLED within 60s (last state '$DEST_STATE'); the source reveal succeeded — check relay logs if it does not settle"

# ═══════════════════════════════ REDEEM (de-tokenization tCeBM → fCeBM) ═════════
# Redeem is the inverse of tokenization. The commercial bank's gateway first transfers
# its tCeBM (Zeto private token) to the Central Bank, then the CB treasury mints the
# equivalent fCeBM (fiat) back. So a redeem must DECREASE the bank's tCeBM and INCREASE
# its fCeBM by the same amount — it must never mint more of the token being redeemed.
# (Scenario A mints fCeBM on approval, so it does not share the scenario-B redeem bug
# where the redeemed token was re-minted and the balance went UP.)
REDEEM_AMT=5000

step "Snapshot Itaú balances before the redeem"
call GET "$ITAU/api/v1/token/balance" "$ITAU_TOK";      T_BEFORE=$(printf '%s' "$BODY" | jget balance)
call GET "$ITAU/api/v1/token/fiat-balance" "$ITAU_TOK"; F_BEFORE=$(printf '%s' "$BODY" | jget balance)
ok "before redeem — tCeBM=$T_BEFORE fCeBM=$F_BEFORE"

step "Itaú requests a redeem of $REDEEM_AMT tCeBM → fCeBM (proxy transfers the tCeBM to the CB)"
call POST "$ITAU/api/v1/payments/redeems" "$ITAU_TOK" "{\"amount\":\"$REDEEM_AMT\"}"
REDEEM_ID=$(printf '%s' "$BODY" | jget redeem_id)
[[ -n $REDEEM_ID ]] || die "no redeem_id in response: $BODY"
ok "redeem requested (redeem_id=$REDEEM_ID)"

step "Brazil treasury approves the redeem — mints fCeBM back to Itaú"
BR_TRE_TOK=$(login "$BR_CB" "$BR_TRE_USER" "$BR_TRE_PASS")   # refresh: the run may have out-lived the earlier token
call POST "$BR_CB/api/v1/payments/redeems/approve" "$BR_TRE_TOK" "{\"redeem_id\":\"$REDEEM_ID\"}"
ok "redeem approved (fiat_mint_tx=$(printf '%s' "$BODY" | jget fiat_mint_tx_hash))"

step "Verify the redeem reduced tCeBM and increased fCeBM (poll every 5s, up to 30s)"
REDEEM_OK=""
for ((waited=0; waited<=30; waited+=5)); do
  call GET "$ITAU/api/v1/token/balance" "$ITAU_TOK";      T_AFTER=$(printf '%s' "$BODY" | jget balance)
  call GET "$ITAU/api/v1/token/fiat-balance" "$ITAU_TOK"; F_AFTER=$(printf '%s' "$BODY" | jget balance)
  # Fail fast on the scenario-B-class bug: tCeBM must never go UP, fCeBM never DOWN.
  if python3 -c "import sys; sys.exit(0 if int('${T_AFTER:-0}') > int('${T_BEFORE:-0}') or int('${F_AFTER:-0}') < int('${F_BEFORE:-0}') else 1)"; then
    die "redeem moved balances the WRONG way: tCeBM $T_BEFORE→$T_AFTER (must decrease), fCeBM $F_BEFORE→$F_AFTER (must increase)"
  fi
  if python3 -c "import sys; sys.exit(0 if int('${T_AFTER:-0}')==int('${T_BEFORE:-0}')-$REDEEM_AMT and int('${F_AFTER:-0}')==int('${F_BEFORE:-0}')+$REDEEM_AMT else 1)"; then
    REDEEM_OK=1; ok "after redeem — tCeBM=$T_AFTER (−$REDEEM_AMT) fCeBM=$F_AFTER (+$REDEEM_AMT) after ${waited}s"; break
  fi
  [[ $waited -lt 30 ]] && info "[redeem] balances still projecting (tCeBM=$T_AFTER fCeBM=$F_AFTER) after ${waited}s — retrying in 5s" && sleep 5
done
[[ -n $REDEEM_OK ]] || info "[redeem] exact deltas not observed within 30s (tCeBM $T_BEFORE→$T_AFTER, fCeBM $F_BEFORE→$F_AFTER); direction is correct — Zeto projection may still be catching up"

# ═══════════════════════ RECEIVER CREDIT (the settlement report) ═══════════════
# cb2 received the destination leg. Its OWN orchestrator holds no record of that —
# cb1 locked the leg and the amount is private — so the only way cb2 can see the
# movement is the report cb1's orchestrator posts to the central bank.
#
# Nothing asserted this before, and that is why it shipped broken twice over, by
# two independent causes: the reporter was never constructed (it read an env name
# set nowhere in the repository), and the query serving the credits scoped on a
# derived bank id that disagreed with cb2's real code. Either alone makes the
# credit invisible, and no test ever asked cb2 whether it could see money it had
# received.
#
# This step needs cb2 specifically: it is a bank whose id a positional split gets
# wrong ("spoke-costa-rica-cb2" reads as "rica-cb2"), so it fails if either half
# regresses.
step "cb2 sees the incoming PvP leg as a credit on its statement (poll every 5s, up to 60s)"
CREDIT_OK=""
for waited in 0 5 10 15 20 25 30 35 40 45 50 55 60; do
  CB2_TOK=$(login "$CB2" "$CB2_USER" "$CB2_PASS")
  try GET "$CB2/api/v1/statement" "$CB2_TOK"
  CREDITS=$(printf '%s' "$BODY" | python3 -c '
import sys, json
try:
    movs = json.load(sys.stdin).get("movements", [])
except Exception:
    print(0); raise SystemExit
print(sum(1 for m in movs if m.get("direction") == "credit" and m.get("kind") == "pvp_settlement"))' 2>/dev/null || echo 0)
  if [[ "${CREDITS:-0}" -ge 1 ]]; then
    CREDIT_OK=1; ok "cb2 statement shows $CREDITS incoming PvP credit(s) after ${waited}s"; break
  fi
  [[ $waited -lt 60 ]] && info "[credit] not projected yet after ${waited}s — retrying in 5s" && sleep 5
done
[[ -n $CREDIT_OK ]] || die "cb2 received the destination leg but its statement shows NO PvP credit after 60s — either the settlement report never reached the central bank, or the credit query is not scoping on cb2. The symptom is identical for both causes; check the CB's pvp_settled_legs table to tell them apart."

# ── done ────────────────────────────────────────────────────────────────────────
printf '\n%s✓ tryout complete — cross-spoke PvP settled + reserve redeemed%s\n' "$GREEN$BOLD" "$RST"
ok "agreement $TRADE_ID: proposed (spoke-brl) → relayed → accepted → both legs locked → settled (secret revealed)"
ok "redeem $REDEEM_ID: Itaú de-tokenised $REDEEM_AMT tCeBM back to fCeBM (tCeBM ↓, fCeBM ↑)"
