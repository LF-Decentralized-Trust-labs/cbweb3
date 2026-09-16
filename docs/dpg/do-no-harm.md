# Do no harm by design

*DPG Standard, indicator 9 — the project anticipates, prevents and does no harm.*

CBWeb3 is settlement infrastructure for central bank money. The harms worth taking
seriously here are not the ones a social platform worries about; they are financial loss,
surveillance of legitimate economic activity, exclusion of participants, and the misuse of
research software in an environment it was never hardened for. Each is addressed below,
including where the answer is "this does not apply" and where the answer is "not yet".

## Data privacy

The system processes a deliberately small set of personal data — operator accounts,
audit records and the credentials issued at onboarding — and no data about the customers of
participating banks. No personal data is written to any ledger. Inter-institutional
transfers are confidential by construction, so a participant cannot observe the business of
another participant simply by running a node. The complete account is in the
[privacy notice](privacy.md).

The corresponding harm — a supervisor or operator with unchecked visibility — is bounded by
role separation enforced at the gateway, and by the rule that the disclosure path used to
decrypt a transaction is itself restricted by role and written to an immutable audit trail.
Accountability is symmetric: the watchers are logged too.

## Inappropriate and user-generated content

**Not applicable.** CBWeb3 hosts no user-generated content: no posts, no messages between
end users, no uploads, no profiles, no public feed. Its inputs are payment instructions and
governance actions submitted by authenticated institutional operators. There is nothing to
moderate, and consequently no moderation mechanism is claimed.

## Financial harm

This is the material risk, and the architecture is shaped by it.

- **Atomicity.** Cross-border exchange settles through Hash Time-Lock Contracts: either
  both legs complete or both unwind. Principal risk — one party paying and the other not
  delivering — is removed by the mechanism rather than by a contractual promise. The
  contracts carry unit, fuzz and invariant tests, and are statically analysed in CI.
- **Containment.** The networks are permissioned and gasless. Only approved participants,
  recorded in an on-chain identity registry, can transact; there is no public mempool, no
  fee auction, and no path by which a token minted on a test network reaches a real market.
- **Emergency controls.** Governance can freeze an account and trip a circuit breaker that
  halts activity, with both actions recorded in the audit trail.
- **No real money.** The networks are test networks. No participant's funds, and no
  member of the public's funds, are at stake in any deployment that exists today.

## Community harm

The project operates under the
[Linux Foundation Code of Conduct](https://lf-decentralized-trust.github.io/governance/governing-documents/code-of-conduct.html),
adopted in [`CONTRIBUTING.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/CONTRIBUTING.md)
and [`CODE_OF_CONDUCT.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/CODE_OF_CONDUCT.md).
It binds every participant in the working group and in the repository, provides a route for
reporting harassment, and its enforcement can extend to removal from the working group.
Participation is open to any individual or organisation, and technical decisions are taken
in public with a documented voting fallback, so that disagreement has a procedure rather
than an outcome decided privately.

## Security harm

Vulnerabilities are reported privately under a published
[security policy](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/SECURITY.md)
with stated acknowledgement and assessment targets and a 90-day coordinated disclosure
window. Fixes are published as GitHub Security Advisories. Supply-chain exposure is
narrowed by dependency automation, enforced license headers, and a repository-wide secret
scan that runs over both the working tree and the entire history.

## What this software is not ready for

Publishing the limits is part of doing no harm. CBWeb3 is a **reference implementation and
test network**, and an institution that deployed it unchanged into production would be
taking risks the project does not claim to have closed:

- **Development defaults are insecure by intention.** Local deployments can disable
  authentication entirely (`NOC_SKIP_AUTH=true`, which logs a warning saying not to use it
  in production), and root certificates, key custody and TLS termination are all wired to
  local-development defaults. Production requires the real key provider, a real certificate
  authority, TLS, and `COOKIE_SECURE=true`.
- **A known upstream defect is pinned.** The reference deployment pins a Paladin release
  carrying a Zeto `transferLocked` defect that can permanently strand a private lock in
  roughly one of every 256 attempts. It is documented in
  [`TOOLCHAIN.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/TOOLCHAIN.md)
  and [`paladin-upgrade.md`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/docs/paladin-upgrade.md),
  and the upgrade is an open item rather than a closed one.
- **Operational readiness is incomplete.** Backup and restore, disaster recovery, staging
  parity and throughput limits are assessed in the platform's production-readiness roadmap,
  not solved by it.

An institution evaluating CBWeb3 should read those documents before deploying anything, and
treat the current release as material for experimentation and study.

## Exclusion

A permissioned network can exclude by design; CBWeb3's answer is that admission is a
governance decision taken by identified institutions under published rules, recorded on
chain, and auditable — not a discretionary act by a platform operator. The software itself
imposes no barrier to adoption: it is openly licensed, runs on ordinary hardware with no
paid dependency, and its documentation and conformance material are public, so no
institution is excluded for lack of a vendor relationship or a licence fee.
