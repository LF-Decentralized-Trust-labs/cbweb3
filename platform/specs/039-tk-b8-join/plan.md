# Implementation Plan: Toolkit do Cenário B — join (full node não-validador) (TK-B8)

**Branch**: `039-tk-b8-join` | **Date**: 2026-07-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/039-tk-b8-join/spec.md`

## Summary

Adicionar o terceiro modo executável — **`join`** — ao toolkit, reusando o motor do TK-B6/B7
(Step/Check-Run, estado, lock, dry-run, report), o executor injetável, `engine/addrs`, `engine/bundle`
(`LoadSpoke`), `engine/pki` (`GenerateBankCSR`) e os templates (TK-B4). Um banco comercial anexa ao
spoke do seu CB como **full node não-validador** (o CB é o validador único). Fluxo canônico (roadmap
§6): **consume-spoke-bundle** → **write-genesis** (do bundle, guard não-destrutivo + checagem de hash)
→ **start-besu-join** (nó não-validador, peer do enode do CB) → **wait-sync** (bloqueia até
`eth_syncing == false` e blockNumber avançando) → **wire-addresses** (endereços de spoke do bundle) →
**provision-keycloak-bank** (+ write-back) → **render-bank-env** → **infra/backend/frontend** →
**gen-csr** (cauda diferida de PKI: par de chaves + CSR local, `0600`, nunca transmitido). A assinatura
do CSR, a emissão gated por KYC e o registro on-chain são **runtime**, fora do toolkit. Uma extensão
nova de motor: o gate **`wait-sync`** (`eth_syncing` JSON-RPC). Sem step de relay nem de noc-agent
(decisão de clarificação: a cadeia já é observada desde o `found-spoke`).

## Technical Context

**Language/Version**: Go 1.26 (módulo `scenario-b/toolkit`).
**Primary Dependencies**: **nenhuma nova** — `net/http` (`eth_syncing`/`wait-sync`), `gopkg.in/yaml.v3`
(estado/bundle), `os/exec` (executor), `engine/pki` (`GenerateBankCSR`, já presente), `crypto/sha256`
(hash do genesis) — todos stdlib ou já em `toolkit/go.mod`. Ferramentas externas (runtime/E2E): Docker
Compose v2, `hyperledger/besu:25.8.0`, Keycloak.
**Storage**: reusa estado por step (`<dataDir>/.provisioning-state.yaml`) + `flock`; genesis do banco
escrito em `<GenesisDir>/genesis.json` (copiado do bundle, **nunca** regenerado); CSR/chave em
`<dataDir>/pki/{bank}.{key,csr}` (chave `0600`, nunca transmitida).
**Testing**: `go test` com `FakeRunner` (steps join: ordem, write-genesis guard + hash, wait-sync via
`EthSyncing` fake, wire idempotente, keycloak write-back, gen-csr idempotente + zero CA material) +
suíte **E2E** (build tag `e2e`, skip-com-aviso) que faz um banco `join` contra um spoke fundado.
**Target Platform**: binário `cbweb3b` (modo `join`) + biblioteca do toolkit.
**Project Type**: CLI + biblioteca Go.
**Performance Goals**: N/A.
**Constraints**: idempotência (re-apply converge); dry-run sem efeitos; **não-validador** (CB é
validador único; `vote-qbft` fora de escopo); write-genesis não-destrutivo; `gen-csr` nunca gera CA
nem transmite a chave; pré-criar `pki/` como usuário do host (armadilha do Cenário A); não alterar
Makefiles/`deploy/local`; não importar `scenario-a/`; sem relay/noc (fluxo canônico).
**Scale/Scope**: 1 modo novo (`join`), ~9 steps, 1 extensão de motor (`wait-sync`), 0 bundle novo
(consome o spoke bundle do TK-B7).

## Constitution Check

*GATE: deve passar antes da Fase 0; re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B8) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Reusa o motor/interfaces/pki do próprio toolkit; **não** importa `scenario-a/`; não edita Makefiles/`deploy/local`. |
| **II. Privacy by Design** | ✅ Sem segredos em manifesto/estado/bundle; a chave privada do banco fica `0600` em `<dataDir>/pki/`, **nunca** transmitida; **zero** `*-ca.key`/`*-ca.crt`; o spoke bundle consumido é público. |
| **III. Atomic Settlement** | ✅ N/A — `join` é onboarding de nó, não caminho de settlement. |
| **IV. Compliance Gate** | ✅ **Reforça**: `gen-csr` + `provision-keycloak-bank` preparam a identidade PKI + OIDC do banco; a **assinatura do CSR e o registro on-chain permanecem no runtime** (compliance do CB + governança) — o toolkit não contorna nem antecipa o gate. |
| **V. Test-First** | ✅ Steps com `go test` (FakeRunner) antes da implementação; E2E é a aceitação final (skip-com-aviso). |
| **VI. Observability** | ✅ Report tipado (done/skipped/failed/planned); `wait-sync` falha com erro claro (nunca verde falso); sem swallow. |

**Novas dependências**: nenhuma. **Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — sem violações
(reforça IV; PKI mínima como no Cenário A).

## Project Structure

### Documentation (this feature)

```text
specs/039-tk-b8-join/
├── plan.md, spec.md
├── research.md          # Fase 0 — reuso TK-B6/B7, write-genesis vs gen-genesis, wait-sync, gen-csr, sem relay/noc
├── data-model.md        # Fase 1 — JoinConfig, step set, wait-sync seam, gen-csr
├── quickstart.md        # Fase 1 — apply join (dry-run e real)
├── contracts/           # Fase 1 — CLI join + JoinSteps + extensão wait-sync
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── engine/orchestrator/
│   ├── sync.go                  # NEW — EthSyncing seam + admin/eth_syncing gate (wait-sync)
│   ├── step_join.go             # NEW — JoinConfig + JoinSteps (os ~9 steps do fluxo canônico)
│   └── *_test.go                # sync_test.go, step_join_test.go
├── engine/bundle/               # (TK-B7) LoadSpoke consumido por consume-spoke-bundle/write-genesis
├── engine/pki/csr.go            # (existente) GenerateBankCSR reusado por gen-csr
├── engine/apply/
│   ├── apply.go                 # + dispatch "join" (applyJoin); remove "not supported yet"
│   └── apply_test.go
├── cmd/cbweb3b/main.go          # doc/usage: join passa a ser suportado
└── tests/e2e/join_e2e_test.go   # NEW — E2E (build tag e2e), skip-com-aviso
```

**Structure Decision**: TK-B8 empilha sobre TK-B6/B7 — reusa `orchestrator`, `exec`, `addrs`,
`bundle` (`LoadSpoke`), `pki` (`GenerateBankCSR`) e os templates. Novos: `step_join.go`, `sync.go`
(gate `wait-sync`), o dispatch `join` no `apply` e a suíte E2E. `write-genesis` **difere** do
`gen-genesis` do TK-B6/B7 (que *gera* via `besu operator`): aqui apenas **escreve o genesis do
bundle** com guard não-destrutivo e checagem de hash (`sha256`), pois o banco deve usar exatamente o
genesis do spoke. `gen-csr` reusa `pki.GenerateBankCSR` acrescentando **idempotência** (Check: pula se
`{bank}.key`+`{bank}.csr` já existem) e **pré-criação** de `<dataDir>/pki/` como usuário do host.

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. A única extensão de motor (`wait-sync` via `eth_syncing`) é um gate coeso, análogo ao
`waitRPC` do TK-B6, injetável para testes; nenhuma dependência nova. O `write-genesis` reusa o padrão
de guard não-destrutivo do `gen-genesis`, trocando *gerar* por *copiar do bundle + validar hash*.
