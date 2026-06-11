# dev.* — local development shortcuts
# PKI generation is idempotent (skipped if files exist). Use FORCE=1 to regenerate.

# ── Spoke-level targets (full stack) ─────────────────────────────────────────

spoke-a: pki.gen-central-bank-a pki.gen-bank-a pki.gen-commercial-banks
	$(MAKE) deploy.up-infra deploy.up-hub-besu deploy.up-spoke-a
	$(MAKE) contracts.setup contracts.deploy-spoke-a contracts.deploy-hub contracts.sync-addresses
	$(MAKE) contracts.register-participants-spoke-a
	$(MAKE) deploy.up-backend-spoke-a
	@echo "Spoke-A stack is up."

spoke-a-down: deploy.down-backend-spoke-a deploy.down-spoke-a
	@echo "Spoke-A stack is down (shared infra left running)."

spoke-b: pki.gen-central-bank-b pki.gen-bank-b pki.gen-commercial-banks
	$(MAKE) deploy.up-infra deploy.up-spoke-b
	$(MAKE) contracts.setup contracts.deploy-spoke-b contracts.sync-addresses
	$(MAKE) contracts.register-participants-spoke-b
	$(MAKE) deploy.up-backend-spoke-b
	@echo "Spoke-B stack is up."

spoke-b-down: deploy.down-backend-spoke-b deploy.down-spoke-b
	@echo "Spoke-B stack is down (shared infra left running)."

cacti-up:
	@echo "Starting Cacti HTLC interop..."
	@docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml up -d --build

cacti-down:
	@echo "Stopping Cacti HTLC interop..."
	@docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml down

spoke-all:
	$(MAKE) spoke-a
	$(MAKE) spoke-b
	$(MAKE) cacti-up
	@echo "Both Spoke-A and Spoke-B stacks are up (with Cacti interop)."

spoke-all-down:
	$(MAKE) cacti-down
	$(MAKE) spoke-b-down
	$(MAKE) spoke-a-down
	@echo "Both Spoke-A and Spoke-B stacks are down (shared infra left running)."

# ── Full environment ─────────────────────────────────────────────────────────

dev.up: pki.gen-all deploy.up contracts.deploy-all-with-sync \
        deploy.up-backend-entities
dev.down: deploy.down-backend-entities deploy.down

# ── Per-entity shortcuts ─────────────────────────────────────────────────────

dev.up-bank-a: pki.gen-bank-a pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-a deploy.up-backend-bank-a
dev.down-bank-a: deploy.down-backend-bank-a

dev.up-bank-b: pki.gen-bank-b pki.gen-commercial-banks deploy.up-infra deploy.up-spoke-b deploy.up-backend-bank-b
dev.down-bank-b: deploy.down-backend-bank-b


dev.up-central-bank-a: pki.gen-central-bank-a deploy.up-infra deploy.up-spoke-a deploy.up-backend-central-bank-a
dev.down-central-bank-a: deploy.down-backend-central-bank-a

dev.up-central-bank-b: pki.gen-central-bank-b deploy.up-infra deploy.up-spoke-b deploy.up-backend-central-bank-b
dev.down-central-bank-b: deploy.down-backend-central-bank-b

.PHONY: spoke-a spoke-a-down spoke-b spoke-b-down spoke-all spoke-all-down cacti-up cacti-down \
	dev.up dev.down \
	dev.up-bank-a dev.down-bank-a dev.up-bank-b dev.down-bank-b \
	dev.up-central-bank-a dev.down-central-bank-a \
	dev.up-central-bank-b dev.down-central-bank-b
