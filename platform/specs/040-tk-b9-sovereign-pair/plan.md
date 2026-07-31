# Implementation Plan: Toolkit do Cenário B — par soberano + liquidez cooperativa + seed-oracle (TK-B9)

**Branch**: `040-tk-b9-sovereign-pair` | **Date**: 2026-07-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/040-tk-b9-sovereign-pair/spec.md`

## Summary

Adicionar a **cauda soberana** ao `found-spoke` (TK-B7): quando o manifesto carrega `spec.pair`, três
steps **soft** rodam após `add-noc-agent` e antes de `emit-spoke-bundle` — `open-sovereign-pair`,
`commit-liquidity`, `seed-oracle`. Reusa o motor (Step soft, Check, estado, report) do TK-B6/B7, o
executor injetável e o `RelayRegistrar` (TK-B5). Idempotência **on-chain** (`getPair(pairId).status`).
**Soberania estrita** (clarificação Q2): cada `apply` executa **só** o ato do CB do manifesto atual —
CB-A faz scaffolding (ato de admin do hub) + `proposePair`; um `found-spoke` **separado** de CB-B faz
`confirmPair`; cada CB faz `commit-liquidity` só do seu lado; `seed-oracle` é local-only. Nenhum run
detém a chave da contraparte.

**Decisão-chave (deviation).** `SeedNewSovereignPair.s.sol` exige **ambas** as chaves
(`CB_A_HUB_PRIVATE_KEY` + `CB_B_HUB_PRIVATE_KEY`) e faz propose+confirm num único script — o modelo
"um run detém tudo", **rejeitado** pela Q2. Portanto o toolkit **não** reusa o script monolítico;
orquestra os atos discretos via o executor: scaffolding (deploy/dedup dos W-tokens, AMM, LCR,
`setCentralBankOf`, grants) com a **chave de admin do hub**; `proposePair`/`confirmPair`/`registerCommit`
/`setRate` via `cast send` com a chave do **CB corrente**; leituras de idempotência via `cast call`
(`getPair`). Sem novo `.s.sol` (não alterar contratos).

## Technical Context

**Language/Version**: Go 1.26 (módulo `scenario-b/toolkit`).
**Primary Dependencies**: **nenhuma nova** — `os/exec` (executor: `cast send`/`cast call`/`forge
create`), `gopkg.in/yaml.v3` (estado), `net/http` (opcional, leituras RPC) — já presentes. Ferramentas
externas (runtime/E2E): Foundry (`cast`/`forge`), Docker, `hyperledger/besu:25.8.0`, relay Cacti.
**Storage**: reusa o estado por step (`<dataDir>/.provisioning-state.yaml`) + `flock`; nenhum artefato
novo. Endereços de W-token/AMM/LCR são descobertos on-chain / do broadcast e escritos no `.env` do
spoke via `addrs.AppendAddr`.
**Testing**: `go test` com `FakeRunner` (open-sovereign-pair: scaffolding+propose por CB-A, confirm por
CB-B, idempotência por `getPair` status, dedup de W-token por moeda; commit-liquidity: registerCommit
do lado do CB; seed-oracle: setRate local-only + skip fora de local; soft-fail não bloqueia) + suíte
**E2E** (build tag `e2e`, skip-com-aviso): dois spokes fundados (CB-A propõe → CB-B confirma → ACTIVE),
commit de ambos os lados casado pelo relay, oráculo semeado.
**Target Platform**: binário `cbweb3b` (cauda do `found-spoke`) + biblioteca do toolkit.
**Project Type**: CLI + biblioteca Go.
**Performance Goals**: N/A.
**Constraints**: steps **soft** (falha não bloqueia found-spoke nem o bundle); idempotência on-chain;
soberania estrita (nenhum run detém a chave da contraparte); `seed-oracle` local-only; dry-run sem
efeitos; **não** alterar contratos/scripts/Makefiles/`deploy/local`; **não** importar `scenario-a/`;
breaker opção A (sem mudança de lógica); W-token dedup por moeda.
**Scale/Scope**: 3 steps novos (soft) na cauda do `found-spoke`; 0 modo novo; 0 contrato novo; ~1
helper de leitura on-chain (`getPair` status via `cast call`).

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B9) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Só toolkit + contratos existentes do B; **não** importa `scenario-a/`; não edita contratos/scripts/Makefiles/`deploy/local`. |
| **II. Privacy by Design** | ✅ **Reforça**: commit-reveal cooperativo — cada CB contribui/custodia **só a própria moeda**; nenhum run detém a chave/valor da contraparte. |
| **III. Atomic Settlement** | ✅ A AMM do par nasce **com** o seu circuit breaker (pause 1-de-N, resume quorum 2); o toolkit não altera a lógica (opção A) e o relay valida `isPaused` antes de swaps (coberto no found-hub/relay). |
| **IV. Compliance Gate** | ✅ Atos soberanos gated: cada CB só assina o seu; fora de `local`, atos sem aprovação ficam `pending`. Não contorna o gate. |
| **V. Test-First** | ✅ Steps com `go test` (FakeRunner) antes da implementação; E2E é a aceitação final (skip-com-aviso). |
| **VI. Observability** | ✅ Report tipado (done/skipped/soft-failed/planned); atos `pending` visíveis; sem swallow. |

**Novas dependências**: nenhuma. **Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — reforça II e IV.
A única deviation (não reusar o script monolítico `SeedNewSovereignPair`) é **exigida** pela soberania
estrita (Q2) e está registrada em Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/040-tk-b9-sovereign-pair/
├── plan.md, spec.md
├── research.md          # Fase 0 — split do SeedNewSovereignPair, idempotência on-chain, dedup W-token, commit-reveal, seed-oracle
├── data-model.md        # Fase 1 — PairConfig, step set soft, getPair status, atos por ator
├── quickstart.md        # Fase 1 — found-spoke com spec.pair (CB-A e CB-B), dry-run e real
├── contracts/           # Fase 1 — cauda soberana do found-spoke + atos on-chain
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── engine/orchestrator/
│   ├── step_found_spoke.go      # EDIT — SpokeConfig +Pair; FoundSpokeSteps injeta a cauda soberana (soft) quando Pair != nil
│   ├── step_sovereign_pair.go   # NEW — sovereignPairSteps(cfg): open-sovereign-pair, commit-liquidity, seed-oracle
│   ├── pairstate.go             # NEW — getPair status via `cast call` (idempotência), pairId determinístico
│   └── *_test.go                # step_sovereign_pair_test.go, pairstate_test.go
├── engine/apply/apply.go        # EDIT — applyFoundSpoke popula SpokeConfig.Pair + chaves/oracle a partir do manifesto/flags
├── cmd/cbweb3b/main.go          # EDIT (se necessário) — flags p/ chaves de CB e taxa do oráculo (local)
└── tests/e2e/sovereign_pair_e2e_test.go  # NEW — E2E (build tag e2e), skip-com-aviso
```

**Structure Decision**: TK-B9 estende o `found-spoke` (TK-B7) com uma **cauda soberana soft**
condicionada a `spec.pair`. Novos: `step_sovereign_pair.go` (os 3 steps), `pairstate.go` (leitura de
`getPair` status para idempotência e o `pairId` determinístico) e a suíte E2E. `FoundSpokeSteps`
passa a anexar os 3 steps (Deps em `add-noc-agent`; `emit-spoke-bundle` passa a depender deles, mas
como são **soft**, uma falha não impede o bundle). Os atos on-chain são discretos (via `cast`/`forge
create`), respeitando a soberania estrita e a proibição de tocar contratos/scripts.

## Complexity Tracking

| Violation / Deviation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| Não reusar `SeedNewSovereignPair.s.sol` (script monolítico); orquestrar atos discretos via `cast`/`forge create` | O script exige **ambas** as chaves de CB e faz propose+confirm juntos — o modelo "um run detém tudo", **rejeitado pela Q2** (soberania estrita). Cada `apply` só pode assinar o ato do seu CB. | Reusar wholesale violaria a soberania estrita (CB-A teria de deter a chave de CB-B). Adicionar um `.s.sol` dividido violaria "não alterar contratos". Restam os atos discretos via executor. |
| `emit-spoke-bundle` depende dos 3 steps soberanos | Manter a ordem canônica (roadmap §407: cauda → bundle). | Como os steps são **soft**, a dependência não bloqueia: uma falha soberana não impede a emissão do bundle. |
