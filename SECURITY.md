# Security Policy

CBWeb3 is infrastructure for tokenized central bank money. Vulnerabilities in it can
affect central banks, supervisors and commercial banks operating on the network. Please
report them privately and give us a chance to fix them before they become public.

## Reporting a vulnerability

**Do not open a public issue, pull request or discussion for a security problem.**

Report it through **[GitHub private vulnerability reporting](https://github.com/LF-Decentralized-Trust-labs/cbweb3/security/advisories/new)**,
which is private to the maintainers.

<!-- TODO: add the LNet security contact address once confirmed. -->

Please include:

- What the problem is and which component is affected
  (platform service, smart contract, API gateway, Toolbox artifact, documentation)
- The version, commit or deployment where you observed it
- Steps to reproduce, ideally minimal
- The impact you believe it has — who could do what, to whom
- Any suggested remediation

If you cannot share details over GitHub, say so in the report and we will arrange another
channel.

## What to expect

| Stage | Target |
|---|---|
| Acknowledgement of your report | 3 business days |
| Initial assessment and severity classification | 10 business days |
| Status update while a fix is in progress | every 15 days |
| Coordinated disclosure after a fix ships | by agreement with the reporter |

We will tell you our assessment even when we conclude the report is not a vulnerability,
and we will explain why.

## Scope

**In scope**

- Smart contracts and their deployment configuration
- Backend services, the API gateway and the WebSocket gateway
- Authentication, authorisation and role separation between participant types
- Cross-network settlement paths (HTLC, bridge, AMM) and their atomicity guarantees
- Privacy guarantees claimed by the documentation
- Toolbox artifacts — interface contracts, mocks, test vectors, conformance tests
- Repository supply chain — CI workflows, dependencies, release artifacts

**Out of scope**

- Infrastructure operated by individual participating institutions, which each
  institution secures and discloses on its own terms
- Findings that require privileged access already granted to the reporter, unless that
  access can be escalated beyond its intended scope
- Reports generated solely by automated scanners with no demonstrated impact

## Coordinated disclosure

We ask reporters not to disclose publicly until a fix is available or 90 days have passed,
whichever comes first. Reporters who follow this policy will be credited in the advisory
unless they prefer otherwise.

## Fixed vulnerabilities

Published as [GitHub Security Advisories](https://github.com/LF-Decentralized-Trust-labs/cbweb3/security/advisories)
and recorded in [CHANGELOG.md](CHANGELOG.md).
