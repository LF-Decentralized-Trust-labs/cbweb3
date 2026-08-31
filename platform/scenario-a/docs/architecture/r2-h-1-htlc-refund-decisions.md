# R2-H-1 — HTLC Refund Hardening: Decisions & Follow-ups

Status: Accepted · Date: 2026-07-31 · Scope: Scenario A (Scenario B HTLC is unwired)

Companion record to the R2-H-1 fix ("Restrict HTLC refund to the Original Sender").
It documents the two operational/design trade-offs surfaced in code review that were
deliberately **not** coded into this PR, and one follow-up filed as separate work.

## Context

`HashTimeLockedContract.refund()` now requires `msg.sender == lockDetails.sender`
(`HTLC__NotSender`). The on-chain HTLC is a public coordination layer mirrored by a
private Zeto lock/unlock on the Paladin sidecar; the gate keeps the two in sync by
ensuring only the party that created the lock (and therefore controls the private
leg) can drive the public refund. Off-chain, the refund/settle REST routes are gated
by a payment-operator role plus a counterparty bank-ownership check, and the
payment-orchestrator re-checks counterparty membership server-side.

## Decision 1 — Key rotation with open locks is an operational constraint (accepted risk)

The orchestrator signs refunds with `BESU_OPERATOR_KEY` (falling back to
`CB_PRIVATE_KEY`). Because `refund()` is sender-only, rotating that key while LOCKED
records still exist leaves those on-chain records unrefundable by the new key: the
new operator address is no longer `lockDetails.sender`.

**Decision.** We accept this as an operational constraint rather than add an
on-chain governance override. A `canGovern()`-gated refund would reintroduce exactly
the public/private desync R2-H-1 exists to prevent (governance could move the public
leg without holding or coordinating the private Zeto leg), and would add a privileged
path requiring its own Zeto-side coordination and audit controls.

**Operational rule.** Do not rotate `BESU_OPERATOR_KEY` / `CB_PRIVATE_KEY` on a spoke
while HTLC records are in `LOCKED`/`REFUNDING`. Drain (settle or refund) open locks
first, or retain the prior key until they reach a terminal state. This applies to
both local (samples/toolkit) and multi-infra (deploy-lnet) deployments.

> Note: the orchestrator's refund flow marks a record REFUNDED once the private Zeto
> `TransferLocked` succeeds even if the on-chain `refund()` reverts (it is logged, not
> fatal). A stranded key therefore does not perpetually block the *record* from
> reaching a terminal state, but it does leave the *public* leg LOCKED on-chain — the
> desync this constraint avoids.

## Decision 2 — `settle()` caller restriction is a separate finding

`HashTimeLockedContract.settle(contractId, secret)` has **no** caller restriction:
any address that observes the revealed secret can mark the public leg `SETTLED` while
the private Zeto leg has not moved — the same desync class as R2-H-1.

Sender-only is the **wrong** control here: in the canonical flow the relay settles the
counterpart leg through the counterparty's orchestrator, so the legitimate on-chain
caller is the receiver (or the relay), not the sender. A correct fix needs a
**receiver-or-relay allowlist**, which requires wiring the relay signer address into
the contract and is out of scope for R2-H-1.

**Decision.** File separately (tracked as a distinct HIGH/MEDIUM item, e.g. "restrict
HTLC settle to receiver-or-relay"). Off-chain, `settle` is already gated by the same
payment-operator role + counterparty check as refund, and the orchestrator re-checks
server-side (`checkHTLCCounterparty`); the residual exposure is a direct on-chain
`settle()` call bypassing the orchestrator.

## What this PR does cover

- On-chain sender-only `refund()` gate in both scenarios (+ negative test).
- Interface natspec documenting the precondition and `HTLC__NotSender`.
- Orchestrator ABI now declares the HTLC custom errors and decodes revert selectors,
  so a rejected refund/settle surfaces its named cause to operators (Principle VI).
- Payment-operator role gate on the mutating HTLC REST routes
  (`ROLE_BANK` / `ROLE_COMMERCIAL_BANK` / `ROLE_TREASURY` / `ROLE_GOVERNANCE`),
  excluding oversight-only sessions (supervisor / NOC) and role-less sessions.
- gRPC-level `PermissionDenied` authz tests for `RefundHTLC` and `SettleHTLC`, and a
  router-level test for the role gate.
