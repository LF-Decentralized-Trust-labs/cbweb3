# payment-orchestrator

> [scenario-a](../../../README.md) › [backend](../../README.md) › payment-orchestrator

The **payment-orchestrator** is the core payment engine of the platform. It coordinates all payment flows: HTLC-based cross-spoke atomic swaps, FX agreement lifecycle management, privacy-preserving Zeto token transfers via Paladin, and escrow deposit/release operations.

---

## Architecture Placement

```
[api-gateway]
      │
      ▼
[payment-orchestrator]   gRPC :9094
      │
      ├──► Paladin / Zeto         privacy-preserving token transfers (ZKP)
      ├──► Besu (HTLC contract)   on-chain HTLC state management
      ├──► Besu (FXAgreement)     FX agreement settlement
      ├──► Cacti relay            cross-spoke interop (secret bridging)
      ├──► PostgreSQL             FX agreement and escrow persistence
      └──► [compliance]  gRPC     participant and role validation
```

---

## Responsibilities

- **HTLC orchestration** — Creates and manages Hash Time-Locked Contracts for atomic cross-spoke swaps. Handles the full lifecycle: lock → (settle or refund).
- **FX agreement management** — Implements the propose → accept/reject/cancel → settle lifecycle for bilateral FX trades between counterparties on different spokes.
- **Zeto transfers** — Executes privacy-preserving tCeBM token transfers via Paladin's Zeto ZKP domain, so transaction amounts remain confidential.
- **Fiat transfers** — Handles standard (non-private) tCeBM ERC-20 transfers on-chain.
- **Escrow** — Manages deposit and release of funds in escrow accounts tied to active FX agreements or settlement workflows.
- **Timeout workers** — Background workers that automatically refund expired HTLCs and cancel timed-out FX agreements.

---

## Key Details

| Property | Value |
|----------|-------|
| Protocol | gRPC |
| Port | `9094` (per-entity offset applied in compose) |
| Language | Go |

### Key gRPC RPCs

| RPC | Description |
|-----|-------------|
| `LockHTLC` | Create an HTLC on the spoke's HTLC contract |
| `ReleaseHTLC` | Reveal the preimage and settle an HTLC |
| `RefundHTLC` | Refund an expired HTLC |
| `InitiateFXAgreement` | Propose an FX swap to a counterparty |
| `AcceptFXAgreement` | Accept an incoming FX proposal |
| `SettleFXAgreement` | Finalize settlement of an accepted FX trade |
| `TransferZeto` | Send tCeBM via Paladin/Zeto (private transfer) |
| `TransferFiat` | Send tCeBM via standard ERC-20 transfer |
| `EscrowDeposit` | Lock funds in escrow for a pending settlement |
| `EscrowRelease` | Release escrowed funds on settlement |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `PAYMENT_GRPC_PORT` | gRPC listen port (default: 9094) |
| `BESU_RPC_URL` | Besu JSON-RPC endpoint |
| `HTLC_CONTRACT_ADDRESS` | Deployed HashTimeLockedContract address |
| `FX_AGREEMENT_ADDRESS` | Deployed FXAgreement contract address |
| `PALADIN_URL` | Paladin node API endpoint |
| `CACTI_URL` | Cacti relay endpoint for cross-spoke secret bridging |
| `POSTGRES_DSN` | PostgreSQL DSN for agreement and escrow persistence |
| `COMPLIANCE_GRPC_ADDR` | Compliance service address for participant checks |

---

## Cross-Spoke HTLC Flow

```
Spoke-A (Initiator)                    Spoke-B (Responder)
        │                                      │
  LockHTLC(hashLock)                           │
        │                                      │
        │◄── Cacti relay broadcasts hashLock ──►│
        │                                 LockHTLC(same hashLock)
        │                                      │
  ReleaseHTLC(secret)                          │
        │                                      │
        │◄── relay detects secret, bridges ───►│
        │                              ReleaseHTLC(secret) [auto]
```

---

## Internal Structure

```
payment-orchestrator/
├── cmd/main.go            Entrypoint
├── internal/
│   ├── grpc/              gRPC server and handlers
│   ├── htlc/              HTLC orchestration logic
│   ├── fx/                FX agreement workflows
│   ├── zeto/              Paladin/Zeto client integration
│   ├── escrow/            Escrow management
│   ├── worker/            Timeout/background workers
│   └── repository/        PostgreSQL repositories
└── Dockerfile
```

---

## Related

- [api-gateway](../api-gateway/README.md) — external entry point that triggers payment operations
- [contracts › HashTimeLockedContract](../../../contracts/README.md) — on-chain HTLC this service manages
- [contracts › FXAgreement](../../../contracts/README.md) — FX settlement contract
- [interop › Cacti relay](../../../interop/README.md) — cross-spoke secret bridging
