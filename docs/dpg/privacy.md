# Privacy notice

*DPG Standard, indicator 7 — adherence to privacy and applicable laws.*

This notice describes the personal data processed by the **CBWeb3 reference
implementation** and by this documentation site. It is written for the system as it exists:
a test network operated between institutions, with no retail users and no live economic
activity.

## Who the users are

CBWeb3 has no consumer-facing product. Its users are **named operators acting on behalf of
an institution** — a central bank, a supervisor, or a commercial bank — who sign in to a
role-specific portal. There are no citizen accounts, no retail wallets and no end-user
onboarding. Any institution deploying CBWeb3 is the controller of its own operators' data;
the project publishes the software and the design, and does not itself operate a network
holding anyone's personal data.

## What personal data the software processes

| Data | Where it lives | Why |
|---|---|---|
| Operator username, email address, assigned role, institution name | Keycloak (identity provider) | Authentication and role-based authorisation |
| Keycloak subject identifier (UUID) | Keycloak, service databases | Stable internal reference to an account |
| Blockchain address bound to an operator | Service database, on-chain participant registry | Authorising on-chain actions for an approved participant |
| Certificate signing request and public key submitted at onboarding, and any legal-entity identifier | Service database | Issuing the participant's X.509 credential |
| Audit records: actor identifier, actor address, action, target, result, timestamp, correlation identifier and **IP address** | Service database | Accountability for privileged actions (freezes, parameter changes, credential operations) |
| Session cookie holding an access token | Operator's browser, `HttpOnly` | Keeping a session |

That is the complete set. In particular, the system does **not** process:

- identity documents, photographs, biometrics or any natural-person dossier;
- residential addresses, telephone numbers or dates of birth;
- data about the customers of participating banks;
- payment data attributable to an identified natural person.

**About the "KYC" endpoints.** The compliance API exposes KYC and AML screening
operations, and the name invites a misreading. What they record is a **participant
lifecycle status** — `PENDING`, `APPROVED`, `FROZEN`, `REVOKED`, `REJECTED` — against an
institutional participant, together with a sanctions-screening result for that participant.
No identity documentation of a natural person is collected, stored or verified anywhere in
the codebase.

## What is on the ledger

No personal data is written to any blockchain in CBWeb3, by design and by architectural
rule (see [ADR-001](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/decisions/ADR-001-privacidade-vs-auditabilidade.md)).
On-chain state holds institutional blockchain addresses, role flags in the participant
registry, and token movements. Addresses are pseudonymous identifiers of institutions, not
of people — but they are persistent and correlatable, which is why inter-institutional
value transfers use privacy-preserving tokens (Zeto zero-knowledge proofs, or Noto with a
notary) rather than transparent transfers, so that amounts and counterparties are not
public to every node on the network.

Supervisory disclosure is a deliberate, narrow exception: an authorised supervisor can
request decryption of a transaction through an audited endpoint restricted by role. Every
such request is written to the audit trail. Confidentiality from other participants and
accountability to the supervisor are both design requirements; neither is achieved by
hiding the mechanism.

## Retention and security

- Audit records are retained for the life of the deployment, because their purpose is
  accountability; they are the one category deliberately not deleted.
- Credentials are never stored as plaintext: signing keys are held by a key provider and
  are non-exportable, and secret material is kept out of the repository — a repository-wide
  secret-scanning gate runs over every change and over the full history.
- Sessions use `HttpOnly`, `SameSite=Strict` cookies; production deployments additionally
  require `COOKIE_SECURE=true` and TLS termination.
- Access is role-separated across central bank, supervisor, governance and commercial bank
  roles, and enforced at the gateway.
- Vulnerabilities are reported privately under [`SECURITY.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/SECURITY.md).

## Applicable law

Deployments are expected in multiple jurisdictions, each with its own regime — Brazil's
**LGPD**, the EU **GDPR** where an EU-established entity is involved, and the national
data-protection and **banking secrecy** laws of each participating country. CBWeb3's design
is built to sit inside those constraints rather than to be reconciled with them afterwards:
data minimisation (only what authentication and accountability require), no personal data
on an immutable ledger, confidentiality of transaction detail by default, and disclosure
only through an audited, role-restricted path.

The regulatory analysis behind these choices is published in
[Privacy vs Confidentiality](../dpg-assesment/Privacy%20vs%20Confidentiality.md) and in the
[privacy research paper](../research-papers/index.md).

Because the project publishes software rather than operating a service, the legal
obligations of a controller — lawful basis, notices to data subjects, rights requests,
retention schedules, breach notification — fall on each deploying institution. Test
environments are expected to use fictitious data.

## This documentation site

The site is published with GitHub Pages. GitHub processes visitors' requests, including IP
addresses, under its own terms. The site can emit **Google Analytics** when a build-time key
is configured; when no key is set, no analytics are loaded. The site sets no other tracking
cookies and hosts no third-party fonts or scripts.

Contributing to the repository is public by nature: pull requests, issues and commit
metadata — including the name and email in a commit's `Signed-off-by` line, required by the
Developer Certificate of Origin — are permanently published in the Git history.

## Contact

Privacy questions about the project can be raised in
[GitHub Discussions](https://github.com/LF-Decentralized-Trust-labs/cbweb3/discussions) or
as an [issue](https://github.com/LF-Decentralized-Trust-labs/cbweb3/issues), or sent to the
project's technical lead at **cvelasquez@lnet.global**. Anything security-sensitive should
go through
[private vulnerability reporting](https://github.com/LF-Decentralized-Trust-labs/cbweb3/security/advisories/new)
rather than a public channel.

Questions about personal data held by a particular **deployment** of CBWeb3 belong to the
institution operating it, not to this project; the project publishes the software and does
not operate a network holding anyone's data.

*Last reviewed: 2026-09-16.*
