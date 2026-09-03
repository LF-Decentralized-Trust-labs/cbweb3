#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-alpine-version.test.sh — regression test for check-alpine-version.sh.
#
# The failure mode this pins is the one that already bit the licence gate: counting
# offenders inside a pipeline, where bash runs the right-hand side in a subshell so
# the counter never escapes and the check exits 0 while printing offending lines.
# Every assertion below therefore checks the exit status *and* the reported count.
#
# It also pins the two ways this particular gate could lie: reporting success when
# no pin exists (nothing to compare against), and reporting success when the scan
# could not read part of the tree.
#
# Usage: bash tools/check-alpine-version.test.sh
# Exit:  0 = all assertions passed, 1 = at least one failed.
set -uo pipefail

CHECKER="$(cd "$(dirname "$0")" && pwd)/check-alpine-version.sh"

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

pin() {  # pin <version> — write a TOOLCHAIN.md whose enforcement row pins <version>
  mkdir -p "$fixture/docs"
  {
    echo "| Version | Enforced by | Notes |"
    echo "|---------|-------------|-------|"
    echo "| Alpine $1 | \`alpine:$1\` in every helper | pinned |"
  } > "$fixture/docs/TOOLCHAIN.md"
}

# --- 1. no pin at all must fail, not pass by default --------------------------
mkdir -p "$fixture/svc"
printf 'FROM alpine:3.23\n' > "$fixture/svc/Dockerfile"
if run 1 "a tree with no pin in docs/TOOLCHAIN.md exits 1"; then
  case $LAST_OUT in
    *"no Alpine pin found"*) pass "explains that the pin is missing" ;;
    *) fail "expected the missing-pin message, got: $LAST_OUT" ;;
  esac
  case $LAST_OUT in
    *OK:*) fail "must not print OK without a pin" ;;
    *) pass "does not print OK without a pin" ;;
  esac
fi

# --- 2. every reference on the pin passes ------------------------------------
pin 3.23
if run 0 "tree entirely on the pinned version exits 0"; then
  case $LAST_OUT in
    *"pinned Alpine (from docs/TOOLCHAIN.md): alpine:3.23"*) pass "reports the pin it read" ;;
    *) fail "expected the pin to be echoed, got: $LAST_OUT" ;;
  esac
fi

# --- 3. one stale reference fails, and is counted ----------------------------
printf 'const helper = "alpine:3.20"\n' > "$fixture/svc/vol.go"
if run 1 "a single stale reference exits 1"; then
  case $LAST_OUT in
    *"FAIL: 1 of "*) pass "counter survives the loops (reports 1)" ;;
    *) fail "expected a count of 1, got: $LAST_OUT" ;;
  esac
  case $LAST_OUT in
    *'::error file=svc/vol.go::'*) pass "emits a GitHub error annotation" ;;
    *) fail "missing ::error annotation for svc/vol.go" ;;
  esac
  case $LAST_OUT in
    *"svc/vol.go:1:"*) pass "names the file and line" ;;
    *) fail "expected file:line in the output, got: $LAST_OUT" ;;
  esac
fi

# --- 4. a second offender must not be swallowed ------------------------------
# This is the assertion that would have caught the licence gate's subshell bug:
# a broken counter typically still reports 1.
printf 'docker run --rm alpine:3.19 sh -c true\n' > "$fixture/svc/up.sh"
if run 1 "two stale references still exit 1"; then
  case $LAST_OUT in
    *"FAIL: 2 of "*) pass "counts every offender (reports 2)" ;;
    *) fail "expected a count of 2, got: $LAST_OUT" ;;
  esac
fi

# --- 5. references anywhere in a file count, not just FROM lines --------------
# The point of this gate over `grep FROM alpine:` is that Go constants, compose
# services and docker run lines all count.
rm -f "$fixture/svc/up.sh"
printf 'services:\n  helper:\n    image: alpine:3.20\n' > "$fixture/svc/compose.yml"
if run 1 "a compose service image is scanned too"; then
  case $LAST_OUT in
    *"svc/compose.yml:3:"*) pass "flags a mid-file compose reference" ;;
    *) fail "expected svc/compose.yml:3, got: $LAST_OUT" ;;
  esac
fi
rm -f "$fixture/svc/compose.yml" "$fixture/svc/vol.go"
run 0 "back to a clean tree" >/dev/null

# --- 6. documented exclusions are skipped ------------------------------------
mkdir -p "$fixture/vendor/x" "$fixture/web/node_modules" "$fixture/web/dist" \
  "$fixture/web/build" "$fixture/web/.next" "$fixture/web/.turbo"
printf 'FROM alpine:3.20\n' > "$fixture/vendor/x/Dockerfile"
for d in node_modules dist build .next .turbo; do
  printf 'FROM alpine:3.20\n' > "$fixture/web/$d/Dockerfile"
done
if run 0 "excluded paths are not scanned"; then
  case $LAST_OUT in
    *vendor*|*node_modules*) fail "an excluded path leaked into the report" ;;
    *) pass "no excluded path appears in the report" ;;
  esac
fi

# --- 7. a scan that cannot complete must not report success -------------------
# Skipped as root, where chmod 000 does not deny access.
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$fixture/locked"
  printf 'FROM alpine:3.20\n' > "$fixture/locked/Dockerfile"
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

# --- 8. paths with spaces are handled one file at a time ----------------------
mkdir -p "$fixture/dir with space"
printf 'FROM alpine:3.20\n' > "$fixture/dir with space/Dockerfile"
if run 1 "file in a path with spaces is flagged"; then
  case $LAST_OUT in
    *"FAIL: 1 of "*) pass "space in path yields exactly 1 offender" ;;
    *) fail "expected 1 offender for spaced path, got: $LAST_OUT" ;;
  esac
fi
rm -rf "$fixture/dir with space"

# --- 9. moving the pin moves the rule ----------------------------------------
# The pin is the single source of truth: with the pin on 3.20, a 3.20 tree passes
# and the 3.23 file becomes the offender.
printf 'const helper = "alpine:3.20"\n' > "$fixture/svc/vol.go"
pin 3.20
if run 1 "with the pin on 3.20, the 3.23 reference is the offender"; then
  case $LAST_OUT in
    *"svc/Dockerfile"*) pass "the pin, not a hardcoded version, decides" ;;
    *) fail "expected svc/Dockerfile to be flagged, got: $LAST_OUT" ;;
  esac
fi

# --- 10. a non-3.x tag is an offender too ------------------------------------
# The gap this closes: the parent card R2-M-12 was ABOUT `alpine:latest`, and a
# pattern matching only `alpine:3.x` reported OK with one sitting in the tree.
pin 3.23
rm -f "$fixture/svc/vol.go"
printf 'FROM alpine:latest\n' > "$fixture/svc/stale.Dockerfile"
if run 1 "alpine:latest is an offender"; then
  case $LAST_OUT in
    *"FAIL: 1 of "*) pass "counts the non-3.x offender" ;;
    *) fail "expected a count of 1, got: $LAST_OUT" ;;
  esac
fi
rm -f "$fixture/svc/stale.Dockerfile"

# --- 11. an UNtagged reference is an offender --------------------------------
# `FROM alpine` and `docker run … alpine` mean :latest by omission. Found live in
# proxy/backup-certs.sh, which the version-only pattern reported as OK.
for form in 'FROM alpine' 'docker run --rm -v x:/data alpine tar czf /b.tgz /data'; do
  printf '%s\n' "$form" > "$fixture/svc/untagged.sh"
  if run 1 "untagged reference is an offender: ${form%% *}…"; then
    case $LAST_OUT in
      *"untagged Alpine reference"*) pass "names it as untagged rather than mismatched" ;;
      *) fail "expected the untagged message, got: $LAST_OUT" ;;
    esac
  fi
done
rm -f "$fixture/svc/untagged.sh"

# --- 12. other Alpine-BASED images are not governed by this pin --------------
# Ten images in this tree carry `-alpine` as a tag suffix. Flagging them would make
# the gate unusable, so this asserts the boundary rather than trusting the regex.
{
  printf 'FROM golang:1.26-alpine AS builder\n'
  printf 'FROM node:22-alpine\n'
  printf 'image: postgres:17-alpine\n'
  printf 'image: "${REDIS_IMAGE_TAG:-7-alpine}"\n'
} > "$fixture/svc/based.Dockerfile"
run 0 "images merely BASED on Alpine (golang:1.26-alpine, node:22-alpine, …) are left alone"
rm -f "$fixture/svc/based.Dockerfile"

# --- 13. a narrated version in a comment is not a reference ------------------
# Several Dockerfiles explain in a comment why they left `alpine:latest`. Flagging
# that would punish the documentation the parent card asked for.
{
  printf '# moved off alpine:latest because the tag is a moving target\n'
  printf '// the old alpine:3.20 helper is gone\n'
  printf 'FROM alpine:3.23\n'
} > "$fixture/svc/narrated.Dockerfile"
run 0 "a version named in a comment is prose, not a reference"
rm -f "$fixture/svc/narrated.Dockerfile"

echo ""
if [ "$failures" -gt 0 ]; then
  echo "FAIL: $failures assertion(s) failed in $(basename "$0")."
  exit 1
fi
echo "OK: check-alpine-version.sh behaves correctly on both the pass and fail paths."
