#!/usr/bin/env bash
# fund.sh — seed + fund the sender identity so the throughput runs don't trip the
# <1% error gate by running out of fCeBM.
#
# Reuses the exact funding path from tryouts/tryout-escrow-flow.sh:
#   deposit (bank) -> approve (CB gov) -> fiat-exchange/mint fCeBM (CB gov).
# That mints fCeBM to the commercial bank, which backs both the HTLC lock path
# (Zeto lock is funded from tCeBM, but the escrow request that the Zeto harness
# drives consumes fCeBM) and the escrow throughput run.
#
# A 50 TPS x 10m HTLC run is ~30k locks; we seed a large fCeBM float (default
# PERF_FUND_AMOUNT, big enough that admission is never funding-blocked — the lock
# coordination records do not each burn the full amount, but we keep a generous
# margin). Funding is idempotent-friendly: each call mints an additional float.
#
# Functions:
#   perf_fund_sender BANK_GW CB_GW BANK_TOKEN CB_TOKEN [AMOUNT]
#
# Requires curl + jq. Honours PERF_DRY_RUN.

# _perf_post GW PATH TOKEN PAYLOAD EXPECT -> echoes body, returns 1 on bad code
_perf_post() {
  local gw="$1" path="$2" token="$3" payload="$4" expect="$5"
  local tmp code body
  tmp=$(mktemp)
  code=$(curl -sS -X POST "${gw}/api/v1/${path}" \
    --cookie "access_token=${token}" \
    -H "Content-Type: application/json" \
    --data "$payload" -o "$tmp" -w '%{http_code}' 2>/dev/null)
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "$expect" ]; then
    log_error "POST failed" path="$path" http_code="$code" expected="$expect" \
      body="$(printf '%s' "$body" | head -c 240)"
    return 1
  fi
  printf '%s' "$body"
}

# perf_fund_sender BANK_GW CB_GW BANK_TOKEN CB_TOKEN [AMOUNT]
perf_fund_sender() {
  local bank_gw="$1" cb_gw="$2" bank_token="$3" cb_token="$4"
  local amount="${5:-${PERF_FUND_AMOUNT:-100000000000}}"

  if [ "${PERF_DRY_RUN:-0}" = "1" ]; then
    log_info "dry-run: would fund sender" amount="$amount"
    return 0
  fi

  log_info "funding sender: register deposit" amount="$amount"
  local dep deposit_id
  dep=$(_perf_post "$bank_gw" "payments/deposits" "$bank_token" \
        "$(jq -nc --arg a "$amount" '{amount:$a}')" "201") || return 1
  deposit_id=$(printf '%s' "$dep" | jq -r '.deposit_id // empty')
  [ -n "$deposit_id" ] || { log_error "no deposit_id returned"; return 1; }

  # CB deposit approval + fiat-exchange require ROLE_TREASURY — the governance CB
  # token ($cb_token) cannot perform them (403). Mint a treasury token (the same
  # dedicated client keycloak init provisions and provision.sh uses).
  local cb_treasury
  cb_treasury=$(curl -sS --max-time 30 -X POST "${cb_gw}/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg id "${CB_A_TREASURY_CLIENT:-central-bank-a-treasury-client}" \
                 --arg sec "${CB_A_TREASURY_SECRET:-central-bank-a-treasury-local-secret}" \
                 '{clientId:$id,clientSecret:$sec}')" 2>/dev/null | jq -r '.accessToken // empty')
  if [ -z "$cb_treasury" ]; then
    log_warn "fund: CB treasury login failed — approval will 403; falling back to governance token"
    cb_treasury="$cb_token"
  fi

  log_info "funding sender: approve deposit" deposit_id="$deposit_id"
  _perf_post "$cb_gw" "payments/deposits/approve" "$cb_treasury" \
    "$(jq -nc --arg d "$deposit_id" '{deposit_id:$d}')" "201" >/dev/null || return 1

  log_info "funding sender: fiat-exchange (mint fCeBM)" deposit_id="$deposit_id"
  _perf_post "$cb_gw" "payments/deposits/fiat-exchange" "$cb_treasury" \
    "$(jq -nc --arg d "$deposit_id" '{deposit_id:$d}')" "201" >/dev/null || return 1

  log_info "sender funded" deposit_id="$deposit_id" amount="$amount"
}
