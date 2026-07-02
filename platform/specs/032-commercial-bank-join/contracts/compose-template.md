# Contrato: Template Compose TK-8 (commercial-bank)

**Feature**: `032-commercial-bank-join`
**Arquivo entregue**: `scenario-a/provisioning/templates/commercial-bank/docker-compose.yaml`

## Variáveis de Ambiente

### Obrigatórias (falha explícita do Compose se ausentes)

| Variável | Descrição | Exemplo |
|----------|-----------|---------|
| `SPOKE_ID` | ID do spoke | `spoke-brl` |
| `BANK_ID` | ID do banco comercial dentro do spoke | `commercial-bank-alpha` |
| `BESU_RPC_PORT` | Porta host para RPC HTTP (interno: 8545) | `8746` |
| `BESU_WS_PORT` | Porta host para RPC WS (interno: 8546) | `8756` |
| `BESU_P2P_PORT` | Porta host para P2P (interno: 30303) | `31403` |
| `BESU_IMAGE` | Imagem Docker do Besu | `hyperledger/besu:25.8.0` |
| `SPOKE_DATA_DIR` | Diretório base de dados do banco | `/var/cbweb3/commercial-bank-alpha` |
| `BESU_ADVERTISED_HOST` | Host anunciado no enode do banco | `cbweb3-spoke-brl-besu.commercial-bank-alpha` |
| `BOOTNODE_ENODE` | Enode completo do bootnode do spoke | `enode://abc...@host:31303` |

### Opcionais (com defaults)

| Variável | Default | Descrição |
|----------|---------|-----------|
| `SPOKE_NETWORK_NAME` | `cbweb3-${SPOKE_ID}-besu` | Nome da rede Docker (deve ser a mesma rede do spoke se cross-stack via DOCKER NAT) |
| `BESU_NAT_PROFILE` | `DOCKER` | `DOCKER` (cross-compose local) ou `NONE` (prod com host explícito) |
| `BESU_LOGGING` | `INFO` | Nível de log do Besu |
| `BESU_HOST_ALLOWLIST` | `*` | Allowlist de hosts para RPC |
| `BESU_CORS_ORIGINS` | `all` | CORS origins para RPC HTTP |

## Diferenças em relação ao TK-4 (central-bank)

1. **Sem `genesis-init` service**: O genesis é provido via bind mount de `SPOKE_DATA_DIR/genesis/genesis.json`. O motor TK-9 escreve e verifica o genesis ANTES de chamar `docker compose up`.

2. **`BOOTNODE_ENODE` é obrigatório**: O `docker-compose.yaml` usa `${BOOTNODE_ENODE?BOOTNODE_ENODE is required}` — falha explícita se vazio.

3. **Node data path**: `${SPOKE_DATA_DIR}/nodes/commercial-bank/data` (em vez de `central-bank/data`).

4. **Container name**: `cbweb3-${SPOKE_ID}-besu.${BANK_ID}` (parametrizado por banco).

5. **`entry.sh` reutilizado sem alteração**: Mesmo script de `central-bank/scripts/entry.sh` (DOCKER/NONE NAT profile, ADR-001 D1).

## Serviços

```
services:
  besu:            # Besu joiner — non-validator até o voto QBFT
  paladin-data-init:  # Init container de permissões (padrão SP-02)
  paladin-bank:    # Paladin do banco comercial (padrão bank-x do SP-02)
```

Sem `genesis-init` service.

## Invariantes

- Nenhum arquivo de genesis é gerado ou sobrescrito pelo compose
- Nenhuma chave privada é gerada pelo compose (diferente do TK-4 que gera validator keys no genesis-init)
- O compose é `restart: always` para `besu`; `restart: "no"` para `paladin-data-init`
