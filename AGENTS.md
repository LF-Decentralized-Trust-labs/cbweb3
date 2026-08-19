# AGENTS.md

Conventions for AI coding tools and agents working in this repository, following the
vendor-neutral [agents.md](https://agents.md/) standard and §8 of the draft
[LFDT AI Guidelines](https://github.com/LF-Decentralized-Trust/governance/pull/321).

This file is for machines and for the humans who direct them. It does not replace
[CONTRIBUTING.md](CONTRIBUTING.md), which governs participation and decision-making.

## Project context

CBWeb3 is an IDB Lab project building a regional test network for Latin America and the
Caribbean that enables the issuance of tokenized central bank money (tCeBM) and the
tokenization of financial assets, with cross-border interoperability between central banks
and financial institutions.

The architecture is **Hub & Spoke**: each country operates a domestic Hyperledger Besu
network (Spoke), connected through a shared transnational settlement layer (Hub) via
Hyperledger Cacti, with Hyperledger Paladin providing on-chain privacy.

This is financial market infrastructure operated by central banks and supervisors. Changes
to settlement logic, authorisation, or privacy guarantees carry real consequences.

## Repository layout

| Path | What it is | Who owns it |
|---|---|---|
| `Toolbox/` | Community integration kit: interface contracts, reference mocks, test vectors, conformance tests, sandbox tutorials | DPG Working Group |
| `docs/` | DPG assessment, research papers, technical blueprint, published documentation site | LNet |
| `.github/workflows/` | CI — path-filtered per area | joint |
| `platform/` | CBWeb3 system code — smart contracts, backend services, API gateway, frontend portals, deployment | AguilaHub / GoLedger *(not yet imported)* |

## Rules

### Disclosure and authorship

- **Disclose AI assistance.** Add an `Assisted-by:` trailer to the commit body:

  ```
  Assisted-by: anthropic:claude-opus-5
  ```

  Format is `PROVIDER:MODEL_VERSION [TOOL1] [TOOL2]`, provider in `kebab-case`.
  A bare `Assisted-by: an LLM` is acceptable.

- **Never list an AI as an author or co-author.** Do not add
  `Co-authored-by: Claude`, `Co-authored-by: Copilot`, `Co-authored-by: Cursor` or
  equivalent. Credit belongs to the humans who prompted, reviewed and submitted the work.

- **Never add `Signed-off-by` on a human's behalf.** Only a human can certify the
  [Developer Certificate of Origin](https://developercertificate.org/). DCO sign-off is
  enforced by an organisation ruleset; the human submitter runs `git commit -s`.

- **AI cannot approve a pull request** or take part in governance decisions.

### Review and accountability

- All AI-generated output is reviewed by a human before submission. AI review does not
  by itself constitute a code review.
- The contributor who submits a pull request is accountable for its correctness,
  security, licensing compliance and long-term maintenance, whatever produced it.
- Be prepared to explain any line of your change from your own understanding.

### Pull requests

- Keep pull requests focused and reviewable. AI tooling makes large changes cheap to
  produce and expensive to review — prefer several small pull requests to one large one.
- Every source file carries `SPDX-License-Identifier: Apache-2.0`.
- Never commit secrets, private keys, credentials, real institutional data, or anything
  derived from the participating banks' data. This repository is public.

## Conventions

| Area | Convention |
|---|---|
| Interface contracts | OpenAPI 3.0.3, linted with Spectral (`.spectral.yml`) |
| Mocks and test vectors | JSON, validated against `Toolbox/schemas/*.schema.json` |
| Conformance tests | Python + pytest, markers declared in `Toolbox/conformance/pytest.ini` |
| Documentation | Markdown, built with MkDocs Material; the site must build with `--strict` |
| Commit messages | Imperative mood, one concern per commit, `git commit -s` |

## Out of scope without maintainer approval

Do not modify these without an explicit request from a maintainer:

- `LICENSE`, `NOTICE`, `CODE_OF_CONDUCT.md`, `CODEOWNERS`, `MAINTAINERS.md`
- `CONTRIBUTING.md` and the governance model it defines
- `Toolbox/contracts/**` — published interface contracts other implementers build
  against. Changes are breaking by default and require a version bump plus updates to
  mocks, test vectors and conformance tests
- `.github/workflows/**` — CI is a merge gate
- `docs/dpg-assesment/**` — evidence for the DPG submission

## Verifying your work

```bash
# Documentation site (must pass --strict)
pip install -r requirements.txt && mkdocs build --strict

# Toolbox interface contracts
spectral lint Toolbox/contracts/*/openapi_*.yaml --fail-severity=error

# Toolbox mocks and test vectors
ajv validate -s Toolbox/schemas/mock.schema.json   -d "Toolbox/mocks/**/*.json"        --spec=draft2020
ajv validate -s Toolbox/schemas/vector.schema.json -d "Toolbox/test-vectors/**/*.json" --spec=draft2020

# Conformance tests against a mock server
prism mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010 &
cd Toolbox/conformance && pytest tests/ -m happy_path --base-url=http://localhost:4010
```
