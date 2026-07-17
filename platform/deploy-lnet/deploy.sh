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

log() { echo "[deploy] $*"; }

log "=== scenario-$SCENARIO / target=$TARGET ==="

# 1) Render manifests — required before apply, but skipped for hub: operators
# run `./deploy.sh render` (or `./render.sh`) explicitly first on the hub VM.
if [[ "$TARGET" == "hub" ]]; then
  log "Step 1/5 — skipping manifest render (run ./deploy.sh render before hub apply)"
  if [[ ! -f "$HERE/hub/manifests/hub.yaml" ]]; then
    echo "[deploy] ERROR: $HERE/hub/manifests/hub.yaml not found — run ./deploy.sh render first" >&2
    exit 2
  fi
else
  log "Step 1/5 — rendering manifests (*.yaml.tmpl → *.yaml)"
  "$HERE/render.sh"
fi

# 1b) Build the platform launcher image once per host — only for targets that
# enable it. The hub has no launcher; every spoke/bank manifest sets
# `launcher: enable` and soft-fails start-launcher if the image is missing.
if [[ "$TARGET" == "hub" ]]; then
  log "Step 2/5 — skipping launcher image (hub has no launcher)"
else
  log "Step 2/5 — checking launcher image cbweb3/launcher:local"
  if ! docker image inspect cbweb3/launcher:local >/dev/null 2>&1; then
    log "launcher image not found — building it now"
    "$ROOT/launcher/build.sh"
  else
    log "launcher image already present — skipping build"
  fi
fi

# 1c) Contract deps (Soldeer) + forge build. `dependencies/` and `out/` are
# gitignored, so a fresh checkout has neither. Soldeer must run before forge
# build (forge-std + OpenZeppelin). Scenario A's deploy/onboard-registry steps
# read compiled artifacts off disk; Scenario B's toolkit also compiles in-band,
# so a successful pre-build here is a cached no-op later.
contracts_dir="$ROOT/scenario-$SCENARIO/contracts"
log "Step 3/5 — preparing scenario-$SCENARIO contracts ($contracts_dir)"
if [[ -d "$contracts_dir" ]]; then
  if [[ ! -d "$contracts_dir/dependencies" ]]; then
    log "installing Solidity dependencies via Soldeer (forge soldeer install)…"
    ( cd "$contracts_dir" && forge soldeer install )
    log "Soldeer dependencies installed under $contracts_dir/dependencies"
  else
    log "Soldeer dependencies already present — skipping install"
  fi
  if [[ "$TARGET" == "hub" ]]; then
    # Hub only needs CBWeb3Hub.s.sol + its transitive imports (not spoke/tests).
    # Always forge clean so deploy does not reuse stale out/ cache.
    hub_artifact="$contracts_dir/out/CBWeb3Hub.s.sol/DeployCBWeb3Hub.json"
    log "cleaning forge artifacts (forge clean)…"
    ( cd "$contracts_dir" && forge clean )
    log "compiling hub contracts only (forge build script/CBWeb3Hub.s.sol)…"
    ( cd "$contracts_dir" && forge build script/CBWeb3Hub.s.sol )
    log "hub contract build finished — $hub_artifact"
  elif ! find "$contracts_dir/out" -type f -name '*.json' 2>/dev/null | grep -q .; then
    log "compiling scenario-$SCENARIO contracts (forge build; out/ empty or missing)…"
    ( cd "$contracts_dir" && forge build )
    log "contract build finished — artifacts in $contracts_dir/out"
  else
    log "compiled artifacts already present in out/ — skipping forge build"
  fi
else
  log "WARNING: contracts directory not found at $contracts_dir — skipping contract setup"
fi

# 2) Build the scenario's toolkit binary (go caches; fast on repeat).
mkdir -p "$HERE/.bin"
export BESU_NAT_PROFILE=NONE   # routable enode advertisement (required by Scenario A, harmless for B)

log "Step 4/5 — building toolkit binary"
case "$SCENARIO" in
  a)
    man="$HERE/scenario-a/manifests/${TARGET}.yaml"
    [[ -f "$man" ]] || { echo "unknown scenario-a target: $TARGET" >&2; exit 2; }
    log "building cbweb3 → $HERE/.bin/cbweb3"
    ( cd "$ROOT/scenario-a/toolkit" && go build -o "$HERE/.bin/cbweb3" ./cmd/cbweb3 )
    log "Step 5/5 — applying Scenario A manifest: $man"
    # cbweb3 locates its templates by walking up from cwd -> run from scenario-a/.
    ( cd "$ROOT/scenario-a" && "$HERE/.bin/cbweb3" apply -f "$man" "${EXTRA[@]}" )
    log "Scenario A apply finished for target=$TARGET"
    ;;
  b)
    log "building cbweb3b → $HERE/.bin/cbweb3b"
    ( cd "$ROOT/scenario-b/toolkit" && go build -o "$HERE/.bin/cbweb3b" ./cmd/cbweb3b )
    hubrpc=()
    outdir=()
    if [[ "$TARGET" == "hub" ]]; then
      man="$HERE/hub/manifests/hub.yaml"
      # Manifests under hub/manifests/; dataDir + bundle under bundles/hub/.
      # EmitHub writes <outDir>/bundles/hub.bundle.yaml → out-dir=deploy-lnet then
      # relocate into bundles/hub/ (hubBundleRef: ../../bundles/hub/hub.bundle.yaml).
      hub_dir="$HERE/bundles/hub"
      mkdir -p "$hub_dir"
      outdir=(--data-dir "$hub_dir" --out-dir "$HERE")
      log "Step 5/5 — applying Scenario B found-hub (hub contracts + stack)"
      log "manifest=$man  data-dir=$hub_dir  (bundle → bundles/hub/hub.bundle.yaml)"
    else
      man="$HERE/scenario-b/manifests/${TARGET}.yaml"
      [[ -f "$man" ]] || { echo "unknown scenario-b target: $TARGET" >&2; exit 2; }
      # found-spoke needs the hub RPC readiness gate pointed at the real hub VM.
      case "$TARGET" in
        cb-brazil|cb-colombia) hubrpc=(--hub-rpc "http://${IP_HUB}:8845") ;;
      esac
      log "Step 5/5 — applying Scenario B manifest: $man"
    fi
    "$HERE/.bin/cbweb3b" apply -f "$man" --repo-root "$ROOT" "${hubrpc[@]}" "${outdir[@]}" "${EXTRA[@]}"
    if [[ "$TARGET" == "hub" && -f "$HERE/bundles/hub.bundle.yaml" ]]; then
      mv -f "$HERE/bundles/hub.bundle.yaml" "$HERE/bundles/hub/hub.bundle.yaml"
      log "hub bundle relocated → $HERE/bundles/hub/hub.bundle.yaml"
    fi
    log "Scenario B apply finished for target=$TARGET"
    ;;
  *)
    usage; exit 2 ;;
esac

log "=== done: scenario-$SCENARIO / target=$TARGET ==="
