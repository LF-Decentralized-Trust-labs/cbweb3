#!/bin/bash
# generate-certs.sh — generate self-signed P-256 TLS certificates for Paladin nodes.
#
# Usage:
#   SPOKE=spoke-a bash generate-certs.sh
#   SPOKE=spoke-b bash generate-certs.sh
#
# Each Paladin node gets its own self-signed cert whose SAN includes the Docker
# container hostname so that mTLS peer verification works inside the network.
set -euo pipefail

SPOKE="${SPOKE:-spoke-a}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

case "$SPOKE" in
  spoke-a) NODE_PAIRS="central-bank:paladin-spoke-a-cb bank-a:paladin-spoke-a-bank-a bank-c:paladin-spoke-a-bank-c" ;;
  spoke-b) NODE_PAIRS="central-bank:paladin-spoke-b-cb bank-b:paladin-spoke-b-bank-b bank-d:paladin-spoke-b-bank-d" ;;
  *)
    echo "ERROR: SPOKE must be 'spoke-a' or 'spoke-b'" >&2
    exit 1
    ;;
esac

for pair in $NODE_PAIRS; do
  name="${pair%%:*}"
  hostname="${pair##*:}"
  cert_dir="$SCRIPT_DIR/$SPOKE/config/$name"
  mkdir -p "$cert_dir"

  echo "Generating TLS cert for '$name' (hostname: $hostname) ..."
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
    -days 3650 -nodes \
    -keyout "$cert_dir/tls.key" \
    -out  "$cert_dir/tls.crt" \
    -subj "/CN=$name" \
    -addext "subjectAltName=DNS:$hostname,DNS:$name,DNS:localhost" \
    2>/dev/null

  chmod 644 "$cert_dir/tls.crt"
  chmod 600 "$cert_dir/tls.key"
  echo "  -> $cert_dir/tls.{crt,key}"
done

echo "Certificate generation complete for $SPOKE."
