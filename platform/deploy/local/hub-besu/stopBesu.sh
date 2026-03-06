#!/bin/bash

# This script removes all docker containers, docker networks and temporary files related
# to the Besu network created by startBesu.sh

# Color codes for colored output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

NETWORK_NAME="cbweb3_hub_besu_network"
CONTAINER_PREFIX="cbweb3-hub-besu"

echo -e "${YELLOW}Cleaning old files...${NC}"
echo
rm -rf tmpFiles/
rm -rf networkFiles/
rm -rf genesis/
rm -rf nodes/
rm -rf config/qbftConfigFile.json
rm -f .env.network

echo -e "${YELLOW}Removing all previous besu node containers...${NC}"
CONTAINERS=$(docker ps -aq --filter "name=${CONTAINER_PREFIX}.")
if [ -n "$CONTAINERS" ]; then
    docker rm -f $CONTAINERS 2>/dev/null || true
fi
echo

echo -e "${YELLOW}Removing docker ${NETWORK_NAME}...${NC}"
docker network rm "${NETWORK_NAME}" 2>/dev/null || true
echo
