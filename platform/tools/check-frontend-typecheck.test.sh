#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# check-frontend-typecheck.test.sh — regression test for the frontend type-check gate.
#
# The gate this guards was previously `tsc --noEmit`, which type-checks *nothing* in
# these apps: each app's root tsconfig is `{"files": [], "references": [...]}`, so
# without walking the project references the compilation unit is empty and the command
# exits 0 whatever the code says. A real type error survived it and reached a
# review-ready PR. The scripts now run `tsc -b`; this test pins the failure path so the
# gate cannot silently regress to reporting success without checking.
#
# Usage: bash tools/check-frontend-typecheck.test.sh <scenario-a|scenario-b> [app]
# Exit:  0 = the type-check correctly rejected a type error, 1 = it did not.
set -uo pipefail

SCENARIO="${1:-}"
APP="${2:-governance}"

case "$SCENARIO" in
  scenario-a | scenario-b) ;;
  *)
    echo "usage: bash tools/check-frontend-typecheck.test.sh <scenario-a|scenario-b> [app]" >&2
    exit 2
    ;;
esac

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND="$REPO_ROOT/$SCENARIO/frontend"
PROBE="$FRONTEND/apps/$APP/src/__typecheck_probe__.ts"

if [ ! -f "$FRONTEND/apps/$APP/package.json" ]; then
  echo "FAIL — no such app: $SCENARIO/frontend/apps/$APP" >&2
  exit 1
fi

# The probe is a file the gate must reject. It is removed on every exit path,
# including an interrupt, so a failed run never leaves the tree uncompilable.
cleanup() { rm -f "$PROBE"; return 0; }
trap cleanup EXIT

cat > "$PROBE" <<'PROBE'
// SPDX-License-Identifier: Apache-2.0
// Temporary fixture written by tools/check-frontend-typecheck.test.sh.
// If this file is present in a commit, the self-test did not clean up after itself.
export const typecheckProbe: number = "not a number";
PROBE

cd "$FRONTEND" || exit 1
if npm run type-check --workspace="$APP" >/dev/null 2>&1; then
  echo "FAIL — type-check passed with a deliberate type error in $SCENARIO/$APP."
  echo "       The gate is not checking anything. Confirm the app's type-check script"
  echo "       runs 'tsc -b' and not 'tsc --noEmit'."
  exit 1
fi

echo "ok   — type-check rejects a type error in $SCENARIO/$APP"
