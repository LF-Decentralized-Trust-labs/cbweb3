# Backend

> [scenario-a](../README.md) › backend

The backend implements the service layer for Scenario A. All services are written in **Go** and follow a tiered architecture: an external REST gateway fronts a set of internal gRPC microservices. Each entity (central bank or commercial bank) runs its own isolated stack — six independent deployments of the same services, one per participant.

---

## Architecture Placement

```
Client (Frontend / API consumer)
        │
        ▼
  [api-gateway]  ←── REST (Fiber HTTP)
        │
        ├──► [auth]                gRPC 9091
        ├──► [compliance]          gRPC 9093
        └──► [payment-orchestrator] gRPC 9094
                    │
                    ├──► Paladin (ZKP privacy)
                    ├──► Besu (blockchain)
                    └──► Cacti relay (cross-spoke)
```

Monitoring runs as a separate plane:

```
[noc-agent]  ──► Docker metrics, Besu RPC
     │
     ▼
[noc-backend]  ←── NOC dashboard (REST 8000)
```

---

## Services

| Service | Protocol | Port | Status | Description |
|---------|----------|------|--------|-------------|
| [api-gateway](services/api-gateway/README.md) | REST | 8080 | Implemented | HTTP entry point; routes to gRPC backends |
| [auth](services/auth/README.md) | gRPC | 9091 | Implemented | Identity, login, wallet, PKI |
| [compliance](services/compliance/README.md) | gRPC | 9093 | Implemented | Onboarding/AML, participant registry, governance |
| [payment-orchestrator](services/payment-orchestrator/README.md) | gRPC | 9094 | Implemented | HTLC, FX, Zeto transfers, escrow |
| [noc-agent](services/noc-agent/README.md) | HTTP push | — | Implemented | Monitoring agent (Docker, Besu metrics) |
| [noc-backend](services/noc-backend/README.md) | REST | 8000 | Implemented | NOC dashboard and telemetry store |
| [fx](services/fx/README.md) | gRPC | — | Planned | FX pricing service |
| [ledger-gateway](services/ledger-gateway/README.md) | gRPC | — | Planned | Blockchain RPC/WS abstraction |
| [payments](services/payments/README.md) | gRPC | — | Planned | Payment domain logic |

---

## Shared Code

```
backend/
├── services/          Individual service directories
└── shared/            Shared Go libraries
    ├── blockchain/    Besu client, contract bindings
    ├── identity/      PKI, certificate utilities
    └── proto/         Generated gRPC stubs (from apis/proto/)
```

`shared/` is imported as a local Go module by all services — it centralises Besu interactions, PKI operations, and the generated protobuf types.

---

## Configuration

Each entity's service instances are configured via environment files in `backend/config/`. Keys include:

| Variable | Used By | Description |
|----------|---------|-------------|
| `AUTH_GRPC_ADDR` | api-gateway | Address of the auth service |
| `COMPLIANCE_GRPC_ADDR` | api-gateway | Address of the compliance service |
| `PAYMENT_GRPC_ADDR` | api-gateway | Address of the payment orchestrator |
| `KEYCLOAK_URL` | auth | Keycloak base URL for this entity's realm |
| `POSTGRES_DSN` | compliance, payment-orchestrator, noc-backend | Database connection string |
| `REDIS_ADDR` | auth | Redis address for nonce management |
| `BESU_RPC_URL` | auth, compliance, payment-orchestrator | Besu JSON-RPC endpoint |
| `PALADIN_URL` | payment-orchestrator | Paladin node API |
