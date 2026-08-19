# Changelog

All notable changes to this repository are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Component-level detail for the platform is kept in `platform/CHANGELOG.md`.

## [Unreleased]

### Added

- `SECURITY.md` — responsible disclosure policy and coordinated disclosure timeline
- `MAINTAINERS.md` — authoritative maintainer list referenced by `CONTRIBUTING.md`
- `NOTICE` — Apache-2.0 attribution notice
- `AGENTS.md` — project conventions for AI coding tools, per the draft LFDT AI Guidelines
- `CHANGELOG.md` — this file
- Pull request template and Dependabot configuration
- Working documentation site: the `Documentation` workflow builds `mkdocs --strict` on
  every pull request and publishes to GitHub Pages from `main`
- `docs/research-papers/index.md` — index of the commissioned research

### Fixed

- The `Documentation` workflow was an empty file; the site had never been published
- `overrides/main.html` was empty, which suppressed the Material theme and would have
  rendered blank pages
- Broken theme asset references in `mkdocs.yml`: the logo pointed at
  `assets/CBWeb3Negros.png` while the file is `assets/CBweb3Negros.png`, and the favicon
  pointed at `assets/project-icon.png`, which does not exist
- Site navigation was almost entirely commented out; only the landing page was reachable
- `docs/index.md` referred to LACChain/LACNet where the project now uses LNet
- `.gitignore` did not exclude Python bytecode caches, and listed `localdocs/` twice
- Toolbox CI ran only on pull requests, so `main` could drift without verification

### Changed

- `mkdocs.yml` no longer loads the `mike` versioning plugin. No versioned documentation
  has ever been deployed, so the version selector would have resolved to nothing.
  Reinstate it together with a versioned deployment when the first release is cut.
