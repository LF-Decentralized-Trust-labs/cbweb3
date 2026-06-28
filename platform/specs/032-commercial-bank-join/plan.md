# Implementation Plan: TK-8 e TK-9 — Commercial Bank Join (`mode: join`)

**Branch**: `032-commercial-bank-join` | **Date**: 2026-06-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/032-commercial-bank-join/spec.md`

## Summary

Implementar o template Compose para banco comercial (TK-8) e o motor de orquestração `mode: join` (TK-9), permitindo que um banco comercial ingresse em um spoke existente a partir de um join bundle produzido pelo TK-6. O motor executa 9 passos idempotentes: escrita do genesis, início do nó Besu como não-validador, aguardo de sincronização, votação QBFT, geração de CSR, solicitação e recebimento do certificado assinado pelo banco central, prova de posse com registro no IdentityRegistry e início do backend do banco. O join bundle é estendido com campos `validators` e `cbEndpoint` para suportar os novos passos.

## Technical Context

**Language/Version**: Go 1.26+ (orchestrator, bundle, manifest, pki packages); Docker Compose v2 + YAML 3.8+ (TK-8 template)
**Primary Dependencies**: `gopkg.in/yaml.v3` (bundle/manifest parsing), `net/http` (CSR HTTP POST + polling), `github.com/ethereum/go-ethereum v1.17.1` (QBFT JSON-RPC, IdentityRegistry), `os/signal`+`syscall` (file lock), `flag` (CLI) — todos presentes em `toolkit/go.mod`; Docker Compose v2 plugin; `hyperledger/besu:25.8.0` (pinned)
**Storage**: Filesystem — `SPOKE_DATA_DIR/genesis/genesis.json` (leitura do bundle; nunca regenerado), `SPOKE_DATA_DIR/tls/commercial-bank.{crt,key}`, `SPOKE_DATA_DIR/.provisioning-state.yaml`, `SPOKE_DATA_DIR/.provisioning.lock`
**Testing**: `go test` (unit tests com stubs para BesuRPC, CB endpoint, IdentityRegistry — mesmo padrão de `orchestrator_test.go`); test script Bash para o template TK-8 (análogo a `tests/test-central-bank-template.sh`)
**Target Platform**: Linux server (Docker Compose local); environment-agnostic via manifest parameters (DOCKER/NONE NAT profile via ADR-001)
**Project Type**: CLI + biblioteca de orquestração (extensão de `scenario-a/toolkit/`)
**Performance Goals**: Join flow completa em < 5 min em hardware local (SC-002); votação QBFT não interrompe produção de blocos por > 6s (SC-006, limiar ADR-002 T2)
**Constraints**: Nenhuma chave privada em arquivos, env, logs ou manifesto (SC-007); genesis nunca regenerado se já existe (FR-003, FR-012); todos os passos idempotentes (FR-014); zero dependências externas novas além das já em `toolkit/go.mod`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Veredicto | Evidência |
|-----------|-----------|-----------|
| **I. Scenario-Scoped Independence** | ✅ PASS | Todos os novos artefatos ficam em `scenario-a/toolkit/` e `scenario-a/provisioning/templates/commercial-bank/`. Zero toque em `scenario-b/`. O toolkit é explicitamente scoped a Scenario A. |
| **II. Privacy by Design** | ✅ PASS (N/A) | Infraestrutura de provisionamento, sem transferências de valor on-chain. Chaves privadas gerenciadas via `keyProvider` interface — nunca materializadas em arquivos. CSR e cert são material público. |
| **III. Atomic Settlement Guarantee** | ✅ PASS (N/A) | Não toca em HTLC, FXAgreement nem caminhos de liquidação. O provisionamento é pré-requisito; não interfere com a atomicidade do settlement. |
| **IV. Compliance Gate Before Participation** | ✅ PASS | O passo `request-cert` POST ao `cbEndpoint` (gateway do CB) passa pelo serviço de Compliance antes de o CB assinar o CSR — conforme `onboarding_proxy.go` (smart mode). O passo `proof-of-possession` registra no IdentityRegistry (gate on-chain). Ambas as gates são obrigatórias antes de o banco operar. Nenhum atalho de auto-registro (`auto-register helper`) é usado. |
| **V. Test-First at Every Layer** | ✅ PASS | Cada `step_*.go` novo tem teste unitário correspondente (`step_*_test.go`) escrito antes da implementação. `RunJoin` tem `run_join_internal_test.go` com steps injetados. Bash script de teste para o template TK-8. Red-Green-Refactor estrito. |
| **VI. Observability and Auditability** | ✅ PASS | Reutiliza `logSkipped`, `logStarted`, `logCompleted`, `logFailed` do `orchestrator.go`. O passo `vote-qbft` loga cada voto individualmente (spoke, validador, resultado). Nenhum swallow silencioso de erro. |

**Constitution Check post-design**: Re-avaliar após Phase 1 design (especialmente o contrato de request-cert e a extensão do bundle).

## Project Structure

### Documentation (this feature)

```text
specs/032-commercial-bank-join/
├── plan.md              # Este arquivo
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── bundle-extension.md
│   ├── credential-request.md
│   └── compose-template.md
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT criado por /speckit-plan)
```

### Source Code (repository root)

```text
scenario-a/provisioning/templates/commercial-bank/   # TK-8: novo template
├── docker-compose.yaml          # Besu joiner (sem genesis-init; BOOTNODE_ENODE obrigatório)
├── paladin-compose.yaml         # Paladin do banco comercial (padrão bank-x do SP-02)
├── scripts/
│   └── entry.sh                 # Reutilizar de central-bank/ (mesmo ADR-001 DOCKER/NONE)
└── paladin-config/
    └── commercial-bank/
        └── config.yaml.tmpl     # Template config Paladin para banco comercial

scenario-a/provisioning/tests/
└── test-commercial-bank-template.sh   # Smoke test do template TK-8

scenario-a/toolkit/engine/bundle/
└── types.go                     # ESTENDER: ValidatorSpec + cbEndpoint em BundleSpec

scenario-a/toolkit/engine/orchestrator/
├── orchestrator.go              # ESTENDER: adicionar RunJoin() + buildJoinSteps()
├── deps.go                      # ESTENDER: JoinDeps struct (BesuRPCURL, CBEndpoint, etc.)
├── step_write_genesis.go        # NOVO: escrever genesis do bundle, verificar hash
├── step_write_genesis_test.go
├── step_start_besu_join.go      # NOVO: docker compose up do TK-8
├── step_start_besu_join_test.go
├── step_wait_sync.go            # NOVO: poll eth_blockNumber até sync
├── step_wait_sync_test.go
├── step_vote_qbft.go            # NOVO: qbft_proposeValidatorVote + aguardar ativação
├── step_vote_qbft_test.go
├── step_gen_csr.go              # NOVO: keyProvider.GenerateKey + pki.GenerateCSR
├── step_gen_csr_test.go
├── step_request_cert.go         # NOVO: POST CSR + blockchain pubkey ao cbEndpoint
├── step_request_cert_test.go
├── step_receive_cert.go         # NOVO: poll/receber cert assinado do CB
├── step_receive_cert_test.go
├── step_proof_possession.go     # NOVO: sign nonce + registrar no IdentityRegistry
├── step_proof_possession_test.go
├── step_start_backend.go        # NOVO: docker compose up do backend do banco
├── step_start_backend_test.go
└── run_join_internal_test.go    # NOVO: testa RunJoin com steps injetados

scenario-a/toolkit/engine/apply/
└── apply.go                     # ESTENDER: routing mode:join → RunJoin()

scenario-a/toolkit/cmd/cbweb3/testdata/
└── commercial-bank-brl.yaml     # Manifesto de exemplo mode:join
```

**Structure Decision**: Extensão cirúrgica do pacote `orchestrator` existente. O padrão de `step_*.go` + `step_*_test.go` é preservado. `RunJoin()` vive no mesmo arquivo `orchestrator.go`, análogo a `RunFound()`. Nenhum novo módulo Go é criado.

## Complexity Tracking

> Sem violações à Constituição. Esta seção documenta uma decisão de design não-óbvia.

| Decisão | Justificativa | Alternativa Rejeitada |
|---------|---------------|-----------------------|
| `step_receive_cert.go` como passo separado de `step_request_cert.go` | O CB pode demorar a assinar (processo assíncrono); separar os passos permite que o estado `request-cert = done` seja persistido antes do polling, evitando reenvio do CSR em caso de restart. | Passo único `request-and-receive-cert` — rejeitado porque mistura envio (idempotente com cuidado) e recebimento (poll); dificulta depuração e re-run seguro. |
| Extensão de `BundleSpec` com `validators[]` e `cbEndpoint` em vez de campo no manifesto | O bundle é emitido pelo CB que conhece seus validadores; o banco comercial não deveria precisar configurar endpoints de RPC dos validadores manualmente. | Colocar `validators[]` no manifesto do banco comercial — rejeitado porque o operador do banco não conhece os RPC endpoints internos dos validadores do CB; viola "operador fornece apenas sua própria configuração". |
