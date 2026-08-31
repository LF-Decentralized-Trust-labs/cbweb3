# Convenção determinística de portas e nomes (TK-B4)

Convenção que o **motor** (TK-B6) aplica ao preencher as variáveis dos templates. O TK-B4 apenas a
**documenta** e a **valida** — não calcula offsets/nomes em código (fica no motor).

## Índice e offset por entidade

Cada entidade (hub, ou um `<spoke>_<entidade>`) recebe um **índice** inteiro `i ≥ 0`. O hub usa
`i = 0`. O offset de porta é `offset = i * STRIDE`, com `STRIDE = 10`.

## Portas (host)

| Recurso | Fórmula (porta host) | Porta interna |
|---|---|---|
| Besu RPC HTTP | `8845 + offset` | 8545 |
| Besu RPC WS | `8846 + offset` | 8546 |
| Besu P2P | `30303 + offset` | 30303 |
| API Gateway | `8080 + offset` | 8080 |
| Frontend | `3000 + offset` | 3000 |
| Postgres | `5432 + offset` | 5432 |
| Redis | `6379 + offset` | 6379 |
| Keycloak | `8081 + offset` | 8080 |
| Paladin RPC | `8548 + offset` | 8548 |

Entidades com índices distintos ⇒ offsets distintos ⇒ **sem colisão de porta**.

## Nomes

| Recurso | Padrão | Exemplo (`ENTITY=bank-a`) |
|---|---|---|
| container | `${CONTAINER_PREFIX}-${ENTITY}-<papel>` | `cbweb3-b-bank-a-besu` |
| network | `${ENTITY_NET_PREFIX}_<rede>` | `bank_a_besu_network` |
| volume | `${ENTITY_VOLUME_PREFIX}_<papel>` | `bank_a_besu_data` |

`ENTITY_VOLUME_PREFIX` e `ENTITY_NET_PREFIX` derivam de `ENTITY` (hífens → underscores) prefixados
pelo spoke quando aplicável (`<spoke>_<entidade>`). Entidades distintas ⇒ prefixos distintos ⇒ sem
colisão de nome de container/rede/volume.

## Papéis de volume (estado)

`besu_data`, `genesis`, `config`, `tls`, `paladin_data` (no `entity-besu`); `pg_data` (infra);
`noc_db_data` (noc). Todos **named volumes**; a única exceção de bind mount de host é o `pki/` do
banco (chave + CSR do onboarding).

## Alcance cross-stack

Serviços que alcançam outro stack (ex.: backend → hub) usam
`extra_hosts: ["host.docker.internal:host-gateway"]` e a rota `host.docker.internal:${HUB_RPC_PORT}`.
