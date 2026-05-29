# Foundry Deploy & Utility Scripts

## RegisterParticipants.s.sol

**Local dev convenience script** — registers hardcoded Besu genesis addresses
as verified participants in the compliance `IdentityRegistry` contract.

### Why it exists

The on-chain `HashTimeLockedContract` gates every `lock()` call with an
`onlyVerified(msg.sender)` modifier that checks `IdentityRegistry.canTransact()`.
In production, participants are registered through the full 3-phase onboarding
flow (credential request → KYC approval → PoP), exercised by
`./tryout-spoke-a-bank-a.sh`.

This script **bypasses** that flow so the cross-spoke atomic-swap tryout
(`./tryout-htlc-cross-spoke.sh`) can run its happy path without standing up
the complete onboarding stack first.

### What it registers

| Address | Role | Key (BESU_OPERATOR_KEY) |
|---|---|---|
| `0x627306…` | CENTRAL_BANK | `c87509a1…` (CB) |
| `0xf17f52…` | COMMERCIAL_BANK | `ae6ae8e5…` (Bank-A / Bank-B) |
| `0xe4add9…` | COMMERCIAL_BANK | `5b02fc9a…` (Bank-C / Bank-D) |

The script is **idempotent** — it skips addresses that are already registered.

### When it runs automatically

| Command | Path |
|---|---|
| `make spoke-all` | `contracts.register-participants-spoke-{a,b}` |
| `make spoke-a` / `make spoke-b` | per-spoke target after `sync-addresses` |
| `make dev.up` | via `contracts.deploy-all-with-sync` |

### Running manually

```bash
# Both spokes
make contracts.register-participants

# Single spoke
make contracts.register-participants-spoke-a
make contracts.register-participants-spoke-b

# Raw forge command
ADMIN_PRIVATE_KEY=0xc87509a1… \
IDENTITY_REGISTRY=0x42699A76… \
  forge script script/RegisterParticipants.s.sol:RegisterParticipants \
  --rpc-url http://127.0.0.1:8645 --broadcast
```

### Environment variables

| Variable | Description |
|---|---|
| `ADMIN_PRIVATE_KEY` | Private key with `GOVERNANCE_ROLE` on the IdentityRegistry (CB key in local dev) |
| `IDENTITY_REGISTRY` | Deployed address of the compliance IdentityRegistry (read from `.env.infra.*` by Make) |
