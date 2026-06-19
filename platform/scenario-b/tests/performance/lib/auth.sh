#!/usr/bin/env bash
# auth.sh — mint Keycloak-backed JWTs for the perf harness WITHOUT touching Keycloak directly.
#
# Mirrors tests/integration exactly: the API gateway exposes
#     POST /api/v1/auth/login  {"clientId","clientSecret"}  -> {"accessToken":"<jwt>"}
# Direct Keycloak HTTP is blocked outside Docker, so the gateway login is the only path that
# works from the host. Client ids are conventional (<entity>-client); secrets come from
# backend/config/.env.infra.<entity> (KC_CLIENT_SECRET) with documented local fallbacks.
#
# Functions (echo the bearer token to stdout):
#   auth_token GATEWAY_URL CLIENT_ID CLIENT_SECRET
#   auth_commercial_bank_token   -> bank-a (commercial_bank role; swap + transfer)
#   auth_central_bank_token      -> central-bank-a (central_bank role; profile/seed governance)
#
# Env overrides:
#   API_GW_URL                  commercial-bank gateway (default http://localhost:18080)
#   API_GW_CENTRAL_BANK_A_URL   CB-A gateway (default http://localhost:38080)
#   KC_BANK_A_CLIENT / KC_CENTRAL_BANK_A_CLIENT   client ids
#   KC_BANK_A_SECRET / KC_CENTRAL_BANK_A_SECRET   secrets (else read from .env.infra.*)

: "${API_GW_URL:=http://localhost:18080}"
: "${API_GW_CENTRAL_BANK_A_URL:=http://localhost:38080}"

# _perf_root → scenario-b/
_perf_root() { cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd; }

# _env_file_value FILE KEY → value of KEY= line, or empty.
_env_file_value() {
  [ -f "$1" ] || return 0
  awk -F= -v k="$2" '$1==k{sub(/^[^=]*=/,"");print;exit}' "$1"
}

# auth_token GATEWAY_URL CLIENT_ID CLIENT_SECRET
auth_token() {
  gw="$1"; cid="$2"; secret="$3"
  [ -n "$secret" ] || log_fatal "empty client secret" client="$cid"
  body="$(printf '{"clientId":"%s","clientSecret":"%s"}' "$cid" "$secret")"
  deadline=$(( $(date +%s) + 90 ))
  while :; do
    resp="$(curl -s -w '\n%{http_code}' --max-time 30 \
      -H 'Content-Type: application/json' \
      -d "$body" "$gw/api/v1/auth/login" 2>/dev/null || true)"
    code="${resp##*$'\n'}"
    payload="${resp%$'\n'*}"
    if [ "$code" = "200" ]; then
      tok="$(printf '%s' "$payload" | sed -n 's/.*"accessToken" *: *"\([^"]*\)".*/\1/p')"
      [ -n "$tok" ] || log_fatal "login 200 but no accessToken" client="$cid"
      printf '%s' "$tok"
      return 0
    fi
    # 503 = auth/compliance warming up — retry. Anything else is a hard failure.
    if [ "$code" != "503" ] || [ "$(date +%s)" -ge "$deadline" ]; then
      log_fatal "gateway login failed" client="$cid" gw="$gw" code="$code"
    fi
    log_warn "auth not ready (503), retrying" gw="$gw"
    sleep 3
  done
}

auth_commercial_bank_token() {
  root="$(_perf_root)"
  cid="${KC_BANK_A_CLIENT:-bank-a-client}"
  secret="${KC_BANK_A_SECRET:-$(_env_file_value "$root/backend/config/.env.infra.bank-a" KC_CLIENT_SECRET)}"
  secret="${secret:-bank-a-local-secret}"
  log_info "minting commercial_bank token" client="$cid" >&2
  auth_token "$API_GW_URL" "$cid" "$secret"
}

auth_central_bank_token() {
  root="$(_perf_root)"
  cid="${KC_CENTRAL_BANK_A_CLIENT:-central-bank-a-client}"
  secret="${KC_CENTRAL_BANK_A_SECRET:-$(_env_file_value "$root/backend/config/.env.infra.central-bank-a" KC_CLIENT_SECRET)}"
  secret="${secret:-central-bank-a-local-secret}"
  log_info "minting central_bank token" client="$cid" >&2
  auth_token "$API_GW_CENTRAL_BANK_A_URL" "$cid" "$secret"
}
