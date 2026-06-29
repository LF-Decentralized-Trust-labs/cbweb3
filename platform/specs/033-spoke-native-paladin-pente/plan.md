# Implementation Plan: Topologia nativa do toolkit — found CB-only e join dinâmico de Paladin/Pente

**Branch**: `033-spoke-native-paladin-pente` | **Date**: 2026-06-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/033-spoke-native-paladin-pente/spec.md`

## Summary

Remover a topologia fixa `spoke-a`/`spoke-b`/`bank-a`/`bank-c` do motor de provisionamento e substituí-la por lógica **nativa do toolkit**, parametrizada por `spec.spoke.id`/`spec.bankId`. O `found` passa a provisionar **apenas** o banco central (CB-only); a criação do contexto **Pente** e do **FXAgreement-in-Pente** migra para o `mode: join`, que ganha passos para subir o nó **Paladin do banco dinamicamente** (cert + config + container + registro on-chain), espelhando os scripts validados em `spk-02`. A rede de referência (`deploy/local` + scripts go-test) permanece intocada; a lógica nativa vive no toolkit Go.

## Technical Context

**Language/Version**: Go 1.26+ (toolkit); YAML (templates Compose); TypeScript (relay — já tratado em 031/RL-1)
**Primary Dependencies**: `github.com/ethereum/go-ethereum v1.17.1` (JSON-RPC QBFT/IdentityRegistry, ABI), `gopkg.in/yaml.v3`, stdlib `crypto/x509`/`crypto/ecdsa` (cert), `net/http` (Paladin RPC + cbEndpoint), `os/exec` (docker compose) — todos já em `toolkit/go.mod`. Paladin RPC (`ptx_*`, `pgroup_*`/Pente) via JSON-RPC.
**Storage**: Filesystem — `SPOKE_DATA_DIR` (genesis, tls, paladin configs, `.provisioning-state.yaml`, `.deployed-addrs.env`); IdentityRegistry on-chain.
**Testing**: `go test ./...` com stubs/mocks de BesuRPC, Paladin RPC e IdentityRegistry (padrão TK-5/TK-9); sem containers/redes externas. Bash de template para os composes.
**Target Platform**: Linux + Docker Compose v2; `hyperledger/besu:25.8.0`, `docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1`.
**Project Type**: CLI + engine (toolkit standalone do Scenario A).
**Performance Goals**: found CB-only ponta a ponta < 5 min local; join < 5 min com spoke sincronizado; overhead de re-`apply` idempotente < 10s.
**Constraints**: Não modificar `deploy/local`/`make/*.mk`; sem chaves privadas em arquivo/env/log; genesis nunca regenerado; idempotência por passo.
**Scale/Scope**: N spokes, N bancos por spoke; demonstração com 2 spokes (`spoke-brl`, `spoke-cop`) e 2 bancos cada.

## Constitution Check

*GATE: deve passar antes da Fase 0 e ser reavaliado após a Fase 1.*

- **Isolamento de cenário (Scenario A only)**: ✅ Toda a mudança vive em `scenario-a/toolkit` e `scenario-a/provisioning`. Não toca `scenario-b/`. Sem código compartilhado entre cenários.
- **Privacidade (Zeto/Noto, sem PII/valores em claro)**: ✅ Não muda o modelo de privacidade. O Pente passa a ser criado pairwise no join; nenhum valor/PII em claro on-chain. Certs contêm apenas material público.
- **Atomicidade / caminhos de timeout**: ✅ Cada passo é idempotente e persistido; falhas abortam sem estado parcial inconsistente; voto QBFT respeita ADR-002 (sem restart). Não há liquidação nesta feature (fora de escopo).
- **Gate de compliance no API gateway**: ✅ Inalterado. O fluxo CSR→CB→cert do TK-9 (que passa pelo banco central) permanece; o registro de identidade no IdentityRegistry é mantido.
- **Test-first, toda camada**: ✅ Cada passo novo (registro nativo de nó Paladin, criação de Pente, start-paladin-join) tem teste falhando antes da implementação, com mocks de RPC. Foundry/`go test` para contratos; `go test` para o motor.
- **Observabilidade**: ✅ Logs JSON estruturados por passo (request id, service, severity, ISO-8601) já no padrão do motor; novos passos seguem o mesmo logger. Sem swallow silencioso de erro.
- **PR enfraquecendo compliance/segurança**: N/A — não enfraquece checagens; pelo contrário, corrige um registro on-chain silenciosamente incorreto (spoke-a por engano).

**Resultado do gate**: PASS. Há **uma** decisão de complexidade a justificar (ver Complexity Tracking): substituir o reuso dos scripts go-test de referência por lógica nativa nos passos de Paladin/Pente.

## Project Structure

### Documentation (this feature)

```text
specs/033-spoke-native-paladin-pente/
├── spec.md              # done
├── plan.md              # this file
├── research.md          # Fase 0 — confirmar APIs Paladin (pgroup/Pente, ptx) e ABI registerIdentity
├── data-model.md        # Fase 1 — PaladinNodeIdentity, PenteContext, ProvisioningState revisado, JoinBundle revisado
├── contracts/           # Fase 1 — assinaturas das funções nativas (RegisterPaladinNode, CreatePenteContext, DeployFXAInPente)
└── tasks.md             # Fase 2 — /speckit.tasks
```

### Source Code (repository root)

```text
scenario-a/toolkit/engine/orchestrator/
├── step.go                       # ALTERAR: CanonicalStepOrder (found perde create-pente/deploy-fxa);
│                                  #          CanonicalJoinStepOrder ganha gen-tls-join, render-config-join,
│                                  #          start-paladin-join, register-paladin-node, create-pente-context, deploy-fxa-pente
├── paladin_registry.go (NOVO)    # lógica nativa: RegisterPaladinNode (ABI registerIdentity, parametrizado por spoke/bankId)
├── pente.go            (NOVO)    # lógica nativa: CreatePenteContext (CB↔banco), DeployFXAInPente (via Paladin RPC)
├── step_register_nodes.go        # ALTERAR (found): registrar SÓ o nó CB via paladin_registry nativo (sem go-test script)
├── step_render_configs.go        # ALTERAR (found): renderizar SÓ o CB
├── step_start_paladin.go         # ALTERAR (found): compose CB-only (já parametrizado)
├── step_*_join*.go     (NOVOS)   # passos do join: gen-tls-join, render-config-join, start-paladin-join,
│                                  #                 register-paladin-node, create-pente-context, deploy-fxa-pente
└── *_test.go                     # testes test-first (mocks RPC)

scenario-a/provisioning/templates/
├── central-bank/paladin-compose.yaml     # ALTERAR: remover serviços bank-a/bank-c (CB-only)
└── commercial-bank/paladin-compose.yaml  # NOVO: serviço Paladin do banco (join), parametrizado por BANK_ID

scenario-a/toolkit/engine/bundle/
├── bundle.go                     # ALTERAR: readContracts deixa de exigir PENTE_CONTEXT_*/FX_AGREEMENT_*
└── *_test.go                     # ALTERAR: testes de contratos obrigatórios

# NÃO TOCAR (rede de referência):
# deploy/local/paladin/scripts/{register_nodes,create_pente_context,deploy_fxagreement_pente}_test.go
```

**Structure Decision**: Toolkit CLI + engine (Go), com novos arquivos `paladin_registry.go`/`pente.go` para a lógica nativa, e um novo `commercial-bank/paladin-compose.yaml`. Os passos de contrato de nível-spoke (deploy-contracts via go-test scripts) permanecem reusados — só os passos com topologia hardcoded migram para nativo.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Lógica nativa Go para registro de nó Paladin e criação de Pente, em vez de reusar os scripts go-test de referência (estratégia "building blocks" do concat.md §5.3) | Os scripts `register_nodes_test.go`/`create_pente_context_test.go`/`deploy_fxagreement_pente_test.go` hardcodam a topologia `spoke-a`/`spoke-b` (`switch spokeName()` com `default: // spoke-a`), registrando nós errados para spokes arbitrários. São rede de referência e não podem ser modificados (constituição: preservar `deploy/local`). | (a) Modificar os scripts → proibido (preservar rede de referência verde). (b) Parametrizar via env nos scripts → exigiria reescrevê-los e ainda assim acoplaria o toolkit a artefatos de referência. (c) Manter reuso → quebra N-spokes (registro silenciosamente incorreto). A lógica nativa é a única que satisfaz N-spokes + isolamento da rede de referência. |
| `found` deixa de criar Pente/FXA (bilateral) e move para o join | Pente é inerentemente pairwise (≥2 nós Paladin). No modelo N-spokes, o relacionamento CB↔banco só existe quando um banco entra. | Manter no found exigiria um nó "banco" fixo (a topologia antiga de 2 bancos), exatamente o que a arquitetura elimina. |
