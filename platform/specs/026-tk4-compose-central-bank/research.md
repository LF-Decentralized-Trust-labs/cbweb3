# Research: TK-4 — Template Compose central-bank

## 1. Init container pattern no Docker Compose v2

**Decisão**: Usar serviço de init (`genesis-init`) com `restart: no` + `depends_on` com `condition: service_completed_successfully`.

**Rationale**: O Docker Compose v2 suporta nativamente o padrão de init container via `condition: service_completed_successfully`. O serviço init encerra com código 0 após verificar/gerar o genesis; o serviço principal (`besu`) só inicia após isso. Não há necessidade de polling manual nem de sidecars.

**Alternativas consideradas**:
- **Entrypoint único no serviço besu**: Um script seria o entrypoint do próprio nó Besu verificando genesis antes de iniciar. Rejeitado porque o binário `besu` não é o responsável por genesis gerado por `besu operator generate-blockchain-config` no contexto do Compose — este subcomando gera artefatos de rede completos (chaves + genesis para múltiplos validadores). Misturar geração e execução em um único serviço viola o princípio de responsabilidade única e dificulta o restart seletivo.
- **Volume com pré-condicionamento no host**: Gerar o genesis no host antes de subir o Compose. Rejeitado porque o motor de orquestração (TK-5) precisaria de `besu` instalado no host, acoplando o toolkit ao runtime do host.

**Como implementar**:
```yaml
genesis-init:
  image: ${BESU_IMAGE}
  restart: no
  volumes:
    - ${SPOKE_DATA_DIR}/genesis:/genesis
    - ${SPOKE_DATA_DIR}/config:/config
    - ${SPOKE_DATA_DIR}/nodes:/nodes
  command: >
    sh -c '
      if [ -f /genesis/genesis.json ]; then
        echo "[genesis-init] genesis.json already exists, skipping generation.";
        exit 0;
      fi;
      echo "[genesis-init] Generating genesis...";
      /opt/besu/bin/besu operator generate-blockchain-config \
        --config-file=/config/qbftConfigFile.json \
        --to=/nodes/networkFiles \
        --private-key-file-name=key;
      cp /nodes/networkFiles/genesis.json /genesis/genesis.json;
      echo "[genesis-init] Genesis generated successfully.";
    '

besu:
  depends_on:
    genesis-init:
      condition: service_completed_successfully
```

---

## 2. Flags Besu para host P2P anunciado

**Decisão**: Usar `--p2p-host=${BESU_ADVERTISED_HOST}` no comando do serviço `besu`.

**Rationale**: O flag `--p2p-host` controla qual IP/hostname é incluído no enode publicado. Sem ele, o Besu usa o IP da interface de rede do contêiner (ex: `172.17.0.x`), que não é roteável fora do host Docker. Definir `--p2p-host` com o DNS name do contêiner ou com o hostname externo garante que o enode retornado por `net_enode` seja utilizável de outros hosts.

**Verificado no código existente**: O `startBesu.sh` (linha 198) faz manualmente `ENODE_INTERNAL=$(echo "$ENODE" | sed ... ${CENTRAL_BANK_IP}/g)` porque não usa `--p2p-host`. O template elimina esse workaround usando o flag nativo.

**Flags completos para o nó bootnode (mode: found)**:
```
--data-path=/data
--genesis-file=/genesis/genesis.json
--min-gas-price=0
--rpc-http-enabled --rpc-http-api=ETH,NET,QBFT
--rpc-http-host=0.0.0.0 --rpc-http-port=8545
--rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT
--rpc-ws-host=0.0.0.0 --rpc-ws-port=8546
--p2p-port=30303
--p2p-host=${BESU_ADVERTISED_HOST}
--host-allowlist=* --rpc-http-cors-origins=all
```

**Nota**: `--bootnodes` é omitido quando `BOOTNODE_ENODE` não está definido. O template usa a sintaxe:
```yaml
command: >
  /opt/besu/bin/besu
  --bootnodes=${BOOTNODE_ENODE:-}
```
Besu ignora `--bootnodes=` vazio (string vazia) — comportamento documentado, validado na versão 25.8.0.

---

## 3. Health check via RPC HTTP

**Decisão**: Health check usando `curl` com `eth_blockNumber`.

**Rationale**: O Besu expõe um endpoint HTTP no mesmo contêiner. A verificação via `curl -sf -X POST http://localhost:8545 -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'` confirma que o nó está aceito conexões e processando RPC. A imagem `hyperledger/besu` inclui `curl` ou pode-se usar o binário Java `wget` nativo.

**Implementação**:
```yaml
healthcheck:
  test: ["CMD", "sh", "-c",
    "curl -sf -X POST http://localhost:8545 \
     -H 'Content-Type: application/json' \
     -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}' | grep -q result"]
  interval: 5s
  timeout: 3s
  retries: 20
  start_period: 10s
```

**Alternativas consideradas**: Usar `/opt/besu/bin/besu --version` (smoke test). Rejeitado — não verifica que o daemon está respondendo, apenas que o binário existe.

---

## 4. Estrutura do `qbftConfigFile.json` e `configTemplate.json`

**Decisão**: O `genesis-init` espera um `qbftConfigFile.json` pré-gerado pelo motor de orquestração (TK-5) e montado em `${SPOKE_DATA_DIR}/config/qbftConfigFile.json`.

**Rationale**: O `startBesu.sh` existente usa `jq` para derivar o `qbftConfigFile.json` a partir de um `configTemplate.json` com o número de nós injetado. Essa lógica pertence ao motor de orquestração (TK-5), não ao template Compose. O template assume que o arquivo de configuração QBFT já está disponível no volume antes de `docker compose up` ser chamado.

**Implicação para TK-5**: O motor deve renderizar o `qbftConfigFile.json` (com o `chainId`, bloco genesis, configuração QBFT e número de validadores) antes de invocar o template.

---

## 5. Rede Docker parametrizada vs. rede compartilhada

**Decisão**: Cada spoke usa uma rede Docker dedicada nomeada `${SPOKE_NETWORK_NAME}` (ex: `cbweb3-spoke-brl-besu`). A rede é definida no próprio template com `driver: bridge`. Não existe dependência da rede `cbweb3_network` compartilhada do sample existente.

**Rationale**: O `startBesu.sh` cria `spoke_a_besu_network` (dedicada) e também conecta os contêineres à `cbweb3_network` compartilhada. Para o toolkit, a conectividade com os backends ocorre via o motor de orquestração que passa as variáveis corretas — a rede Besu do spoke é isolada. A `cbweb3_network` compartilhada é um artefato do sample de referência e não deve ser replicada no template.

**Alternativas consideradas**: Usar `network_mode: host`. Rejeitado — não é portável para ambientes de produção (EKS, etc.) e elimina o isolamento de rede entre spokes.

---

## 6. Localização do template no repositório

**Decisão**: `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`

**Rationale**: O design do toolkit (concat.md §4) especifica que o toolkit é novo e autônomo, sem modificar `deploy/local/`. Criar o template sob `scenario-a/provisioning/` mantém o isolamento por Scenario (Princípio I da Constitution) e estabelece a estrutura de diretórios do toolkit.

```text
scenario-a/provisioning/
  templates/
    central-bank/
      docker-compose.yaml          ← este artefato (TK-4)
      scripts/
        genesis-guard.sh           ← script de guarda (alternativa ao inline command)
  schema/                          ← TK-1 (existente: 023-manifest-schema-validation)
  engine/                          ← TK-5 (futuro)
    certsource/                    ← TK-3 (existente: 025-tk3-certsource-interface)
    keyprovider/                   ← TK-2 (existente: 024-tk2-keyprovider-interface)
```

---

## 7. Interação com KeyProvider (TK-2) e CertSource (TK-3)

**Decisão**: O template Compose NÃO integra diretamente KeyProvider nem CertSource. Essas interfaces são consumidas pelo motor de orquestração (TK-5) antes de invocar o Compose.

**Rationale**: A section 5.3 do design (concat.md) é explícita: "The template provides container topology only." A geração de chaves (TK-2) e a emissão de certs (TK-3) são orquestradas pelo motor antes de `docker compose up`. O motor passa caminhos de cert/TLS como variáveis para o template montar volumes.

**O que o template recebe**: variáveis de caminho como `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CA_CERT_FILE` (opcionais para mTLS, desativado por padrão no perfil local) — não objetos de interface Go.
