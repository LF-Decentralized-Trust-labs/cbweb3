# Feature Specification: TK-5 — Motor de orquestração (`mode: found`)

**Feature Branch**: `027-tk5-orchestration-engine`  
**Created**: 2026-06-27  
**Status**: Draft  
**Input**: User description: "TK-5 — CRIAR: motor de orquestração (mode: found) para o toolkit de provisionamento do Scenario A (Fase 1B)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Provisionar spoke de banco central do zero (`mode: found`) (Priority: P1)

Um operador invoca o engine com um manifesto `mode: found` para um banco central que está fundando um novo spoke. O engine executa os 10 passos em sequência — deploy de contratos, TLS, configs, registro de nós, start do Paladin, token Zeto, contexto Pente, FXAgreement no Pente, onboarding real no IdentityRegistry, registro no relay — e ao final o spoke está operacional: Paladin respondendo, contratos deployados, banco central registrado on-chain, relay ciente do spoke.

**Why this priority**: É o único caminho de provisionamento de um novo spoke. Sem este fluxo completo não é possível adicionar nenhum participante à rede, o que é o gating item de toda a Fase 1B.

**Independent Test**: Pode ser testado num ambiente local limpo invocando `RunFound(ctx, manifest, deps)` com um manifesto `spoke-brl`, aguardando a conclusão, e verificando: (a) `ptx_getTransaction` no Paladin CB responde sem erro, (b) `IdentityRegistry.isParticipant(evmAddress)` retorna `true` para o endereço do banco central, (c) `GET /spokes/spoke-brl` no relay retorna 200.

**Acceptance Scenarios**:

1. **Given** um manifesto válido com `mode: found`, `spoke.id: spoke-brl`, e o Besu do spoke já rodando (via template TK-4), **When** `RunFound` é chamado, **Then** os 10 passos são executados em sequência, cada um registra `status: done` no arquivo de estado, e o Paladin CB responde em `PALADIN_CB_URL`.
2. **Given** o spoke-brl já totalmente provisionado (todos os passos `done`), **When** `RunFound` é chamado novamente, **Then** todos os passos retornam `check() = true`, nenhum script ou comando externo é executado, e `RunFound` retorna `nil` em menos de 2 segundos.
3. **Given** `RunFound` falhando no passo 6 (criação do token Zeto) por timeout do Paladin, **When** o problema é corrigido e `RunFound` é chamado novamente, **Then** os passos 1–5 são pulados (já `done`), a execução retoma no passo 6 e completa os passos 6–10.

---

### User Story 2 — Idempotência total: re-execução não destrói estado existente (Priority: P1)

Um operador re-executa o engine sobre um spoke já provisionado (por exemplo, após um restart da máquina). O engine detecta que cada passo já foi executado, não invoca nenhum subprocess e retorna sem erro. Em especial: genesis nunca é regenerado, volumes do Paladin nunca são limpos desnecessariamente, e o IdentityRegistry não recebe uma segunda tentativa de registro.

**Why this priority**: A idempotência é o requisito de segurança central de todo o toolkit. Um engine que re-executa passos cegamente pode destruir o estado da chain (regenerar genesis), criar tokens duplicados no Paladin, ou falhar com "participant already registered" — qualquer um desses casos inutiliza o spoke.

**Independent Test**: Pode ser testado executando `RunFound` duas vezes consecutivas em um spoke limpo, instrumentando cada `Step.run()` com um contador de chamadas, e afirmando que na segunda execução o contador é zero para todos os passos.

**Acceptance Scenarios**:

1. **Given** passo 1 com `REGISTRY_CONTRACT_ADDRESS` presente em `.deployed-addrs.env`, **When** `check()` do passo 1 é chamado, **Then** retorna `true` sem nenhuma chamada de subprocess.
2. **Given** passo 9 com o endereço EVM do banco central já registrado on-chain, **When** `check()` do passo 9 é chamado via consulta ao `IdentityRegistry`, **Then** retorna `true` e o passo não reexecuta o fluxo de onboarding.
3. **Given** genesis já existente em `${SPOKE_DATA_DIR}/genesis/genesis.json`, **When** o engine é chamado, **Then** em nenhum momento `besu operator generate-blockchain-config` é invocado pelo engine.

---

### User Story 3 — Retomada de execução interrompida (Priority: P2)

O engine é interrompido (SIGTERM, falha de rede, timeout) no meio da sequência. Na próxima execução, o engine lê o arquivo de estado, determina o último passo concluído, e retoma a partir do passo seguinte — sem re-executar nada que já está `done`.

**Why this priority**: Provisionamento de um spoke pode levar vários minutos. Exigir que o operador recomece do zero após qualquer falha parcial é inaceitável em ambiente de produção; também viola o princípio de não-destruição (re-executar o passo 5 limpa volumes desnecessariamente).

**Independent Test**: Pode ser testado injetando um erro forçado no passo 7 (criar contexto Pente), verificando que os passos 1–6 estão `done` no arquivo de estado, corrigindo a condição de falha, e re-executando — afirmando que os passos 1–6 são pulados e a execução começa no 7.

**Acceptance Scenarios**:

1. **Given** o engine interrompido com os passos 1–5 `done` e os passos 6–10 `pending`, **When** `RunFound` é chamado novamente, **Then** o log mostra `skipping step=deploy-contracts reason=already-done` para os passos 1–5 e executa normalmente a partir do passo 6.
2. **Given** um passo em estado `running` (engine morreu durante a execução), **When** `RunFound` retoma, **Then** o passo é tratado como `pending` (re-executado) pois o estado `running` nunca é persistido como `done` sem conclusão confirmada.

---

### User Story 4 — Observabilidade: log estruturado de cada transição de passo (Priority: P2)

Para cada transição de passo (início, conclusão, skip, falha), o engine emite uma linha de log JSON estruturado em stdout com campos obrigatórios. O operador consegue reconstruir o que aconteceu em cada execução lendo os logs sem acesso ao arquivo de estado interno.

**Why this priority**: A Constitution (Princípio VI) exige logs estruturados com `request_id`, `service`, `severity`, e `timestamp` ISO-8601. O engine que opera o spoke inteiro precisa ser auditável — qualquer falha de provisionamento tem impacto operacional direto.

**Independent Test**: Pode ser testado capturando stdout de `RunFound`, validando que cada linha é JSON válido, e afirmando que os campos `spoke_id`, `step`, `action`, `severity`, `ts` estão presentes em cada linha; e que uma falha no passo N emite `severity: ERROR` com o campo `error` preenchido.

**Acceptance Scenarios**:

1. **Given** `RunFound` executando com sucesso, **When** os logs são inspecionados, **Then** cada passo emite exatamente duas linhas: `action: step_started` e `action: step_completed`, ambas com `spoke_id`, `step`, `severity: INFO`, `ts` em ISO-8601.
2. **Given** `RunFound` falhando no passo 8, **When** os logs são inspecionados, **Then** o passo 8 emite `action: step_failed` com `severity: ERROR` e `error` contendo a mensagem de falha completa sem truncamento.
3. **Given** um passo pulado por idempotência, **When** os logs são inspecionados, **Then** o passo emite `action: step_skipped` com `reason: already-done`.

---

### User Story 5 — Onboarding real no IdentityRegistry sem shortcut (Priority: P1)

Para `mode: found`, o banco central registra sua própria identidade on-chain no `IdentityRegistry` do spoke que acabou de fundar. O fluxo usa o endereço EVM gerenciado pelo `KeyProvider` (TK-2) e executa prova de posse via nonce assinado — o mesmo fluxo que `onboarding_proxy.go` (smart mode) implementa. O shortcut de auto-register não é utilizado.

**Why this priority**: A Constitution (Princípio IV) proíbe contornar o compliance gate. O audit D12 identificou que o `setup-spoke-*` atual pula o onboarding real. Corrigir isso no TK-5 é um requisito explícito do design doc.

**Independent Test**: Pode ser testado verificando que após `RunFound`, `IdentityRegistry.isParticipant(evmAddress)` retorna `true` para o endereço EVM do banco central, e que o endereço foi derivado via `KeyProvider.GetPublicKey()` — não de uma chave hardcoded ou gerada em arquivo.

**Acceptance Scenarios**:

1. **Given** um spoke recém-fundado com `IdentityRegistry` deployado, **When** o passo 9 executa, **Then** o endereço EVM do banco central (obtido via `KeyProvider.GetPublicKey(speakeID+"/cb")`) é registrado on-chain com `role: ROLE_CENTRAL_BANK` e `IdentityRegistry.isParticipant` retorna `true`.
2. **Given** o endereço já registrado (re-execução), **When** `check()` do passo 9 consulta o `IdentityRegistry` on-chain, **Then** retorna `true` sem tentar registrar novamente.
3. **Given** `KeyProvider` retornando erro em `GetPublicKey`, **When** o passo 9 tenta obter a chave, **Then** o engine falha no passo 9 com `ErrKeyProviderFailure` e não tenta nenhuma interação com o contrato.

---

### Edge Cases

- O que acontece quando o Besu do spoke ainda não está pronto quando o engine inicia (passo 1 falha por timeout de RPC)?
- O que acontece quando o Paladin não sobe após o restart no passo 5 (health check expira)?
- O que acontece quando `.deployed-addrs.env` existe mas está corrompido (chave presente com valor vazio)?
- O que acontece quando o relay está indisponível no passo 10 — o engine falha o spoke inteiro ou marca o passo como `pending` para retry? → O passo 10 é um passo normal (hard step): quando um `relay.endpoint` está configurado, o engine usa o `RelayRegistrar` HTTP e o passo FALHA a execução (`step_failed`) se o relay rejeitar ou estiver inacessível; como é o último passo, a execução reporta falha. O `check()` do passo retorna `false` quando o relay está indisponível, de modo que uma re-execução tenta novamente. Quando nenhum `relay.endpoint` está configurado, o injetor (TK-7) fornece `NoOpRelayRegistrar` e o passo conclui com sucesso como no-op (o registro no relay é simplesmente pulado).
- O que acontece quando o engine é executado por dois processos simultaneamente no mesmo `SPOKE_DATA_DIR`?
- O que acontece quando `SPOKE_DATA_DIR` não existe no momento da execução?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O engine DEVE expor a função pública `RunFound(ctx context.Context, manifest *Manifest, deps Deps) error` como único entry point para `mode: found`.
- **FR-002**: O engine DEVE executar os 10 passos na ordem definida; passos posteriores NUNCA devem ser executados se um passo anterior falhou.
- **FR-003**: Cada passo DEVE implementar `check(ctx) (bool, error)` que retorna `true` quando o passo já foi concluído com sucesso; `run(ctx)` só é invocado se `check` retornar `false`.
- **FR-004**: O engine DEVE persistir o estado de cada passo em `<SPOKE_DATA_DIR>/.provisioning-state.yaml` com campos `step`, `status` (`pending|done|failed`), e `completedAt` (ISO-8601); o estado `running` NUNCA é persistido — um passo que morreu mid-run é tratado como `pending` na próxima execução.
- **FR-005**: O engine DEVE invocar os scripts de deploy de contratos (passos 1, 4, 6, 7, 8) via `os/exec` passando `SPOKE=<spoke-id>` e demais variáveis de ambiente necessárias — sem reimplementar a lógica dos scripts.
- **FR-006**: O engine DEVE usar a interface `CertSource` (TK-3) para o passo 2 (geração de TLS) em vez de invocar `generate-certs.sh` diretamente.
- **FR-007**: O engine DEVE invocar `docker compose` via `os/exec` contra o template TK-4 (`provisioning/templates/central-bank/docker-compose.yaml`) para os subpassos de stop/clean/start do Paladin no passo 5.
- **FR-008**: O engine DEVE verificar a existência de `${SPOKE_DATA_DIR}/genesis/genesis.json` no início da execução; se o genesis não existir, o engine DEVE retornar `ErrGenesisNotFound` antes de executar qualquer passo — a responsabilidade de gerar o genesis é do template TK-4 (serviço `genesis-init`), não do engine.
- **FR-009**: O passo 9 DEVE usar o endereço EVM obtido via `KeyProvider.GetPublicKey()` (TK-2) e executar o fluxo real de prova de posse antes de chamar `IdentityRegistry.registerParticipant`; NUNCA usar o shortcut de auto-register.
- **FR-010**: O passo 10 DEVE registrar o spoke no relay via HTTP POST para `<manifest.relay.endpoint>/api/v1/spokes` com payload contendo `spoke_id`, `besu_rpc_url`, `besu_ws_url`, `htlc_address`, e `grpc_endpoint`.
- **FR-011**: O engine DEVE emitir logs JSON estruturados em stdout para cada transição de passo com os campos obrigatórios: `ts` (ISO-8601), `severity` (`INFO|ERROR`), `service: "orchestrator"`, `spoke_id`, `step` (nome do passo), e `action` (`step_started|step_completed|step_skipped|step_failed`). Falhas DEVEM incluir o campo `error` com a mensagem completa.
- **FR-012**: O engine DEVE respeitar o `context.Context` passado: cancelamento ou timeout do contexto DEVE interromper a execução no passo corrente e retornar `ctx.Err()`.
- **FR-013**: O engine DEVE ser thread-safe a nível de instância; múltiplas goroutines chamando `RunFound` com diferentes `SPOKE_DATA_DIR` DEVEM operar de forma independente; múltiplas goroutines chamando `RunFound` para o mesmo `SPOKE_DATA_DIR` DEVEM serializar via file lock.
- **FR-014**: Todos os timeouts por passo DEVEM ser configuráveis via `Deps.Timeouts`; os valores padrão são: passos 1/4/6/7/8 (subprocesso Go test) = 5 min; passo 5 (health-check Paladin) = 5 min com intervalo de 2 s; passo 9 (onboarding on-chain) = 2 min; passo 10 (relay) = 30 s.

### Sequência de passos

| # | Nome canônico | `check()` — condição de skip | `run()` — mecanismo |
|---|---|---|---|
| 1 | `deploy-contracts` | `REGISTRY_CONTRACT_ADDRESS`, `ZETO_FACTORY_ADDRESS`, `PENTE_FACTORY_ADDRESS` presentes em `.deployed-addrs.env` | `os/exec`: `go test -run TestDeploy{EVMRegistry,ZetoFactory,PenteFactory}` com `SPOKE=` e `BESU_RPC_URL=` |
| 2 | `gen-tls` | Arquivos de cert TLS já existem no caminho esperado em `SPOKE_DATA_DIR` | `CertSource.IssueLeafCert()` (TK-3) |
| 3 | `render-configs` | Arquivo `config.yaml` do Paladin CB já existe no caminho de destino | `os/exec`: `bash render-configs.sh` com `SPOKE=` |
| 4 | `register-nodes` | Arquivo de estado do passo `done` (não há consulta idempotente disponível no Paladin antes do start) | `os/exec`: `go test -run TestRegisterPaladinNodes` com `SPOKE=` e `BESU_RPC_URL=` |
| 5 | `start-paladin` | Paladin CB container em estado `running` E health check `ptx_getTransaction` retorna `PD020704` | `docker compose down` + `docker volume rm` + `docker compose up -d` + polling |
| 6 | `create-zeto-token` | `ZETO_TOKEN_ADDRESS` presente em `.deployed-addrs.env` | `os/exec`: `go test -run TestCreateZetoTokenInstance` com `SPOKE=` e `PALADIN_CB_URL=` |
| 7 | `create-pente-context` | `PENTE_CONTEXT_GROUP_ID` e `PENTE_CONTEXT_ADDRESS` presentes em `.deployed-addrs.env` | `os/exec`: `go test -run TestCreatePenteContextBilateral` com `SPOKE=` e `PALADIN_CB_URL=` |
| 8 | `deploy-fxa-pente` | `FX_AGREEMENT_DEPLOYED_AT` presente em `.deployed-addrs.env` | `os/exec`: `go test -run TestDeployFXAgreementPente` com `SPOKE=`, endereços do `.deployed-addrs.env`, e `PALADIN_CB_URL=` |
| 9 | `onboard-registry` | `IdentityRegistry.isParticipant(evmAddress)` retorna `true` on-chain | Chamada ao fluxo de onboarding real: `KeyProvider.GetPublicKey` → prova de posse → `IdentityRegistry.registerParticipant` |
| 10 | `register-relay` | `GET <relay.endpoint>/api/v1/spokes/<spoke-id>` retorna HTTP 200 | `POST <relay.endpoint>/api/v1/spokes` com payload do spoke |

### Key Entities

- **`Orchestrator`**: Struct Go que encapsula `RunFound`. Recebe `Deps` (dependências injetadas: `KeyProvider`, `CertSource`, caminhos de scripts, URL do relay) e o manifesto parseado.
- **`Manifest`**: Struct parseada do YAML de manifesto (TK-1). Campos relevantes: `spec.spoke.id`, `spec.node.rpc.port`, `spec.relay.endpoint`, `spec.keyProvider`, `spec.certSource`.
- **`Deps`**: Struct de dependências injetáveis: `KeyProvider` (TK-2), `CertSource` (TK-3), `ScriptsDir` (caminho base dos scripts Go test), `ComposeTemplatePath` (TK-4), `Timeouts`.
- **`StepState`**: Struct persistida em `.provisioning-state.yaml`. Campos: `step string`, `status string`, `completedAt string`.
- **`ProvisioningState`**: Struct raiz do arquivo YAML de estado. Campo `steps []StepState`.
- **`Step`**: Interface interna com métodos `Name() string`, `Check(ctx) (bool, error)`, `Run(ctx) error`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test ./scenario-a/toolkit/engine/orchestrator/...` passa com 100% dos testes, incluindo race detector habilitado (`go test -race`), antes de qualquer implementação ser considerada completa (test-first).
- **SC-002**: Uma execução de `RunFound` em ambiente local limpo com spoke-brl resulta em Paladin CB respondendo em `PALADIN_CB_URL` dentro de 10 minutos, com todos os 10 passos no estado `done` em `.provisioning-state.yaml`.
- **SC-003**: Uma segunda execução de `RunFound` sobre o mesmo `SPOKE_DATA_DIR` (todos os passos `done`) completa em menos de 2 segundos e não invoca nenhum subprocess (verificável por log: todos os passos emitem `action: step_skipped`).
- **SC-004**: `IdentityRegistry.isParticipant(evmAddress)` retorna `true` para o endereço EVM do banco central após a execução, onde o endereço foi obtido via `KeyProvider.GetPublicKey()`.
- **SC-005**: O relay responde HTTP 200 em `GET /api/v1/spokes/spoke-brl` após a execução completa do passo 10.
- **SC-006**: Cada linha de log emitida é JSON válido contendo os campos `ts`, `severity`, `service`, `spoke_id`, `step`, e `action`; falhas contêm adicionalmente `error`.
- **SC-007**: `RunFound` invocado com contexto cancelado (timeout) interrompe no passo corrente e retorna `context.DeadlineExceeded` — sem deixar subprocessos órfãos.
- **SC-008**: O arquivo `genesis.json` no `SPOKE_DATA_DIR` não é modificado em nenhum momento durante a execução do engine (hash SHA-256 idêntico antes e depois).
- **SC-009**: A execução do engine sobre um spoke já provisionado não produz entradas duplicadas no `IdentityRegistry` nem no relay.

## Assumptions

- Go 1.26+ com `os/exec`, `context`, `encoding/json`, `sync`, `net/http` da stdlib — sem dependências externas além das já presentes em `toolkit/go.mod`.
- O Besu do spoke está rodando e acessível em `BESU_RPC_URL` antes de `RunFound` ser chamado — o engine não é responsável por subir o Besu (isso é responsabilidade do template TK-4 e do TK-7).
- Os scripts Go test existentes em `deploy/local/paladin/scripts/` são invocados como subprocessos via `os/exec`; o engine não reimplementa sua lógica.
- A interface `KeyProvider` (TK-2) e `CertSource` (TK-3) já estão implementadas e testadas antes de TK-5 ser desenvolvido.
- O template Compose central-bank (TK-4) já existe em `provisioning/templates/central-bank/docker-compose.yaml` e passou em seus próprios testes.
- O relay Cacti expõe endpoints REST em `spec.relay.endpoint`; o schema exato do payload de registro será refinado em TK-6 e na integração com o relay.
- O arquivo de estado `.provisioning-state.yaml` é exclusivo do spoke (caminho relativo a `SPOKE_DATA_DIR`); não é compartilhado entre spokes distintos.
- `mode: join` (banco comercial, TK-9) é escopo de uma spec separada; o engine TK-5 implementa apenas `mode: found`.
- O emissor do join bundle (TK-6) é um componente separado que lê os artefatos produzidos por TK-5 — TK-5 não emite o bundle diretamente.
- A Fase 4 (prod) plugará implementações reais de `KeyProvider` e `CertSource` na mesma interface; o engine não muda.
- O engine reside em `scenario-a/toolkit/engine/orchestrator/`. Nenhum arquivo em `deploy/local/` ou `make/` é modificado.
