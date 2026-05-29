# Quickstart: Testar Auth & Onboarding Restaurado

**Branch**: `003-restore-auth-onboarding` | **Date**: 2026-05-06

## Pré-requisitos

```bash
# Stack completa rodando (Spoke A + Spoke B + todos os backends)
make dev.up

# Ou subir individualmente:
make dev.up-central-bank-a dev.up-bank-a dev.up-bank-c
make dev.up-central-bank-b dev.up-bank-b dev.up-bank-d

# Verificar containers ativos
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "api-gateway|central"
```

## Validação Rápida — Todos os Bancos

```bash
# Spoke A: Bank-A onboarding completo (7 passos)
./tryouts/tryout-spoke-a-bank-a.sh

# Spoke A: Bank-C onboarding completo (7 passos)
./tryouts/tryout-spoke-a-bank-c.sh

# Spoke B: Bank-B onboarding completo (7 passos)
./tryouts/tryout-spoke-b-bank-b.sh

# Spoke B: Bank-D onboarding completo (7 passos)
./tryouts/tryout-spoke-b-bank-d.sh
```

Todos devem terminar com `PASS=7, FAIL=0`.

## Validação do my-status — Bank-A

```bash
# Verifica GET /onboarding/my-status (PASS=4, FAIL=0)
./tryouts/tryout-my-onboarding-status.sh
```

## Validação do my-status — Outros Bancos

```bash
# Bank-B (porta 28080)
BANK_URL=http://localhost:28080/api/v1 \
BANK_ENV=backend/config/.env.infra.bank-b \
./tryouts/tryout-my-onboarding-status.sh

# Bank-C (porta 48080)
BANK_URL=http://localhost:48080/api/v1 \
BANK_ENV=backend/config/.env.infra.bank-c \
./tryouts/tryout-my-onboarding-status.sh

# Bank-D (porta 58080)
BANK_URL=http://localhost:58080/api/v1 \
BANK_ENV=backend/config/.env.infra.bank-d \
./tryouts/tryout-my-onboarding-status.sh
```

## Diagnóstico de Problemas

```bash
# Confirmar que rotas estão registradas (esperado: não retornar 404)
curl -s http://localhost:18080/api/v1/onboarding/my-status
# → {"error":"..."} ou {"status":"NONE"} — nunca "Cannot GET ..."

# Checar logs do API Gateway por erros de startup
docker logs backend-api-gateway-bank-a 2>&1 | grep -E "ERROR|onboarding|cleanup"
# Após a fix: não deve aparecer "relation swap_order_scenario_b does not exist"

# Verificar TTL do nonce (30 min = 1800s)
docker logs backend-auth-bank-a 2>&1 | grep -i nonce
```

## Verificar Regressão Scenario B

```bash
# As rotas v2 devem continuar funcionando após a restauração
curl -s http://localhost:18080/healthz
# → {"status":"ok"}
```
