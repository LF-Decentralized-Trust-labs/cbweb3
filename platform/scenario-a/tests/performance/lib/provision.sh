#!/usr/bin/env bash
# provision.sh — fund the originator (bank-a) and custodian (bank-d) operators with
# tCeBM so the FX-settlement happy-path benchmark (k6/fx-settlement-throughput.js)
# can lock real value instead of reverting. Scenario A analogue of Scenario B's
# lib/provision-swap.sh.
#
# Mirrors the integration test's Phase 3 (tests/integration/livehappy_path_test.go):
# the central banks mint via their dedicated TREASURY client (ROLE_TREASURY) — the
# plain central-bank client is ROLE_GOVERNANCE and would 403 on /token/mint.
#   - cb-a treasury → mints tCeBM to the originator  (funded_operator@spoke-a-bank-a)
#   - cb-b treasury → mints tCeBM to the custodian   (funded_operator@spoke-b-bank-d)
#
# Non-fatal and honored flags (PERF_DRY_RUN / PERF_SKIP_PROVISION) match the rest of
# the suite. Source after lib/log.sh.
#
# Env (all defaulted):
#   API_GW_CENTRAL_BANK_A_URL (38080), API_GW_CENTRAL_BANK_B_URL (60080)
#   ORIGINATOR_IDENTITY (funded_operator@spoke-a-bank-a)
#   CUSTODIAN_IDENTITY  (funded_operator@spoke-b-bank-d)
#   PROVISION_AMOUNT    (tCeBM minted to each operator)
#   CB_A_TREASURY_CLIENT/SECRET, CB_B_TREASURY_CLIENT/SECRET (keycloak init defaults)

: "${API_GW_CENTRAL_BANK_A_URL:?API_GW_CENTRAL_BANK_A_URL is required — derive it with tests/integration/toolkit-env.sh (the perf make targets do this for you)}"
: "${API_GW_CENTRAL_BANK_B_URL:?API_GW_CENTRAL_BANK_B_URL is required — derive it with tests/integration/toolkit-env.sh (the perf make targets do this for you)}"
: "${ORIGINATOR_IDENTITY:=funded_operator@spoke-a-bank-a}"
: "${CUSTODIAN_IDENTITY:=funded_operator@spoke-b-bank-d}"
: "${PROVISION_AMOUNT:=5000000000000}"
: "${CB_A_TREASURY_CLIENT:=central-bank-a-treasury-client}"
: "${CB_A_TREASURY_SECRET:=central-bank-a-treasury-local-secret}"
: "${CB_B_TREASURY_CLIENT:=central-bank-b-treasury-client}"
: "${CB_B_TREASURY_SECRET:=central-bank-b-treasury-local-secret}"

_prov_login() { # GW CLIENT SECRET -> access_token (stdout)
  curl -sS --max-time 30 -X POST "$1/api/v1/auth/login" -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg id "$2" --arg sec "$3" '{clientId:$id,clientSecret:$sec}')" 2>/dev/null \
    | jq -r '.accessToken // empty' 2>/dev/null
}

_prov_mint() { # GW TOKEN TO AMOUNT -> http_code (stdout)
  curl -s -o /dev/null -w '%{http_code}' --max-time 60 --cookie "access_token=$2" \
    -H 'Content-Type: application/json' -X POST "$1/api/v1/token/mint" \
    --data "$(jq -nc --arg to "$3" --arg a "$4" '{to:$to,amount:$a}')" 2>/dev/null
}

# provision_settlement_actors — mint tCeBM to originator + custodian. Non-fatal: warns
# and lets the benchmark surface any consequence (matches scenario-b semantics).
provision_settlement_actors() {
  if [ "${PERF_DRY_RUN:-0}" = "1" ] || [ "${PERF_SKIP_PROVISION:-0}" = "1" ]; then
    log_info "provision: skipped (dry-run or PERF_SKIP_PROVISION=1)"; return 0
  fi
  command -v jq >/dev/null 2>&1 || { log_warn "provision: jq missing — skipping"; return 1; }

  log_info "provision: minting tCeBM to originator + custodian" \
    originator="$ORIGINATOR_IDENTITY" custodian="$CUSTODIAN_IDENTITY" amount="$PROVISION_AMOUNT"
  local ta tb c1 c2
  ta="$(_prov_login "$API_GW_CENTRAL_BANK_A_URL" "$CB_A_TREASURY_CLIENT" "$CB_A_TREASURY_SECRET")"
  tb="$(_prov_login "$API_GW_CENTRAL_BANK_B_URL" "$CB_B_TREASURY_CLIENT" "$CB_B_TREASURY_SECRET")"
  [ -n "$ta" ] || { log_warn "provision: cb-a treasury login failed (mint needs ROLE_TREASURY)"; return 1; }
  [ -n "$tb" ] || { log_warn "provision: cb-b treasury login failed"; return 1; }
  c1="$(_prov_mint "$API_GW_CENTRAL_BANK_A_URL" "$ta" "$ORIGINATOR_IDENTITY" "$PROVISION_AMOUNT")"
  c2="$(_prov_mint "$API_GW_CENTRAL_BANK_B_URL" "$tb" "$CUSTODIAN_IDENTITY" "$PROVISION_AMOUNT")"
  log_info "provision: mint complete" originator_http="$c1" custodian_http="$c2"
  case "$c1" in 200 | 201) ;; *) log_warn "provision: originator mint non-2xx" http="$c1" ;; esac
  case "$c2" in 200 | 201) ;; *) log_warn "provision: custodian mint non-2xx" http="$c2" ;; esac
}
