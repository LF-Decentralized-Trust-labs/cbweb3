# Implementation Plan: Bridge-Based CB Liquidity (Fluxo Soberano)

**Branch**: `007-bridge-based-cb-liquidity` | **Date**: 2026-05-20 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/007-bridge-based-cb-liquidity/spec.md`

## Summary

Substituir o padrão G5-cross (depreciado em spec-005) pelo fluxo soberano de provisão de liquidez transfronteiriça: cada Banco Central emite sua moeda no próprio spoke, bloqueia tokens no Bridge, recebe W-tCeBM no Hub via Bridge Relayer, e deposita W-tCeBM no AMM usando exclusivamente seu próprio signer soberano. A coordenação bilateral entre gateways é feita on-chain via novo contrato `LiquidityCommitRegistry` no Hub — trustless, sem comunicação inter-gateway direta, escalável para N CBs futuros.

## Technical Context

**Language/Version**: Go 1.25.5 (api-gateway backend), Solidity 0.8.20 (LiquidityCommitRegistry + deploy scripts), TypeScript 5.4 + Node 20 (CommitMatchedWatcher — extend Cacti Relayer)  
**Primary Dependencies**: Fiber v2.52.9 (HTTP), gRPC/protobuf (inter-serviço), GORM + PostgreSQL driver, go-ethereum v1.17.1, ethers v6 (watcher TS), @grpc/grpc-js, Hyperledger Cacti packages, OpenZeppelin 5.x (AccessControl, ReentrancyGuard no novo contrato), Foundry/Forge (testes Solidity)  
**Storage**: PostgreSQL — tabelas `bridged_asset_positions` (existente), `pool_commits` (migração: +`on_chain_commit_id` bytes32 nullable), `liquidity_positions` (sem mudança); Redis — noncestore existente  
**Testing**: `go test ./...` (Go), `forge test` (Solidity — novos testes em `LiquidityCommitRegistry.t.sol`), `npm test` (TypeScript watcher)  
**Target Platform**: Linux server (Docker Compose), Hyperledger Besu (Hub + Spokes)  
**Project Type**: Web-service (api-gateway) + Smart contract (Hub) + Event watcher daemon (TypeScript)  
**Performance Goals**: `CommitMatched` → ambas as pernas de `addSingleSidedLiquidity` confirmadas em p95 ≤ 30s em ambiente local (NFR-001); p95 ≤ 10s para o protocolo cross-gateway em condições normais (SC-003)  
**Constraints**: Zero mudança de contrato de API REST externo (NFR-002); `msg.sender` on-chain sempre é o signer soberano do CB emissor (SC-001); nenhuma comunicação direta entre gateways (NFR-003)  
**Scale/Scope**: 2 CBs (CB-A/CB-B) no ambiente local; design escalável para N CBs sem alteração de protocolo

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

O arquivo `constitution.md` contém apenas o template sem princípios explicitamente preenchidos. Com base na motivação da feature e nos princípios implícitos derivados das specs anteriores:

| Princípio implícito | Status | Observação |
|---|---|---|
| CB-A não deve deter e controlar a moeda do CB-B | ✅ PASS | Objetivo central desta feature — elimina G5-cross |
| `msg.sender` on-chain deve corresponder ao emissor soberano do token | ✅ PASS | Enforceado por `LiquidityCommitRegistry` e validação do handler de execução |
| APIs externas não devem ter breaking changes | ✅ PASS | Sem mudança de assinatura de endpoints existentes (NFR-002) |
| Novos contratos devem usar padrões de segurança existentes (OpenZeppelin) | ✅ PASS | `LiquidityCommitRegistry` usa `AccessControl` e padrão OWASP-safe (sem transferências de tokens) |
| Coordenação inter-gateway não deve exigir segredo compartilhado off-chain | ✅ PASS | Coordenação é on-chain via eventos; `INTER_GATEWAY_AUTH_SECRET` não aplicável (NFR-003) |

**Re-check pós-design**: PASS. Nenhuma violação identificada. O design é mais restritivo que o estado anterior (elimina uma violação existente em spec-005).

## Project Structure

### Documentation (this feature)

```text
specs/007-bridge-based-cb-liquidity/
├── plan.md              # Este arquivo
├── research.md          # Phase 0 ✅
├── data-model.md        # Phase 1 ✅
├── quickstart.md        # Phase 1 ✅
├── contracts/           # Phase 1 ✅
│   ├── ILiquidityCommitRegistry.sol
│   └── README.md
└── tasks.md             # Phase 2 — gerado por /speckit.tasks (não criado aqui)
```

### Source Code (repository root)

```text
contracts/
├── src/
│   └── LiquidityCommitRegistry.sol          # NOVO contrato Solidity (spec-007)
├── interfaces/
│   └── ILiquidityCommitRegistry.sol         # NOVO (derivado de contracts/ desta spec)
├── test/
│   └── LiquidityCommitRegistry.t.sol        # NOVO testes Foundry
└── script/
    └── SeedNewSovereignPair.s.sol           # NOVO script parametrizado — *C2 remediation*

backend/
└── services/
    └── api-gateway/
        └── internal/
            ├── http/
            │   ├── handlers/
            │   │   ├── liquidity_handler.go # MODIFICADO: commit gate T008-T010, execute-matched-commit T012, removeLiquidity T015, sovereign-add T021, provider_id JWT FR-007/FR-009
            │   │   └── token_handler.go     # MODIFICADO: guard anti-G5-cross (FR-004 / T016)
            │   └── router/v2/router.go      # MODIFICADO: rota /internal/amm/execute-matched-commit, /sovereign-add
            ├── app/
            │   └── liquidity_commit_registry.go  # NOVO: adapter on-chain para LiquidityCommitRegistry
            │   └── cb_checker.go                 # NOVO: CentralBankChecker (anti-G5-cross)
            └── services/
                └── sovereign_liquidity_service.go # NOVO: orquestrador do fluxo soberano

interop/
└── hub-and-spoke/
    └── cacti/
        └── src/
            └── liquidity-commit-watcher.ts  # NOVO: event watcher para CommitMatched

tryouts/
└── tryout-sovereign-cb-liquidity.sh         # NOVO script E2E dedicado (FR-008)
```

---

## Phase 0: Research

**Status**: ✅ Completo — ver [research.md](./research.md)

### Decisões-chave

| Questão | Decisão |
|---|---|
| W-tCeBM: tipo ou deploy? | Novo deploy de `TokenizedCentralBankMoney.sol` (mesmo contrato, símbolo distinto) |
| `getCentralBankOf` existe? | SIM — `IIdentityRegistry.sol:90`; usado em `PairRegistry` |
| Event watcher padrão | Novo TS `liquidity-commit-watcher.ts` (infra do Cacti HTLC relay, sem modificar o existente) |
| `addSingleSidedLiquidity` assinatura | `(bool isTokenA, uint256 amount)` — isTokenA mapeado de `side: "A"\|"B"` |
| Guard anti-G5-cross (FR-004) | `getParticipant(recipient).role == CENTRAL_BANK` (mais direto que `getCentralBankOf`) |
| PairRegistry flow | proposePair (CB-A) + confirmPair (CB-B) — bilateral, assinado pelos signers corretos |

---

## Phase 1: Design

### Artefatos gerados

| Artefato | Status | Link |
|---|---|---|
| `research.md` | ✅ | [research.md](./research.md) |
| `data-model.md` | ✅ | [data-model.md](./data-model.md) |
| `contracts/ILiquidityCommitRegistry.sol` | ✅ | [contracts/ILiquidityCommitRegistry.sol](./contracts/ILiquidityCommitRegistry.sol) |
| `contracts/README.md` | ✅ | [contracts/README.md](./contracts/README.md) |
| `quickstart.md` | ✅ | [quickstart.md](./quickstart.md) |

### LiquidityCommitRegistry — Resumo do design

```
registerCommit(poolPair, side, amount, wTokenAddress)
  → valida: getCentralBankOf(wTokenAddress) == msg.sender
  → armazena commit com expiresAt = now + 72h
  → se contrapartida PENDING: emite CommitMatched atomicamente
  → retorna: bytes32 commitId

cancelCommit(commitId)
  → valida: commit.signer == msg.sender e status == PENDING
  → emite CommitCancelled

expireCommit(commitId)
  → permissionless; valida: block.timestamp > expiresAt
  → emite CommitExpired
```

**Segurança OWASP**: sem transferências de tokens no contrato (apenas coordenação); anti-reentrância não aplicável; `msg.sender` como fonte de verdade de identidade on-chain.

### Gate pós-design

Re-verificação Constitution Check: ✅ PASS — design preserva soberania monetária, sem breaking changes de API, sem segredos compartilhados inter-gateway.

---

## Implementation Notes (para /speckit.tasks)

### Ordem de implementação obrigatória

1. **Contratos Solidity primeiro** (bloqueiam tudo):
   - `LiquidityCommitRegistry.sol` (implementação da interface)
   - `LiquidityCommitRegistry.t.sol` (8 testes Foundry — ver contracts/README.md)
   - `SeedNewSovereignPair.s.sol` (script de deploy parametrizado — *remediação C2*)

2. **Backend Go**:
   - Adapter `liquidity_commit_registry.go` (chamadas on-chain via go-ethereum)
   - Guard anti-G5-cross em `token_handler.go` (FR-004 / T016) — independente, pode ser paralelo
   - Gate `bridge_state=ACTIVE` em commit handler (FR-001) — depende do adapter
   - `provider_id` JWT validation em `liquidity_handler.go` (FR-007, FR-009) — independente
   - Rota + handler `/internal/amm/execute-matched-commit` — depende do adapter
   - `sovereign_liquidity_service.go` — orquestra tudo

3. **TypeScript watcher**:
   - `liquidity-commit-watcher.ts` — depende do contrato deployado (endereço `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`)
   - Pode ser desenvolvido em paralelo com o backend Go

4. **Script de deploy + integração**:
   - `SeedNewSovereignPair.s.sol` executado no ambiente local (`make contracts.seed-sovereign-pair`)
   - Variáveis de ambiente adicionadas aos docker-compose correspondentes

5. **Script E2E** (último — valida tudo junto):
   - `tryout-sovereign-cb-liquidity.sh`

### Dependências de dados

- `PoolCommit` precisa de migração: adicionar coluna `on_chain_commit_id bytes32 NULL`
- Nenhuma outra mudança de schema

### Ambiente local

- `LIQUIDITY_COMMIT_REGISTRY_ADDRESS`, `SOVEREIGN_PAIR_AMM_MAP` (JSON: `{"W-BRL-ARS":"0x..."}`, em vez de `SOVEREIGN_AMM_ADDRESS` único), `SOVEREIGN_PAIR_IDS` (lista), `W_TOKEN_BRL_ADDRESS`, `W_TOKEN_ARS_ADDRESS` adicionados aos `.env` dos gateways de CB-A e CB-B
- Watcher TypeScript roda como container adicional no `docker-compose-backend.bank-a.yaml` e `bank-b.yaml`

---

## Artifacts Map

```text
specs/007-bridge-based-cb-liquidity/
├── spec.md              ← Input (existente, com 5 Q&As respondidas)
├── plan.md              ← Este arquivo (Phase 0+1 output)
├── research.md          ← Phase 0: decisões e referências
├── data-model.md        ← Phase 1: entidades, estados, env vars
├── quickstart.md        ← Phase 1: guia passo-a-passo do fluxo soberano
├── contracts/
│   ├── ILiquidityCommitRegistry.sol  ← Interface Solidity do contrato
│   └── README.md        ← Especificação completa dos contratos novos
└── tasks.md             ← Phase 2 (gerado por /speckit.tasks — ainda não criado)
```
