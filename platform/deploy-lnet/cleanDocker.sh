#!/usr/bin/env bash
# Wipe the local Docker state so a fresh deploy runs without interference:
# stops every container, then removes all containers, volumes, images and
# user-defined networks, and finishes with a full system prune.
#
# WARNING: this is host-wide. It removes ALL Docker resources on the machine it
# runs on, not just this project's. Intended for dedicated deploy hosts.
#
# Usage:
#   ./cleanDocker.sh            # prompts for confirmation when attached to a TTY
#   ./cleanDocker.sh --yes      # skip the prompt (used by ship.sh and CI)
#
# Environment variables:
#   DOCKER_CLEAN_YES   truthy (1/yes/true) to skip the confirmation prompt
#   DOCKER_SUDO        truthy to prefix every docker command with `sudo`
set -euo pipefail

YES=no
[[ "${1:-}" == "--yes" || "${1:-}" == "-y" ]] && YES=yes
case "${DOCKER_CLEAN_YES:-no}" in 1|yes|true|on) YES=yes ;; esac

DOCKER=(docker)
case "${DOCKER_SUDO:-no}" in 1|yes|true|on) DOCKER=(sudo docker) ;; esac

if ! command -v "${DOCKER[0]}" >/dev/null 2>&1; then
  echo "[cleanDocker] docker not found on this host" >&2
  exit 2
fi
if ! "${DOCKER[@]}" info >/dev/null 2>&1; then
  echo "[cleanDocker] cannot talk to the Docker daemon (is it running? permissions?)" >&2
  exit 2
fi

if [[ "$YES" != "yes" ]]; then
  if [[ -t 0 ]]; then
    read -r -p "[cleanDocker] This wipes ALL Docker state on this host. Continue? [y/N] " ans
    [[ "$ans" =~ ^[Yy]$ ]] || { echo "[cleanDocker] aborted"; exit 0; }
  else
    echo "[cleanDocker] refusing to run non-interactively without --yes (or DOCKER_CLEAN_YES=yes)" >&2
    exit 2
  fi
fi

# Helper: run a docker subcommand over a (possibly empty) id list.
_apply() { # _apply <action-desc> <docker-args...> -- <id...>
  local desc="$1"; shift
  local args=(); while [[ "$1" != "--" ]]; do args+=("$1"); shift; done; shift
  if [[ "$#" -gt 0 ]]; then
    echo "[cleanDocker] $desc ($# item(s))"
    "${DOCKER[@]}" "${args[@]}" "$@" || true
  else
    echo "[cleanDocker] $desc (none)"
  fi
}

echo "[cleanDocker] stopping running containers"
mapfile -t running < <("${DOCKER[@]}" ps -q)
_apply "stop containers" stop -- "${running[@]}"

echo "[cleanDocker] removing containers"
mapfile -t containers < <("${DOCKER[@]}" ps -aq)
_apply "remove containers" rm -f -- "${containers[@]}"

echo "[cleanDocker] removing images"
mapfile -t images < <("${DOCKER[@]}" images -aq)
_apply "remove images" rmi -f -- "${images[@]}"

echo "[cleanDocker] removing volumes"
mapfile -t volumes < <("${DOCKER[@]}" volume ls -q)
_apply "remove volumes" volume rm -f -- "${volumes[@]}"

echo "[cleanDocker] pruning networks"
"${DOCKER[@]}" network prune -f || true

echo "[cleanDocker] full system prune (containers, images, networks, volumes, build cache)"
"${DOCKER[@]}" system prune -af --volumes || true

echo "[cleanDocker] done — Docker environment is clean"
