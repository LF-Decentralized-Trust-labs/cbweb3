# CBWeb3 Scenario A — Deposit, Escrow & Redeem Flow

> Lifecycle of tokenized central bank money: Fiat → public ERC-20 (FiatCentralBankMoney)
> → private ZKP token (Zeto tCeBM) → back to public ERC-20.
>
> This is the **liquidity injection** process that enables commercial banks to
> hold private tCeBM for use in PvP settlement (see [transfer.md](transfer.md)).

---

## Sequence Diagram

```mermaid
sequenceDiagram
    actor CBBank as Commercial Bank
    actor CBGov as Central Bank (Governance)
    participant BankGW as API Gateway<br/>(Bank Node)<br/>PaymentProxyHandler
    participant CBGW as API Gateway<br/>(CB Node)<br/>PaymentHandler
    participant PayOrch as Payment Orchestrator<br/>(gRPC Service)
    participant DB as PostgreSQL<br/>(off-chain state)
    participant FiatContract as FiatCentralBankMoney<br/>(Tokenized FIAT Money — ERC-20 / Besu)
    participant PaladinCB as Paladin Sidecar<br/>(CB Node — JSON-RPC)
    participant ZetoContract as Zeto_Anon<br/>(Smart Contract / Besu)
    participant PaladinBank as Paladin Sidecar<br/>(Bank Node — JSON-RPC)

    rect rgb(220, 235, 255)
        Note over CBBank, FiatContract: PHASE 1 — Deposit: Fiat → Tokenized FIAT Money (ERC-20, public Besu)

        CBBank->>BankGW: POST /api/v1/payments/deposits<br/>{ besu_address, amount }
        BankGW->>CBGW: POST /internal/v1/payments/deposits (proxy, Docker-internal)
        CBGW->>PayOrch: gRPC RegisterDeposit(requester_besu_address, amount)
        PayOrch->>DB: INSERT DepositRecord { status: PENDING }
        PayOrch-->>CBGW: deposit_id
        CBGW-->>BankGW: 201 { deposit_id }
        BankGW-->>CBBank: 201 { deposit_id }

        CBGov->>CBGW: POST /api/v1/payments/deposits/approve { deposit_id }
        Note right of CBGov: Requires ROLE_GOVERNANCE
        CBGW->>PayOrch: gRPC ApproveDeposit(deposit_id)
        PayOrch->>DB: UPDATE DepositRecord { status: APPROVED }

        CBGov->>CBGW: POST /api/v1/payments/deposits/fiat-exchange { deposit_id }
        CBGW->>PayOrch: gRPC RequestFiatExchange(deposit_id)
        Note over PayOrch: Guards: status == APPROVED<br/>AND mint_tx_hash == "" (idempotent)
        PayOrch->>FiatContract: Tokenized FIAT Money.mint(requester_besu_address, amount)
        FiatContract-->>PayOrch: mint_tx_hash
        PayOrch->>DB: UPDATE DepositRecord { mint_tx_hash }
        PayOrch-->>CBGov: 201 { mint_tx_hash }
        Note right of FiatContract: Bank now holds Tokenized FIAT Money<br/>(public ERC-20 on Besu)
    end

    rect rgb(220, 255, 220)
        Note over CBBank, ZetoContract: PHASE 2 — Escrow: Tokenized FIAT Money (public) → tCeBM/Zeto (private, ZK)

        CBBank->>BankGW: POST /api/v1/payments/escrows<br/>{ besu_address, paladin_identity, amount }
        BankGW->>CBGW: POST /internal/v1/payments/escrows (proxy)
        CBGW->>PayOrch: gRPC RequestEscrow(besu_address, paladin_identity, amount)
        PayOrch->>DB: INSERT EscrowRecord { status: PENDING }
        PayOrch-->>CBBank: escrow_id

        CBGov->>CBGW: POST /api/v1/payments/escrows/approve { escrow_id }
        Note right of CBGov: Requires ROLE_GOVERNANCE
        CBGW->>PayOrch: gRPC ApproveEscrow(escrow_id)

        Note over PayOrch, FiatContract: Step 2.1 — Destroy Tokenized FIAT Money on public chain
        PayOrch->>FiatContract: Tokenized FIAT Money.burn(requester_besu_address, amount)
        FiatContract-->>PayOrch: burn_tx_hash
        Note right of FiatContract: ERC-20 tokens destroyed<br/>(visible on-chain)

        Note over PayOrch, ZetoContract: Step 2.2 — Mint private tCeBM via Paladin (Zeto domain)
        PayOrch->>PaladinCB: ptx_sendTransaction {<br/>  type: "private",<br/>  domain: "zeto",<br/>  function: "mint",<br/>  to: bank_paladin_identity,<br/>  amount<br/>}
        Note over PaladinCB: Paladin resolves Paladin identity → ETH address<br/>via ptx_resolveVerifier(identity, "ecdsa:secp256k1")
        Note over PaladinCB: Generates ZK proof (AnonNullifier)<br/>for the new private UTXO state
        PaladinCB->>ZetoContract: submitTransaction(ZK proof + encrypted state commitment)
        ZetoContract-->>PaladinCB: on-chain tx confirmed
        PaladinCB->>PaladinCB: ptx_getTransactionFull(txID)<br/>(polls every 3s, timeout 120s)
        PaladinCB-->>PayOrch: mint_tx_hash (ptx receipt)

        PayOrch->>DB: UPDATE EscrowRecord {<br/>  status: APPROVED,<br/>  burn_tx_hash,<br/>  mint_tx_hash<br/>}
        PayOrch-->>CBGov: 200 { burn_tx_hash, mint_tx_hash }
        Note right of ZetoContract: Bank identity now holds<br/>private Zeto UTXO states<br/>(hidden from on-chain observers)

        alt Escrow Rejected
            CBGov->>CBGW: POST /api/v1/payments/escrows/reject { escrow_id, reason }
            CBGW->>PayOrch: gRPC RejectEscrow(escrow_id)
            PayOrch->>DB: UPDATE EscrowRecord { status: REJECTED, reason }
            Note over PayOrch: Tokenized FIAT Money NOT burned yet → no compensation needed
        end
    end

    rect rgb(255, 245, 210)
        Note over CBBank, FiatContract: PHASE 3 — Redeem: tCeBM/Zeto (private) → Tokenized FIAT Money (public)

        Note over CBBank, PaladinBank: Pre-step: Bank transfers Zeto tokens to CB's Paladin identity
        CBBank->>PaladinBank: ptx_sendTransaction {<br/>  type: "private",<br/>  domain: "zeto",<br/>  function: "transfer",<br/>  to: CB_paladin_identity,<br/>  amount<br/>}
        Note over PaladinBank: Generates ZK proof — sender identity<br/>is private / hidden on-chain
        PaladinBank->>ZetoContract: submitTransaction(ZK proof — nullifies bank UTXOs,<br/>creates new UTXO for CB identity)
        ZetoContract-->>PaladinBank: zeto_transfer_tx_hash
        PaladinBank-->>CBBank: zeto_transfer_tx_hash

        CBBank->>BankGW: POST /api/v1/payments/redeems {<br/>  besu_address, paladin_identity,<br/>  amount, zeto_transfer_tx_hash<br/>}
        BankGW->>CBGW: POST /internal/v1/payments/redeems (proxy)
        CBGW->>PayOrch: gRPC RequestRedeem(...)
        PayOrch->>DB: INSERT RedeemRecord {<br/>  status: PENDING,<br/>  zeto_transfer_tx_hash<br/>}
        PayOrch-->>CBBank: redeem_id

        CBGov->>CBGW: POST /api/v1/payments/redeems/approve { redeem_id }
        Note right of CBGov: CB verifies the zeto_transfer_tx_hash<br/>before approving
        CBGW->>PayOrch: gRPC ApproveRedeem(redeem_id)
        PayOrch->>FiatContract: Tokenized FIAT Money.mint(requester_besu_address, amount)
        FiatContract-->>PayOrch: fiat_mint_tx_hash
        PayOrch->>DB: UPDATE RedeemRecord {<br/>  status: APPROVED,<br/>  fiat_mint_tx_hash<br/>}
        PayOrch-->>CBGov: 200 { fiat_mint_tx_hash }
        Note right of FiatContract: Tokenized FIAT Money re-minted publicly<br/>Bank is back on public chain

        alt Redeem Rejected
            CBGov->>CBGW: POST /api/v1/payments/redeems/reject { redeem_id, reason }
            CBGW->>PayOrch: gRPC RejectRedeem(redeem_id)
            PayOrch->>PaladinCB: ptx_sendTransaction {<br/>  function: "transfer",<br/>  to: bank_paladin_identity,<br/>  amount<br/>}
            Note over PaladinCB: Returns Zeto tokens to bank<br/>(private ZK transfer from CB identity)
            PaladinCB->>ZetoContract: submitTransaction(ZK proof)
            PayOrch->>DB: UPDATE RedeemRecord { status: REJECTED, reason }
        end
    end
```

---

## Phase Summary

| Phase | Operation | Input | Output | Actor |
|-------|-----------|-------|--------|-------|
| 1 — Deposit | Fiat → FiatCentralBankMoney (ERC-20) | Bank fiat deposit request | `mint_tx_hash` | Central Bank (governance) |
| 2 — Escrow | FiatCentralBankMoney → tCeBM (Zeto) | Escrow request + approval | `burn_tx_hash` + `mint_tx_hash` (ZKP) | Central Bank (governance) |
| 3 — Redeem | tCeBM (Zeto) → FiatCentralBankMoney | Redeem request + ZK transfer proof | `fiat_mint_tx_hash` | Central Bank (governance) |

## Key Design Properties

| Property | Description |
|----------|-------------|
| **Governance-gated** | All state transitions require Central Bank approval (ROLE_GOVERNANCE) |
| **Idempotent minting** | Guards prevent double-mint: `mint_tx_hash == ""` check before execution |
| **Privacy boundary** | FiatCentralBankMoney is public (ERC-20); tCeBM is private (Zeto ZKP UTXO) |
| **Rejection safety** | Escrow rejection before burn = no compensation; Redeem rejection = Zeto tokens returned via ZK transfer |
| **Audit trail** | All tx hashes (fiat mint, burn, zeto mint) stored in PostgreSQL per record |

## API Endpoints

| Method | Endpoint | Actor | Phase |
|--------|----------|-------|-------|
| POST | `/api/v1/payments/deposits` | Commercial Bank | 1 |
| POST | `/api/v1/payments/deposits/approve` | Central Bank | 1 |
| POST | `/api/v1/payments/deposits/fiat-exchange` | Central Bank | 1 |
| POST | `/api/v1/payments/escrows` | Commercial Bank | 2 |
| POST | `/api/v1/payments/escrows/approve` | Central Bank | 2 |
| POST | `/api/v1/payments/escrows/reject` | Central Bank | 2 |
| POST | `/api/v1/payments/redeems` | Commercial Bank | 3 |
| POST | `/api/v1/payments/redeems/approve` | Central Bank | 3 |
| POST | `/api/v1/payments/redeems/reject` | Central Bank | 3 |
