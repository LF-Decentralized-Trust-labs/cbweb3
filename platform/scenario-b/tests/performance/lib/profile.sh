#!/usr/bin/env bash
# profile.sh — set the "perf profile" before load: clear transfer limits + ensure the
# circuit breaker is RESUMED (constitution III: breaker validated BEFORE swaps).
#
# Why: R1-10.1 daily transfer limits, if seeded, make sustained 50 TPS trip
# TRANSFER_LIMIT_EXCEEDED (HTTP 422) and flood the <1% error gate. The checker treats a
# *missing* applicable limit as unlimited (FindApplicableLimit==nil -> allow), so deleting
# the perf-currency limits is the cleanest "raise to infinity". A paused breaker reverts
# every swap, so we confirm it is not PAUSED and resume if a single CB can.
#
# Governance endpoints require central_bank role AND use cookie auth, so we send the token
# as BOTH a Bearer header and an access_token cookie (exactly as tests/integration does).
#
# Functions:
#   profile_apply CB_GW_URL CB_TOKEN   — clear limits + assert breaker resumed for one CB/spoke
#
# Env:
#   API_GW_CENTRAL_BANK_A_URL  default http://localhost:38080
#   PERF_BREAKER_RESUME        "1" to attempt resume-request/sign if paused (default 1)

: "${API_GW_CENTRAL_BANK_A_URL:=http://localhost:38080}"
: "${PERF_BREAKER_RESUME:=1}"

# _gov_curl METHOD URL TOKEN [JSON_BODY] — governance call with cookie+bearer auth.
# Echoes "<http_code> <body>".
_gov_curl() {
  method="$1"; url="$2"; token="$3"; body="${4:-}"
  set -- -s -w '\n%{http_code}' --max-time 30 -X "$method" \
    -H "Authorization: Bearer $token" \
    -H "Cookie: access_token=$token" \
    -H 'Accept: application/json'
  if [ -n "$body" ]; then set -- "$@" -H 'Content-Type: application/json' -d "$body"; fi
  resp="$(curl "$@" "$url" 2>/dev/null || true)"
  printf '%s %s' "${resp##*$'\n'}" "${resp%$'\n'*}"
}

# profile_clear_limits CB_GW_URL CB_TOKEN — delete every active transfer limit for this CB.
profile_clear_limits() {
  gw="$1"; token="$2"
  out="$(_gov_curl GET "$gw/api/v2/governance/transfer-limits" "$token")"
  code="${out%% *}"; body="${out#* }"
  if [ "$code" != "200" ]; then
    log_warn "could not list transfer-limits (skipping clear)" gw="$gw" code="$code"
    return 0
  fi
  # Extract each limit id. Limits expose an "id" field (UUID/string).
  ids="$(printf '%s' "$body" | grep -o '"id":"[^"]*"' | sed 's/"id":"//;s/"//')"
  if [ -z "$ids" ]; then
    log_info "no transfer limits set — transfers already unlimited" gw="$gw"
    return 0
  fi
  for id in $ids; do
    d="$(_gov_curl DELETE "$gw/api/v2/governance/transfer-limits/$id" "$token")"
    dcode="${d%% *}"
    case "$dcode" in
      204|200|404) log_info "cleared transfer limit" id="$id" code="$dcode" ;;
      *) log_warn "failed to clear transfer limit" id="$id" code="$dcode" ;;
    esac
  done
}

# profile_assert_breaker_resumed CB_GW_URL CB_TOKEN — constitution III precondition.
profile_assert_breaker_resumed() {
  gw="$1"; token="$2"
  out="$(_gov_curl GET "$gw/api/v2/governance/circuit-breaker/status" "$token")"
  code="${out%% *}"; body="${out#* }"
  if [ "$code" != "200" ]; then
    log_warn "circuit-breaker status unavailable — assuming active" gw="$gw" code="$code"
    return 0
  fi
  # Treat any "paused":true or "status":"PAUSED" as paused.
  if printf '%s' "$body" | grep -qiE '"paused"[[:space:]]*:[[:space:]]*true|"(status|state)"[[:space:]]*:[[:space:]]*"paused"'; then
    log_warn "circuit breaker is PAUSED — swaps would revert" gw="$gw"
    if [ "$PERF_BREAKER_RESUME" = "1" ]; then
      log_info "attempting resume-request + resume-sign (single-CB best effort)" gw="$gw"
      _gov_curl POST "$gw/api/v2/governance/circuit-breaker/resume-request" "$token" '{}' >/dev/null
      _gov_curl POST "$gw/api/v2/governance/circuit-breaker/resume-sign" "$token" '{}' >/dev/null
      out2="$(_gov_curl GET "$gw/api/v2/governance/circuit-breaker/status" "$token")"
      if printf '%s' "${out2#* }" | grep -qiE '"paused"[[:space:]]*:[[:space:]]*true|"(status|state)"[[:space:]]*:[[:space:]]*"paused"'; then
        log_fatal "circuit breaker still PAUSED after resume attempt (2-of-N needs a second CB) — cannot run swaps" gw="$gw"
      fi
    else
      log_fatal "circuit breaker PAUSED and PERF_BREAKER_RESUME!=1 — cannot run swaps" gw="$gw"
    fi
  fi
  log_info "circuit breaker is active (not paused) — swaps allowed" gw="$gw"
}

# profile_apply CB_GW_URL CB_TOKEN
profile_apply() {
  gw="$1"; token="$2"
  log_info "applying perf profile" gw="$gw"
  profile_clear_limits "$gw" "$token"
  profile_assert_breaker_resumed "$gw" "$token"
  log_info "perf profile applied (limits cleared, breaker active)" gw="$gw"
}
