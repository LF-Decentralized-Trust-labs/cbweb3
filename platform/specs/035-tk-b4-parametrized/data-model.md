# Data Model — TK-B4 (templates de compose parametrizados)

Fase 1. Entidades, regras de validação e estados. Não há persistência (assets em disco + validação
stateless).

## ComposeTemplate

Um arquivo de composição parametrizado sob `provisioning/templates/`.

- **Campos**: `path`, `services[]`, `networks[]`, `volumes[]`, `referencedVars[]` (todos os
  `${VAR}` citados).
- **Conjunto**: `hub`, `entity-besu`, `entity-infra`, `entity-keycloak`, `entity-backend`,
  `entity-frontend`, `relay`, `noc`. O `entity-besu` é o nó Besu (+Paladin) do spoke/banco e o lar
  do estado `besu_data`/`genesis`/`config`/`tls`/`paladin_data`.
- **Regras**:
  - Todo valor discriminante (container_name, ports, chain id, image, network/volume name) referencia
    `${VAR}` (FR-002/FR-010).
  - Nenhum identificador fixo de entidade (`spoke-a`/`spoke-b`) embutido (FR-010).
  - Nenhum literal de segredo (chave privada PEM, senha, cert) (FR-009).

## VariableContract

O conjunto de variáveis que um template exige (`vars/<template>.env.example`).

- **Campos**: `template`, `required[]`, `optional[] (com default)`.
- **Regras**:
  - `required` = todo `${VAR}` sem default no template (`${VAR:?}` ou sem `:-`).
  - `optional` = `${VAR:-default}` (apenas itens **não** discriminantes, ex.: tag de imagem).
  - O `.env.example` cobre 100% das variáveis obrigatórias do template (SC-001/SC-005).

## NamingScheme (convenção documentada — `vars/NAMING.md`)

Convenção determinística que o **motor** aplica; o TK-B4 só a documenta e a validação a usa para
montar envs de exemplo.

- **Portas**: `RPC = 8845 + offset`, `WS = 8846 + offset`, `P2P = 30303 + offset`, bandas análogas
  para gateway/backend/frontend, onde `offset` é atribuído por entidade.
- **Nomes**: `container = <prefix>-<entity>-<papel>`, `network = <entity>_<rede>`,
  `volume = <ENTITY_VOLUME_PREFIX>_<papel>`.
- **Regra**: entidades distintas ⇒ offsets/prefixos distintos ⇒ sem colisão (SC-002).

## VolumeSet

Os named volumes determinísticos de estado por entidade.

- **Papéis**: `besu_data`, `genesis`, `config`, `tls`, `pg_data`, `paladin_data`, `noc_db_data`.
- **Regra**: todos são **named volumes** (`name: ${ENTITY_VOLUME_PREFIX}_<papel>`); **nenhum** bind
  mount de host, **exceto** `pki/` do banco (FR-004/FR-005).

## ValidationResult (saída da validação Go)

- **Campos**: `template`, `ok bool`, `errors[] {rule, detail}`.
- **Regras (uma por SC)**:
  - `interpolation` — todo `${VAR}` obrigatório presente no env; nenhum placeholder residual
    (SC-001); variável obrigatória ausente ⇒ erro (SC-005).
  - `named-volumes` — estado em named volume; sem bind host exceto `pki/` (SC-003).
  - `no-secrets` — nenhum literal sensível (SC-006).
  - `no-collision` — dois envs de entidades distintas ⇒ 0 colisão de porta/nome/rede/volume
    (SC-002).
  - `deploy-local-untouched` — nenhum arquivo de `deploy/local` alterado (SC-004; verificação de
    repositório, não do template).

## Transições de estado

Nenhuma. Assets estáticos; a validação é uma função pura `(template, env) → ValidationResult`.
