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
  # Extract each limit id. The TransferLimit JSON exposes "limit_id" (the DELETE route's :id).
  ids="$(printf '%s' "$body" | grep -o '"limit_id":"[^"]*"' | sed 's/"limit_id":"//;s/"//')"
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

# _breaker_state BODY -> echoes the "state" value (LIVE | HALTED | RESUME_PENDING | "").
_breaker_state() {
  printf '%s' "$1" | grep -o '"state":"[^"]*"' | head -1 | sed 's/"state":"//;s/"//'
}

# profile_assert_breaker_resumed CB_GW_URL CB_TOKEN — constitution III precondition.
# The circuit-breaker status reports {"state":"LIVE"|"HALTED"|"RESUME_PENDING"}. Swaps are only
# allowed when state==LIVE; HALTED (paused) and RESUME_PENDING (1-of-N requested, not yet 2-of-N)
# both block swaps. Resume is 2-of-N, so a single CB can request+sign but may not fully clear it.
profile_assert_breaker_resumed() {
  gw="$1"; token="$2"
  out="$(_gov_curl GET "$gw/api/v2/governance/circuit-breaker/status" "$token")"
  code="${out%% *}"; body="${out#* }"
  if [ "$code" != "200" ]; then
    log_warn "circuit-breaker status unavailable — assuming LIVE" gw="$gw" code="$code"
    return 0
  fi
  state="$(_breaker_state "$body")"
  if [ "$state" = "LIVE" ] || [ -z "$state" ]; then
    log_info "circuit breaker LIVE — swaps allowed" gw="$gw" state="${state:-unknown}"
    return 0
  fi
  log_warn "circuit breaker not LIVE — swaps would revert" gw="$gw" state="$state"
  if [ "$PERF_BREAKER_RESUME" != "1" ]; then
    log_fatal "circuit breaker $state and PERF_BREAKER_RESUME!=1 — cannot run swaps" gw="$gw"
  fi
  log_info "attempting resume-request + resume-sign (2-of-N; single-CB best effort)" gw="$gw"
  _gov_curl POST "$gw/api/v2/governance/circuit-breaker/resume-request" "$token" '{}' >/dev/null
  _gov_curl POST "$gw/api/v2/governance/circuit-breaker/resume-sign" "$token" '{}' >/dev/null
  out2="$(_gov_curl GET "$gw/api/v2/governance/circuit-breaker/status" "$token")"
  state2="$(_breaker_state "${out2#* }")"
  if [ "$state2" != "LIVE" ]; then
    log_fatal "circuit breaker still $state2 after resume attempt (2-of-N needs a second CB) — cannot run swaps" gw="$gw"
  fi
  log_info "circuit breaker resumed to LIVE — swaps allowed" gw="$gw"
}

# profile_apply CB_GW_URL CB_TOKEN
profile_apply() {
  gw="$1"; token="$2"
  log_info "applying perf profile" gw="$gw"
  profile_clear_limits "$gw" "$token"
  profile_assert_breaker_resumed "$gw" "$token"
  log_info "perf profile applied (limits cleared, breaker active)" gw="$gw"
}
