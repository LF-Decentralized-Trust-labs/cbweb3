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
