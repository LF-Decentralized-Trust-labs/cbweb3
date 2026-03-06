#!/bin/bash

# Color codes for colored output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

NETWORK_NAME="spoke_a_besu_network"
CONTAINER_PREFIX="cbweb3-spoke-a-besu"
BOOTNODE_CONTAINER="${CONTAINER_PREFIX}.bootnode"
NODE_CONTAINER_PREFIX="${CONTAINER_PREFIX}.node"

BOOT_P2P_PORT=31303
BOOT_RPC_PORT=8645

# Remove previous Besu network
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo -e "${YELLOW}Stopping any existing Besu network...${NC}"
if [ -f "$SCRIPT_DIR/stopBesu.sh" ]; then
    bash "$SCRIPT_DIR/stopBesu.sh"
fi

echo -e "${YELLOW}Detecting operating system...${NC}"
# Detect operating system
OS="$(uname)"
if [ "$OS" = "Darwin" ]; then
    # macOS specific settings
    SED_CMD="sed -i ''"
    MKTEMP_CMD="mktemp -t tmp"
else
    # Linux specific settings
    SED_CMD="sed -i"
    MKTEMP_CMD="mktemp"
fi

echo -e "${YELLOW}Setting up default values...${NC}"
# Default values
NODES=3
BASE_P2P_PORT=30320
BASE_RPC_PORT=8646
DEBUG_MODE=false

echo -e "${YELLOW}Checking if besu binary is installed...${NC}"
# Check if besu binary is installed and download it if not
if ! [ -x "$(command -v ./bin/besu)" ]; then
    if [ "$OS" = "Darwin" ]; then
        wget -P . https://github.com/hyperledger/besu/releases/download/25.8.0/besu-25.8.0.tar.gz || curl -L -o besu-25.8.0.tar.gz https://github.com/hyperledger/besu/releases/download/25.8.0/besu-25.8.0.tar.gz
    else
        wget -P . https://github.com/hyperledger/besu/releases/download/25.8.0/besu-25.8.0.tar.gz
    fi
    tar --strip-components=1 -xzf besu-25.8.0.tar.gz
    rm besu-25.8.0.tar.gz
fi
echo

BESU=./bin/besu

# Parse command line arguments
echo -e "${YELLOW}Parsing command line arguments...${NC}"
while getopts ":n:d-:" opt; do
    case $opt in
    n)
        NODES=${OPTARG}
        ;;
    d)
        DEBUG_MODE=true
        ;;
    -)
        case $OPTARG in
            debug)
                DEBUG_MODE=true
                ;;
        esac
        ;;
    esac
done

if [ -z "$NODES" ]; then
    echo -e "${RED}NODES is not set. Please set the number of nodes to create.${NC}"
    exit 1
fi

# Export NODES for use in functions
export NODES

# Define extra logging flag if debug mode is enabled
if [ "$DEBUG_MODE" = true ]; then
    BESU_LOGGING="--logging=DEBUG"
    echo -e "${YELLOW}Debug mode enabled. Besu nodes will start with DEBUG logging.${NC}"
else
    BESU_LOGGING=""
fi

echo -e "${YELLOW}Creating qbftConfigFile.json based on template...${NC}"
jq '.blockchain += {
    "nodes": {
      "generate": true,
      "count": '"$NODES"'
    }
  }' config/configTemplate.json >config/qbftConfigFile.json

echo -e "${YELLOW}Creating bootnode folder...${NC}"
mkdir -p nodes/bootnode

echo -e "${YELLOW}Generating blockchain config and keys...${NC}"
mkdir tmpFiles && cd tmpFiles
../$BESU operator generate-blockchain-config --config-file=../config/qbftConfigFile.json --to=networkFiles --private-key-file-name=key

cd ..

counter=0
for folder in tmpFiles/networkFiles/keys/*; do
    if [ $counter -eq 0 ]; then
        echo -e "${YELLOW}Copying bootnode files...${NC}"
        mkdir -p nodes/bootnode/data
        cp -r "$folder"/* nodes/bootnode/data/
    else
        echo -e "${YELLOW}Copying node $counter files...${NC}"
        mkdir -p nodes/node$counter
        mkdir -p nodes/node$counter/data
        cp -r "$folder"/* nodes/node$counter/data/
    fi
    counter=$((counter + 1))
done

echo -e "${YELLOW}Copying genesis file...${NC}"
mkdir -p genesis
cp tmpFiles/networkFiles/genesis.json genesis/genesis.json

echo -e "${YELLOW}Removing tmpFiles...${NC}"
if [ "$OS" = "Darwin" ]; then
    rm -rf tmpFiles
else
    sudo rm -rf tmpFiles
fi
echo

echo -e "${BLUE}Starting docker network '${NETWORK_NAME}'...${NC}"
docker network create --driver bridge "${NETWORK_NAME}"
if [ $? -eq 0 ]; then
    echo -e "${GREEN}Docker network created successfully.${NC}\n"
else
    echo -e "${YELLOW}Docker network may already exist. Continuing...${NC}\n"
fi

echo -e "${BLUE}Starting bootnode on docker...${NC}"
docker run -d \
    --name "${BOOTNODE_CONTAINER}" \
    --user root \
    -v "$(pwd)/nodes/bootnode/data:/opt/besu/data" \
    -v "$(pwd)/genesis:/opt/besu/genesis" \
    -p ${BOOT_P2P_PORT}:30303 \
    -p ${BOOT_RPC_PORT}:8545 \
    -p ${BOOT_P2P_PORT}:30303/udp \
    --network "${NETWORK_NAME}" \
    --restart always \
    hyperledger/besu:latest \
    --data-path=data --genesis-file=genesis/genesis.json --min-gas-price=0 --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT --rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT --host-allowlist='*' --rpc-http-cors-origins='all' --rpc-http-host='0.0.0.0' --rpc-ws-host='0.0.0.0' --rpc-http-port=8545 --rpc-ws-port=8546 --p2p-port=30303 $BESU_LOGGING

echo

echo -e "${GREEN}Bootnode created!${NC}"
echo -e "${YELLOW}Waiting 5 seconds for bootnode to start...${NC}"
sleep 5
echo -e "${YELLOW}Fetching ENODE from bootnode...${NC}"

max_retries=30
retry_delay=3
retry_count=0

while [ $retry_count -lt $max_retries ]; do
    ENODE=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"net_enode","params":[],"id":1}' "http://127.0.0.1:${BOOT_RPC_PORT}" | jq -r '.result')
    if [ -n "$ENODE" ] && [ "$ENODE" != "null" ]; then
        echo -e "${GREEN}ENODE retrieved successfully.${NC}"
        break
    else
        echo -e "${RED}Failed to retrieve ENODE. Retrying in $retry_delay seconds...${NC}"
        sleep $retry_delay
        ((retry_count++))
    fi
done

if [ $retry_count -eq $max_retries ]; then
    echo -e "${RED}Max retries reached. Unable to retrieve ENODE.${NC}"
    exit 1
fi

echo -e "${BLUE}ENODE: $ENODE${NC}\n"

export E_ADDRESS="${ENODE#enode://}"
DOCKER_NODE_1_ADDRESS=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${BOOTNODE_CONTAINER}")
export E_ADDRESS=$(echo "$E_ADDRESS" | sed -e "s/127.0.0.1/$DOCKER_NODE_1_ADDRESS/g")
export E_ADDRESS="enode://$E_ADDRESS"

generate_nodes_function() {
    local i=1
    while [ $i -le $((NODES - 1)) ]; do
        local node_name="${NODE_CONTAINER_PREFIX}-${i}"
        local p2p_port=$((BASE_P2P_PORT + i))
        local rpc_port=$((BASE_RPC_PORT + i))

        echo -e "${BLUE}Creating docker container ${node_name}...${NC}"
        docker run -d \
            --name ${node_name} \
            --user root \
            -v "$(pwd)/nodes/node${i}/data:/opt/besu/data" \
            -v "$(pwd)/genesis:/opt/besu/genesis" \
            -p ${rpc_port}:8545 \
            -p ${p2p_port}:30303 \
            -p ${p2p_port}:30303/udp \
            --network "${NETWORK_NAME}" \
            --restart always \
            hyperledger/besu:latest \
            --data-path=data --genesis-file=genesis/genesis.json --min-gas-price=0 --bootnodes=${E_ADDRESS} --p2p-port=30303 --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT --rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT --host-allowlist='*' --rpc-http-cors-origins='all' --rpc-http-host='0.0.0.0' --rpc-ws-host='0.0.0.0' --rpc-http-port=8545 --rpc-ws-port=8546 $BESU_LOGGING

        if [ $? -eq 0 ]; then
            echo -e "${GREEN}Node ${node_name} started successfully!${NC}\n"
        else
            echo -e "${RED}Failed to start node ${node_name}.${NC}\n"
        fi
        i=$((i + 1))
    done
}

generate_nodes_function

echo -e "${YELLOW}Creating network tracker file...${NC}"
cat >.env.network <<EOF
NODES=$NODES
ITERATION=1
E_ADDRESS=$E_ADDRESS
NETWORK_NAME=$NETWORK_NAME
CONTAINER_PREFIX=$CONTAINER_PREFIX
BOOT_RPC_PORT=$BOOT_RPC_PORT
BASE_RPC_PORT=$BASE_RPC_PORT
BASE_P2P_PORT=$BASE_P2P_PORT
EOF

echo -e "${GREEN}============================="
echo -e "Network started successfully!"
echo -e "=============================${NC}\n"
