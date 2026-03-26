#!/bin/bash
#
# startBesu.sh — Inicia a rede Besu do spoke-b com 3 nós fixos nomeados por entidade:
#
#   cbweb3-spoke-b-besu.central-bank-b  (bootnode / validador)  RPC: 8745
#   cbweb3-spoke-b-besu.bank-b        (validador)              RPC: 8746
#   cbweb3-spoke-b-besu.bank-d        (validador)              RPC: 8747
#
# Cada backend conecta ao RPC do seu próprio nó via BESU_RPC_URL no .env.infra.*
#
# Uso: ./startBesu.sh [-d|--debug]

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

NETWORK_NAME="spoke_b_besu_network"
CONTAINER_PREFIX="cbweb3-spoke-b-besu"

NODE_CENTRAL_BANK_B="${CONTAINER_PREFIX}.central-bank-b"
NODE_BANK_B="${CONTAINER_PREFIX}.bank-b"
NODE_BANK_D="${CONTAINER_PREFIX}.bank-d"

RPC_PORT_CENTRAL_BANK_B=8745
RPC_PORT_BANK_B=8746
RPC_PORT_BANK_D=8747

P2P_PORT_CENTRAL_BANK_B=31403
P2P_PORT_BANK_B=31404
P2P_PORT_BANK_D=31405

NODES=3

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo -e "${YELLOW}Stopping any existing Besu network...${NC}"
if [ -f "$SCRIPT_DIR/stopBesu.sh" ]; then
    bash "$SCRIPT_DIR/stopBesu.sh"
fi

OS="$(uname)"
if [ "$OS" = "Darwin" ]; then
    SED_CMD="sed -i ''"
    MKTEMP_CMD="mktemp -t tmp"
else
    SED_CMD="sed -i"
    MKTEMP_CMD="mktemp"
fi

DEBUG_MODE=false
while getopts ":d-:" opt; do
    case $opt in
    d) DEBUG_MODE=true ;;
    -)
        case $OPTARG in
            debug) DEBUG_MODE=true ;;
        esac
        ;;
    esac
done

if [ "$DEBUG_MODE" = true ]; then
    BESU_LOGGING="--logging=DEBUG"
    echo -e "${YELLOW}Debug mode enabled.${NC}"
else
    BESU_LOGGING=""
fi

echo -e "${YELLOW}Checking if besu binary is installed...${NC}"
if ! [ -x "$(command -v ./bin/besu)" ]; then
    if [ "$OS" = "Darwin" ]; then
        wget -P . https://github.com/hyperledger/besu/releases/download/25.8.0/besu-25.8.0.tar.gz || \
            curl -L -o besu-25.8.0.tar.gz https://github.com/hyperledger/besu/releases/download/25.8.0/besu-25.8.0.tar.gz
    else
        wget -P . https://github.com/hyperledger/besu/releases/download/25.8.0/besu-25.8.0.tar.gz
    fi
    tar --strip-components=1 -xzf besu-25.8.0.tar.gz
    rm besu-25.8.0.tar.gz
fi
BESU=./bin/besu

echo -e "${YELLOW}Creating qbftConfigFile.json (3 validators)...${NC}"
jq '.blockchain += {"nodes": {"generate": true, "count": '"$NODES"'}}' \
    config/configTemplate.json > config/qbftConfigFile.json

echo -e "${YELLOW}Generating blockchain config and keys...${NC}"
mkdir -p nodes/central-bank-b/data nodes/bank-b/data nodes/bank-d/data
mkdir tmpFiles && cd tmpFiles
../$BESU operator generate-blockchain-config \
    --config-file=../config/qbftConfigFile.json \
    --to=networkFiles \
    --private-key-file-name=key
cd ..

counter=0
for folder in tmpFiles/networkFiles/keys/*; do
    case $counter in
    0)
        echo -e "${YELLOW}Copying central-bank-b node files...${NC}"
        cp -r "$folder"/* nodes/central-bank-b/data/
        ;;
    1)
        echo -e "${YELLOW}Copying bank-b node files...${NC}"
        cp -r "$folder"/* nodes/bank-b/data/
        ;;
    2)
        echo -e "${YELLOW}Copying bank-d node files...${NC}"
        cp -r "$folder"/* nodes/bank-d/data/
        ;;
    esac
    counter=$((counter + 1))
done

echo -e "${YELLOW}Copying genesis file...${NC}"
mkdir -p genesis
cp tmpFiles/networkFiles/genesis.json genesis/genesis.json

echo -e "${YELLOW}Removing tmpFiles...${NC}"
if ! rm -rf tmpFiles 2>/dev/null; then
    docker run --rm -v "$(pwd):/workspace" alpine:3.20 \
        sh -c "rm -rf /workspace/tmpFiles" >/dev/null 2>&1 || true
fi

echo -e "${BLUE}Starting docker network '${NETWORK_NAME}'...${NC}"
docker network create --driver bridge "${NETWORK_NAME}" && \
    echo -e "${GREEN}Docker network created.${NC}\n" || \
    echo -e "${YELLOW}Network may already exist. Continuing...${NC}\n"

# Ensure shared network exists (backends need to reach Besu nodes by container name)
docker network inspect cbweb3_network >/dev/null 2>&1 || docker network create cbweb3_network

# ─── Start central-bank-b node (bootnode) ─────────────────────────────────────
echo -e "${BLUE}Starting central-bank-b node (bootnode)...${NC}"
docker run -d \
    --name "${NODE_CENTRAL_BANK_B}" \
    --user root \
    -v "$(pwd)/nodes/central-bank-b/data:/opt/besu/data" \
    -v "$(pwd)/genesis:/opt/besu/genesis" \
    -p ${RPC_PORT_CENTRAL_BANK_B}:8545 \
    -p ${P2P_PORT_CENTRAL_BANK_B}:30303 \
    -p ${P2P_PORT_CENTRAL_BANK_B}:30303/udp \
    --network "${NETWORK_NAME}" \
    --restart always \
    hyperledger/besu:latest \
    --data-path=data --genesis-file=genesis/genesis.json --min-gas-price=0 \
    --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT \
    --rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT \
    --host-allowlist='*' --rpc-http-cors-origins='all' \
    --rpc-http-host='0.0.0.0' --rpc-ws-host='0.0.0.0' \
    --rpc-http-port=8545 --rpc-ws-port=8546 --p2p-port=30303 $BESU_LOGGING

echo -e "${GREEN}central-bank-b node started.${NC}"
echo -e "${YELLOW}Waiting 5 seconds for central-bank-b node to be ready...${NC}"
sleep 5

# Fetch ENODE from central-bank-b node via host port
echo -e "${YELLOW}Fetching ENODE from central-bank-b node...${NC}"
max_retries=30
retry_delay=3
retry_count=0
ENODE=""

while [ $retry_count -lt $max_retries ]; do
    ENODE=$(curl -s -X POST \
        --data '{"jsonrpc":"2.0","method":"net_enode","params":[],"id":1}' \
        "http://127.0.0.1:${RPC_PORT_CENTRAL_BANK_B}" | jq -r '.result')
    if [ -n "$ENODE" ] && [ "$ENODE" != "null" ]; then
        echo -e "${GREEN}ENODE retrieved successfully.${NC}"
        break
    fi
    echo -e "${RED}Failed to retrieve ENODE. Retrying in $retry_delay seconds...${NC}"
    sleep $retry_delay
    ((retry_count++))
done

if [ $retry_count -eq $max_retries ]; then
    echo -e "${RED}Max retries reached. Unable to retrieve ENODE.${NC}"
    exit 1
fi

echo -e "${BLUE}ENODE: $ENODE${NC}\n"

# Get IP from the Besu-specific network only (avoids multi-network IP concatenation)
CENTRAL_BANK_B_IP=$(docker inspect \
    -f "{{(index .NetworkSettings.Networks \"${NETWORK_NAME}\").IPAddress}}" \
    "${NODE_CENTRAL_BANK_B}")
ENODE_INTERNAL=$(echo "$ENODE" | sed -e "s/127.0.0.1/${CENTRAL_BANK_B_IP}/g")
echo -e "${BLUE}ENODE (internal): $ENODE_INTERNAL${NC}\n"

# Now connect central-bank-b to shared network (after IP/ENODE capture to avoid multi-IP issue)
docker network connect cbweb3_network "${NODE_CENTRAL_BANK_B}" 2>/dev/null || true

# ─── Start bank-b node ───────────────────────────────────────────────────────
echo -e "${BLUE}Starting bank-b node...${NC}"
docker run -d \
    --name "${NODE_BANK_B}" \
    --user root \
    -v "$(pwd)/nodes/bank-b/data:/opt/besu/data" \
    -v "$(pwd)/genesis:/opt/besu/genesis" \
    -p ${RPC_PORT_BANK_B}:8545 \
    -p ${P2P_PORT_BANK_B}:30303 \
    -p ${P2P_PORT_BANK_B}:30303/udp \
    --network "${NETWORK_NAME}" \
    --restart always \
    hyperledger/besu:latest \
    --data-path=data --genesis-file=genesis/genesis.json --min-gas-price=0 \
    --bootnodes=${ENODE_INTERNAL} \
    --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT \
    --rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT \
    --host-allowlist='*' --rpc-http-cors-origins='all' \
    --rpc-http-host='0.0.0.0' --rpc-ws-host='0.0.0.0' \
    --rpc-http-port=8545 --rpc-ws-port=8546 --p2p-port=30303 $BESU_LOGGING

docker network connect cbweb3_network "${NODE_BANK_B}" 2>/dev/null || true
echo -e "${GREEN}bank-b node started.${NC}\n"

# ─── Start bank-d node ───────────────────────────────────────────────────────
echo -e "${BLUE}Starting bank-d node...${NC}"
docker run -d \
    --name "${NODE_BANK_D}" \
    --user root \
    -v "$(pwd)/nodes/bank-d/data:/opt/besu/data" \
    -v "$(pwd)/genesis:/opt/besu/genesis" \
    -p ${RPC_PORT_BANK_D}:8545 \
    -p ${P2P_PORT_BANK_D}:30303 \
    -p ${P2P_PORT_BANK_D}:30303/udp \
    --network "${NETWORK_NAME}" \
    --restart always \
    hyperledger/besu:latest \
    --data-path=data --genesis-file=genesis/genesis.json --min-gas-price=0 \
    --bootnodes=${ENODE_INTERNAL} \
    --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT \
    --rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT \
    --host-allowlist='*' --rpc-http-cors-origins='all' \
    --rpc-http-host='0.0.0.0' --rpc-ws-host='0.0.0.0' \
    --rpc-http-port=8545 --rpc-ws-port=8546 --p2p-port=30303 $BESU_LOGGING

docker network connect cbweb3_network "${NODE_BANK_D}" 2>/dev/null || true
echo -e "${GREEN}bank-d node started.${NC}\n"

echo -e "${YELLOW}Creating network tracker file...${NC}"
cat > .env.network <<EOF
NODES=$NODES
NETWORK_NAME=$NETWORK_NAME
CONTAINER_PREFIX=$CONTAINER_PREFIX
NODE_CENTRAL_BANK_B=$NODE_CENTRAL_BANK_B
NODE_BANK_B=$NODE_BANK_B
NODE_BANK_D=$NODE_BANK_D
RPC_PORT_CENTRAL_BANK_B=$RPC_PORT_CENTRAL_BANK_B
RPC_PORT_BANK_B=$RPC_PORT_BANK_B
RPC_PORT_BANK_D=$RPC_PORT_BANK_D
P2P_PORT_CENTRAL_BANK_B=$P2P_PORT_CENTRAL_BANK_B
P2P_PORT_BANK_B=$P2P_PORT_BANK_B
P2P_PORT_BANK_D=$P2P_PORT_BANK_D
ENODE=$ENODE_INTERNAL
EOF

echo -e "${GREEN}============================="
echo -e "Network started successfully!"
echo -e "=============================${NC}"
echo ""
echo -e "  ${BLUE}central-bank-b${NC}  RPC: http://localhost:${RPC_PORT_CENTRAL_BANK_B}  (${NODE_CENTRAL_BANK_B})"
echo -e "  ${BLUE}bank-b        ${NC}  RPC: http://localhost:${RPC_PORT_BANK_B}          (${NODE_BANK_B})"
echo -e "  ${BLUE}bank-d        ${NC}  RPC: http://localhost:${RPC_PORT_BANK_D}          (${NODE_BANK_D})"
echo ""
echo -e "  Network : ${NETWORK_NAME}"
echo -e "  ENODE   : ${ENODE_INTERNAL}"
echo ""
