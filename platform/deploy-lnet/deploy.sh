#!/usr/bin/env bash
# Render the address markers, then run the toolkit for one target host.
#
# Usage:
#   ./deploy.sh <scenario> <target> [extra toolkit flags]
#   ./deploy.sh cacti [--down]                  # VM .20: both Cacti relays (A :4000 + B :7000)
#   ./deploy.sh render                          # only render *.yaml.tmpl -> *.yaml
#
#   scenario : a | b
#   target   : hub (scenario b only) | cb-brazil | cb1 | cb2 | cb-colombia | cb3 | cb4
#
# Examples (run each on its own VM):
#   ./deploy.sh cacti                 # VM .20  — start both Cacti relays
#   ./deploy.sh b hub                 # VM .20  — Scenario B found-hub
#   ./deploy.sh b cb-brazil           # VM .21  — Scenario B found-spoke (adds --hub-rpc)
#   ./deploy.sh b cb1 --dry-run       # VM .22  — Scenario B join, preview only
#   ./deploy.sh a cb-brazil           # VM .21  — Scenario A found
#   ./deploy.sh a cb3                 # VM .25  — Scenario A join
#
# Addresses come from addresses.env (override by exporting IP_* before running).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
# shellcheck source=/dev/null
source "$HERE/addresses.env"

usage() { sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

SCENARIO="${1:-}"
if [[ "$SCENARIO" == "render" ]]; then "$HERE/render.sh"; exit 0; fi

# VM .20 shared infra: both Cacti relays. Distinct COMPOSE_PROJECT_NAME so they never
# share the default `cacti` project/network (a --down of one would drop the other's net).
if [[ "$SCENARIO" == "cacti" ]]; then
  downflag=()
  [[ "${2:-}" == "--down" || "${2:-}" == "down" ]] && downflag=(--down)
  echo "[deploy] Scenario A Cacti (HTLC relay) — host port ${CACTI_PORT_A:-4000}"
  COMPOSE_PROJECT_NAME=cbweb3-cacti-a CACTI_PORT="${CACTI_PORT_A:-4000}" \
    "$ROOT/scenario-a/provisioning/scripts/start-cacti.sh" "${downflag[@]}"
  echo "[deploy] Scenario B Cacti (liquidity relay) — host port ${CACTI_PORT_B:-7000}"
  COMPOSE_PROJECT_NAME=cbweb3-cacti-b CACTI_PORT="${CACTI_PORT_B:-7000}" \
    "$ROOT/scenario-b/provisioning/scripts/start-cacti.sh" "${downflag[@]}"
  exit 0
fi

TARGET="${2:-}"
if [[ -z "$SCENARIO" || -z "$TARGET" ]]; then usage; exit 2; fi
shift 2
EXTRA=("$@")

# 1) Always render first so this target and every bundle-ref stay consistent.
"$HERE/render.sh"

# 2) Build the scenario's toolkit binary (go caches; fast on repeat).
mkdir -p "$HERE/.bin"
export BESU_NAT_PROFILE=NONE   # routable enode advertisement (required by Scenario A, harmless for B)

case "$SCENARIO" in
  a)
    man="$HERE/scenario-a/manifests/${TARGET}.yaml"
    [[ -f "$man" ]] || { echo "unknown scenario-a target: $TARGET" >&2; exit 2; }
    ( cd "$ROOT/scenario-a/toolkit" && go build -o "$HERE/.bin/cbweb3" ./cmd/cbweb3 )
    # cbweb3 locates its templates by walking up from cwd -> run from scenario-a/.
    ( cd "$ROOT/scenario-a" && "$HERE/.bin/cbweb3" apply -f "$man" "${EXTRA[@]}" )
    ;;
  b)
    ( cd "$ROOT/scenario-b/toolkit" && go build -o "$HERE/.bin/cbweb3b" ./cmd/cbweb3b )
    hubrpc=()
    if [[ "$TARGET" == "hub" ]]; then
      man="$HERE/hub.yaml"
    else
      man="$HERE/scenario-b/manifests/${TARGET}.yaml"
      [[ -f "$man" ]] || { echo "unknown scenario-b target: $TARGET" >&2; exit 2; }
      # found-spoke needs the hub RPC readiness gate pointed at the real hub VM.
      case "$TARGET" in
        cb-brazil|cb-colombia) hubrpc=(--hub-rpc "http://${IP_HUB}:8845") ;;
      esac
    fi
    "$HERE/.bin/cbweb3b" apply -f "$man" --repo-root "$ROOT" "${hubrpc[@]}" "${EXTRA[@]}"
    ;;
  *)
    usage; exit 2 ;;
esac
