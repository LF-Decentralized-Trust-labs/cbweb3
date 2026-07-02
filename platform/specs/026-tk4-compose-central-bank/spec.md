# Feature Specification: TK-4 — Template Compose central-bank

**Feature Branch**: `026-tk4-compose-central-bank`  
**Created**: 2026-06-27  
**Status**: Draft  
**Input**: User description: "TK-4 — CRIAR: template Compose central-bank para o toolkit de provisionamento do Scenario A (Fase 1B)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Iniciar spoke do banco central sem regenerar genesis (Priority: P1)

Um operador executa o toolkit com `mode: found` para provisionar um novo spoke de banco central. O template Compose sobe o nó Besu com os parâmetros do manifesto. Se o genesis ainda não existe, o init container o gera e encerra; o nó Besu então inicia com o genesis recém-criado. Em execuções subsequentes o init container detecta que o genesis já existe e não executa nenhuma geração — o nó Besu inicia imediatamente sobre o estado persistido.

**Why this priority**: A idempotência do genesis é o requisito de segurança central do TK-4. O `startBesu.sh` existente regenera o genesis em toda execução, destruindo o estado da chain. Replicar esse comportamento no novo template tornaria o toolkit inútil para implantações reais.

**Independent Test**: Pode ser testado (a) subindo o Compose em um diretório de dados vazio e verificando que o genesis é criado, (b) derrubando e resubindo e verificando que o genesis não muda (hash idêntico), (c) inspecionando os logs do init container para confirmar a mensagem "genesis already exists, skipping generation".

**Acceptance Scenarios**:

1. **Given** um diretório de dados sem `genesis/genesis.json`, **When** o Compose sobe com variáveis do manifesto, **Then** o init container gera o genesis uma única vez e o nó Besu inicia em seguida.
2. **Given** um diretório de dados com `genesis/genesis.json` já existente, **When** o Compose é reiniciado, **Then** o init container não executa `besu operator generate-blockchain-config`, o genesis não é alterado, e o nó Besu inicia sobre o estado persistido.
3. **Given** o init container detecta um `genesis/genesis.json` malformado (arquivo vazio ou JSON inválido), **When** tenta iniciar o nó Besu, **Then** o init container aborta com um erro descritivo antes de iniciar o nó.

---

### User Story 2 — Parametrizar endereçamento e portas pelo manifesto (Priority: P1)

Um operador declara no manifesto os valores de endereçamento do nó (`advertisedHost`, `rpc.port`, `ws.port`, `p2p.port`) e de imagem (`image`). O template Compose lê esses valores via variáveis de ambiente renderizadas pelo motor de orquestração (TK-5). Não existe nenhum valor de porta ou hostname codificado no template.

**Why this priority**: A capacidade de parametrizar endereçamento é o que permite múltiplos spokes coexistirem em hosts distintos — a premissa central do track A. Qualquer valor hardcoded inviabiliza a adição de um segundo spoke sem editar o template.

**Independent Test**: Pode ser testado invocando o Compose com dois conjuntos diferentes de variáveis (portas 8645/8655/31303 para spoke-brl, e 8646/8656/31304 para spoke-usd) e verificando que os contêineres resultantes expõem portas distintas e têm nomes de contêiner distintos sem conflito.

**Acceptance Scenarios**:

1. **Given** variáveis `BESU_RPC_PORT=8645`, `BESU_WS_PORT=8655`, `BESU_P2P_PORT=31303`, `BESU_ADVERTISED_HOST=cbweb3-spoke-brl-besu.central-bank-brazil`, **When** o Compose sobe, **Then** o nó Besu escuta nessas portas e usa o `advertisedHost` declarado no `--p2p-host`.
2. **Given** variáveis de imagem `BESU_IMAGE=hyperledger/besu:25.8.0`, **When** o Compose sobe, **Then** o nó usa exatamente essa imagem sem hardcode de tag.
3. **Given** variável `SPOKE_DATA_DIR=/data/spokes/spoke-brl`, **When** o Compose sobe, **Then** os volumes de dados e genesis são montados nesse diretório base, sem dependência de um caminho absoluto hardcoded.

---

### User Story 3 — Nó Besu do banco central como bootnode do spoke (Priority: P2)

O banco central é o fundador do spoke (`mode: found`). Seu nó Besu é o único bootnode; nenhum bootnode externo é necessário para o primeiro nó subir. O template expõe o enode do bootnode para que o motor de orquestração (TK-5) possa incluí-lo no join bundle (TK-6).

**Why this priority**: O papel de bootnode é específico do `mode: found`. O template precisa subir o nó sem `--bootnodes` e expor o enode via RPC para que o motor possa emitir o join bundle com a informação correta.

**Independent Test**: Pode ser testado subindo o template em `mode: found`, aguardando o health check, e verificando que `net_enode` via RPC retorna um enode com o `advertisedHost` declarado (não `127.0.0.1`).

**Acceptance Scenarios**:

1. **Given** o template em `mode: found` (sem `--bootnodes`), **When** o nó Besu sobe e passa o health check, **Then** `net_enode` retorna um enode cujo host é exatamente o valor de `BESU_ADVERTISED_HOST`.
2. **Given** o template sem `BOOTNODE_ENODE` definido, **When** o nó Besu sobe, **Then** o nó não tenta conectar a nenhum bootnode externo e inicia a rede isolado (comportamento correto para o fundador).

---

### User Story 4 — Template utilizável por perfis local e prod sem código duplicado (Priority: P3)

Um operador que migra do perfil `local` para `prod` não precisa editar o template. A troca é apenas nos valores das variáveis (`BESU_IMAGE`, `BESU_ADVERTISED_HOST`, `SPOKE_DATA_DIR`). O template não contém nenhum `if [ "$ENV" = "local" ]` ou bifurcação por ambiente.

**Why this priority**: Garante que o princípio "environment is a parameter, never a fork" do design seja realizável sem duplicar o template. A Fase 4 (prod) plugará novos valores, não novo código.

**Independent Test**: Pode ser testado executando `docker compose config` com variáveis de perfil `local` e depois com variáveis de perfil `prod` e verificando que a saída difere apenas nos valores (imagem, host, caminhos), não na estrutura YAML ou nos serviços definidos.

**Acceptance Scenarios**:

1. **Given** as mesmas variáveis com `BESU_IMAGE` apontando para uma imagem de registro privado, **When** o Compose sobe, **Then** o nó usa a imagem do registro sem modificação no template.
2. **Given** um `SPOKE_DATA_DIR` apontando para um volume persistente externo (produção), **When** o Compose sobe, **Then** os dados do nó são persisitidos nesse volume sem hardcode de caminho local.

---

### Edge Cases

- O que acontece quando o init container não tem permissão de escrita no `SPOKE_DATA_DIR`?
- O que acontece quando o init container detecta um `genesis/genesis.json` que existe mas tem `chainId` diferente do esperado?
- O que acontece quando `BESU_ADVERTISED_HOST` não está definido (variável ausente)?
- O que acontece quando o health check do nó Besu falha após N tentativas — o Compose tenta reiniciar o init container?
- O que acontece quando `BESU_IMAGE` aponta para uma imagem inexistente no registry?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O template DEVE ser um arquivo `docker-compose.yaml` válido com substituição de variáveis via `${VAR:-default}`, sem lógica condicional por ambiente embutida no YAML.
- **FR-002**: O template DEVE incluir um serviço de init (`genesis-init`) que executa `besu operator generate-blockchain-config` **somente se** `${SPOKE_DATA_DIR}/genesis/genesis.json` não existir; caso contrário, encerra com código 0 sem executar nenhuma geração.
- **FR-003**: O serviço `genesis-init` DEVE ser declarado com `restart: no` e o serviço `besu` DEVE declarar `depends_on: genesis-init: condition: service_completed_successfully`.
- **FR-004**: O template DEVE expor as seguintes variáveis de substituição obrigatórias, todas sem valor padrão (falha explícita se ausente): `SPOKE_ID`, `BESU_ADVERTISED_HOST`, `BESU_RPC_PORT`, `BESU_WS_PORT`, `BESU_P2P_PORT`, `BESU_IMAGE`, `SPOKE_DATA_DIR`.
- **FR-005**: O nó Besu DEVE iniciar sem `--bootnodes` quando `BOOTNODE_ENODE` não estiver definido (comportamento correto para `mode: found`); DEVE incluir `--bootnodes=${BOOTNODE_ENODE}` quando definido (comportamento para nós adicionais do mesmo spoke).
- **FR-006**: O nó Besu DEVE usar `--p2p-host=${BESU_ADVERTISED_HOST}` para que o enode publicado contenha o host declarado, não o IP interno do contêiner.
- **FR-007**: O template DEVE incluir um health check no serviço `besu` que verifique via `eth_blockNumber` (RPC HTTP) que o nó está respondendo antes de declarar o serviço saudável.
- **FR-008**: O template DEVE declarar redes nomeadas parametrizadas (`${SPOKE_NETWORK_NAME}`) para que múltiplos spokes não compartilhem a mesma rede Docker.
- **FR-009**: O template NÃO DEVE usar a imagem `hyperledger/besu:latest` — a versão da imagem DEVE ser parametrizada via `BESU_IMAGE`.
- **FR-010**: O template NÃO DEVE hardcodar nenhum valor de porta, hostname, caminho de volume ou nome de contêiner — todos DEVEM vir de variáveis.
- **FR-011**: O template DEVE ser idempotente: múltiplas execuções de `docker compose up` sobre dados existentes não alteram o genesis, não apagam dados e retornam o nó ao estado running sem erro.

### Key Entities

- **`genesis-init`**: Serviço init do Compose (init container pattern). Usa a mesma imagem Besu. Executa um script de guarda que verifica a existência do genesis antes de gerar. Encerra após execução (não reinicia).
- **`besu`** (serviço principal): Nó Hyperledger Besu 25.8.0. Inicia somente após `genesis-init` completar com sucesso. Não executa lógica de genesis — apenas consome o genesis gerado pelo init container.
- **`SPOKE_DATA_DIR`**: Diretório base em que todos os artefatos do spoke (genesis, dados do nó, chaves) são montados. Nunca hardcoded; sempre parametrizado para suportar múltiplos spokes no mesmo host.
- **Genesis guard script**: Script shell embutido no comando do `genesis-init` (ou em um arquivo separado copiado na imagem de provisioning). Implementa a lógica: `[ -f "${SPOKE_DATA_DIR}/genesis/genesis.json" ] && exit 0 || besu operator generate-blockchain-config ...`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `docker compose up` em um diretório de dados vazio resulta em um nó Besu respondendo a `eth_blockNumber` via RPC dentro de 60 segundos.
- **SC-002**: `docker compose down && docker compose up` em dados existentes (genesis já presente) retorna o nó ao estado running sem alterar o hash do genesis — verificável por `sha256sum` antes e depois.
- **SC-003**: `docker compose config` com variáveis de spoke-brl e depois spoke-usd produz saídas com nomes de contêiner, redes e portas distintos, sem conflito e sem edição no template.
- **SC-004**: `net_enode` via RPC retorna um enode cujo host é exatamente o valor de `BESU_ADVERTISED_HOST` (não `127.0.0.1` nem o IP do contêiner).
- **SC-005**: A tentativa de subir o Compose sem definir qualquer uma das variáveis obrigatórias resulta em erro explícito do Docker Compose (substituição sem default falha), não em comportamento silencioso com valor vazio.
- **SC-006**: O template passa `docker compose config --quiet` (validação de schema YAML) sem warnings.

## Assumptions

- Docker Compose v2 (plugin `docker compose`) — não `docker-compose` v1.
- A imagem padrão de referência é `hyperledger/besu:25.8.0` (pinada, conforme constitution); `BESU_IMAGE` permite sobrescrever.
- O motor de orquestração (TK-5) é responsável por renderizar as variáveis de ambiente a partir do manifesto antes de invocar `docker compose up` — o template não lê o manifesto YAML diretamente.
- `KeyProvider` (TK-2) e `CertSource` (TK-3) são integrados pelo motor de orquestração (TK-5), não diretamente pelo template Compose. O template expõe variáveis para caminhos de cert/TLS que o motor preenche após obter os materiais das interfaces.
- O template é exclusivo para o nó Besu do banco central. Os serviços de backend (api-gateway, auth, compliance, payment-orchestrator) são um Compose separado, existente em `backend/docker-compose-backend.*.yaml`, que não é alterado por esta spec.
- O template reside em `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`. O diretório `deploy/local/` não é modificado.
- O `SPOKE_DATA_DIR` existe no host antes de `docker compose up` ser invocado (responsabilidade do motor TK-5 criar o diretório se necessário).
- `mode: join` (banco comercial) é escopo de TK-8, não desta spec.
