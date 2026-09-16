# Open standards and best practices

*DPG Standard, indicator 8 — adherence to open standards and recognised best practices.*

## Standards implemented

| Domain | Standard | Where it appears |
|---|---|---|
| Ledger protocol | **Ethereum** execution semantics and **JSON-RPC** API | Hyperledger Besu spokes and hub; the interface any client uses to read the chain |
| Consensus | **QBFT** (IBFT 2.0 family), a permissioned BFT algorithm | Every spoke and the hub, gasless |
| Tokens | **ERC-20** | `FiatCentralBankMoney` (tCeBM) and the AMM pool contracts |
| Contract interfaces | **Solidity ABI**, EVM bytecode | Published ABIs and generated contract documentation |
| Cryptography | **secp256k1** ECDSA, **Keccak-256**; EC `prime256v1` for TLS/PKI | Signing, addressing, certificates |
| Confidential transactions | **Zero-knowledge proofs** (Zeto) and notarised transfers (Noto), Hyperledger Paladin domains | Inter-institutional value transfer |
| Atomic settlement | **Hash Time-Lock Contracts** | Cross-spoke payment-versus-payment |
| Web APIs | **OpenAPI 3.0.3**, HTTP/1.1, JSON | Both scenario gateways and the Toolbox contracts |
| Identity and sessions | **OpenID Connect** / **OAuth 2.0**, **JWT** (RFC 7519) | Keycloak, gateway authorisation |
| Credentials | **X.509** certificates from **PKCS#10** signing requests | Participant onboarding |
| Data serialisation | JSON, YAML, **JSON Schema**, **Protocol Buffers** enums | APIs, configuration, Toolbox artifacts |
| Persistence | **SQL** (PostgreSQL) | Service state, exportable with standard tooling |
| Packaging | **OCI** container images, Docker Compose | Deployment |
| Licensing metadata | **SPDX** identifiers | Every first-party source file |

Financial-messaging standardisation — **ISO 20022** — is a stated direction for future
integration with existing payment systems, not a claim about the current implementation.
It is discussed in the [Technical Blueprint](../CBWeb3_Technical_Blueprint.md) and is not
implemented today.

## Engineering practice

**Version control and provenance.** All work is in public Git. Every commit carries a
Developer Certificate of Origin `Signed-off-by` line. Where a change was produced with AI
assistance, the commit records it in an `Assisted-by:` trailer, following LF Decentralized
Trust's draft AI guidelines — see
[`AGENTS.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/AGENTS.md).
When the platform was brought into this repository, its full 1,391-commit history was
imported with paths rewritten so that `git log` and `git blame` remain usable, rather than
squashed into a single import commit.

**Releases.** [Semantic Versioning](https://semver.org/) for interface contracts and
releases, with a [Keep a Changelog](https://keepachangelog.com/)-format
[`CHANGELOG.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/CHANGELOG.md).

**Continuous integration.** Every pull request runs the repository's gates: Toolbox CI
(contract linting with Spectral, JSON Schema validation of mocks and vectors, conformance
tests, artifact-path validation), a documentation build, and a repository-wide secret scan
over both the working tree and the full history. The platform carries its own backend,
contract and license-header workflows. The gates are written to fail on the defects they
exist to catch — they are periodically tested against deliberately planted defects, because
a gate that has never failed is not known to work.

**Testing.** Go unit and integration suites across the backend services; Foundry unit,
fuzz and **invariant** tests for the smart contracts; a conformance suite and published
test vectors in the Toolbox so that third-party implementations can be checked against the
same expectations; k6 load suites for performance baselines.

**Security practice.** Static analysis of contracts with Slither; dependency updates via
Dependabot; enforced license headers; secret scanning; a published
[security policy](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/SECURITY.md)
with private vulnerability reporting, stated response targets and coordinated disclosure.

**Documentation practice.** Architecture Decision Records for design decisions with their
rationale; an authoritative single-source toolchain file that the build enforces rather
than merely describes; user manuals per portal; and a published documentation site built
from the repository on every change to `main`.

## Principles

CBWeb3 is built to the
[Principles for Digital Development](https://digitalprinciples.org/) — in particular
*design for scale* (a hub-and-spoke topology that admits new countries without redesign),
*use open standards, open data, open source and open innovation* (everything above),
*reuse and improve* (the system is assembled from existing Hyperledger projects rather than
rebuilt), *build for sustainability* (co-governed in a neutral foundation, not dependent on
a single vendor or funder), and *be collaborative* (an open working group with public
decisions).

It also follows the direction of the BIS and CPMI work on wholesale CBDC and cross-border
payments — atomic payment-versus-payment to remove principal risk, sovereignty of each
domestic ledger, and interoperability without a shared global ledger — which is the
design's stated rationale in the [Technical Blueprint](../CBWeb3_Technical_Blueprint.md).
