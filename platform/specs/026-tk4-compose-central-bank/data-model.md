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

## Estrutura de diretórios em `SPOKE_DATA_DIR` (histórico) → volumes Docker (atual)

O desenho original desta seção descrevia `${SPOKE_DATA_DIR}/config/qbftConfigFile.json`
e `${SPOKE_DATA_DIR}/genesis/genesis.json` como bind mounts renderizados pelo TK-5
antes do `docker compose up`. Isso não é mais verdade — ver os dois addenda abaixo.
O template `central-bank/docker-compose.yaml` não referencia `SPOKE_DATA_DIR` em
nenhum mount.

> **Desvio de design 1 (pós-implementação)**: `nodes/central-bank/data/` (chave do
> validador `key`/`key.pub` + banco RocksDB do Besu) passou a ser o volume Docker
> nomeado `besu_data` (`${SPOKE_ID}_cb_besu_data`), montado em `/opt/besu/data` no
> serviço `besu`. Motivo: o chain data do Besu não precisa ser inspecionado/portado
> manualmente pelo host — segue o mesmo padrão já usado para o Paladin (`cb_data`) e
> o Postgres (`pg_data`).
>
> **Desvio de design 2 (pós-implementação)**: `config/qbftConfigFile.json` e
> `genesis/genesis.json` também deixaram de ser bind mounts. `qbftConfigFile.json` é
> semeado pelo motor (`step_start_besu_found.go`, via `engine/dockervolume` — pipe
> direto de memória, sem tocar disco do host) no volume nomeado `cb_config`.
> `genesis.json` é gravado pelo próprio `genesis-init` diretamente no volume nomeado
> `cb_genesis` (nunca precisou de escrita pelo host em modo `found`). A única
> releitura pelo host (o `EmitBundle`/TK-6) passou a usar
> `engine/dockervolume.ReadFile` sobre `cb_genesis` em vez de ler
> `SPOKE_DATA_DIR/genesis/genesis.json`. O scratch intermediário do `genesis-init`
> (`networkFiles/`, nunca lido fora da própria execução do container) também virou
> volume nomeado (`cb_scratch`).
>
> Um serviço `besu-volumes-init` (mesma receita do `paladin-data-init`) garante que
> `besu_data` e `cb_genesis` — criados root-owned — sejam graváveis pelo
> `genesis-init` (que roda como `HOST_UID:HOST_GID`). `cb_config` não precisa desse
> tratamento: só é lido, nunca escrito, pelo `genesis-init`.
>
> Ver `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`,
> `scenario-a/toolkit/engine/dockervolume/` e
> `scenario-a/toolkit/engine/bundle/bundle.go` (`readGenesis`).

**Invariante**: O `genesis-init` não depende de nenhum diretório do host. Ele assume
que os volumes nomeados `cb_config`, `cb_genesis`, `cb_scratch` e `besu_data` já
existem (criados pelo `docker compose up`; `besu_data`/`cb_genesis` inicializados
pelo `besu-volumes-init`) e que `cb_config/qbftConfigFile.json` já foi semeado pelo
motor antes do `up`.

---

## Serviços do Compose (topologia de contêineres)

```text
docker-compose.yaml (central-bank template)
  services:
    besu-data-init        restart: no   → chmod no volume besu_data recém-criado; encerra
    genesis-init          restart: no   → verifica/gera genesis; encerra
    besu                  restart: always → nó Besu; inicia após genesis-init OK
  networks:
    ${SPOKE_NETWORK_NAME} driver: bridge → rede isolada por spoke
  volumes:
    config/ e genesis/    → bind mounts via SPOKE_DATA_DIR
    besu_data             → volume Docker nomeado (desvio; ver nota acima)
```

---

## Diagrama de dependência e sequência

```
Motor TK-5
  │
  ├─► renderiza qbftConfigFile.json → ${SPOKE_DATA_DIR}/config/
  ├─► cria estrutura de diretórios em SPOKE_DATA_DIR (config/, genesis/)
  └─► docker compose up
         │
         ├─► besu-data-init (restart: no) → chmod 777 no volume besu_data
         │
         ├─► genesis-init (depends_on besu-data-init; restart: no)
         │     ├─[genesis existe?]─ sim → exit 0
         │     └─[genesis existe?]─ não → besu operator generate → exit 0
         │                              → grava key/key.pub em besu_data
         │
         └─► besu (depends_on genesis-init: service_completed_successfully)
               ├─► monta genesis/genesis.json (read-only, bind mount)
               ├─► monta besu_data em /opt/besu/data (read-write, volume nomeado)
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
