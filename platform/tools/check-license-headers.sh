#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-license-headers.sh — fail if any first-party source file is missing an
# SPDX-License-Identifier header. Covers Go and TypeScript across both scenarios.
#
# Scope (matches the DPG license-header policy in docs/DPG-COMPLIANCE.md):
#   - Hand-written .go files. Generated code is excluded: *.pb.go, *_grpc.pb.go,
#     and abigen output under any /bindings/ directory.
#   - First-party .ts / .tsx under source trees. Vendored / build output is
#     excluded: node_modules, dist, build, .next, .turbo.
#
# Usage: tools/check-license-headers.sh
# Exit:  0 = all good, 1 = one or more files missing the header.
set -euo pipefail

cd "$(dirname "$0")/.."

SPDX='SPDX-License-Identifier'
missing=0

has_header() {
  # Header must appear within the first 5 lines.
  head -5 "$1" | grep -q "$SPDX"
}

check() {
  local label="$1"; shift
  local f
  while IFS= read -r f; do
    if ! has_header "$f"; then
      echo "::error file=$f::missing $SPDX header"
      echo "MISSING ($label): $f"
      missing=$((missing + 1))
    fi
  done
}

# Go — exclude vendored + generated sources.
find . -type f -name '*.go' \
  -not -path '*/vendor/*' \
  -not -name '*.pb.go' \
  -not -path '*/bindings/*' \
  | check go

# TypeScript — exclude vendored + build output.
find . -type f \( -name '*.ts' -o -name '*.tsx' \) \
  -not -path '*/node_modules/*' \
  -not -path '*/dist/*' \
  -not -path '*/build/*' \
  -not -path '*/.next/*' \
  -not -path '*/.turbo/*' \
  | check ts

if [ "$missing" -gt 0 ]; then
  echo ""
  echo "FAIL: $missing file(s) missing an $SPDX header."
  echo "Add '// $SPDX: Apache-2.0' as the first line (see docs/DPG-COMPLIANCE.md)."
  exit 1
fi

echo "OK: all first-party Go and TypeScript sources carry an $SPDX header."
