# Implementation Plan: TK-5 — Motor de orquestração (`mode: found`)

**Branch**: `027-tk5-orchestration-engine` | **Date**: 2026-06-27 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/027-tk5-orchestration-engine/spec.md`

---

## Summary

Implementar o motor de orquestração Go que reproduz a sequência `setup-spoke-*` do `make/40-paladin.mk` como código executável, idempotente e parametrizado por spoke. O engine expõe `RunFound(ctx, manifest, deps) error` que executa 10 passos em sequência — deploy de contratos, TLS, configs Paladin, registro de nós, start/health-check do Paladin, token Zeto, contexto Pente, FXAgreement no Pente, onboarding real no IdentityRegistry, e registro no relay — com verificação de idempotência por passo via fonte de verdade externa (`.deployed-addrs.env`, filesystem, on-chain, HTTP). Estado de execução persiste em `.provisioning-state.yaml`.

---

## Technical Context

**Language/Version**: Go 1.26+  
**Module**: `github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit`  
**Primary Dependencies**:
- `github.com/ethereum/go-ethereum v1.17.1` — já em `toolkit/go.mod`; usado para `ethclient` (passo 9)
- `gopkg.in/yaml.v3 v3.0.1` — já em `toolkit/go.mod`; usado para `.provisioning-state.yaml`
- stdlib: `os/exec`, `context`, `encoding/json`, `net/http`, `sync`, `text/template`, `syscall`

**Storage**: Filesystem — `<SPOKE_DATA_DIR>/.provisioning-state.yaml`, `<SPOKE_DATA_DIR>/.deployed-addrs.env`  
**Testing**: `go test -race ./engine/orchestrator/...`  
**Target Platform**: Linux (Docker Compose v2 disponível no host)  
**Project Type**: Library Go — consumida pelo TK-7 (CLI `apply`)  
**Performance Goals**: Re-execução idempotente < 2 s; primeira execução completa < 10 min  
**Constraints**: Nenhuma dependência externa nova além do que já está em `toolkit/go.mod`; genesis nunca regenerado pelo engine

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Princípio I — Scenario-Scoped Independence ✅

Todo o código novo reside em `scenario-a/toolkit/engine/orchestrator/` e `scenario-a/provisioning/templates/central-bank/`. Nenhum arquivo de `scenario-b/` é tocado. Nenhum arquivo de `deploy/local/` ou `make/` é modificado.

### Princípio II — Privacy by Design ✅

O engine orquestra o Paladin (Zeto + Pente) mas não bypassa os mecanismos de privacidade. Os passos 6–8 criam a infra ZKP (token Zeto, contexto Pente, FXAgreement-em-Pente). O engine não processa valores de transferência nem identidades de contraparte em plaintext.

### Princípio III — Atomic Settlement Guarantee ✅

O engine é infraestrutura de provisionamento, não de liquidação. Ele não altera contratos HTLC nem o fluxo de settlement. Os contratos deployados no passo 1 (IdentityRegistry, ZetoFactory, PenteFactory) são pré-requisitos para o settlement mas não interferem com ele.

### Princípio IV — Compliance Gate Before Participation ✅

O passo 9 implementa o onboarding real no IdentityRegistry (`registerParticipant` on-chain via prova de posse). O shortcut de auto-register **não é usado**. O engine é o primeiro ponto a garantir que o CB registra-se corretamente antes de qualquer atividade na rede. Ver FR-009 e User Story 5.

### Princípio V — Test-First at Every Layer ✅

SC-001 exige que `go test -race ./engine/orchestrator/...` passe antes de qualquer implementação ser considerada completa. O plano (Fase 1) define testes failing primeiro para cada passo (`check()` e `run()`), seguidos da implementação. Testes unitários usam mocks para isolar subprocessos e Docker.

### Princípio VI — Observability and Auditability ✅

FR-011 define o contrato de log: cada transição de passo emite uma linha JSON com `ts` (ISO-8601), `severity`, `service`, `spoke_id`, `step`, `action`. Falhas incluem `error`. O contrato de log está documentado em `contracts/orchestrator-api.md`. Silent failures são proibidos — cada erro de passo é logado antes de propagar.

**Constitution Check pós-design**: Todos os 6 princípios satisfeitos. Nenhuma violação. Nenhuma entrada na tabela de Complexity Tracking necessária.

---

## Project Structure

### Documentation (this feature)

```text
specs/027-tk5-orchestration-engine/
├── plan.md              ← este arquivo
├── research.md          ← Phase 0: 8 decisões de design
├── data-model.md        ← Phase 1: entidades, structs, invariantes
├── quickstart.md        ← Phase 1: uso e exemplos
├── contracts/
│   └── orchestrator-api.md  ← Phase 1: RunFound, erros, log contract, step names
└── tasks.md             ← Phase 2 (gerado por /speckit.tasks)
```

### Source Code — novo código

```text
scenario-a/toolkit/engine/orchestrator/
├── orchestrator.go          # RunFound, Orchestrator struct, DefaultTimeouts
├── state.go                 # ProvisioningState, StepState, load/save/lock
├── step.go                  # Step interface, StepDeployContracts...StepRegisterRelay constants
├── deps.go                  # Deps, Timeouts, SpokeInfo, RelayRegistrar, NoOpRelayRegistrar
├── logger.go                # logLine() — emit JSON to stdout
├── addrs.go                 # DeployedAddrs, parseDeployedAddrs()
├── steps/
│   ├── deploy_contracts.go  # passo 1: check(.deployed-addrs.env) + run(os/exec go test)
│   ├── gen_tls.go           # passo 2: check(cert file exists) + run(CertSource.IssueLeafCert)
│   ├── render_configs.go    # passo 3: check(config.yaml exists) + run(text/template)
│   ├── register_nodes.go    # passo 4: check(state file) + run(os/exec go test)
│   ├── start_paladin.go     # passo 5: check(docker+HTTP) + run(compose down/vol rm/up + poll)
│   ├── create_zeto.go       # passo 6: check(.deployed-addrs.env) + run(os/exec go test)
│   ├── create_pente.go      # passo 7: check(.deployed-addrs.env) + run(os/exec go test)
│   ├── deploy_fxa.go        # passo 8: check(.deployed-addrs.env) + run(os/exec go test)
│   ├── onboard_registry.go  # passo 9: check(on-chain isParticipant) + run(go-ethereum tx)
│   └── register_relay.go    # passo 10: check(HTTP GET) + run(RelayRegistrar.Register)
├── orchestrator_test.go     # testes de integração: RunFound end-to-end (com mocks)
├── state_test.go            # testes unitários: load/save/lock
├── addrs_test.go            # testes unitários: parseDeployedAddrs
└── steps/
    ├── deploy_contracts_test.go
    ├── gen_tls_test.go
    ├── render_configs_test.go
    ├── register_nodes_test.go
    ├── start_paladin_test.go
    ├── create_zeto_test.go
    ├── create_pente_test.go
    ├── deploy_fxa_test.go
    ├── onboard_registry_test.go
    └── register_relay_test.go

scenario-a/provisioning/templates/central-bank/
├── docker-compose.yaml      # existente (TK-4) — NÃO modificar
├── paladin-compose.yaml     # NOVO: template parametrizado dos nós Paladin
└── paladin-config/
    ├── central-bank/
    │   └── config.yaml.tmpl # NOVO: template config do nó CB
    └── bank/
        └── config.yaml.tmpl # NOVO: template config genérico de nó banco
```

**Structure Decision**: Single Go module (`scenario-a/toolkit`) com novo package `engine/orchestrator`. Nenhum novo módulo Go. Templates Paladin em `provisioning/templates/central-bank/` ao lado do template Besu existente. Código de `deploy/local/` e `make/` preservado intacto como rede de referência.

---

## Complexity Tracking

> Nenhuma violação da Constitution identificada. Tabela não aplicável.

---

## Phases

### Fase 0 — Research ✅ (concluída)

Ver [research.md](./research.md). Todas as 8 questões resolvidas:

| ID | Decisão |
|----|---------|
| R-01 | Criar `paladin-compose.yaml` parametrizado em `provisioning/templates/central-bank/` |
| R-02 | Usar go-ethereum `ethclient` + ABI para `registerParticipant` (passo 9) |
| R-03 | Rendering de configs Paladin em Go (`text/template`) — não invocar `render-configs.sh` |
| R-04 | `syscall.Flock` para file lock em `.provisioning.lock` |
| R-05 | Interface `RelayRegistrar` injetável com graceful fallback (`NoOpRelayRegistrar`) |
| R-06 | Templates Paladin em `provisioning/templates/central-bank/paladin-config/` |
| R-07 | Passo 5: `stop → clean_volumes → start`; clean apenas na primeira execução |
| R-08 | Passo 4: único passo com check exclusivo no arquivo de estado |

---

### Fase 1 — Implementação

#### Pré-condição obrigatória (Constitution V — test-first)

Para cada passo, o ciclo é: **escrever teste failing → implementar `check()` → implementar `run()` → verde**.

---

#### 1.0 — Scaffolding do package

- Criar `scenario-a/toolkit/engine/orchestrator/` com arquivos de base vazios
- Definir `Step` interface, constantes de nomes, tipos `Deps`, `Timeouts`, `SpokeInfo`, `RelayRegistrar`, `NoOpRelayRegistrar`
- Implementar `logger.go` com `logLine(w io.Writer, entry logEntry)`
- Implementar `addrs.go` com `parseDeployedAddrs(path string) (DeployedAddrs, error)`
- Implementar `state.go`:
  - `loadState(path) (ProvisioningState, error)` — retorna estado com todos os passos `pending` se arquivo não existe
  - `saveState(path, state) error` — escrita atômica via temp file + `os.Rename`
  - `lockState(dir) (unlock func(), error)` — `syscall.Flock(LOCK_EX|LOCK_NB)`
- **Testes unitários**: `state_test.go`, `addrs_test.go`

---

#### 1.1 — Passo 1: `deploy-contracts`

**`check()`**: lê `.deployed-addrs.env`; retorna `true` se `REGISTRY_CONTRACT_ADDRESS`, `ZETO_FACTORY_ADDRESS`, `PENTE_FACTORY_ADDRESS` são todos não-vazios.

**`run()`**: executa em sequência via `os/exec`:
```
cd <ScriptsDir> && SPOKE=<spoke-id> BESU_RPC_URL=<url> go test ./... -run TestDeployEVMRegistry -v -count=1 -timeout 5m
cd <ScriptsDir> && SPOKE=<spoke-id> BESU_RPC_URL=<url> go test ./... -run TestDeployZetoFactory -v -count=1 -timeout 5m
cd <ScriptsDir> && SPOKE=<spoke-id> BESU_RPC_URL=<url> go test ./... -run TestDeployPenteFactory -v -count=1 -timeout 5m
```

Os scripts escrevem as keys em `<ScriptsDir>/../<spoke-id>/.deployed-addrs.env`. O engine espera que o arquivo seja criado lá.

**Atenção**: o caminho de escrita do `.deployed-addrs.env` pelos scripts Go test é relativo ao `ScriptsDir` (`../<spoke-id>/.deployed-addrs.env`). Para novos spokes, o `ScriptsDir` referencia `deploy/local/paladin/scripts/` e o arquivo é escrito em `deploy/local/paladin/<spoke-id>/.deployed-addrs.env`. Após escrita, o engine copia/symlink para `<SPOKE_DATA_DIR>/.deployed-addrs.env` para centralizar em `SPOKE_DATA_DIR`.

---

#### 1.2 — Passo 2: `gen-tls`

**`check()`**: verifica se `<SPOKE_DATA_DIR>/tls/central-bank.crt` existe.

**`run()`**: chama `deps.CertSource.IssueLeafCert(ctx, csrPEM, spokeID)`. Para `mode: found`, o CB é o fundador — ele gera um self-signed cert para si mesmo usando a CA do spoke (TK-3 `LocalCertSource` inicializa a CA na primeira chamada). O CSR é gerado no passo usando a chave pública do CB (`deps.KeyProvider.GetPublicKey`). O cert emitido é escrito em `<SPOKE_DATA_DIR>/tls/central-bank.crt`.

**Nota sobre OU**: Para o banco central no `mode: found`, o OU não é `ROLE_COMMERCIAL_BANK`. O TK-3 restringe `IssueLeafCert` a `ROLE_COMMERCIAL_BANK`. Para o CB, o engine gera diretamente um self-signed cert usando go stdlib (`x509.CreateCertificate`) com OU=`ROLE_CENTRAL_BANK`, **sem** chamar `IssueLeafCert`. `IssueLeafCert` é reservado para o `mode: join` (TK-9).

---

#### 1.3 — Passo 3: `render-configs`

**`check()`**: verifica se `<SPOKE_DATA_DIR>/paladin/central-bank/config.yaml` existe.

**`run()`**: lê `DeployedAddrs` de `.deployed-addrs.env`, renderiza templates de `PaladinConfigTemplateDir` via `text/template`, escreve os `config.yaml` em `<SPOKE_DATA_DIR>/paladin/<node>/config.yaml`.

Nodes renderizados para `mode: found` (central bank): `central-bank`, `bank-a`, `bank-c` (default). Lista de nodes configurável via manifesto.

---

#### 1.4 — Passo 4: `register-nodes`

**`check()`**: consulta `.provisioning-state.yaml`; retorna `true` somente se `status == "done"` para este passo.

**`run()`**: executa via `os/exec`:
```
cd <ScriptsDir> && SPOKE=<spoke-id> BESU_RPC_URL=<url> go test ./... -run TestRegisterPaladinNodes -v -count=1 -timeout 5m
```

Trata erro "already registered" como idempotente (não falha).

---

#### 1.5 — Passo 5: `start-paladin`

**`check()`**: verifica dois critérios:
1. Container `paladin-<spoke-id>-cb` em estado `running` via `docker compose ps --format json`
2. Health check: `POST http://localhost:<PALADIN_CB_RPC_PORT>` com `ptx_getTransaction` retorna `PD020704` no body

**`run()`** (apenas na primeira execução — `pending`):
```bash
# Stop Paladin se estiver rodando
docker compose -f <paladin-compose.yaml> down
# Limpar volumes de dados do Paladin (bloco-indexer deve re-sincronizar)
docker volume rm -f <spoke-id>_paladin_cb_data <spoke-id>_paladin_bank_a_data ...
# Iniciar com configs renderizadas
SPOKE_ID=<id> PALADIN_CB_RPC_PORT=<port> ... docker compose -f <paladin-compose.yaml> up -d
# Polling health check com timeout e intervalo configuráveis
```

---

#### 1.6 — Passos 6, 7, 8: `create-zeto-token`, `create-pente-context`, `deploy-fxa-pente`

Padrão idêntico: `check()` lê chave específica de `.deployed-addrs.env`, `run()` invoca `go test` via `os/exec` com variáveis relevantes. Ver tabela da spec (Sequência de passos) para variáveis exatas.

---

#### 1.7 — Passo 9: `onboard-registry`

**`check()`**: chama `IdentityRegistry.isParticipant(evmAddress)` on-chain via go-ethereum `ethclient`:
1. Deriva `evmAddress` via `keyprovider.EVMAddress(deps.KeyProvider.GetPublicKey(ctx, spokeID+"/cb"))`
2. Chama o contrato com ABI encoding de `isParticipant(address)` — retorna `bool`

**`run()`**: fluxo de onboarding real:
1. `pubkey, _ := deps.KeyProvider.GetPublicKey(ctx, spokeID+"/cb")` (gera se não existe)
2. `evmAddr, _ := keyprovider.EVMAddress(pubkey)`
3. Montar nonce de prova de posse: `nonce = sha256(evmAddr + spokeID + timestamp)`
4. `sig, _ := deps.KeyProvider.Sign(ctx, spokeID+"/cb", nonce)`
5. Montar tx para `registerParticipant(evmAddr, "Central Bank "+spokeID, RoleCentralBank, [32]byte{})` assinada com `sig`
6. Enviar tx via `ethclient` e aguardar receipt
7. Chamar `SetCertFingerprint(evmAddr, sha256(certPEM))` e aguardar receipt

**Deps adicionais**: `ethclient.Client` instanciado internamente a partir de `deps.BesuRPCURL`. O ABI do IdentityRegistry é embarcado como constante no passo (JSON string do ABI Solidity compilado).

---

#### 1.8 — Passo 10: `register-relay`

**`check()`**: `deps.RelayRegistrar.IsRegistered(ctx, spokeID)` — retorna `false` graciosamente se relay indisponível.

**`run()`**: `deps.RelayRegistrar.Register(ctx, SpokeInfo{...})`. Se retornar `ErrRelayUnavailable` ou `ErrNotImplemented`, o engine loga `step_failed` mas **não propaga o erro** (o spoke está funcional sem o relay estar disponível no momento do provisionamento). O passo fica como `failed` no arquivo de estado — será re-tentado na próxima execução.

---

#### 1.9 — `orchestrator.go` — `RunFound`

```
func RunFound(ctx, manifest, deps):
  1. Verificar genesis.json existe
  2. Obter file lock (ErrProvisioningLocked se ocupado)
  3. defer unlock
  4. Carregar estado de .provisioning-state.yaml
  5. Construir lista de 10 steps com deps/manifest resolvidos
  6. Para cada step:
     a. log step_started (se check() retornar false)
     b. check() → true: log step_skipped + continue
     c. check() → false: run()
        - ok: marcar done + salvar estado + log step_completed
        - err: marcar failed + salvar estado + log step_failed + return err (exceto passo 10)
  7. Return nil
```

---

#### 1.10 — Templates Paladin

Criar `paladin-compose.yaml` com variáveis:
- `${SPOKE_ID}` — prefixo de nomes de containers e volumes
- `${PALADIN_CB_RPC_PORT}` — porta HTTP do nó CB
- `${PALADIN_IMAGE}` — imagem Paladin
- `${SPOKE_DATA_DIR}` — diretório base de configurações e dados
- `${SPOKE_NETWORK_NAME}` — rede Docker a ser usada (a mesma do Besu)

Criar `paladin-config/central-bank/config.yaml.tmpl` e `paladin-config/bank/config.yaml.tmpl` baseados nos templates existentes em `deploy/local/paladin/spoke-a/config/`.

---

### Fase 2 — Testes de integração E2E

Após a implementação dos 10 passos, executar o fluxo completo em ambiente local:

1. Subir Besu com TK-4
2. Invocar `RunFound` programaticamente ou via TK-7
3. Verificar SC-001 a SC-009 (todos os success criteria da spec)
4. Executar `RunFound` segunda vez — verificar SC-003 (< 2 s, todos skipped)

**Não escopo desta fase**: conexão com um segundo spoke, relay RL-1, `mode: join` (TK-8/TK-9).

---

## Dependências e pré-requisitos

| Dependência | Status | Pré-requisito para |
|-------------|--------|--------------------|
| TK-2 `KeyProvider` interface + `LocalKeyProvider` | ✅ Implementado | Passo 2, 9 |
| TK-3 `CertSource` interface + `LocalCertSource` | ✅ Implementado | Passo 2 |
| TK-4 template Besu central-bank | ✅ Implementado | Passo 5 (Besu precisa estar rodando) |
| TK-1 manifest schema + `manifest.ParseFile` | Verificar status | `RunFound` entry |
| IdentityRegistry ABI JSON (Foundry output) | `contracts/out/IdentityRegistry.sol/` | Passo 9 |
| Paladin compose template (novo, R-01) | A criar neste PR | Passo 5 |
| Paladin config templates (novo, R-06) | A criar neste PR | Passo 3 |
