# Implementation Plan: Toolkit do Cenário B — found-spoke + spoke bundle (TK-B7)

**Branch**: `038-tk-b7-found` | **Date**: 2026-07-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/038-tk-b7-found/spec.md`

## Summary

Adicionar o segundo modo executável — **`found-spoke`** — ao toolkit, reusando o motor do TK-B6
(Step/Check-Run, estado, lock, dry-run, report), o executor injetável, `engine/addrs`, `gen-genesis`
e as interfaces `KeyProvider`/`CertSource`/`RelayRegistrar`. O modo: **consome o hub bundle** →
**register-cb** (auto-registra o CB no `IdentityRegistry` do hub via `RegisterParticipants.s.sol`) →
**gen-genesis-spoke** → **start-besu-spoke** (CB validador) → **deploy-spoke-contracts** (um
`forge script CBWeb3Spoke.s.sol`: IdentityRegistry → tCeBM doméstico → SpokeBridge → fCeBM) →
**wire-hub-addresses** → **provision-keycloak-spoke** (+ write-back) → **render/infra/backend/frontend**
→ **register-relay-spoke** (via `RelayRegistrar`, runtime) → **add-noc-agent** (soft) →
**emit-spoke-bundle** (versionado, com **genesis + enode** + chainId + endereços — para o `join`).
Duas extensões pequenas do motor: **steps soft** (não-fatais) e **captura de enode** (admin_nodeInfo).
Par soberano / liquidez / oracle são TK-B9 (fora de escopo).

## Technical Context

**Language/Version**: Go 1.26 (módulo `scenario-b/toolkit`).
**Primary Dependencies**: **nenhuma nova** — `github.com/ethereum/go-ethereum` (RPC/enode via
admin_nodeInfo), `gopkg.in/yaml.v3` (estado/bundle), `net/http` (RelayRegistrar/enode), `os/exec`
(executor) já presentes. Ferramentas externas (runtime/E2E): Docker Compose v2, Foundry `forge`,
`hyperledger/besu:25.8.0`, Keycloak.
**Storage**: reusa estado por step (`<dataDir>/.provisioning-state.yaml`) + `flock`; genesis do spoke
em `<GenesisDir>/genesis.json`; **spoke bundle** em `<outDir>/bundles/spoke-<id>.bundle.yaml` (inclui
genesis + enode, ao contrário do hub bundle RPC-only).
**Testing**: `go test` com `FakeRunner` (steps found-spoke: ordem, register-cb vs hub, deploy via
`CBWeb3Spoke.s.sol`, wire/keycloak idempotentes, register-relay via `RelayRegistrar` fake,
`add-noc-agent` soft, bundle round-trip) + suíte **E2E** (build tag `e2e`, skip-com-aviso) que funda
um spoke contra um hub fundado.
**Target Platform**: binário `cbweb3b` (modo `found-spoke`) + biblioteca do toolkit.
**Project Type**: CLI + biblioteca Go.
**Performance Goals**: N/A.
**Constraints**: idempotência (re-apply converge); dry-run sem efeitos; spoke bundle sem segredos;
`add-noc-agent` não-fatal; não alterar Makefiles/`deploy/local`; não importar `scenario-a/`; par
soberano/liquidez/oracle fora de escopo; auth-por-CB no relay fora de escopo (§14.D); E2E
skip-com-aviso.
**Scale/Scope**: 1 modo novo (`found-spoke`), ~12 steps, 1 bundle novo (spoke), 2 extensões do motor
(soft steps + enode).

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B7) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Reusa o motor/interfaces do próprio toolkit; **não** importa `scenario-a/`; não edita Makefiles/`deploy/local`. |
| **II. Privacy by Design** | ✅ Spoke bundle sem segredos (genesis/enode/endereços são públicos/rede); `tCeBM` doméstico é camada de reserva; nenhuma chave privada no bundle (FR-011). |
| **III. Atomic Settlement** | ✅ N/A — `found-spoke` não é caminho de settlement; par soberano (com AMM/breaker) é TK-B9. |
| **IV. Compliance Gate** | ✅ **Reforça**: `register-cb` (IdentityRegistry do hub) + `provision-keycloak-spoke` estabelecem identidade + OIDC do spoke — a base do gate. Não contorna. |
| **V. Test-First** | ✅ Steps/bundle com `go test` (FakeRunner) antes da implementação; E2E é a aceitação final (skip-com-aviso). |
| **VI. Observability** | ✅ Report tipado (done/skipped/failed/planned + **soft-failed**); `add-noc-agent` é observabilidade; sem swallow. |

**Novas dependências**: nenhuma. Docker/Foundry/Besu/Keycloak já são o stack. Nada a justificar.

**Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — sem violações (reforça IV).

## Project Structure

### Documentation (this feature)

```text
specs/038-tk-b7-found/
├── plan.md, spec.md
├── research.md          # Fase 0 — reuso do TK-B6, deploy spoke, register-cb no hub, enode, soft steps, spoke bundle
├── data-model.md        # Fase 1 — SpokeConfig, step set, SpokeBundle, soft step, enode
├── quickstart.md        # Fase 1 — apply found-spoke (dry-run e real)
├── contracts/           # Fase 1 — CLI found-spoke + SpokeBundle + extensões do motor
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── engine/orchestrator/
│   ├── step.go                  # + campo Soft (step não-fatal)
│   ├── orchestrator.go          # soft-fail: step Soft que falha → soft-failed, não interrompe
│   ├── step_genesis.go          # generalizar genGenesis(name, chainID, dir) p/ hub e spoke
│   ├── enode.go                 # NEW — captura enode via admin_nodeInfo (injetável)
│   ├── step_found_spoke.go      # NEW — SpokeConfig + FoundSpokeSteps (os ~12 steps)
│   └── *_test.go
├── engine/bundle/
│   ├── types.go                 # + SpokeBundle{Version,SpokeID,ChainID,Enode,SpokeRPC,Genesis,Contracts}
│   ├── spoke.go                 # NEW — EmitSpoke/LoadSpoke/ValidateSpoke (sem segredos)
│   └── spoke_test.go
├── engine/apply/
│   ├── apply.go                 # + dispatch "found-spoke" (applyFoundSpoke)
│   └── apply_test.go
├── engine/relayregistrar/       # (TK-B5) consumido por register-relay-spoke
└── tests/e2e/found_spoke_e2e_test.go   # NEW — E2E (build tag e2e), skip-com-aviso
```

**Structure Decision**: TK-B7 empilha sobre o TK-B6 — reusa `orchestrator`, `exec`, `addrs`,
`bundle` e as interfaces plugáveis. Novos: `step_found_spoke.go`, `enode.go`, `bundle/spoke.go`, o
dispatch `found-spoke` no `apply` e a suíte E2E. Duas extensões mínimas do motor: `Step.Soft` e o
tratamento soft-fail no `Run`. O `gen-genesis` é generalizado (hub e spoke). O `register-cb` e o
`deploy-spoke-contracts` são `forge script` (como no TK-B6), via `CommandRunner`.

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. As duas extensões do motor (soft steps + enode) são incrementos pequenos e coesos, não
desvios de padrão; nenhuma dependência nova. O spoke bundle reusa o padrão do hub bundle, acrescendo
genesis+enode conforme o roadmap §11.
