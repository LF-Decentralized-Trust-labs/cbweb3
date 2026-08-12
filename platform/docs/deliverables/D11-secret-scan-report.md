# D11 — Secret Scan Report

**Finding:** R2-11.3 (NFR-OPS-003) — deliverables review, LNet, 2026-06-04
**Scope:** `cbweb3-platform`, both scenarios, full repository and full commit history
**Tool:** gitleaks 8.28.0 (MIT), default ruleset extended by the repository config
**Configuration:** [`.gitleaks.toml`](../../.gitleaks.toml) · [`.gitleaksignore`](../../.gitleaksignore)
**CI gate:** [`.github/workflows/gitleaks.yml`](../../.github/workflows/gitleaks.yml)

## 1. Why this report exists

The review found that the claim "no keys in source" was asserted without evidence: the repository ran `gosec` (Go SAST) and Slither (Solidity) but had no secret-scan job. The correction required is twofold — produce a scan report as a D11 artifact, and wire the scan into CI as a gate. This document is the first half; the workflow above is the second.

## 2. Method

Two scans, both gating, both run on every push and pull request to `develop` and `main`:

| Scan | Command | Catches |
| --- | --- | --- |
| Working tree | `gitleaks detect --no-git --source .` | A secret present in the source as it stands |
| Commit history | `gitleaks git .` | A secret committed at any point and later deleted |

The history scan requires a full clone (`fetch-depth: 0`). Both use `--redact`, so no secret value is ever written to a build log, and `--exit-code 1`, so a detection fails the job.

The scanner binary is version-pinned and checksum-pinned: the release archive's SHA-256 is verified before extraction, so a replaced release asset fails the job rather than running as the scanner.

### 2.1 Running the scan locally

`gitleaks detect --no-git` does **not** honour `.gitignore`. Run directly in a working checkout it therefore also scans local runtime state — `samples/cbweb3-data/`, `deploy-lnet/bundles/`, rendered `.env.infra` files, generated PKI keys — and reports findings for files that are not in the repository and never will be. During this exercise a developer machine produced 81 such findings across 32 files, every one of them untracked and gitignored.

That is noise, not a gate failure. CI is unaffected because `actions/checkout` produces a clean tree. To reproduce what CI does, scan an export rather than the working directory:

```bash
git archive HEAD | tar -x -C /tmp/scan && cd /tmp/scan
gitleaks detect --no-git --source . --config .gitleaks.toml --redact --exit-code 1
```

These generated paths are deliberately **not** added to the allowlist. They are already excluded from the repository by `.gitignore`, and exempting them by path would silently suppress a real secret if such a file were ever force-added.

## 3. Result

**Working tree: clean.** Zero findings across the repository at the scanned revision.

**Commit history: 46 findings, 11 commits, 8 files — all triaged, none requiring rotation.**

Every one of the eight files is **absent from HEAD**. No finding corresponds to a credential that authenticates against any production or homologation (HML) environment. No key material in the deployed surface — the three deploy paths listed in [§5](#5-deployed-surface) — appears in the scan.

### 3.1 Findings by category

| Category | Count | Assessment |
| --- | --- | --- |
| EVM contract addresses in documentation and a generated join bundle | 34 | Not secrets. Blockchain addresses are public by construction; the generic entropy rule reacts to their hex. |
| Local Keycloak JWTs and one HTLC preimage in a sample walkthrough | 8 | Local-only, expired, non-reusable. See 3.2. |
| Shell syntax misread as high-entropy values | 4 | Not secrets. 13- and 21-character strings adjacent to empty `SPOKE_*_TOKEN=""` assignments. |

### 3.2 Items given individual triage

Four distinct values were credential-shaped and were examined individually. Values are deliberately not reproduced here; the evidence below is drawn from surrounding context and, for the tokens, from their own unencrypted claims.

**Two Keycloak JWTs** — `scenario-a/portals.md`, commits `719f5ca5` and `c7804f73`, 2026-07-02.
Both carry `iss: http://cbweb3-bank-itau-keycloak:8080/realms/bank-itau`. That host is a Docker Compose container alias, resolvable only from inside a local sample network and not routable from anywhere else. The access token had a five-minute lifetime and the refresh token thirty minutes; both expired on 2026-07-02, within an hour of issuance. They authenticate against nothing that exists outside a developer machine, and they no longer authenticate at all.

**One HTLC preimage** — same file and commits.
Matched because it sits in a JSON body under the key `"secret"`. It is the preimage of a hash-locked contract in a local demo payment, captured from a browser request whose `Referer` is `http://localhost:25646` — the sample bank portal port. An HTLC preimage is revealed to the counterparty by protocol design when the lock is claimed; it is not a credential, and it belongs to a contract on a local chain that no longer exists.

**One Besu operator key** — `backend/docker-compose-backend.bank-c.yaml` and `…bank-d.yaml`, commits `715729ab` and `797cfccd`, 2026-03-27 (same key in both files).
The service that consumes it targets `PALADIN_URL: http://host.docker.internal:31668` with `PALADIN_IDENTITY: funded_operator@spoke-a-bank-c`, and sources contract addresses from `deploy/local/paladin/spoke-a/.deployed-addrs.env`. `host.docker.internal` resolves to the developer's own machine. This is a funded operator account on an ephemeral local Besu chain created by the superseded `make *.up` flow. It has never been used in production or HML. The file path predates the scenario split and the file is absent from HEAD.

**Conclusion of triage:** no rotation was required for any finding, and no history rewrite is needed.

## 4. Accepted findings and how they are suppressed

The 46 history findings reduce to 44 distinct fingerprints, recorded in [`.gitleaksignore`](../../.gitleaksignore) with a per-file rationale.

The suppression mechanism is a fingerprint list, not a baseline report file. This is deliberate: a gitleaks baseline is a JSON report that, unless redacted, embeds the secret values themselves — committing one would place the very strings under review into the tree. A fingerprint is `commit:file:rule:line` and carries no value.

The list narrows rather than widens the gate:

- A fingerprint pins both a commit and a line, so a **new** secret fails even when added to a file that already has accepted entries.
- Findings reachable from HEAD are never suppressed this way. The rule recorded in the file is to fix the source instead.

## 5. Deployed surface

Exemptions in `.gitleaks.toml` are justified against what is actually deployed, which is three scripts:

| Path | Role |
| --- | --- |
| `scenario-a/samples/deploy-all.sh` | Single-host Scenario A samples, via the `cbweb3` CLI |
| `scenario-b/samples/deploy-all.sh` | Single-host Scenario B samples, via the `cbweb3b` CLI |
| `deploy-lnet/deploy.sh` | Multi-host LNET deploy, both scenarios |

Each allowlist entry is tagged **LIVE** (read by one of those paths, by the toolkit templates they render, or by the test suites they invoke) or **PROVISIONAL** (read by none of them; a leftover of the superseded `make *.up` flow). The provisional entries — `deploy/local/` node keys and the `spk-0*` spike genesis — are marked for deletion rather than permanent exemption; removal is currently blocked only by `scenario-{a,b}/make/*.mk` still referencing them.

Production key material is never committed. It is delivered at runtime through the environment or a secret manager, and the provisioning toolkit mints per-entity keys in memory (`KeyProvider` / `CertSource`) without persisting them.

The handling rules those exemptions are measured against — the no-key-in-VCS principle, the per-environment key sources, and the `KeyProvider` path to HSM-backed signing — are in [`docs/secret-management.md`](../secret-management.md) (finding R2-10.4). Note in particular the caveat recorded there: only the local tier is selectable today, so the LNET deploy runs on public dev keys by design.

## 6. Gate verification

The gate was verified by control rather than by observing a green check, since a passing scan proves nothing on its own about whether the scanner would catch anything.

| Control | Expected | Result |
| --- | --- | --- |
| Working tree, unmodified | pass | pass |
| `develop` tree with this configuration | pass | pass |
| AWS key planted in a Markdown file | **fail** | fail |
| PEM private key planted in the tree | **fail** | fail |
| 64-hex key planted in a non-example env file | **fail** | fail |
| History, unmodified, with the ignore list | pass | pass |
| New commit containing a secret | **fail** | fail |
| New secret added to a file that has accepted entries | **fail** | fail |

Two configuration forms were rejected during implementation because gitleaks 8.28.0 accepts them silently without applying them: `targetRules`, and `condition = "AND"` combined with `paths` and `regexes`. The latter degrades to OR and would have blanket-exempted an entire file from every rule. It was caught only by the planted-key control. Both are documented in `.gitleaks.toml` so they are not reintroduced.

## 7. Residual scope

Two items are outside what a repository change can deliver:

1. **The check is not merge-blocking.** On this repository's GitHub plan, `GET /branches/{develop,main}/protection` returns 404 and `/rulesets` returns 403 ("Upgrade to GitHub Pro"), so neither classic branch protection nor rulesets can mark `gitleaks secret scan` as required. The gate runs and fails correctly, but nothing yet prevents a merge over a red check. This needs a plan decision by the project lead.
2. **No CODEOWNERS.** A single pull request can currently weaken `.gitleaks.toml` or `.gitleaksignore` alongside the secret it hides. Both paths, plus `.github/workflows/`, should be owned so that such a change requires review.

Reviewers should also treat as change-requiring any diff that weakens the gate itself: a new exclusion path, `continue-on-error` or `|| true` on a scan step, a lowered `fetch-depth`, or a fingerprint added for a finding that is reachable from HEAD.
