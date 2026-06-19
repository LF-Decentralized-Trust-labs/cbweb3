#!/usr/bin/env bash
# provision-swap.sh — one-time setup so the cross-currency swap benchmark can actually
# execute swaps (instead of reverting). Mirrors the integration happy path
# (tests/integration/happy_path_test.go) provisioning:
#
#   1. Onboard the PAYER bank at its CB        (initiate -> CB approve-kyc -> complete)
#      → registers it as a participant, fixing "PAYER_NOT_FOUND" on bridge-in.
#   2. Onboard the BENEFICIARY bank at its CB  (same chain)
#      → fixes "BENEFICIARY_NOT_FOUND" on the relay bridge-out.
#   3. Fund the payer with tCeBM reserves      (deposit -> CB approve -> escrow -> CB approve)
#      → the mandatory prerequisite before a cross-currency swap can burn→mint.
#
# Every step is IDEMPOTENT: an already-ACTIVE bank or an already-funded payer is skipped,
# so re-runs against a warm stack are cheap. Honors SKIP_STACK (dry-run) and
# PERF_SKIP_PROVISION=1. Source after log.sh and auth.sh.
#
# Env (gateways default to the local entity host ports):
#   API_GW_URL (payer/bank-a 18080), API_GW_BANK_B_URL (beneficiary/bank-b 28080),
#   API_GW_CENTRAL_BANK_A_URL (38080), API_GW_CENTRAL_BANK_B_URL (60080)
#   PAYER_BANK_CODE (bank-a), BENEFICIARY_BANK_ID (bank-b)
#   PROVISION_AMOUNT (1e24 tCeBM minted to the payer — covers any tiny-swap run)

: "${API_GW_URL:=http://localhost:18080}"
: "${API_GW_BANK_B_URL:=http://localhost:28080}"
: "${API_GW_CENTRAL_BANK_A_URL:=http://localhost:38080}"
: "${API_GW_CENTRAL_BANK_B_URL:=http://localhost:60080}"
: "${PROVISION_AMOUNT:=1000000000000000000000000}"

_jpost() { # URL TOKEN JSON
  curl -s --max-time 60 --cookie "access_token=$2" -H 'Content-Type: application/json' \
    -X POST "$1" --data "$3" 2>/dev/null
}
_jget() { # URL TOKEN
  curl -s --max-time 30 --cookie "access_token=$2" -H 'Accept: application/json' "$1" 2>/dev/null
}

# _onboard BANK_GW CB_GW BANK_TOKEN CB_TOKEN BANK_CODE COUNTRY → ensure bank is ACTIVE.
_onboard() {
  bgw="$1"; cbgw="$2"; btok="$3"; ctok="$4"; code="$5"; country="$6"
  ms="$(_jget "$bgw/api/v1/onboarding/my-status?bank_code=$code" "$btok")"
  st="$(printf '%s' "$ms" | jq -r '.status // "NONE"')"
  if [ "$st" = "ACTIVE" ]; then log_info "onboard: already ACTIVE" bank="$code"; return 0; fi
  req="$(printf '%s' "$ms" | jq -r '.request_id // empty')"
  uid="$(printf '%s' "$ms" | jq -r '.user_id // empty')"
  if [ -z "$req" ]; then
    init="$(_jpost "$bgw/api/v1/onboarding/initiate" "$btok" \
      "$(jq -nc --arg c "$code" --arg ct "$country" \
        '{institution_name:("Perf "+$c),bank_code:$c,country:$ct,role:"commercial_bank",email:("admin@"+$c+".test"),username:($c+"-admin")}')")"
    req="$(printf '%s' "$init" | jq -r '.request_id // empty')"
    uid="$(printf '%s' "$init" | jq -r '.user_id // empty')"
  fi
  [ -n "$req" ] || { log_warn "onboard: no request_id" bank="$code"; return 1; }
  _jpost "$cbgw/api/v1/compliance/approve-kyc" "$ctok" \
    "$(jq -nc --arg s "$uid" '{subject:$s,reason:"perf-provision"}')" >/dev/null
  i=0
  while [ "$i" -lt 12 ]; do
    s="$(_jget "$bgw/api/v1/onboarding/status/$req" "$btok" | jq -r '.status // "?"')"
    case "$s" in KYC_APPROVED|ACTIVE) break ;; esac
    i=$((i + 1)); sleep 3
  done
  _jpost "$bgw/api/v1/onboarding/complete" "$btok" \
    "$(jq -nc --arg r "$req" --arg u "$uid" '{request_id:$r,user_id:$u}')" >/dev/null
  final="$(_jget "$bgw/api/v1/onboarding/my-status?bank_code=$code" "$btok" | jq -r '.status // "?"')"
  if [ "$final" = "ACTIVE" ]; then log_info "onboard: ACTIVE" bank="$code"; return 0; fi
  log_warn "onboard: did not reach ACTIVE" bank="$code" status="$final"; return 1
}

# _fund_payer BANK_GW CB_GW BANK_TOKEN CB_TOKEN AMOUNT → ensure payer holds tCeBM.
_fund_payer() {
  bgw="$1"; cbgw="$2"; btok="$3"; ctok="$4"; amt="$5"
  bal="$(_jget "$bgw/api/v1/token/balance" "$btok" | jq -r '.balance // "0"')"
  if [ -n "$bal" ] && [ "$bal" != "0" ]; then log_info "fund: payer already holds tCeBM" balance="$bal"; return 0; fi
  dep="$(_jpost "$bgw/api/v1/payments/deposits" "$btok" "$(jq -nc --arg a "$amt" '{amount:$a}')")"
  did="$(printf '%s' "$dep" | jq -r '.deposit_id // empty')"
  [ -n "$did" ] || { log_warn "fund: no deposit_id"; return 1; }
  _jpost "$cbgw/api/v1/payments/deposits/approve" "$ctok" "$(jq -nc --arg d "$did" '{deposit_id:$d}')" >/dev/null
  sleep 4
  esc="$(_jpost "$bgw/api/v1/payments/escrows" "$btok" "$(jq -nc --arg d "$did" --arg a "$amt" '{deposit_id:$d,amount:$a}')")"
  eid="$(printf '%s' "$esc" | jq -r '.escrow_id // empty')"
  [ -n "$eid" ] || { log_warn "fund: no escrow_id"; return 1; }
  _jpost "$cbgw/api/v1/payments/escrows/approve" "$ctok" "$(jq -nc --arg e "$eid" '{escrow_id:$e}')" >/dev/null
  i=0
  while [ "$i" -lt 12 ]; do
    bal="$(_jget "$bgw/api/v1/token/balance" "$btok" | jq -r '.balance // "0"')"
    [ -n "$bal" ] && [ "$bal" != "0" ] && { log_info "fund: payer tCeBM funded" balance="$bal"; return 0; }
    i=$((i + 1)); sleep 3
  done
  log_warn "fund: payer tCeBM still zero after escrow"; return 1
}

# provision_swap_actors — idempotent one-time setup. Non-fatal: logs warnings and lets the
# swap benchmark surface the consequence, so provisioning hiccups don't abort the whole suite.
provision_swap_actors() {
  if [ "${SKIP_STACK:-0}" = "1" ] || [ "${PERF_SKIP_PROVISION:-0}" = "1" ]; then
    log_info "provision: skipped (dry-run or PERF_SKIP_PROVISION=1)"; return 0
  fi
  require_cmd jq; require_cmd curl
  root="$(_perf_root)"
  payer="${PAYER_BANK_CODE:-bank-a}"
  benef="${BENEFICIARY_BANK_ID:-bank-b}"

  ba="$(auth_commercial_bank_token)"
  cba="$(auth_central_bank_token)"
  bb_secret="${KC_BANK_B_SECRET:-$(_env_file_value "$root/backend/config/.env.infra.bank-b" KC_CLIENT_SECRET)}"
  bb_secret="${bb_secret:-bank-b-local-secret}"
  cbb_secret="${KC_CENTRAL_BANK_B_SECRET:-$(_env_file_value "$root/backend/config/.env.infra.central-bank-b" KC_CLIENT_SECRET)}"
  cbb_secret="${cbb_secret:-central-bank-b-local-secret}"
  bb="$(auth_token "$API_GW_BANK_B_URL" "${KC_BANK_B_CLIENT:-bank-b-client}" "$bb_secret")"
  cbb="$(auth_token "$API_GW_CENTRAL_BANK_B_URL" "${KC_CENTRAL_BANK_B_CLIENT:-central-bank-b-client}" "$cbb_secret")"

  log_info "provision: onboarding payer + beneficiary" payer="$payer" beneficiary="$benef"
  _onboard "$API_GW_URL" "$API_GW_CENTRAL_BANK_A_URL" "$ba" "$cba" "$payer" "BR" || log_warn "payer onboarding incomplete"
  _onboard "$API_GW_BANK_B_URL" "$API_GW_CENTRAL_BANK_B_URL" "$bb" "$cbb" "$benef" "AR" || log_warn "beneficiary onboarding incomplete"
  log_info "provision: funding payer with tCeBM" amount="$PROVISION_AMOUNT"
  _fund_payer "$API_GW_URL" "$API_GW_CENTRAL_BANK_A_URL" "$ba" "$cba" "$PROVISION_AMOUNT" || log_warn "payer funding incomplete"

  # Approve the AMM to spend the payer's tokens (both sides) so the hub-only exact-output
  # swap (SWAP_MODE=amm / measurement 1) does not revert on allowance. Idempotent.
  for side in A B; do
    _jpost "$API_GW_URL/api/v2/amm/token/approve-amm" "$ba" \
      "$(jq -nc --arg a "$PROVISION_AMOUNT" --arg s "$side" '{amount:$a,side:$s}')" >/dev/null \
      && log_info "provision: approve-amm ok" side="$side" || log_warn "provision: approve-amm failed" side="$side"
  done
  log_info "provision: done"
}
