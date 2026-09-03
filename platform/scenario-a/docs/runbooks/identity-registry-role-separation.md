# IdentityRegistry — Two-Step Onboarding & Role Separation (R1-10.6 / R2-10.6)

## What this control is

`IdentityRegistry` onboarding is a **two-step** flow. A single governance action must
never produce a transacting participant:

1. `registerParticipant(account, name, role, zkPointer)` — restricted to
   `GOVERNANCE_ROLE`. Creates the participant in **`Pending`**. A `Pending`
   participant is **not** whitelisted and **cannot** transact (`canTransact` /
   `canGovern` / `isWhitelisted` all return `false`).
2. `verifyParticipant(account)` — restricted to `VERIFIER_ROLE`. Promotes
   `Pending → Verified` and emits `IdentityUpdated(Pending, Verified)`. Only after
   this does the account pass the `onlyVerified` gate in
   `HashTimeLockedContract.lock` and the in-group `FXAgreement`.

`updateStatus` is split by target: promoting **to `Verified`** requires
`VERIFIER_ROLE` (mirrors `verifyParticipant`); every other transition
(`Suspended` / `Expired` freezing, or back to `Pending`) requires `GOVERNANCE_ROLE`.
This closes the bypass where a `GOVERNANCE_ROLE` holder could otherwise mint
`Verified` directly through `updateStatus`.

## Roles

| Role                 | Purpose                                | Solidity constant  |
|----------------------|----------------------------------------|--------------------|
| `DEFAULT_ADMIN_ROLE` | Grant/revoke the roles below           | OpenZeppelin default |
| `GOVERNANCE_ROLE`    | Register participants; freeze/expire   | `keccak256("GOVERNANCE_ROLE")` |
| `VERIFIER_ROLE`      | Verify (promote `Pending → Verified`)  | `keccak256("VERIFIER_ROLE")` |

**Separation of duties**: `GOVERNANCE_ROLE` (registrar) and `VERIFIER_ROLE`
(verifier) SHOULD be held by different entities in production, so the party that
onboards a participant is not the party that authorises it to transact.

## Bootstrap default (local / single-operator / pilot)

The constructor grants `DEFAULT_ADMIN_ROLE`, `GOVERNANCE_ROLE` **and**
`VERIFIER_ROLE` to the single `admin` address. This is deliberate so that:

- local dev (`scenario-a/samples`), the toolkit E2E (`setupBilateralFXAContext`
  deploys the in-group registry with the operator key as admin and drives both
  register and verify), and the seed scripts keep working with one operator key, and
- the Go compliance/auth services can complete the two-step in one flow using a
  single configured signer (`CB_PRIVATE_KEY`).

`compliance.ApproveKYC` is the on-chain onboarding point (the CB governance decision):
it now performs **register then verify** with the CB signer, so an approved bank ends
`Verified` and its payment-orchestrator passes `HashTimeLockedContract`'s
`onlyVerified` check. `auth.OnboardParticipant` / `CompleteOnboarding` and the startup
governance bootstrap do the same. With one key holding both roles this is convenient;
the on-chain control is only *materially* separated once the roles are split (below).

## Production role separation

> **Prerequisite (residual):** the Go registry client currently signs both the
> register and the verify transaction with the **same** signer. Revoking
> `VERIFIER_ROLE` from that signer will make the verify step revert and **break
> onboarding**. Before performing the split below, wire a **dedicated verifier
> signer** into the compliance/verifier service (a second key / KMS handle used
> only for `verifyParticipant` and `updateStatus→Verified`). Track this as the
> follow-up to R1-10.6 / R2-10.6 (governance-portal verify action, T-P1-16).

Once a dedicated verifier signer exists:

```bash
ADMIN=0x...            # bootstrap operator (registrar)
VERIFIER=0x...         # CEMLA / compliance authority key (verifier)
REGISTRY=0x...         # deployed IdentityRegistry (PARTICIPANT_REGISTRY_ADDRESS)
RPC=http://<besu>:8545
VERIFIER_ROLE=$(cast keccak "VERIFIER_ROLE")

# 1. Grant VERIFIER_ROLE to the compliance authority key.
cast send $REGISTRY "grantRole(bytes32,address)" $VERIFIER_ROLE $VERIFIER \
  --rpc-url $RPC --private-key $ADMIN_KEY

# 2. Revoke VERIFIER_ROLE from the bootstrap operator (now registrar-only).
cast send $REGISTRY "revokeRole(bytes32,address)" $VERIFIER_ROLE $ADMIN \
  --rpc-url $RPC --private-key $ADMIN_KEY
```

> **Pente note:** in Scenario A the registry is deployed **inside** the bilateral
> Pente privacy group. Role grants/revokes and both onboarding steps must therefore
> be executed **in-group** (via `pgroup_sendTransaction`), against the in-group
> registry address — not against a base-ledger copy.

## Deployment assertion (verify the intended posture)

```bash
GOVERNANCE_ROLE=$(cast keccak "GOVERNANCE_ROLE")
VERIFIER_ROLE=$(cast keccak "VERIFIER_ROLE")

cast call $REGISTRY "hasRole(bytes32,address)(bool)" $GOVERNANCE_ROLE $ADMIN --rpc-url $RPC     # expect true
cast call $REGISTRY "hasRole(bytes32,address)(bool)" $VERIFIER_ROLE   $ADMIN --rpc-url $RPC     # expect false (after split)
cast call $REGISTRY "hasRole(bytes32,address)(bool)" $VERIFIER_ROLE   $VERIFIER --rpc-url $RPC  # expect true
```

For **local / single-operator** the expected posture is the bootstrap default:
`ADMIN` holds all three roles and no separation is asserted.

## Scenario A note (Pente determinism)

Scenario A's `IdentityRegistry` is deployed inside Pente privacy groups, where every
group member re-executes the transaction to endorse it. Any `block.*` value written
into endorsed state diverges across endorsers and wedges the private transaction, so
`lastUpdate` is intentionally kept at `0` in `registerParticipant`,
`verifyParticipant`, `updateStatus` and `setCertFingerprint`. The audit timestamp is
taken from the emitted event's block off-chain. (Contrast Scenario B, whose registry
lives on the base ledger and writes `block.timestamp`.)

## Residual scope (tracked, not in this change)

- Dedicated verifier signer in the compliance/verifier service (prerequisite above).
- Governance Portal "verify" action + Pending queue (T-P1-16).
- Ratification of the actor model (governance-proposes / verifier-approves vs.
  participant-self-registers) in ADR-002 / T-P1-16.
