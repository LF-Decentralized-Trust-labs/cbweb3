# Quickstart — TK-B4 (templates de compose parametrizados)

Como validar os templates e como o motor (fase futura) os consome. Nada sobe containers nesta fase.

## Estrutura entregue

```text
scenario-b/provisioning/templates/
  hub.compose.yaml  entity-infra.compose.yaml  entity-keycloak.compose.yaml
  entity-backend.compose.yaml  entity-frontend.compose.yaml
  relay.compose.yaml  noc.compose.yaml
  vars/*.env.example        # contrato de variáveis por template
  vars/NAMING.md            # convenção determinística de portas (offset) e nomes
scenario-b/toolkit/engine/composetemplate/   # validação Go
```

## Validar via Go (estático, sem Docker)

```bash
cd scenario-b/toolkit
go test ./engine/composetemplate/...
```

Cobre: interpolação completa de cada template com o `.env.example` (SC-001), falha em variável
obrigatória ausente (SC-005), named volumes sem bind host indevido (SC-003), ausência de segredos
(SC-006) e ausência de colisão entre duas entidades (SC-002).

## Validação semântica opcional (com Docker)

```bash
# Renderização/validação canônica do compose (pulada nos testes se o Docker não existir):
docker compose -f scenario-b/provisioning/templates/hub.compose.yaml \
  --env-file scenario-b/provisioning/templates/vars/hub.env.example config -q
```

Exit 0 e nenhum `${...}` residual = interpolação completa.

## Como o motor usará (fase futura, TK-B6)

1. Calcula o `offset` e o `ENTITY_VOLUME_PREFIX` da entidade (convenção em `NAMING.md`).
2. Preenche as variáveis do contrato do template.
3. Renderiza/sobe via `docker compose -f <template> --env-file <env>`.

O TK-B4 entrega os templates + contratos + validação; o cálculo de offset/nomes e a subida ficam no
motor.

## Verificar que `deploy/local` está intacto (SC-004)

```bash
git status --short scenario-b/deploy/local   # deve ser vazio
```
