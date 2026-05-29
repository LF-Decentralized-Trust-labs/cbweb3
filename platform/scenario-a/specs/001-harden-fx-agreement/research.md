# Research: Harden FX Agreement for Production

## Decision 1: Persist FX Agreements and Audit Trail in PostgreSQL

**Decision**: Implement `FXAgreementRepository` in payment-orchestrator backed by PostgreSQL, with two canonical tables: `fx_agreements` (current state) and `fx_agreement_events` (append-only transitions).

**Rationale**:
- Current state is in-memory (`map`) and is lost on restart.
- Regulatory and operational reconciliation requires immutable event history by `trade_id`.
- The codebase already uses GORM + PostgreSQL in compliance service, reducing delivery risk.
- Multi-instance deployment requires shared durable state.

**Alternatives considered**:
- Keep in-memory map + periodic snapshots: rejected due to crash windows and weak audit guarantees.
- Event-sourcing only in queue/broker: rejected for added infra and complexity for current scope.
- Public-chain only persistence: rejected because full FX terms are bilateral and privacy-sensitive.

## Decision 2: Harden Cross-Spoke Relay with Persistent Dedup and Retry

**Decision**: Add durable relay state (processed keys + pending retries), exponential backoff retry, and full lifecycle propagation (`PROPOSED`, `ACCEPTED`, `REJECTED`, `CANCELLED`, `SETTLED`).

**Rationale**:
- Current dedup uses in-memory `Map/Set` and loses state on restart.
- Current forwarding is fire-and-forget with no durable retry path.
- Full lifecycle parity is required to avoid spoke divergence in terminal states.
- Internal endpoint must move from implicit network trust to service authentication.

**Alternatives considered**:
- Keep polling without persistence and rely on idempotent handlers: rejected due to event loss risk.
- Replace immediately with full event bus architecture: rejected as larger migration than required for this phase.
- Keep `/internal` open in trusted network only: rejected for production shared-infrastructure threat model.

## Decision 3: Bilateral FX Agreement Lifecycle in Pente (Private Context)

**Decision**: Deploy `FXAgreement.sol` contract to Pente private contexts (one per bilateral agreement pair) instead of public Besu. Maintain shared `IdentityRegistry` and `CommitmentHashRegistry` on public Besu for identity verification and HTLC gating.

**Architecture**:
```
Public Besu (Shared):
  ├─ IdentityRegistry — Participant verification (used by service layer)
  ├─ HashTimeLockedContract — HTLC coordination
  └─ CommitmentHashRegistry — FX commitment hash registry for gates

Private Pente (Per Bilateral Context):
  └─ FXAgreement — Bilateral agreement terms (only group members visible)
```

**Rationale**:
- Bilateral FX terms must not be exposed on public Besu state (confidentiality requirement).
- Pente native privacy means only counterparties see agreement details.
- Eliminates on-chain data exposure even with anonymization.
- Enables future atomicity via Pente `externalCalls` to couple accept() → lock() in single transaction.

**Alternatives considered**:
- Deploy `FXAgreement.sol` on public Besu: rejected — bilateral terms exposed to all network participants.
- Keep service-only bilateral record: rejected — weaker mutual attestation without on-chain record.
- Commitment-only without private contract: initially considered, but less future-proof than full contract deployment.

**Implementation Status**:
- ✅ CommitmentHashRegistry.sol deployed to public Besu for HTLC fallback gating
- ✅ Service-layer integration with PenteClient for bilateral context creation
- ⏳ FXAgreement.sol deployment to Pente via Paladin HTTP API (manifest-based)

## Decision 4: Dual-Gate HTLC Enforcement Strategy

**Decision**: Implement defense-in-depth enforcement: 
1. **Primary Gate**: Service-layer validates FX agreement state from database
2. **Secondary Gate**: On-chain fallback via CommitmentHashRegistry for when Pente contract unavailable or as additional validation layer
3. **Future Path (Phase B)**: Pente `externalCalls` atomic coupling when Pente infrastructure fully ready

**Enforcement Flow**:
```
HTLC.lock() called:
  ├─ Check 1: Service layer validates agreement state (ACCEPTED, not expired)
  ├─ Check 2: On-chain FXAgreement.accept() state (if Pente contract callable)
  └─ Check 3: CommitmentHashRegistry.isAccepted() (if Pente unavailable)
     → All gates must pass, fail-closed on any rejection
```

**Rationale**:
- Service-layer gate provides immediate enforcement without on-chain latency
- On-chain fallback ensures verifiability even if service is compromised
- CommitmentHashRegistry (lightweight) reduces gas costs vs full on-chain contract state
- Preserves option for future Pente externalCalls atomicity without redesign

**Alternatives considered**:
- Service-layer-only: rejected — residual bypass risk if service compromised
- On-chain-only: rejected — introduces latency and gas costs for every agreement
- Immediate full externalCalls: rejected — requires Pente infrastructure readiness not yet confirmed

## Decision 5: Automatic Expiry and Operational Recovery Jobs

**Decision**: Implement a periodic expiration worker that transitions expired non-terminal agreements and emits corresponding audit events; add operational metrics for lag and retry depth.

**Rationale**:
- Expiry is modeled but not fully enforced over time without active requests.
- Zombie agreements create bilateral divergence and operator confusion.

**Alternatives considered**:
- Manual operator cleanup: rejected due to operational risk and inconsistency.
- Expiry only at HTLC lock time: rejected because stale proposals remain open indefinitely.

## Confirmed Impacts

**Backend Services**:
- `backend/services/payment-orchestrator/internal/grpc/server/server.go` — FX agreement lifecycle, Pente integration
- `backend/services/payment-orchestrator/internal/domain/fx.go` — Domain types for agreements
- `backend/services/payment-orchestrator/internal/ports/` — FX repository and Pente client ports
- `backend/services/payment-orchestrator/internal/repository/` — PostgreSQL persistence
- `backend/services/payment-orchestrator/cmd/payment-orchestrator/main.go` — Pente client initialization
- `backend/services/api-gateway/internal/http/router/router.go` — REST endpoint integration
- `interop/hub-and-spoke/cacti/src/htlc-relay.ts` — Cross-spoke event relay

**Smart Contracts (Public Besu)**:
- `contracts/src/IdentityRegistry.sol` — Unchanged (shared participant verification)
- `contracts/src/HashTimeLockedContract.sol` — Updated to use CommitmentHashRegistry
- `contracts/src/CommitmentHashRegistry.sol` — NEW: Commitment hash storage for HTLC fallback gating
- `contracts/script/HashTimeLockedContract.s.sol` — Updated deploy script with COMMITMENT_HASH_REGISTRY_ADDRESS
- `contracts/script/CommitmentHashRegistry.s.sol` — NEW: Deploy script

**Smart Contracts (Private Pente)**:
- `contracts/src/FXAgreement.sol` — Will be deployed to Pente contexts (not public Besu)
- `deploy/local/paladin/contracts/` — NEW: Pente FXAgreement deployment manifests (YAML-based)

**APIs & Protobuf**:
- `apis/proto/payment_orchestrator/v1/payment_orchestrator.proto` — FX agreement RPC methods

## Migration Risks and Preconditions

**Deployment Order** (Critical):
1. IdentityRegistry (public Besu)
2. CommitmentHashRegistry (public Besu)
3. HashTimeLockedContract (public Besu) — must reference CommitmentHashRegistry_ADDRESS
4. FXAgreement (private Pente contexts) — deployed via Paladin API

**Schema & Database**:
- PostgreSQL schema rollout must be in place before switching writes from memory to DB
- Idempotency keys for relay forwarding must be durable and unique per transition
- Bilateral context references (GroupID, ContractAddress) must be persisted

**Pente Infrastructure**:
- Pente integration requires environment readiness (`PENTE_ENABLED=true`)
- Identity mapping across spokes must be configured before bilateral context creation
- Paladin HTTP API must be accessible for context/contract deployment requests
- Private group initialization must succeed before service can proceed

**Service-Layer Enforcement**:
- Service-layer agreement validation must be operational before HTLC locks are accepted
- Feature flag rollout required to enable/disable strict FX enforcement without production lockout
- Feature-flagged rollout is required to avoid production lockout during cutover.
