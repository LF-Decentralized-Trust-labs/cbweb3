# api-gateway

> [scenario-a](../../../README.md) › [backend](../../README.md) › api-gateway

The **API Gateway** is the single external entry point for all client interactions. It exposes a REST API (built with [Fiber](https://gofiber.io/)) and acts as an orchestration layer — authenticating requests, enforcing authorization, and proxying operations to the internal gRPC microservices.

---

## Architecture Placement

```
Frontend / External Client
         │
         ▼
   [api-gateway]   REST :8080
         │
         ├──► [auth]          gRPC — token validation, login, wallet ops
         ├──► [compliance]    gRPC — KYC/AML, governance participant management
         └──► [payment-orchestrator] gRPC — HTLC, FX, token transfers, escrow
```

For commercial banks, some operations are proxied to the Central Bank's own `api-gateway` endpoint (`CENTRAL_BANK_API_URL`) to request governance actions the bank cannot perform itself.

---

## Responsibilities

- **Authentication** — Delegates login and token validation to the auth service; attaches identity context to downstream calls.
- **Wallet binding** — Handles the association of a Keycloak user to an on-chain wallet address.
- **KYC routing** — Exposes KYC status lookups backed by the compliance service.
- **Governance proxy** — Forwards participant registration and governance certificate requests to the compliance service.
- **Payment routing** — Exposes HTLC lock/release, FX agreement lifecycle, Zeto transfers, and escrow operations backed by the payment orchestrator.

---

## Key Details

| Property | Value |
|----------|-------|
| Protocol | REST (HTTP/1.1) |
| Port | `8080` (per-entity offset in compose — e.g., Bank-A: 18080) |
| Framework | [Fiber v2](https://gofiber.io/) |
| Auth model | Bearer JWT validated via the auth gRPC service |

### Current Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/healthz` | — | Health check |
| `POST` | `/auth/login` | — | Authenticate via Keycloak |
| `POST` | `/auth/wallet/bind` | Bearer | Bind wallet address to user |
| `GET` | `/compliance/kyc/status/:subject` | Bearer | KYC status lookup |

### API Documentation

The service exposes its OpenAPI spec at runtime:

```bash
# YAML spec
curl http://localhost:8080/openapi.yaml

# Swagger UI
open http://localhost:8080/swagger
```

The spec file is versioned in `docs/openapi.yaml`.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `PORT` | HTTP listen port (default: 8080) |
| `AUTH_GRPC_ADDR` | auth service address (e.g., `auth:9091`) |
| `COMPLIANCE_GRPC_ADDR` | compliance service address |
| `PAYMENT_GRPC_ADDR` | payment-orchestrator address |
| `CENTRAL_BANK_API_URL` | Central bank gateway URL (commercial banks only) |
| `REQUEST_TIMEOUT_SEC` | gRPC call timeout |

---

## Run Locally

```bash
cp .env.example .env
# edit .env with local service addresses
go run ./cmd/api-gateway
```

---

## Internal Structure

```
api-gateway/
├── cmd/api-gateway/main.go    Entrypoint — loads config, starts Fiber
├── internal/
│   ├── app/app.go             App wiring, gRPC client setup
│   ├── http/
│   │   ├── router/router.go   Route registration
│   │   └── middleware/auth.go Bearer token extraction and validation
│   └── handler/               HTTP handlers per domain area
└── docs/openapi.yaml          OpenAPI specification
```

---

## Related

- [auth service](../auth/README.md) — handles identity operations this gateway calls
- [compliance service](../compliance/README.md) — handles KYC and governance operations
- [payment-orchestrator](../payment-orchestrator/README.md) — handles all payment flows
- [apis › openapi](../../../apis/README.md) — versioned OpenAPI specs
