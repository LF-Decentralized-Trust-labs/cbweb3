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

| Recurso | Padrão | Exemplo (`metadata.name: central-bank-brazil`, `ENTITY=central-bank`) |
|---|---|---|
| container | `${CONTAINER_PREFIX}-${ENTITY}-<papel>` | `sc-b-cbweb3-central-bank-brazil-central-bank-besu` |
| network | `${ENTITY_NET_PREFIX}_<rede>` | `central-bank-brazil_besu_network` |
| volume | `${ENTITY_VOLUME_PREFIX}_<papel>` | `central-bank-brazil_besu_data` |
| alias de proxy | `${ENTITY_NET_PREFIX}-<papel>` | `central-bank-brazil-governance` |

`ENTITY_NET_PREFIX` e `ENTITY_VOLUME_PREFIX` são **ambos** o `metadata.name` do manifesto passado
por `sanitizePrefix` (`engine/apply/apply.go`): minúsculas, e **espaço, underscore e barra viram
hífen**. Não há conversão para underscore em lado nenhum — o underscore que aparece nos exemplos
acima é o separador literal que o próprio template escreve depois do prefixo (`..._net`,
`..._besu_data`).

`ENTITY` é o **papel** da entidade (`central-bank`, `hub`, ou o id do banco), não o nome do
deployment. É por isso que o nome de container repete: prefixo (que já contém o nome) mais papel.

`CONTAINER_PREFIX` é `sc-b-cbweb3-` + o mesmo prefixo saneado.

Entidades com `metadata.name` distintos ⇒ prefixos distintos ⇒ sem colisão de container, rede ou
volume.

> **Limite de comprimento.** O alias de proxy é um rótulo DNS, que para em 63 octetos (RFC 1035).
> Como ele deriva do `metadata.name`, o nome tem um teto — validado em
> `engine/manifest` (`MaxMetadataNameLen`) e mantido em dia com o sufixo mais longo dos templates
> por `TestMaxMetadataNameLenMatchesTheLongestAlias`.

## Papéis de volume (estado)

`besu_data`, `genesis`, `config`, `tls`, `paladin_data` (no `entity-besu`); `pg_data` (infra);
`noc_db_data` (noc). Todos **named volumes**; a única exceção de bind mount de host é o `pki/` do
banco (chave + CSR do onboarding).

## Alcance cross-stack

Serviços que alcançam outro stack (ex.: backend → hub) usam
`extra_hosts: ["host.docker.internal:host-gateway"]` e a rota `host.docker.internal:${HUB_RPC_PORT}`.
