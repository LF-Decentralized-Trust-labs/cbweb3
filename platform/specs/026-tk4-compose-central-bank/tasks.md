---
description: "Tasks for TK-4 — Template Compose central-bank"
---

# Tasks: TK-4 — Template Compose central-bank

**Input**: Design documents from `/specs/026-tk4-compose-central-bank/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, quickstart.md ✅

**Tests**: Incluídos — a Constitution (Princípio V) exige test-first. O script de integração é escrito **antes** do template e deve **FALHAR** antes da implementação.

**Organization**: Tarefas agrupadas por user story para implementação e teste independentes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependências incompletas)
- **[Story]**: User story de referência ([US1], [US2], [US3], [US4])

---

## Phase 1: Setup (Estrutura de Diretórios)

**Purpose**: Criar a estrutura de diretórios do toolkit. Nenhum código ainda.

- [X] T001 Criar diretórios `scenario-a/provisioning/templates/central-bank/` e `scenario-a/provisioning/tests/` e `scenario-a/provisioning/templates/central-bank/examples/`

**Checkpoint**: Diretórios existem; nenhum arquivo além das pastas.

---

## Phase 2: Foundational (Test-First — Script de Integração)

**Purpose**: Escrever o teste de integração que **DEVE FALHAR** antes do template existir. Bloqueante para todas as user stories.

**⚠️ CRITICAL**: O teste deve ser executado e falhar antes de qualquer implementação.

- [X] T002 Escrever `scenario-a/provisioning/tests/test-central-bank-template.sh` com os seguintes cenários de teste: (a) 1ª execução gera genesis, (b) 2ª execução preserva genesis (hash idêntico via sha256sum), (c) net_enode contém BESU_ADVERTISED_HOST, (d) cleanup automático via trap — ver plano `plan.md` Passo 1 para o script completo
- [X] T003 Rodar `bash scenario-a/provisioning/tests/test-central-bank-template.sh` e confirmar que **FALHA** com erro de template não encontrado (confirma que o teste está ativo e correto antes da implementação)

**Checkpoint**: Teste existe, falha corretamente. Implementação pode começar.

---

## Phase 3: User Story 1 — Genesis Guard Idempotente (Priority: P1) 🎯 MVP

**Goal**: O template sobe o nó Besu gerando genesis apenas na 1ª execução; reinicializações subsequentes preservam o genesis e os dados do nó.

**Independent Test**: `bash scenario-a/provisioning/tests/test-central-bank-template.sh` — os cenários (a) e (b) devem passar.

### Implementação — US1

- [X] T004 [US1] Criar `scenario-a/provisioning/templates/central-bank/docker-compose.yaml` com o serviço `genesis-init`: imagem `${BESU_IMAGE}`, `restart: "no"`, volumes `${SPOKE_DATA_DIR}/config:/config:ro`, `${SPOKE_DATA_DIR}/genesis:/genesis`, `${SPOKE_DATA_DIR}/nodes:/nodes`, e command shell com a lógica do genesis guard: `[ -f /genesis/genesis.json ] && echo "skipping" && exit 0 || besu operator generate-blockchain-config ...` — ver plano `plan.md` Passo 2 para o YAML completo
- [X] T005 [US1] Adicionar serviço `besu` ao `scenario-a/provisioning/templates/central-bank/docker-compose.yaml` com `depends_on: genesis-init: condition: service_completed_successfully`, volumes `${SPOKE_DATA_DIR}/nodes/central-bank/data:/opt/besu/data` e `${SPOKE_DATA_DIR}/genesis:/opt/besu/genesis:ro`, `restart: always`
- [X] T006 [US1] Adicionar seção `networks` ao `scenario-a/provisioning/templates/central-bank/docker-compose.yaml` com rede `spoke_besu` parametrizada: `name: ${SPOKE_NETWORK_NAME:-cbweb3-${SPOKE_ID}-besu}`, `driver: bridge`; adicionar `networks: [spoke_besu]` a ambos os serviços
- [X] T007 [US1] Criar `scenario-a/provisioning/templates/central-bank/examples/qbftConfigFile.json` com configuração QBFT mínima (chainId 1337, 1 validador) para uso em testes e quickstart
- [X] T008 [US1] Verificar que o cenário (a) do teste passa: `bash scenario-a/provisioning/tests/test-central-bank-template.sh` — 1ª execução deve gerar genesis e nó deve responder a `eth_blockNumber`
- [X] T009 [US1] Verificar que o cenário (b) do teste passa: 2ª execução deve preservar genesis (sha256sum idêntico) e logs do `genesis-init` devem conter `"already exists, skipping"`

**Checkpoint**: Genesis guard funcional. US1 testável de forma independente.

---

## Phase 4: User Story 2 — Parametrização de Endereçamento e Portas (Priority: P1)

**Goal**: Todas as portas, hostnames, caminhos e imagem do template vêm de variáveis de ambiente sem nenhum valor hardcoded. Ausência de variável obrigatória causa falha explícita do `docker compose`.

**Independent Test**: Executar `docker compose config` com dois conjuntos de variáveis (spoke-brl e spoke-usd) e confirmar container names, portas e network names distintos sem conflito.

### Implementação — US2

- [X] T010 [US2] Auditar `scenario-a/provisioning/templates/central-bank/docker-compose.yaml` e confirmar que todas as 7 variáveis obrigatórias (`SPOKE_ID`, `BESU_ADVERTISED_HOST`, `BESU_RPC_PORT`, `BESU_WS_PORT`, `BESU_P2P_PORT`, `BESU_IMAGE`, `SPOKE_DATA_DIR`) não possuem valor `:-default` (falha explícita se ausentes); variáveis opcionais (`BOOTNODE_ENODE`, `SPOKE_NETWORK_NAME`, `BESU_CONTAINER_NAME`, `BESU_LOGGING`) devem ter defaults conforme data-model.md
- [X] T011 [US2] Adicionar mapeamento de portas ao serviço `besu` em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`: `"${BESU_RPC_PORT}:8545"`, `"${BESU_WS_PORT}:8546"`, `"${BESU_P2P_PORT}:30303"`, `"${BESU_P2P_PORT}:30303/udp"`
- [X] T012 [US2] Verificar parametrização: executar `docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml config` com `SPOKE_ID=spoke-brl BESU_RPC_PORT=8645 ...` e depois com `SPOKE_ID=spoke-usd BESU_RPC_PORT=8646 ...`; confirmar que container names e network names são distintos em ambas as saídas e que nenhum valor de porta ou host aparece hardcoded no YAML renderizado
- [X] T013 [US2] Verificar falha explícita: remover `SPOKE_ID` do ambiente e executar `docker compose config`; confirmar que o Compose retorna erro, não comportamento silencioso com valor vazio

**Checkpoint**: Nenhum valor hardcoded no template. Dois spokes podem coexistir sem conflito de portas ou nomes.

---

## Phase 5: User Story 3 — Nó Besu como Bootnode do Spoke (Priority: P2)

**Goal**: O enode publicado pelo nó contém o `BESU_ADVERTISED_HOST` declarado (não o IP do contêiner). O nó sobe sem `--bootnodes` em `mode: found`; com `--bootnodes` quando `BOOTNODE_ENODE` está definido.

**Independent Test**: Subir o template, chamar `net_enode` via RPC e confirmar que o host no enode é igual ao valor de `BESU_ADVERTISED_HOST`.

### Implementação — US3

- [X] T014 [US3] Adicionar flag `--p2p-host=${BESU_ADVERTISED_HOST}` ao comando do serviço `besu` em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`
- [X] T015 [US3] Adicionar flag condicional `${BOOTNODE_ENODE:+--bootnodes=${BOOTNODE_ENODE}}` ao comando do serviço `besu` (incluído apenas quando `BOOTNODE_ENODE` está definido e não vazio)
- [X] T016 [US3] Adicionar health check ao serviço `besu` em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`: `test: ["CMD", "sh", "-c", "curl -sf -X POST http://localhost:8545 -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}' | grep -q result"]`, `interval: 5s`, `timeout: 3s`, `retries: 20`, `start_period: 10s`
- [X] T017 [US3] Verificar enode: executar `bash scenario-a/provisioning/tests/test-central-bank-template.sh` — o cenário (c) deve passar (enode contém `BESU_ADVERTISED_HOST`)
- [X] T018 [US3] Verificar bootnode ausente: subir template sem definir `BOOTNODE_ENODE` e confirmar que o comando Besu não inclui `--bootnodes` nos logs do contêiner (`docker logs <container> | grep -v bootnodes`)

**Checkpoint**: Enode publicado é roteável externamente. Nó sobe corretamente em `mode: found` (sem bootnode externo).

---

## Phase 6: User Story 4 — Template Agnóstico de Perfil (Priority: P3)

**Goal**: Trocar de perfil `local` para `prod` requer apenas mudança de valores das variáveis — nenhuma edição no template. A estrutura YAML renderizada é idêntica entre perfis.

**Independent Test**: `docker compose config` com variáveis locais vs. prod produz saídas com estrutura YAML idêntica; apenas os valores diferem.

### Implementação — US4

- [X] T019 [US4] Verificar agnóstico de perfil: executar `docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml config` com conjunto de variáveis `local` (BESU_IMAGE=hyperledger/besu:25.8.0, BESU_ADVERTISED_HOST=localhost, SPOKE_DATA_DIR=/tmp/local-spoke) e depois com conjunto `prod` (BESU_IMAGE=<lnet-registry>/besu:25.8.0, BESU_ADVERTISED_HOST=spoke-brl.prod.example.com, SPOKE_DATA_DIR=/data/spokes/spoke-brl); comparar os dois YAMLs e confirmar que apenas os valores de variáveis diferem, não os serviços, dependências ou estrutura

**Checkpoint**: Template validado como environment-agnostic. Promoção local→prod é troca de valores, não de código.

---

## Phase 7: Polish & Cross-Cutting

**Purpose**: Validação final, documentação e limpeza.

- [X] T020 [P] Executar `docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml config --quiet` e confirmar que retorna 0 sem warnings (valida schema YAML)
- [X] T021 Executar suite completa de integração: `bash scenario-a/provisioning/tests/test-central-bank-template.sh` — confirmar `PASS: template central-bank OK`
- [X] T022 [P] Verificar que `scenario-a/deploy/local/spoke-besu-a/startBesu.sh` e `scenario-a/deploy/local/spoke-besu-b/startBesu.sh` não foram modificados (confirma isolamento do sample de referência)
- [X] T023 Atualizar contexto do agente: `bash .specify/scripts/bash/update-agent-context.sh claude`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — começa imediatamente
- **Foundational (Phase 2)**: Depende de Setup — **BLOQUEIA** todas as user stories
- **US1 (Phase 3)**: Depende de Foundational — MVP entregável ao final desta fase
- **US2 (Phase 4)**: Depende de Foundational — pode começar em paralelo com US1 (arquivo compartilhado: coordenar)
- **US3 (Phase 5)**: Depende de US1 (template deve existir) — adiciona flags ao YAML existente
- **US4 (Phase 6)**: Depende de US3 (template completo) — validação pura, sem código novo
- **Polish (Phase 7)**: Depende de todas as user stories

### User Story Dependencies

- **US1 (P1)**: Pode começar após Foundational. Cria o `docker-compose.yaml` do zero.
- **US2 (P1)**: Pode começar após Foundational. Edita o mesmo `docker-compose.yaml` de US1 — **coordenar se paralelo**.
- **US3 (P2)**: Depende de US1 (arquivo existe). Adiciona flags ao serviço `besu`.
- **US4 (P3)**: Depende de US3 (template completo com --p2p-host e health check).

### Parallel Opportunities

- T001 (Setup) — sem dependências
- T002 e T007 (qbftConfigFile.json) — podem rodar em paralelo
- T010, T011 (auditoria + portas US2) — diferentes seções do mesmo arquivo: serializar
- T020, T022 (validação final, verificação de integridade) — arquivos distintos, paralelizáveis

---

## Parallel Example: User Story 1

```bash
# Criar template e exemplo de configuração em paralelo:
Task T004: "Criar docker-compose.yaml com genesis-init"
Task T007: "Criar examples/qbftConfigFile.json"

# Após T004 completo:
Task T005: "Adicionar serviço besu com depends_on"
Task T006: "Adicionar seção networks"
```

---

## Implementation Strategy

### MVP First (User Story 1 apenas)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002, T003 — teste falha)
3. Complete Phase 3: US1 (T004–T009 — template mínimo com genesis guard)
4. **STOP e VALIDE**: `bash scenario-a/provisioning/tests/test-central-bank-template.sh`
5. MVP: spoke de banco central sobe e não destrói estado em reinicializações

### Incremental Delivery

1. Setup + Foundational → testes prontos (falham)
2. US1 → genesis guard funcional → testes (a)(b) passam → MVP
3. US2 → parametrização completa → teste de dois spokes sem conflito
4. US3 → enode correto + health check → teste (c) passa
5. US4 → validação agnóstica de perfil → template production-ready
6. Polish → validação final completa

---

## Notes

- [P] = arquivos diferentes, sem dependências incompletas
- O `docker-compose.yaml` é **único arquivo de implementação** — US1, US2 e US3 editam o mesmo arquivo em sequência; US4 é validação
- Test-first é **obrigatório** (Constitution Princípio V): T002 deve falhar antes de T004
- `startBesu.sh` e `make/*.mk` **não são modificados** — preservados como sample de referência
- Commit após cada checkpoint para rastreabilidade

---

## Phase 8: Convergence

> Gerado por `/speckit-converge`. Findings baseados em inspeção do código atual e de `scenario-a/provisioning/spikes/spk-01-cross-stack-enode/` (ADR-001 + scripts de verificação do spike).

- [X] T024 CRITICAL: Revisar cenário (c) do script `scenario-a/provisioning/tests/test-central-bank-template.sh` (T002) — remover verificação de host via `net_enode` e substituir por: (1) confirmação de RPC reachability via `eth_blockNumber`, (2) confirmação de que o enode no join bundle não contém `172.x.x.x` nem `127.0.0.1`; ver padrão em `spikes/spk-01-cross-stack-enode/scripts/verify-enode.sh:10-37` — razão: Besu 25.8.0 retorna `127.0.0.1` em `net_enode` e `admin_nodeInfo` independentemente de `--nat-method`, tornando SC-004 e US3/AC1 (verificação de host via RPC) não implementáveis como especificado per SC-004 (contradicts)
- [X] T025 Adicionar suporte ao three-profile NAT matrix do ADR-001 ao serviço `besu` em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`: introduzir variável opcional `BESU_NAT_PROFILE` (default: `DOCKER`) que controla `--nat-method=${BESU_NAT_PROFILE:-DOCKER}`; remover `--p2p-host=${BESU_ADVERTISED_HOST}` como flag incondicional (usar apenas quando `BESU_NAT_PROFILE=NONE`) — o ADR-001 D1 especifica `--nat-method=DOCKER` para local cross-compose como perfil padrão de TK-4 per ADR-001 Downstream Impact (contradicts)
- [X] T026 [P] Adicionar `ADMIN` à lista `--rpc-http-api` do serviço `besu` em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`: mudar de `ETH,NET,QBFT` para `ETH,NET,QBFT,ADMIN`; a API ADMIN é necessária para `admin_nodeInfo` e `admin_peers` (extração do enode pós-NAT e verificação de peering — padrão do spike `stack-found.yml:21`) per FR-007, ADR-001 (partial)
- [X] T027 Adicionar tratamento DNS-to-IP para `--bootnodes` ao template: quando `BOOTNODE_ENODE` estiver definido, o serviço `besu` deve usar um entrypoint script (adaptado de `spikes/spk-01-cross-stack-enode/scripts/validator-entry.sh`) que resolve o hostname do bootnode via `getent hosts` antes de passar `--bootnodes` ao Besu — razão: Besu 25.8.0 rejeita non-IP values em `--bootnodes` (ADR-001 rejected alternatives); criar `scenario-a/provisioning/templates/central-bank/scripts/entry.sh` per FR-005, ADR-001 (partial)
- [X] T028 [P] Verificar se o genesis-init no Compose (T004) precisa do flag `--user $(id -u):$(id -g)` para evitar que arquivos gerados em `SPOKE_DATA_DIR` sejam owned por root; inspecionar comportamento de `docker run --user` vs `services.genesis-init.user` no Compose e adicionar `user: "${HOST_UID:-0}:${HOST_GID:-0}"` ao serviço `genesis-init` se necessário — ver padrão em `spikes/spk-01-cross-stack-enode/scripts/genesis-once.sh:27` per FR-002 (unrequested)

## Phase 9: Convergence

- [X] T029 Corrigir validação de `BESU_ADVERTISED_HOST` como variável obrigatória em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml:92`: mudar `${BESU_ADVERTISED_HOST:-}` para `${BESU_ADVERTISED_HOST?BESU_ADVERTISED_HOST is required when BESU_NAT_PROFILE=NONE}` no bloco `environment` do serviço `besu`; verificar que `docker compose config` sem `BESU_ADVERTISED_HOST` retorna erro explícito per FR-004 (partial)
- [X] T030 Adicionar validação de conteúdo do genesis.json ao comando do serviço `genesis-init` em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`: após a guarda `[ -f /genesis/genesis.json ]`, validar que o arquivo não está vazio (`[ -s /genesis/genesis.json ]`) e que é JSON válido (via `python3 -c "import json,sys; json.load(sys.stdin)" < /genesis/genesis.json` ou verificação equivalente disponível na imagem Besu); caso inválido, imprimir mensagem descritiva e `exit 1` antes de iniciar o nó per US1/AC3 (partial)
- [X] T031 [P] Documentar desvio de FR-006 via ADR-001 em `scenario-a/provisioning/templates/central-bank/scripts/entry.sh`: adicionar comentário explicando que `--nat-method=DOCKER` (perfil padrão) não inclui `--p2p-host` porque o ADR-001 D1 determina que Docker NAT auto-descobre o IP do host para cross-compose local, e que `--p2p-host` é exclusivo do perfil NONE (prod/staging com advertisedHost explícito); o enode verificável para o join bundle é extraído via `admin_nodeInfo`, não via `net_enode` per FR-006, SC-004, ADR-001 (contradicts)
- [X] T032 [P] Atualizar tabela de variáveis em `specs/026-tk4-compose-central-bank/data-model.md`: adicionar `HOST_UID` e `HOST_GID` à seção "Obrigatórias" com tipo `int`, exemplos `$(id -u)` / `$(id -g)`, e descrição explicando que são necessários para evitar arquivos owned por root em `SPOKE_DATA_DIR` (genesis-init roda como usuário do host) per plan:variables (unrequested)
