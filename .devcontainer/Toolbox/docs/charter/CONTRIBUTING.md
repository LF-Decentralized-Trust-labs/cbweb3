# Contributing to CBWeb3 Toolbox

Thank you for contributing. The CBWeb3 Toolbox is designed to be **integration-friendly**, **contract-first**, and **safe-to-publish** (non-sensitive by default). This guide describes how to propose, implement, and review changes.

> Docs and artifacts should be written in **English** to maximize community reuse.

---

## What to contribute

We welcome contributions across:
- **Contracts** (schemas / OpenAPI / ABIs / error models)
- **Reference mocks** (canonical simulated endpoints/services)
- **Test vectors** (deterministic fixtures)
- **Conformance tests** (verifiable compliance checks)
- **Sandbox guidance** (quickstarts, tutorials, non-sensitive sample configs)
- **Docs** (glossary, onboarding, FAQs)

If you’re unsure, start by opening an issue describing your proposal and expected outputs.

---

## Contribution workflow

### 1) Find or open an issue
- Check the repository’s **GitHub Project** and existing issues.
- Use labels to find the right area and priority:
  - `area:*` (contracts, mocks, test-vectors, conformance, sandbox, docs)
  - `prio:P0/P1/P2`
  - `needs-owner`

If creating a new issue, include:
- **Problem statement**
- **Proposed artifact(s)** (contract/mock/vector/test/doc)
- **Definition of Done**
- **Dependencies / assumptions**
- **Security considerations** (what could go wrong, data exposure, abuse cases)

### 2) Assign yourself (or comment intent)
- If you can take ownership, assign yourself.
- If you cannot assign, comment: “I can take this” + timeline + questions.

### 3) Create a branch and implement
Branch naming convention (recommended):
- `toolbox/<area>/<short-title>`  
Examples: `toolbox/contracts/pvp-errors`, `toolbox/test-vectors/pvp-timeout`

### 4) Open a Pull Request (PR)
Your PR must include:
- **What** changed
- **Why** (link the issue)
- **How it was validated** (local checks, sample runs, test evidence)
- **Security considerations** (inputs, auth boundaries, logging, secrets)
- **Breaking changes** (and migration notes)

PRs should be small and reviewable. If large, split into staged PRs.

---

## Quality requirements (must pass)

### Non-sensitive by default (hard requirement)
Do **not** commit:
- private keys, credentials, tokens
- real endpoints, internal IPs, confidential configs
- real participant data or logs containing sensitive values

If needed, use **synthetic** or **redacted** samples.

### Contract-first consistency
If you modify a contract:
- update examples and error model
- update test vectors (if applicable)
- update conformance tests (if applicable)
- add changelog notes (especially for breaking changes)

### Reproducibility
Artifacts must include clear “how to validate” steps:
- contracts: schema validation guidance
- mocks: run instructions + known limitations
- vectors: how to execute them
- conformance tests: how to run locally and expected outputs

---

## Review process

- Reviews follow **CODEOWNERS** rules (area owners must approve).
- Maintainership prioritizes:
  - clarity and correctness of specs
  - deterministic behavior (mocks/vectors/tests)
  - backward compatibility and explicit versioning
  - security posture (OWASP-minded hygiene)

If a PR is blocked, reviewers should request specific changes and/or propose a follow-up issue.

---

## Versioning and changelogs

- Prefer **semantic versioning** for artifacts that are consumed externally:
  - MAJOR: breaking changes
  - MINOR: backward-compatible additions
  - PATCH: backward-compatible fixes
- For meaningful changes, include a short changelog entry near the relevant artifact.

---

## Documentation style

- Use clear headings, short paragraphs, and explicit definitions.
- Define technical terms briefly (assume mixed audience: engineers + PMs).
- Prefer normative language in specs:
  - **MUST**, **SHOULD**, **MAY** (use sparingly and consistently)

---

## Security reporting

If you believe you found a vulnerability:
- **Do not open a public issue.**
- Use the repository’s security reporting process (see `SECURITY.md` if present) or contact maintainers privately.

---

## Code of conduct

All contributors are expected to follow the repository’s code of conduct (see `CODE_OF_CONDUCT.md` if present).

---

## Quick checklist (before you open a PR)

- [ ] I linked the PR to an issue in the Project board
- [ ] No secrets / sensitive data included
- [ ] Contracts updated consistently with examples and errors
- [ ] Test vectors and/or conformance tests updated if relevant
- [ ] Clear validation steps included
- [ ] Security considerations documented
