# Prerequisites

Software requirements for working with the CBWeb3 Toolbox sandbox.

---

## Required

| Tool | Version | Purpose | Install |
|------|---------|---------|---------|
| **Node.js** | 18+ | Run Prism mock server and Spectral linter | [nodejs.org](https://nodejs.org/) |
| **npm** | 9+ | Install Node packages (comes with Node.js) | Bundled with Node.js |
| **Python** | 3.9+ | Run conformance tests | [python.org](https://www.python.org/) |
| **pip** | Latest | Install Python packages | Bundled with Python |
| **curl** | Any | Execute API calls in tutorials | Pre-installed on most systems |
| **Git** | 2.30+ | Clone the repository | [git-scm.com](https://git-scm.com/) |

## Optional

| Tool | Purpose | Install |
|------|---------|---------|
| **httpie** | Friendlier alternative to curl | `pip install httpie` |
| **jq** | Format JSON output in terminal | `apt install jq` / `brew install jq` |
| **Docker** | Future: run Besu devnet locally | [docker.com](https://www.docker.com/) |

---

## Setup

### 1. Clone the repository

```bash
git clone https://github.com/LF-Decentralized-Trust-labs/cbweb3.git
cd cbweb3
```

### 2. Install Node.js tools (no global install needed)

All Node.js tools are invoked via `npx`, which downloads them on first use:

```bash
# Test that Prism works
npx @stoplight/prism-cli --version

# Test that Spectral works
npx @stoplight/spectral-cli --version
```

### 3. Install Python dependencies

```bash
pip install pytest requests
```

### 4. Verify everything works

```bash
# Lint all three OpenAPI contracts
for spec in Toolbox/contracts/*/openapi_*.yaml; do
  npx @stoplight/spectral-cli lint "$spec"
done

# List conformance tests (without running them)
cd Toolbox/conformance && pytest --co -q && cd ../..
```

All three contracts — `auth`, `pvp` and `amm` — lint clean with no findings at any
severity. If Spectral reports anything, the contract has drifted and that is a bug.

If both commands succeed, you're ready to proceed to [mock-server-setup.md](mock-server-setup.md).
