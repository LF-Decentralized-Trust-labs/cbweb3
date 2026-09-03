#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Wipe Docker state for a clean redeploy while KEEPING the reverse proxy intact.
#
# Preserves, by name:
#   - container : $PROXY_CONTAINER_NAME (default cbweb3-proxy)
#   - image     : whatever image that container currently runs
#   - volume    : $PROXY_DATA_VOLUME     (default cbweb3-proxy-data)  <- the cert store
#
# Everything else — all other containers (running or not), all other images, and
# all other volumes (named + anonymous) — is removed, so the new deploy starts
# from fresh state and old data cannot leak in. Networks and build cache can be
# pruned too with --prune-cache.
#
# Volumes are NEVER force-removed: a volume still attached to a running container
# (i.e. the proxy's cert store) is refused by Docker and skipped. That refusal is
# the safety net, not an error.
#
# Usage:
#   proxy/reset-for-redeploy.sh              # DRY RUN — list what would be removed
#   proxy/reset-for-redeploy.sh --yes        # actually remove (prompts once)
#   proxy/reset-for-redeploy.sh --yes --prune-cache   # also prune networks + builder cache
#   proxy/reset-for-redeploy.sh --yes --force         # skip the interactive prompt
#
# Overridable env:
#   PROXY_CONTAINER_NAME   (default cbweb3-proxy)
#   PROXY_DATA_VOLUME      (default cbweb3-proxy-data)
set -euo pipefail

PROXY_NAME="${PROXY_CONTAINER_NAME:-cbweb3-proxy}"
PROXY_VOL="${PROXY_DATA_VOLUME:-cbweb3-proxy-data}"

APPLY=0
PRUNE_CACHE=0
FORCE=0
for arg in "$@"; do
  case "$arg" in
    --yes|-y)      APPLY=1 ;;
    --prune-cache) PRUNE_CACHE=1 ;;
    --force|-f)    FORCE=1 ;;
    -h|--help)     grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown arg: $arg" >&2; exit 1 ;;
  esac
done

# --- Resolve what to keep, and refuse to run if the proxy isn't there ---------
PROXY_CID="$(docker ps -aq --filter "name=^${PROXY_NAME}$")"
if [ -z "$PROXY_CID" ]; then
  echo "ABORT: no container named '${PROXY_NAME}' found." >&2
  echo "       Refusing to continue — without an anchor this would delete everything." >&2
  exit 1
fi
if ! docker volume inspect "$PROXY_VOL" >/dev/null 2>&1; then
  echo "ABORT: proxy volume '${PROXY_VOL}' not found." >&2
  echo "       Check the name (PROXY_DATA_VOLUME) before deleting any volumes." >&2
  exit 1
fi
PROXY_IMG="$(docker inspect --format '{{.Image}}' "$PROXY_CID" | sed 's/^sha256://' | cut -c1-12)"

echo "Keeping:"
echo "  container : ${PROXY_NAME} (${PROXY_CID})"
echo "  image     : ${PROXY_IMG}"
echo "  volume    : ${PROXY_VOL}"
echo

# grep -v returns 1 when nothing else exists; tolerate that with `|| true`.
OTHER_CONTAINERS="$(docker ps -aq | grep -v "^${PROXY_CID}$" || true)"
OTHER_IMAGES="$(docker images -q | sort -u | grep -v "^${PROXY_IMG}$" || true)"
OTHER_VOLUMES="$(docker volume ls -q | grep -v "^${PROXY_VOL}$" || true)"

list() { # $1 = label, $2 = ids
  echo "== $1 =="
  if [ -n "$2" ]; then echo "$2"; else echo "  (none)"; fi
  echo
}
list "containers to remove" "$OTHER_CONTAINERS"
list "images to remove"     "$OTHER_IMAGES"
list "volumes to remove"    "$OTHER_VOLUMES"

if [ "$APPLY" -ne 1 ]; then
  echo "DRY RUN — nothing deleted. Re-run with --yes to apply."
  exit 0
fi

if [ "$FORCE" -ne 1 ]; then
  printf 'Delete everything listed above (keeping %s / %s)? [type "yes"]: ' "$PROXY_NAME" "$PROXY_VOL"
  read -r reply
  [ "$reply" = "yes" ] || { echo "aborted."; exit 1; }
fi

# --- Execute: containers -> images -> volumes ---------------------------------
# Order matters: remove other containers first so their volumes detach and can go.
[ -n "$OTHER_CONTAINERS" ] && echo "$OTHER_CONTAINERS" | xargs -r docker rm -f
[ -n "$OTHER_IMAGES" ]     && echo "$OTHER_IMAGES"     | xargs -r docker rmi -f || true
# NOT -f: an in-use volume (the proxy's) must be refused, never forced.
[ -n "$OTHER_VOLUMES" ]    && echo "$OTHER_VOLUMES"    | xargs -r -n1 docker volume rm 2>/dev/null || true

if [ "$PRUNE_CACHE" -eq 1 ]; then
  docker network prune -f
  docker builder prune -af
fi

echo
echo "Done. Verifying the proxy survived:"
docker volume inspect "$PROXY_VOL" >/dev/null && echo "  cert volume OK: ${PROXY_VOL}"
docker ps --filter "name=^${PROXY_NAME}$" --format '  proxy container: {{.Names}} ({{.Status}})'
