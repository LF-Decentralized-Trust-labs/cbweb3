#!/usr/bin/env bash
# auth.sh — mint an AUTH_TOKEN (raw access_token JWT) automatically.
#
# Reuses the exact credential source the tryout scripts use:
#   backend/config/.env.infra.<entity>  ->  KC_CLIENT_ID + KC_CLIENT_SECRET
# and the same login endpoint:
#   POST {GW}/api/v1/auth/login {clientId, clientSecret}  -> { "accessToken": ... }
# (api-gateway/internal/http/handlers/auth.go). The k6 scripts then set this JWT
# as the access_token cookie.
#
# Functions:
#   perf_read_kc_client_id   ENV_FILE
#   perf_read_kc_secret      ENV_FILE
#   perf_mint_token          GW_URL ENV_FILE   -> echoes the raw JWT (or returns 1)

# perf_read_kc_secret ENV_FILE
perf_read_kc_secret() {
  local env_file="$1" secret
  [ -f "$env_file" ] || { log_error "env file not found" file="$env_file"; return 1; }
  secret=$(grep -E '^KC_CLIENT_SECRET=' "$env_file" | head -1 | cut -d= -f2-)
  [ -n "$secret" ] || { log_error "KC_CLIENT_SECRET missing" file="$env_file"; return 1; }
  printf '%s' "$secret"
}

# perf_read_kc_client_id ENV_FILE  (falls back to the entity-derived default)
perf_read_kc_client_id() {
  local env_file="$1" cid
  cid=$(grep -E '^KC_CLIENT_ID=' "$env_file" 2>/dev/null | head -1 | cut -d= -f2-)
  printf '%s' "$cid"
}

# perf_mint_token GW_URL ENV_FILE
# Logs in and echoes the raw access_token JWT. Returns 1 on failure.
perf_mint_token() {
  local gw="$1" env_file="$2"
  local client_id client_secret resp token
  client_id=$(perf_read_kc_client_id "$env_file")
  client_secret=$(perf_read_kc_secret "$env_file") || return 1
  [ -n "$client_id" ] || { log_error "KC_CLIENT_ID missing" file="$env_file"; return 1; }

  resp=$(curl -sS -X POST "${gw}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "$(jq -nc --arg id "$client_id" --arg sec "$client_secret" \
          '{clientId:$id, clientSecret:$sec}')" 2>/dev/null) || {
    log_error "login request failed" gw="$gw" client_id="$client_id"; return 1; }

  token=$(printf '%s' "$resp" | jq -r '.accessToken // empty' 2>/dev/null)
  if [ -z "$token" ]; then
    log_error "login returned no accessToken" gw="$gw" client_id="$client_id" \
      response="$(printf '%s' "$resp" | head -c 300)"
    return 1
  fi
  log_info "auth token minted" gw="$gw" client_id="$client_id" token_len="${#token}"
  printf '%s' "$token"
}
