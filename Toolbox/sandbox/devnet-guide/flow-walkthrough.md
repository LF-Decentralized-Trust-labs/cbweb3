# PvP Settlement Flow Walkthrough

A step-by-step explanation of the Payment-versus-Payment (PvP) cross-border settlement using Hash Time-Lock Contracts (HTLC).

> This walkthrough describes the flow that the Toolbox artifacts cover: contracts, mocks, test vectors, and conformance tests.
>
> **URL convention:** Endpoints are shown as defined in the OpenAPI spec (e.g., `POST /fx/agreement`). The server base path `/api/v1` is omitted. When calling a real implementation, prepend the base path to each endpoint.

---

## Overview

PvP settlement ensures that **both legs of a cross-border FX transaction settle atomically** — either both succeed, or both are automatically reversed. This eliminates settlement risk.

### Participants (synthetic test data)

| Role | Entity | Currency | Address |
|------|--------|----------|---------|
| **Initiator** | Central Bank A | tCeBM-A | `0xCB001_CountryA` |
| **Counterparty** | Central Bank B | tCeBM-B | `0xCB002_CountryB` |

### Trade terms (synthetic)
- Country A sends: **1,000,000 tCeBM-A**
- Country B sends: **58,000 tCeBM-B**
- Exchange rate: **0.058**

---

## The 6-step flow

```
  Central Bank              Central Bank
  Country A                Country B
  (Initiator)               (Counterparty)
       │                          │
       │  Step 1: Propose FX      │
       │  POST /fx/agreement      │
       ├─────────────────────────>│
       │                          │
       │  Step 2: Accept FX       │
       │  POST /fx/agreement/     │
       │        {id}/accept       │
       │<─────────────────────────┤
       │                          │
       │  Step 3: Lock A           │
       │  POST /htlc/lock         │
       │  (hashLock + timeLock)   │
       ├──────┐                   │
       │      │ Locked on         │
       │      │ Spoke A           │
       │<─────┘                   │
       │                          │
       │  Step 4: Lock B           │
       │  POST /htlc/lock         │
       │  (SAME hashLock)         │
       │                   ┌──────┤
       │                   │      │
       │                   │      │
       │                   └─────>│
       │                          │
       │  Step 5: Settle B        │
       │  POST /htlc/settle       │
       │  (reveal secret)         │
       ├─────────────────────────>│
       │         Secret is now    │
       │         visible on-chain │
       │                          │
       │  Step 6: Settle A        │
       │  POST /htlc/settle       │
       │  (use revealed secret)   │
       │<─────────────────────────┤
       │                          │
       │    ✅ Settlement complete │
       │                          │
```

---

## Step-by-step detail

### Step 1: Create FX Agreement

**Who:** Country A (initiator)
**Endpoint:** `POST /fx/agreement`
**What happens:** Country A proposes a trade: "I will send 1,000,000 tCeBM-A in exchange for tCeBM-B at rate 0.058."

The system creates an agreement with status **PROPOSED**.

**Mock file:** `mocks/pvp/happy-path/01_create_fx_agreement.json`

---

### Step 2: Accept FX Agreement

**Who:** Country B (counterparty)
**Endpoint:** `POST /fx/agreement/{agreementId}/accept`
**What happens:** Country B reviews and accepts the proposed terms.

The agreement transitions to **READY_FOR_SETTLEMENT**. Both parties have now committed to the trade.

**Mock file:** `mocks/pvp/happy-path/02_accept_fx_agreement.json`

---

### Step 3: Lock A (initiator)

**Who:** Country A (initiator)
**Endpoint:** `POST /htlc/lock`
**What happens:** Country A generates a **secret** (`cbweb3-test-secret-2026`) and computes its **SHA-256 hash** (the `hashLock`). It then locks 1,000,000 tCeBM-A in an HTLC smart contract with:
- `hashLock`: SHA-256 of the secret (publicly visible)
- `timeLock`: Unix timestamp when the lock expires

The funds are now **escrowed** — Country A cannot spend them, but no one can claim them yet either.

**Key concept:** The initiator knows the secret. The counterparty only sees the hash.

**Mock file:** `mocks/pvp/happy-path/03_htlc_lock_initiator.json`

---

### Step 4: Lock B (counterparty)

**Who:** Country B (counterparty)
**Endpoint:** `POST /htlc/lock`
**What happens:** After verifying (via the interoperability layer) that Country A has locked funds, Country B locks 58,000 tCeBM-B using the **same hashLock**.

Now both sides have funds locked. The stage is set for atomic settlement.

**Important:** The counterparty's `timeLock` is set slightly earlier than the initiator's, ensuring the initiator has time to settle after revealing the secret.

**Mock file:** `mocks/pvp/happy-path/04_htlc_lock_counterparty.json`

---

### Step 5: Settle B (initiator reveals secret)

**Who:** Country A (initiator)
**Endpoint:** `POST /htlc/settle`
**What happens:** Country A submits the plain-text secret to claim the 58,000 tCeBM-B locked by Country B.

The smart contract verifies: `SHA-256(secret) == hashLock`. If it matches, the funds are released to Country A.

**Critical moment:** The secret is now **revealed on-chain**. Anyone can read it.

**Mock file:** `mocks/pvp/happy-path/05_htlc_settle_counterparty.json`

---

### Step 6: Settle A (counterparty uses revealed secret)

**Who:** Country B (counterparty)
**Endpoint:** `POST /htlc/settle`
**What happens:** Country B reads the revealed secret from the blockchain and uses it to claim the 1,000,000 tCeBM-A locked by Country A.

**Settlement is complete.** Both parties have received their funds atomically.

**Mock file:** `mocks/pvp/happy-path/06_htlc_settle_initiator.json`

---

## What if something goes wrong?

### Timeout / Refund path

If the initiator does **not** reveal the secret before the `timeLock` expires:

1. **Settlement attempt fails** → `POST /htlc/settle` returns `410 Gone` (HTLC expired)
2. **Refund** → `POST /htlc/refund` returns the locked funds to the original sender

Both locks expire, and both parties get their funds back. No one loses money.

**Mock files:** `mocks/pvp/timeout-refund/`

### Error cases covered by test vectors

| Scenario | Expected result | Test vector |
|----------|----------------|-------------|
| Invalid secret (hash mismatch) | `400` HTLC_HASH_MISMATCH | `htlc-err-01` |
| Settle after timeout | `410` HTLC_EXPIRED | `htlc-err-02` |
| Refund before timeout | `409` HTLC_NOT_EXPIRED | `htlc-err-03` |
| Settle already-settled HTLC | `409` HTLC_ALREADY_SETTLED | `htlc-err-04` |
| Lock without accepted agreement | `400` AGREEMENT_NOT_ACCEPTED | `htlc-edge-01` |

---

## Cryptographic details

| Field | Value |
|-------|-------|
| **Secret** | `cbweb3-test-secret-2026` |
| **Hash algorithm** | SHA-256 |
| **hashLock** | `0x7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069` |

Implementations can verify: `SHA-256("cbweb3-test-secret-2026") == 0x7f83b165...`

---

## Next steps

- **Try it yourself:** [Tutorial 1: PvP Settlement against Mock Server](../tutorials/01-pvp-settlement-mock.md)
- **Understand the rules:** [Conformance Requirements](../../conformance/spec/conformance_requirements.md)
- **See the full contract:** [PvP OpenAPI Spec](../../contracts/pvp/openapi_pvp_v0.1.0.yaml)
