#!/usr/bin/env bash
#
# mock-fx-feeder.sh — minimal mocked FX-rate feeder for the W-BRL/W-ARS pair.
#
# Pushes a real-life-like, slightly-jittered BRL/ARS rate into the Hub ManualOracle
# on a fixed interval for a bounded duration (default: every 30s for 1 hour). This
# stands in for a real external price feed: the api-gateway reads ManualOracle.getRate
# to suggest a counterpart matching amount, and this script keeps that rate fresh and
# moving so the suggestion looks realistic during a demo.
#
# The oracle pair key is ORDER-SENSITIVE (keccak(token0, token1)), so each tick sets
# BOTH directions: getRate(BRL, ARS) = ARS-per-BRL, and the inverse getRate(ARS, BRL).
#
# Signer must hold CENTRAL_BANK_ROLE on the oracle. In local dev that is the platform
# admin/deployer (0x627306…), whose key already has the role.
#
# Usage:
#   ./mock-fx-feeder.sh                 # defaults below, runs ~1h
#   DURATION_SECS=600 INTERVAL_SECS=15 ./mock-fx-feeder.sh
#
set -euo pipefail

# ── Config (override via env) ────────────────────────────────────────────────
# Addresses default to whatever the deploy synced into the CB-A env (so they track
# nuke+up redeploys automatically), then fall back to the well-known local-dev values.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/backend/config/.env.infra.central-bank-a}"
CONTRACTS_ENV="${CONTRACTS_ENV:-${ROOT_DIR}/contracts/.env}"

read_env() { # read_env <KEY> <FILE>
  [[ -f "$2" ]] && grep -E "^$1=" "$2" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"[:space:]'
}

RPC_URL="${RPC_URL:-http://localhost:8845}"                       # Hub Besu RPC
ORACLE="${ORACLE:-$(read_env ORACLE_ADDRESS "${ENV_FILE}")}"
ORACLE="${ORACLE:-0xfb88de099e13c3ed21f80a7a1e49f8caecf10df6}"     # ManualOracle (Hub)
TOKEN_BRL="${TOKEN_BRL:-$(read_env SOVEREIGN_HUB_TOKEN_A_ADDRESS "${ENV_FILE}")}"
TOKEN_BRL="${TOKEN_BRL:-0x38cf23c52bb4b13f051aec09580a2de845a7fa35}" # W-tCeBM_BRL (18dp)
TOKEN_ARS="${TOKEN_ARS:-$(read_env SOVEREIGN_HUB_TOKEN_B_ADDRESS "${ENV_FILE}")}"
TOKEN_ARS="${TOKEN_ARS:-0x4e72770760c011647d4873f60a3cf6cdea896cd8}" # W-tCeBM_ARS (18dp)
# Signer with CENTRAL_BANK_ROLE on the oracle (system-managed = admin/deployer 0x627306…).
SIGNER_KEY="${SIGNER_KEY:-$(read_env ADMIN_PRIVATE_KEY "${CONTRACTS_ENV}")}"
SIGNER_KEY="${SIGNER_KEY:-0xc87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3}"
[[ "${SIGNER_KEY}" == 0x* ]] || SIGNER_KEY="0x${SIGNER_KEY}"

BASE_RATE="${BASE_RATE:-250.0}"        # ARS per 1 BRL (real-life-like base)
JITTER_PCT="${JITTER_PCT:-2.0}"        # +/- random-walk band around base, in percent
DURATION_SECS="${DURATION_SECS:-3600}" # total run time; <=0 runs until killed (used by make up)
INTERVAL_SECS="${INTERVAL_SECS:-30}"   # seconds between updates

# ── Pre-flight ───────────────────────────────────────────────────────────────
command -v cast >/dev/null || { echo "ERROR: foundry 'cast' not found in PATH" >&2; exit 1; }
ROLE="$(cast keccak 'CENTRAL_BANK_ROLE')"
SIGNER_ADDR="$(cast wallet address --private-key "$SIGNER_KEY")"
HAS_ROLE="$(cast call "$ORACLE" 'hasRole(bytes32,address)(bool)' "$ROLE" "$SIGNER_ADDR" --rpc-url "$RPC_URL" 2>/dev/null || echo error)"
if [[ "$HAS_ROLE" != "true" ]]; then
  echo "ERROR: signer $SIGNER_ADDR lacks CENTRAL_BANK_ROLE on oracle $ORACLE (hasRole=$HAS_ROLE)" >&2
  exit 1
fi

echo "mock-fx-feeder starting"
echo "  oracle   : $ORACLE @ $RPC_URL"
echo "  pair     : W-BRL($TOKEN_BRL) / W-ARS($TOKEN_ARS)"
echo "  signer   : $SIGNER_ADDR"
if [[ "${DURATION_SECS}" -le 0 ]]; then
  echo "  base     : $BASE_RATE ARS/BRL  +/-${JITTER_PCT}%  every ${INTERVAL_SECS}s until stopped"
else
  echo "  base     : $BASE_RATE ARS/BRL  +/-${JITTER_PCT}%  every ${INTERVAL_SECS}s for ${DURATION_SECS}s"
fi
echo ""

# ── Feed loop ────────────────────────────────────────────────────────────────
# DURATION_SECS <= 0 runs indefinitely (until the process is killed) — used by `make up`
# so the feeder lives for the life of the stack. A positive value bounds the run.
END_TS=$(( $(date +%s) + DURATION_SECS ))
prev="$BASE_RATE"
while [[ "${DURATION_SECS}" -le 0 || $(date +%s) -lt $END_TS ]]; do
  # Mean-reverting random walk around BASE_RATE, bounded to the jitter band.
  # Emits: <human_rate> <fwd_wei (ARS per BRL, 1e18)> <inv_wei (BRL per ARS, 1e18)>
  read -r human fwd_wei inv_wei < <(python3 - "$prev" "$BASE_RATE" "$JITTER_PCT" <<'PY'
import sys, random
prev, base, jitter = float(sys.argv[1]), float(sys.argv[2]), float(sys.argv[3])
step = base * (jitter/100.0) * random.uniform(-0.35, 0.35)   # small per-tick move
val = prev + step + (base - prev) * 0.10                     # gentle pull toward base
lo, hi = base*(1-jitter/100.0), base*(1+jitter/100.0)
val = max(lo, min(hi, val))
fwd = int(val * 10**18)              # ARS per BRL, 18dp
inv = int((10**18 * 10**18) // fwd)  # BRL per ARS, 18dp
print(f"{val:.4f} {fwd} {inv}")
PY
)
  prev="$human"
  ts="$(date '+%H:%M:%S')"
  if cast send "$ORACLE" 'setRate(address,address,uint256)' "$TOKEN_BRL" "$TOKEN_ARS" "$fwd_wei" \
        --private-key "$SIGNER_KEY" --rpc-url "$RPC_URL" >/dev/null 2>&1 \
     && cast send "$ORACLE" 'setRate(address,address,uint256)' "$TOKEN_ARS" "$TOKEN_BRL" "$inv_wei" \
        --private-key "$SIGNER_KEY" --rpc-url "$RPC_URL" >/dev/null 2>&1; then
    echo "[$ts] rate set: 1 BRL = $human ARS  (fwd=$fwd_wei inv=$inv_wei)"
  else
    echo "[$ts] WARN: setRate failed (rpc=$RPC_URL) — will retry next tick" >&2
  fi
  sleep "$INTERVAL_SECS"
done

echo "mock-fx-feeder finished after ${DURATION_SECS}s"
