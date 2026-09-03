#!/usr/bin/env bash
# Backup / restore the reverse proxy's certificate store.
#
# Caddy keeps issued certificates, private keys, AND the ACME account key under /data,
# which is persisted in the `cbweb3-proxy-data` docker volume. As long as that volume
# survives, existing certificates keep serving even after the Cloudflare API token is
# revoked (renewal stops, but the current certs remain valid until expiry, ~90 days).
#
# This script snapshots the whole store to a tarball ON THE VM, so you can restore it
# after wiping/recreating the environment (or moving to another host) without needing a
# live token. Backing up the ACME account key matters: restoring it lets Caddy resume
# renewals against the SAME Let's Encrypt account once a token is available again.
#
# Usage:
#   proxy/backup-certs.sh backup            # snapshot -> $PROXY_CERT_BACKUP_DIR
#   proxy/backup-certs.sh restore <file>    # restore a snapshot into the volume
#   proxy/backup-certs.sh list              # list snapshots
#
# The backup directory defaults to /opt/cbweb3/proxy-cert-backups (override with
# PROXY_CERT_BACKUP_DIR). Store snapshots on the VM's persistent disk; they contain
# PRIVATE KEYS — keep the directory root-only (this script creates it 0700).
set -euo pipefail

VOLUME="${PROXY_DATA_VOLUME:-cbweb3-proxy-data}"
BACKUP_DIR="${PROXY_CERT_BACKUP_DIR:-/opt/cbweb3/proxy-cert-backups}"

usage() { echo "usage: $0 {backup|restore <file.tgz>|list}" >&2; exit 1; }

volume_exists() { docker volume inspect "$VOLUME" >/dev/null 2>&1; }

case "${1:-}" in
  backup)
    volume_exists || { echo "volume $VOLUME not found (is the proxy deployed?)" >&2; exit 1; }
    mkdir -p "$BACKUP_DIR"; chmod 700 "$BACKUP_DIR"
    ts="$(date +%Y%m%d-%H%M%S)"
    out="proxy-data-${ts}.tgz"
    docker run --rm -v "$VOLUME":/data:ro -v "$BACKUP_DIR":/backup alpine:3.23 \
      tar czf "/backup/${out}" -C /data .
    chmod 600 "$BACKUP_DIR/${out}"
    echo "Backup written: $BACKUP_DIR/${out}"
    ;;
  restore)
    archive="${2:-}"; [ -n "$archive" ] || usage
    [ -f "$archive" ] || { echo "no such file: $archive" >&2; exit 1; }
    volume_exists || docker volume create "$VOLUME" >/dev/null
    docker run --rm -v "$VOLUME":/data -v "$(cd "$(dirname "$archive")" && pwd)":/backup alpine:3.23 \
      sh -c "cd /data && tar xzf /backup/$(basename "$archive")"
    echo "Restored $archive into volume $VOLUME."
    echo "Recreate the proxy to pick it up: docker rm -f cbweb3-proxy, then re-apply."
    ;;
  list)
    ls -lh "$BACKUP_DIR" 2>/dev/null || echo "no backups in $BACKUP_DIR"
    ;;
  *) usage ;;
esac
