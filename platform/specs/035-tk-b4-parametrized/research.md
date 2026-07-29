# Research — TK-B4 (templates de compose parametrizados)

Fase 0. Decisões que resolvem o Technical Context. Formato: Decisão / Rationale / Alternativas.

## R1 — Fonte de derivação de cada template

- **Decisão**: derivar por reescrita (não copiar):
  - `entity-infra`, `entity-keycloak` ← `deploy/local/compose.yml` (postgres, redis, keycloak).
  - `entity-backend`, `entity-frontend` ← composes/serviços de backend/frontend do cenário.
  - `hub` e o Besu de entidade ← **`deploy/local/*-besu/startBesu.sh`** (hoje `docker run` +
    bind mount de host `nodes/<entidade>/data`), convertidos em serviços compose com named volumes.
  - `relay` ← compose do relay em `interop/hub-and-spoke`.
  - `noc` ← `deploy/local/compose.noc.yml` (noc-db/backend/portal + noc-agent).
- **Rationale**: o roadmap §7 exige templates **derivados** dos composes atuais; os nós Besu não
  têm compose hoje (sobem por script imperativo), então seus templates são net-new mas fiéis ao
  comportamento do script.
- **Alternativas**: mover/editar os composes de `deploy/local` — **proibido** (FR-003, restrição
  rígida do roadmap).

## R2 — Estratégia de validação (sem subir containers)

- **Decisão**: validação **primária estática em Go** (parse YAML com `yaml.v3`), cobrindo
  SC-001..006: interpolação completa contra o contrato de variáveis, named volumes, ausência de
  bind mount host indevido, ausência de segredos e ausência de colisão cross-entidade. Uma
  verificação **opcional** roda `docker compose config` quando o Docker está presente
  (`t.Skip` caso contrário) para a checagem semântica canônica do compose.
- **Rationale**: `docker compose config` sozinho **não** verifica named-volume-vs-bind, colisão
  entre entidades nem segredos; e exige Docker no CI. O parse estático cobre todas as regras, é
  determinístico e não adiciona dependências (yaml.v3 já está no módulo).
- **Alternativas**: só `docker compose config` (não cobre SC-002/003/006 e exige Docker); script
  bash (menos testável, foge do padrão `go test` do toolkit) — rejeitadas.

## R3 — Named volumes determinísticos por entidade

- **Decisão**: cada volume de estado é declarado com nome derivado do identificador de
  entidade/spoke, via variável: `volumes: <lógico>: { name: "${ENTITY_VOLUME_PREFIX}_<papel>" }`,
  com papéis `besu_data`, `genesis`, `config`, `tls`, `pg_data`, `paladin_data`, `noc_db_data`.
- **Rationale**: estratégia do toolkit de referência (roadmap §7); nomes determinísticos permitem
  o hand-off por bundle (o emissor lê genesis/enode do volume) e evitam colisão entre entidades.
- **Alternativas**: bind mounts de host (`nodes/<entidade>/data`, uso atual) — substituídos, exceto
  o `pki/` (R6).

## R4 — Esquema de porta por offset (convenção documentada)

- **Decisão**: documentar em `vars/NAMING.md` um esquema **determinístico por offset**: cada
  entidade recebe uma banda a partir de um `PORT_OFFSET` (ex.: RPC = `8845 + offset`,
  WS = `8846 + offset`, P2P = `30303 + offset`, gateway/backend por faixa análoga). Os templates
  apenas **consomem** as variáveis de porta; o **cálculo** do offset é do motor (TK-B6).
- **Rationale**: a escolha do usuário excluiu derivação em código do TK-B4; a convenção precisa
  existir e ser consultável (FR-006/FR-011) para o motor e para a validação cross-entidade.
- **Alternativas**: portas fixas por entidade nomeada (hoje `RPC_PORT_CENTRAL_BANK_A`, …) — não
  escala para N entidades dinâmicas; rejeitada.

## R5 — Alcance cross-stack

- **Decisão**: `extra_hosts: ["host.docker.internal:host-gateway"]` nos serviços que precisam
  alcançar outro stack; a rota backend→hub usa `host.docker.internal:${HUB_RPC_PORT}` via variável.
- **Rationale**: stacks por entidade são compose files independentes; o gateway do host é o ponto
  de encontro (roadmap §7, "backends → hub em :8845").
- **Alternativas**: rede Docker externa compartilhada entre todos os stacks — mais acoplado e
  frágil para N entidades; a rede externa `cbweb3_network` permanece só onde já é usada.

## R6 — Exceção de bind mount: `pki/` do banco

- **Decisão**: o único bind mount de host permitido é `<dataDir>/pki` (chave + CSR do banco),
  necessário ao host para o fluxo de onboarding/KYC; declarado explicitamente e reconhecido pela
  validação como exceção legítima.
- **Rationale**: FR-005 e roadmap §7/§15; o restante do estado do nó vai para named volumes.
- **Alternativas**: pki também em volume — quebraria o acesso do host ao material de CSR no
  onboarding; rejeitada.

**Saída**: nenhuma `NEEDS CLARIFICATION` remanescente.
