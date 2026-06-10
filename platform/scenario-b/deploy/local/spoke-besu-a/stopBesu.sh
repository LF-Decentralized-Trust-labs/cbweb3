#!/bin/bash
set -e

# This script removes all docker containers, docker networks and temporary files related
# to the Besu network created by startBesu.sh

# Color codes for colored output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

NETWORK_NAME="spoke_a_besu_network"
CONTAINER_PREFIX="cbweb3-spoke-a-besu"

cleanup_path() {
    local relative_path="$1"
    if rm -rf "$relative_path" 2>/dev/null; then
        return 0
    fi

    # Fallback for root-owned files created by dockerized Besu processes.
    docker run --rm -v "$(pwd):/workspace" alpine:3.20 sh -c "rm -rf /workspace/$relative_path" >/dev/null 2>&1 || true
}

echo -e "${YELLOW}Cleaning old files...${NC}"
echo
cleanup_path "tmpFiles"
cleanup_path "networkFiles"
cleanup_path "genesis"
cleanup_path "nodes/central-bank-a"
cleanup_path "nodes/bank-a"
cleanup_path "config/qbftConfigFile.json"
cleanup_path ".env.network"

echo -e "${YELLOW}Removing all previous besu node containers...${NC}"
CONTAINERS=$(docker ps -aq --filter "name=${CONTAINER_PREFIX}")
if [ -n "$CONTAINERS" ]; then
    docker rm -f $CONTAINERS 2>/dev/null || true
fi
echo

echo -e "${YELLOW}Removing docker ${NETWORK_NAME}...${NC}"
docker network rm "${NETWORK_NAME}" 2>/dev/null || true
echo
