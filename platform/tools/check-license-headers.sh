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
# Usage: tools/check-license-headers.sh [scan-root]
#        scan-root defaults to the repository root. It exists so the self-test
#        (tools/check-license-headers.test.sh) can point the checker at a fixture
#        tree; CI always runs it with no argument.
# Exit:  0 = all good, 1 = one or more files missing the header,
#        2 = the scan itself could not complete (never reported as success).
set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"

SPDX='SPDX-License-Identifier'
missing=0

has_header() {
  # Header must appear within the first 5 lines. Read into a variable instead of
  # piping into grep: under `set -o pipefail` a short-circuiting grep can leave
  # head killed by SIGPIPE, which would look like a missing header.
  local head5
  head5=$(head -5 "$1")
  [[ $head5 == *"$SPDX"* ]]
}

report() {
  local label="$1" file="$2"
  echo "::error file=$file::missing $SPDX header"
  echo "MISSING ($label): $file"
}

LIST="$(mktemp)"
trap 'rm -f "$LIST"' EXIT

# scan <label> <find-command...> — count files under $LIST that lack the header.
#
# Two failure modes are deliberately designed out here:
#
#  1. The counter must live in THIS shell. Bash runs the right-hand side of a
#     pipeline in a subshell, so `missing=$((missing + 1))` inside a
#     `find ... | while` loop is discarded and the parent still reads 0 — that is
#     exactly how this gate printed OK while 94 files had no header. The loop
#     therefore reads from a file, never from a pipe. Do not reintroduce
#     `find ... | while` here. (zsh does not fork the last pipeline stage, so the
#     bug is invisible under zsh; always test with bash.)
#
#  2. Discovery failure must not read as a clean tree. Process substitution
#     silently drops find's exit status, so an unreadable directory would shrink
#     the file list and still reach the OK message. find runs as its own command
#     with its status checked, and a failed traversal aborts with exit 2.
scan() {
  local label="$1"; shift
  local f rc=0
  "$@" >"$LIST" || rc=$?
  if [ "$rc" -ne 0 ]; then
    echo ""
    echo "ERROR: could not enumerate $label sources — find failed (exit $rc)."
    echo "Refusing to report success on a partial scan."
    exit 2
  fi
  while IFS= read -r f; do
    has_header "$f" || { report "$label" "$f"; missing=$((missing + 1)); }
  done <"$LIST"
}

# Go — exclude vendored + generated sources.
scan go find . -type f -name '*.go' \
  -not -path '*/vendor/*' \
  -not -name '*.pb.go' \
  -not -path '*/bindings/*'

# TypeScript — exclude vendored + build output.
scan ts find . -type f '(' -name '*.ts' -o -name '*.tsx' ')' \
  -not -path '*/node_modules/*' \
  -not -path '*/dist/*' \
  -not -path '*/build/*' \
  -not -path '*/.next/*' \
  -not -path '*/.turbo/*'

if [ "$missing" -gt 0 ]; then
  echo ""
  echo "FAIL: $missing file(s) missing an $SPDX header."
  echo "Add '// $SPDX: Apache-2.0' as the first line (see docs/DPG-COMPLIANCE.md)."
  exit 1
fi

echo "OK: all first-party Go and TypeScript sources carry an $SPDX header."
