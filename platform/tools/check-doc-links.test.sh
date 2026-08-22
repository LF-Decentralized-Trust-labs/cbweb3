#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-doc-links.test.sh — regression test for check-doc-links.sh.
#
# Pins the failure path first, for the reason the licence gate taught the project: a
# checker that counts inside a pipeline loses the counter to a subshell and reports
# success while printing the very problems it found. Every assertion here checks the
# exit status AND the reported count.
#
# Usage: bash tools/check-doc-links.test.sh
set -uo pipefail

CHECKER="$(cd "$(dirname "$0")" && pwd)/check-doc-links.sh"
failures=0
fixture=""

cleanup() { [ -n "$fixture" ] && rm -rf "$fixture"; return 0; }
trap cleanup EXIT
fixture="$(mktemp -d)"

pass() { echo "ok   — $1"; }
fail() { echo "FAIL — $1"; failures=$((failures + 1)); }

run() {
  local want="$1" desc="$2" out rc
  out=$(bash "$CHECKER" "$fixture" 2>&1); rc=$?
  if [ "$rc" -ne "$want" ]; then
    fail "$desc (exit $rc, wanted $want)"; echo "$out" | sed 's/^/       /'; return 1
  fi
  LAST_OUT="$out"; pass "$desc"; return 0
}

mkdir -p "$fixture/docs/sub"

# --- 1. a link to a file that exists passes -----------------------------------
printf 'ok\n' > "$fixture/docs/target.md"
printf 'See [target](target.md)\n' > "$fixture/docs/index.md"
if run 0 "a resolvable link passes"; then
  case $LAST_OUT in
    *"all 1 relative link"*) pass "counts the link it checked" ;;
    *) fail "expected 1 link counted, got: $LAST_OUT" ;;
  esac
fi

# --- 2. a broken link fails, and is counted ----------------------------------
printf 'See [gone](nope.md)\n' >> "$fixture/docs/index.md"
if run 1 "a broken link fails"; then
  case $LAST_OUT in
    *"FAIL: 1 of 2"*) pass "counter survives the loops (1 of 2)" ;;
    *) fail "expected '1 of 2', got: $LAST_OUT" ;;
  esac
  case $LAST_OUT in
    *'::error file=docs/index.md::'*) pass "emits a GitHub annotation" ;;
    *) fail "missing ::error annotation" ;;
  esac
fi

# --- 3. a SECOND broken link must not be swallowed ---------------------------
# The assertion that would have caught the subshell-counter bug: a broken counter
# typically still reports 1.
printf 'And [also gone](missing/other.md)\n' >> "$fixture/docs/index.md"
if run 1 "two broken links still fail"; then
  case $LAST_OUT in
    *"FAIL: 2 of 3"*) pass "counts every broken link (2 of 3)" ;;
    *) fail "expected '2 of 3', got: $LAST_OUT" ;;
  esac
fi

# --- 4. external links and anchors are not filesystem paths -------------------
rm -f "$fixture/docs/index.md"
{
  printf 'A [site](https://example.org/x) and an [anchor](#section)\n'
  printf 'and [mail](mailto:a@b.c) and [http](http://example.org)\n'
} > "$fixture/docs/index.md"
if run 0 "external links, anchors and mailto are skipped"; then
  case $LAST_OUT in
    *"all 0 relative link"*) pass "none of them counted as a path" ;;
    *) fail "expected 0 relative links, got: $LAST_OUT" ;;
  esac
fi

# --- 5. a link carrying an anchor resolves on the file part -------------------
printf 'See [target](target.md#heading)\n' > "$fixture/docs/index.md"
run 0 "a link with an #anchor resolves on the file part"

# --- 6. links resolve relative to the FILE, not the scan root ----------------
# The bug this pins: resolving every link from docs/ makes a correct link in a
# subdirectory look broken, and a broken one in the root look fine.
printf 'up to [target](../target.md)\n' > "$fixture/docs/sub/deep.md"
rm -f "$fixture/docs/index.md"
run 0 "a relative link from a subdirectory resolves against its own directory"

# --- 7. a link to a directory is valid ---------------------------------------
printf 'see [the subdir](sub/)\n' > "$fixture/docs/index.md"
run 0 "a link to a directory resolves"

# --- 8. no docs/ at all is an error, not a silent pass ------------------------
mv "$fixture/docs" "$fixture/docs-renamed"
if run 1 "a tree with no docs/ fails instead of passing vacuously"; then
  case $LAST_OUT in
    *"no docs/ directory"*) pass "says why it refused" ;;
    *) fail "expected the missing-docs message, got: $LAST_OUT" ;;
  esac
fi
mv "$fixture/docs-renamed" "$fixture/docs"

echo ""
if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures assertion(s) failed in $(basename "$0")."
  exit 1
fi
echo "OK: check-doc-links.sh behaves correctly on both the pass and fail paths."
