# apis

> [scenario-a](../README.md) › apis

The **apis** directory centralizes all API contracts for the platform: REST OpenAPI specifications, gRPC Protobuf definitions, and auto-generated client SDKs. It is the single source of truth for inter-service interfaces.

---

## Structure

```
apis/
├── openapi/               REST API specifications (OpenAPI 3.x YAML)
│   └── api-gateway.yaml   Read-only copy of the served spec (v2.3.0)
├── proto/                 gRPC service definitions (Protocol Buffers)
│   ├── auth/              Auth service RPCs and message types
│   ├── compliance/        Compliance service RPCs and message types
│   ├── payment_orchestrator/  Payment orchestrator RPCs and message types
│   └── buf.yaml           Buf code generation config
└── sdk/                   Auto-generated client libraries
    ├── java/
    ├── python/
    └── typescript/
```

---

## OpenAPI (REST)

**Served spec (source of truth):** [`../backend/services/api-gateway/docs/openapi.yaml`](../backend/services/api-gateway/docs/openapi.yaml) — OpenAPI 3.0.3, v2.3.0, embedded in the gateway binary and served at `GET /openapi.yaml`.

[`openapi/api-gateway.yaml`](openapi/api-gateway.yaml) is a byte-identical copy kept for consumers that read the spec from this directory; edit the served file, never this one. Between them they cover:

| Tag | Endpoints | Auth |
|-----|-----------|------|
| Health | `GET /healthz` | Public |
| Authentication | Login, refresh, logout, PKI wallet bind, `GET /auth/me` | Mixed |
| Onboarding | 3-phase PKI+Blockchain onboarding | Mixed |
| Compliance | KYC status, AML screen, participant management | Cookie + GOVERNANCE role |
| Governance | Registry, accounts, circuit breaker, parameters, audit, users | Cookie + GOVERNANCE role |
| HTLC | Lock, lock-with-hash, settle, refund, status, search | Cookie |
| Token | Mint, burn, transfer, balance | Cookie |
| Escrow | Deposit, escrow, redeem lifecycle | Cookie |
| FX Agreements | Propose, accept, reject, cancel, settle, audit trail | Cookie |
| Internal | Relay endpoints for Cacti cross-spoke bridge | `X-Relay-Auth` header |

**75 operationIds** — 58 original + 17 adicionados (FX Agreements, lock-with-hash, governance/approve-kyc, relay internal).

The spec is also **served at runtime** by the api-gateway and auto-synced from the source:

```bash
# Spec em runtime (após cd samples && ./deploy-all.sh)
curl http://localhost:18080/openapi.yaml        # bank-a
curl http://localhost:38080/openapi.yaml        # central-bank-a

# Swagger UI
open http://localhost:18080/docs

# Spec source (embedded no binário via go:embed)
scenario-a/backend/services/api-gateway/docs/openapi.yaml
```

> **Sincronização:** `openapi/api-gateway.yaml` é uma cópia do spec embarcado no serviço. Ao atualizar `backend/services/api-gateway/docs/openapi.yaml`, copie para `apis/openapi/api-gateway.yaml` para manter ambos em sincronia.

---

## Protobuf / gRPC

The `proto/` directory defines the service contracts between internal gRPC microservices. These definitions are the authoritative source for:

- [auth service](../backend/services/auth/README.md) — `proto/auth/`
- [compliance service](../backend/services/compliance/README.md) — `proto/compliance/`
- [payment-orchestrator](../backend/services/payment-orchestrator/README.md) — `proto/payment_orchestrator/`

Code generation uses [Buf](https://buf.build/):

```bash
make proto-gen       # Generate Go, TypeScript, Python, Java stubs
make proto-lint      # Lint proto files
make proto-breaking  # Check for breaking changes against main
```

Generated Go stubs are output to `backend/shared/proto/` and imported by all services.

---

## SDKs

The `sdk/` directory contains auto-generated client libraries for external consumers of the platform's REST API. Generated from the OpenAPI specs via [openapi-generator](https://github.com/OpenAPITools/openapi-generator).

| Language | Path | Status |
|----------|------|--------|
| TypeScript | `sdk/typescript/` | Generated |
| Python | `sdk/python/` | Generated |
| Java | `sdk/java/` | Generated |

---

## Related

- [api-gateway](../backend/services/api-gateway/README.md) — serves the OpenAPI spec at `/openapi.yaml`
- [auth](../backend/services/auth/README.md) — gRPC contract defined in `proto/auth/`
- [compliance](../backend/services/compliance/README.md) — gRPC contract defined in `proto/compliance/`
- [payment-orchestrator](../backend/services/payment-orchestrator/README.md) — gRPC contract defined in `proto/payment_orchestrator/`
