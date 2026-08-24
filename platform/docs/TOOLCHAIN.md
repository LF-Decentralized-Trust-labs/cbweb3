# Toolchain — required versions

> **Authoritative.** This file is the single source of truth for the versions of every
> tool required to build, test and deploy the CBWeb3 platform. Prerequisite tables in
> `README.md`, the scenario READMEs and the runbooks restate these values and must not
> diverge from them. When a version changes, change it here first, then propagate.

The floor is **the same for Scenario A and Scenario B**. The two scenarios are separate
products (see `.specify/memory/constitution.md`), but they are built by the same people on
the same machines, so a per-scenario toolchain split costs more than it buys. Any
deviation must be recorded in [Recorded deviations](#recorded-deviations) with a reason.

---

## Required tools

| Tool | Minimum version | Purpose |
|------|-----------------|---------|
| **Docker** | 24.x | Container runtime for every service |
| **Docker Compose** | v2 (plugin) | Multi-container stack management (`docker compose`, never `docker-compose`) |
| **GNU Make** | 3.81 | Build automation (`make` targets) |
| **Go** | **1.26** | Backend services, both provisioning toolkits (`cbweb3` / `cbweb3b`), Paladin scripts, Go test suites |
| **Node.js** | **22 LTS** | Frontend applications (React/Vite), the Cacti relay, the launcher |
| **npm** | 10 | JavaScript package management |
| **Foundry** (`forge`, `cast`) | nightly | Solidity compilation, testing, deployment |
| **jq** | 1.6 | JSON processing in shell scripts |
| **openssl** | 3.x | PKI certificate generation (EC prime256v1) |
| **k6** | 0.50 | Load and performance tests (`make <scenario>.perf-baseline`) — needed only to run the perf suites |
| **bash** | 5+ | The deployment and check scripts. Run `tools/check-license-headers.sh` with **bash**, never zsh |

Pinned container images are not developer prerequisites — they are pulled automatically:
`hyperledger/besu:25.8.0`, Paladin Core (project-pinned), Keycloak, Postgres, Redis.

---

## Where each version is enforced

A documented floor that nothing checks drifts again within a release. Each row below is
the mechanism that actually fails the build when the floor is violated.

| Version | Enforced by | Notes |
|---------|-------------|-------|
| Go 1.26 | `go` directive in **every** `go.mod` (27 modules, both scenarios) | Uniform. `go build` refuses an older toolchain; with `GOTOOLCHAIN=auto` (the default) a newer toolchain is fetched on demand |
| Go 1.26 | `FROM golang:1.26-alpine` in every backend service `Dockerfile` | The builder image must not be older than the `go` directive, or container builds break |
| Go 1.26 | CI: `actions/setup-go` with `go-version-file: <module>/go.mod` | Derived, never hardcoded — CI follows `go.mod` automatically |
| Node 22 | `.nvmrc` at the repository root | `nvm use` in any subdirectory resolves to it |
| Node 22 | `"engines": { "node": ">=22", "npm": ">=10" }` in the frontend, relay and launcher `package.json` files | Machine-readable floor; add `engine-strict=true` to an `.npmrc` to make it fatal rather than a warning |
| Node 22 | `FROM node:22-alpine` in every frontend, relay and launcher `Dockerfile` | Frontends and relays now share one runtime |
| Node 22 | `@types/node: ^22.x` in every `package.json` that declares it | Types must describe the runtime that actually executes the code, not a newer or older one |
| Node 22 | CI: `actions/setup-node` with `node-version: "22"` | |
| Besu 25.8.0 | `BESU_IMAGE` default in the compose templates | Pinned; not a developer prerequisite |
| Alpine 3.23 | `alpine:3.23` everywhere: the shipped runtime `Dockerfile` stages, the toolkit helper-image constants (`dockervolume.HelperImage` in Scenario A, `volHelperImage` in Scenario B) and the helper `docker run`/compose services | Gated by `tools/check-alpine-version.sh`, which reads this row as the pin. Supported until 2027-11-01 |

Verify the whole matrix is still self-consistent:

```bash
# every module declares the same Go version
find . -name go.mod -not -path '*/node_modules/*' -not -path '*/vendor/*' \
  -exec grep -H '^go ' {} \; | awk '{print $2}' | sort -u        # expect one line: 1.26

# every Go builder image matches
grep -rh 'FROM golang:' --include='Dockerfile*' scenario-a scenario-b \
  | grep -v vendor | sort -u                                     # expect one line: 1.26-alpine

# every Node image matches
grep -rh 'FROM node:' --include='Dockerfile*' . | sort -u        # expect one line: 22-alpine

# every Alpine reference matches the pin in the table above
bash tools/check-alpine-version.sh                               # expect: OK, one version
```

Alpine is the one image pinned by a checker rather than by eye, because its references
are spread across four kinds of surface — runtime `Dockerfile` stages, Go constants in
both toolkits, `docker run` invocations in shell scripts, and compose helper services —
and a `grep` for `FROM alpine:` finds only the first kind. Bumping it means editing the
row above and running the checker, which lists every reference still on the old value.

---

## Installation

### macOS

```bash
brew install go node jq openssl make k6

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Docker Desktop — https://www.docker.com/products/docker-desktop/
# npm ships with Node.js
```

Homebrew's `go` and `node` track the latest stable release, which satisfies the floor.
If you pin versions with `nvm`, run `nvm use` from the repository root to pick up `.nvmrc`.

### Linux (Ubuntu/Debian)

```bash
# Go 1.26
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Node.js 22 via nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
nvm install 22 && nvm use 22

# Tools
sudo apt-get install -y jq openssl make

# Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# k6 — https://grafana.com/docs/k6/latest/set-up/install-k6/

# Docker Engine — https://docs.docker.com/engine/install/ubuntu/
```

### Verification

```bash
docker --version          # 24.x or newer
docker compose version    # v2.x
make --version            # 3.81 or newer
go version                # go1.26.x or newer
node --version            # v22.x
npm --version             # 10.x or newer
forge --version
jq --version
openssl version
k6 version                # only needed for the perf suites
```

---

## Docker resources

The two scenarios differ in container count, so their resource guidance differs. This is
a sizing recommendation, not a toolchain version, and it is expected to differ.

| Scenario | Containers (approx.) | RAM minimum | RAM recommended | CPUs | Disk |
|----------|----------------------|-------------|-----------------|------|------|
| Scenario A | ~30 | 8 GB | 16 GB | 4 (8 recommended) | 20 GB free |
| Scenario B | ~35 | 12 GB | 16 GB | 4 (8 recommended) | 25 GB free |

Running both scenarios at once needs the sum of the two.

---

## Recorded deviations

| Deviation | Scope | Reason |
|-----------|-------|--------|
| _(none)_ | — | Go and Node floors are currently identical across both scenarios and every module. |

To add a row here: state the module or scenario, the version it pins, and why the unified
floor does not work for it. A deviation with no entry in this table is drift, and reviewers
should treat it as a change request.

---

## Related

- [`CONTRIBUTING.md`](../CONTRIBUTING.md) — branch workflow and pre-PR checks
- [`scenario-a/docs/runbooks/environment-setup.md`](../scenario-a/docs/runbooks/environment-setup.md) — Scenario A port reference
- [`scenario-b/docs/runbooks/environment-setup.md`](../scenario-b/docs/runbooks/environment-setup.md) — Scenario B port reference
- [`docs/DPG-COMPLIANCE.md`](DPG-COMPLIANCE.md) — licence-header obligation enforced in CI
