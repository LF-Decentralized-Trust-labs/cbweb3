# Tasks: TK-8 e TK-9 — Commercial Bank Join (`mode: join`)

**Input**: Design documents from `specs/032-commercial-bank-join/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Tests**: Incluídos — Constitution V exige test-first (Red-Green-Refactor). Testes devem ser escritos e falhar antes de cada implementação.

**Organization**: Tasks agrupadas por user story. US1 é MVP completo e independentemente testável.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode rodar em paralelo (arquivos diferentes, sem dependências de tasks incompletas)
- **[Story]**: User story correspondente (US1, US2, US3)
- Caminhos absolutos a partir de `scenario-a/toolkit/` ou `scenario-a/provisioning/`

---

## Phase 1: Setup (Estrutura e Scaffolding)

**Purpose**: Criar diretórios, arquivos de exemplo e dados de teste necessários para as fases seguintes.

- [X] T001 Criar estrutura de diretórios do template TK-8: `scenario-a/provisioning/templates/commercial-bank/scripts/` e `paladin-config/commercial-bank/`
- [X] T002 [P] Copiar `entry.sh` de `scenario-a/provisioning/templates/central-bank/scripts/entry.sh` para `scenario-a/provisioning/templates/commercial-bank/scripts/entry.sh` (reutilização sem alteração — ADR-001)
- [X] T003 [P] Criar manifesto de exemplo `scenario-a/toolkit/cmd/cbweb3/testdata/commercial-bank-brl.yaml` com `mode: join`, `joinBundleRef`, `bankId` conforme data-model.md

---

## Phase 2: Foundational (Pré-requisitos bloqueantes)

**Purpose**: Extensões de tipos e structs de que TODAS as fases dependem.

**⚠️ CRÍTICO**: Nenhuma implementação de step pode começar sem esta fase completa.

- [X] T004 Adicionar `ValidatorSpec` struct e campos `Validators []ValidatorSpec` + `CBEndpoint string` ao `BundleSpec` em `scenario-a/toolkit/engine/bundle/types.go` (conforme `contracts/bundle-extension.md`)
- [X] T005 [P] Adicionar testes unitários para os novos campos em `scenario-a/toolkit/engine/bundle/bundle_test.go`: verificar que bundle com `validators` vazio retorna erro de validação; verificar que bundle com `cbEndpoint` vazio retorna erro de validação
- [X] T006 [P] Adicionar `JoinDeps` e `JoinTimeouts` structs com defaults em `scenario-a/toolkit/engine/orchestrator/deps.go` (conforme data-model.md — campos: `KeyProvider`, `BankCode`, `Institution`, `ComposeTemplatePath`, `BackendComposePath`, `Timeouts JoinTimeouts`)
- [X] T007 [P] Adicionar constantes de nomes de passo join ao `scenario-a/toolkit/engine/orchestrator/step.go`: `StepWriteGenesis`, `StepStartBesuJoin`, `StepWaitSync`, `StepVoteQBFT`, `StepGenCSR`, `StepRequestCert`, `StepReceiveCert`, `StepProofPossession`, `StepStartBackend` (conforme research.md D8)

**Checkpoint**: Tipos e constantes definidos — implementação dos steps pode começar.

---

## Phase 3: User Story 1 — Join end-to-end (Priority: P1) 🎯 MVP

**Goal**: Operador executa `cbweb3 apply -f commercial-bank.yaml` e o banco comercial ingressa completamente no spoke (9 passos).

**Independent Test**: `cbweb3 apply -f testdata/commercial-bank-brl.yaml` contra spoke local TK-4/5 completa todos os passos sem erro; `qbft_getValidatorsByBlockNumber` inclui o endereço do joiner; IdentityRegistry contém o banco.

### Testes (escrever primeiro — devem FALHAR antes da implementação) ⚠️

- [X] T008 [P] [US1] Escrever teste `TestRunJoin_AllStepsDone` em `scenario-a/toolkit/engine/orchestrator/run_join_internal_test.go` — injeta 9 steps stub que retornam `done=true` no `Check()`, verifica que nenhum `Run()` é chamado
- [X] T009 [P] [US1] Escrever teste `TestWriteGenesisStep_NewFile` em `scenario-a/toolkit/engine/orchestrator/step_write_genesis_test.go` — verifica que genesis é escrito quando ausente e que hash SHA-256 é verificado
- [X] T010 [P] [US1] Escrever teste `TestWriteGenesisStep_ExistingHashMatch` e `TestWriteGenesisStep_ExistingHashMismatch` em `step_write_genesis_test.go` — mismatch deve retornar erro sem sobrescrever
- [X] T011 [P] [US1] Escrever teste `TestWaitSyncStep_ReachesTarget` em `scenario-a/toolkit/engine/orchestrator/step_wait_sync_test.go` — stub de `eth_blockNumber` incrementa; step completa quando altura atingida
- [X] T012 [P] [US1] Escrever teste `TestVoteQBFTStep_QuorumReached` e `TestVoteQBFTStep_QuorumNotReached` em `scenario-a/toolkit/engine/orchestrator/step_vote_qbft_test.go` — N=1 validador; quórum=1; falha quando validador inacessível
- [X] T013 [P] [US1] Escrever teste `TestGenCSRStep_ProducesKeyAndCSR` em `scenario-a/toolkit/engine/orchestrator/step_gen_csr_test.go` — verifica que `{bankCode}.key` (0600) e `{bankCode}.csr` são criados em SPOKE_DATA_DIR/pki/
- [X] T014 [P] [US1] Escrever teste `TestRequestCertStep_HTTP200` e `TestRequestCertStep_HTTP202` em `scenario-a/toolkit/engine/orchestrator/step_request_cert_test.go` — stub HTTP server; HTTP 200 persiste cert; HTTP 202 persiste request_id
- [X] T015 [P] [US1] Escrever teste `TestReceiveCertStep_AlreadyPresent` e `TestReceiveCertStep_PollsUntilReady` em `scenario-a/toolkit/engine/orchestrator/step_receive_cert_test.go` — skip se `.crt` existe; polling retorna cert na 2ª tentativa
- [X] T016 [P] [US1] Escrever teste `TestProofPossessionStep_RegistersIdentity` em `scenario-a/toolkit/engine/orchestrator/step_proof_possession_test.go` — stub de IdentityRegistry JSON-RPC; verifica chamada com endereço e assinatura corretos
- [X] T017 [P] [US1] Escrever testes de `pki.GenerateBankCSR`, `pki.SubmitCSRToCB` e `pki.StoreCertificate` em `scenario-a/toolkit/engine/pki/csr_test.go` — tabela de casos: bankCode inválido, outputDir sem permissão, HTTP 400 do CB

### Implementação do PKI (stubs em `pki/csr.go`)

- [X] T018 [P] [US1] Implementar `pki.GenerateBankCSR` em `scenario-a/toolkit/engine/pki/csr.go` — gerar ECDSA P-256, construir PKCS#10 CSR com `CN={bankCode}, OU=ROLE_COMMERCIAL_BANK, O={institution}, C=BR`, escrever `{bankCode}.key` (0600) e `{bankCode}.csr` em `outputDir`; nunca criar `*-ca.key`/`*-ca.crt`
- [X] T019 [P] [US1] Implementar `pki.SubmitCSRToCB` em `scenario-a/toolkit/engine/pki/csr.go` — POST JSON `{"csr_pem","bank_code","blockchain_pubkey"}` ao `cbURL`; retornar `certPEM` de HTTP 200 ou `""` com request_id de HTTP 202; falhar em 4xx/5xx sem retry (conforme `contracts/credential-request.md`)
- [X] T020 [P] [US1] Implementar `pki.StoreCertificate` em `scenario-a/toolkit/engine/pki/csr.go` — escrever `certPEM` em `{outputDir}/{bankCode}.crt` com permissão 0600

### Implementação dos steps

- [X] T021 [US1] Implementar `step_write_genesis.go` — `Check()`: verifica se `SPOKE_DATA_DIR/genesis/genesis.json` existe E hash SHA-256 bate com bundle; `Run()`: decodifica `bundle.Genesis.Content` (base64), escreve arquivo, verifica hash; aborta se hash de arquivo existente diverge (sem sobrescrita)
- [X] T022 [US1] Implementar `step_start_besu_join.go` — `Check()`: consulta health de `BESU_RPC_PORT` via `eth_blockNumber`; `Run()`: seta variáveis env TK-8 (SPOKE_ID, BANK_ID, BOOTNODE_ENODE, etc.), executa `docker compose -f {ComposeTemplatePath}/docker-compose.yaml up -d`
- [X] T023 [US1] Implementar `step_wait_sync.go` — `Check()`: sempre retorna `false` (sync não é idempotente por estado persistido; usa `done` do state); `Run()`: poll `eth_blockNumber` com intervalo `WaitSyncInterval` até altura ≥ target ou `WaitSync` timeout; loga progresso
- [X] T024 [US1] Implementar `step_vote_qbft.go` — `Check()`: consulta `qbft_getValidatorsByBlockNumber("latest")` e verifica se joiner address está presente; `Run()`: itera `bundle.Validators`, POST `qbft_proposeValidatorVote(joiner_addr, true)` em cada um, conta sucessos, verifica quórum ⌊N/2⌋+1; poll ativação com `VoteQBFTInterval` até joiner no validator set ou `VoteQBFT` timeout
- [X] T025 [US1] Implementar `step_gen_csr.go` — `Check()`: verifica se `SPOKE_DATA_DIR/pki/{bankCode}.csr` existe; `Run()`: chama `pki.GenerateBankCSR(bankCode, institution, dataDir+"/pki")`
- [X] T026 [US1] Implementar `step_request_cert.go` — `Check()`: verifica se `SPOKE_DATA_DIR/.cert-request-id` existe (request_id persistido) OU `SPOKE_DATA_DIR/tls/{bankCode}.crt` existe; `Run()`: lê CSR de `SPOKE_DATA_DIR/pki/{bankCode}.csr`, chama `pki.SubmitCSRToCB(csrPath, bundle.CBEndpoint, timeout)`; se HTTP 200 → armazena cert temporário; se HTTP 202 → persiste `request_id` e `poll_url` em `.cert-request-id`
- [X] T027 [US1] Implementar `step_receive_cert.go` — `Check()`: verifica se `SPOKE_DATA_DIR/tls/{bankCode}.crt` existe; `Run()`: se cert temporário (de T026 HTTP 200) → chama `pki.StoreCertificate`; senão → poll `poll_url` com `ReceiveCertInterval` até cert recebido ou `ReceiveCert` timeout; persiste via `pki.StoreCertificate`
- [X] T028 [US1] Implementar `step_proof_possession.go` — `Check()`: consulta IdentityRegistry via JSON-RPC `eth_call` para verificar se bankCode já está registrado; `Run()`: obtém blockchain pubkey via `KeyProvider.GetPublicKey(bankCode)`, gera nonce, assina via `KeyProvider.Sign(bankCode, nonce)`, submete transação ao IdentityRegistry (reutilizando padrão de `step_onboard_registry.go`)
- [X] T029 [US1] Implementar `step_start_backend.go` — `Check()`: verifica se container do backend está running via `docker inspect`; `Run()`: executa `docker compose -f {BackendComposePath} up -d`; loga nomes dos containers iniciados
- [X] T030 [US1] Implementar `RunJoin()` + `buildJoinSteps()` em `scenario-a/toolkit/engine/orchestrator/orchestrator.go` — análogo a `RunFound()`: valida bundle presente (retorna `ErrBundleNotFound` se ausente), adquire file lock, carrega state, constrói step list via `buildJoinSteps()`, executa loop idempotente de Check→Run→persist

### Template Compose TK-8

- [X] T031 [P] [US1] Escrever `scenario-a/provisioning/templates/commercial-bank/docker-compose.yaml` — serviços: `besu` (joiner, `BOOTNODE_ENODE` obrigatório, node data path `nodes/commercial-bank/data`, sem genesis-init), `paladin-data-init` (alpine init container de permissões), `paladin-bank` (imagem Paladin parametrizada por `BANK_ID`); variáveis conforme `contracts/compose-template.md`
- [X] T032 [P] [US1] Escrever `scenario-a/provisioning/templates/commercial-bank/paladin-config/commercial-bank/config.yaml.tmpl` — análogo ao template de `bank/config.yaml.tmpl` do TK-4, parametrizado por BANK_ID, BESU_RPC_PORT, SPOKE_DATA_DIR, PALADIN_GRPC_PORT

### Roteamento no CLI

- [X] T033 [US1] Estender `scenario-a/toolkit/engine/apply/apply.go` para rotear `spec.Mode == "join"` → `orchestrator.RunJoin(ctx, m, joinDeps)` via switch statement; retornar erro descritivo para mode desconhecido

**Checkpoint**: `cbweb3 apply -f commercial-bank-brl.yaml` executa o join completo. Testar end-to-end com spoke local. Verificar validator set e IdentityRegistry.

---

## Phase 4: User Story 2 — Re-apply idempotente (Priority: P2)

**Goal**: Re-executar `apply` após interrupção retoma do ponto de falha sem repetir passos concluídos.

**Independent Test**: Injetar falha no step 5 (`gen-csr`), verificar que re-run pula steps 1-4 com "skipped" e executa apenas steps 5-9.

### Testes ⚠️

- [X] T034 [P] [US2] Escrever teste `TestRunJoin_ResumeFromStep5` em `scenario-a/toolkit/engine/orchestrator/run_join_internal_test.go` — estado inicial tem steps 1-4 como `done`, steps 5-9 como `pending`; injetar step 5 que retorna erro; verificar que `Run()` foi chamado somente no step 5
- [X] T035 [P] [US2] Escrever teste `TestRunJoin_AllDone_NoRunCalled` — estado inicial com todos os 9 steps `done`; verificar que nenhum `Run()` é chamado e que o comando retorna `nil`
- [X] T036 [P] [US2] Escrever teste de integração `TestRunJoin_StatePersistedOnSuccess` em `scenario-a/toolkit/engine/orchestrator/orchestrator_integration_test.go` — usa stubs injetáveis; executa RunJoin completo; verifica `.provisioning-state.yaml` com 9 steps `done`

### Verificação

- [X] T037 [US2] Verificar que `Check()` de cada um dos 9 steps lê corretamente o `ProvisioningState` do disco (revisar implementações de T021-T029) — ajustar onde o check de estado persistido estiver faltando
- [X] T038 [US2] Verificar que o file lock (`.provisioning.lock`) é liberado corretamente em todos os caminhos de erro de RunJoin — adicionar testes para ErrProvisioningLocked em `run_join_internal_test.go`

**Checkpoint**: Re-run após qualquer interrupção é seguro. Nenhum passo é executado duas vezes.

---

## Phase 5: User Story 3 — Dry-run (Priority: P3)

**Goal**: `-dry-run` para `mode: join` lista os 9 passos com configurações resolvidas sem efeitos.

**Independent Test**: `cbweb3 apply -f commercial-bank.yaml -dry-run` produz saída com 9 linhas "would run: {step}" e nenhum container é criado, nenhum arquivo de estado é escrito, nenhuma transação on-chain.

### Testes ⚠️

- [X] T039 [US3] Escrever teste `TestApply_DryRun_JoinMode` em `scenario-a/toolkit/engine/apply/apply_test.go` — verifica que dry-run com `mode: join` emite saída de 9 passos e que nenhum side effect ocorre (nenhum arquivo criado em dataDir)

### Implementação

- [X] T040 [US3] Estender `scenario-a/toolkit/engine/apply/dryrun.go` para enumerar os 9 passos do join com nomes de constante e configurações resolvidas (BankID, BOOTNODE_ENODE do bundle, CBEndpoint, validators count) quando `spec.Mode == "join"`

**Checkpoint**: Dry-run para mode:join funcional. Operador pode pré-visualizar o plano sem efeitos.

---

## Phase Final: Polish e Cross-Cutting

**Purpose**: Qualidade, validação end-to-end e atualização do TK-6.

- [X] T041 [P] Escrever `scenario-a/provisioning/tests/test-commercial-bank-template.sh` — smoke test: sobe TK-8 compose com BOOTNODE_ENODE de spoke existente (ou stub), verifica que `eth_blockNumber` responde em ≤30s, verifica que genesis.json é lido corretamente (não gerado)
- [X] T042 [P] Atualizar `scenario-a/toolkit/engine/bundle/bundle.go` (`EmitBundle`) para popular `Validators` via `qbft_getValidatorsByBlockNumber("latest")` (lê addresses dos validadores ativos) e `CBEndpoint` via `manifest.Spec.Relay.Endpoint` ou campo `spec.cbEndpoint` do manifesto CB — adicionar mapeamento `validatorAddress → rpcUrl` em `BundleInput`
- [X] T043 [P] Escrever teste de integração `TestEmitBundle_PopulatesValidators` em `scenario-a/toolkit/engine/bundle/bundle_integration_test.go` — stub de `qbft_getValidatorsByBlockNumber` retorna 1 address; verifica que bundle emitido tem `validators[0].address` correto
- [X] T044 [P] Atualizar `scenario-a/toolkit/cmd/cbweb3/testdata/mode-join.yaml` para incluir campos obrigatórios do novo schema (`bankId`, `joinBundleRef` apontando para bundle com `validators` e `cbEndpoint`)
- [X] T045 Executar `go test ./...` em `scenario-a/toolkit/` e verificar que todos os testes passam
- [X] T046 [P] Validar quickstart.md (`specs/032-commercial-bank-join/quickstart.md`) end-to-end contra spoke local TK-4/5: found → bundle → join → verify

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sem dependências — pode iniciar imediatamente
- **Foundational (Phase 2)**: Depende do Setup — BLOQUEIA todos os steps
- **US1 (Phase 3)**: Depende do Foundational — todos os step tests/implementations começam aqui
  - Tests PKI (T017) podem começar em paralelo com tests de step (T008-T016)
  - Implementations PKI (T018-T020) podem começar em paralelo entre si
  - Step implementations (T021-T030) precisam das constantes (T007) e JoinDeps (T006)
  - `RunJoin()` (T030) depende de todos os steps (T021-T029)
  - Template TK-8 (T031-T032) é independente dos steps — pode iniciar em Phase 2
  - Apply routing (T033) depende de `RunJoin()` (T030)
- **US2 (Phase 4)**: Depende do US1 completo — testa o comportamento emergente de idempotência
- **US3 (Phase 5)**: Depende do routing (T033) mas pode iniciar em paralelo com US2
- **Polish (Final)**: Depende de US1, US2, US3 completos

### User Story Dependencies

- **US1 (P1)**: Pode iniciar após Foundational — MVP completo e independente
- **US2 (P2)**: Pode iniciar após US1 — valida o comportamento de state persistence já implementado
- **US3 (P3)**: Pode iniciar após T033 (routing) — independente de US2

### Oportunidades de Paralelismo na Phase 3

- T008-T017 (escrever todos os testes failing): 100% paralelos entre si
- T018-T020 (implementar pki stubs): paralelos entre si
- T031-T032 (template TK-8): paralelos com step implementations
- T021-T025 (steps 1-5): podem progredir em paralelo após as constantes (T007)

---

## Parallel Example: US1 — Escrita dos testes failing

```bash
# Lançar em paralelo (arquivos separados, sem dependências):
Task T008: "Escrever run_join_internal_test.go"
Task T009: "Escrever step_write_genesis_test.go"
Task T010: "Escrever step_write_genesis_test.go (hash cases)"  # mesmo arquivo que T009; sequencial
Task T011: "Escrever step_wait_sync_test.go"
Task T012: "Escrever step_vote_qbft_test.go"
Task T013: "Escrever step_gen_csr_test.go"
Task T014: "Escrever step_request_cert_test.go"
Task T015: "Escrever step_receive_cert_test.go"
Task T016: "Escrever step_proof_possession_test.go"
Task T017: "Escrever csr_test.go"
Task T031: "Escrever docker-compose.yaml do TK-8"
Task T032: "Escrever config.yaml.tmpl do Paladin"
```

---

## Implementation Strategy

### MVP First (US1)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational (CRÍTICO — bloqueia tudo)
3. Escrever testes failing de US1 (T008-T017) — **devem falhar agora**
4. Implementar PKI stubs (T018-T020) — testes pki passam
5. Implementar steps 1-9 (T021-T029) + RunJoin (T030)
6. Escrever template TK-8 (T031-T032) + routing (T033)
7. **PARAR E VALIDAR**: `cbweb3 apply -f commercial-bank.yaml` end-to-end
8. Deploy/demo com spoke local

### Incremental

1. Setup + Foundational → base pronta
2. US1 completo → banco comercial pode ingressar (MVP!)
3. US2 → re-apply seguro após falha
4. US3 → dry-run disponível
5. Polish → TK-6 atualizado, testes de integração, smoke test template

---

## Notes

- Testes DEVEM ser escritos e falhar antes de cada implementação (Constitution V)
- Cada task de step (`step_*.go`) tem um arquivo de teste correspondente (`step_*_test.go`)
- `[P]` = arquivos diferentes, sem dependências incompletas no mesmo arquivo
- Executar `go test ./engine/orchestrator/...` após cada step implementado
- Nunca criar `*-ca.key` ou `*-ca.crt` em código de banco comercial (constraint de `pki/csr.go`)
- O template TK-8 nunca gera genesis — é a diferença crítica em relação ao TK-4
- Commit após cada task ou grupo lógico (`/speckit-git-commit`)

---

## Phase 6: Convergence

> Lacunas entre intent (spec/plan/data-model) e a camada de manifesto/apply existente, não cobertas por T001–T046. O escopo de implementação principal (T001–T046) já está rastreado e não é re-anexado. Ordenado por severidade (HIGH → LOW).

- [X] T047 Resolver e propagar o identificador do banco comercial per FR-016 (missing): decidir a fonte do bank id — adicionar campo `BankID string yaml:"bankId"` a `manifest.Spec` em `scenario-a/toolkit/engine/manifest/types.go` **ou** derivar de `metadata.name`; implementar a escolha, validar em `manifest/validate.go` quando `mode:join`, e popular `JoinDeps.BankCode` (T006) + a variável de ambiente `BANK_ID` do template TK-8 (T031). Reconciliar a afirmação "nenhuma mudança de schema Go é necessária" em data-model.md §2. Atualizar T003/T044 conforme a decisão.
- [X] T048 Construir `JoinDeps` na camada apply per FR-017 (partial): estender `scenario-a/toolkit/engine/apply/` (apply.go + profile.go/deps.go) para, no ramo `mode:join`, carregar o join bundle a partir de `spec.joinBundleRef`, resolver o `ComposeTemplatePath` do template **commercial-bank** e o `BackendComposePath`, e montar `orchestrator.JoinDeps` (BankCode, KeyProvider, timeouts). Hoje `ResolveDeps`/`resolveLocalProfileFromInput` constroem apenas `orchestrator.Deps` (mode:found). Complementa T033 (que cobre apenas o switch de roteamento).
- [X] T049 Exigir `spec.joinBundleRef` em `mode:join` na validação per FR-001/FR-002 (missing): adicionar regra em `scenario-a/toolkit/engine/manifest/validate.go` que retorna erro claro quando `m.Spec.Mode == "join"` e `m.Spec.JoinBundleRef == ""`. Adicionar caso de teste em `validate_test.go`.
- [X] T050 Exigir `spec.node.dataDir` em `mode:join` na validação per FR-003 / data-model §2 (partial): estender a checagem em `scenario-a/toolkit/engine/manifest/validate.go:115` (hoje só `mode:found`) para também exigir `dataDir` quando `mode:join`, pois o engine de join o usa como `SPOKE_DATA_DIR`. Adicionar caso de teste em `validate_test.go`.
- [X] T051 Corrigir `scenario-a/toolkit/cmd/cbweb3/testdata/mode-join.yaml` per data-model §2 (partial): trocar `role: bank` (inválido — enum aceita `central-bank`/`commercial-bank`, falha na validação atual) por `role: commercial-bank` e alinhar `certSource` ao formato do schema. Independente de, e complementar a, T044.

---

## Phase 7: Convergence

> Lacunas pós-implementação entre intent (spec/FRs/contratos) e o código. Ordenado por severidade (HIGH → MEDIUM).

- [X] T052 Popular `cbEndpoint` no bundle emitido pelo found path per FR-015 / SC-001 / SC-002 (partial): hoje `runFoundMode` em `scenario-a/toolkit/engine/apply/apply.go` monta `bundle.BundleInput` sem `CBEndpoint`, e não há fonte de `cbEndpoint` no manifesto. Bundles emitidos por `cbweb3 apply` (found) saem com `cbEndpoint` vazio e são rejeitados por `bundle.ValidateForJoin` no `mode:join`, quebrando o ciclo found→bundle→join. Adicionar campo `spec.cbEndpoint` (opcional) a `manifest.Spec` em `engine/manifest/types.go`, propagá-lo via `ApplyInput`/profile, e setar `BundleInput.CBEndpoint` em `runFoundMode`. Adicionar teste cobrindo um bundle emitido que passa `ValidateForJoin`.
- [X] T053 Enviar `blockchain_pubkey` no POST de credential-request per FR-010 / contracts/credential-request.md (partial): o contrato especifica `blockchain_pubkey` no corpo, mas o passo `request-cert` (`step_request_cert.go` via `pki.SubmitCSRToCB`) envia apenas `csr_pem`+`bank_code`. Obter a pubkey via `JoinDeps.KeyProvider.GetPublicKey(bankCode)` e incluí-la no POST. Como a assinatura de `pki.SubmitCSRToCB` é fixada pelos testes do spec 016, adicionar uma função/variante no pacote `pki` (ou montar o POST no step) que aceite a pubkey sem quebrar os testes existentes. Adicionar teste verificando que o corpo inclui `blockchain_pubkey`.
