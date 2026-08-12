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
# Usage:
#   ./enable-paladin-ui.sh                     # every running paladin-* container on this host
#   ./enable-paladin-ui.sh paladin-spoke-costa-rica-cb [more...]
#   ./enable-paladin-ui.sh --dry-run           # show what would change, touch nothing
#
# Idempotent: a config that already declares staticServers is left alone.
set -euo pipefail

DRY_RUN=false
TARGETS=()
for arg in "$@"; do
  case "$arg" in
    --dry-run|-n) DRY_RUN=true ;;
    -h|--help) sed -n '4,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "unknown flag: $arg" >&2; exit 2 ;;
    *) TARGETS+=("$arg") ;;
  esac
done

log() { echo "[paladin-ui] $*"; }

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  while IFS= read -r name; do [[ -n "$name" ]] && TARGETS+=("$name"); done \
    < <(docker ps --filter 'name=^paladin-' --format '{{.Names}}' | sort)
fi

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  echo "[paladin-ui] ERROR: no running paladin-* container on this host — pass a container name explicitly" >&2
  exit 1
fi

# awk patch: insert the staticServers mapping directly under `rpcServer.http`.
# Inserting right after the `http:` key (rather than after its last scalar) keeps
# the edit anchored on a single line — mapping key order is irrelevant to YAML.
# Exit 1 if the anchor was never found, so a surprising config fails loudly
# instead of being rewritten unchanged.
AWK_PATCH="$(mktemp)"
trap 'rm -f "$AWK_PATCH"' EXIT
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

patched=0
skipped=0
failed=0

for ctr in "${TARGETS[@]}"; do
  log "=== $ctr"

  vol="$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/etc/paladin"}}{{.Name}}{{end}}{{end}}' "$ctr" 2>/dev/null || true)"
  if [[ -z "$vol" ]]; then
    log "SKIP — no named volume mounted at /etc/paladin (unexpected for a cbweb3 Paladin node)"
    failed=$((failed + 1))
    continue
  fi
  log "config volume: $vol"

  if docker run --rm --user 0:0 -v "$vol":/cfg alpine:3.20 \
       grep -q 'staticServers' /cfg/config.yaml 2>/dev/null; then
    log "already declares staticServers — nothing to do"
    skipped=$((skipped + 1))
    continue
  fi

  if [[ "$DRY_RUN" == true ]]; then
    log "DRY-RUN — would patch config.yaml on $vol and restart $ctr"
    continue
  fi

  # Backup first, then patch. cp onto the existing config.yaml keeps that file's
  # ownership/mode (root:root 0644 as seeded by writeVolumeFile), which the
  # container's uid 1000 needs to read it.
  if ! docker run --rm --user 0:0 -v "$vol":/cfg -v "$AWK_PATCH":/patch.awk:ro alpine:3.20 sh -c '
        set -e
        [ -f /cfg/config.yaml.pre-ui.bak ] || cp -p /cfg/config.yaml /cfg/config.yaml.pre-ui.bak
        awk -f /patch.awk /cfg/config.yaml > /tmp/config.yaml
        cp /tmp/config.yaml /cfg/config.yaml
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
    log "WARNING — /ui returned $code after restart; check: docker logs --tail 50 $ctr"
    failed=$((failed + 1))
  fi
done

log "=== done: $patched patched, $skipped already enabled, $failed need attention"
[[ "$DRY_RUN" == true ]] && exit 0
[[ $failed -eq 0 ]]
