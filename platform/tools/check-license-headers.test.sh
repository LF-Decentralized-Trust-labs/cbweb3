#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-license-headers.test.sh — regression test for check-license-headers.sh.
#
# The gate this guards once counted offending files inside a `find ... | while`
# loop. Bash runs the right-hand side of a pipeline in a subshell, so the counter
# never escaped it and the check exited 0 while reporting MISSING lines. These
# tests pin the failure path: a header-less fixture MUST produce exit 1.
#
# Usage: bash tools/check-license-headers.test.sh
# Exit:  0 = all assertions passed, 1 = at least one failed.
set -uo pipefail

CHECKER="$(cd "$(dirname "$0")" && pwd)/check-license-headers.sh"
SPDX_LINE='// SPDX-License-Identifier: Apache-2.0'

failures=0
fixture=""

cleanup() {
  # u+rwX first: an interrupted run can leave the chmod 000 fixture behind, and
  # rm -rf cannot descend into it. Always returns 0 so the trap never masks the
  # script's own exit status.
  [ -n "$fixture" ] && { chmod -R u+rwX "$fixture" 2>/dev/null; rm -rf "$fixture"; }
  return 0
}
trap cleanup EXIT

# Fixture lives outside the repository tree so the real scan never sees it.
fixture="$(mktemp -d)"

pass() { echo "ok   — $1"; }
fail() { echo "FAIL — $1"; failures=$((failures + 1)); }

# run <expected-exit> <description> — invokes the checker against the fixture.
run() {
  local want="$1" desc="$2" out rc
  # Invoked via `bash` — same as the CI step, and independent of the exec bit.
  out=$(bash "$CHECKER" "$fixture" 2>&1)
  rc=$?
  if [ "$rc" -ne "$want" ]; then
    fail "$desc (exit $rc, wanted $want)"
    echo "$out" | sed 's/^/       /'
    return 1
  fi
  LAST_OUT="$out"
  pass "$desc"
  return 0
}

# --- 1. header-less Go file must fail -----------------------------------------
mkdir -p "$fixture/svc"
printf 'package svc\n\nfunc A() {}\n' > "$fixture/svc/a.go"

if run 1 "header-less .go file exits 1"; then
  case $LAST_OUT in
    *FAIL*) pass "output contains FAIL" ;;
    *) fail "output should contain FAIL, got: $LAST_OUT" ;;
  esac
  case $LAST_OUT in
    *'::error file=./svc/a.go::'*) pass "emits GitHub error annotation" ;;
    *) fail "missing ::error annotation for ./svc/a.go" ;;
  esac
  case $LAST_OUT in
    *'FAIL: 1 file(s)'*) pass "counter survives the loop (reports 1)" ;;
    *) fail "counter did not reach the FAIL message" ;;
  esac
fi

# --- 2. header-less TypeScript file must also fail ----------------------------
printf 'export const a = 1\n' > "$fixture/svc/a.ts"
if run 1 "header-less .ts file exits 1"; then
  case $LAST_OUT in
    *'FAIL: 2 file(s)'*) pass "counts every offender (reports 2)" ;;
    *) fail "expected 2 offenders, got: $LAST_OUT" ;;
  esac
fi

# --- 3. adding headers makes it pass ------------------------------------------
printf '%s\n\npackage svc\n\nfunc A() {}\n' "$SPDX_LINE" > "$fixture/svc/a.go"
printf '%s\n\nexport const a = 1\n' "$SPDX_LINE" > "$fixture/svc/a.ts"
if run 0 "clean tree exits 0"; then
  case $LAST_OUT in
    *OK:*) pass "output contains OK" ;;
    *) fail "output should contain OK, got: $LAST_OUT" ;;
  esac
fi

# --- 4. header past the 5-line window does not count --------------------------
printf 'package svc\n\n// 2\n// 3\n// 4\n// 5\n%s\n' "$SPDX_LINE" > "$fixture/svc/late.go"
run 1 "header after line 5 is rejected" && :
rm -f "$fixture/svc/late.go"

# --- 5. documented exclusions are skipped -------------------------------------
mkdir -p "$fixture/vendor/x" "$fixture/svc/bindings" "$fixture/web/node_modules" \
  "$fixture/web/dist" "$fixture/web/build" "$fixture/web/.next" "$fixture/web/.turbo"
printf 'package x\n' > "$fixture/vendor/x/x.go"
printf 'package svc\n' > "$fixture/svc/bindings/gen.go"
printf 'package svc\n' > "$fixture/svc/api.pb.go"
for d in node_modules dist build .next .turbo; do
  printf 'export const x = 1\n' > "$fixture/web/$d/x.ts"
done
run 0 "excluded paths are not scanned"

# --- 6. a scan that cannot complete must not report success -------------------
# Process substitution drops find's exit status, so an unreadable directory would
# otherwise shrink the file list and still print OK. Skipped as root, where
# chmod 000 does not deny access.
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$fixture/locked"
  printf 'package svc\n' > "$fixture/locked/hidden.go"
  chmod 000 "$fixture/locked"
  if run 2 "unreadable directory aborts with exit 2"; then
    case $LAST_OUT in
      *"Refusing to report success"*) pass "explains why it refused" ;;
      *) fail "expected the refusal message, got: $LAST_OUT" ;;
    esac
    case $LAST_OUT in
      *OK:*) fail "must not print OK on a partial scan" ;;
      *) pass "does not print OK on a partial scan" ;;
    esac
  fi
  chmod 755 "$fixture/locked"
  rm -rf "$fixture/locked"
else
  echo "skip — running as root, cannot make a directory unreadable"
fi

# --- 7. paths with spaces are handled one file at a time ----------------------
mkdir -p "$fixture/dir with space"
printf 'package svc\n' > "$fixture/dir with space/b.go"
if run 1 "file in a path with spaces is flagged"; then
  case $LAST_OUT in
    *'FAIL: 1 file(s)'*) pass "space in path yields exactly 1 offender" ;;
    *) fail "expected 1 offender for spaced path, got: $LAST_OUT" ;;
  esac
fi
rm -rf "$fixture/dir with space"

echo ""
if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures assertion(s) failed in $(basename "$0")."
  exit 1
fi
echo "OK: check-license-headers.sh behaves correctly on both the pass and fail paths."
