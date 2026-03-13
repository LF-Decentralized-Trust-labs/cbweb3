<div align="center">

# CBWeb3 Platform - Core Smart Contracts

[![Solidity](https://img.shields.io/badge/Solidity-%5E0.8.20-363636.svg?logo=solidity)](https://soliditylang.org/)
[![Framework](https://img.shields.io/badge/Foundry-Fast-e6522c.svg)](https://getfoundry.sh/)
[![Coverage](https://img.shields.io/badge/Coverage-%E2%89%A590%25-brightgreen.svg)]()
[![Security](https://img.shields.io/badge/Slither-0_Vulnerabilities-success.svg)]()
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

## 📖 Overview

This repository contains the core Solidity smart contracts for the **CBWeb3 Platform**, implemented as part of the **Deliverable 1 (Phase 1) Roadmap** and the **SDD Deliverable 7**.

The current implementation focuses on the foundational EVM logic execution, mathematical accuracy, state transitions, and security primitives required for cross-border settlement. It is designed to be deployed on a local **Hyperledger Besu** environment, acting as the foundation before the upcoming Hyperledger Paladin/Zeto privacy integrations.

## 🏗️ Core Architecture

The system is composed of three main architectural pillars, rigorously segregated into Data Libraries, Interfaces, and Implementation contracts to adhere to SOLID principles.

### 1. Tokenized Central Bank Money (`tCeBM`)

A standard ERC-20 implementation mocking the fiat-pegged assets (e.g., `tCeBM_BRL`, `tCeBM_EUR`).

- **Role-Based Access Control (RBAC):** Utilises OpenZeppelin's `AccessControl`.
- **Monetary Authority:** Only addresses holding the `CENTRAL_BANK_ROLE` can mint or burn the supply, strictly mirroring Central Bank capabilities on a domestic ledger.

### 2. Hash Time-Lock Contract (HTLC) - _Scenario A_

The cryptographic escrow engine facilitating trustless cross-border atomic swaps.

- **Finite State Machine (FSM):** Enforces strict state transitions (`INVALID` ➔ `LOCKED` ➔ `SETTLED` or `REFUNDED`).
- **Cryptographic Checks:** Validates the exact SHA-256 preimage before settling funds.
- **Time-Locks:** Prevents `refund()` execution until the exact timestamp expiry is reached.

### 3. Automated Market Maker (AMM) - _Scenario B_

A Constant Product Liquidity Pool ($x \cdot y = k$) enabling seamless foreign exchange (FX) settlement.

- **Exact-Output Pricing:** Calculates the precise input (`amountIn`) required to purchase an exact output (`amountOut`).
- **Slippage Protection:** Reverts transactions if the mathematically required input exceeds the payer's acceptable `maxAmountIn`.
- **Circuit Breaker:** Implements OpenZeppelin's `Pausable` modifier, allowing the `GOVERNANCE_ROLE` to halt all pool operations in emergency scenarios.

## 🛡️ Security & Standards

Security is a primary directive in this repository. The codebase strictly enforces:

- **Zero Warnings:** Compiles without warnings on `solc ^0.8.20`.
- **Reentrancy Protection:** OpenZeppelin's `ReentrancyGuard` applied to all external state-changing functions.
- **Asset Safety:** Uses `SafeERC20` wrappers for all token transfers to prevent silent failures.
- **Static Analysis:** CI/CD pipeline integrated with **Slither**, strictly enforcing a Zero (0) Critical and Zero (0) High vulnerability policy.

## ⚙️ Prerequisites

- [Foundry](https://book.getfoundry.sh/getting-started/installation) (Forge, Cast, Anvil, Chisel)
- [Docker](https://www.docker.com/) (Required for running the Slither static analysis suite)
- [Make](https://www.gnu.org/software/make/) (For executing Makefile commands)

## 🚀 Getting Started

### 1. Installation

Clone the repository and install the required Foundry dependencies using Soldeer:

```bash
git clone <repository_url>
cd cbweb3-platform
forge soldeer install
```

### 2. Environment Configuration

Copy the example environment file and populate it with your local Besu credentials and desired RBAC addresses.

```bash
cp .env.example .env
```

_Ensure the `.env` includes `DEPLOYER_PRIVATE_KEY`, `ADMIN_ADDRESS`, `CENTRAL_BANK_ADDRESS`, and `GOVERNANCE_ADDRESS`._

### 3. Build & Test

The repository enforces a strict **≥ 90% test coverage** policy across all smart contract logic.

```bash
# Compile the smart contracts
forge build

# Run the unit test suite
forge test

# Generate the coverage report
make contracts.coverage
```

### 4. Static Analysis (Slither)

To run the Trail of Bits `eth-security-toolbox` Docker container and validate the code against 100+ vulnerability detectors:

```bash
make contracts.slither
```

### 5. Local Deployment (Hyperledger Besu)

A master orchestration script (`DeployCBWeb3.s.sol`) is provided to deploy the entire core topology (Tokens, HTLC, and AMM) in a single transaction sequence.

```bash
# Load environment variables
source .env

# Execute the deployment script via Forge Broadcast
forge script script/DeployCBWeb3.s.sol:DeployCBWeb3 --rpc-url $BESU_RPC_URL --broadcast
```

## 📝 Upgradability Note

_Per the D1 specifications, upgradable proxies (UUPS/Transparent) are deferred and will be introduced in a subsequent architectural phase._
