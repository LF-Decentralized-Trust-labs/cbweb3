# Data Model: TK-4 — Template Compose central-bank

TK-4 não introduz entidades de banco de dados nem modelos Go. Os artefatos são arquivos de configuração e scripts. Este documento descreve a estrutura de dados do template e as variáveis de contrato com o motor de orquestração (TK-5).

---

## Variáveis de ambiente do template (contrato com TK-5)

O motor de orquestração renderiza essas variáveis antes de invocar `docker compose up -f <template>`.

### Obrigatórias (sem valor padrão — falha explícita se ausente)

| Variável | Tipo | Exemplo | Descrição |
|---|---|---|---|
| `SPOKE_ID` | string | `spoke-brl` | Identificador do spoke; usado em nomes de contêiner e rede |
| `BESU_ADVERTISED_HOST` | string | `cbweb3-spoke-brl-besu.central-bank-brazil` | Host anunciado no enode (`--p2p-host`); nunca inferido |
| `BESU_RPC_PORT` | int | `8645` | Porta host mapeada para RPC HTTP (interno: 8545) |
| `BESU_WS_PORT` | int | `8655` | Porta host mapeada para RPC WS (interno: 8546) |
| `BESU_P2P_PORT` | int | `31303` | Porta host mapeada para P2P (interno: 30303) |
| `BESU_IMAGE` | string | `hyperledger/besu:25.8.0` | Imagem Docker do Besu; nunca `latest` |
| `SPOKE_DATA_DIR` | path | `/data/spokes/spoke-brl` | Diretório base para volumes de dados do spoke |
| `HOST_UID` | int | `$(id -u)` | UID do usuário host; genesis-init roda com este UID para evitar arquivos owned por root em `SPOKE_DATA_DIR` |
| `HOST_GID` | int | `$(id -g)` | GID do grupo host; análogo ao `HOST_UID` |

### Opcionais (com valor padrão)

| Variável | Padrão | Descrição |
|---|---|---|
| `BOOTNODE_ENODE` | `` (vazio) | Enode do bootnode; vazio = este nó é o bootnode (`mode: found`) |
| `SPOKE_NETWORK_NAME` | `cbweb3-${SPOKE_ID}-besu` | Nome da rede Docker dedicada ao spoke |
| `BESU_CONTAINER_NAME` | `cbweb3-${SPOKE_ID}-besu.central-bank` | Nome do contêiner Besu |
| `BESU_LOGGING` | `INFO` | Nível de log do Besu |
| `TLS_CERT_FILE` | `` (vazio) | Caminho para cert TLS (mTLS; desativado se vazio) |
| `TLS_KEY_FILE` | `` (vazio) | Caminho para chave TLS (mTLS; desativado se vazio) |
| `CA_CERT_FILE` | `` (vazio) | Caminho para CA cert (mTLS; desativado se vazio) |

---

## Estrutura de diretórios em `SPOKE_DATA_DIR`

O motor de orquestração (TK-5) garante que esta estrutura exista antes de invocar o Compose.

```text
${SPOKE_DATA_DIR}/
  config/
    qbftConfigFile.json     ← renderizado por TK-5 antes do `docker compose up`
  genesis/
    genesis.json            ← criado pelo genesis-init na 1ª execução; nunca sobrescrito
  nodes/
    central-bank/
      data/
        key                 ← chave privada do validador (gerada pelo genesis-init)
        key.pub             ← chave pública correspondente
```

**Invariante**: O `genesis-init` NÃO cria o diretório `${SPOKE_DATA_DIR}`. Ele assume que os subdiretórios `config/`, `genesis/`, `nodes/central-bank/data/` já existem com as permissões corretas.

---

## Serviços do Compose (topologia de contêineres)

```text
docker-compose.yaml (central-bank template)
  services:
    genesis-init          restart: no   → verifica/gera genesis; encerra
    besu                  restart: always → nó Besu; inicia após genesis-init OK
  networks:
    ${SPOKE_NETWORK_NAME} driver: bridge → rede isolada por spoke
  volumes:
    (bind mounts via SPOKE_DATA_DIR; sem named volumes)
```

---

## Diagrama de dependência e sequência

```
Motor TK-5
  │
  ├─► renderiza qbftConfigFile.json → ${SPOKE_DATA_DIR}/config/
  ├─► cria estrutura de diretórios em SPOKE_DATA_DIR
  └─► docker compose up
         │
         ├─► genesis-init (restart: no)
         │     ├─[genesis existe?]─ sim → exit 0
         │     └─[genesis existe?]─ não → besu operator generate → exit 0
         │
         └─► besu (depends_on genesis-init: service_completed_successfully)
               ├─► monta genesis/genesis.json (read-only)
               ├─► monta nodes/central-bank/data/ (read-write)
               └─► inicia daemon com --p2p-host=${BESU_ADVERTISED_HOST}
```

---

## Arquivo de saída do template (emitido pelo motor TK-5 após `docker compose up`)

O motor TK-5 interroga o nó via RPC e produz o seguinte output para alimentar o join bundle (TK-6):

| Campo | Fonte | Exemplo |
|---|---|---|
| `enode` | `net_enode` via RPC | `enode://abc...@cbweb3-spoke-brl-besu.central-bank-brazil:31303` |
| `genesis_hash` | `eth_getBlockByNumber("0x0")` | `0x...` |
| `chain_id` | `eth_chainId` | `1337` |
| `rpc_endpoint` | variável `BESU_RPC_PORT` | `http://cbweb3-spoke-brl-besu.central-bank-brazil:8645` |
