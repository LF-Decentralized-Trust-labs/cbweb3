# Tasks: RL-1/RL-2/RL-3 — Relay: Registro Dinâmico de Spokes

**Input**: Design documents from `specs/031-relay-spoke-registry/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Organization**: Tasks grouped by user story for independent implementation and testing.  
**Test-first**: Constituição Princípio V — testes DEVEM falhar antes da implementação. Rodar `npx vitest run` após cada tarefa de teste e confirmar falha.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependência de tarefas incompletas)
- **[Story]**: US1/US2/US3 mapeado às user stories do spec.md

---

## Phase 1: Setup

**Purpose**: Adicionar infraestrutura de testes e stub do loader antes de qualquer implementação.

- [X] T001 Adicionar `js-yaml ^4.1.0`, `@types/js-yaml ^4.0.9` e `vitest ^2.0.0` ao `package.json` em `scenario-a/interop/hub-and-spoke/cacti/`; adicionar script `"test": "vitest run"` e `"test:watch": "vitest"`; rodar `npm install` para confirmar sem erros
- [X] T002 Criar `scenario-a/interop/hub-and-spoke/cacti/src/spokes-config.ts` com stubs exportados vazios: `export function loadSpokesConfig(): SpokeConfig[] { throw new Error("not implemented"); }`, `export function validateSpokesConfig(spokes: unknown[]): void {}`, `export function buildLegacyShim(log: Pick<Console,"warn">): SpokeConfig[] { throw new Error("not implemented"); }` — suficiente para que imports em testes compilem

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Atualizar interfaces TypeScript que todas as user stories dependem.

**⚠️ CRÍTICO**: Nenhuma user story pode começar antes desta fase.

- [X] T003 Atualizar `SpokeConfig` em `scenario-a/interop/hub-and-spoke/cacti/src/config.ts`: renomear campo `name` → `id`; remover `counterpartGrpc` e `counterpartName`; adicionar `grpcEndpoint: string`; manter todos os outros campos (`besuRpc`, `besuWs`, `htlcAddress`, `internalApiUrl`); remover os objetos `spokeA`/`spokeB` do `config` export (deixar `spokes: [] as SpokeConfig[]` como placeholder); compilar com `npx tsc --noEmit` para confirmar
- [X] T004 Atualizar `SpokeDep` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`: renomear campo `name` → `id`; remover `counterpartGrpc` e `counterpartName`; adicionar `grpcEndpoint: string`; substituir TODAS as referências `spoke.name` → `spoke.id` no corpo do arquivo (log strings, ring buffer entries); adicionar campo privado `private grpcClients = new Map<string, PaymentOrchestratorClient>()` à classe `HtlcRelay`; compilar com `npx tsc --noEmit`

**Checkpoint**: Interfaces TypeScript compilam; stubs em config.ts e htlc-relay.ts compilam sem erros.

---

## Phase 3: User Story 1 — Operador registra N spokes via YAML (Priority: P1) 🎯 MVP

**Goal**: Operador inicia o relay com `CACTI_SPOKES_CONFIG=spokes.yaml` com N spokes; relay inicia e registra todos.

**Independent Test**: `CACTI_SPOKES_CONFIG=/tmp/test-spokes.yaml npx vitest run` — testes do loader passam; relay inicia e loga spokes.

### Testes para US1 (escrever ANTES — devem FALHAR)

> **CONSTITUTION V**: Rodar `npx vitest run` após cada tarefa de teste e confirmar que os testes FALHAM antes de implementar.

- [X] T005 [US1] Escrever testes failing para `loadSpokesConfig` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.test.ts`: (a) YAML válido com dois spokes → retorna array de dois `SpokeConfig`; (b) YAML com campo `besuRpc` ausente → lança `Fatal: spoke[0].besuRpc is required`; (c) YAML com lista vazia → lança `Fatal: at least one spoke must be configured`; (d) YAML com IDs duplicados → lança `Fatal: duplicate spoke id "spoke-a"`; (e) arquivo inexistente → lança `Fatal: cannot read spokes config: <path>`; rodar `npx vitest run` e confirmar falha
- [X] T006 [US1] Escrever testes failing para `buildLegacyShim` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.test.ts`: (a) `SPOKE_A_BESU_RPC` e `SPOKE_B_BESU_RPC` definidos → retorna dois entries onde `spoke-a.grpcEndpoint = SPOKE_A_PAYMENT_GRPC` e `spoke-b.grpcEndpoint = SPOKE_B_PAYMENT_GRPC`; (b) `SPOKE_A_BESU_RPC` ausente → lança `Fatal: SPOKE_A_BESU_RPC is required`; (c) log de deprecação é chamado com aviso `CACTI_SPOKES_CONFIG is preferred`; rodar `npx vitest run` e confirmar falha

### Implementação para US1

- [X] T007 [P] [US1] Implementar `loadSpokesFromYaml(filePath: string): unknown[]` em `scenario-a/interop/hub-and-spoke/cacti/src/spokes-config.ts`: lê arquivo com `fs.readFileSync`, parseia com `js-yaml.load`, extrai array de `spokes`; lança `Fatal: cannot read spokes config: <path>: no such file` se arquivo não existe
- [X] T008 [P] [US1] Implementar `validateSpokesConfig(spokes: unknown[])` em `scenario-a/interop/hub-and-spoke/cacti/src/spokes-config.ts`: verifica array não-vazio (`Fatal: at least one spoke must be configured`); para cada entry, verifica campos obrigatórios `id`, `besuRpc`, `besuWs`, `htlcAddress`, `internalApiUrl`, `grpcEndpoint` (`Fatal: spoke[N].<field> is required`); verifica IDs únicos (`Fatal: duplicate spoke id "<id>"`); retorna `SpokeConfig[]` tipado
- [X] T009 [US1] Implementar `buildLegacyShim(log: Pick<Console,"warn">): SpokeConfig[]` em `scenario-a/interop/hub-and-spoke/cacti/src/spokes-config.ts`: lê `SPOKE_A_BESU_RPC`, `SPOKE_A_BESU_WS` (default `ws://localhost:8655`), `SPOKE_A_HTLC_ADDRESS`, `SPOKE_A_INTERNAL_API`, `SPOKE_A_PAYMENT_GRPC` e equivalentes `SPOKE_B_*`; lança `Fatal: <VAR> is required` para obrigatórias ausentes; chama `log.warn("[relay] DEPRECATION: SPOKE_A_*/SPOKE_B_* env vars are deprecated — use CACTI_SPOKES_CONFIG instead")` antes de retornar; retorna dois entries com `id: "spoke-a"` e `id: "spoke-b"` onde `grpcEndpoint = SPOKE_A_PAYMENT_GRPC` (spoke-a) e `grpcEndpoint = SPOKE_B_PAYMENT_GRPC` (spoke-b)
- [X] T010 [US1] Implementar `loadSpokesConfig(log: Pick<Console,"warn">): SpokeConfig[]` em `scenario-a/interop/hub-and-spoke/cacti/src/spokes-config.ts`: se `CACTI_SPOKES_CONFIG` definido → chama `loadSpokesFromYaml` + `validateSpokesConfig`; senão se `SPOKE_A_BESU_RPC` + `SPOKE_B_BESU_RPC` definidos → chama `buildLegacyShim`; senão lança `Fatal: relay requires either CACTI_SPOKES_CONFIG or SPOKE_A_BESU_RPC + SPOKE_B_BESU_RPC`; retorna `SpokeConfig[]`
- [X] T011 [US1] Atualizar `scenario-a/interop/hub-and-spoke/cacti/src/config.ts`: substituir placeholder `spokes: []` por `spokes: loadSpokesConfig(console)` chamado na inicialização do módulo; remover as funções `requireEnv`/`optionalEnv` usadas para os campos de spoke (mantê-las apenas para `apiPort`, `pollIntervalMs`, `relayAuthSecret`, `socketIoAllowedOrigins`, `relayStorePath`, `protoPath`); importar `loadSpokesConfig` de `./spokes-config`
- [X] T012 [US1] Rodar `npx vitest run` em `scenario-a/interop/hub-and-spoke/cacti/` — os testes T005 e T006 devem agora passar; compilar com `npx tsc --noEmit` para confirmar sem erros TypeScript

**Checkpoint**: `loadSpokesConfig` funciona com YAML e com shim legado. Tests T005/T006 passando. `config.spokes` é um array populado no startup.

---

## Phase 4: User Story 2 — Relay roteia liquidação para o spoke de destino correto (Priority: P1)

**Goal**: `LogHTLCClaimed` em spoke-a com `dest_spoke_id=spoke-b` → relay chama `SettleHTLC` no gRPC do spoke-b (não do spoke-a).

**Independent Test**: Teste de integração com dois mocks de gRPC — confirmar que `SettleHTLC` é chamado no mock do spoke-b.

### Testes para US2 (escrever ANTES — devem FALHAR)

- [X] T013 [US2] Escrever testes failing para `resolveCounterpart` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.test.ts`: (a) ring buffer tem lock de spoke-a e lock de spoke-b com mesmo hashLock → `resolveCounterpart("spoke-a", contractIdA)` retorna `{ destSpokeId: "spoke-b", contractId: contractIdB }`; (b) sem lock correspondente → retorna `undefined` com log de warning; rodar `npx vitest run` e confirmar falha
- [X] T014 [US2] Escrever testes failing para roteamento de liquidação em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.test.ts`: (a) `dest_spoke_id` presente no registry → `SettleHTLC` chamado no gRPC mock do destino; (b) `dest_spoke_id=spoke-c` ausente do registry → log de erro `settlement skipped: dest_spoke_id "spoke-c" not in registry`, sem crash; rodar `npx vitest run` e confirmar falha

### Implementação para US2

- [X] T015 [US2] Atualizar `resolveCounterpartContractId` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`: renomear para `resolveCounterpart`; mudar assinatura para `private resolveCounterpart(spokeId: string, contractId: string): { destSpokeId: string; contractId: string } | undefined`; retornar `{ destSpokeId: counterpartLock.spoke, contractId: counterpartLock.contractId }` quando encontrado; retornar `undefined` com log de warning quando não encontrado
- [X] T016 [US2] Atualizar `HtlcRelay.start()` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`: popular `this.grpcClients` antes de iniciar os loops de polling — `for (const spoke of this.spokes) { this.grpcClients.set(spoke.id, createGrpcClient(spoke.grpcEndpoint, this.protoPath)); }` — criar clientes gRPC por `spoke.grpcEndpoint` em vez de `counterpartGrpc`
- [X] T017 [US2] Atualizar `startSpokeLoop` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`: remover linha `const grpcClient = createGrpcClient(spoke.counterpartGrpc, ...)` do início do loop; ao processar `LogHTLCClaimed`, chamar `this.resolveCounterpart(spoke.id, contractId)` → se `undefined`, logar warning e `continue`; obter cliente por `this.grpcClients.get(resolved.destSpokeId)` → se `undefined`, logar erro `settlement skipped: dest_spoke_id "${resolved.destSpokeId}" not in registry` e `continue`; passar cliente correto para `settleOnCounterpart`
- [X] T018 [US2] Atualizar `pollFXAgreementsRest` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` — FX PROPOSED: remover bloco `if (spoke.counterpartName && ...)` de validação bilateral; substituir `client` passado para `tryForwardFXAction` por `this.grpcClients.get(evt.destSpokeId)` → se `undefined`, logar erro e `continue`; incluir `destSpokeId: evt.destSpokeId` no payload passado para `scheduleRetry`
- [X] T019 [US2] Atualizar `pollFXAgreementsRest` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` — FX ACCEPTED/REJECTED/CANCELLED/SETTLED: ler `a["source_spoke_id"] as string` do raw agreement em cada bloco de estado; obter cliente por `this.grpcClients.get(sourceSpokeId)` → se `undefined`, logar erro e `continue`; incluir `destSpokeId: sourceSpokeId` (a contraparte da ação de retorno) no payload passado para `scheduleRetry`
- [X] T020 [US2] Atualizar `processDueRetriesForSpoke` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`: remover parâmetro `client: PaymentOrchestratorClient`; extrair `const destSpokeId = item.payload?.destSpokeId as string | undefined`; se ausente, logar erro e `continue`; obter `const client = this.grpcClients.get(destSpokeId)` → se ausente, logar erro e `continue`; atualizar chamada em `startSpokeLoop` para remover `grpcClient` do argumento
- [X] T021 [US2] Rodar `npx vitest run` — testes T013 e T014 devem agora passar; rodar `npx tsc --noEmit` para confirmar TypeScript limpo

**Checkpoint**: Roteamento por `dest_spoke_id` funcional. `resolveCounterpart` retorna `{ destSpokeId, contractId }`. FX forwarding usa lookup por ID em vez de `counterpartGrpc`.

---

## Phase 5: User Story 3 — Polling dinâmico cobre todos os spokes registrados (Priority: P2)

**Goal**: `config.spokes` com N entries → N connectors Besu criados e N loops de polling iniciados.

**Independent Test**: Startup com três spokes no YAML → três `[<id>] starting poll loop` nos logs.

### Implementação para US3

- [X] T022 [US3] Atualizar `scenario-a/interop/hub-and-spoke/cacti/src/index.ts`: substituir criação hardcoded `connectorSpokeA`/`connectorSpokeB` por loop dinâmico sobre `config.spokes` — `for (const spoke of config.spokes) { const connector = new PluginLedgerConnectorBesu({instanceId: ..., rpcApiHttpHost: spoke.besuRpc, rpcApiWsHost: spoke.besuWs, pluginRegistry, logLevel: "INFO"}); await connector.onPluginInit(); connectors.set(spoke.id, connector); console.log(\`[cacti] registered spoke: ${spoke.id} rpc=${spoke.besuRpc} htlc=${spoke.htlcAddress}\`); }`; remover variáveis `connectorSpokeA`/`connectorSpokeB`; remover `connectors.set("spoke-a", ...)` / `connectors.set("spoke-b", ...)` hardcoded; atualizar log de startup para não listar spokes individualmente com nomes fixos
- [X] T023 [US3] Verificar que `HtlcRelay` é instanciado com `config.spokes` (array dinâmico) em `scenario-a/interop/hub-and-spoke/cacti/src/index.ts` — linha `new HtlcRelay(config.spokes, ...)` já usa o array; confirmar que `config.spokes` é agora o array carregado pelo loader (não objetos literais); rodar `npx tsc --noEmit`

**Checkpoint**: `index.ts` compila sem referências a `spokeA`/`spokeB`. Startup dinâmico funcional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T024 [P] Atualizar `scenario-a/interop/hub-and-spoke/cacti/env-sample`: adicionar bloco `# Registro de spokes (método recomendado)` com `CACTI_SPOKES_CONFIG=/etc/cacti/spokes.yaml`; adicionar bloco `# Vars legadas (DEPRECATED — use CACTI_SPOKES_CONFIG)` precedendo `SPOKE_A_BESU_RPC` etc.; remover `SPOKE_A_PAYMENT_GRPC` e `SPOKE_B_PAYMENT_GRPC` dos exemplos ativos e movê-los para a seção de deprecated
- [X] T025 Rodar `npx tsc --noEmit` em `scenario-a/interop/hub-and-spoke/cacti/` — deve passar sem erros; corrigir quaisquer erros TypeScript residuais
- [X] T026 Rodar `npx vitest run` em `scenario-a/interop/hub-and-spoke/cacti/` — todos os testes devem passar; confirmar que a saída inclui os testes T005, T006, T013, T014

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — pode iniciar imediatamente
- **Foundational (Phase 2)**: Depende de Phase 1 completa — BLOQUEIA todas as user stories
- **US1 (Phase 3)**: Depende de Phase 2 — pode iniciar após Foundational
- **US2 (Phase 4)**: Depende de Phase 2 + US1 (usa `SpokeConfig[]` populado pelo loader)
- **US3 (Phase 5)**: Depende de Phase 2 + US1 (`config.spokes` precisa estar populado)
- **Polish (Phase 6)**: Depende de todas as user stories

### User Story Dependencies

- **US1 (P1)**: Depende de Foundational. Sem dependência de US2/US3.
- **US2 (P1)**: Depende de Foundational + US1 (registry precisa estar populado via loader).
- **US3 (P2)**: Depende de Foundational + US1 (índex.ts usa `config.spokes`).

### Within Each User Story

- Testes DEVEM ser escritos e FALHAR antes da implementação (Constituição V)
- Stubs antes de implementações completas
- Compilar com `tsc --noEmit` após cada mudança de interface
- Rodar `vitest run` após cada fase de implementação para confirmar passing

### Parallel Opportunities

- T007 e T008 podem rodar em paralelo (arquivos diferentes dentro de `spokes-config.ts` são independentes como funções)
- T022 e T024 podem rodar em paralelo (arquivos diferentes: `index.ts` e `env-sample`)
- T025 e T026 podem rodar em paralelo (verificações independentes)

---

## Parallel Example: US1

```bash
# Escrever testes em paralelo (mesmo arquivo — serializado):
npx vitest run  # confirmar T005 falha
npx vitest run  # confirmar T006 falha

# Implementar funções independentes em paralelo:
# Developer A: loadSpokesFromYaml + validateSpokesConfig (T007, T008)
# Developer B: buildLegacyShim (T009)
# → Depois: loadSpokesConfig orquestra ambas (T010)
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (CRÍTICO — bloqueia todas as stories)
3. Completar Phase 3: US1 — relay reconfigurável via YAML
4. **PARAR e VALIDAR**: Iniciar relay com YAML de dois spokes; confirmar logs de registro
5. Continuar para US2 (roteamento)

### Incremental Delivery

1. Setup + Foundational → interfaces prontas
2. US1 → relay carrega N spokes de YAML (demo possível)
3. US2 → liquidação roteada por `dest_spoke_id` (demo end-to-end com dois spokes)
4. US3 → polling dinâmico limpo em index.ts
5. Polish → TypeScript limpo + todos os testes passando

---

## Notes

- **Mudança semântica crítica** (ver research.md §3): `SPOKE_A_PAYMENT_GRPC` → `grpcEndpoint` de spoke-a (não de spoke-b). O shim em T009 deve mapear cada spoke ao seu PRÓPRIO endpoint gRPC.
- **`spoke.name` → `spoke.id`** (T003/T004): renomear todos os ~30 usos em `htlc-relay.ts`. Usar `grep -n "spoke\.name"` para localizar.
- **`processDueRetriesForSpoke` sem client** (T020): a assinatura muda de `(client, spokeName)` para `(spokeName)`. Atualizar a chamada na linha ~467 de `htlc-relay.ts`.
- Confirmar que `RelayStore` não precisa de mudança de schema — `destSpokeId` vai no campo `payload` já existente.
- Arquivos **NÃO tocados**: `docker-compose.yaml`, `relay-store.ts`, `CACTI_README.md`, qualquer arquivo fora de `scenario-a/interop/hub-and-spoke/cacti/`.
- **Método `pollSpoke` (não `startSpokeLoop`)**: T016 e T017 referenciam "startSpokeLoop" mas o método real é `pollSpoke` (linha 335 de `htlc-relay.ts`). Usar `pollSpoke` ao implementar.

---

## Phase 7: Convergence

- [X] T027 Remover `client: PaymentOrchestratorClient` da assinatura de `pollFXAgreementsRest` em `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts`; atualizar a chamada na linha 468 de `pollSpoke` de `await this.pollFXAgreementsRest(grpcClient, spoke)` para `await this.pollFXAgreementsRest(spoke)` — executar junto com T018/T019 para evitar erro de compilação TypeScript (partial, FR-005)
- [X] T028 Remover `grpcClient.close()` da linha 483 de `scenario-a/interop/hub-and-spoke/cacti/src/htlc-relay.ts` (fim de `pollSpoke`); em substituição, em `HtlcRelay.start()` adicionar listener no AbortSignal para fechar todos os clientes em `this.grpcClients` quando o relay encerrar: `signal.addEventListener("abort", () => { for (const c of this.grpcClients.values()) c.close(); }, { once: true })` — executar junto com T016 (missing, plan: gRPC client lifecycle)
