#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-doc-links.sh — every relative link under docs/ resolves to a real path.
#
# Why this exists. docs/architecture/viewpoints.md is an index: it tells a reader which
# document answers which architectural concern. An index whose links rot is worse than
# no index — a declared gap warns the reader, a dead reference misleads them. The
# viewpoints file says exactly that about itself, so the claim needs enforcement rather
# than good intentions.
#
# Scope is deliberately docs/ only, not the whole repository. Measured at the time this
# was written: docs/ had 174 relative links and zero broken, so this gate starts green
# and protects that. The per-scenario trees did NOT: scenario-a/docs had 29 broken links
# and scenario-b/docs 26, from two causes — missing screenshot assets, and one
# off-by-one in relative depth in scenario-a/docs/INDEX.md (../../../ overshoots the
# repository root). Widening this gate is worth doing AFTER that debt is paid; widening
# it now would only mean disabling it.
#
# Usage: bash tools/check-doc-links.sh [scan-root]
# Exit:  0 = every link resolves, 1 = at least one does not
#
# Run with bash, never zsh: zsh does not fork the last stage of a pipeline, which masks
# the class of bug that once let the licence gate report success without verifying.
set -uo pipefail

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${1:-$(cd "$SELF/.." && pwd)}"
DOCS="$ROOT/docs"

if [ ! -d "$DOCS" ]; then
  echo "FAIL: no docs/ directory under $ROOT" >&2
  exit 1
fi

broken=0
total=0
files=0

while IFS= read -r -d '' md; do
  files=$((files + 1))
  dir="$(dirname "$md")"
  # Markdown inline links, minus external ones and pure anchors. The link target is
  # everything up to a '#' or whitespace.
  while IFS= read -r target; do
    [ -z "$target" ] && continue
    case "$target" in
      http://*|https://*|mailto:*|"#"*) continue ;;
    esac
    total=$((total + 1))
    # Resolve relative to the file's own directory.
    if [ ! -e "$dir/$target" ]; then
      broken=$((broken + 1))
      rel="${md#$ROOT/}"
      echo "::error file=$rel::broken link: $target"
      echo "  $rel -> $target"
    fi
  done < <(grep -oE '\]\([^)]+\)' "$md" 2>/dev/null | sed -E 's/^\]\(//; s/\)$//; s/[#[:space:]].*$//')
done < <(find "$DOCS" -type f -name '*.md' -print0)

echo ""
if [ "$broken" -gt 0 ]; then
  echo "FAIL: $broken of $total relative link(s) in $files file(s) under docs/ do not resolve." >&2
  exit 1
fi
echo "OK: all $total relative link(s) in $files file(s) under docs/ resolve."
