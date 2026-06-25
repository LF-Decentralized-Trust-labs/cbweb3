# Research: FIX-1 — PKI Local CA Model

**Date**: 2026-06-25
**Branch**: `016-fix-pki-local-ca`

## Summary

All unknowns resolved from direct code inspection. No external research required. The platform already has a complete, well-structured CSR → CB-signs implementation that the new toolkit must reuse.

---

## Finding 1 — The CSR → CB-signs flow is fully implemented, end-to-end

**Decision**: The toolkit MUST call the existing CSR signing infrastructure, not invent a new path.

**Rationale**:

The entire CSR → CB-signs chain already exists across three layers:

| Layer | File | What it does |
|-------|------|--------------|
| Shared lib | `scenario-a/backend/shared/identity/ca.go` | `GenerateCSR(cn, org, ou, country)` → (csrPEM, keyPEM); `SignCSR(caCertPEM, caKeyPEM, csrPEM, years)` → IssuedCert |
| Compliance PKI wrapper | `scenario-a/backend/services/compliance/internal/pki/ca.go` | `CA.SignCSR(csrPEM)` — loads CB CA from `CA_CERT_FILE`/`CA_KEY_FILE` env vars, delegates to shared lib |
| Compliance gRPC | `ComplianceService.SignParticipantCSR` (proto + server) | Full gRPC method: receives `{csr_pem, user_id, role, institution_name, legal_entity_id}`, signs via `CA.SignCSR`, updates participant record, triggers on-chain IdentityRegistry registration |
| API Gateway endpoint | `POST /api/v1/onboarding/credential-request` | CB-side HTTP handler that validates and routes to ComplianceService |
| Smart proxy | `scenario-a/backend/services/api-gateway/internal/http/handlers/onboarding_proxy.go` | Commercial bank side: reads `{bankCode}.csr` from disk, enriches payload with CSR + blockchain pubkey, forwards to CB `credential-request` endpoint |

The toolkit (TK-3 engine / TK-9 join flow) must replicate only what the smart proxy does: generate keypair + CSR, then POST to the CB `credential-request` endpoint. No new signing code is needed.

**Alternatives considered**: Writing a standalone `openssl`-based CSR signing step in the provisioning engine — rejected because the Go shared lib already handles this correctly and is tested (`ca_signature_test.go`); duplicating it would create drift.

---

## Finding 2 — The local Makefile shortcut: what it does wrong and why it is a shortcut

**Decision**: The Makefile target `pki.gen-commercial-bank-*` generates a **per-bank CA**, then uses that bank CA to sign the bank's own leaf cert. This is structurally wrong (bank is its own trust anchor) but is acceptable as a local dev convenience.

**Rationale**:

In `make/05-pki.mk`, the `gen_commercial_bank_cert` macro:
1. Creates `bank-a-ca.key` and `bank-a-ca.crt` (a self-signed CA owned by the bank)
2. Generates `bank-a.key` and `bank-a.csr`
3. Signs `bank-a.crt` with `bank-a-ca.key`

In production, step 1 must not happen. The bank never holds a CA key. Instead:
1. Bank generates only `bank-a.key` and `bank-a.csr`
2. CB receives the CSR via `credential-request` and signs it with `central-bank-a-ca.key`
3. Bank stores the returned `cert_pem` as its leaf cert

The fix for P1 (Developer story) is purely additive: add warning comments + `[DEV ONLY]` terminal output to the Makefile. The existing flow must remain functional.

**Alternatives considered**: Changing the Makefile to generate a bank-less cert signed by the CB CA — rejected because it would require the CB CA key to be present in a commercial bank developer's environment, which violates key isolation. The local shortcut is acceptable as long as it is clearly marked.

---

## Finding 3 — Where the toolkit will generate/submit the CSR (TK-3 / TK-9)

**Decision**: The toolkit's PKI step uses `GenerateCSR` from the shared library to produce keypair + CSR, then calls `POST /api/v1/onboarding/credential-request` on the CB API gateway.

**Rationale**:

The task-notions document (§5.3) lists the orchestration sequence for a commercial bank join:
1. `generate/obtain TLS` — this is the CSR step
2. CB returns signed cert (via `credential-request`)
3. PoP → `onboarding/complete` → cert stored

TK-3 is the orchestration engine (Phase 1 step 5 from task-notions). The PKI step in TK-3 for a `role: central-bank / mode: found` spoke would call `GenerateSelfSignedCA` to bootstrap the CB root (identical to `pki.gen-central-bank-*` in the Makefile, which is already correct).

TK-9 is the commercial-bank join flow (Phase 3 step 10 from task-notions). The PKI step in TK-9 for `role: commercial-bank / mode: join` would:
1. Call `GenerateCSR` → write `bank-x.key`, `bank-x.csr` to the key provider
2. POST to CB `credential-request` endpoint
3. Poll `onboarding/status` until approved
4. Call `onboarding/complete` with PoP signature
5. Store returned `cert_pem` as `bank-x.crt`

This exactly mirrors what `onboarding_proxy.go` (smart mode) does — the toolkit becomes the direct client rather than routing through the proxy.

**Alternatives considered**: Calling the compliance gRPC `SignParticipantCSR` directly from the toolkit — rejected because the toolkit is an external operator tool, not an internal service; it should use the same public API boundary as any participant.

---

## Finding 4 — Test coverage for the shared PKI library

**Decision**: `ca_signature_test.go` already exists at `scenario-a/backend/shared/identity/`. New toolkit PKI tests must be added within the toolkit package (not the shared library), using `NewCAFromPEM` for test CA setup.

**Rationale**: The shared library's `GenerateCSR`, `SignCSR`, and `GenerateSelfSignedCA` functions are already unit-tested. The toolkit's PKI logic (calling these functions in the correct sequence, failing fast on CB unreachable, not generating bank CA keys) is what needs new tests. Per constitution Principle V, failing tests must be written before implementing the toolkit CSR step.

**Alternatives considered**: Adding toolkit-specific tests to the shared library — rejected; the shared library is a pure utility, not a test harness for toolkit workflows.

---

## All NEEDS CLARIFICATION Items — Resolved

None existed in the spec. All decisions were resolved by direct code inspection.
