#!/usr/bin/env bash
# ttf.sh — Time-To-Finality (threshold 3) post-processor.
#
# Correlates each HTLC lock's contract_id (emitted by k6 with PRINT_IDS=1 as
# "CONTRACT_ID <id> <t0_epoch_ms>") with the on-chain LogHTLCLocked event on the
# Spoke-A HTLC coordination contract, then computes per-spoke TTF p50/p95.
#
#   t0 = client send time (epoch ms) — emitted by the transfer harness.
#   t1 = block timestamp of the LogHTLCLocked event for that contractId.
#   TTF = t1 - t0  (seconds).  Gate: p95 < 5s per spoke.
#
# Event (contracts/src/interfaces/IHashTimeLockedContract.sol):
#   LogHTLCLocked(bytes32 indexed contractId, address indexed sender,
#                 address indexed receiver, bytes32 hashLock,
#                 uint256 timeLock, bytes32 zetoLockRef)
#   topic0 = keccak("LogHTLCLocked(bytes32,address,address,bytes32,uint256,bytes32)")
#          = 0x3a6921b47247ac0d3d33978f338f1aebb28e7f026c59d0348dc5382457b5256c
#   topic1 = contractId (32-byte, left-padded).
#
# Usage:
#   perf_ttf_compute IDS_FILE RPC_URL HTLC_ADDRESS OUT_JSON
#     IDS_FILE      file of "CONTRACT_ID <id> [<t0_ms>]" lines (k6 stdout grep)
#     RPC_URL       Spoke-A bank-a Besu RPC (host default http://localhost:8646)
#     HTLC_ADDRESS  HTLC coordination contract address (from .env.infra.bank-a)
#     OUT_JSON      path to write {p50_s,p95_s,samples,gate_pass} summary

PERF_TTF_TOPIC0="0x3a6921b47247ac0d3d33978f338f1aebb28e7f026c59d0348dc5382457b5256c"

# _perf_rpc RPC METHOD PARAMS_JSON -> echoes .result (raw JSON), 1 on error
_perf_rpc() {
  local rpc="$1" method="$2" params="$3" resp
  resp=$(curl -sS -X POST "$rpc" -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg m "$method" --argjson p "$params" \
          '{jsonrpc:"2.0",id:1,method:$m,params:$p}')" 2>/dev/null) || return 1
  if printf '%s' "$resp" | jq -e '.error' >/dev/null 2>&1; then
    log_warn "rpc error" method="$method" err="$(printf '%s' "$resp" | jq -c '.error')"
    return 1
  fi
  printf '%s' "$resp" | jq -c '.result'
}

# _perf_pad_topic ID -> 0x + 64 hex chars (left-pad the contractId to bytes32)
_perf_pad_topic() {
  local id="${1#0x}"
  printf '0x%064s' "$id" | tr ' ' '0'
}

# _perf_block_ts_seconds RPC BLOCK_HEX -> decimal epoch seconds of that block
_perf_block_ts_seconds() {
  local rpc="$1" block="$2" res ts_hex
  res=$(_perf_rpc "$rpc" "eth_getBlockByNumber" "$(jq -nc --arg b "$block" '[$b,false]')") || return 1
  ts_hex=$(printf '%s' "$res" | jq -r '.timestamp // empty')
  [ -n "$ts_hex" ] || return 1
  printf '%d' "$ts_hex"   # bash understands 0x.. hex in printf %d
}

# perf_ttf_compute IDS_FILE RPC HTLC_ADDR OUT_JSON
perf_ttf_compute() {
  local ids_file="$1" rpc="$2" htlc="$3" out="$4"
  local samples_file
  samples_file=$(mktemp)

  if [ -z "$htlc" ] || [ "$htlc" = "0x" ]; then
    log_error "HTLC_ADDRESS empty — cannot correlate TTF (run contracts.sync-addresses)"
    rm -f "$samples_file"
    return 1
  fi

  local line id t0_ms topic logs block t1_s ttf_s n=0
  while read -r line; do
    # line: CONTRACT_ID <id> [<t0_ms>]
    set -- $line
    id="$2"; t0_ms="${3:-}"
    [ -n "$id" ] || continue
    topic=$(_perf_pad_topic "$id")
    logs=$(_perf_rpc "$rpc" "eth_getLogs" "$(jq -nc \
      --arg addr "$htlc" --arg t0 "$PERF_TTF_TOPIC0" --arg t1 "$topic" \
      '[{address:$addr, fromBlock:"earliest", toBlock:"latest", topics:[$t0,$t1]}]')") || continue
    block=$(printf '%s' "$logs" | jq -r '.[0].blockNumber // empty')
    [ -n "$block" ] || { log_warn "no LogHTLCLocked event found" contract_id="$id"; continue; }
    t1_s=$(_perf_block_ts_seconds "$rpc" "$block") || continue
    if [ -n "$t0_ms" ]; then
      # TTF in seconds with millisecond input on t0.
      ttf_s=$(awk -v t1="$t1_s" -v t0ms="$t0_ms" 'BEGIN{printf "%.3f", t1 - (t0ms/1000.0)}')
    else
      log_warn "no t0 for contract — block-only timing unavailable" contract_id="$id"
      continue
    fi
    # Guard against negative skew (clock drift) — clamp to 0.
    awk -v v="$ttf_s" 'BEGIN{exit !(v < 0)}' && ttf_s=0
    echo "$ttf_s" >> "$samples_file"
    n=$((n + 1))
  done < <(grep '^CONTRACT_ID' "$ids_file" 2>/dev/null)

  if [ "$n" -eq 0 ]; then
    log_error "TTF: no samples correlated" ids_file="$ids_file"
    jq -nc '{p50_s:null,p95_s:null,samples:0,gate_pass:false,note:"no samples correlated"}' > "$out"
    rm -f "$samples_file"
    return 1
  fi

  # p50 / p95 over the sample set.
  local p50 p95
  p50=$(sort -n "$samples_file" | awk '{a[NR]=$1} END{print a[int((NR+1)*0.50+0.5)>NR?NR:int((NR+1)*0.50+0.5)]}')
  p95=$(sort -n "$samples_file" | awk '{a[NR]=$1} END{i=int((NR)*0.95+0.999); if(i<1)i=1; if(i>NR)i=NR; print a[i]}')
  local pass
  pass=$(awk -v p="$p95" 'BEGIN{print (p < 5.0) ? "true" : "false"}')

  jq -nc --argjson p50 "$p50" --argjson p95 "$p95" --argjson n "$n" --argjson pass "$pass" \
    '{p50_s:$p50, p95_s:$p95, samples:$n, gate_pass:$pass, gate:"p95 < 5s per spoke"}' > "$out"
  log_info "TTF computed" samples="$n" p50_s="$p50" p95_s="$p95" gate_pass="$pass"
  rm -f "$samples_file"
}
