# Contracts — TK-B4 (contratos de variáveis + convenção de nomes/portas)

Fase 1. O "contrato" de cada template é o conjunto de variáveis que ele exige e a convenção
determinística de nomes/portas. Os arquivos vivos ficam em `provisioning/templates/vars/`.

## Convenção de nomes/portas (NAMING.md)

Aplicada pelo **motor** (TK-B6); documentada aqui e usada pela validação para montar envs.

| Recurso | Fórmula | Exemplo (offset=10) |
|---|---|---|
| RPC (host) | `8845 + offset` | 8855 |
| WS (host) | `8846 + offset` | 8856 |
| P2P (host) | `30303 + offset` | 30313 |
| Gateway/backend (host) | faixa por offset | (por entidade) |
| container | `<PREFIX>-<ENTITY>-<papel>` | `cbweb3-bank-a-besu` |
| network | `<ENTITY>_<rede>` | `bank_a_besu_network` |
| volume | `<ENTITY_VOLUME_PREFIX>_<papel>` | `bank_a_besu_data` |

**Invariante**: entidades distintas ⇒ `offset`/`ENTITY_VOLUME_PREFIX` distintos ⇒ sem colisão.

## Contrato por template (variáveis obrigatórias — resumo)

Cada `vars/<template>.env.example` declara o conjunto completo; abaixo o essencial obrigatório.

| Template | Variáveis obrigatórias (essenciais) |
|---|---|
| `hub` | `HUB_CONTAINER_PREFIX`, `HUB_CHAIN_ID`, `HUB_RPC_PORT`, `HUB_WS_PORT`, `HUB_P2P_PORT`, `HUB_VOLUME_PREFIX`, `BESU_IMAGE` |
| `entity-besu` | `ENTITY`, `ENTITY_VOLUME_PREFIX`, `SPOKE_CHAIN_ID`, `ENTITY_RPC_PORT`, `ENTITY_WS_PORT`, `ENTITY_P2P_PORT`, `BOOTNODE_ENODE`, `BESU_IMAGE`, `PALADIN_IMAGE` |
| `entity-infra` | `ENTITY`, `ENTITY_VOLUME_PREFIX`, `POSTGRES_PORT`, `REDIS_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD` |
| `entity-keycloak` | `ENTITY`, `KEYCLOAK_PORT`, `KC_DB_URL`, `POSTGRES_USER`, `POSTGRES_PASSWORD` |
| `entity-backend` | `ENTITY`, `ENTITY_BESU_RPC_PORT`, `HUB_RPC_PORT` (via host-gateway), `GATEWAY_PORT`, `ENTITY_VOLUME_PREFIX` |
| `entity-frontend` | `ENTITY`, `FRONTEND_PORT`, `GATEWAY_URL` |
| `relay` | `RELAY_CONTAINER_NAME`, `RELAY_PORT`, `RELAY_VOLUME_PREFIX` (sem spokes fixos) |
| `noc` | `NOC_DB_VOLUME`, `NOC_BACKEND_PORT`, `NOC_PORTAL_PORT`, `NOC_AGENT_ENTITY`, `NOC_AGENT_BESU_RPC` |

**Regras de contrato**:
- Toda variável **obrigatória** usa `${VAR:?}` (falha se ausente) — nenhum default silencioso para
  valor discriminante (FR-002, SC-005).
- Variável **opcional** (`${VAR:-default}`) só para itens não discriminantes (ex.: `BESU_IMAGE`,
  tags de imagem).
- Cross-stack: `entity-backend` alcança o hub por `host.docker.internal:${HUB_RPC_PORT}` (FR-007).
- Volumes de estado: named (`name: ${..._VOLUME_PREFIX}_<papel>`); única exceção de bind host =
  `pki/` do banco (FR-004/FR-005).

## Contrato da validação (Go — `engine/composetemplate`)

```go
// Template carregado de um arquivo YAML de compose.
type Template struct { Path string; RefVars []string; /* services/networks/volumes */ }

func Load(path string) (*Template, error)

// Validate confere o template contra um env e as regras (interpolação, volumes,
// segredos). Retorna um resultado com os erros por regra.
func Validate(t *Template, env map[string]string) Result

// CheckNoCollision confere que dois envs de entidades distintas não colidem
// (porta, nome de container, rede, volume).
func CheckNoCollision(t *Template, envA, envB map[string]string) Result

type Result struct { OK bool; Errors []RuleError }
type RuleError struct { Rule, Detail string }
```

Erros por regra: `interpolation`, `named-volumes`, `no-secrets`, `no-collision`.
