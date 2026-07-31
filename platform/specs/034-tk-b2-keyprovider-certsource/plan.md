# Implementation Plan: Toolkit do Cenário B — KeyProvider + CertSource (TK-B2/B3)

**Branch**: `034-tk-b2-keyprovider-certsource` | **Date**: 2026-07-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/034-tk-b2-keyprovider-certsource/spec.md`

## Summary

Implementar as duas interfaces plugáveis de custódia do toolkit do Cenário B: **KeyProvider**
(chaves blockchain secp256k1, custódia por entidade, `kms://`) e **CertSource** (CA do spoke
ECDSA P-256, emissão de leaf a partir de CSR, `self-signed`/`ca://`), cada uma com
implementação **local in-memory** + **stub de produção** atrás de factory por URI. Invariante:
**nenhum segredo** (chave do KeyProvider ou da CA) cruza manifesto/estado/bundle. Reimplementa
(não importa) o padrão do toolkit de referência (Cenário A). Fases seguintes (TK-B5+) consomem
estas interfaces; TK-B2/B3 não inclui motor/steps/bundles/execução.

## Technical Context

**Language/Version**: Go 1.26 (`go.mod` do módulo declara `go 1.26`, alinhado ao stack do repositório — "Go 1.26+"; o toolchain 1.26 é resolvido via `GOTOOLCHAIN=auto`).
**Primary Dependencies**: `github.com/ethereum/go-ethereum` (secp256k1 + derivação de endereço
EVM + assinatura) — **nova dependência**, justificada abaixo; stdlib `crypto/ecdsa`,
`crypto/elliptic` (P-256), `crypto/x509`, `crypto/rand`, `encoding/pem` (CertSource + CSR). Já
presente: `gopkg.in/yaml.v3` (TK-B1).
**Storage**: nenhum — chaves e CA vivem **apenas em memória**; nada é persistido (Princípio de
"sem segredos").
**Testing**: `go test` (gerar/assinar/verificar; emitir/validar CSR→leaf; factories; asserção de
"sem segredos").
**Target Platform**: biblioteca Go do módulo `scenario-b/toolkit` (consumida pelo motor futuro).
**Project Type**: módulo Go isolado — lib (novos pacotes `engine/keyprovider`,
`engine/certsource`, `engine/pki`).
**Performance Goals**: N/A (operações de cripto locais).
**Constraints**: sem segredos em manifesto/estado/bundle; chave privada da CA nunca em
disco/log/env; prod = stub; não importar `scenario-a/toolkit`.
**Scale/Scope**: chaves por entidade (N entidades); CA por spoke (N spokes). Escopo: as duas
interfaces + local + stub + factories; sem motor/steps.

## Constitution Check

*GATE: deve passar antes da Fase 0. Re-checado após a Fase 1.*

| Princípio | Avaliação (TK-B2/B3) |
|---|---|
| **I. Scenario-Scoped Independence** | ✅ Novos pacotes no módulo `scenario-b/toolkit`; **não importa** `scenario-a/toolkit` (padrão reimplementado). |
| **II. Privacy by Design** | ✅ As interfaces **são** a fronteira de custódia: chaves privadas e a chave da CA **nunca** saem do processo nem entram em manifesto/estado/bundle (FR-008, FR-012). Reforça o princípio. |
| **III. Atomic Settlement** | ✅ N/A — sem caminho de settlement nesta fase. |
| **IV. Compliance Gate** | ✅ N/A em runtime; o `CertSource` **habilita** o modelo CB-as-CA que sustenta o onboarding/compliance de fases futuras, sem contornar gate algum. |
| **V. Test-First** | ✅ Interfaces e impls locais nascem com testes `go test` que falham antes da implementação. |
| **VI. Observability** | ✅ Erros **tipados** e distinguíveis (FR-013); sem falhas silenciosas. (Libs, não serviços — logging estruturado não se aplica aqui.) |

**Nova dependência (justificativa exigida pela Constituição):** `github.com/ethereum/go-ethereum`
para **secp256k1** e derivação de endereço EVM — necessária porque as chaves do KeyProvider
assinam transações Besu/QBFT (curva secp256k1, fora do `crypto/ecdsa` da stdlib). Já prevista no
roadmap §3 e presente no toolkit de referência do Cenário A. Será referenciada no PR e no README
do cenário.

**Resultado (pré-Fase 0 e pós-Fase 1)**: PASS — sem violações.

## Project Structure

### Documentation (this feature)

```text
specs/034-tk-b2-keyprovider-certsource/
├── plan.md, spec.md
├── research.md          # Fase 0 — curva/derivação, seed do emulador, CA em memória
├── data-model.md        # Fase 1 — KeyProvider/CertSource (interfaces, estado, erros)
├── quickstart.md        # Fase 1 — usar as interfaces
├── contracts/           # Fase 1 — assinaturas das interfaces (Go API) + tabela de factory
└── checklists/requirements.md
```

### Source Code (repository)

```text
scenario-b/toolkit/
├── engine/keyprovider/
│   ├── keyprovider.go   # interface + erros
│   ├── local.go         # in-memory secp256k1 (derivação determinística + seed); ExportPrivateKeyHex (local-only)
│   ├── prod.go          # stub (ErrNotImplemented)
│   ├── factory.go       # New(uri): kms://local-emulator → local; kms://… → prod
│   └── *_test.go
├── engine/certsource/
│   ├── certsource.go    # interface + erros
│   ├── local.go         # CA self-signed P-256 por spoke (só memória); IssueLeafCert/GetTrustAnchor
│   ├── prod.go          # stub (ErrNotImplemented)
│   ├── factory.go       # New(uri): self-signed[://…] → local; ca://… → prod
│   └── *_test.go
└── engine/pki/
    ├── csr.go           # GenerateBankCSR (P-256, OU=ROLE_COMMERCIAL_BANK) — testes + fases futuras
    └── csr_test.go
```

**Structure Decision**: adicionar três pacotes ao módulo `scenario-b/toolkit` já existente
(`engine/keyprovider`, `engine/certsource`, `engine/pki`), espelhando os pacotes homônimos do
toolkit de referência. Não há CLI nesta fase (as interfaces são consumidas pelo motor em TK-B5+).

## Complexity Tracking

> Preencher só se o Constitution Check tiver violações a justificar.

Sem violações. A nova dependência `go-ethereum` **não** é um desvio: está no stack declarado do
roadmap (§3) e é a via idiomática para secp256k1/EVM — a alternativa (implementar secp256k1 na
mão) seria pior. A duplicação do padrão do Cenário A é exigida pela Constituição (Princípio I).
