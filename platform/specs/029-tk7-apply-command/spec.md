# Feature Specification: TK-7 — Comando `apply`

**Feature Branch**: `029-tk7-apply-command`
**Created**: 2026-06-27
**Status**: Draft
**Input**: User description: "TK-7 — CRIAR: comando `apply` para o toolkit de provisionamento do Scenario A (Fase 1B)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Provisionamento completo com `cbweb3 apply -f manifest.yaml` (Priority: P1)

Um operador de banco central com um manifesto `mode: found, environment: local` válido executa `cbweb3 apply -f manifest.yaml`. O comando lê o manifesto, valida o schema, resolve implementações concretas (KeyProvider, CertSource, RelayRegistrar, BesuRPCURL) a partir dos campos do manifesto e das defaults do perfil `local`, e dispara `orchestrator.RunFound`. Ao final, chama `bundle.EmitBundle` e emite para stdout um relatório estruturado YAML/JSON com o resultado de cada passo. Sai com código 0.

**Why this priority**: É a interface operacional de toda a Fase 1B do toolkit. TK-1 a TK-6 existem para serem compostos aqui. Sem `apply`, o toolkit não tem ponto de entrada — é uma coleção de bibliotecas sem CLI.

**Independent Test**: Pode ser testado em integration test com Besu mockado (`httptest.Server`), `ScriptsDir` e `ComposeTemplatePath` apontando para diretórios de teste. Invocar `cbweb3 apply -f testdata/central-bank-brl.yaml` e verificar: (a) saída YAML válida em stdout; (b) `bundles/spoke-brl.bundle.yaml` existe no `outputDir`; (c) exit code 0.

**Acceptance Scenarios**:

1. **Given** manifesto `mode: found, environment: local` válido, `genesis.json` presente em `<dataDir>/genesis/`, e todos os serviços mockados respondendo com sucesso, **When** `cbweb3 apply -f manifest.yaml`, **Then** sai 0, stdout contém YAML com `status: success`, todos os 10 passos com `status: completed` ou `status: skipped`, e `bundles/<spoke-id>.bundle.yaml` foi criado.
2. **Given** spoke já 100% provisionado (`.provisioning-state.yaml` com todos os passos `done`), **When** `cbweb3 apply -f manifest.yaml` é re-executado, **Then** todos os passos mostram `status: skipped`, bundle é re-emitido deterministicamente, sai 0 — idempotente.
3. **Given** manifesto válido mas um passo falha no meio (ex: health-check do Paladin excede timeout), **When** `cbweb3 apply -f manifest.yaml`, **Then** sai não-zero, stdout contém o relatório com o passo falho mostrando `status: failed` e campo `error`, passos anteriores com `status: completed`, nenhum estado é corrompido.

---

### User Story 2 — `--dry-run` mostra o plano sem executar (Priority: P1)

`cbweb3 apply --dry-run -f manifest.yaml` (ou `-n`) lê o manifesto, valida o schema, lê o arquivo de estado (`.provisioning-state.yaml`) para determinar o que já foi feito, e imprime um relatório de plano com `dry-run: true`. Cada passo mostra `status: pending` (seria executado) ou `status: skipped` (já concluído). Nenhum side-effect: nenhum passo é executado, nenhum bundle é emitido, nenhum estado é escrito ou alterado.

**Why this priority**: Operadores precisam inspecionar o plano antes de executar — especialmente em staging/prod. Dry-run é também o principal instrumento de diagnóstico quando o provisionamento fica parado num passo.

**Independent Test**: Executar com `SPOKE_DATA_DIR` vazio e verificar: (a) saída com `dry-run: true`, todos os passos `pending`; (b) nenhum arquivo criado no `SPOKE_DATA_DIR`; (c) exit 0. Executar com estado parcial e verificar que passos concluídos aparecem `skipped`.

**Acceptance Scenarios**:

1. **Given** `SPOKE_DATA_DIR` vazio e manifesto `mode: found` válido, **When** `cbweb3 apply --dry-run -f manifest.yaml`, **Then** sai 0, stdout contém `dry-run: true`, todos os 10 passos com `status: pending`, nenhum arquivo criado em `SPOKE_DATA_DIR`, nenhum bundle emitido.
2. **Given** `.provisioning-state.yaml` com 5 de 10 passos `done`, **When** `--dry-run`, **Then** os primeiros 5 passos mostram `status: skipped` e os últimos 5 mostram `status: pending` — nenhum passo é executado.
3. **Given** manifesto com erro de validação (campo `spec.spoke.id` ausente), **When** `cbweb3 apply --dry-run -f manifest.yaml`, **Then** sai 1 com erro de validação identificando o campo — inspeção de estado não chega a ocorrer.
4. **Given** dry-run e Besu **não** disponível, **When** `--dry-run`, **Then** sai 0 sem tentar conexão com Besu — dry-run não acessa serviços externos.

---

### User Story 3 — Saída estruturada (JSON ou YAML) para automação (Priority: P1)

O comando emite para stdout um relatório de execução estruturado no formato JSON ou YAML (controlado por `--output json|yaml`, padrão `yaml`). O relatório contém: `spoke` (id), `mode`, `dryRun` (bool), `status` (`success|failed|dry-run`), e um array `steps` com `{name, status, completedAt?, error?}`. Em caso de sucesso com `mode: found`, inclui `bundle.path`. Erros descritivos vão para stderr (exceto nos campos `error` do relatório).

**Why this priority**: O comando `apply` deve ser scriptável em pipelines CI/CD. Saída não-estruturada não pode ser parseada de forma confiável. JSON/YAML é o contrato para integração com orquestradores externos e scripts de automação.

**Independent Test**: `cbweb3 apply -f manifest.yaml --output json | jq .status` deve retornar `"success"`. `cbweb3 apply -f manifest.yaml --output yaml` deve produzir YAML sintáticamente válido que `yq .status` parseia corretamente.

**Acceptance Scenarios**:

1. **Given** execução bem-sucedida com `--output json`, **When** stdout é parseado por `jq`, **Then** `jq .status == "success"`, `jq '.steps | length == 10'`, cada step tem `name` e `status`, `jq .bundle.path` aponta para o arquivo emitido.
2. **Given** `--output yaml` (default), **When** stdout é parseado por `yq`, **Then** `yq .status == "success"` e o YAML é sintaticamente válido.
3. **Given** `--output invalid-format`, **When** `cbweb3 apply --output invalid-format -f manifest.yaml`, **Then** sai 1 com "unknown output format: invalid-format" em stderr antes de ler o manifesto.
4. **Given** execução com falha de passo, **When** `--output json`, **Then** stdout ainda é JSON válido com `status: failed`, campo `error` no passo falho, e exit code não-zero — stdout nunca quebra o formato.

---

### User Story 4 — Falha rápida em manifesto inválido (Priority: P1)

Se o manifesto estiver ausente, com YAML malformado, ou reprovado pela validação de schema, `apply` sai imediatamente com exit 1 e mensagem clara identificando o problema. Nenhum passo de provisionamento começa. Nenhum estado é escrito.

**Why this priority**: Previne provisionamento parcial a partir de manifesto inválido. Qualquer erro detectável antes da execução deve ser capturado antes de qualquer side-effect — princípio de fail-fast com erro claro.

**Independent Test**: Passar manifesto sem `spec.mode` e verificar exit 1 com mensagem identificando o campo ausente, antes de qualquer passo executar.

**Acceptance Scenarios**:

1. **Given** `-f /nonexistent.yaml`, **When** `cbweb3 apply -f /nonexistent.yaml`, **Then** sai 1, stderr contém "manifest file not found: /nonexistent.yaml", nenhum arquivo criado.
2. **Given** YAML com sintaxe inválida (aspas desbalanceadas), **When** `apply`, **Then** sai 1 com erro de parse YAML em stderr, nenhum provisionamento iniciado.
3. **Given** YAML válido mas `spec.spoke.id` ausente, **When** `apply`, **Then** sai 1, stderr contém "validation error: spec.spoke.id is required", nenhum provisionamento.
4. **Given** manifesto com `spec.mode: join`, **When** `apply`, **Then** sai 1 com "`mode: join` not yet supported — will be implemented in TK-9", nenhum provisionamento.
5. **Given** flag `-f` omitida, **When** `cbweb3 apply`, **Then** sai 1 com "required flag -f (--file) is missing".

---

### User Story 5 — Resolução de dependências a partir do manifesto e perfil (Priority: P2)

O comando constrói o `orchestrator.Deps` completo a partir do manifesto e do perfil `environment`. Para `environment: local`: `KeyProvider` via `keyprovider.New(spec.keyProvider)`, `CertSource` via `certsource.New(spec.certSource)`, `RelayRegistrar` via `orchestrator.NewHTTPRelayRegistrar(spec.relay.endpoint)`, `BesuRPCURL` como `http://localhost:<spec.node.rpc.port>`. Paths de runtime (ScriptsDir, ComposeTemplatePath, PaladinConfigTemplateDir) são resolvidos de variáveis de ambiente com defaults relativos ao binário. `environment: prod` rejeita com mensagem clara.

**Why this priority**: A resolução incorreta de Deps silencia erros que só se manifestariam durante o provisionamento. Fail-fast com URI inválida previne uma categoria de erros difíceis de diagnosticar.

**Acceptance Scenarios**:

1. **Given** `environment: local` e `keyProvider: kms://local-emulator`, **When** `apply`, **Then** engine recebe instância de `LocalKeyProvider` (não nil); sem erro de resolução.
2. **Given** `certSource: self-signed://local`, **When** `apply`, **Then** engine recebe `LocalCertSource` (não nil).
3. **Given** `relay.endpoint: http://cbweb3-cacti:4000`, **When** `apply`, **Then** `RelayRegistrar` usa esse endpoint.
4. **Given** `environment: prod`, **When** `apply`, **Then** sai 1 com "environment `prod` is not yet supported in TK-7 — only `local` is available".
5. **Given** `keyProvider: kms://unknown-scheme`, **When** `apply`, **Then** sai 1 com "keyProvider URI error: ..." antes de iniciar qualquer passo.

---

### Edge Cases

- O que acontece quando `SPOKE_DATA_DIR` não existe? → `apply` cria o diretório via `os.MkdirAll` antes de iniciar; spoke em diretório novo = spoke fresco. Não é erro.
- O que acontece quando o contexto é cancelado (Ctrl+C / SIGTERM)? → Signal handler cancela o contexto; o engine propaga `ctx.Err()`; o relatório mostra qual passo estava em andamento com `status: interrupted`; exit não-zero.
- O que acontece quando `--output json` e a execução falha? → stdout ainda é JSON válido com `status: failed`; exit não-zero. O formato de saída nunca é quebrado por erros.
- O que acontece quando a emissão do bundle falha após todos os 10 passos do engine concluírem? → Relatório mostra todos os passos `completed`, campo `bundleError` no topo, exit não-zero. O spoke está provisionado mas o bundle não foi escrito; o operador pode re-executar `apply` para re-emitir.
- O que acontece quando `relay.endpoint` está ausente no manifesto para `mode: found`? → `orchestrator.RunFound` usa `NoOpRelayRegistrar` (o passo `register-relay` é soft-failure por design); o relatório mostra `status: failed` no passo register-relay mas a execução continua — comportamento herdado do engine.
- O que acontece quando múltiplas instâncias de `cbweb3 apply` rodam concorrentemente para o mesmo spoke? → O file lock em `.provisioning-state.yaml` (TK-5) retorna `ErrProvisioningLocked`; o segundo processo sai com exit não-zero e mensagem clara.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O binário CLI DEVE ser nomeado `cbweb3` e construído a partir de `scenario-a/toolkit/cmd/cbweb3/main.go`.
- **FR-002**: O subcomando `apply` DEVE aceitar `-f <path>` (alias `--file <path>`) como flag obrigatória identificando o arquivo YAML do manifesto.
- **FR-003**: O subcomando `apply` DEVE aceitar `--dry-run` (alias `-n`) como flag booleana opcional. Default: false.
- **FR-004**: O subcomando `apply` DEVE aceitar `--output <format>` (alias `-o`) com valores válidos `json` e `yaml`. Default: `yaml`.
- **FR-005**: O comando DEVE parsear o manifesto usando `engine/manifest` (parse + validate) e sair 1 com mensagem clara em stderr em qualquer erro de validação, antes de qualquer efeito colateral.
- **FR-006**: Para `mode: found`, o comando DEVE chamar `orchestrator.RunFound` com `Deps` resolvido. Para qualquer outro `mode`, DEVE sair 1 com mensagem "not yet supported" antes de qualquer acesso ao sistema de arquivos.
- **FR-007**: O comando DEVE resolver `KeyProvider` via `keyprovider.New(spec.keyProvider)` e `CertSource` via `certsource.New(spec.certSource)`. Erro de resolução → sair 1.
- **FR-008**: Em modo `--dry-run`, o comando NÃO DEVE chamar `orchestrator.RunFound` nem `bundle.EmitBundle`. DEVE ler `.provisioning-state.yaml` (se existir) para determinar estado atual de cada passo e produzir relatório de plano com `dry-run: true`.
- **FR-009**: Após `orchestrator.RunFound` completar com sucesso, o comando DEVE chamar `bundle.EmitBundle` para `mode: found`.
- **FR-010**: O relatório estruturado de saída DEVE conter: `spoke` (string), `mode` (string), `dryRun` (bool), `status` (`success|failed|dry-run`), `steps` (array de `{name, status, completedAt?, error?}`), e `bundle.path` (string, apenas em sucesso com `mode: found`).
- **FR-011**: Exit code DEVE ser 0 em sucesso e em dry-run bem-sucedido; não-zero em qualquer erro (parse, validação, engine, bundle).
- **FR-012**: O comando DEVE instalar handler de sinais (SIGINT, SIGTERM) que cancela o contexto e garante que o relatório parcial seja emitido antes de sair.
- **FR-013**: Saída estruturada DEVE sempre ser JSON ou YAML válido em stdout. Mensagens de erro humano-legíveis que não fazem parte do relatório DEVEM ir exclusivamente para stderr.
- **FR-014**: Apenas `environment: local` é suportado em TK-7. Qualquer outro valor DEVE causar sair 1 com mensagem listando o suportado.
- **FR-015**: Paths de runtime (`ScriptsDir`, `ComposeTemplatePath`, `PaladinConfigTemplateDir`) DEVEM ser resolvíveis via variáveis de ambiente (`CBWEB3_SCRIPTS_DIR`, `CBWEB3_COMPOSE_TEMPLATE`, `CBWEB3_PALADIN_CONFIG_DIR`) com defaults relativos à localização do binário. Nunca hardcoded como paths absolutos.
- **FR-016**: O binário DEVE ser buildável com `go build ./cmd/cbweb3/...` usando apenas dependências já presentes em `toolkit/go.mod` (zero novas dependências externas).

### Key Entities *(include if feature involves data)*

- **`ApplyResult`**: Relatório estruturado emitido em stdout. Campos: `spoke` (string), `mode` (string), `dryRun` (bool), `status` (`success|failed|dry-run`), `steps` ([]StepResult), `bundle` (*BundleRef, nil em dry-run ou falha), `error` (string, nil em sucesso).
- **`StepResult`**: Resultado por passo. Campos: `name` (string), `status` (`completed|skipped|failed|pending|interrupted`), `completedAt` (string RFC-3339, omitempty), `error` (string, omitempty).
- **`BundleRef`**: Referência ao bundle emitido. Campos: `path` (string — `bundles/<spoke-id>.bundle.yaml`).
- **`LocalProfile`**: Defaults para `environment: local`. Resolve `BesuRPCURL` como `http://localhost:<node.rpc.port>`, `RelayRegistrar` via `NewHTTPRelayRegistrar(relay.endpoint)`. Paths de runtime lidos de env vars com fallback relativo ao binário.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test -race ./cmd/cbweb3/...` passa **antes** de qualquer implementação ser escrita (red-green cycle exigido pela Constitution, Princípio V).
- **SC-002**: `cbweb3 apply -f testdata/central-bank-brl.yaml` provisionando um spoke local completo termina com exit 0, stdout YAML válido com `status: success`, e `bundles/spoke-brl.bundle.yaml` presente e com todos os campos obrigatórios.
- **SC-003**: `cbweb3 apply --dry-run -f testdata/central-bank-brl.yaml` completa em < 2 s sem acesso a Besu ou serviços externos — dominado apenas por leitura de arquivo de estado.
- **SC-004**: `cbweb3 apply -f manifest.yaml --output json | jq .status` retorna `"success"` sem erro — stdout é sempre JSON válido quando `--output json`.
- **SC-005**: Re-execução de `cbweb3 apply -f manifest.yaml` em spoke 100% provisionado: exit 0, todos os passos `skipped`, bundle re-emitido — idempotência comprovada.
- **SC-006**: Manifesto com campo obrigatório ausente causa exit 1 com mensagem identificando o campo — nenhum passo de provisionamento começa, nenhum arquivo é criado.
- **SC-007**: `go build ./cmd/cbweb3/...` bem-sucedido com zero novas dependências em `toolkit/go.mod`.

## Assumptions

- `engine/manifest` (TK-1): `Parse(path string) (*Manifest, error)` e `Validate(m *Manifest) error` estão implementados e estáveis (`engine/manifest/validate.go` e `types.go` verificados).
- `orchestrator.RunFound` (TK-5) e `bundle.EmitBundle` (TK-6) estão implementados com contratos públicos estáveis.
- `keyprovider.New(uri string) (KeyProvider, error)` e `certsource.New(uri string) (CertSource, error)` existem e funcionam (verificados em `engine/keyprovider/factory.go` e `engine/certsource/factory.go`).
- `orchestrator.NewHTTPRelayRegistrar(endpoint string) RelayRegistrar` existe (verificado em `engine/orchestrator/deps.go`).
- O dry-run lê `.provisioning-state.yaml` diretamente (via `loadState`) para determinar estado dos passos; não acessa Besu nem serviços externos.
- `mode: join` está fora do escopo de TK-7 e será implementado em TK-9 (Fase 3).
- `environment: prod` (KMS real + CA real) está fora do escopo de TK-7 e será implementado na Fase 4 (PR-1, PR-2).
- O CLI não é interativo por design — nenhum prompt ao usuário.
- A lógica de `apply` suficientemente complexa (resolução de Deps, construção do relatório, dry-run) vive em `engine/apply/` (pacote separado) para ser testável de forma isolada; `cmd/cbweb3/main.go` é thin wrapper de flag parsing e chamada ao pacote `apply`.
