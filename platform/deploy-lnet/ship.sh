#!/usr/bin/env bash
# Archive the whole project, copy the tarball to a deploy machine over SCP, and
# extract it there. Authenticates with either an SSH key or a password, selected
# via environment variables (no secrets are ever written to disk or the tarball).
#
# Usage:
#   DEPLOY_HOST=10.10.0.20 ./ship.sh
#   DEPLOY_HOST=10.10.0.20 DEPLOY_AUTH=key DEPLOY_KEY=~/.ssh/lnet ./ship.sh
#   DEPLOY_HOST=10.10.0.20 DEPLOY_AUTH=password DEPLOY_PASSWORD=secret ./ship.sh
#   DEPLOY_HOST=10.10.0.20 DEPLOY_CLEAN=yes ./ship.sh   # wipe remote dest first
#   ./ship.sh --dry-run                          # build the tarball, skip transfer
#
# Environment variables:
#   DEPLOY_HOST        (required) target hostname or IP
#   DEPLOY_USER        SSH user                        (default: $USER)
#   DEPLOY_PORT        SSH port                         (default: 22)
#   DEPLOY_DEST        remote extraction directory      (default: ~/cbweb3-platform)
#   DEPLOY_AUTH        key | password                   (default: auto — key if
#                      DEPLOY_KEY is set, else password if DEPLOY_PASSWORD is set,
#                      else key with the agent/default identity)
#   DEPLOY_KEY         path to the private key          (key auth)
#   DEPLOY_PASSWORD    SSH/sudo password                (password auth; needs sshpass)
#   DEPLOY_STRICT_HOST_KEY   yes | no | accept-new      (default: accept-new)
#   DEPLOY_EXTRA_EXCLUDES    space-separated extra tar --exclude patterns
#   DEPLOY_CLEAN       when truthy (1/yes/true), wipe the remote DEPLOY_DEST
#                      before extracting (rm -rf) for a clean mirror. Default: no
#                      (extraction overlays onto existing contents).
#   DEPLOY_DOCKER_CLEAN  when truthy, run deploy-lnet/cleanDocker.sh on the
#                      remote as the LAST step (after extract) to wipe all Docker
#                      state (containers/images/volumes/networks + prune) for an
#                      interference-free deploy. Host-wide. Default: no.
#   DEPLOY_DOCKER_SUDO   when truthy, tell the remote cleanDocker.sh to prefix
#                      docker with sudo (forwarded as DOCKER_SUDO). Default: no.
#
# Notes:
#   - Password auth requires `sshpass` on this machine (brew install hudochenkov/sshpass/sshpass).
#   - Heavy/generated trees (.git, node_modules, build output, local binaries) are
#     excluded by default; add more with DEPLOY_EXTRA_EXCLUDES.
#   - By default the tarball is extracted ON TOP of whatever is already in
#     DEPLOY_DEST (overlay): matching paths are overwritten, unmatched remote
#     files are left in place. Set DEPLOY_CLEAN=yes for a clean mirror instead —
#     it runs `rm -rf` on DEPLOY_DEST first, so remote-only files (including
#     excluded/generated trees such as node_modules, .git, dist) are removed too.
#     Unsafe targets ("", "/", "~", ".", "./", "..") are refused.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"

# Auto-load ./.env if present. A variable already set in the environment wins,
# so `DEPLOY_HOST=x ./ship.sh` still overrides the file. Point DEPLOY_ENV_FILE
# elsewhere to use a different file; set it empty to skip loading entirely.
ENV_FILE="${DEPLOY_ENV_FILE-$HERE/.env}"
if [[ -n "$ENV_FILE" && -f "$ENV_FILE" ]]; then
  _preset="$(export -p | grep -E '^(declare -x|export) (DEPLOY_|SSHPASS)' || true)"
  set -a
  # shellcheck source=/dev/null
  source "$ENV_FILE"
  set +a
  eval "$_preset"   # re-apply values that were already set in the environment
fi

DRY_RUN=no
[[ "${1:-}" == "--dry-run" || "${1:-}" == "-n" ]] && DRY_RUN=yes

DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_USER="${DEPLOY_USER:-$USER}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
# Relative path -> resolved under the remote login home. Avoid a bare `~` in
# .env: it would expand to the LOCAL home before being sent to the remote.
DEPLOY_DEST="${DEPLOY_DEST:-cbweb3-platform}"
DEPLOY_STRICT_HOST_KEY="${DEPLOY_STRICT_HOST_KEY:-accept-new}"

# Wipe the remote destination before extracting when DEPLOY_CLEAN is truthy.
DEPLOY_CLEAN="${DEPLOY_CLEAN:-no}"
case "${DEPLOY_CLEAN,,}" in
  1|yes|true|on) DEPLOY_CLEAN=yes ;;
  *)             DEPLOY_CLEAN=no ;;
esac

# Run cleanDocker.sh on the remote as the last step when DEPLOY_DOCKER_CLEAN is truthy.
DEPLOY_DOCKER_CLEAN="${DEPLOY_DOCKER_CLEAN:-no}"
case "${DEPLOY_DOCKER_CLEAN,,}" in
  1|yes|true|on) DEPLOY_DOCKER_CLEAN=yes ;;
  *)             DEPLOY_DOCKER_CLEAN=no ;;
esac
DEPLOY_DOCKER_SUDO="${DEPLOY_DOCKER_SUDO:-no}"
case "${DEPLOY_DOCKER_SUDO,,}" in
  1|yes|true|on) DEPLOY_DOCKER_SUDO=yes ;;
  *)             DEPLOY_DOCKER_SUDO=no ;;
esac

if [[ "$DRY_RUN" == "no" && -z "$DEPLOY_HOST" ]]; then
  echo "[ship] DEPLOY_HOST is required (see header for usage)" >&2
  exit 2
fi

# --- pick an auth mode ------------------------------------------------------
DEPLOY_AUTH="${DEPLOY_AUTH:-}"
if [[ -z "$DEPLOY_AUTH" ]]; then
  if [[ -n "${DEPLOY_KEY:-}" ]]; then DEPLOY_AUTH=key
  elif [[ -n "${DEPLOY_PASSWORD:-}" ]]; then DEPLOY_AUTH=password
  else DEPLOY_AUTH=key; fi
fi

SSH_OPTS=(-p "$DEPLOY_PORT" -o "StrictHostKeyChecking=$DEPLOY_STRICT_HOST_KEY")
SCP_OPTS=(-P "$DEPLOY_PORT" -o "StrictHostKeyChecking=$DEPLOY_STRICT_HOST_KEY")
SSH_PREFIX=()

case "$DEPLOY_AUTH" in
  key)
    if [[ -n "${DEPLOY_KEY:-}" ]]; then
      key_path="${DEPLOY_KEY/#\~/$HOME}"
      [[ -f "$key_path" ]] || { echo "[ship] key not found: $key_path" >&2; exit 2; }
      SSH_OPTS+=(-i "$key_path" -o IdentitiesOnly=yes)
      SCP_OPTS+=(-i "$key_path" -o IdentitiesOnly=yes)
    fi
    ;;
  password)
    command -v sshpass >/dev/null 2>&1 || {
      echo "[ship] password auth needs sshpass (brew install hudochenkov/sshpass/sshpass)" >&2
      exit 2
    }
    [[ -n "${DEPLOY_PASSWORD:-}" ]] || { echo "[ship] DEPLOY_PASSWORD is empty" >&2; exit 2; }
    export SSHPASS="$DEPLOY_PASSWORD"
    SSH_PREFIX=(sshpass -e)
    SSH_OPTS+=(-o PubkeyAuthentication=no)
    SCP_OPTS+=(-o PubkeyAuthentication=no)
    ;;
  *)
    echo "[ship] DEPLOY_AUTH must be 'key' or 'password' (got: $DEPLOY_AUTH)" >&2
    exit 2
    ;;
esac

# --- build the tarball ------------------------------------------------------
EXCLUDES=(
  --exclude='./.git'
  --exclude='./deploy-lnet/.bin'
  --exclude='*/node_modules'
  --exclude='*/.turbo'
  --exclude='*/dist'
  --exclude='*/build'
  --exclude='*/out'
  --exclude='*/target'
  --exclude='*/coverage'
  --exclude='*/.next'
  --exclude='./tmp'
  --exclude='*/tmp'
  --exclude='./evidence-bundles'
  --exclude='.DS_Store'
  --exclude='._*'
)
if [[ -n "${DEPLOY_EXTRA_EXCLUDES:-}" ]]; then
  for pat in $DEPLOY_EXTRA_EXCLUDES; do EXCLUDES+=(--exclude="$pat"); done
fi

TARBALL="$(mktemp -t cbweb3-platform.XXXXXX).tar.gz"
trap 'rm -f "$TARBALL"' EXIT

echo "[ship] archiving $ROOT -> $TARBALL"
# COPYFILE_DISABLE=1 stops macOS BSD tar from embedding AppleDouble (`._*`)
# extended-attribute headers, so the archive stays clean on a Linux target.
COPYFILE_DISABLE=1 tar -czf "$TARBALL" "${EXCLUDES[@]}" -C "$ROOT" .
echo "[ship] tarball size: $(du -h "$TARBALL" | cut -f1)"

if [[ "$DRY_RUN" == "yes" ]]; then
  echo "[ship] --dry-run: tarball built, skipping transfer"
  trap - EXIT
  echo "[ship] kept at: $TARBALL"
  exit 0
fi

# --- transfer + extract -----------------------------------------------------
REMOTE="${DEPLOY_USER}@${DEPLOY_HOST}"
REMOTE_TMP="/tmp/cbweb3-platform.$$.tar.gz"

echo "[ship] copying tarball to ${REMOTE}:${REMOTE_TMP}"
"${SSH_PREFIX[@]}" scp "${SCP_OPTS[@]}" "$TARBALL" "${REMOTE}:${REMOTE_TMP}"

REMOTE_CLEAN=""
if [[ "$DEPLOY_CLEAN" == "yes" ]]; then
  # Refuse obviously dangerous targets before issuing rm -rf on the remote.
  case "$DEPLOY_DEST" in
    ""|"/"|"~"|"."|"./"|"..") echo "[ship] refusing to clean unsafe DEPLOY_DEST: '$DEPLOY_DEST'" >&2; exit 2 ;;
  esac
  echo "[ship] DEPLOY_CLEAN=yes — wiping ${REMOTE}:${DEPLOY_DEST} before extract"
  REMOTE_CLEAN="rm -rf -- ${DEPLOY_DEST};"
fi

echo "[ship] extracting into ${REMOTE}:${DEPLOY_DEST}"
"${SSH_PREFIX[@]}" ssh "${SSH_OPTS[@]}" "$REMOTE" \
  "set -e; ${REMOTE_CLEAN} mkdir -p ${DEPLOY_DEST}; tar -xzf ${REMOTE_TMP} -C ${DEPLOY_DEST}; rm -f ${REMOTE_TMP}; echo '[remote] extracted to' \$(cd ${DEPLOY_DEST} && pwd)"

echo "[ship] done — codebase deployed to ${REMOTE}:${DEPLOY_DEST}"

# --- last step: clean Docker on the remote ----------------------------------
if [[ "$DEPLOY_DOCKER_CLEAN" == "yes" ]]; then
  REMOTE_SUDO=""
  [[ "$DEPLOY_DOCKER_SUDO" == "yes" ]] && REMOTE_SUDO="DOCKER_SUDO=yes "
  echo "[ship] DEPLOY_DOCKER_CLEAN=yes — running cleanDocker.sh on ${REMOTE} (last step)"
  "${SSH_PREFIX[@]}" ssh "${SSH_OPTS[@]}" "$REMOTE" \
    "set -e; cd ${DEPLOY_DEST}/deploy-lnet && ${REMOTE_SUDO}bash ./cleanDocker.sh --yes"
  echo "[ship] done — remote Docker environment cleaned"
fi
