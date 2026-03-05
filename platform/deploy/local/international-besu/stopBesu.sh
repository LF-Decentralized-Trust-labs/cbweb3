#!/bin/bash

# This script removes all docker containers, docker networks and temporary files related 
# to the Besu network created by startBesu.sh

# Color codes for colored output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Cleaning old files...${NC}"
echo
if [ "$OS" = "Darwin" ]; then
    rm -rf tmpFiles/
    rm -rf networkFiles/
    rm -rf genesis/
    rm -rf nodes/
    rm -rf config/qbftConfigFile.json
    rm -f .env.network
else
    sudo rm -rf tmpFiles/
    sudo rm -rf networkFiles/
    sudo rm -rf genesis/
    sudo rm -rf nodes/
    sudo rm -rf config/qbftConfigFile.json
    sudo rm -f .env.network
fi

echo -e "${YELLOW}Removing all previous besu node containers...${NC}"
docker rm -f $(docker ps -f name=besu. -aq) 2>/dev/null || true
echo

echo -e "${YELLOW}Removing docker besu_test_network...${NC}"
docker network rm besu_test_network 2>/dev/null || true
echo