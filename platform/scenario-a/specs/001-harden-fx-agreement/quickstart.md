# Quickstart: Harden FX Agreement for Production

## 1. Preconditions

- Backend stack running (API Gateway + payment-orchestrator + relay + Postgres).
- Environment files configured per entity.
- Existing FX Agreement APIs reachable.

## 2. Apply Schema and Repository Wiring

1. Create DB migrations for:
- `fx_agreements`
- `fx_agreement_events`
- relay durability tables (processed/pending)

2. Wire `FXAgreementRepository` in payment-orchestrator startup.
3. Keep transitional compatibility path behind feature flags until cutover.

## 3. Implement Lifecycle Persistence and Audit

1. Replace in-memory agreement writes/reads with repository operations.
2. On every transition (`propose/accept/reject/cancel/settle`), append immutable event.
3. Ensure `Get` and `List` APIs read from persistent source.

Verification:
- Propose agreement.
- Restart payment-orchestrator.
- Confirm `GetFXAgreement` still returns same state and data.
- Confirm audit event sequence for `trade_id`.

## 4. Harden Cross-Spoke Relay

1. Add persistent idempotency key store.
2. Add retry queue with exponential backoff.
3. Extend forwarding coverage to `CANCELLED` and `SETTLED`.
4. Protect internal FX list endpoint with service-to-service auth (shared secret or mTLS).

Verification:
- Simulate counterpart outage.
- Confirm event enters retry path and is delivered after recovery.
- Confirm no duplicate state transitions after relay restart.

## 5. Integrate Pente Bilateral Context

1. Introduce `PenteClient` adapter in payment-orchestrator.
2. Create/reuse bilateral group per counterparty pair.
3. Persist `group_id` and `contract_address` references in agreement record.

Verification:
- New accepted agreement contains bilateral reference metadata.
- Counterpart can query same agreement context by shared reference.

## 6. Strengthen HTLC Agreement Coupling

1. Enforce strict agreement linkage in lock operations.
2. Reject locks when agreement is not `ACCEPTED`, expired, or mismatched in amount/receiver.
3. Run production mode fail-closed on missing spoke identity context.

Verification:
- Lock with invalid or expired agreement must fail.
- Lock with valid accepted agreement must succeed.

## 7. Expiry Automation

1. Add periodic expiration worker.
2. Auto-cancel expired non-terminal agreements.
3. Emit audit event for system-driven cancellation.

Verification:
- Create short-lived agreement in `PROPOSED`.
- Wait for expiry window.
- Confirm automatic transition to `CANCELLED` + audit event.

## 8. Recommended Validation Commands

```bash
# Payment orchestrator tests
cd backend/services/payment-orchestrator
go test ./...

# API gateway tests (if route changes included)
cd ../api-gateway
go test ./...

# Relay build
cd ../../../interop/hub-and-spoke/cacti
npm install
npm run build

# Contracts validation (if HTLC gate/ABI changed)
cd ../../../contracts
forge build
forge test -vvv
```

## 9. Exit Criteria

- No data loss after restart.
- Full audit chain for every transition.
- Relay delivery reliability and dedup proven under failure simulation.
- HTLC lock bypass paths blocked.
- Expiration automation active and observable.

## 10. Validation Result (2026-04-14)

- Payment Orchestrator: `go test ./...` passou.
- API Gateway: rotas/adapters alterados compilam; suíte completa mantém falhas preexistentes fora do escopo (CORS test setup e onboarding assertion).
- Contracts: `forge build` passou após adicionar fallback gate por commitment no HTLC.
- Relay TypeScript: build não executado neste ambiente por ausência de `tsc` no PATH.

## 11. Local Paladin Readiness (Zeto + Pente) — OBRIGATÓRIO — 2026-04-15

**Status**: CRITICAL DELIVERABLE — Zeto E Pente DEVEM estar operacionais simultaneamente (não opcionais).

### Current observed status (2026-04-15)

- Zeto: setup local AUTOMÁTICO e validado via targets de `make` no pipeline Paladin ✅
- Pente: integração de aplicação backend disponível (`PENTE_ENABLED`, `PENTE_BASE_URL`, `PenteClient`), MAS sem bootstrap local no diretório `deploy/local/paladin` — **GAP CRÍTICO**

### OBRIGATÓRIO: Completion required to enable production validation

**Phase 7 (T043–T050) é deliverable crítico** para fechar o gap e garantir operação real:

1. Criar manifests/scripts de bootstrap OBRIGATÓRIO do Pente no Paladin local.
2. Adicionar targets `make` OBRIGATÓRIOS para deploy/bootstrap Pente por spoke (não feature-flagged).
3. Atualizar pipeline principal para invocar Zeto + Pente automaticamente (ambos REQUIRED).
4. Executar setup completo e validar **simultaneamente** (não sequencialmente):
   - ✅ Zeto token instance operacional
   - ✅ Criação de contexto FX Pente com retorno de `group_id` e `contract_address`
   - ✅ Fluxo FX agreement (propose → accept) funcionando COM Pente ativo

### Validation evidence (OBRIGATÓRIO para considerar feito)

- Comandos `make` executados com output de sucesso por spoke.
- IDs de contexto/contrato Pente retornados (evidência de sucesso).
- Execução integrada de fluxo FX com Pente habilitado (`PENTE_ENABLED=true`, não bypass/fallback).

### Execution Steps for Phase 7 (T043–T050)

**Prerequisites**: Besu networks running (`make deploy.up-spoke-a deploy.up-spoke-b`)

**Phase 7A: Paladin Setup (Zeto + Pente)**
```bash
# Setup spoke-a with Pente (includes Zeto automatically)
make setup-spoke-a

# Setup spoke-b with Pente (includes Zeto automatically)
make setup-spoke-b
```

**Expected Output**:
```
Spoke-a setup complete (Zeto + Pente BOTH OPERATIONAL)
Spoke-b setup complete (Zeto + Pente BOTH OPERATIONAL)
```

**Phase 7B: FX Agreement Flow with Pente Active**
```bash
# With PENTE_ENABLED=true in .env.infra.bank-a and .env.infra.bank-b:

# 1. Start backend stack
make backend.start-spoke-a
make backend.start-spoke-b

# 2. Run E2E FX flow with Pente validation
./tryouts/tryout-fx-agreement-e2e.sh --skip-onboarding

# 3. Verify Pente context was used
#    Expected: FX_AGREEMENT_ADDRESS logged
#    Expected: Bilateral context operations succeeded
#    Expected: No fallback to CommitmentHashRegistry
```

**Phase 7C: Validation Results**

Record the following evidence after execution:

1. **Make targets output** (copy from terminal):
   - `make setup-spoke-a` completion timestamp
   - `make setup-spoke-b` completion timestamp
   - PENTE_CONTEXT_GROUP_ID values
   - FX_AGREEMENT_ADDRESS values

2. **E2E test output** (copy from terminal):
   - Trade ID created
   - Agreement accepted (with Pente bilateral context used)
   - HTLC lock succeeded (using Pente-backed FXAgreement)
   - HTLC settlement succeeded

3. **Log verification**:
   ```bash
   # Confirm Pente is active
   docker logs backend-payment-orchestrator-bank-a | grep -i pente
   docker logs backend-payment-orchestrator-bank-b | grep -i pente
   ```
   Expected: "PENTE_ENABLED=true" or similar success messages

**Phase 7 Completion Criteria**:
- [ ] `make setup-spoke-a` completed successfully (Zeto + Pente)
- [ ] `make setup-spoke-b` completed successfully (Zeto + Pente)
- [ ] E2E test passed with `PENTE_ENABLED=true`
- [ ] No fallback to CommitmentHashRegistry observed
- [ ] Bilateral FX context operations succeeded
- [ ] Evidence documented below
