# Quickstart: TK-4 — Template Compose central-bank

## Pré-requisitos

- Docker Engine 24+ com `docker compose` plugin v2
- `curl`, `jq`, `sha256sum` disponíveis no PATH
- Estrutura de diretórios do spoke criada (responsabilidade do motor TK-5 em produção)

## Execução mínima (desenvolvimento local)

```bash
# 1. Cria estrutura de dados do spoke
DATA_DIR="$(pwd)/.spoke-brl-data"
mkdir -p "$DATA_DIR/config" "$DATA_DIR/genesis" "$DATA_DIR/nodes/central-bank/data"

# 2. Coloca o qbftConfigFile.json (renderizado pelo motor TK-5 em produção)
cp scenario-a/provisioning/templates/central-bank/examples/qbftConfigFile.json \
   "$DATA_DIR/config/qbftConfigFile.json"

# 3. Sobe o spoke (1a execucao: gera genesis)
export SPOKE_ID=spoke-brl \
       BESU_ADVERTISED_HOST=cbweb3-spoke-brl-besu.central-bank-brazil \
       BESU_RPC_PORT=8645 BESU_WS_PORT=8655 BESU_P2P_PORT=31303 \
       BESU_IMAGE=hyperledger/besu:25.8.0 \
       SPOKE_DATA_DIR="$DATA_DIR"

docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml \
  --project-name spoke-brl up -d

# 4. Aguarda o no responder
until curl -sf -X POST http://localhost:8645 \
     -H 'Content-Type: application/json' \
     -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
     | grep -q result; do sleep 2; done
echo "No Besu pronto."

# 5. Obtem o enode (para o join bundle)
curl -s -X POST http://localhost:8645 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"net_enode","params":[],"id":1}' | jq -r '.result'
```

## Reinicialização (idempotente)

```bash
docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml \
  --project-name spoke-brl down

# genesis-init detecta genesis existente e encerra sem regenerar
docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml \
  --project-name spoke-brl up -d
```

## Executar testes de integração

```bash
bash scenario-a/provisioning/tests/test-central-bank-template.sh
```

## Parar o spoke

```bash
docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml \
  --project-name spoke-brl down
```
