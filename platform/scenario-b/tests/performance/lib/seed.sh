#!/usr/bin/env bash
# seed.sh — ensure the AMM pair has enough depth for the 30-TPS swap run and is ACTIVE.
#
# Pool reserves in Scenario B come from the cooperative commit-reveal flow (two CBs), not a
# single admin call. The canonical idempotent seeders are:
#   - W-BRL-ARS (the sovereign pair `scenario-b.up` registers): tryouts/tryout-sovereign-cb-liquidity.sh
#     — supports AMOUNT_A/AMOUNT_B for depth sizing (defaults are already deep ~1e23/2e23 wei).
#   - BRL-USD (legacy pool): `make scenario-b.tryout-us1`.
# Both skip when the pool already has bilateral liquidity, so re-running is safe. seed.sh checks
# the live pool first and only invokes the seeder when the pool is not ACTIVE, then re-verifies
# and computes a depth headroom estimate.
#
# Depth math (why this matters): an exact-output swap of AMOUNT_OUT against a constant-product
# pool moves price; if cumulative output over the run approaches a reserve, later swaps exceed
# max_amount_in and revert (failing the <1% gate). We compute the run's total target output and
# warn if it is a large fraction of reserve_b.
#
# Functions:
#   seed_pool_status PAIR TOKEN -> echoes "<pool_status> <reserve_a> <reserve_b>"
#   seed_ensure_depth PAIR TOKEN SWAP_TPS DURATION_SECS AMOUNT_OUT
#
# Env:
#   API_GW_URL                 commercial-bank gateway (pool status is unauthenticated, but a
#                              token is accepted)
#   PERF_MAKE_DIR              dir to run `make` from (default scenario-b root)
#   SKIP_SEED                  "1" to skip seeding entirely (dry-run)

: "${API_GW_URL:=http://localhost:3000}"
: "${SKIP_SEED:=0}"

# seed_pool_status PAIR [TOKEN]
seed_pool_status() {
  pair="$1"; token="${2:-}"
  set -- -s --max-time 20 -H 'Accept: application/json'
  [ -n "$token" ] && set -- "$@" -H "Authorization: Bearer $token"
  body="$(curl "$@" "$API_GW_URL/api/v2/amm/pool/$pair/status" 2>/dev/null || true)"
  status="$(printf '%s' "$body" | grep -o '"pool_status":"[^"]*"' | sed 's/.*://;s/"//g')"
  ra="$(printf '%s' "$body" | grep -o '"reserve_a":"[^"]*"' | sed 's/.*://;s/"//g')"
  rb="$(printf '%s' "$body" | grep -o '"reserve_b":"[^"]*"' | sed 's/.*://;s/"//g')"
  printf '%s %s %s' "${status:-UNKNOWN}" "${ra:-0}" "${rb:-0}"
}

# seed_ensure_depth PAIR TOKEN SWAP_TPS DURATION_SECS AMOUNT_OUT
seed_ensure_depth() {
  pair="$1"; token="$2"; tps="$3"; secs="$4"; amount_out="$5"

  st="$(seed_pool_status "$pair" "$token")"
  status="${st%% *}"; rest="${st#* }"; ra="${rest%% *}"; rb="${rest##* }"
  log_info "pool status" pair="$pair" status="$status" reserve_a="$ra" reserve_b="$rb"

  if [ "$status" != "ACTIVE" ]; then
    if [ "$SKIP_SEED" = "1" ]; then
      log_warn "pool not ACTIVE and SKIP_SEED=1 — leaving as-is" pair="$pair"
    else
      makedir="${PERF_MAKE_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
      case "$pair" in
        W-BRL-ARS|W-*)
          log_info "pool not ACTIVE — seeding sovereign pair via cooperative commit-reveal" pair="$pair"
          ( cd "$makedir" && POOL_PAIR="$pair" bash tryouts/tryout-sovereign-cb-liquidity.sh ) \
            || log_warn "sovereign liquidity tryout returned non-zero (pool may still be partially seeded)" pair="$pair"
          ;;
        *)
          require_cmd make
          log_info "pool not ACTIVE — seeding legacy pool (make scenario-b.tryout-us1)" pair="$pair"
          ( cd "$makedir" && make scenario-b.tryout-us1 ) \
            || log_warn "tryout-us1 returned non-zero (pool may still be partially seeded)"
          ;;
      esac
      st="$(seed_pool_status "$pair" "$token")"
      status="${st%% *}"; rest="${st#* }"; ra="${rest%% *}"; rb="${rest##* }"
      log_info "pool status after seed" pair="$pair" status="$status" reserve_a="$ra" reserve_b="$rb"
    fi
  fi

  if [ "$status" != "ACTIVE" ]; then
    if [ "$SKIP_SEED" = "1" ]; then
      log_warn "pool not ACTIVE (SKIP_SEED=1 dry-run) — swaps would fail against a real stack" pair="$pair" status="$status"
      return 0
    fi
    log_fatal "pool is not ACTIVE after seeding — swaps cannot run" pair="$pair" status="$status"
  fi

  # Depth headroom: total target output over the run vs reserve_b (the output side).
  # Uses integer arithmetic on the human reserve (reserves are wei strings — too big for
  # shell ints, so compare digit-length as a coarse but safe proxy when needed).
  total_out=$(( tps * secs * amount_out ))
  rb_len=${#rb}
  out_len=${#total_out}
  if [ "$rb_len" -le "$out_len" ]; then
    log_warn "shallow pool: run target output may approach/exceed reserve_b — late swaps may revert (price impact). Consider a deeper seed or lower AMOUNT_OUT/DURATION." \
      total_target_output="$total_out" reserve_b_digits="$rb_len"
  else
    log_info "pool depth looks sufficient for the run" total_target_output="$total_out" reserve_b="$rb"
  fi
}
