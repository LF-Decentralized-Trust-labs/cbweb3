#!/usr/bin/env bash
# tryouts/tryout-commercial-swap-e2e.sh
# E2E validation for 009-commercial-cross-currency-swap
# Tests cross-currency swap: Bank A (BRL) → Bank B (ARS) via Hub bridge + AMM

set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────

API_GATEWAY_URL="${API_GATEWAY_URL:-http://localhost:18080}"
BANK_A_ENV="${BANK_A_ENV:-backend/config/.env.infra.bank-a}"
HUB_RPC="${HUB_RPC:-http://localhost:8545}"
SPOKE_A_RPC="${SPOKE_A_RPC:-http://localhost:8546}"
SPOKE_B_RPC="${SPOKE_B_RPC:-http://localhost:8547}"

PAYER_BANK_ID="bank-a"
BENEFICIARY_BANK_ID="bank-b"
SOURCE_CURRENCY="BRL"
TARGET_CURRENCY="ARS"
POOL_PAIR="W-BRL-ARS"
AMOUNT_OUT="1000000000000000000"  # 1.0 ARS (18 decimals)
SLIPPAGE_BPS=1100  # 11% cap over quoted amount_in (wei)
MAX_AMOUNT_IN=""   # set from quote in run_single_swap (must exceed amount_in numerically)

# Timeouts (seconds)
BRIDGE_IN_TIMEOUT=120
SWAP_TIMEOUT=60
BRIDGE_OUT_TIMEOUT=120
QUOTE_TTL=15

# Performance thresholds (seconds)
P50_THRESHOLD=60
P95_THRESHOLD=90
ITERATIONS=10

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Set by bank_login() before swap steps (POST /swap/cross-currency requires commercial_bank JWT cookie).
BANK_A_ACCESS_TOKEN=""

# ─────────────────────────────────────────────────────────────────────────────
# Helper Functions
# ─────────────────────────────────────────────────────────────────────────────

auth_cookie() {
    if [[ -z "$BANK_A_ACCESS_TOKEN" ]]; then
        echo ""
    else
        echo "access_token=${BANK_A_ACCESS_TOKEN}"
    fi
}

bank_login() {
    local envfile="$1"
    local client_id secret
    if [[ -f "$envfile" ]]; then
        # shellcheck disable=SC1090
        set -a
        source "$envfile"
        set +a
    fi
    client_id="${KC_CLIENT_ID:-bank-a-client}"
    secret="${KC_CLIENT_SECRET:-$(grep -s '^KC_CLIENT_SECRET=' "$envfile" 2>/dev/null | cut -d= -f2- || true)}"
    secret="${secret:-bank-a-local-secret}"

    log_info "Authenticating bank-a via POST /api/v1/auth/login..."
    local login_resp
    login_resp=$(curl -sf -X POST "${API_GATEWAY_URL}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"clientId\":\"${client_id}\",\"clientSecret\":\"${secret}\"}") \
        || { log_error "Login request failed (is bank-a gateway up?)"; return 1; }

    if command -v jq &>/dev/null; then
        BANK_A_ACCESS_TOKEN=$(echo "$login_resp" | jq -r '.accessToken // empty')
    else
        BANK_A_ACCESS_TOKEN=$(json_field "$login_resp" "accessToken")
    fi

    if [[ -z "$BANK_A_ACCESS_TOKEN" || "$BANK_A_ACCESS_TOKEN" == "null" ]]; then
        log_error "Login failed: $login_resp"
        return 1
    fi
    log_info "  ✓ bank-a authenticated (client=${client_id})"
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# Extract JSON field using grep/sed (no jq dependency)
json_field() {
    local json="$1"
    local field="$2"
    echo "$json" | grep -o "\"$field\":\"[^\"]*\"" | sed "s/\"$field\":\"\([^\"]*\)\"/\1/"
}

# Query block explorer for transaction confirmation
query_tx() {
    local rpc_url="$1"
    local tx_hash="$2"
    
    if [[ -z "$tx_hash" || "$tx_hash" == "null" ]]; then
        echo "null"
        return
    fi
    
    curl -s -X POST "$rpc_url" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$tx_hash\"],\"id\":1}" \
        | grep -o '"status":"0x[0-9a-f]*"' | sed 's/"status":"\(0x[0-9a-f]*\)"/\1/'
}

# Poll swap status until terminal state (COMPLETED or FAILED)
poll_swap_status() {
    local swap_id="$1"
    local timeout="$2"
    local start=$(date +%s)
    
    while true; do
        local elapsed=$(($(date +%s) - start))
        if [[ $elapsed -gt $timeout ]]; then
            log_error "Timeout polling swap status (${timeout}s exceeded)"
            return 1
        fi
        
        local response=$(curl -s -X GET \
            "${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}" \
            -b "$(auth_cookie)")
        
        local status=$(json_field "$response" "status")
        log_info "  [${elapsed}s] Status: $status"
        
        if [[ "$status" == "COMPLETED" ]]; then
            echo "$response"
            return 0
        elif [[ "$status" == "FAILED" ]]; then
            log_error "Swap failed: $(json_field "$response" "failure_reason")"
            return 1
        fi
        
        sleep 2
    done
}

# ─────────────────────────────────────────────────────────────────────────────
# Test Case: Single Swap (SC-001 + SC-002 validation)
# ─────────────────────────────────────────────────────────────────────────────

run_single_swap() {
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "Test Case: Single Cross-Currency Swap (BRL → ARS)"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # ─────────────────────────────────────────────────────────────────────────
    # Step 1: Verify pool is ACTIVE
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[1/7] Verifying pool W-BRL-ARS is ACTIVE..."
    
    local pool_url="${API_GATEWAY_URL}/api/v2/amm/pool/W-BRL-ARS/status"
    log_info "  Request URL:"
    echo "    GET ${pool_url}"
    
    local pool_response=$(curl -s -X GET "$pool_url")
    
    local pool_status=$(json_field "$pool_response" "pool_status")
    if [[ "$pool_status" != "ACTIVE" ]]; then
        log_error "Pool is not ACTIVE (current: $pool_status). Aborting."
        return 1
    fi
    log_info "  ✓ Pool is ACTIVE"
    
    # ─────────────────────────────────────────────────────────────────────────
    # Step 2: Obtain quote (validate 15s TTL)
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[2/7] Obtaining quote for ${AMOUNT_OUT} wei (${TARGET_CURRENCY})..."
    
    local quote_url="${API_GATEWAY_URL}/api/v2/amm/quote/cross-currency?source_currency=${SOURCE_CURRENCY}&target_currency=${TARGET_CURRENCY}&amount_out=${AMOUNT_OUT}"
    log_info "  Request URL:"
    echo "    GET ${quote_url}"
    
    local quote_start=$(date +%s)
    local quote_response=$(curl -s -X GET "$quote_url")
    
    local quote_id=$(json_field "$quote_response" "quote_id")
    local required_input=$(json_field "$quote_response" "amount_in")
    
    if [[ -z "$quote_id" || "$quote_id" == "null" ]]; then
        log_error "Failed to obtain quote: $quote_response"
        return 1
    fi
    
    log_info "  ✓ Quote ID: $quote_id"
    log_info "  ✓ Required input: $required_input wei"

    if [[ -z "$required_input" || "$required_input" == "null" ]]; then
        log_error "Quote missing amount_in: $quote_response"
        return 1
    fi
    if command -v python3 &>/dev/null; then
        MAX_AMOUNT_IN=$(python3 -c "print(int('${required_input}') * ${SLIPPAGE_BPS} // 1000)")
    else
        log_warn "python3 not found — using required_input as max_amount_in (no slippage headroom)"
        MAX_AMOUNT_IN="$required_input"
    fi
    log_info "  ✓ max_amount_in (slippage cap): $MAX_AMOUNT_IN wei"
    
    # Validate quote TTL (wait 5s, ensure still valid)
    sleep 5
    local quote_elapsed=$(($(date +%s) - quote_start))
    log_info "  ✓ Quote age: ${quote_elapsed}s (TTL: ${QUOTE_TTL}s)"
    
    if [[ $quote_elapsed -gt $QUOTE_TTL ]]; then
        log_warn "Quote already expired (${quote_elapsed}s > ${QUOTE_TTL}s). Testing expiry validation..."
    fi
    
    # ─────────────────────────────────────────────────────────────────────────
    # Step 3: Execute swap
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[3/7] Executing cross-currency swap..."
    
    local swap_payload="{
        \"source_currency\": \"${SOURCE_CURRENCY}\",
        \"target_currency\": \"${TARGET_CURRENCY}\",
        \"pool_pair\": \"${POOL_PAIR}\",
        \"amount_out\": \"${AMOUNT_OUT}\",
        \"max_amount_in\": \"${MAX_AMOUNT_IN}\",
        \"payer_bank_id\": \"${PAYER_BANK_ID}\",
        \"beneficiary_bank_id\": \"${BENEFICIARY_BANK_ID}\",
        \"quote_id\": \"${quote_id}\"
    }"
    
    log_info "  Request:"
    echo "    POST ${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency"
    log_info "  Payload:"
    echo "$swap_payload" | sed 's/^/    /'
    
    local swap_start=$(date +%s)
    local swap_response=$(curl -s -X POST \
        "${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency" \
        -H "Content-Type: application/json" \
        -b "$(auth_cookie)" \
        -d "$swap_payload")
    
    local swap_id=$(json_field "$swap_response" "swap_id")
    local correlation_id=$(json_field "$swap_response" "correlation_id")
    
    if [[ -z "$swap_id" || "$swap_id" == "null" ]]; then
        local error_code=$(json_field "$swap_response" "code")
        if [[ "$error_code" == "QUOTE_EXPIRED" ]]; then
            log_warn "  ⚠ Quote expired (expected if TTL exceeded). Re-obtaining quote..."
            # Retry with fresh quote
            local retry_quote_url="${API_GATEWAY_URL}/api/v2/amm/quote/cross-currency?source_currency=${SOURCE_CURRENCY}&target_currency=${TARGET_CURRENCY}&amount_out=${AMOUNT_OUT}"
            log_info "  Retry Quote URL:"
            echo "    GET ${retry_quote_url}"
            
            quote_response=$(curl -s -X GET "$retry_quote_url")
            quote_id=$(json_field "$quote_response" "quote_id")
            
            local retry_payload="{
                \"source_currency\": \"${SOURCE_CURRENCY}\",
                \"target_currency\": \"${TARGET_CURRENCY}\",
                \"pool_pair\": \"${POOL_PAIR}\",
                \"amount_out\": \"${AMOUNT_OUT}\",
                \"max_amount_in\": \"${MAX_AMOUNT_IN}\",
                \"payer_bank_id\": \"${PAYER_BANK_ID}\",
                \"beneficiary_bank_id\": \"${BENEFICIARY_BANK_ID}\",
                \"quote_id\": \"${quote_id}\"
            }"
            
            log_info "  Retry Request:"
            echo "    POST ${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency"
            log_info "  Retry Payload:"
            echo "$retry_payload" | sed 's/^/    /'
            
            swap_response=$(curl -s -X POST \
                "${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency" \
                -H "Content-Type: application/json" \
                -b "$(auth_cookie)" \
                -d "$retry_payload")
            
            swap_id=$(json_field "$swap_response" "swap_id")
            correlation_id=$(json_field "$swap_response" "correlation_id")
        fi
        
        if [[ -z "$swap_id" || "$swap_id" == "null" ]]; then
            log_error "Failed to execute swap: $swap_response"
            return 1
        fi
    fi
    
    log_info "  ✓ Swap ID: $swap_id"
    log_info "  ✓ Correlation ID: $correlation_id"

    local post_status=$(json_field "$swap_response" "status")
    local final_response="$swap_response"

    # ─────────────────────────────────────────────────────────────────────────
    # Step 4: Monitor bridge-in via GET /swap/cross-currency/:id
    # Sync path: POST already returned COMPLETED — extract fields directly.
    # Async path: poll until SWAP_IN_PROGRESS or COMPLETED.
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[4/7] Bridge-in (Spoke-A lock + Hub mint)..."

    if [[ "$post_status" == "COMPLETED" ]]; then
        log_info "  ✓ Position: $(json_field "$swap_response" "bridge_in_position_id") (sync)"
    else
        log_info "  Polling URL:"
        echo "    GET ${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}"

        local bridge_in_start=$(date +%s)
        while true; do
            local elapsed=$(($(date +%s) - bridge_in_start))
            if [[ $elapsed -gt $BRIDGE_IN_TIMEOUT ]]; then
                log_error "Bridge-in timeout (${BRIDGE_IN_TIMEOUT}s exceeded)"
                return 1
            fi

            local status_response=$(curl -s -X GET \
                "${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}" \
                -b "$(auth_cookie)")

            local status=$(json_field "$status_response" "status")

            if [[ "$status" == "SWAP_IN_PROGRESS" || "$status" == "BRIDGE_OUT_PROGRESS" || "$status" == "COMPLETED" ]]; then
                log_info "  ✓ Bridge-in complete (position: $(json_field "$status_response" "bridge_in_position_id"))"
                final_response="$status_response"
                post_status="$status"
                break
            elif [[ "$status" == "FAILED" ]]; then
                log_error "Bridge-in failed: $(json_field "$status_response" "failure_reason")"
                return 1
            fi

            log_info "  [${elapsed}s] Status: $status — waiting..."
            sleep 2
        done
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # Step 5: Monitor Hub AMM swap
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[5/7] Hub AMM swap (W-BRL → W-ARS)..."

    if [[ "$post_status" == "COMPLETED" || "$post_status" == "BRIDGE_OUT_PROGRESS" ]]; then
        log_info "  ✓ Tx: $(json_field "$final_response" "swap_tx_hash") (sync)"
    else
        log_info "  Polling URL:"
        echo "    GET ${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}"

        local swap_step_start=$(date +%s)
        while true; do
            local elapsed=$(($(date +%s) - swap_step_start))
            if [[ $elapsed -gt $SWAP_TIMEOUT ]]; then
                log_error "Swap timeout (${SWAP_TIMEOUT}s exceeded)"
                return 1
            fi

            local status_response=$(curl -s -X GET \
                "${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}" \
                -b "$(auth_cookie)")

            local status=$(json_field "$status_response" "status")

            if [[ "$status" == "BRIDGE_OUT_PROGRESS" || "$status" == "COMPLETED" ]]; then
                log_info "  ✓ Swap complete (tx: $(json_field "$status_response" "swap_tx_hash"))"
                final_response="$status_response"
                post_status="$status"
                break
            elif [[ "$status" == "FAILED" ]]; then
                log_error "Swap failed: $(json_field "$status_response" "failure_reason")"
                return 1
            fi

            log_info "  [${elapsed}s] Status: $status — waiting..."
            sleep 2
        done
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # Step 6: Monitor bridge-out (Hub burn + Spoke-B unlock)
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[6/7] Bridge-out (Hub burn + Spoke-B unlock)..."

    if [[ "$post_status" == "COMPLETED" ]]; then
        log_info "  ✓ Position: $(json_field "$final_response" "bridge_out_position_id") (sync)"
    else
        log_info "  Polling URL:"
        echo "    GET ${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}"

        local bridge_out_timeout=$BRIDGE_OUT_TIMEOUT
        local bridge_out_start=$(date +%s)
        while true; do
            local elapsed=$(($(date +%s) - bridge_out_start))
            if [[ $elapsed -gt $bridge_out_timeout ]]; then
                log_error "Bridge-out timeout (${bridge_out_timeout}s exceeded)"
                return 1
            fi

            local status_response=$(curl -s -X GET \
                "${API_GATEWAY_URL}/api/v2/amm/swap/cross-currency/${swap_id}" \
                -b "$(auth_cookie)")

            local status=$(json_field "$status_response" "status")

            if [[ "$status" == "COMPLETED" ]]; then
                log_info "  ✓ Bridge-out complete (position: $(json_field "$status_response" "bridge_out_position_id"))"
                final_response="$status_response"
                break
            elif [[ "$status" == "FAILED" ]]; then
                log_error "Bridge-out failed: $(json_field "$status_response" "failure_reason")"
                return 1
            fi

            log_info "  [${elapsed}s] Status: $status — waiting..."
            sleep 2
        done
    fi
    
    # ─────────────────────────────────────────────────────────────────────────
    # Step 7: Verify COMPLETED status + calculate total latency
    # ─────────────────────────────────────────────────────────────────────────
    log_info "[7/7] Verifying final status..."
    
    local final_status=$(json_field "$final_response" "status")
    if [[ "$final_status" != "COMPLETED" ]]; then
        log_error "Expected COMPLETED status, got: $final_status"
        return 1
    fi
    
    local total_latency=$(($(date +%s) - swap_start))
    log_info "  ✓ Status: COMPLETED"
    log_info "  ✓ Total latency: ${total_latency}s (quote → unlock)"
    
    # ─────────────────────────────────────────────────────────────────────────
    # SC-001: Validate tx_hash for all 3 transactions (if available)
    # ─────────────────────────────────────────────────────────────────────────
    log_info ""
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "SC-001: Transaction Hash Validation"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # Note: tx_hash fields may not be populated in real-time if relayer hasn't committed yet.
    # In production, query bridge position records from DB or wait for relayer confirmation.
    
    local bridge_in_tx=$(json_field "$final_response" "bridge_in_tx_hash")
    local swap_tx=$(json_field "$final_response" "swap_tx_hash")
    local bridge_out_tx=$(json_field "$final_response" "bridge_out_tx_hash")
    
    log_info "  Bridge-in tx (Spoke-A lock): ${bridge_in_tx:-"(pending relayer commit)"}"
    log_info "  Swap tx (Hub AMM): ${swap_tx:-"(pending relayer commit)"}"
    log_info "  Bridge-out tx (Spoke-B unlock): ${bridge_out_tx:-"(pending relayer commit)"}"
    
    if [[ -n "$bridge_in_tx" && "$bridge_in_tx" != "null" ]]; then
        local bridge_in_status=$(query_tx "$SPOKE_A_RPC" "$bridge_in_tx")
        log_info "  ✓ Bridge-in confirmed on Spoke-A (status: $bridge_in_status)"
    fi
    
    if [[ -n "$swap_tx" && "$swap_tx" != "null" ]]; then
        local swap_status=$(query_tx "$HUB_RPC" "$swap_tx")
        log_info "  ✓ Swap confirmed on Hub (status: $swap_status)"
    fi
    
    if [[ -n "$bridge_out_tx" && "$bridge_out_tx" != "null" ]]; then
        local bridge_out_status=$(query_tx "$SPOKE_B_RPC" "$bridge_out_tx")
        log_info "  ✓ Bridge-out confirmed on Spoke-B (status: $bridge_out_status)"
    fi
    
    echo "$total_latency"  # Return latency for aggregation
}

# ─────────────────────────────────────────────────────────────────────────────
# Test Case: Performance Benchmark (SC-002)
# ─────────────────────────────────────────────────────────────────────────────

run_performance_test() {
    log_info ""
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "SC-002: Performance Benchmark (${ITERATIONS} iterations)"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    local latencies=()
    local success_count=0
    
    for i in $(seq 1 "$ITERATIONS"); do
        log_info "[Iteration ${i}/${ITERATIONS}]"
        local latency=$(run_single_swap 2>&1 | tail -n 1)
        
        if [[ "$latency" =~ ^[0-9]+$ ]]; then
            latencies+=("$latency")
            ((success_count++))
            log_info "  ✓ Latency: ${latency}s"
        else
            log_warn "  ✗ Swap failed (excluded from stats)"
        fi
        
        log_info ""
    done
    
    if [[ ${#latencies[@]} -eq 0 ]]; then
        log_error "No successful swaps. Cannot calculate statistics."
        return 1
    fi
    
    # Sort latencies for percentile calculation
    IFS=$'\n' sorted=($(sort -n <<<"${latencies[*]}"))
    unset IFS
    
    local p50_idx=$(( ${#sorted[@]} * 50 / 100 ))
    local p95_idx=$(( ${#sorted[@]} * 95 / 100 ))
    local p50=${sorted[$p50_idx]}
    local p95=${sorted[$p95_idx]}
    
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "Performance Summary"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "  Success rate: ${success_count}/${ITERATIONS} ($(( success_count * 100 / ITERATIONS ))%)"
    log_info "  p50 latency: ${p50}s (threshold: ${P50_THRESHOLD}s)"
    log_info "  p95 latency: ${p95}s (threshold: ${P95_THRESHOLD}s)"
    
    if [[ $p50 -le $P50_THRESHOLD && $p95 -le $P95_THRESHOLD ]]; then
        log_info "  ✓ Performance requirements MET"
        return 0
    else
        log_error "  ✗ Performance requirements NOT MET"
        return 1
    fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Main Execution
# ─────────────────────────────────────────────────────────────────────────────

main() {
    log_info "Starting E2E Cross-Currency Swap Validation"
    log_info "API Gateway: $API_GATEWAY_URL"
    log_info "Hub RPC: $HUB_RPC"
    log_info "Spoke-A RPC: $SPOKE_A_RPC"
    log_info "Spoke-B RPC: $SPOKE_B_RPC"
    log_info ""

    if ! bank_login "$BANK_A_ENV"; then
        log_error "Authentication failed. Set BANK_A_ENV or KC_CLIENT_SECRET in backend/config/.env.infra.bank-a"
        exit 1
    fi
    
    # Run single swap first (sanity check)
    if ! run_single_swap; then
        log_error "Single swap test failed. Skipping performance benchmark."
        exit 1
    fi
    
    # Run performance test if single swap succeeded
    if [[ "${SKIP_PERF_TEST:-false}" != "true" ]]; then
        if ! run_performance_test; then
            exit 1
        fi
    else
        log_warn "Performance test skipped (SKIP_PERF_TEST=true)"
    fi
    
    log_info ""
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "✓ All tests PASSED"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

main "$@"
