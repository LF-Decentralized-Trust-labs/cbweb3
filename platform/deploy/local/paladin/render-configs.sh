#!/bin/bash
# render-configs.sh — render config.yaml from config.yaml.tmpl using deployed contract addresses.
#
# Usage:
#   SPOKE=spoke-a bash render-configs.sh
#   SPOKE=spoke-b bash render-configs.sh
#
# Reads REGISTRY_CONTRACT_ADDRESS and ZETO_FACTORY_ADDRESS from
#   deploy/local/paladin/<spoke>/.deployed-addrs.env
# and uses envsubst to produce config.yaml from each config.yaml.tmpl.
set -euo pipefail

SPOKE="${SPOKE:-spoke-a}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ADDRS_FILE="$SCRIPT_DIR/$SPOKE/.deployed-addrs.env"

if [ ! -f "$ADDRS_FILE" ]; then
    echo "ERROR: $ADDRS_FILE not found. Run 'make paladin.deploy-contracts-$SPOKE' first." >&2
    exit 1
fi

# shellcheck source=/dev/null
set -a
source "$ADDRS_FILE"
set +a

: "${REGISTRY_CONTRACT_ADDRESS:?REGISTRY_CONTRACT_ADDRESS not set in $ADDRS_FILE}"
: "${ZETO_FACTORY_ADDRESS:?ZETO_FACTORY_ADDRESS not set in $ADDRS_FILE}"

echo "Rendering Paladin configs for $SPOKE ..."
echo "  REGISTRY_CONTRACT_ADDRESS = $REGISTRY_CONTRACT_ADDRESS"
echo "  ZETO_FACTORY_ADDRESS      = $ZETO_FACTORY_ADDRESS"

case "$SPOKE" in
  spoke-b) NODES="central-bank bank-b bank-d" ;;
  *)       NODES="central-bank bank-a bank-c" ;;
esac

for node in $NODES; do
    tmpl="$SCRIPT_DIR/$SPOKE/config/$node/config.yaml.tmpl"
    out="$SCRIPT_DIR/$SPOKE/config/$node/config.yaml"
    if [ ! -f "$tmpl" ]; then
        echo "WARNING: template not found: $tmpl" >&2
        continue
    fi
    envsubst '${REGISTRY_CONTRACT_ADDRESS} ${ZETO_FACTORY_ADDRESS}' < "$tmpl" > "$out"
    echo "  -> $out"
done

echo "Config rendering complete for $SPOKE."
