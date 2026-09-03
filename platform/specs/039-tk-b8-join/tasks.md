---
description: "Task list — TK-B8 (join: full node não-validador)"
---

# Tasks: Toolkit do Cenário B — join (full node não-validador) (TK-B8)

**Input**: Design documents from `/specs/039-tk-b8-join/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/join.md, quickstart.md

**Tests**: INCLUÍDOS (test-first). Steps/gate com **FakeRunner** + seams injetáveis (sem
Docker/Besu/Keycloak). Suíte **E2E** (build tag `e2e`) faz um banco `join` contra um spoke fundado —
**skip com aviso** se o ambiente/spoke bundle ausente.

**Organization**: por user story (US1–US3). Reusa o motor/exec/addrs/bundle(`LoadSpoke`)/apply do
TK-B6/B7 e `engine/pki.GenerateBankCSR`. Fluxo canônico (roadmap §6) — **sem** relay/noc.

**Paths**: módulo `scenario-b/toolkit`. Caminhos relativos à raiz do repositório.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: paralelizável (arquivo distinto, sem dependência pendente)
- **[Story]**: US1 / US2 / US3

---

## Phase 1: Setup

- [X] T001 Criar o fixture `scenario-b/toolkit/engine/manifest/testdata/bundles/spoke-a.bundle.yaml` (spoke bundle válido: `version`, `spokeId: spoke-a`, `chainId: 1338`, `enode`, `spokeRpc`/`spokeWs`, `genesis` inline mínimo, `contracts` com identityRegistry/tCeBM/spokeBridge/fCeBM) para que `join.yaml` (`joinBundleRef: ./bundles/spoke-a.bundle.yaml`) resolva no dispatch/dry-run

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: gate `wait-sync` + tipo `JoinConfig` usados pelas user stories.

**⚠️ CRITICAL**: `wait-sync` (US1) e `JoinConfig`/`WithDefaults` (todas as US) bloqueiam as US.

- [X] T002 [P] Teste em `scenario-b/toolkit/engine/orchestrator/sync_test.go`: `waitSync` conclui quando o `EthSyncing` fake retorna `syncing=false` e `block>0` (após uma sequência `syncing=true`); timeout com `EthSyncing` sempre-syncing → erro claro; servidor HTTP mock para `ethSyncing` (eth_syncing + eth_blockNumber)
- [X] T003 Implementar `scenario-b/toolkit/engine/orchestrator/sync.go`: seam `EthSyncing func(ctx, rpcURL) (syncing bool, block uint64, err error)`, default `ethSyncing` (POST `eth_syncing` + `eth_blockNumber`) e `waitSync(ctx, rpcURL, timeout, poll, EthSyncing)` (polling até `!syncing && block>0`; erro no timeout)
- [X] T004 Implementar em `scenario-b/toolkit/engine/orchestrator/step_join.go` o tipo `JoinConfig` (Runner, TemplatesDir, OutDir, BankID, Institution, SpokeID, SpokeChainID, BankRPC, SpokeBundlePath, GenesisDir, DataDir, BankEnvFile, KeycloakEnv; seams WaitRPC/WaitSync/WaitKeycloak/ReadClientSecret/EthSyncing) + `WithDefaults()`

**Checkpoint**: `go test ./engine/orchestrator/...` verde (wait-sync); `JoinConfig` compila.

---

## Phase 3: User Story 1 — Genesis do bundle + nó não-validador + sync (Priority: P1) 🎯 MVP

**Goal**: escrever o genesis do spoke (do bundle, guard não-destrutivo), subir o nó do banco (não-
validador) e aguardar a sincronização.

**Independent Test**: bundle inválido → erro antes de efeitos; `write-genesis` escreve o genesis do
bundle e pula em hash igual / erra em divergência; `start-besu-join` sobe o nó; `wait-sync` só conclui
sincronizado; o nó não é validador QBFT.

### Tests for User Story 1 ⚠️

- [X] T005 [US1] Teste em `scenario-b/toolkit/engine/orchestrator/step_join_test.go`: `consume-spoke-bundle` falha em bundle inválido/ausente; `write-genesis` (FakeRunner) escreve `SpokeBundle.Genesis` em `<GenesisDir>/genesis.json`, `Check` pula quando o `sha256` bate e **erra** em divergência; `start-besu-join` compõe `entity-besu` + `WaitRPC`; `wait-sync` chama `WaitSync` (fake)

### Implementation for User Story 1

- [X] T006 [US1] Implementar em `scenario-b/toolkit/engine/orchestrator/step_join.go` os steps `consume-spoke-bundle` (via `bundle.LoadSpoke`), `write-genesis` (escreve do bundle; `Check` por `sha256`, guard não-destrutivo, erro em divergência), `start-besu-join` (compose `entity-besu` não-validador, bootnode = enode do bundle, + `WaitRPC`) e `wait-sync` (`WaitSync`)

**Checkpoint**: genesis do banco = genesis do bundle; nó sobe e sincroniza (FakeRunner).

---

## Phase 4: User Story 2 — Wire endereços + Keycloak + serviços do banco (Priority: P1)

**Goal**: conectar os endereços de spoke (do bundle), provisionar o Keycloak do banco (write-back) e
subir infra/backend/frontend.

**Independent Test**: `wire-addresses` idempotente; `provision-keycloak-bank` write-back idempotente;
serviços montam compose.

### Tests for User Story 2 ⚠️

- [X] T007 [US2] Teste em `scenario-b/toolkit/engine/orchestrator/step_join_test.go`: `wire-addresses` escreve os endereços de spoke do bundle no `.env.bank` (idempotente, upsert); `provision-keycloak-bank` grava `KEYCLOAK_CLIENT_SECRET` (write-back) e `Check` idempotente

### Implementation for User Story 2

- [X] T008 [US2] Implementar em `scenario-b/toolkit/engine/orchestrator/step_join.go` os steps `wire-addresses` (endereços de spoke via `addrs.AppendAddr`), `provision-keycloak-bank` (+ write-back via `ReadClientSecret`), `render-bank-env`, `start-bank-infra`/`backend`/`frontend`

**Checkpoint**: serviços do banco montáveis; env conectado à cadeia do spoke.

---

## Phase 5: User Story 3 — Cauda diferida de PKI: gen-csr (Priority: P1)

**Goal**: gerar localmente par de chaves + CSR (`OU=ROLE_COMMERCIAL_BANK`, CN=`bankId`), chave `0600`,
nunca transmitida; zero material de CA.

**Independent Test**: `gen-csr` produz `{bank}.key` (`0600`) + `{bank}.csr`; idempotente (pula se
existem); pré-cria `<dataDir>/pki/`; **zero** `*-ca.key`/`*-ca.crt`.

### Tests for User Story 3 ⚠️

- [X] T009 [US3] Teste em `scenario-b/toolkit/engine/orchestrator/step_join_test.go`: `gen-csr` cria `{bankId}.key` (modo `0600`) e `{bankId}.csr` (`OU=ROLE_COMMERCIAL_BANK`, CN=`bankId`) em `<dataDir>/pki/`; `Check` pula se ambos existem; nenhum arquivo `*-ca.*` é criado; o dir `pki/` é pré-criado (não falha por permissão)

### Implementation for User Story 3

- [X] T010 [US3] Implementar o step `gen-csr` em `scenario-b/toolkit/engine/orchestrator/step_join.go`: `os.MkdirAll(<dataDir>/pki, 0o700)` (usuário do host) + `pki.GenerateBankCSR(BankID, Institution, <dataDir>/pki)`; `Check` = `{bank}.key` **e** `{bank}.csr` presentes

**Checkpoint**: CSR local gerado; chave `0600`; zero CA material; idempotente.

---

## Phase 6: Integration — JoinSteps + apply dispatch + CLI

- [X] T011 Teste em `scenario-b/toolkit/engine/orchestrator/step_join_test.go`: `JoinSteps(cfg)` monta o conjunto canônico na ordem/deps (consume → write-genesis → start-besu-join → wait-sync; consume → wire-addresses; wait-sync/wire → render → infra/backend/frontend; gen-csr na cauda); topoSort sem ciclo; **não** contém `register-relay-*` nem `add-noc-agent`
- [X] T012 Implementar `JoinSteps(cfg JoinConfig) []Step` (ordem/deps do fluxo canônico) em `scenario-b/toolkit/engine/orchestrator/step_join.go`
- [X] T013 Teste em `scenario-b/toolkit/engine/apply/apply_test.go`: dispatch `join` (dry-run → steps `planned`; sem efeitos); spoke bundle ausente/inválido → erro antes de efeito; **atualizar** `TestApplyJoinUnsupported` → `TestApplyJoinDryRun` (join deixou de ser "not supported yet")
- [X] T014 Implementar `applyJoin` em `scenario-b/toolkit/engine/apply/apply.go` (valida o spoke bundle cedo via `bundle.LoadSpoke`; monta `JoinConfig` do manifesto + flags; `--spoke-rpc` = RPC do nó do banco; runner dry/real) — trocar o `case "join"` ("not supported yet") pelo dispatch; adicionar `resolveJoinBundle(ref, manifestPath)`
- [X] T015 Atualizar `scenario-b/toolkit/cmd/cbweb3b/main.go` (doc/usage: `join` passa a ser suportado — remover "join is not supported yet (TK-B8)") e **atualizar** `scenario-b/toolkit/cmd/cbweb3b/main_test.go` (`TestApplyUnsupportedModeExits` usa `join.yaml` esperando exit 1 → converter para um teste de `join` dry-run com exit 0, ou repointar para um modo realmente não suportado)

**Checkpoint**: `apply join --dry-run` planeja o conjunto; `go test ./engine/apply/... ./cmd/cbweb3b/...` verde.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T016 Suíte E2E em `scenario-b/toolkit/tests/e2e/join_e2e_test.go` (build tag `e2e`): requer Docker/Besu + um spoke fundado (`CBWEB3B_E2E_JOIN_MANIFEST`, `CBWEB3B_E2E_REPO_ROOT`, `CBWEB3B_E2E_BANK_RPC`); ausente ⇒ `t.Skip` com aviso; caso presente, faz o `join`, valida a sincronização (`eth_syncing==false`) e o CSR gerado (SC-007)
- [X] T017 [P] `gofmt`/`go vet ./...` e suíte `cd scenario-b/toolkit && go test ./...` (+ `go build -tags e2e ./tests/e2e/...`); validar os passos do `quickstart.md`
- [X] T018 [P] Atualizar a nota de status TK-B8 em `scenario-b/docs/design/scenario-b-toolkit-roadmap.md` (§15) e a tabela do toolkit em `scenario-b/README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sem dependências (fixture do bundle).
- **Foundational (Phase 2)**: depois do Setup; `wait-sync` + `JoinConfig` bloqueiam US1–US3.
- **US1 (Phase 3)**: depois da Foundational (wait-sync). `consume-spoke-bundle` é base para US1/US2.
- **US2 (Phase 4)**: depois de US1 (consume p/ wire; wait-sync p/ keycloak/serviços).
- **US3 (Phase 5)**: independente de rede (PKI local); pode rodar em paralelo às US1/US2 no design,
  mas fica no mesmo `step_join.go` → sequencial no arquivo.
- **Integration (Phase 6)**: depois de US1–US3 (monta o conjunto + dispatch + CLI).
- **Polish (Phase 7)**: por último; E2E depende do join completo + spoke fundado.

### Within Each User Story / Notes

- Teste escrito e **falhando** antes da implementação.
- Os construtores de step ficam em `step_join.go` (mesmo arquivo) ⇒ US1–US3 são sequenciais nele;
  `sync.go` é arquivo distinto → `[P]` na Foundational.
- **Atenção (regressão)**: tornar `join` suportado quebra 2 testes do TK-B7 que afirmam "join not
  supported yet" — `apply_test.go::TestApplyJoinUnsupported` (T013) e
  `main_test.go::TestApplyUnsupportedModeExits` (T015). Ambos devem ser atualizados.

### Parallel Opportunities

- Foundational: T002/T003 (`sync.go`) [P] em relação ao `step_join.go`.
- Polish: T017/T018 [P].

---

## Implementation Strategy

### MVP First (US1)

1. Setup (fixture) + Foundational (wait-sync + JoinConfig).
2. US1 (consume + write-genesis + besu não-validador + wait-sync) → o banco sincroniza como full node.

### Incremental Delivery

1. Foundational → US1 → US2 → US3 → Integração (apply/CLI) → Polish (E2E, docs).
2. Cada US validável isolada com FakeRunner; a Integração amarra o `apply join`.

---

## Notes

- **Fluxo canônico (roadmap §6)**: sem `register-relay-*` e sem `add-noc-agent` (clarificação).
- **write-genesis ≠ gen-genesis**: escreve o genesis do bundle (guard não-destrutivo + `sha256`), não
  gera via `besu operator`.
- **PKI**: só `gen-csr` (chave `0600`, nunca transmitida, zero CA); assinatura/registro = runtime.
- **Não-validador**: CB é validador único (genesis `count: 1`); `node.validator: true` → warning (já
  em `manifest/validate.go`).
- Sem novas dependências Go; não importar `scenario-a/`; não alterar Makefiles/`deploy/local`.
- E2E: skip-com-aviso quando spoke/docker/besu ausentes (nunca falso verde).
