# ---------------------------------------------------------------------------
# dev.* — one-shot shortcuts for local development (spoke-a only)
#
# Topology: bank-a, bank-b and central-bank all reside in spoke-a,
#           sharing a single Besu network (chain 1338), one Keycloak
#           instance (3 realms), one Postgres (3 databases) and one
#           Redis (3 logical DBs: 0, 1, 2).
#
# Usage:
#   make dev.up            → PKI + full environment (besu + infra + all entities + contracts)
#   make dev.down          → stop full environment in reverse order
#   make dev.up-bank-a     → PKI + infra + besu + bank-a backend
#   make dev.down-bank-a   → stop bank-a backend (infra/besu left running)
#   make dev.up-bank-b     → PKI + infra + besu + bank-b backend
#   make dev.down-bank-b   → stop bank-b backend (infra/besu left running)
#   make dev.up-central-bank → PKI + infra + besu + central-bank backend
#   make dev.down-central-bank → stop central-bank backend (infra/besu left running)
#
# PKI generation is idempotent: files are skipped if they already exist.
# Run with FORCE=1 to regenerate (e.g. make dev.up FORCE=1).
# ---------------------------------------------------------------------------

## Start the full spoke-a environment in the correct order:
##   1. PKI for central-bank, bank-a, bank-b and all commercial banks
##   2. shared docker network + infra (Keycloak, Postgres, Redis) — waits for readiness
##   3. Spoke-A Besu node
##   4. Deploy and sync all smart contracts
##   5. All entity backend services (bank-a, bank-b, central-bank)
dev.up: pki.gen-all deploy.up contracts.deploy-all-with-sync deploy.up-backend-entities

## Stop the full spoke-a environment in reverse order (PKI files are left intact)
dev.down: deploy.down-backend-entities deploy.down

## Start the Bank-A entity environment:
##   1. Bank-A CA  (bank-a-ca.key / bank-a-ca.crt)
##   2. Commercial bank PKI bundles (bank-001 … bank-006)
##   3. shared docker network + infra (Keycloak, Postgres, Redis) — waits for readiness
##   4. Spoke-A Besu node
##   5. Bank-A backend services (api-gateway, auth, compliance)
dev.up-bank-a: pki.gen-bank-a pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-bank-a

## Stop the Bank-A entity backend (infra and Besu are left running)
dev.down-bank-a: deploy.down-backend-bank-a

## Start the Bank-B entity environment:
##   1. Bank-B CA  (bank-b-ca.key / bank-b-ca.crt)
##   2. Commercial bank PKI bundles (bank-001 … bank-006)
##   3. shared docker network + infra (Keycloak, Postgres, Redis) — waits for readiness
##   4. Spoke-A Besu node
##   5. Bank-B backend services (api-gateway, auth, compliance)
dev.up-bank-b: pki.gen-bank-b pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-bank-b

## Stop the Bank-B entity backend (infra and Besu are left running)
dev.down-bank-b: deploy.down-backend-bank-b

## Start the Central-Bank entity environment:
##   1. Central Bank CA  (central-bank-ca.key / central-bank-ca.crt)
##   2. shared docker network + infra (Keycloak, Postgres, Redis) — waits for readiness
##   3. Spoke-A Besu node
##   4. Central-Bank backend services (api-gateway, auth, compliance)
dev.up-central-bank: pki.gen-central-bank deploy.up-infra deploy.up-spoke-a deploy.up-backend-central-bank

## Stop the Central-Bank entity backend (infra and Besu are left running)
dev.down-central-bank: deploy.down-backend-central-bank

.PHONY: dev.up dev.down dev.up-bank-a dev.down-bank-a dev.up-bank-b dev.down-bank-b dev.up-central-bank dev.down-central-bank
