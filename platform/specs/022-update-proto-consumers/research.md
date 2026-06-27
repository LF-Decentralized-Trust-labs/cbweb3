# Research: MD-3 — Estado dos Consumidores do Proto (Scenario A)

**Data**: 2026-06-27
**Método**: Leitura direta dos arquivos-fonte — nenhuma suposição, apenas código verificado

---

## Decisão principal

**Decision**: O Scenario A está em grande parte implementado. Os campos spoke-keyed já estão presentes em todos os layers de produção com exceção do vendor do `api-gateway`. O trabalho residual é pontual: atualizar o vendor stale, fechar cobertura de testes e verificar compilação end-to-end.

**Rationale**: MD-1 foi aplicado ao arquivo `.proto` e a regeneração do `backend/shared/proto` já foi executada. Os consumidores (domain, gRPC server, relay, frontend) foram atualizados antes desta spec ser criada.

**Alternatives considered**: Reescrever os consumidores do zero — rejeitado porque o código já existe e está correto.

---

## Mapeamento detalhado por componente

### 1. Proto definition — `apis/proto/.../payment_orchestrator.proto`

**Status**: ✅ COMPLETO

Campos `reserved 17, 18` + `reserved "spoke_a_receiver", "spoke_b_receiver"` presentes.
Novos campos adicionados:

```protobuf
// FXAgreement
string source_spoke_id = 21;
string dest_spoke_id   = 22;
string source_receiver = 23;
string dest_receiver   = 24;

// ProposeFXAgreementRequest
string source_spoke_id = 16;
string dest_spoke_id   = 17;
string source_receiver = 18;
string dest_receiver   = 19;
```

---

### 2. Generated pb.go — `backend/shared/proto/.../payment_orchestrator.pb.go`

**Status**: ✅ COMPLETO

Struct `FXAgreement` expõe:
- `SourceSpokeId string` (tag `bytes,21`)
- `DestSpokeId string` (tag `bytes,22`)
- `SourceReceiver string` (tag `bytes,23`)
- `DestReceiver string` (tag `bytes,24`)

Getters gerados: `GetSourceSpokeId()`, `GetDestSpokeId()`, `GetSourceReceiver()`, `GetDestReceiver()`.
Campos antigos não presentes.

---

### 3. Vendor pb.go — `api-gateway/vendor/.../payment_orchestrator.pb.go`

**Status**: ❌ STALE

Este arquivo é uma cópia vendor do shared proto que o serviço `api-gateway` importa como dependência Go. Ele ainda expõe `SpokeAReceiver`/`SpokeBReceiver` e os getters antigos (`GetSpokeAReceiver()`, `GetSpokeBReceiver()`).

**Causa**: O vendor não foi atualizado após a regeneração do shared proto.
**Ação necessária**: `go mod vendor` dentro de `scenario-a/backend/services/api-gateway/` para sincronizar o vendor com o módulo local atualizado.

---

### 4. Domain model — `payment-orchestrator/internal/domain/fx.go`

**Status**: ✅ COMPLETO

```go
type FXAgreementRecord struct {
    // ...
    SourceSpokeId  string // Spoke ID do leg de origem (ex: "spoke-brl")
    DestSpokeId    string // Spoke ID do leg de destino (ex: "spoke-usd")
    SourceReceiver string // Identidade Paladin no spoke de origem
    DestReceiver   string // Identidade Paladin no spoke de destino
    // ...
}
```

Campos antigos (`SpokeAReceiver`, `SpokeBReceiver`) não presentes.

---

### 5. gRPC server handler — `payment-orchestrator/internal/grpc/server/server.go`

**Status**: ✅ COMPLETO

Handler `ProposeFXAgreement` (linhas ~966-987):
```go
record := &domain.FXAgreementRecord{
    // ...
    SourceSpokeId:  req.SourceSpokeId,
    DestSpokeId:    req.DestSpokeId,
    SourceReceiver: req.SourceReceiver,
    DestReceiver:   req.DestReceiver,
    // ...
}
```

Função `fxRecordToProto` (linhas ~1353-1356):
```go
SourceSpokeId:  r.SourceSpokeId,
DestSpokeId:    r.DestSpokeId,
SourceReceiver: r.SourceReceiver,
DestReceiver:   r.DestReceiver,
```

---

### 6. Relay — `interop/hub-and-spoke/cacti/src/htlc-relay.ts`

**Status**: ✅ PARSING COMPLETO | ⚠️ ENDPOINT SELECTION — ver nota

**Interface interna `FXProposalEvent`** (linhas 53-74):
```typescript
sourceSpokeId: string;   // linha 67
destSpokeId: string;     // linha 68
sourceReceiver: string;  // linha 69
destReceiver: string;    // linha 70
```

**`pollFXAgreementsRest()`** (linhas ~611-614): extrai `source_spoke_id`/`dest_spoke_id` do payload REST ✅

**`proposeOnCounterpart()`** (linhas ~757-760): passa `source_spoke_id`/`dest_spoke_id` no gRPC ✅

**`resolveCounterpartContractId()`**: usa `hashLock` para encontrar o contrato HTLC correspondente no spoke contraparte — **este é o comportamento correto** para a mecânica HTLC (o hashLock é o vínculo entre os dois contratos); não é um uso posicional de A/B. ✅

**Nota — endpoint selection**: A seleção de QUAL endpoint gRPC chamar em `proposeOnCounterpart` ainda pode usar `config.ts` com `counterpartGrpc` estático (bilateral). Isso é tecnicamente correto para dois spokes, mas o roteamento dinâmico por `dest_spoke_id` (RL-2) é escopo da Fase 2. MD-3 pede apenas que o parsing dos legs leia `dest_spoke_id` — o que já acontece. O endpoint selection dinâmico é RL-1 + RL-2.

---

### 7. Frontend — `frontend/apps/bank/src/`

**Status**: ✅ COMPLETO

**`types/fx-agreement.types.ts`**: interfaces `FXAgreement` e `ProposeFXAgreementRequest` já usam `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver`.

**`pages/AgreementProposalPage.tsx`**: formulário submete:
```typescript
source_spoke_id: sourceSpokeId.trim() || undefined,
dest_spoke_id: destSpokeId.trim() || undefined,
source_receiver: sourceReceiver.trim() || undefined,
dest_receiver: destReceiver.trim() || undefined,
```
Inputs visuais com placeholder `"spoke-brl"` / `"spoke-usd"`.

---

## Resumo executivo

| Componente | Arquivo-chave | Status | Ação |
|---|---|---|---|
| Proto definition | `apis/proto/.../payment_orchestrator.proto` | ✅ | Nenhuma |
| Generated pb.go (shared) | `backend/shared/proto/...` | ✅ | Nenhuma |
| Vendor pb.go (api-gateway) | `api-gateway/vendor/...` | ❌ stale | `go mod vendor` |
| Domain model | `domain/fx.go` | ✅ | Nenhuma |
| gRPC handler | `grpc/server/server.go` | ✅ | Nenhuma |
| Relay parsing | `htlc-relay.ts` (REST + gRPC payload) | ✅ | Nenhuma |
| Relay endpoint selection | `htlc-relay.ts` (`proposeOnCounterpart`) | ⚠️ bilateral OK | Dinâmico = RL-2 (Fase 2) |
| Frontend tipos | `fx-agreement.types.ts` | ✅ | Nenhuma |
| Frontend formulário | `AgreementProposalPage.tsx` | ✅ | Nenhuma |
| Testes repositório | `gorm_repos_test.go` | ✅ | Cobrem migração MD-2 |
| Testes gRPC handler | `server_test.go` (se existir) | ⚠️ verificar | Adicionar assert novos campos |

---

## Comando proto-gen (referência)

```bash
# A partir de scenario-a/
make proto-gen
# Equivale a: cd apis/proto && buf generate
# Output: backend/shared/proto/payment_orchestrator/v1/*.pb.go
```

---

## Pendências abertas

1. **Vendor update**: `cd scenario-a/backend/services/api-gateway && go mod vendor`
2. **Build check**: `cd scenario-a/backend && go build ./...`
3. **Test coverage**: verificar se `gorm_repos_test.go` ou `server_test.go` têm assertions explícitas para `SourceSpokeId`/`DestSpokeId`; adicionar se ausentes.
4. **TypeScript type-check**: `cd scenario-a/frontend && tsc --noEmit`
5. **Grep final**: `grep -rn "spoke_a_receiver\|spoke_b_receiver" scenario-a/ --include="*.go" --include="*.ts" --include="*.tsx"` deve retornar zero ocorrências (exceto arquivos de migração/testes de upgrade).
