#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-frontend-tests.test.sh — regression test for the frontend test gate.
#
# The gate this guards did not exist until the ISO 20022 amount work added it. CI
# type-checked and built every app, and built every portal image, but never ran a
# single frontend test — so every suite in the repository, including the ones
# written to pin the amount rule, was unexecuted. A regression that keeps the types
# valid (loosening the amount regex, dropping the comma refusal, turning truncation
# into rounding) passed CI green.
#
# Two ways that gate could silently stop guarding, both pinned here:
#   * the step runs but the suite exits 0 whatever the tests say;
#   * an app's `test` script is `vitest` rather than `vitest run`, which watches
#     instead of running once. Vitest happens to run once when CI=true, so this
#     regresses invisibly on a developer's machine and depends on an env var in CI.
#
# Usage: bash tools/check-frontend-tests.test.sh <scenario-a|scenario-b> [app]
# Exit:  0 = the gate correctly rejected a failing test, 1 = it did not.
set -uo pipefail

SCENARIO="${1:-}"
APP="${2:-bank}"

case "$SCENARIO" in
  scenario-a | scenario-b) ;;
  *)
    echo "usage: bash tools/check-frontend-tests.test.sh <scenario-a|scenario-b> [app]" >&2
    exit 2
    ;;
esac

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND="$REPO_ROOT/$SCENARIO/frontend"
PROBE="$FRONTEND/apps/$APP/src/__test_gate_probe__.test.ts"

if [ ! -f "$FRONTEND/apps/$APP/package.json" ]; then
  echo "FAIL — no such app: $SCENARIO/frontend/apps/$APP" >&2
  exit 1
fi

# Every app must run its suite once and exit, not watch. `vitest` alone watches
# unless CI is set; a gate that depends on an environment variable to terminate is
# not a gate. Checked before the probe, because a watching suite would hang here.
TEST_SCRIPT=$(node -e "process.stdout.write(require('$FRONTEND/apps/$APP/package.json').scripts.test || '')")
case "$TEST_SCRIPT" in
  *"vitest run"*) ;;
  *)
    echo "FAIL — $SCENARIO/$APP has test script '$TEST_SCRIPT'."
    echo "       It must be 'vitest run'; bare 'vitest' watches unless CI is set."
    exit 1
    ;;
esac

# The probe is a test the gate must fail on. It is removed on every exit path,
# including an interrupt, so a failed run never leaves a red suite behind.
cleanup() { rm -f "$PROBE"; return 0; }
trap cleanup EXIT

cat > "$PROBE" <<'PROBE'
// SPDX-License-Identifier: Apache-2.0
// Temporary fixture written by tools/check-frontend-tests.test.sh.
// If this file is present in a commit, the self-test did not clean up after itself.
import { describe, expect, it } from "vitest";

describe("test gate probe", () => {
  it("must fail so the gate can prove it reports failure", () => {
    expect(1).toBe(2);
  });
});
PROBE

cd "$FRONTEND" || exit 1
if npm run test --workspace="$APP" >/dev/null 2>&1; then
  echo "FAIL — the suite passed with a deliberately failing test in $SCENARIO/$APP."
  echo "       The gate is not running the tests, or is swallowing their exit code."
  exit 1
fi

echo "ok   — the test gate rejects a failing test in $SCENARIO/$APP"
