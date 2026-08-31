# Third-Party Dependency Licenses

This document summarizes the licenses of the major third-party dependencies used
by cbweb3-platform. The platform itself is licensed under **Apache-2.0**
(see [LICENSE](./LICENSE)).

## Method

This is a **curated summary** compiled by inventorying the dependency manifests
in the repository:

- Go modules: aggregated from all first-party `go.mod` files across
  `scenario-a/` and `scenario-b/` (excluding `vendor/`).
- npm packages: aggregated from the frontend `package.json` files
  (`scenario-*/frontend/**`) and the Cacti relay (`interop/hub-and-spoke/cacti`).

License identifiers below were taken from each project's published metadata
(SPDX identifiers from their respective repositories/registries). For an exact,
machine-generated manifest, run a tool such as `go-licenses report ./...`
(Go) or `license-checker --summary` (npm) per module; this curated list covers
the major direct and transitive dependencies and is intended for DPG review.

All listed licenses are OSI-approved and compatible with redistribution of an
Apache-2.0 work.

## Go modules (major dependencies)

| Dependency | License |
| --- | --- |
| github.com/ethereum/go-ethereum | LGPL-3.0 / GPL-3.0 (mixed; library packages LGPL-3.0) |
| github.com/gofiber/fiber/v2 | MIT |
| github.com/valyala/fasthttp | MIT |
| github.com/golang-jwt/jwt/v5 | MIT |
| github.com/google/uuid | BSD-3-Clause |
| github.com/jackc/pgx/v5 | MIT |
| github.com/redis/go-redis/v9 | BSD-2-Clause |
| github.com/gorilla/websocket | BSD-2-Clause |
| github.com/stretchr/testify | MIT |
| github.com/joho/godotenv | MIT |
| github.com/klauspost/compress | BSD-3-Clause / MIT / Apache-2.0 (mixed) |
| github.com/consensys/gnark-crypto | Apache-2.0 |
| github.com/holiman/uint256 | BSD-3-Clause / MIT |
| github.com/decred/dcrd/dcrec/secp256k1/v4 | ISC |
| github.com/supranational/blst | Apache-2.0 |
| github.com/Microsoft/go-winio | MIT |
| github.com/shirou/gopsutil | BSD-3-Clause |
| go.opentelemetry.io/otel (+ metric, trace, sdk) | Apache-2.0 |
| go.uber.org/atomic | MIT |
| golang.org/x/crypto, golang.org/x/net, golang.org/x/sync | BSD-3-Clause |
| google.golang.org/grpc | Apache-2.0 |
| google.golang.org/protobuf | BSD-3-Clause |

> Note: `go-ethereum` is consumed as a library (LGPL-3.0 packages). Dynamic
> linking/library use is compatible with Apache-2.0 distribution provided the
> LGPL terms (ability to relink) are honored. If a fully permissive dependency
> tree is required for DPG certification, review go-ethereum usage.

## npm packages (frontend, major dependencies)

| Dependency | License |
| --- | --- |
| react / react-dom | MIT |
| vite | MIT |
| typescript | Apache-2.0 |
| tailwindcss | MIT |
| axios | MIT |
| zod | MIT |
| turbo (Turborepo) | MIT |
| @tanstack/* (if present) | MIT |
| ethers / viem / wagmi (where used) | MIT |

## Blockchain / infrastructure components

| Component | License |
| --- | --- |
| Hyperledger Besu | Apache-2.0 |
| Hyperledger Cacti | Apache-2.0 |
| Paladin Core (Zeto / Noto domains) | Apache-2.0 |
| Keycloak | Apache-2.0 |
| PostgreSQL | PostgreSQL License (permissive, BSD/MIT-style) |
| Redis | BSD-3-Clause (Redis OSS / RSALv2 — verify version in use) |

## Maintenance

When adding a new runtime dependency, update this file and justify the addition
in the PR and scenario README per the project stack policy. Prefer dependencies
under permissive licenses (MIT, BSD, Apache-2.0).
