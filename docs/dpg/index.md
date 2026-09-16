# CBWeb3 and the DPG Standard

CBWeb3 is a candidate **Digital Public Good**. This section is the public evidence base
for that candidacy: one page per indicator of the
[DPG Standard](https://www.digitalpublicgoods.net/standard), each stating what CBWeb3
does today — not what it intends to do — and linking to the artifact in this repository
that backs the claim.

!!! note "What CBWeb3 is, and is not"
    CBWeb3 is a **regional test network and reference implementation** for tokenized
    central bank money. It is research and experimentation infrastructure: no production
    deployment exists, no real money moves through it, and it serves no retail public.
    Its users are central banks, supervisors and commercial banks operating in a
    controlled environment. Claims on these pages are scoped accordingly.

## The nine indicators

| # | Indicator | Status | Evidence |
|---|---|---|---|
| 1 | Relevance to the SDGs | Documented | [SDG alignment](sdg-alignment.md) |
| 2 | Approved open licence | Met | [`LICENSE`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/LICENSE) (Apache-2.0), [`NOTICE`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/NOTICE) |
| 3 | Clear ownership | Documented | [Ownership and governance](ownership.md) |
| 4 | Platform independence | Documented | [Platform independence](platform-independence.md) |
| 5 | Documentation | Met | [Documentation map](#documentation-indicator-5) below |
| 6 | Mechanism for extracting data | Documented | [Data export](data-export.md) |
| 7 | Privacy and applicable laws | Documented | [Privacy notice](privacy.md) |
| 8 | Standards and best practices | Documented | [Open standards](standards.md) |
| 9 | Do no harm by design | Documented | [Do no harm](do-no-harm.md) |

## Open licensing (indicator 2)

The whole repository is licensed **Apache-2.0**, an OSI- and DPGA-approved licence:

- [`LICENSE`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/LICENSE) — the full text, at the repository root.
- [`NOTICE`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/NOTICE) — attribution, `Copyright 2025-2026 The CBWeb3 Authors`.
- Every first-party source file carries an `SPDX-License-Identifier: Apache-2.0` header:
  Solidity, Go, TypeScript and CI workflows. The header is not a convention — a
  `License Headers` CI job fails the build when a file is missing one.
- Third-party dependencies are inventoried in
  [`platform/LICENSES-THIRD-PARTY.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/LICENSES-THIRD-PARTY.md);
  all listed licences are OSI-approved and redistribution-compatible with Apache-2.0.
- Documentation is Markdown in the same repository, under the same licence.

## Documentation (indicator 5)

Documentation is public, versioned alongside the code, and sufficient for a competent
third party to understand, deploy and extend the system.

| Need | Where |
|---|---|
| What the system is and how it settles payments | [Introduction](../index.md), [repository README](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/README.md) |
| Full technical design | [Technical Blueprint](../CBWeb3_Technical_Blueprint.md) |
| Architecture views and component documentation | [`platform/docs/architecture/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/platform/docs/architecture) |
| Design decisions and their rationale | [`platform/docs/decisions/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/platform/docs/decisions) (ADRs) |
| Deployment and toolchain | [`platform/README.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/README.md), [`platform/docs/TOOLCHAIN.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/TOOLCHAIN.md) |
| End-user manuals per portal | [`platform/docs/user-manuals/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/platform/docs/user-manuals) |
| API reference | [`platform/scenario-a/backend/services/api-gateway/docs/openapi.yaml`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/scenario-a/backend/services/api-gateway/docs/openapi.yaml) — OpenAPI 3.0.3, 74 paths |
| Integration contracts for third parties | [`Toolbox/contracts/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/Toolbox/contracts) — PvP, AMM and auth contracts, versioned |
| Smart contract reference | [`platform/scenario-a/contracts/docs/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/platform/scenario-a/contracts/docs) (generated from source) |
| How to participate | [`CONTRIBUTING.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/CONTRIBUTING.md), [`MAINTAINERS.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/MAINTAINERS.md) |
| Research underpinning the design | [Research papers](../research-papers/index.md) — governance, interoperability and privacy analyses |

## Keeping this current

DPG status is granted for one year and renewed annually. These pages are part of the
repository and change through pull requests like any other artifact; a claim that stops
being true is a defect and should be reported as
[an issue](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues).
