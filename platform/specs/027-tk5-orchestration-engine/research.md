# Research: TK-5 — Motor de orquestração (`mode: found`)

**Feature**: `027-tk5-orchestration-engine`  
**Phase**: 0 — Outline & Research  

---

## R-01: Template Compose do Paladin — novo template necessário

**Pergunta**: O TK-4 (`provisioning/templates/central-bank/docker-compose.yaml`) cobre o nó Besu. Para os nós Paladin, existe o `deploy/local/paladin/spoke-a/docker-compose.yml` — hardcoded para spoke-a. TK-5 precisa de um template Paladin parametrizado para spokes novos?

**Decisão**: Sim. O engine não pode reutilizar o compose hardcoded de spoke-a para spoke-brl. TK-5 deve criar `scenario-a/provisioning/templates/central-bank/paladin-compose.yaml` — um template Paladin parametrizado pelos mesmos princípios do TK-4 (sem valores hardcoded de porta, nome de container, rede Docker). O template expõe as variáveis: `SPOKE_ID`, `PALADIN_CB_RPC_PORT`, `PALADIN_CB_WS_PORT`, `PALADIN_CB_GRPC_PORT`, `PALADIN_IMAGE`, `SPOKE_DATA_DIR`, `SPOKE_NETWORK_NAME`.

**Rationale**: Sem um template parametrizado, o engine não consegue provisionar um segundo spoke sem criar um novo diretório de compose hardcoded — o que contradiz o requisito central do toolkit.

**Alternativa rejeitada**: Gerar o compose programaticamente em Go (manipulação YAML). Rejeitada: YAML gerado em código é difícil de auditar e manter; um template declarativo é preferível.

---

## R-02: Chamada on-chain ao IdentityRegistry (passo 9)

**Pergunta**: Como o engine chama `registerParticipant` no IdentityRegistry diretamente de Go, sem passar pelo api-gateway?

**Decisão**: Usar `github.com/ethereum/go-ethereum v1.17.1` (já em `toolkit/go.mod`) via `ethclient.Dial(besuRpcURL)` + ABI encoding direto com `go-ethereum/accounts/abi`. O engine define uma struct interna `registryClient` que encapsula o `ethclient.Client` e o ABI do IdentityRegistry. A chave de assinatura da transação é obtida via `KeyProvider.Sign()` (TK-2) — a chave privada nunca sai do provider.

**ABI necessário**: `registerParticipant(address wallet, string name, uint8 role, bytes32 zkPointer)`. O ABI JSON do contrato compilado está em `contracts/out/IdentityRegistry.sol/IdentityRegistry.json` (Foundry output).

**Rationale**: O engine opera no nível de infraestrutura — ele é o bootstrapper da rede. Chamar diretamente via go-ethereum é correto neste contexto; o api-gateway não existe ainda para o spoke que está sendo fundado.

**Alternativa rejeitada**: Invocar o script Go test `TestRegisterEVMParticipant` via `os/exec`. Rejeitada: o passo 9 é o onboarding real — ele precisa de controle fino sobre a chave usada para assinar a transação (via KeyProvider). Um subprocess não tem como receber a assinatura do KeyProvider do processo pai.

---

## R-03: Invocação de `render-configs.sh` — path e variáveis

**Pergunta**: O `render-configs.sh` usa `$(dirname $0)` para localizar o spoke dir. Como TK-5 o invoca para um spoke novo (spoke-brl) que não tem diretório próprio em `deploy/local/paladin/`?

**Decisão**: O `render-configs.sh` existente assume que o spoke dir já existe em `<script_dir>/<spoke>/` com os templates `config.yaml.tmpl`. Para um novo spoke provisionado pelo toolkit, o engine deve:
1. Gerar o diretório de configs do Paladin em `<SPOKE_DATA_DIR>/paladin/<node>/` com `config.yaml.tmpl` renderizados a partir de templates em `provisioning/templates/central-bank/paladin-config/`.
2. Invocar `render-configs.sh` com `SPOKE=<spoke-id>` e `cwd` setado para o dir de configs gerado, **ou** — mais simples — implementar a renderização de configs em Go diretamente usando `text/template` (sem dependência do script).

**Decisão refinada**: Implementar a renderização de configs do Paladin em Go usando `text/template`, lendo os templates de `provisioning/templates/central-bank/paladin-config/`. Isso elimina a dependência de path de `render-configs.sh` e é testável unitariamente. O script existente `render-configs.sh` continua intocado (é parte da rede de referência).

**Rationale**: O `render-configs.sh` é um script da rede de referência (`deploy/local/`), que não deve ser modificado. Para o toolkit, reproduzir a mesma lógica em Go é mais robusto e testável.

---

## R-04: File lock para execuções concorrentes (FR-013)

**Pergunta**: Como garantir serialização quando dois processos chamam `RunFound` para o mesmo `SPOKE_DATA_DIR`?

**Decisão**: `syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)` em um arquivo `<SPOKE_DATA_DIR>/.provisioning.lock`. Se o lock não for obtido imediatamente (LOCK_NB), retorna `ErrProvisioningLocked` com mensagem indicando PID do processo que detém o lock (via `/proc/<pid>/...` se disponível). O lock é liberado via `defer syscall.Flock(fd, syscall.LOCK_UN)`.

**Rationale**: `syscall.Flock` é Linux-only mas o target platform é Linux. É a forma mais simples e confiável de serializar execuções no mesmo diretório de dados.

---

## R-05: Relay registration endpoint (passo 10)

**Pergunta**: Qual é o endpoint REST do relay Cacti para registrar um spoke? O schema do payload está definido?

**Decisão**: O relay Cacti (`interop/hub-and-spoke/cacti/`) não expõe hoje um endpoint REST de registro de spokes — ele lê spokes de `config.ts` estaticamente (problema RL-1 identificado no concat.md §3). Portanto:

1. O passo 10 é implementado **com uma interface injetável** `RelayRegistrar`:
   ```go
   type RelayRegistrar interface {
       Register(ctx context.Context, spoke SpokeInfo) error
       IsRegistered(ctx context.Context, spokeID string) (bool, error)
   }
   ```
2. A implementação padrão (`httpRelayRegistrar`) tenta `POST <relay.endpoint>/api/v1/spokes` — retorna `ErrNotImplemented` se o relay responder 404/405 (endpoint ainda não existe).
3. O `check()` do passo 10 chama `IsRegistered`; se o relay retornar 404, considera o passo `pending`.
4. O passo 10 é um passo normal (hard step): `Run` propaga o erro de `Register`, então um relay configurado que rejeita/está inacessível registra `step_failed` e falha a execução (como é o último passo, o run reporta falha); `Check` retorna `false` quando o relay está indisponível, de modo que uma nova execução faz retry. Quando nenhum `relay.endpoint` é configurado, TK-7 injeta `NoOpRelayRegistrar` e o passo conclui como no-op de sucesso (registro pulado).

**Rationale**: Acoplamento rígido ao endpoint do relay bloquearia TK-5 de ser testável hoje. A interface `RelayRegistrar` permite mock nos testes e a implementação real é plugável quando RL-1 for concluído.

---

## R-06: Template de configs do Paladin — localização dos templates

**Pergunta**: Os nós Paladin precisam de `config.yaml` renderizado com endereços de contratos. Onde ficam os templates para novos spokes?

**Decisão**: Criar `scenario-a/provisioning/templates/central-bank/paladin-config/` com:
- `central-bank/config.yaml.tmpl` — template do nó CB
- `bank/config.yaml.tmpl` — template genérico de nó banco (parametrizado por nome)

Os templates são parametrizados por: `REGISTRY_CONTRACT_ADDRESS`, `ZETO_FACTORY_ADDRESS`, `PENTE_FACTORY_ADDRESS`, `PALADIN_CB_RPC_PORT`, `BESU_RPC_URL`, `PALADIN_NODE_NAME`.

O engine renderiza esses templates em Go (`text/template`) e os escreve em `<SPOKE_DATA_DIR>/paladin/<node>/config.yaml`.

---

## R-07: Restart do Paladin após deploy de contratos (passo 5)

**Pergunta**: O Makefile faz `stop → clean_volumes → start` no Paladin após os contratos serem deployados. Por quê? O engine deve replicar exatamente esse comportamento?

**Contexto verificado**: Os volumes do Paladin contêm o estado do block indexer (SQLite + LevelDB). Se o Paladin foi iniciado antes dos contratos (para registrar nós), seu block indexer não capturou os eventos de deploy. O clean de volumes força o Paladin a reindexar a chain a partir do bloco 0 com os contratos já existentes.

**Decisão**: Sim, o passo 5 deve replicar `stop → clean_volumes → start`. O `check()` do passo 5 verifica se o Paladin está em estado `running` E se o health check `ptx_getTransaction` retorna `PD020704` (Paladin pronto). Apenas se ambas as condições forem verdadeiras o passo é pulado.

**Atenção**: O `clean_volumes` só ocorre na primeira execução (transição `pending → done`). Em idempotência (passo já `done`), volumes não são limpos.

---

## R-08: Passo 4 (`register-nodes`) — idempotência via arquivo de estado

**Pergunta**: O `TestRegisterPaladinNodes` é idempotente? Pode ser executado duas vezes sem erro?

**Contexto**: O script registra os nós Paladin no IdentityRegistry de Paladin (diferente do IdentityRegistry EVM do passo 9). Executar duas vezes pode causar "node already registered" error.

**Decisão**: O `check()` do passo 4 usa apenas o arquivo de estado (`.provisioning-state.yaml`) — não existe consulta idempotente disponível no Paladin antes do primeiro start. Esta é a única exceção ao princípio de verificação externa: o estado persistido é a única fonte de verdade para o passo 4.

**Mitigação**: Se o passo 4 for re-executado incorretamente (arquivo de estado perdido/corrompido), o script Go test pode retornar erro "already registered". O engine trata este erro específico como idempotente (não falha) se a mensagem contém "already registered".

---

## Sumário de decisões

| ID | Decisão |
|----|---------|
| R-01 | Criar `paladin-compose.yaml` parametrizado em `provisioning/templates/central-bank/` |
| R-02 | Usar go-ethereum `ethclient` + ABI para chamar `registerParticipant` on-chain |
| R-03 | Implementar rendering de configs Paladin em Go (`text/template`) — não invocar `render-configs.sh` |
| R-04 | `syscall.Flock` para file lock em `.provisioning.lock` |
| R-05 | Interface `RelayRegistrar` injetável; implementação HTTP com graceful fallback para relay indisponível |
| R-06 | Templates Paladin em `provisioning/templates/central-bank/paladin-config/` |
| R-07 | Passo 5: `stop → clean_volumes → start` replicado; clean só na primeira execução |
| R-08 | Passo 4: único passo com check exclusivo no arquivo de estado (sem consulta externa disponível) |
