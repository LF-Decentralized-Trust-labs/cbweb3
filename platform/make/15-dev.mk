# ---------------------------------------------------------------------------
# dev.* — one-shot shortcuts for local development
#
# Usage:
#   make dev.up-spoke-a    → PKI + full Spoke-A stack (infra + besu + backend)
#   make dev.down-spoke-a  → stop full Spoke-A stack in reverse order
#
# PKI generation is idempotent: files are skipped if they already exist.
# Run with FORCE=1 to regenerate (e.g. make dev.up-spoke-a FORCE=1).
# ---------------------------------------------------------------------------

## Start the full Spoke-A environment in the correct order:
##   1. Central Bank CA for Spoke-A  (spoke-a-ca.key / spoke-a-ca.crt)
##   2. Commercial bank PKI bundles   (bank-001 … bank-006)
##   3. shared docker network + infra (Keycloak, Postgres) — waits for readiness
##   4. Spoke-A Besu node
##   5. Spoke-A backend services      (api-gateway, auth, compliance, …)
dev.up-spoke-a: pki.gen-spoke-a pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-spoke-a

## Stop the full Spoke-A environment in reverse order (PKI files are left intact)
dev.down-spoke-a: deploy.down-backend-spoke-a deploy.down-spoke-a deploy.down-infra

.PHONY: dev.up-spoke-a dev.down-spoke-a
