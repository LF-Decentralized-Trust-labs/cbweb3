# Quickstart: MD-3 — Verificação e Finalização dos Consumidores do Proto

**Contexto**: A maioria das mudanças já está em produção no codebase. Este quickstart descreve como executar as verificações e as ações residuais.

---

## Pré-condições

- Branch: `022-update-proto-consumers`
- MD-1 (proto atualizado) e MD-2 (migração de banco) já aplicados
- `buf` instalado (para proto-gen)

---

## 1. Verificar grep de campos antigos

```bash
# A partir da raiz do repo
grep -rn "spoke_a_receiver\|spoke_b_receiver" scenario-a/ \
  --include="*.go" --include="*.ts" --include="*.tsx" \
  --include="*.proto"
```

**Esperado**: Retornar apenas arquivos de migração/testes de upgrade (`fx_agreement_gorm.go`, `gorm_repos_test.go`) e nenhum arquivo de código de produção.

---

## 2. Atualizar vendor do api-gateway

```bash
cd scenario-a/backend/services/api-gateway
go mod vendor
```

Após este comando, verificar que o vendor pb.go expõe os novos campos:

```bash
grep -n "SourceSpokeId\|DestSpokeId\|SourceReceiver\|DestReceiver" \
  vendor/github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1/payment_orchestrator.pb.go
```

---

## 3. Build Go completo

```bash
cd scenario-a/backend
go build ./...
```

**Esperado**: Zero erros de compilação relacionados a campos do FXAgreement.

---

## 4. Testes de backend

```bash
cd scenario-a/backend/services/payment-orchestrator
go test ./...
```

**Esperado**: Todos os testes passando. Verificar especificamente se há assertions para `SourceSpokeId`/`DestSpokeId` nos testes de integração do gRPC handler.

---

## 5. Type-check do frontend

```bash
cd scenario-a/frontend
# Via Turborepo
npx turbo run type-check --filter=bank
# Ou diretamente
cd apps/bank && npx tsc --noEmit
```

**Esperado**: Zero erros de tipo relacionados a `fx-agreement.types.ts`.

---

## 6. Verificação do relay

```bash
cd scenario-a/interop/hub-and-spoke/cacti
npx tsc --noEmit
```

**Esperado**: Compilação limpa. Verificar que `htlc-relay.ts` não referencia `spokeAReceiver` ou `spokeBReceiver` como campos de dados.

---

## 7. Grep final de validação

```bash
# Zero ocorrências esperadas em código de produção
grep -rn "SpokeAReceiver\|SpokeBReceiver\|spoke_a_receiver\|spoke_b_receiver" scenario-a/ \
  --include="*.go" --include="*.ts" --include="*.tsx" \
  --exclude-dir=vendor \
  | grep -v "_test.go" \
  | grep -v "fx_agreement_gorm.go"
```
