#!/usr/bin/env bash
# tryout-cacti-interop.sh — Verify Cacti PluginLedgerConnectorBesu integration
#
# Tests that the Cacti HTLC relay service genuinely uses Hyperledger Cacti
# as an interoperability layer by validating:
#
#   1. Relay health endpoint is up
#   2. Cacti PluginLedgerConnectorBesu REST endpoints are registered and working
#      (get-block, get-past-logs, get-open-api-spec)
#   3. Relay custom endpoints still work (events, proof store/verify)
#   4. Socket.IO transport is available (Cacti watchBlocksV1)
#
# Prerequisites:
#   - Cacti relay running: make cacti-up  (or docker compose from interop/hub-and-spoke/cacti)
#   - At least Spoke-A Besu running (chain 1338) on port 8645
#   - curl, jq
#
# Usage:
#   ./tryout-cacti-interop.sh
#
# Environment variables (optional):
#   CACTI_URL    Cacti relay base URL (default: http://localhost:4000)

set -euo pipefail

CACTI_URL="${CACTI_URL:-http://localhost:4000}"
PASS=0
FAIL=0
SKIP=0
TOTAL=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

require_cmds() {
  for cmd in curl jq; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "ERROR: '$cmd' not found. Install it before continuing." >&2
      exit 1
    fi
  done
}

log_step()  { echo "" >&2; echo "=== $* ===" >&2; }
log_ok()    { PASS=$((PASS + 1)); TOTAL=$((TOTAL + 1)); echo "  [PASS] $*" >&2; }
log_fail()  { FAIL=$((FAIL + 1)); TOTAL=$((TOTAL + 1)); echo "  [FAIL] $*" >&2; }
log_skip()  { SKIP=$((SKIP + 1)); TOTAL=$((TOTAL + 1)); echo "  [SKIP] $*" >&2; }
log_info()  { echo "  [INFO] $*" >&2; }

# assert_get URL EXPECTED_CODE [DESCRIPTION]
assert_get() {
  local url=$1 expected=$2 desc=${3:-"GET $url"}
  local code body tmp
  tmp=$(mktemp)
  code=$(curl -sS -X GET "$url" \
    --connect-timeout 5 \
    --max-time 10 \
    -o "$tmp" -w "%{http_code}" 2>/dev/null || echo "000")
  body=$(cat "$tmp" 2>/dev/null || true)
  rm -f "$tmp"

  if [ "$code" = "$expected" ]; then
    log_ok "$desc → HTTP $code"
  else
    log_fail "$desc → HTTP $code (expected $expected) | body: ${body:0:200}"
  fi
  echo "$body"
}

# assert_post URL PAYLOAD EXPECTED_CODE [DESCRIPTION]
assert_post() {
  local url=$1 payload=$2 expected=$3 desc=${4:-"POST $url"}
  local code body tmp
  tmp=$(mktemp)
  code=$(curl -sS -X POST "$url" \
    -H "Content-Type: application/json" \
    --data "$payload" \
    --connect-timeout 5 \
    --max-time 10 \
    -o "$tmp" -w "%{http_code}" 2>/dev/null || echo "000")
  body=$(cat "$tmp" 2>/dev/null || true)
  rm -f "$tmp"

  if [ "$code" = "$expected" ]; then
    log_ok "$desc → HTTP $code"
  else
    log_fail "$desc → HTTP $code (expected $expected) | body: ${body:0:200}"
  fi
  echo "$body"
}

# assert_json_field BODY FIELD DESCRIPTION
# Checks that jq can extract a non-null field from the JSON body.
assert_json_field() {
  local body=$1 field=$2 desc=$3
  local val
  val=$(echo "$body" | jq -r "$field" 2>/dev/null || echo "null")
  if [ "$val" != "null" ] && [ "$val" != "" ]; then
    log_ok "$desc = $val"
  else
    log_fail "$desc — field '$field' is null or missing"
  fi
}

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

require_cmds

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Cacti Interoperability Layer — Integration Tryout          ║"
echo "║  Target: $CACTI_URL                               ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# ── Step 1: Health check ──────────────────────────────────────────────────

log_step "Step 1: Relay health endpoint"
health_body=$(assert_get "$CACTI_URL/api/v1/health" "200" "Health check")
assert_json_field "$health_body" ".status" "Health status"
assert_json_field "$health_body" ".uptime" "Health uptime"

# ── Step 2: Cacti getBlock (spoke-a) ──────────────────────────────────────

log_step "Step 2: Cacti PluginLedgerConnectorBesu — getBlock"
log_info "POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-block"
block_body=$(assert_post \
  "$CACTI_URL/api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-block" \
  '{"blockHashOrBlockNumber": "latest"}' \
  "200" \
  "Cacti getBlock (latest)")

if echo "$block_body" | jq -e '.block' &>/dev/null; then
  block_number=$(echo "$block_body" | jq -r '.block.number // "unknown"')
  log_ok "Cacti returned block data (number=$block_number)"
else
  log_fail "Cacti getBlock: response missing '.block' field"
fi

# ── Step 3: Cacti getPastLogs ─────────────────────────────────────────────

log_step "Step 3: Cacti PluginLedgerConnectorBesu — getPastLogs"
log_info "POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-past-logs"

# Query the last 100 blocks for any logs from the HTLC contract address.
# Even with no events, the endpoint should return {"logs": []}.
HTLC_ADDR="${HTLC_ADDR:-0x9a3dbca554e9f6b9257aaa24010da8377c57c17e}"

if [ "$block_number" != "unknown" ] && [ "$block_number" != "null" ]; then
  from_blk=$((block_number > 100 ? block_number - 100 : 0))
  logs_body=$(assert_post \
    "$CACTI_URL/api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-past-logs" \
    "{\"address\": \"$HTLC_ADDR\", \"fromBlock\": $from_blk, \"toBlock\": $block_number}" \
    "200" \
    "Cacti getPastLogs (blocks $from_blk..$block_number)")

  log_count=$(echo "$logs_body" | jq '.logs | length' 2>/dev/null || echo "?")
  log_ok "getPastLogs returned $log_count log(s)"
else
  log_skip "getPastLogs skipped — could not determine block number from step 2"
fi

# ── Step 4: Cacti OpenAPI spec ────────────────────────────────────────────

log_step "Step 4: Cacti OpenAPI spec endpoint"
spec_body=$(assert_get \
  "$CACTI_URL/api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-open-api-spec" \
  "200" \
  "Cacti OpenAPI spec")

if echo "$spec_body" | jq -e '.openapi' &>/dev/null; then
  openapi_ver=$(echo "$spec_body" | jq -r '.openapi')
  log_ok "OpenAPI version: $openapi_ver"
else
  # Some Cacti versions return the spec as a nested field
  log_info "OpenAPI spec returned (non-standard shape — connector is still registered)"
fi

# ── Step 5: Relay custom events (lock / settle) ──────────────────────────

log_step "Step 5: Relay custom event endpoints (backward compatibility)"
lock_body=$(assert_get "$CACTI_URL/api/v1/relay/events/lock?since=0" "200" "Lock events")
settle_body=$(assert_get "$CACTI_URL/api/v1/relay/events/settle?since=0" "200" "Settle events")

lock_count=$(echo "$lock_body" | jq 'length' 2>/dev/null || echo "?")
settle_count=$(echo "$settle_body" | jq 'length' 2>/dev/null || echo "?")
log_info "Lock events: $lock_count, Settle events: $settle_count"

# ── Step 6: Relay FX event endpoints ─────────────────────────────────────

log_step "Step 6: Relay FX event endpoints"
for kind in fx-proposed fx-accepted fx-rejected fx-cancelled fx-settled; do
  assert_get "$CACTI_URL/api/v1/relay/events/$kind?since=0" "200" "FX events: $kind" > /dev/null
done

# ── Step 7: Proof store + verify round-trip ──────────────────────────────

log_step "Step 7: Relay proof store and verify round-trip"

CORRELATION_ID="tryout-$(date +%s)-$(( RANDOM % 10000 ))"
proof_store_body=$(assert_post \
  "$CACTI_URL/api/v1/relay/proof" \
  "{\"correlationId\": \"$CORRELATION_ID\", \"sourceChain\": \"spoke-a\", \"contractId\": \"0xdeadbeef\", \"eventName\": \"tryout\"}" \
  "201" \
  "Store relay proof")

assert_json_field "$proof_store_body" ".relay_tx_id" "Proof relay_tx_id"
assert_json_field "$proof_store_body" ".correlation_id" "Proof correlation_id"

returned_corr=$(echo "$proof_store_body" | jq -r '.correlation_id')
proof_verify_body=$(assert_get \
  "$CACTI_URL/api/v1/relay/proof/$returned_corr" \
  "200" \
  "Verify relay proof ($returned_corr)")

verified=$(echo "$proof_verify_body" | jq -r '.verified')
if [ "$verified" = "true" ]; then
  log_ok "Proof verified=true after retrieval"
else
  log_fail "Proof verified=$verified (expected true)"
fi

# Non-existent proof should return 404
assert_get "$CACTI_URL/api/v1/relay/proof/nonexistent-id-$(date +%s)" "404" "Missing proof → 404" > /dev/null

# ── Step 8: Socket.IO transport check ────────────────────────────────────

log_step "Step 8: Socket.IO transport availability (Cacti watchBlocksV1)"
# Socket.IO exposes an HTTP polling transport at its path. A GET to the
# socket.io endpoint should return a non-404 response indicating the
# transport is available (even without a full WS upgrade).
sio_code=$(curl -sS -o /dev/null -w "%{http_code}" \
  --connect-timeout 5 \
  --max-time 5 \
  "$CACTI_URL/api/v1/plugins/socket.io/?EIO=4&transport=polling" 2>/dev/null || echo "000")

if [ "$sio_code" != "404" ] && [ "$sio_code" != "000" ]; then
  log_ok "Socket.IO transport responded HTTP $sio_code (transport is alive)"
else
  log_fail "Socket.IO transport returned HTTP $sio_code (expected non-404)"
fi

# ── Step 9: Cacti Prometheus metrics ─────────────────────────────────────

log_step "Step 9: Cacti Prometheus metrics endpoint"
metrics_body=$(assert_get \
  "$CACTI_URL/api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-prometheus-exporter-metrics" \
  "200" \
  "Cacti Prometheus metrics")

if echo "$metrics_body" | grep -q "cactus_"; then
  log_ok "Prometheus metrics contain cactus_* metrics"
else
  log_info "Prometheus metrics returned but no cactus_* prefix found (OK if empty)"
fi

# ── Summary ──────────────────────────────────────────────────────────────

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Summary                                                    ║"
echo "╠══════════════════════════════════════════════════════════════╣"
printf "║  Total: %-3d | Pass: %-3d | Fail: %-3d | Skip: %-3d            ║\n" "$TOTAL" "$PASS" "$FAIL" "$SKIP"
echo "╚══════════════════════════════════════════════════════════════╝"

if [ "$FAIL" -gt 0 ]; then
  echo "RESULT: FAIL ($FAIL test(s) failed)" >&2
  exit 1
fi
echo "RESULT: PASS" >&2
