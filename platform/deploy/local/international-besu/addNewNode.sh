#!/bin/bash

# Color variables for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

if ! [ -f ".env.network" ]; then
    echo -e "${RED}.env.network file not found. The network is not initialized.${NC}"
    exit 1
fi

export $(grep -v '^#' .env.network | xargs)

NEW_NODES=1
BASE_P2P_PORT=30303
BASE_RPC_PORT=8545
DEBUG_MODE=""

# Parse options
while getopts n:d-: opt; do
    case $opt in
    n)
        NEW_NODES=${OPTARG}
        ;;
    d)
        DEBUG_MODE="--logging=DEBUG"
        ;;
    -)
        case $OPTARG in
            debug)
                DEBUG_MODE="--logging=DEBUG"
                ;;
        esac
        ;;
    esac
done

ECHO_NODES=$NEW_NODES

generate_nodes_function() {
    for ((i = NODES; i <= (NEW_NODES + NODES - 1); i++)); do
        local node_name="besu.node-${i}"
        local p2p_port=$((BASE_P2P_PORT + i))
        local rpc_port=$((BASE_RPC_PORT + i))

        echo -e "${BLUE}Creating node: ${node_name}${NC}"
        docker run -d \
            --name ${node_name} \
            --user root \
            -v "$(pwd)/nodes/node${i}/data:/opt/besu/data" \
            -v "$(pwd)/genesis:/opt/besu/genesis" \
            -p ${rpc_port}:${rpc_port} \
            -p ${p2p_port}:${p2p_port} \
            -p ${rpc_port}:${rpc_port}/udp \
            -p ${p2p_port}:${p2p_port}/udp \
            --network besu_test_network \
            --restart always \
            hyperledger/besu:latest \
            --data-path=data --genesis-file=genesis/genesis.json --bootnodes=${E_ADDRESS} --p2p-port=${p2p_port} --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT --host-allowlist='*' --rpc-http-cors-origins='all' --rpc-http-port=${rpc_port} ${DEBUG_MODE}
        echo -e "${GREEN}Node ${node_name} created.${NC}\n"
    done
}

generate_nodes_function

echo -e "${YELLOW}Waiting for nodes to start and sync...${NC}\n"
sleep 10

echo -e "${BLUE}Requesting new validator status for new nodes...${NC}"
for ((i = NODES; i <= (NEW_NODES + NODES - 1); i++)); do

    MAX_TRIES=30
    TRY_COUNT=1

    while [ $TRY_COUNT -lt $MAX_TRIES ]; do
        export NODE_ADDRESS=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_coinbase","params":[],"id":1}' http://localhost:$((BASE_RPC_PORT + i)) | jq -r '.result')

        if [ -n "$NODE_ADDRESS" ]; then
            if [ "$NODE_ADDRESS" != "null" ]; then
                echo -e "${GREEN}NODE_ADDRESS retrieved.${NC}"
                break
            fi
        else
            echo -e "${YELLOW}NODE_ADDRESS not retrieved. Trying again...${NC}"
            sleep 5
            TRY_COUNT=$((TRY_COUNT + 1))
        fi
    done

    if [ $MAX_TRIES -eq $TRY_COUNT ]; then
        echo -e "${RED}Failed to retrieve NODE_ADDRESS. Stopping and removing node...${NC}"
        docker stop node${i}
        docker rm node${i} -f
        break
    fi

    echo -e "${BLUE}Cleaned NODE_ADDRESS: $NODE_ADDRESS${NC}"

    echo -e "${YELLOW}Starting Validator Voting Process${NC}"
    echo -e "Requesting validator to node ${i} from http://localhost:8545..."
    curl -s -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"qbft_proposeValidatorVote\",\"params\":[\"$NODE_ADDRESS\",true],\"id\":1}" http://localhost:8545
    echo ""

    echo -e "Checking pending votes..."
    curl -s -X POST --data '{"jsonrpc":"2.0","method":"qbft_getPendingVotes","params":[], "id":1}' http://localhost:8545
    echo ""

    echo -e "Running requests from all validators to node${i}..."
    for ((j = 1; j <= NODES - 1; j++)); do
        rpc_port=$((BASE_RPC_PORT + j))
        echo -e "Requesting validator to node ${i} from http://localhost:${rpc_port}..."
        curl -s -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"qbft_proposeValidatorVote\",\"params\":[\"$NODE_ADDRESS\",true],\"id\":1}" http://localhost:$rpc_port
        echo ""
    done

    echo -e "${YELLOW}Waiting for validator to be added on list...${NC}"
    while [ true ]; do
        VALIDATOR_LIST_LENGTH=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"qbft_getValidatorsByBlockNumber","params":["latest"],"id":1}' http://localhost:8545 | jq '.result | length')
        if [ $VALIDATOR_LIST_LENGTH -eq $NODES ]; then
            echo -e "Validator add pending..."
            sleep 5
            continue
        else
            echo -e "${GREEN}Validator added!${NC}"
            break
        fi
    done

    echo -e "${YELLOW}Close Validator Voting Process${NC}"
    echo -e "Closing validator voting process for ${NODE_ADDRESS} from http://localhost:8545..."
    curl -s -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"qbft_discardValidatorVote\",\"params\":[\"$NODE_ADDRESS\"],\"id\":1}" http://localhost:8545
    echo ""

    for ((j = 1; j <= NODES - 1; j++)); do
        rpc_port=$((BASE_RPC_PORT + j))
        echo -e "Discard validator vote to node ${i} from http://localhost:${rpc_port}..."
        curl -s -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"qbft_discardValidatorVote\",\"params\":[\"$NODE_ADDRESS\"],\"id\":1}" http://localhost:$rpc_port
        echo ""
    done

    echo -e "Checking pending votes..."
    curl -s -X POST --data '{"jsonrpc":"2.0","method":"qbft_getPendingVotes","params":[], "id":1}' http://localhost:8545
    echo ""

    NODES=$((NODES + 1))
    NEW_NODES=$((NEW_NODES - 1))
done

echo -e "${BLUE}Updating network tracker file...${NC}"
echo "NODES=$((NODES + NEW_NODES))" >.env.network
echo "ITERATION=$((ITERATION + 1))" >>.env.network
echo "E_ADDRESS=${E_ADDRESS}" >>.env.network

echo -e "${GREEN}==================================================${NC}"
echo -e "${GREEN}$ECHO_NODES validator node(s) added successfully!${NC}"
echo -e "${GREEN}==================================================${NC}\n"
