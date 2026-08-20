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
# TAGGED matches any tagged Alpine reference, not just a 3.x one, so `alpine:latest`
# and `alpine:edge` are offenders rather than invisible. That gap mattered: the parent
# card R2-M-12 was ABOUT `alpine:latest`, and a pattern that only saw `alpine:3.x`
# could not have caught the very thing it follows up.
#
# It deliberately needs a colon and a tag. Ten images in this tree carry `-alpine` as a
# TAG SUFFIX — golang:1.26-alpine, node:22-alpine, postgres:17-alpine, redis:7-alpine,
# nginx:1.27-alpine, caddy:2-alpine and the ${X:-N-alpine} env defaults. Those are other
# images that happen to be Alpine-based and are not governed by this pin; none has a
# colon after "alpine", so requiring one excludes them without an exception list.
TAGGED='alpine:[A-Za-z0-9._-]+'

# UNTAGGED catches `FROM alpine`, `image: alpine` and `docker run … alpine`, which mean
# :latest by omission. Restricted to those three image positions on purpose: a bare
# match on the word would flag every sentence that mentions Alpine.
UNTAGGED='(FROM[[:space:]]+|image:[[:space:]]*"?|docker[[:space:]]+run[[:space:]].*[[:space:]])alpine([[:space:]"'"'"']|$)'

# PATTERN is what the PIN row is read with — the pin itself is always a version.
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

# is_comment <grep-hit> — true when the matched line is a comment in any of the
# languages this tree uses (#, //). grep -n prefixes "LINE:", stripped first.
is_comment() {
  local text="${1#*:}"
  case "${text#"${text%%[![:space:]]*}"}" in
    '#'*|'//'*) return 0 ;;
    *) return 1 ;;
  esac
}

offenders=0
total=0
while IFS= read -r -d '' file; do
  rel="${file#$ROOT/}"
  # Files whose job is to name bad versions are not subject to the rule: this checker
  # and its self-test, and the sibling Dockerfile-hardening guards, whose fixture
  # tables list `alpine:latest` precisely as the thing to reject.
  case "$rel" in
    tools/check-alpine-version*) continue ;;
    *dockerfile_hardening_test.go) continue ;;
  esac
  # -I skips binaries; a match count of 0 costs nothing.
  while IFS= read -r hit; do
    [ -z "$hit" ] && continue
    # A commented-out or narrated reference is not one Docker will ever pull. Several
    # Dockerfiles explain in a comment why they moved off `alpine:latest`; flagging that
    # would punish the documentation R2-M-12 asked for.
    is_comment "$hit" && continue
    total=$((total + 1))
    case "$hit" in
      *"$PINNED"*) ;;
      *)
        offenders=$((offenders + 1))
        echo "::error file=$rel::Alpine reference does not match the pin $PINNED"
        echo "  $rel:$hit"
        ;;
    esac
  done < <(grep -InE "$TAGGED" "$file" 2>/dev/null)

  # An untagged reference is :latest by omission — the defect R2-M-12 fixed. It is
  # counted and reported separately because there is no wrong version to name.
  while IFS= read -r hit; do
    [ -z "$hit" ] && continue
    is_comment "$hit" && continue
    total=$((total + 1))
    offenders=$((offenders + 1))
    echo "::error file=$rel::untagged Alpine reference; pin it to $PINNED"
    echo "  $rel:$hit"
  done < <(grep -InE "$UNTAGGED" "$file" 2>/dev/null)
done < "$list"

echo ""
if [ "$offenders" -gt 0 ]; then
  echo "FAIL: $offenders of $total Alpine reference(s) do not use $PINNED." >&2
  echo "      Bump them, or change the pin in docs/TOOLCHAIN.md if the fleet is moving." >&2
  exit 1
fi
echo "OK: all $total Alpine reference(s) use $PINNED."
