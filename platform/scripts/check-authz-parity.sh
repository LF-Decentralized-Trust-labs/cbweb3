#!/usr/bin/env bash
# R2-H-8: the per-scenario gRPC authz packages must stay byte-identical so a
# security fix cannot land in one scenario only. Exits non-zero on any drift.
set -euo pipefail

A="scenario-a/backend/shared/proto/authz"
B="scenario-b/backend/shared/proto/authz"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$HERE"

status=0
# Compare in both directions so an added/removed file in either copy is caught.
while IFS= read -r f; do
  rel="${f#"$A/"}"
  if [[ ! -f "$B/$rel" ]]; then
    echo "DRIFT: $A/$rel has no counterpart in $B" >&2
    status=1
  elif ! diff -q "$A/$rel" "$B/$rel" >/dev/null; then
    echo "DRIFT: $A/$rel differs from $B/$rel" >&2
    status=1
  fi
done < <(find "$A" -type f -name '*.go' | sort)

while IFS= read -r f; do
  rel="${f#"$B/"}"
  if [[ ! -f "$A/$rel" ]]; then
    echo "DRIFT: $B/$rel has no counterpart in $A" >&2
    status=1
  fi
done < <(find "$B" -type f -name '*.go' | sort)

if [[ "$status" -eq 0 ]]; then
  echo "authz parity OK — scenario-a and scenario-b copies are identical"
fi
exit "$status"
