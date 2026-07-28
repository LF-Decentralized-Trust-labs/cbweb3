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
#              | noc-hub (scenario b only) | noc-brazil | noc-colombia (NOC — both scenarios)
#
# Examples (run each on its own VM):
#   ./deploy.sh cacti                 # VM .20  — start both Cacti relays
#   ./deploy.sh b hub                 # VM .20  — Scenario B found-hub
#   ./deploy.sh b noc-hub             # VM .20  — Scenario B NOC portal (after b hub)
#   ./deploy.sh b cb-brazil           # VM .21  — Scenario B found-spoke (adds --hub-rpc)
#   ./deploy.sh b noc-brazil          # VM .21  — Scenario B NOC portal (after b cb-brazil)
#   ./deploy.sh b cb1 --dry-run       # VM .22  — Scenario B join, preview only
#   ./deploy.sh a cb-brazil           # VM .21  — Scenario A found
#   ./deploy.sh a noc-brazil          # VM .21  — Scenario A NOC data plane (after a cb-brazil)
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
  # The liquidity relay reads isPaused() on-chain (circuit breaker) and mints/burns
  # via HUB_BESU_RPC. On the LNET hub VM the hub Besu is published on 8845 (the
  # compose default 33845 is the single-host +25000 sample). Wrong host/port here →
  # the breaker read fails and every bridge-out is refused with a 409 (fail-safe).
  COMPOSE_PROJECT_NAME=cbweb3-cacti-b CACTI_PORT="${CACTI_PORT_B:-7000}" \
    HUB_BESU_RPC="${HUB_BESU_RPC:-http://host.docker.internal:8845}" \
    HUB_BESU_WS="${HUB_BESU_WS:-ws://host.docker.internal:8846}" \
    "$ROOT/scenario-b/provisioning/scripts/start-cacti.sh" "${downflag[@]}"
  exit 0
fi

TARGET="${2:-}"
if [[ -z "$SCENARIO" || -z "$TARGET" ]]; then usage; exit 2; fi
shift 2
EXTRA=("$@")

# NOC observe targets (noc-hub / noc-brazil / noc-colombia) deploy the NOC control
# plane (portal + backend). Unlike found/join they build no contracts, run no
# launcher, and emit/relocate no bundle — they only consume a NOC bundle.
IS_NOC=false
case "$TARGET" in noc-*) IS_NOC=true ;; esac

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
if [[ "$TARGET" == "hub" || "$IS_NOC" == true ]]; then
  log "Step 2/5 — skipping launcher image (hub/NOC has no launcher)"
else
  log "Step 2/5 — checking launcher image cbweb3/launcher:local"
  if ! docker image inspect cbweb3/launcher:local >/dev/null 2>&1; then
    log "launcher image not found — building it now"
    "$ROOT/launcher/build.sh"
  else
    log "launcher image already present — skipping build"
  fi
fi

# 1b') Reverse-proxy image (Caddy): the per-host :80 entrypoint that fronts this
# entity's portals + api-gateway by path. Built once per host (every target, hub
# included). Only used when a manifest sets `proxy: enable`; the proxy step
# soft-fails if the image is missing, so an unbuilt image never blocks a deploy.
log "Step 2b/5 — checking proxy image cbweb3/proxy:local"
if ! docker image inspect cbweb3/proxy:local >/dev/null 2>&1; then
  log "proxy image not found — building it now"
  "$ROOT/proxy/build.sh"
else
  log "proxy image already present — skipping build"
fi

# 1c) Contract deps (Soldeer) + forge build. `dependencies/` and `out/` are
# gitignored, so a fresh checkout has neither. Soldeer must run before forge
# build (forge-std + OpenZeppelin). Scenario A's deploy/onboard-registry steps
# read compiled artifacts off disk; Scenario B's toolkit also compiles in-band,
# so a successful pre-build here is a cached no-op later.
if [[ "$IS_NOC" == true ]]; then
  log "Step 3/5 — skipping contracts (observe/NOC deploys no on-chain node)"
else
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
fi

# 2) Build the scenario's toolkit binary (go caches; fast on repeat).
mkdir -p "$HERE/.bin"
export BESU_NAT_PROFILE=NONE   # routable enode advertisement (required by Scenario A, harmless for B)

# Map a spoke/bank target to its spoke id (join bundle drop-zone under
# bundles/<scenario>/<spoke>/). Commercial banks still join that spoke, but their
# writable dataDir is per-bank (see entity_dir_for) so state never mixes with the CB.
spoke_id_for() {
  case "$1" in
    cb-brazil|cb1|cb2) echo "spoke-brazil" ;;
    cb-colombia|cb3|cb4) echo "spoke-colombia" ;;
    *) echo "" ;;
  esac
}

# Writable dataDir folder name under bundles/scenario-{a,b}/:
#   founders (cb-brazil, cb-colombia) → spoke id (also holds <spoke>.bundle.yaml)
#   everything else (commercial banks) → target name (= bankId), so state is isolated
entity_dir_for() {
  case "$1" in
    cb-brazil|cb-colombia) spoke_id_for "$1" ;;
    *) echo "$1" ;;
  esac
}

log "Step 4/5 — building toolkit binary"
case "$SCENARIO" in
  a)
    man="$HERE/scenario-a/manifests/${TARGET}.yaml"
    [[ -f "$man" ]] || { echo "unknown scenario-a target: $TARGET" >&2; exit 2; }
    log "building cbweb3 → $HERE/.bin/cbweb3"
    ( cd "$ROOT/scenario-a/toolkit" && go build -o "$HERE/.bin/cbweb3" ./cmd/cbweb3 )
    if [[ "$IS_NOC" == true ]]; then
      # observe (NOC data plane): no on-chain node, no spoke_id/bundle relocation.
      # State lives under bundles/scenario-a/<target> so it never collides with a
      # same-named Scenario B NOC. The nocBundleRef resolves against the manifest's
      # own dir (central-bank-brazil's found relocated the NOC bundle there).
      data_dir="$HERE/bundles/scenario-a/$TARGET"
      mkdir -p "$data_dir"
      log "Step 5/5 — applying Scenario A observe (NOC data plane): $man"
      log "data-dir=$data_dir"
      # cbweb3 locates templates + repo root by walking up from cwd -> run from
      # scenario-a/. CBWEB3_OUTPUT_DIR is unused by observe (no bundle emit).
      ( cd "$ROOT/scenario-a" && \
          CBWEB3_OUTPUT_DIR="$HERE" \
          "$HERE/.bin/cbweb3" apply -f "$man" "${EXTRA[@]}" )
      log "Scenario A apply finished for target=$TARGET"
    else
    spoke_id="$(spoke_id_for "$TARGET")"
    entity_dir="$(entity_dir_for "$TARGET")"
    [[ -n "$spoke_id" && -n "$entity_dir" ]] || { echo "unknown scenario-a target: $TARGET" >&2; exit 2; }
    # dataDir = bundles/scenario-a/<spoke|bank>/ (writable).
    # Founders emit <outDir>/bundles/<spoke>.bundle.yaml; CBWEB3_OUTPUT_DIR=$HERE
    # then relocate into the spoke folder (not the bank folder).
    data_dir="$HERE/bundles/scenario-a/$entity_dir"
    mkdir -p "$data_dir"
    # Banks still need the spoke join bundle on disk (joinBundleRef); ensure the
    # drop-zone exists even when dataDir is the per-bank folder.
    mkdir -p "$HERE/bundles/scenario-a/$spoke_id"
    log "Step 5/5 — applying Scenario A manifest: $man"
    log "data-dir=$data_dir  (join bundle drop-zone → bundles/scenario-a/$spoke_id/$spoke_id.bundle.yaml)"
    # cbweb3 locates its templates by walking up from cwd -> run from scenario-a/.
    # Manifest dataDir is relative to scenario-a/; CBWEB3_OUTPUT_DIR forces emit under deploy-lnet/.
    ( cd "$ROOT/scenario-a" && \
        CBWEB3_OUTPUT_DIR="$HERE" \
        "$HERE/.bin/cbweb3" apply -f "$man" "${EXTRA[@]}" )
    if [[ "$TARGET" == "cb-brazil" || "$TARGET" == "cb-colombia" ]]; then
      src="$HERE/bundles/${spoke_id}.bundle.yaml"
      dst="$HERE/bundles/scenario-a/$spoke_id/${spoke_id}.bundle.yaml"
      if [[ -f "$src" ]]; then
        mv -f "$src" "$dst"
        log "spoke bundle relocated → $dst"
      fi
      # NOC bundle rides along into the scenario-a spoke folder (scenario-scoped,
      # so a Scenario B spoke-<x>.noc.bundle.yaml never overwrites it). observe
      # (a noc-<x> target) consumes it from there via nocBundleRef.
      noc_src="$HERE/bundles/${spoke_id}.noc.bundle.yaml"
      noc_dst="$HERE/bundles/scenario-a/$spoke_id/${spoke_id}.noc.bundle.yaml"
      if [[ -f "$noc_src" ]]; then
        mv -f "$noc_src" "$noc_dst"
        log "spoke NOC bundle relocated → $noc_dst"
      fi
    fi
    log "Scenario A apply finished for target=$TARGET"
    fi
    ;;
  b)
    log "building cbweb3b → $HERE/.bin/cbweb3b"
    ( cd "$ROOT/scenario-b/toolkit" && go build -o "$HERE/.bin/cbweb3b" ./cmd/cbweb3b )
    if [[ "$IS_NOC" == true ]]; then
      # observe (NOC control plane): own state dir, no --out-dir/bundle relocation,
      # no --hub-rpc. State lives under bundles/scenario-b/ so it never collides
      # with a same-named Scenario A NOC. The nocBundleRef resolves against the
      # manifest's own dir (the found-* apply relocated the NOC bundle there).
      man="$HERE/scenario-b/manifests/${TARGET}.yaml"
      [[ -f "$man" ]] || { echo "unknown scenario-b target: $TARGET" >&2; exit 2; }
      noc_dir="$HERE/bundles/scenario-b/${TARGET}"
      mkdir -p "$noc_dir"
      log "Step 5/5 — applying Scenario B observe (NOC portal+backend): $man"
      log "data-dir=$noc_dir"
      "$HERE/.bin/cbweb3b" apply -f "$man" --repo-root "$ROOT" --data-dir "$noc_dir" "${EXTRA[@]}"
      log "Scenario B apply finished for target=$TARGET"
    else
    hubrpc=()
    outdir=()
    spoke_id=""
    entity_dir=""
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
      spoke_id="$(spoke_id_for "$TARGET")"
      entity_dir="$(entity_dir_for "$TARGET")"
      [[ -n "$spoke_id" && -n "$entity_dir" ]] || { echo "unknown scenario-b target: $TARGET" >&2; exit 2; }
      # dataDir = bundles/scenario-b/<spoke|bank>/; EmitSpoke writes
      # <outDir>/bundles/<spoke>.bundle.yaml → relocate into the spoke folder.
      data_dir="$HERE/bundles/scenario-b/$entity_dir"
      mkdir -p "$data_dir"
      mkdir -p "$HERE/bundles/scenario-b/$spoke_id"
      outdir=(--data-dir "$data_dir" --out-dir "$HERE")
      # found-spoke needs the hub RPC readiness gate pointed at the real hub VM.
      case "$TARGET" in
        cb-brazil|cb-colombia) hubrpc=(--hub-rpc "http://${IP_HUB}:8845") ;;
      esac
      log "Step 5/5 — applying Scenario B manifest: $man"
      log "data-dir=$data_dir  (join bundle drop-zone → bundles/scenario-b/$spoke_id/$spoke_id.bundle.yaml)"
    fi
    "$HERE/.bin/cbweb3b" apply -f "$man" --repo-root "$ROOT" "${hubrpc[@]}" "${outdir[@]}" "${EXTRA[@]}"
    if [[ "$TARGET" == "hub" && -f "$HERE/bundles/hub.bundle.yaml" ]]; then
      mv -f "$HERE/bundles/hub.bundle.yaml" "$HERE/bundles/hub/hub.bundle.yaml"
      log "hub bundle relocated → $HERE/bundles/hub/hub.bundle.yaml"
      # NOC bundle rides along into bundles/hub/ (hub is Scenario B-only).
      if [[ -f "$HERE/bundles/hub.noc.bundle.yaml" ]]; then
        mv -f "$HERE/bundles/hub.noc.bundle.yaml" "$HERE/bundles/hub/hub.noc.bundle.yaml"
        log "hub NOC bundle relocated → $HERE/bundles/hub/hub.noc.bundle.yaml"
      fi
    elif [[ -n "$spoke_id" && ( "$TARGET" == "cb-brazil" || "$TARGET" == "cb-colombia" ) ]]; then
      src="$HERE/bundles/${spoke_id}.bundle.yaml"
      dst="$HERE/bundles/scenario-b/$spoke_id/${spoke_id}.bundle.yaml"
      if [[ -f "$src" ]]; then
        mv -f "$src" "$dst"
        log "spoke bundle relocated → $dst"
      fi
      # NOC bundle rides along into the scenario-b spoke folder (scenario-scoped,
      # so a Scenario A spoke-<x>.noc.bundle.yaml never overwrites it).
      noc_src="$HERE/bundles/${spoke_id}.noc.bundle.yaml"
      noc_dst="$HERE/bundles/scenario-b/$spoke_id/${spoke_id}.noc.bundle.yaml"
      if [[ -f "$noc_src" ]]; then
        mv -f "$noc_src" "$noc_dst"
        log "spoke NOC bundle relocated → $noc_dst"
      fi
    fi
    log "Scenario B apply finished for target=$TARGET"
    fi
    ;;
  *)
    usage; exit 2 ;;
esac

log "=== done: scenario-$SCENARIO / target=$TARGET ==="
