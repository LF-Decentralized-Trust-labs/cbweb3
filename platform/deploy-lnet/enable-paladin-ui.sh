#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Enable Paladin's bundled web UI on an already-deployed host — no redeploy.
#
# Paladin serves its React UI from /app/ui inside the image, but only when the
# RPC HTTP listener is given a static-file server. Configs rendered before that
# block existed answer JSON-RPC only, so http://<host>:<rpc-port>/ui returns a
# bare "404 page not found". This script patches config.yaml in place on the
# named volume each Paladin container mounts at /etc/paladin, then restarts the
# container so it re-reads the config. Chain state, keystore and the Paladin DB
# live on separate volumes and are never touched.
#
# The same block is now in the config templates
# (scenario-a/provisioning/templates/**/paladin-config/*/config.yaml.tmpl), so
# hosts provisioned from scratch get the UI without this script. Existing hosts
# need it because step_render_configs skips itself when config.yaml is already
# present on the volume — a re-apply will not rewrite it.
#
# THE RESTART IS THE RISKY PART, NOT THE EDIT. Paladin queries eth_chainId at
# startup and, after ~10 retries, exits rc=1 ("PD010003: Error starting ethereum
# client") if its Besu node is unreachable. The compose policy is
# `restart: unless-stopped`, so a Paladin restarted while its chain is down goes
# into a crash loop instead of coming back. A Paladin that is ALREADY running
# tolerates losing the chain, which is why a degraded node can look healthy until
# you bounce it. Each target is therefore gated on a live eth_chainId probe of
# the very URL in its own config (blockchain.http.url), reached the same way the
# container reaches it. --force skips the gate; nothing else does.
#
# Usage:
#   ./enable-paladin-ui.sh                     # every running paladin-* container on this host
#   ./enable-paladin-ui.sh paladin-spoke-costa-rica-cb [more...]
#   ./enable-paladin-ui.sh --dry-run           # report only: probe the chain, touch nothing
#   ./enable-paladin-ui.sh --force <name>      # patch + restart even if the chain probe fails
#
# Idempotent: a config that already declares staticServers is left alone.
#
# Requires on the host: docker, curl, and the ability to pull alpine:3.23 (or to
# have it cached) — the config volume is read and written through short-lived
# alpine containers, since the volume is not reachable from the host filesystem.
# On a host with a restricted registry every such `docker run` fails and the
# script reports the failure instead of changing anything.
set -euo pipefail

DRY_RUN=false
FORCE=false
TARGETS=()
for arg in "$@"; do
  case "$arg" in
    --dry-run|-n) DRY_RUN=true ;;
    --force) FORCE=true ;;
    -h|--help) sed -n '4,36p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "unknown flag: $arg" >&2; exit 2 ;;
    *) TARGETS+=("$arg") ;;
  esac
done

log() { echo "[paladin-ui] $*"; }

# ${arr[*]-} rather than ${#arr[@]}: the length operator on an unset/empty array
# is an "unbound variable" error under `set -u` before bash 4.4, and macOS still
# ships bash 3.2 — someone running --dry-run from a laptop would hit it.
if [[ -z "${TARGETS[*]-}" ]]; then
  while IFS= read -r name; do [[ -n "$name" ]] && TARGETS+=("$name"); done \
    < <(docker ps --filter 'name=^paladin-' --format '{{.Names}}' | sort)
fi

if [[ -z "${TARGETS[*]-}" ]]; then
  echo "[paladin-ui] ERROR: no running paladin-* container on this host — pass a container name explicitly" >&2
  exit 1
fi

# awk patch: insert the staticServers mapping directly under `rpcServer.http`.
# Inserting right after the `http:` key (rather than after its last scalar) keeps
# the edit anchored on a single line — mapping key order is irrelevant to YAML.
# Exit 1 if the anchor was never found, so a surprising config fails loudly
# instead of being rewritten unchanged.
AWK_PATCH="$(mktemp)"
# Second awk: print blockchain.http.url, so the chain probe targets exactly what
# this node dials rather than an assumed port.
AWK_CHAINURL="$(mktemp)"
trap 'rm -f "$AWK_PATCH" "$AWK_CHAINURL"' EXIT

cat > "$AWK_PATCH" <<'AWK'
/^rpcServer:[[:space:]]*$/ { in_rpc = 1; print; next }
in_rpc && /^[^[:space:]]/  { in_rpc = 0 }
in_rpc && !done && /^  http:[[:space:]]*$/ {
  print
  print "    staticServers:"
  print "      - enabled: true"
  print "        staticPath: /app/ui"
  print "        urlPath: /ui"
  print "        baseRedirect: ui/activity"
  done = 1
  next
}
{ print }
END { exit(done ? 0 : 1) }
AWK

cat > "$AWK_CHAINURL" <<'AWK'
/^blockchain:[[:space:]]*$/ { in_bc = 1; next }
in_bc && /^[^[:space:]]/    { in_bc = 0 }
in_bc && /^  http:[[:space:]]*$/ { in_http = 1; next }
in_bc && /^  [^[:space:]]/  { in_http = 0 }
in_bc && in_http && /^    url:/ {
  sub(/^[[:space:]]*url:[[:space:]]*/, "")
  gsub(/["']/, "")
  print
  exit 0
}
AWK

patched=0
skipped=0
failed=0
blocked=0

for ctr in "${TARGETS[@]}"; do
  log "=== $ctr"

  vol="$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/etc/paladin"}}{{.Name}}{{end}}{{end}}' "$ctr" 2>/dev/null || true)"
  if [[ -z "$vol" ]]; then
    log "SKIP — no named volume mounted at /etc/paladin (unexpected for a cbweb3 Paladin node)"
    failed=$((failed + 1))
    continue
  fi
  log "config volume: $vol"

  if docker run --rm --user 0:0 -v "$vol":/cfg alpine:3.23 \
       grep -q 'staticServers' /cfg/config.yaml 2>/dev/null; then
    log "already declares staticServers — nothing to do"
    skipped=$((skipped + 1))
    continue
  fi

  # --- gate: is this node's chain reachable? ---------------------------------
  # Probed from a container with the same host-gateway alias the Paladin service
  # gets (extra_hosts), so "host.docker.internal:<port>" resolves as it does for
  # the real node. A node whose chain is down must not be restarted.
  chain_url="$(docker run --rm --user 0:0 -v "$vol":/cfg -v "$AWK_CHAINURL":/url.awk:ro \
                 alpine:3.23 awk -f /url.awk /cfg/config.yaml 2>/dev/null || true)"
  if [[ -z "$chain_url" ]]; then
    log "WARNING — could not read blockchain.http.url from config.yaml; treating the chain as unverified"
  else
    log "chain probe: eth_chainId -> $chain_url"
    chain_out="$(docker run --rm --add-host host.docker.internal:host-gateway alpine:3.23 \
                   wget -q -T 6 -O- --header 'Content-Type: application/json' \
                   --post-data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' \
                   "$chain_url" 2>/dev/null || true)"
    if [[ "$chain_out" == *'"result"'* ]]; then
      log "chain OK — $chain_out"
    else
      log "BLOCKED — $chain_url did not answer eth_chainId."
      log "  Restarting $ctr now would very likely crash-loop it: Paladin exits rc=1 when the"
      log "  chain is unreachable at startup, and the compose restart policy keeps retrying."
      log "  Bring this spoke's Besu node back first, then re-run. Override with --force."
      if [[ "$FORCE" != true ]]; then
        blocked=$((blocked + 1))
        continue
      fi
      log "  --force given — proceeding anyway"
    fi
  fi

  if [[ "$DRY_RUN" == true ]]; then
    log "DRY-RUN — would patch config.yaml on $vol and restart $ctr"
    continue
  fi

  # Backup, then install the patched config with an atomic rename inside the
  # volume. Mode/owner are set explicitly (root:root 0644, matching
  # writeVolumeFile) because the container reads config.yaml as uid 1000.
  if ! docker run --rm --user 0:0 -v "$vol":/cfg -v "$AWK_PATCH":/patch.awk:ro alpine:3.23 sh -c '
        set -e
        [ -f /cfg/config.yaml.pre-ui.bak ] || cp -p /cfg/config.yaml /cfg/config.yaml.pre-ui.bak
        awk -f /patch.awk /cfg/config.yaml > /cfg/.config.yaml.new || {
          rm -f /cfg/.config.yaml.new
          exit 1
        }
        chown 0:0 /cfg/.config.yaml.new
        chmod 0644 /cfg/.config.yaml.new
        mv /cfg/.config.yaml.new /cfg/config.yaml
      '; then
    log "ERROR — patch failed (rpcServer.http anchor not found?); config.yaml left as-is"
    failed=$((failed + 1))
    continue
  fi
  log "config.yaml patched (backup: config.yaml.pre-ui.bak on the same volume)"

  log "restarting $ctr"
  docker restart "$ctr" >/dev/null

  # Verify through the published host port rather than assuming it.
  hostport="$(docker port "$ctr" 8548/tcp 2>/dev/null | head -1 | sed 's/.*://')"
  if [[ -z "$hostport" ]]; then
    log "WARNING — container port 8548 is not published; cannot verify from the host"
    patched=$((patched + 1))
    continue
  fi

  code=000
  for _ in $(seq 1 30); do
    code="$(curl -s -m 3 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${hostport}/ui" || true)"
    [[ "$code" == "302" ]] && break
    sleep 3
  done
  if [[ "$code" == "302" ]]; then
    log "OK — /ui redirects to /ui/activity on host port $hostport"
    patched=$((patched + 1))
  else
    log "WARNING — /ui returned $code after 90s; the node may not have come back up."
    log "  Inspect:  docker logs --tail 50 $ctr"
    log "  Roll back: docker run --rm --user 0:0 -v $vol:/cfg alpine:3.23 \\"
    log "               cp /cfg/config.yaml.pre-ui.bak /cfg/config.yaml && docker restart $ctr"
    failed=$((failed + 1))
  fi
done

log "=== done: $patched patched, $skipped already enabled, $blocked blocked (chain down), $failed need attention"
# Both modes share this status. A --dry-run that counted a blocked target (chain
# down) or a failed one (no named volume at /etc/paladin) exits non-zero, so a
# wrapper iterating over hosts can detect it without parsing the log.
[[ $failed -eq 0 && $blocked -eq 0 ]]
