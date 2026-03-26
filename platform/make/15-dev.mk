# dev.* — local development shortcuts
# PKI generation is idempotent (skipped if files exist). Use FORCE=1 to regenerate.

# ── Spoke-level targets (full stack) ─────────────────────────────────────────

spoke-a: pki.gen-central-bank-a pki.gen-bank-a pki.gen-bank-c pki.gen-commercial-banks \
         deploy.up-infra deploy.up-spoke-a \
         contracts.setup contracts.deploy-spoke-a contracts.sync-addresses \
         deploy.up-backend-spoke-a
	@echo "Spoke-A stack is up."

spoke-a-down: deploy.down-backend-spoke-a deploy.down-spoke-a
	@echo "Spoke-A stack is down (shared infra left running)."

spoke-b: pki.gen-central-bank-b pki.gen-bank-b pki.gen-bank-d pki.gen-commercial-banks \
         deploy.up-infra deploy.up-spoke-b \
         contracts.setup contracts.deploy-spoke-b contracts.sync-addresses \
         deploy.up-backend-spoke-b
	@echo "Spoke-B stack is up."

spoke-b-down: deploy.down-backend-spoke-b deploy.down-spoke-b
	@echo "Spoke-B stack is down (shared infra left running)."

# ── Full environment ─────────────────────────────────────────────────────────

dev.up: pki.gen-all deploy.up contracts.deploy-all-with-sync deploy.up-backend-entities
dev.down: deploy.down-backend-entities deploy.down

# ── Per-entity shortcuts ─────────────────────────────────────────────────────

dev.up-bank-a: pki.gen-bank-a pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-bank-a
dev.down-bank-a: deploy.down-backend-bank-a

dev.up-bank-b: pki.gen-bank-b pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-b deploy.up-backend-bank-b
dev.down-bank-b: deploy.down-backend-bank-b

dev.up-bank-c: pki.gen-bank-c pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-bank-c
dev.down-bank-c: deploy.down-backend-bank-c

dev.up-bank-d: pki.gen-bank-d pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-b deploy.up-backend-bank-d
dev.down-bank-d: deploy.down-backend-bank-d

dev.up-central-bank-a: pki.gen-central-bank-a deploy.up-infra deploy.up-spoke-a deploy.up-backend-central-bank-a
dev.down-central-bank-a: deploy.down-backend-central-bank-a

dev.up-central-bank-b: pki.gen-central-bank-b deploy.up-infra deploy.up-spoke-b deploy.up-backend-central-bank-b
dev.down-central-bank-b: deploy.down-backend-central-bank-b

.PHONY: spoke-a spoke-a-down spoke-b spoke-b-down \
	dev.up dev.down \
	dev.up-bank-a dev.down-bank-a dev.up-bank-b dev.down-bank-b \
	dev.up-bank-c dev.down-bank-c dev.up-bank-d dev.down-bank-d \
	dev.up-central-bank-a dev.down-central-bank-a \
	dev.up-central-bank-b dev.down-central-bank-b
