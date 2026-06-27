# Implementation Plan: TK-2 — Interface keyProvider

**Branch**: `024-tk2-keyprovider-interface` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/024-tk2-keyprovider-interface/spec.md`

## Summary

Criar a abstração de gerenciamento de chaves secp256k1 para o toolkit de provisionamento do Scenario A. A regra central é que chaves privadas nunca aparecem em arquivos, variáveis de ambiente, manifestos ou logs — o motor de orquestração recebe apenas chaves públicas e assinaturas. A feature entrega: (1) a interface `KeyProvider` com três operações, (2) uma implementação local em memória para o perfil `local`, (3) um stub de produção para a Fase 4, e (4) um factory que instancia o provedor correto a partir da URI do campo `spec.keyProvider` do manifesto.

O padrão de referência já existe no projeto: `backend/services/auth/internal/kms/` usa exatamente esse modelo com `go-ethereum/crypto` e `sync.RWMutex`.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: `github.com/ethereum/go-ethereum v1.17.1` (já presente em múltiplos módulos do Scenario A; adicionar ao `toolkit/go.mod`)  
**Storage**: Nenhum — implementação local usa in-memory map; sem persistência  
**Testing**: `go test ./...` com tabelas de teste; padrão `t.TempDir()` para isolamento  
**Target Platform**: Linux (mesma plataforma do toolkit)  
**Project Type**: Biblioteca Go (package), integrada ao toolkit de provisionamento  
**Performance Goals**: Operações individuais imperceptíveis para o operador (<100ms local) — sem requisito de throughput  
**Constraints**: Chaves privadas nunca em disco, log, ou retorno de função; thread-safe sob chamadas concorrentes para o mesmo ID  
**Scale/Scope**: Uma chave por participante por sessão; toolkit single-process

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Status | Observação |
|-----------|--------|------------|
| **I. Scenario-Scoped Independence** | ✅ PASS | Todo o código vai para `scenario-a/toolkit/engine/keyprovider/`. Nenhum arquivo de `scenario-b/` tocado. |
| **II. Privacy by Design** | ✅ PASS | A interface garante por contrato que chaves privadas nunca são retornadas. O emulador local não grava material de chave em disco ou log. |
| **III. Atomic Settlement Guarantee** | ✅ N/A | TK-2 é infraestrutura de provisionamento, não lógica de liquidação. Não envolve HTLC, FXAgreement ou relay. |
| **IV. Compliance Gate Before Participation** | ✅ PASS | TK-2 viabiliza a geração de chaves necessária para o fluxo de onboarding real no IdentityRegistry (FR-001 do concat.md). Não bypassa nenhum gate de compliance. |
| **V. Test-First at Every Layer** | ✅ PASS | `local_test.go` é criado antes de `local.go` e `factory.go`. Ciclo Red-Green-Refactor explicitamente exigido. |
| **VI. Observability and Auditability** | ✅ PASS | TK-2 é uma biblioteca; não emite logs de infraestrutura. Nenhuma chave privada pode aparecer em log (garantido pelo design da interface). |

**Resultado**: Sem violações. Aprovado para Phase 1.

**Justificativa de dependência nova** (Technology Stack Constraints): `go-ethereum v1.17.1` já é dependência padrão em `backend/shared/blockchain/go.mod`, `backend/services/auth/go.mod` e outros. A adição ao `toolkit/go.mod` é consistente com o padrão do projeto e não introduz nova tecnologia.

## Project Structure

### Documentation (this feature)

```text
specs/024-tk2-keyprovider-interface/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code

```text
scenario-a/toolkit/
├── go.mod                          # adicionar github.com/ethereum/go-ethereum v1.17.1
├── go.sum
└── engine/
    ├── manifest/                   # existente (TK-1) — não modificar
    ├── genesis/                    # existente — não modificar
    ├── pki/                        # existente — não modificar
    └── keyprovider/                # NOVO
        ├── keyprovider.go          # interface + sentinel errors + helper EVMAddress
        ├── local.go                # emulador local in-memory (secp256k1 via go-ethereum)
        ├── prod.go                 # stub prod (ErrNotImplemented em tudo)
        ├── factory.go              # New(uri string) (KeyProvider, error)
        └── local_test.go           # testes table-driven (escritos antes das implementações)
```

**Structure Decision**: Package único `engine/keyprovider/` dentro do módulo `scenario-a/toolkit/`, paralelo a `engine/manifest/` e `engine/genesis/`. Sem subpacotes — o escopo é pequeno e a interface, as implementações e o factory formam uma unidade coesa.

## Complexity Tracking

*Sem violações da constituição — seção vazia.*
