#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-alpine-version.sh — one Alpine version across the repository.
#
# Why this exists. R2-M-12 moved the shipped service images to a supported Alpine
# and deliberately left the throwaway volume/log helpers alone. That was right for
# that card's scope, but it left the tree holding three values at once with nothing
# to say which was intentional: a reader opening volumefs.go saw `alpine:3.20` next
# to Dockerfiles on `alpine:3.23` and could not tell whether the difference meant
# something.
#
# The security exposure of the helpers is small — they run sh, cp, chmod, chown and
# exit. The real costs are the ambiguity and having to rediscover every reference at
# the next bump. This gate answers both: the pinned version lives in
# docs/TOOLCHAIN.md (which CLAUDE.md names as the authority on toolchain versions),
# and a bump becomes "edit the pin, run this, fix what it lists".
#
# Why this checker is deliberately repo-wide, and why the change that introduced it
# touches both scenarios. The constitution's scenario-isolation rule exists to stop
# product logic leaking between scenario-a/ and scenario-b/: a behaviour change in one
# must not ride along in the other. A toolchain pin is the opposite kind of object. It
# is a single fact about the build environment, and TOOLCHAIN.md already states that
# the floor "is the same for both scenarios" and that per-scenario splits must be
# recorded as deviations. Pinning Alpine per scenario would therefore create exactly
# the drift the rule is meant to prevent, and would make this gate unable to answer
# the only question worth asking: is the tree on one version? No product code, no
# contract, no service behaviour crosses between the trees here — the change is a
# version literal plus the constant each tree already owned.
#
# Usage: bash tools/check-alpine-version.sh [scan-root]
# Exit:  0 = every reference matches the pin
#        1 = a reference disagrees with the pin, or no pin was found
#        2 = the scan could not complete (never reported as success)
#
# Run with bash, never zsh: zsh does not fork the last stage of a pipeline, which
# masks the class of bug that once let the licence gate report success without
# verifying anything.
set -uo pipefail

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${1:-$(cd "$SELF/.." && pwd)}"
PIN_FILE="$ROOT/docs/TOOLCHAIN.md"
PATTERN='alpine:3\.[0-9]+'

# The pin is read from the document of record rather than hardcoded here, so this
# gate cannot drift from what the docs claim. The row's first cell is the image.
# The row is identified by its first cell ("Alpine 3.x"), matching the table's
# existing convention of indexing by version rather than by image name.
PINNED=""
if [ -f "$PIN_FILE" ]; then
  PINNED="$(grep -E '^\| *Alpine 3\.[0-9]+ *\|' "$PIN_FILE" | grep -oE "$PATTERN" | head -1)"
fi
if [ -z "$PINNED" ]; then
  echo "FAIL: no Alpine pin found in ${PIN_FILE#$ROOT/}." >&2
  echo "      Add a row to the enforcement table whose first cell is the version and" >&2
  echo "      which names the image, e.g." >&2
  echo "      | Alpine 3.23 | \`alpine:3.23\` in every helper ... | ... |" >&2
  exit 1
fi
echo "pinned Alpine (from ${PIN_FILE#$ROOT/}): $PINNED"

# Collect files with find, capturing its exit status directly rather than through a
# pipeline: an unreadable directory must abort, not silently shrink the file list.
list="$(mktemp)"
trap 'rm -f "$list"' EXIT
find "$ROOT" \
  \( -name node_modules -o -name vendor -o -name .git -o -name dist \
     -o -name build -o -name .next -o -name .turbo \) -prune -o \
  -type f -print0 > "$list"
find_rc=$?
if [ "$find_rc" -ne 0 ]; then
  echo "FAIL: the scan could not complete (find exited $find_rc)." >&2
  echo "      Refusing to report success on a partial scan." >&2
  exit 2
fi

offenders=0
total=0
while IFS= read -r -d '' file; do
  rel="${file#$ROOT/}"
  # This checker and its self-test necessarily name other versions to describe the
  # rule, so they are not subject to it.
  case "$rel" in tools/check-alpine-version*) continue ;; esac
  # -I skips binaries; a match count of 0 costs nothing.
  while IFS= read -r hit; do
    [ -z "$hit" ] && continue
    total=$((total + 1))
    case "$hit" in
      *"$PINNED"*) ;;
      *)
        offenders=$((offenders + 1))
        echo "::error file=$rel::Alpine reference does not match the pin $PINNED"
        echo "  $rel:$hit"
        ;;
    esac
  done < <(grep -InE "$PATTERN" "$file" 2>/dev/null)
done < "$list"

echo ""
if [ "$offenders" -gt 0 ]; then
  echo "FAIL: $offenders of $total Alpine reference(s) do not use $PINNED." >&2
  echo "      Bump them, or change the pin in docs/TOOLCHAIN.md if the fleet is moving." >&2
  exit 1
fi
echo "OK: all $total Alpine reference(s) use $PINNED."
