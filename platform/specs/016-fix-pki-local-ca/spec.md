# Feature Specification: Fix PKI Local CA Model — Document Bootstrap Shortcut and Enforce CB-Signed Flow in New Toolkit

**Feature Branch**: `016-fix-pki-local-ca`
**Created**: 2026-06-25
**Status**: Draft
**Input**: FIX-1 — In `make/05-pki.mk`, each commercial bank generates its own CA (`bank-a-ca.key/crt`) and self-signs. The correct production model (CB signs the bank's CSR) is already implemented in `onboarding_proxy.go` (smart mode). Fix: (1) document that `pki.gen-commercial-bank-*` is a local-dev bootstrap shortcut only; (2) ensure the new provisioning toolkit (TK-3/TK-9) uses the real CSR → CB-signs flow.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer Bootstraps Local PKI with Clarity (Priority: P1)

A developer running Scenario A locally executes `make pki.gen-commercial-bank-bank-a` to bootstrap credentials for testing. Currently the Makefile silently creates a bank-owned CA and self-signs the bank cert — a behavior that does not match production and has caused confusion about what the trust model actually is.

After this fix, the developer sees an explicit notice in the Makefile comments (and in any terminal output) that `pki.gen-commercial-bank-*` is a **local development shortcut only** and that in production the central bank CA issues every commercial bank certificate. The bootstrap path still works; the semantics are now clear.

**Why this priority**: This is the fastest, lowest-risk part of the fix and directly unblocks onboarding new team members without misleading them about the PKI model. It requires no code change — only documentation/comment additions.

**Independent Test**: Can be validated by reading the updated `make/05-pki.mk` comments and any associated README additions; no runtime execution required.

**Acceptance Scenarios**:

1. **Given** a developer reads `make/05-pki.mk`, **When** they look at the `pki.gen-commercial-bank-*` target and the `gen_commercial_bank_cert` macro, **Then** they find at minimum one clear, prominent comment stating that this target is for local development only and does not represent the production PKI flow.
2. **Given** a developer runs `make pki.gen-commercial-bank-bank-a`, **When** the target executes, **Then** the terminal output includes a visible warning line (e.g., `[DEV ONLY] ...`) distinguishing this from the production CSR→CB flow.
3. **Given** a new team member reads the scenario-a README PKI section, **When** they follow the local setup instructions, **Then** the documentation explicitly contrasts the local bootstrap path with the production CB-signed path and references `onboarding_proxy.go` smart mode as the authoritative production implementation.

---

### User Story 2 - Operator Provisions a Commercial Bank with CB-Signed Certificate (Priority: P2)

An operator uses the new provisioning toolkit (TK-3 certSource interface + TK-9 commercial-bank join flow) to onboard a commercial bank onto a spoke. The operator provides a `commercial-bank` manifest (`mode: join`). The toolkit must issue the bank's TLS certificate via the real production flow: the bank generates its keypair and CSR locally; the CSR is submitted to the spoke's central bank; the central bank signs it using the spoke CA (`central-bank-<x>-ca.{crt,key}`); the bank receives a leaf cert whose chain terminates at the CB CA.

**Why this priority**: This is the core correctness fix. Allowing the toolkit to silently use a bank-self-signed CA in "new toolkit" code would reproduce the exact problem the fix is meant to eliminate, and it would never be caught until a production deployment.

**Independent Test**: Can be tested by running the commercial-bank join flow in the toolkit against a locally-founded spoke, then inspecting the issued certificate's issuer — it must match the central bank CA, not a bank-owned CA.

**Acceptance Scenarios**:

1. **Given** a spoke has been founded by a central bank (its CA exists), **When** an operator applies a `commercial-bank` manifest via the toolkit, **Then** the toolkit generates only a keypair and CSR on the bank side — no CA key or CA cert is created for the bank.
2. **Given** a commercial bank CSR has been generated, **When** the toolkit submits the CSR to the central bank's credential-request endpoint, **Then** the central bank signs it with `central-bank-<x>-ca.key` and returns a leaf certificate.
3. **Given** the commercial bank has received its leaf cert, **When** the cert chain is verified, **Then** the issuer is the spoke's central bank CA — not a bank-owned CA.
4. **Given** the central bank API is unreachable during the join flow, **When** the toolkit attempts to submit the CSR, **Then** the join operation fails fast with a clear error and does not fall back to self-signing.

---

### User Story 3 - Security Auditor Verifies No Bank CA Exists in Production Artifacts (Priority: P3)

A security auditor or reviewer inspects a commercial bank's deployed credential set (PKI files, join bundle, Compose configs). Under the correct model the bank holds a private key and a leaf cert; it must never hold a CA private key. The auditor can confirm this by checking that no `bank-*-ca.key` artifact appears in any production-path output of the new toolkit.

**Why this priority**: This is a verifiability guarantee derived from the other stories. It can only be tested after Story 2 is implemented but provides an important audit signal.

**Independent Test**: Validate by inspecting the artifact set emitted by a complete commercial-bank join flow in the toolkit — the directory must contain `bank-x.key`, `bank-x.csr`, `bank-x.crt` but must not contain `bank-x-ca.key` or `bank-x-ca.crt`.

**Acceptance Scenarios**:

1. **Given** a commercial bank has been fully onboarded via the toolkit, **When** the bank's PKI artifact directory is listed, **Then** no file matching `*-ca.key` or `*-ca.crt` exists under the bank's credential path.
2. **Given** a code review of the TK-3/TK-9 implementation, **When** all certificate-generation paths are traced, **Then** no code path generates a CA key for a commercial bank entity.

---

### Edge Cases

- What happens when the central bank CA key is missing or inaccessible when the toolkit attempts to sign the CSR? The join flow must fail with a descriptive error, not silently fall back to self-signing.
- What happens if a developer runs `pki.gen-commercial-bank-*` after already having run the toolkit join flow? The Makefile target must be idempotent (skip if cert exists) and must not overwrite a toolkit-issued cert with a self-signed one (guarded by the existing `FORCE=1` pattern).
- What happens when the existing local-dev sample network (`deploy/local` + current Makefile) is used? Its behavior must be unchanged — this fix adds documentation only, no functional changes to the existing Makefile path.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `pki.gen-commercial-bank-*` Makefile targets and the `gen_commercial_bank_cert` macro MUST include a prominent comment block stating they are **local development bootstrap shortcuts** that do not represent the production PKI model.
- **FR-002**: The terminal output of `pki.gen-commercial-bank-*` MUST include a visible `[DEV ONLY]` warning line to distinguish this path from the production CSR→CB flow at runtime.
- **FR-003**: The scenario-a `README.md` (or equivalent PKI documentation) MUST explicitly describe both paths: (a) local bootstrap shortcut (Makefile) and (b) production path (CSR → central bank signs), with a reference to `onboarding_proxy.go` smart mode as the authoritative production implementation.
- **FR-004**: The new provisioning toolkit (certSource interface / TK-3 and commercial-bank join flow / TK-9) MUST issue commercial bank certificates exclusively via the CSR → central-bank-signs flow; no toolkit code path may generate a CA keypair for a commercial bank entity.
- **FR-005**: When the toolkit submits a CSR for signing, it MUST use the spoke's central bank credential-request endpoint (the same endpoint used by `onboarding_proxy.go` smart mode), not any local self-signing shortcut.
- **FR-006**: The toolkit join flow MUST fail fast with a clear error message if the central bank CA is unreachable or if certificate signing fails; it MUST NOT fall back to self-signing.
- **FR-007**: The existing `pki.gen-commercial-bank-*` Makefile targets MUST continue to function for local bootstrapping (no functional regression); only comments and output messaging are added.

### Key Entities

- **Spoke CA (Central Bank CA)**: The X.509 CA held exclusively by the founding central bank (`central-bank-<x>-ca.{crt,key}`). It is the trust root for all participant certificates on its spoke. Ships in the join bundle as the spoke's trust anchor.
- **Commercial Bank Leaf Certificate**: An X.509 certificate (`bank-x.crt`) issued and signed by the Spoke CA. The bank holds only its private key (`bank-x.key`) — never a CA key.
- **CSR (Certificate Signing Request)**: Generated by the commercial bank during the join flow (`bank-x.csr`). Submitted to the central bank for signing. Contains `OU=ROLE_COMMERCIAL_BANK`.
- **Join Bundle**: The artifact emitted by a founded spoke containing enode, genesis, deployed contract addresses, relay endpoint, and the Spoke CA cert as trust anchor. Consumed by commercial banks doing `mode: join`.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of comments in `make/05-pki.mk` adjacent to commercial-bank cert generation explicitly state the local-dev-only nature of that path — verifiable by reading the file, zero ambiguous cases.
- **SC-002**: A commercial bank onboarded via the toolkit has a certificate whose issuer is the spoke's central bank CA — verifiable via `openssl verify` against the CB CA cert with a zero-error result.
- **SC-003**: Zero files matching `*-ca.key` or `*-ca.crt` appear in the artifact output of a commercial bank join flow executed via the toolkit.
- **SC-004**: The existing local dev stack (`make pki.gen-all`) continues to complete without errors after this fix — no regression in the sample network path.
- **SC-005**: Any attempt by the toolkit to onboard a commercial bank without a reachable central bank CA results in a non-zero exit code and a human-readable error within 30 seconds — no silent success.

---

## Assumptions

- This fix is scoped exclusively to Scenario A; Scenario B is not affected.
- TK-3 refers to the certSource interface component of the new provisioning toolkit (§5.3 of task-notions), which owns the TLS generation step during spoke setup.
- TK-9 refers to the commercial-bank join flow component (Phase 3 / step 10 of task-notions), which is the entry point for commercial bank onboarding via the toolkit.
- The new provisioning toolkit is not yet implemented; this specification defines what the PKI behavior of those toolkit components MUST be before they are built.
- The `onboarding_proxy.go` smart mode is the canonical reference implementation of the CSR → CB-signs flow and should be reused or mirrored by the toolkit — not reimplemented from scratch.
- No changes are made to the central bank CA generation targets (`pki.gen-central-bank-*`) — those are correct as-is (CB self-signs its own CA, which is the spoke root).
- The local KMS emulator and self-signed cert rooting described in the task-notions do not conflict with this fix: self-signed rooting applies to the CB CA itself (the root), not to commercial bank certs (which are always CB-issued leaf certs).
- Samuel Venzi is the owner and reviewer for this fix (per task-notions metadata).
