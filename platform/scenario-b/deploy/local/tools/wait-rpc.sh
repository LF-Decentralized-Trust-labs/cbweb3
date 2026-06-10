#!/usr/bin/env bash
#
# wait-rpc.sh <rpc-url> [timeout-secs]
#
# Blocks until an EVM JSON-RPC endpoint reliably serves eth_getBlockByNumber("latest")
# with a full block result — which is precisely what forge's fork backend needs when it
# instantiates the forked environment at deploy time. Freshly-started Besu QBFT nodes
# answer admin/ENODE calls before they serve block reads cleanly, causing forge to fail
# with "could not instantiate forked environment / failed to get latest block". Gating
# each deploy on this check removes that cold-start race.
#
# Requires N consecutive good reads (not just one) to avoid a flaky just-booted node.
# Uses curl so it has no dependency on foundry/cast being on PATH.
#
set -euo pipefail

RPC="${1:?usage: wait-rpc.sh <rpc-url> [timeout-secs]}"
TIMEOUT="${2:-90}"
NEED_CONSECUTIVE=3

check() {
  curl -fsS -m 3 -X POST -H 'content-type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["latest",false]}' \
    "${RPC}" 2>/dev/null | grep -q '"result":{'
}

echo "  [wait-rpc] waiting for ${RPC} to serve blocks (need ${NEED_CONSECUTIVE} consecutive, timeout ${TIMEOUT}s)..."
ok=0
for ((i = 0; i < TIMEOUT; i++)); do
  if check; then
    ok=$((ok + 1))
    if [[ "${ok}" -ge "${NEED_CONSECUTIVE}" ]]; then
      echo "  [wait-rpc] ${RPC} ready"
      exit 0
    fi
  else
    ok=0
  fi
  sleep 1
done

echo "  [wait-rpc] ERROR: ${RPC} did not serve a stable latest block within ${TIMEOUT}s" >&2
exit 1
