# Implementation Plan: TK-6 — Emissor do Join Bundle

**Branch**: `028-tk6-join-bundle-emitter` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/028-tk6-join-bundle-emitter/spec.md`

## Summary

Implementar o emissor do join bundle (`scenario-a/toolkit/engine/bundle/`): uma função Go pública `EmitBundle` que lê os artefatos produzidos pelo engine TK-5 em `SPOKE_DATA_DIR` (genesis, endereços de contratos, cert CA TLS, enode via JSON-RPC) e emite `bundles/<spoke-id>.bundle.yaml` — o artefato público e self-contained que permite a um banco comercial se conectar ao spoke (`mode: join`, TK-9). O bundle jamais contém chaves privadas; host e porta do enode são sempre derivados do manifesto, nunca do Besu. A interface `EnodeProvider` abstrai a chamada RPC para permitir testes sem Besu real.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: `gopkg.in/yaml.v3` (serialização do bundle), `encoding/pem` (validação CA cert), `crypto/sha256` (hash genesis), `encoding/base64` (embedding genesis), `net/http` (JSON-RPC admin_nodeInfo) — todos stdlib ou já em `toolkit/go.mod`  
**Storage**: Filesystem — `<SPOKE_DATA_DIR>/.deployed-addrs.env`, `<SPOKE_DATA_DIR>/genesis/genesis.json`, `<SPOKE_DATA_DIR>/tls/central-bank.crt`; saída: `<outputDir>/bundles/<spoke-id>.bundle.yaml`  
**Testing**: `go test -race` com `httptest.Server` para mock do Besu; `t.TempDir()` para artefatos de teste; build tag `integration` para E2E  
**Target Platform**: Linux (Docker Compose / Kubernetes); mesma plataforma do engine TK-5  
**Project Type**: Library Go — pacote `bundle` dentro do toolkit do Scenario A  
**Performance Goals**: `EmitBundle` completa em < 5 s (dominada pela chamada RPC ao Besu, máx 10 s timeout)  
**Constraints**: Zero dependências externas além das já em `toolkit/go.mod`; nenhum arquivo de chave privada lido ou escrito; escrita atômica via temp+rename (nunca bundle parcial no disco)  
**Scale/Scope**: Um bundle por spoke; emitido uma vez após `mode: found` completar (TK-5); consumido uma vez por banco comercial em `mode: join` (TK-9)

## Constitution Check

*GATE: Avaliado antes de Phase 0. Re-avaliado após Phase 1.*

| Princípio | Avaliação | Justificativa |
|---|---|---|
| **I — Scenario-Scoped Independence** | ✅ PASS | Pacote reside em `scenario-a/toolkit/engine/bundle/`; zero referência a `scenario-b/`; nenhum código compartilhado além de `engine/addrs/` (pacote interno do toolkit) |
| **II — Privacy by Design** | ✅ PASS | Bundle contém apenas material público (endereços, genesis, enode, cert CA). FR-008/009 garantem que `caCertPEM` nunca contém `PRIVATE KEY`. KeyProvider (TK-2) não é acessado pelo emissor |
| **III — Atomic Settlement Guarantee** | ✅ N/A | O bundle é um artefato de configuração/provisionamento, não uma transação de valor. Não altera nem intermedia nenhum caminho de liquidação |
| **IV — Compliance Gate Before Participation** | ✅ PASS | O emissor não introduz nem contorna nenhum compliance gate. O passo 9 do TK-5 (onboarding real no IdentityRegistry) já ocorreu antes de `EmitBundle` ser chamado |
| **V — Test-First at Every Layer** | ✅ PASS | SC-001 exige `go test -race ./engine/bundle/...` verde antes de qualquer implementação. Testes failing first em cada fase (Phases 3–6 de tasks.md) |
| **VI — Observability and Auditability** | ✅ PASS | `log/slog` em `EmitBundle`: início, conclusão, e erros com campos `spoke_id` e `error`. Zero silent failures (FR-013 proíbe swallowing) |

**Resultado**: Sem violações. Todas as gates passam. Pode prosseguir para Phase 0.

**Re-check pós-design (Phase 1)**: Princípio II confirmado pelo invariante de segurança em `data-model.md` (seção "Invariantes de segurança") e pela guarda explícita de `PRIVATE KEY` em FR-008.

## Project Structure

### Documentation (this feature)

```text
specs/028-tk6-join-bundle-emitter/
├── plan.md              # Este arquivo (/speckit.plan command output)
├── research.md          # Phase 0 output — 9 decisões documentadas (R-01 a R-09)
├── data-model.md        # Phase 1 output — schema YAML, structs Go, erros, invariantes
├── quickstart.md        # Phase 1 output — uso em Go, verificação manual, erros comuns
├── contracts/
│   └── bundle-schema.md # Phase 1 output — contrato EmitBundle, EnodeProvider, schema YAML
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code

```text
scenario-a/toolkit/engine/
├── addrs/                          # NOVO — pacote compartilhado (movido de orchestrator/)
│   ├── addrs.go                    # DeployedAddrs + parseDeployedAddrs (movido de orchestrator)
│   └── addrs_test.go
├── bundle/                         # NOVO — pacote principal do TK-6
│   ├── types.go                    # JoinBundle, BundleSpec, BundleInput e sub-types
│   ├── errors.go                   # Erros sentinela (ErrInvalidMode, ErrGenesisNotFound, etc.)
│   ├── enode.go                    # EnodeProvider interface + BesuEnodeProvider + parseAndRewriteEnode
│   ├── enode_test.go
│   ├── bundle.go                   # EmitBundle + readGenesis + readCACert + serialização atômica
│   └── bundle_test.go
└── orchestrator/
    ├── addrs.go                    # ALTERADO — re-exporta de engine/addrs/ (import path update)
    └── addrs_test.go               # ALTERADO — import atualizado
```

**Structure Decision**: Pacote `bundle` independente em `engine/bundle/`. A função `parseDeployedAddrs` é movida para `engine/addrs/` para ser compartilhada sem acoplamento (pesquisa R-06). Nenhum arquivo em `deploy/local/` ou `make/` é tocado.

## Complexity Tracking

> Sem violações da Constitution. Seção preenchida apenas para decisões não-óbvias de design.

| Decisão | Alternativa rejeitada | Motivo da rejeição |
|---|---|---|
| `EnodeProvider` como interface injetável | Chamar Besu diretamente em `EmitBundle` | Impede testes unitários sem Besu real; duplica o problema resolvido por `RelayRegistrar` em TK-5 |
| Mover `parseDeployedAddrs` para `engine/addrs/` | Duplicar em `engine/bundle/` | Dois lugares para manter; pacote compartilhado é o padrão correto |
| Genesis embutido como base64 (self-contained) | Referência por hash+path | TK-9 não precisaria de canal lateral; genesis < 50 KB nunca é problema de tamanho |
| Substituição incondicional de host/porta do enode | Substituir apenas se host for IP privado | Heurística de IP privado é frágil; manifesto é sempre a fonte de verdade (design doc §4) |

## Phases

### Phase 0: Research ✅

**Output**: [research.md](research.md)

Decisões documentadas:
- **R-01**: `admin_nodeInfo` — extração de enode-id + substituição incondicional de host/porta
- **R-02**: Genesis embedding base64 (self-contained); limite de 1 MB como warning
- **R-03**: PEM validation via `encoding/pem` + busca de `PRIVATE KEY` no conteúdo total
- **R-04**: Escrita atômica via temp+rename (idêntico ao `state.go` do TK-5)
- **R-05**: Caller é TK-7 (`cbweb3 apply`), não TK-5 — separation of concerns
- **R-06**: `parseDeployedAddrs` movida para `engine/addrs/` (pacote compartilhado)
- **R-07**: `EnodeProvider` interface injetável com `BesuEnodeProvider` como impl padrão
- **R-08**: `base64.StdEncoding` (RFC 4648, sem newlines) para genesis content
- **R-09**: Hash format `sha256:<hex-lowercase>` (consistente com OCI digest)

Todas as NEEDS CLARIFICATION resolvidas. Nenhum risco técnico desbloqueador.

### Phase 1: Design & Contracts ✅

**Output**: [data-model.md](data-model.md), [contracts/bundle-schema.md](contracts/bundle-schema.md), [quickstart.md](quickstart.md)

Artefatos produzidos:
- **data-model.md**: Schema YAML completo, structs Go (`JoinBundle`, `BundleInput`, `EnodeProvider`, erros sentinela), caminhos de artefatos, invariantes de segurança
- **contracts/bundle-schema.md**: Contrato público de `EmitBundle`, `EnodeProvider`, erros, schema YAML com restrições de validação, exemplo de uso Go, regra de versionamento
- **quickstart.md**: Exemplo Go funcional integrado ao TK-7, verificação manual do bundle, tabela de erros comuns, instruções de distribuição

**Constitution Check pós-design**: Todos os 6 princípios confirmados. Nenhum ajuste de design necessário.

### Phase 2: Tasks

**Output**: [tasks.md](tasks.md) — gerado por `/speckit.tasks`

25 tarefas (T001–T025) organizadas em 7 fases:
1. **Phase 1 — Scaffolding**: mover `parseDeployedAddrs` para `engine/addrs/` (T001–T002)
2. **Phase 2 — Tipos**: structs, erros, `EnodeProvider` interface (T003–T006)
3. **Phase 3 — Validação**: fail-fast por artefato ausente, test-first (T007–T009)
4. **Phase 4 — Coleta**: enode parsing, genesis, cert CA, test-first (T010–T015)
5. **Phase 5 — Serialização**: composição final + escrita atômica (T016–T018)
6. **Phase 6 — BesuEnodeProvider**: impl HTTP + testes via `httptest` (T019–T021)
7. **Phase 7 — Polish**: logs `slog`, testes de segurança, integração E2E (T022–T025)
