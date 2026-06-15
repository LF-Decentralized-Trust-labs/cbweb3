# Digital Public Good (DPG) Standard — Compliance Checklist

This checklist tracks cbweb3-platform's alignment with the
[DPG Standard](https://digitalpublicgoods.net/standard/) indicators. It is a
living document; update it as gaps are closed.

Status legend: ✅ done · 🟡 partial / in progress · ⬜ not started

## 1. Relevance to Sustainable Development Goals
- 🟡 Project relevance (CBDC interoperability for financial inclusion / SDG 8, 9,
  17) should be stated in the root README. Track in a follow-up docs task.

## 2. Use of an approved open license
- ✅ Root `LICENSE` added: **Apache-2.0** (an OSI-approved, DPG-approved license).
- ✅ Copyright: `Copyright 2026 LACNet Networks`.

## 3. Clear ownership
- ✅ Ownership/attribution: LACNet Networks / central-bank consortium
  (module path `github.com/LACNetNetworks/cbweb3-platform`).

## 4. Platform independence
- 🟡 Built on open components (Hyperledger Besu, Cacti, Paladin, Keycloak,
  PostgreSQL, Go, React). No mandatory closed-source dependency identified.
  Verify no proprietary lock-in in deploy tooling.

## 5. Documentation
- 🟡 Architecture rules in `.specify/memory/constitution.md`; per-scenario
  READMEs exist. Adequate for contributors; end-user/deployment docs ongoing.

## 6. Mechanism for extracting data
- 🟡 Service persistence is PostgreSQL (standard, exportable). On-chain state is
  readable via standard Ethereum JSON-RPC. Document export procedures.

## 7. Adherence to privacy and applicable laws
- ✅ Privacy-by-design: inter-bank value transfers use ZetoToken (ZKP) or
  NotoToken (notary); no plaintext PII/amounts on-chain (per constitution).
- 🟡 Formal privacy policy / data-handling doc to be added.

## 8. Adherence to standards & best practices
- ✅ SPDX identifiers corrected across the codebase:
  - Solidity: all `UNLICENSED` → `Apache-2.0` (116 files).
  - Go: `// SPDX-License-Identifier: Apache-2.0` added to first-party sources
    (391 files); vendored/generated code excluded.
- ✅ `CONTRIBUTING.md` added (branch workflow, test-first, SPDX policy).
- ✅ Third-party dependency license report: `LICENSES-THIRD-PARTY.md`.
- 🟡 CI enforcement of license headers (e.g. a header-lint check) recommended.

## 9. Do no harm by design
- 🟡 Compliance gate (IdentityRegistry + Compliance service + Keycloak OIDC) and
  circuit-breaker controls are in place per the constitution. Security review
  process documented in CONTRIBUTING (no weakening of compliance/security).

## Remaining gaps / follow-ups
- Add SDG relevance statement and project description to the root README.
- Add a privacy policy / data-handling document.
- Add CI license-header enforcement and an automated (machine-generated)
  third-party license manifest (`go-licenses`, `license-checker`).
- One Solidity test file (`scenario-b/contracts/test/IdentityRegistryLP.t.sol`)
  is `MIT`-licensed; left as-is (compatible). Normalize if desired.
- Confirm go-ethereum (LGPL) usage is acceptable for DPG certification or
  isolate it.
