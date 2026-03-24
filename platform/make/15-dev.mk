# ---------------------------------------------------------------------------
# dev.* — one-shot shortcuts for local development
#
# Usage:
#   make dev.up            → PKI + full environment (all nodes + contracts + backends)
#   make dev.down          → stop full environment in reverse order
#   make dev.up-spoke-a    → PKI + full Spoke-A stack (infra + besu + backend)
#   make dev.down-spoke-a  → stop full Spoke-A stack in reverse order
#   make dev.up-spoke-b    → PKI + full Spoke-B stack (infra + besu + backend)
#   make dev.down-spoke-b  → stop full Spoke-B stack in reverse order
#   make dev.up-hub        → PKI + full Hub stack (infra + besu + backend)
#   make dev.down-hub      → stop full Hub stack in reverse order
#
# PKI generation is idempotent: files are skipped if they already exist.
# Run with FORCE=1 to regenerate (e.g. make dev.up-spoke-a FORCE=1).
# ---------------------------------------------------------------------------

## Start the full environment (all domains) in the correct order:
##   1. PKI for hub, spoke-a, spoke-b and all commercial banks
##   2. shared docker network + infra (Keycloak, Postgres) — waits for readiness
##   3. Hub + Spoke-A + Spoke-B Besu nodes
##   4. Deploy and sync all smart contracts
##   5. Hub + Spoke-A + Spoke-B backend services
dev.up: pki.gen-all deploy.up contracts.deploy-all-with-sync deploy.up-backend-domains

## Stop the full environment in reverse order (PKI files are left intact)
dev.down: deploy.down-backend-domains deploy.down

## Start the full Spoke-A environment in the correct order:
##   1. Central Bank CA for Spoke-A  (spoke-a-ca.key / spoke-a-ca.crt)
##   2. Commercial bank PKI bundles   (bank-001 … bank-006)
##   3. shared docker network + infra (Keycloak, Postgres) — waits for readiness
##   4. Spoke-A Besu node
##   5. Spoke-A backend services      (api-gateway, auth, compliance, …)
dev.up-spoke-a: pki.gen-spoke-a pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-spoke-a

## Stop the full Spoke-A environment in reverse order (PKI files are left intact)
dev.down-spoke-a: deploy.down-backend-spoke-a deploy.down-spoke-a deploy.down-infra

## Start the full Spoke-B environment in the correct order:
##   1. Central Bank CA for Spoke-B  (spoke-b-ca.key / spoke-b-ca.crt)
##   2. Commercial bank PKI bundles   (bank-001 … bank-006)
##   3. shared docker network + infra (Keycloak, Postgres) — waits for readiness
##   4. Spoke-B Besu node
##   5. Spoke-B backend services      (api-gateway, auth, compliance, …)
dev.up-spoke-b: pki.gen-spoke-b pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-b deploy.up-backend-spoke-b

## Stop the full Spoke-B environment in reverse order (PKI files are left intact)
dev.down-spoke-b: deploy.down-backend-spoke-b deploy.down-spoke-b deploy.down-infra

## Start the full Hub environment in the correct order:
##   1. Central Bank CA for Hub  (hub-ca.key / hub-ca.crt)
##   2. shared docker network + infra (Keycloak, Postgres) — waits for readiness
##   3. Hub Besu node
##   4. Hub backend services      (api-gateway, auth, compliance, …)
dev.up-hub: pki.gen-hub deploy.up-infra deploy.up-hub deploy.up-backend-hub

## Stop the full Hub environment in reverse order (PKI files are left intact)
dev.down-hub: deploy.down-backend-hub deploy.down-hub deploy.down-infra

.PHONY: dev.up dev.down dev.up-spoke-a dev.down-spoke-a dev.up-spoke-b dev.down-spoke-b dev.up-hub dev.down-hub
