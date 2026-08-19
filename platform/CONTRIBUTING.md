# Contributing to cbweb3-platform

Thanks for your interest in contributing. This is a research platform for CBDC
interoperability, developed as a Digital Public Good. Contributions are welcome
under the project's [Apache-2.0 License](./LICENSE).

## Project structure

Two independent scenarios live side by side and are treated as separate products:

- `scenario-a/` — Enhanced Correspondent Banking (dual-layer HTLC)
- `scenario-b/` — International Hub (FXAgreement + AMM + LiquidityCommitRegistry)

Do **not** share code across `scenario-a/` ↔ `scenario-b/` except via an
explicitly versioned shared library. A PR that touches both scenarios needs
explicit justification in its description.

Authoritative architecture and process rules live in
`.specify/memory/constitution.md`. Read it before non-trivial changes.

## Branch workflow

- Create **feature branches** off `develop` (or a scenario integration branch).
- Open pull requests against `develop`.
- `main` is **merge-only via PR**; never push directly.
- Use conventional, descriptive branch names, e.g. `feat/...`, `fix/...`,
  `chore/...`, `docs/...`.

## Toolchain

The required tool versions are platform-wide — Scenario A and Scenario B share one floor —
and are listed in [`docs/TOOLCHAIN.md`](docs/TOOLCHAIN.md). In short: **Go 1.26**,
**Node 22 LTS**, npm 10, Docker 24 with Compose v2, GNU Make 3.81, Foundry (nightly), `jq`,
`openssl` 3.x, and k6 0.50 for the performance suites.

Each floor is enforced somewhere that fails the build — the `go` directive in every
`go.mod`, the `golang:1.26-alpine` and `node:22-alpine` builder images, the root `.nvmrc`,
the `engines` field in each `package.json`, and CI. If you raise a version, raise it in all
of those places and in `docs/TOOLCHAIN.md` in the same PR. A module that needs a different
version needs a row in that file's Recorded deviations table explaining why; without one,
reviewers should treat the split as drift and request a change.

## Before you open a PR

- **Test-first, every layer.** Foundry (`forge`) for contracts, `go test` for
  services, E2E suites per scenario. Add a failing test before implementation.
- Run the relevant checks:
  - Contracts: `make contracts.build`, `make contracts.test`, `make contracts.fmt`
  - Backend: `make scenario-b.test-backend` (or per-service `make test.<service>`)
  - Format Go with `gofmt`; lint Solidity with `make contracts.lint`.
- Keep changes scenario-isolated unless cross-scenario scope is justified.
- Do not weaken compliance or security checks; such PRs require project-lead
  approval.

## Licensing of contributions

- All first-party source files must carry an SPDX header:
  - Solidity: `// SPDX-License-Identifier: Apache-2.0` as the first line.
  - Go: `// SPDX-License-Identifier: Apache-2.0` as the first line.
- By submitting a contribution you agree it is licensed under Apache-2.0
  (see the License's Section 5, Submission of Contributions).
- Do not add SPDX headers to vendored or generated code (`*.pb.go`,
  `*_gen.go`, anything under `vendor/` or `node_modules/`, generated mocks).

## Commit messages

Use clear, imperative commit messages. Conventional Commit prefixes
(`feat:`, `fix:`, `chore:`, `docs:`, `test:`) are encouraged.

## Code of conduct

Be respectful and constructive. Assume good faith and keep discussions focused
on the technical and policy goals of the platform.
