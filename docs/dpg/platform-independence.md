# Platform independence

*DPG Standard, indicator 4 — no mandatory dependency on a closed component.*

CBWeb3 can be built, deployed and operated end to end with open-source software only.
There is no paid tier, no licence key, no proprietary runtime and no hosted service that
the system cannot run without.

## Runtime components

| Layer | Component | Licence |
|---|---|---|
| Ledger (spokes and hub) | [Hyperledger Besu](https://www.hyperledger.org/projects/besu) 25.8.0, QBFT consensus, gasless | Apache-2.0 |
| Privacy | [Hyperledger Paladin](https://lf-decentralized-trust.github.io/paladin/) (Zeto zero-knowledge tokens, Noto notary) | Apache-2.0 |
| Cross-network relay | [Hyperledger Cacti](https://www.hyperledger.org/projects/cacti) | Apache-2.0 |
| Identity and sessions | [Keycloak](https://www.keycloak.org/) (OpenID Connect) | Apache-2.0 |
| Persistence | [PostgreSQL](https://www.postgresql.org/) | PostgreSQL Licence (OSI-approved) |
| Cache | Redis, pinned to `7.2-alpine` — [why the pin matters](#a-note-on-the-redis-pin) | BSD-3-Clause |
| Backend services | Go 1.26 | BSD-3-Clause |
| Frontends | React / Vite on Node.js 22 LTS | MIT |
| Contracts | Solidity, built and tested with [Foundry](https://getfoundry.sh/) | MIT / Apache-2.0 |
| Packaging | Docker and Docker Compose, Alpine base images | Apache-2.0 |
| Documentation site | [MkDocs](https://www.mkdocs.org/) with Material for MkDocs | BSD-2-Clause / MIT |

Exact version floors are pinned and enforced in
[`platform/docs/TOOLCHAIN.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/TOOLCHAIN.md);
the full dependency inventory with licences is in
[`platform/LICENSES-THIRD-PARTY.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/LICENSES-THIRD-PARTY.md).
All listed licences are OSI-approved and compatible with redistribution of an Apache-2.0
work. `go-ethereum` is consumed as a library under LGPL-3.0, which the project's usage
respects and which does not restrict downstream adopters of CBWeb3 itself.

## The three points worth naming

Rather than assert independence in the abstract, these are the places where a closed
component could plausibly have crept in, and what is actually there:

**Key custody.** Production deployments of a payment system are expected to hold signing
keys in an HSM or a cloud KMS, which are typically proprietary. CBWeb3 does not depend on
any particular one: key custody sits behind a `KeyProvider` interface selected by a
`kms://` URI. The default, and the only implementation shipped, is a **local software
provider** that runs with no external service. Any HSM or KMS — including open
implementations such as SoftHSM or HashiCorp Vault — can be slotted in by implementing
the same interface, and the rest of the system is unaware of the choice. See
[`platform/docs/secret-management.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/secret-management.md).

**Certificate authority.** Participant onboarding uses X.509 certificates issued from
PKCS#10 certificate signing requests. Certificates are generated with OpenSSL; the
reference deployment roots them itself. No commercial CA is required, though an operator
who wants one can use it.

**Hosting and CI.** The repository, its continuous integration and the documentation site
are hosted on GitHub. That is a convenience of the Linux Foundation's infrastructure, not
a technical dependency: the CI jobs are ordinary shell, Go, Node and Python commands that
run on any machine; the documentation site builds locally with `mkdocs build`; and the Git
history is complete and portable, so the project can be moved to any Git host without loss.
The documentation site can emit Google Analytics, but only when a
`GOOGLE_ANALYTICS_KEY` environment variable is supplied at build time — an unset variable
is the default and the site is fully functional without it.

## A note on the Redis pin

The cache image is pinned to **`redis:7.2-alpine`**, and the pin is a licensing decision
rather than a version preference. Redis releases through 7.2 are BSD-3-Clause; from 7.4 the
project relicensed to RSALv2 / SSPLv1, which is not OSI-approved, and only Redis 8 added an
AGPLv3 option. The previous default was the floating tag `redis:7-alpine`, which therefore
drifted onto a non-open licence — so the pin was introduced to keep every runtime component
of CBWeb3 under an OSI-approved licence.

Nothing architectural depends on the choice. The cache is reached over the ordinary Redis
wire protocol through the BSD-licensed `go-redis` client, so any protocol-compatible server
works unchanged; `REDIS_IMAGE_TAG` stays overridable, and an operator who wants to move
forward should prefer [Valkey](https://valkey.io/) (`valkey/valkey:8-alpine`, BSD-3-Clause,
maintained under the Linux Foundation). The rationale is recorded in
[`platform/docs/TOOLCHAIN.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/TOOLCHAIN.md).

## Deployment

Deployment is `docker compose` against images built from source in this repository. No
private registry, no orchestration vendor and no managed database is required. A complete
two-spoke environment with central banks, commercial banks, portals, relay and ledgers runs
on a single developer machine; see
[`platform/README.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/README.md).
