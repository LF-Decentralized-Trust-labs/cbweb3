# compliance

> [scenario-a](../../../README.md) › [backend](../../README.md) › compliance

The **compliance** service is the governance and identity backbone of the platform. It manages on-chain participant registration, issues governance certificates, and runs KYC/AML checks for all entities on a given spoke.

---

## Architecture Placement

```
[api-gateway]
      │
      ▼
[compliance]   gRPC :9093
      │
      ├──► PostgreSQL          participant data store
      ├──► Besu (IdentityRegistry contract)   on-chain writes
      └──► PKI / CA            certificate issuance
```

The compliance service operates in two modes depending on the entity type:

- **Central Bank mode** — can write on-chain (upsert participants on the IdentityRegistry contract) and issue governance certificates.
- **Commercial Bank mode** — PKI-only; cannot write on-chain directly, submits governance requests via the Central Bank's gateway.

---

## Responsibilities

- **Participant registry** — Maintains an off-chain mirror of registered participants (PostgreSQL) and syncs writes to the on-chain `IdentityRegistry` contract.
- **KYC/AML checks** — Evaluates whether a given subject is cleared for a transaction by checking on-chain registration status and compliance flags.
- **Governance certificates** — Issues PKI-signed certificates that attest to a participant's governance role (Central Bank, Commercial Bank, etc.).
- **Role management** — Resolves RBAC roles from on-chain data for use by other services.

---

## Key Details

| Property | Value |
|----------|-------|
| Protocol | gRPC |
| Port | `9093` (per-entity offset applied in compose) |
| Language | Go |

### Key gRPC RPCs

| RPC | Description |
|-----|-------------|
| `UpsertParticipant` | Register or update a participant on-chain and in the local store |
| `GetParticipant` | Look up a participant's profile and compliance status |
| `ListParticipants` | Enumerate all registered participants for a spoke |
| `IssueGovernanceCertificate` | Issue a PKI certificate attesting to a participant's governance role |
| `CheckKYC` | Validate KYC clearance for a given subject |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `COMPLIANCE_GRPC_PORT` | gRPC listen port (default: 9093) |
| `POSTGRES_DSN` | PostgreSQL connection string |
| `BESU_RPC_URL` | Besu JSON-RPC endpoint for this spoke |
| `IDENTITY_REGISTRY_ADDRESS` | Deployed IdentityRegistry contract address |
| `ENTITY_TYPE` | `central_bank` or `commercial_bank` |
| `PKI_CA_CERT` | Path to the CA certificate for governance cert issuance |
| `PKI_CA_KEY` | Path to the CA private key |

---

## Internal Structure

```
compliance/
├── cmd/main.go            Entrypoint
├── internal/
│   ├── grpc/              gRPC server and handler implementations
│   ├── repository/        PostgreSQL participant repository
│   ├── blockchain/        Besu/IdentityRegistry client
│   └── pki/               Certificate generation utilities
└── Dockerfile
```

---

## Related

- [api-gateway](../api-gateway/README.md) — calls this service for KYC checks and governance operations
- [auth](../auth/README.md) — calls this service to resolve participant roles during token validation
- [contracts › IdentityRegistry](../../../contracts/README.md) — the on-chain registry this service writes to
