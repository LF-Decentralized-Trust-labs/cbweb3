# Data Model: PKI File Contract for Scenario A

**Feature**: 016-fix-pki-local-ca
**Date**: 2026-06-25

This document defines the PKI artifact contract that the provisioning toolkit (TK-3/TK-9) must produce and consume. All files are on-disk PEM-encoded credentials managed by the key provider (local KMS emulator for local profile; real KMS for prod profile).

---

## Central Bank (mode: found) — Spoke CA artifacts

The founding central bank generates a self-signed root CA. This is correct and unchanged.

| File | Owner | Description | Must not contain |
|------|-------|-------------|-----------------|
| `central-bank-{x}-ca.key` | Central Bank | ECDSA P-256 CA private key. Signs every commercial bank CSR for this spoke. Ships in join bundle. | Any other entity's key material |
| `central-bank-{x}-ca.crt` | Central Bank | Self-signed X.509 CA certificate. Spoke trust anchor. Ships in join bundle. | — |

**Source**: `shared/identity.GenerateSelfSignedCA(cn, org, years)` — already used by Makefile `pki.gen-central-bank-*`. Toolkit reuses same function.

---

## Commercial Bank (mode: join) — Participant artifacts

A joining commercial bank generates ONLY a keypair and CSR. The leaf cert is returned by the CB after signing.

| File | Owner | Description | Must not contain |
|------|-------|-------------|-----------------|
| `{bankCode}.key` | Commercial Bank | ECDSA P-256 private key. Never leaves the bank's key provider. | — |
| `{bankCode}.csr` | Commercial Bank | PKCS#10 Certificate Signing Request. `CN={bankCode}`, `O={institution}`, `OU=ROLE_COMMERCIAL_BANK`, `C=BR`. Submitted to CB for signing. | Private key material |
| `{bankCode}.crt` | Issued by CB CA | X.509 leaf certificate. Issuer = `central-bank-{x}-ca.crt`. Returned by CB `credential-request` flow. | CA flag (`isCA=false` enforced by `shared/identity.SignCSR`) |

**Prohibited files** (must never appear in a production toolkit join flow):

| Prohibited file | Why |
|----------------|-----|
| `{bankCode}-ca.key` | Commercial bank MUST NOT hold a CA private key |
| `{bankCode}-ca.crt` | Commercial bank MUST NOT have a CA cert |

**Source**:
- Keypair + CSR: `shared/identity.GenerateCSR(bankCode, institution, "ROLE_COMMERCIAL_BANK", "BR")`
- Signing (CB side): `compliance/pki.CA.SignCSR(csrPEM)` → called via `POST /api/v1/onboarding/credential-request`

---

## Join Bundle — Spoke trust anchor propagation

The join bundle emitted by a founded spoke includes the CB CA cert as the spoke trust anchor. A joining commercial bank consumes this before submitting its CSR.

| Field | Description |
|-------|-------------|
| `trust_anchor_pem` | `central-bank-{x}-ca.crt` in PEM format |
| `credential_request_url` | CB API endpoint for CSR submission (e.g., `https://cb-api-gateway/api/v1/onboarding/credential-request`) |

---

## State transitions (commercial bank PKI lifecycle)

```
[initial]
   │
   ▼ GenerateCSR()
[keypair + CSR on disk]
   │
   ▼ POST credential-request (CSR → CB)
[CSR submitted, status: pending]
   │
   ▼ CB signs via ComplianceService.SignParticipantCSR
[status: approved, cert_pem available]
   │
   ▼ POST onboarding/complete (PoP signature)
[cert stored as {bankCode}.crt, on-chain IdentityRegistry registered]
   │
   ▼
[ENROLLED — PKI complete]
```

**Error states**:
- CB unreachable → fail fast, exit non-zero, no fallback to self-signing
- CSR signature invalid → CB rejects, fail fast
- PoP signature invalid → CB rejects completion, cert not stored
