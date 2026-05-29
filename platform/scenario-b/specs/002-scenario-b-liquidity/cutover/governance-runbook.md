# Governance Runbook — Scenario B

> Documento de referência operacional para os mecanismos de governança do Cenário B.
> Audiência: Operadores de Hub, Bancos Centrais autorizados, Compliance.

---

## 1. Circuit Breaker Assimétrico (FR-030 / FR-044)

### Visão Geral

O Circuit Breaker garante que qualquer Banco Central autorizado possa parar o AMM de forma imediata (fail-safe), mas a retomada requer consenso de pelo menos 2 signatários distintos.

| Operação | Quórum | Endpoint |
|----------|--------|----------|
| Pause | 1-of-N (qualquer BC autorizado) | `POST /api/v2/governance/circuit-breaker/pause` |
| Resume (proposta) | 1 BC inicia | `POST /api/v2/governance/circuit-breaker/resume-request` |
| Resume (assinatura extra) | 1 BC adicional (total ≥ 2) | `POST /api/v2/governance/circuit-breaker/resume-sign` |
| Status | — | `GET /api/v2/governance/circuit-breaker/status` |

### Estados

```
LIVE  ──[pause/1-of-N]──>  HALTED
HALTED  ──[resume-request]──>  RESUME_PENDING
RESUME_PENDING  ──[resume-sign ≥2 distintos]──>  LIVE
RESUME_PENDING  ──[resume-sign <2 ou mesmo signatário]──>  RESUME_PENDING  (CircuitBreakerResumeDisputed emitido)
```

### Regras de Segurança (SC-026)

- **Pause** executa instantaneamente em 1 bloco após confirmação on-chain (SC-017).
- **Resume** requer signatários com `signer_bank_id` estritamente distintos — mesmo BC não pode atingir quórum sozinho.
- Quando quórum insuficiente, o evento `CircuitBreakerResumeDisputed` é emitido on-chain e a transição é revertida.
- Todas as assinaturas são persistidas como append-only em `circuit_breaker_signatures` (FR-048).
- Swaps são rejeitados enquanto o estado for `HALTED`.

### Procedimento Operacional

#### Ativar Pause

```bash
curl -X POST https://<gateway>/api/v2/governance/circuit-breaker/pause \
  -H "Authorization: Bearer <central_bank_token>" \
  -H "Content-Type: application/json" \
  -d '{"pair":"BRL/DREX","bank_id":"BCB","reason_code":"SYSTEMIC_RISK","signature":"<hex>"}'
```

Resposta esperada: `{"state":"HALTED","pair":"BRL/DREX"}`

#### Propor Resume

```bash
curl -X POST https://<gateway>/api/v2/governance/circuit-breaker/resume-request \
  -H "Authorization: Bearer <central_bank_token_1>" \
  -d '{"pair":"BRL/DREX","bank_id":"BCB","signature":"<hex>"}'
```

Resposta esperada: `{"state":"RESUME_PENDING","request_id":"<uuid>"}`

#### Assinar Resume (2° Signatário)

```bash
curl -X POST https://<gateway>/api/v2/governance/circuit-breaker/resume-sign \
  -H "Authorization: Bearer <central_bank_token_2>" \
  -d '{"pair":"BRL/DREX","request_id":"<uuid>","bank_id":"BCB2","signature":"<hex>"}'
```

Resposta esperada (quórum atingido): `{"state":"LIVE","pair":"BRL/DREX"}`

---

## 2. Master Viewing Key — Multi-Assinatura (FR-034 / FR-035 / FR-036 / SC-027)

### Visão Geral

O Master Viewing Key (MVK) permite que autoridades regulatórias com role `central_bank` solicitem disclosure de dados de transações confidenciais via Paladin JSON-RPC. O quórum mínimo é 2-of-3 signatários autorizados. Requests expiram automaticamente após **72 horas**.

| Operação | Endpoint |
|----------|----------|
| Abrir solicitação | `POST /api/v2/oversight/disclosure-request` |
| Assinar solicitação | `POST /api/v2/oversight/disclosure-sign` |
| Consultar status | `GET /api/v2/oversight/disclosure-status/:requestID` |

### Ciclo de Vida

```
PENDING  ──[assinatura 1]──>  PENDING (quorum_reached=1)
PENDING  ──[assinatura 2 distinta]──>  APPROVED  (chama Paladin RPC)
PENDING  ──[72h sem quórum]──>  EXPIRED  (via DisclosureExpiryWorker)
APPROVED  ──[leitura]──>  APPROVED  (imutável após aprovação)
```

### Regras (SC-027)

- `quorum_required = 2`, `quorum_reached` incrementado a cada `DisclosureSignature` válida.
- Signatários `signer_bank_id` devem ser distintos.
- `expires_at = opened_at + 72h` — configurável mas não pode ser zero.
- Todas as assinaturas são append-only (FR-035); UPDATE/DELETE bloqueados por trigger PL/pgSQL.
- O `DisclosureExpiryWorker` roda a cada 1 minuto e expira requests pendentes.

### Procedimento Operacional

```bash
# 1. Abrir request
curl -X POST https://<gateway>/api/v2/oversight/disclosure-request \
  -H "Authorization: Bearer <central_bank_token>" \
  -d '{"tx_ref":"TX-001","requestor_id":"BACEN","reason_code":"AML_INVESTIGATION"}'

# 2. Primeira assinatura
curl -X POST https://<gateway>/api/v2/oversight/disclosure-sign \
  -H "Authorization: Bearer <central_bank_token>" \
  -d '{"request_id":"<uuid>","signer_id":"BACEN"}'

# 3. Segunda assinatura (BC diferente)
curl -X POST https://<gateway>/api/v2/oversight/disclosure-sign \
  -H "Authorization: Bearer <central_bank_token_2>" \
  -d '{"request_id":"<uuid>","signer_id":"CMN"}'
# → Paladin RPC é invocado automaticamente ao atingir quórum

# 4. Verificar status
curl https://<gateway>/api/v2/oversight/disclosure-status/<request_id>
```

---

## 3. Riscos Aceitos (FR-047 / FR-053 / FR-054 / SC-025)

| Risco | Descrição | Mitigação |
|-------|-----------|-----------|
| **Sem rate limiting** | FR-053/FR-054 — throttling não implementado nesta iteração. | Monitorar manualmente; adicionar em iteração futura. |
| **Sem observabilidade estruturada** | Prometheus/OTel ausentes (FR-051/FR-052). Apenas logs em stdout. | Coletar logs via infraestrutura externa. |
| **Incompatibilidade de frontend** | `frontend/apps/` intocado (Decision 18 / SC-025). Interface existente não foi atualizada para API v2. | Criar adaptador de UI em sprint posterior. |
| **Retenção indefinida sem purge** | Sem jobs de TTL/purge (FR-045/FR-047). Particionamento por tempo substitui purge no curto prazo. | Avaliar e implementar TTL em iteração futura. |
| **Hardening contra DBA** | FR-050 — triggers append-only não impedem DDL por superusuário (ex: `DROP TRIGGER`). | Controle via permissões de banco e auditoria de acesso. |

---

## 4. Monitoramento de Liquidez (FR-028 / SC-014 / SC-023)

O `LiquidityMonitorService` faz polling periódico do status do pool e dispara `LiquidityAlert` quando a razão reserveA/total excede 70%. O alerta é log + persist; não há notificação automática push nesta iteração.

- Cadência alvo: p95 ≤ 15s (SC-023).
- Threshold: 70/30 — se `reserve_a / (reserve_a + reserve_b) > 0.70`.
- Logs em stdout: `[LiquidityMonitor] imbalance detected for pair=BRL/DREX ratio=0.72`.
