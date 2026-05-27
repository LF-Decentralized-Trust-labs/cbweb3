# cbweb3-platform Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-05-22

## Active Technologies
- Go 1.25.5 (payment-orchestrator), TypeScript 5.4 + Node 20 (relay Cacti), Solidity 0.8.20 (contratos) + gRPC/protobuf, go-ethereum v1.17.1, GORM + Postgres driver (padrão compliance), ethers v6, @grpc/grpc-js, Hyperledger Cacti packages, Paladin Pente client (001-harden-fx-agreement)
- PostgreSQL — `fx_agreements`, `fx_agreement_events`, relay durability tables; padrão GORM conforme compliance service (001-harden-fx-agreement)
- Go 1.25.5 (both services) (002-noc-monitoring-service)
- Dedicated PostgreSQL 15 container (`noc-db`). Tables prefixed `noc_`. See `data-model.md`. (002-noc-monitoring-service)

- Go 1.25.5 (payment-orchestrator), TypeScript 5.4 + Node 20 (relay Cacti), Solidity 0.8.20 (contracts). + gRPC/protobuf, go-ethereum, GORM + Postgres driver (padrao compliance), ethers v6, @grpc/grpc-js, Hyperledger Cacti packages. (feature/agreement-v2)

## Project Structure

```text
src/
tests/
```

## Commands

npm test && npm run lint

## Code Style

Go 1.25.5 (payment-orchestrator), TypeScript 5.4 + Node 20 (relay Cacti), Solidity 0.8.20 (contracts).: Follow standard conventions

## Recent Changes
- 002-noc-monitoring-service: Added Go 1.25.5 (both services)
- 001-harden-fx-agreement: Added Go 1.25.5 (payment-orchestrator), TypeScript 5.4 + Node 20 (relay Cacti), Solidity 0.8.20 (contratos) + gRPC/protobuf, go-ethereum v1.17.1, GORM + Postgres driver (padrão compliance), ethers v6, @grpc/grpc-js, Hyperledger Cacti packages, Paladin Pente client

- feature/agreement-v2: Added Go 1.25.5 (payment-orchestrator), TypeScript 5.4 + Node 20 (relay Cacti), Solidity 0.8.20 (contracts). + gRPC/protobuf, go-ethereum, GORM + Postgres driver (padrao compliance), ethers v6, @grpc/grpc-js, Hyperledger Cacti packages.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

<!-- rtk-instructions v2 -->
# RTK — Token-Optimized CLI

**rtk** is a CLI proxy that filters and compresses command outputs, saving 60-90% tokens.

## Rule

Always prefix shell commands with `rtk`:

```bash
# Instead of:              Use:
git status                 rtk git status
git log -10                rtk git log -10
cargo test                 rtk cargo test
docker ps                  rtk docker ps
kubectl get pods           rtk kubectl pods
```

## Meta commands (use directly)

```bash
rtk gain              # Token savings dashboard
rtk gain --history    # Per-command savings history
rtk discover          # Find missed rtk opportunities
rtk proxy <cmd>       # Run raw (no filtering) but track usage
```
<!-- /rtk-instructions -->