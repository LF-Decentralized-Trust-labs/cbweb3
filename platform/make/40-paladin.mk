PALADIN_DIR        := deploy/local/paladin
PALADIN_SCRIPTS    := $(PALADIN_DIR)/scripts
PALADIN_TIMEOUT    ?= 5m
BESU_READY_WAIT    ?= 60
PALADIN_READY_WAIT ?= 60
BESU_RPC_URL_A     ?= http://127.0.0.1:8645
BESU_RPC_URL_B     ?= http://127.0.0.1:8745

# ── contract deployment ──────────────────────────────────────────────────────

paladin.deploy-registry-spoke-a:
	@echo "Deploying IdentityRegistry on spoke-a..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-a BESU_RPC_URL=$(BESU_RPC_URL_A) \
		go test ./... -run TestDeployEVMRegistry -v -count=1 -timeout $(PALADIN_TIMEOUT)

paladin.deploy-registry-spoke-b:
	@echo "Deploying IdentityRegistry on spoke-b..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-b BESU_RPC_URL=$(BESU_RPC_URL_B) \
		go test ./... -run TestDeployEVMRegistry -v -count=1 -timeout $(PALADIN_TIMEOUT)

paladin.deploy-zeto-spoke-a:
	@echo "Deploying ZetoFactory on spoke-a..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-a BESU_RPC_URL=$(BESU_RPC_URL_A) \
		go test ./... -run TestDeployZetoFactory -v -count=1 -timeout $(PALADIN_TIMEOUT)

paladin.deploy-zeto-spoke-b:
	@echo "Deploying ZetoFactory on spoke-b..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-b BESU_RPC_URL=$(BESU_RPC_URL_B) \
		go test ./... -run TestDeployZetoFactory -v -count=1 -timeout $(PALADIN_TIMEOUT)

paladin.deploy-contracts-spoke-a: paladin.deploy-registry-spoke-a paladin.deploy-zeto-spoke-a

paladin.deploy-contracts-spoke-b: paladin.deploy-registry-spoke-b paladin.deploy-zeto-spoke-b

# ── Zeto token instance creation (requires Paladin nodes running) ────────

paladin.create-zeto-token-spoke-a:
	@echo "Creating Zeto tCeBM token instance on spoke-a via Paladin API..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-a PALADIN_CB_URL=http://127.0.0.1:31648 \
		go test ./... -run TestCreateZetoTokenInstance -v -count=1 -timeout $(PALADIN_TIMEOUT)

paladin.create-zeto-token-spoke-b:
	@echo "Creating Zeto tCeBM token instance on spoke-b via Paladin API..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-b PALADIN_CB_URL=http://127.0.0.1:31748 \
		go test ./... -run TestCreateZetoTokenInstance -v -count=1 -timeout $(PALADIN_TIMEOUT)

# ── node registration ────────────────────────────────────────────────────────

paladin.register-nodes-spoke-a:
	@echo "Registering Paladin nodes on spoke-a..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-a BESU_RPC_URL=$(BESU_RPC_URL_A) \
		go test ./... -run TestRegisterPaladinNodes -v -count=1 -timeout $(PALADIN_TIMEOUT)

paladin.register-nodes-spoke-b:
	@echo "Registering Paladin nodes on spoke-b..."
	@cd $(PALADIN_SCRIPTS) && \
		SPOKE=spoke-b BESU_RPC_URL=$(BESU_RPC_URL_B) \
		go test ./... -run TestRegisterPaladinNodes -v -count=1 -timeout $(PALADIN_TIMEOUT)

# ── TLS certs & config rendering ─────────────────────────────────────────────

paladin.generate-certs-spoke-a:
	@echo "Generating TLS certificates for spoke-a Paladin nodes..."
	@SPOKE=spoke-a bash $(PALADIN_DIR)/generate-certs.sh

paladin.generate-certs-spoke-b:
	@echo "Generating TLS certificates for spoke-b Paladin nodes..."
	@SPOKE=spoke-b bash $(PALADIN_DIR)/generate-certs.sh

paladin.render-configs-spoke-a:
	@echo "Rendering Paladin configs for spoke-a..."
	@SPOKE=spoke-a bash $(PALADIN_DIR)/render-configs.sh

paladin.render-configs-spoke-b:
	@echo "Rendering Paladin configs for spoke-b..."
	@SPOKE=spoke-b bash $(PALADIN_DIR)/render-configs.sh

# ── docker compose ───────────────────────────────────────────────────────────

paladin.start-spoke-a:
	@echo "Starting Paladin nodes for spoke-a..."
	@PALADIN_UID=$$(id -u) PALADIN_GID=$$(id -g) \
		docker compose -f $(PALADIN_DIR)/spoke-a/docker-compose.yml up -d

paladin.stop-spoke-a:
	@echo "Stopping Paladin nodes for spoke-a..."
	@docker compose -f $(PALADIN_DIR)/spoke-a/docker-compose.yml down

# Remove all spoke-a Paladin data volumes (SQLite + LevelDB state).
# Must be called whenever Besu is regenerated from scratch so Paladin's
# persisted block-indexer state does not fall out of sync with the new chain.
paladin.clean-volumes-spoke-a:
	@echo "Removing Paladin data volumes for spoke-a..."
	@docker volume rm -f \
		spoke-a_paladin_spoke_a_cb_data \
		spoke-a_paladin_spoke_a_bank_a_data \
		spoke-a_paladin_spoke_a_bank_c_data || true

paladin.start-spoke-b:
	@echo "Starting Paladin nodes for spoke-b..."
	@PALADIN_UID=$$(id -u) PALADIN_GID=$$(id -g) \
		docker compose -f $(PALADIN_DIR)/spoke-b/docker-compose.yml up -d

paladin.stop-spoke-b:
	@echo "Stopping Paladin nodes for spoke-b..."
	@docker compose -f $(PALADIN_DIR)/spoke-b/docker-compose.yml down

# Remove all spoke-b Paladin data volumes (SQLite + LevelDB state).
paladin.clean-volumes-spoke-b:
	@echo "Removing Paladin data volumes for spoke-b..."
	@docker volume rm -f \
		spoke-b_paladin_spoke_b_cb_data \
		spoke-b_paladin_spoke_b_bank_b_data \
		spoke-b_paladin_spoke_b_bank_d_data || true

# ── full spoke setup ─────────────────────────────────────────────────────────
# Expects the corresponding Besu network to already be running.

setup-spoke-a: deploy.up-spoke-a
	@echo "Waiting for Besu spoke-a to be ready ($(BESU_READY_WAIT)s)..."
	@sleep $(BESU_READY_WAIT)
	@$(MAKE) paladin.deploy-contracts-spoke-a
	@$(MAKE) paladin.generate-certs-spoke-a
	@$(MAKE) paladin.render-configs-spoke-a
	@$(MAKE) paladin.register-nodes-spoke-a
	@$(MAKE) paladin.stop-spoke-a
	@$(MAKE) paladin.clean-volumes-spoke-a
	@$(MAKE) paladin.start-spoke-a
	@echo "Waiting for Paladin spoke-a nodes to be ready ($(PALADIN_READY_WAIT)s)..."
	@sleep $(PALADIN_READY_WAIT)
	@$(MAKE) paladin.create-zeto-token-spoke-a
	@echo "Spoke-a setup complete."

setup-spoke-b: deploy.up-spoke-b
	@echo "Waiting for Besu spoke-b to be ready ($(BESU_READY_WAIT)s)..."
	@sleep $(BESU_READY_WAIT)
	@$(MAKE) paladin.deploy-contracts-spoke-b
	@$(MAKE) paladin.generate-certs-spoke-b
	@$(MAKE) paladin.render-configs-spoke-b
	@$(MAKE) paladin.register-nodes-spoke-b
	@$(MAKE) paladin.stop-spoke-b
	@$(MAKE) paladin.clean-volumes-spoke-b
	@$(MAKE) paladin.start-spoke-b
	@echo "Waiting for Paladin spoke-b nodes to be ready ($(PALADIN_READY_WAIT)s)..."
	@sleep $(PALADIN_READY_WAIT)
	@$(MAKE) paladin.create-zeto-token-spoke-b
	@echo "Spoke-b setup complete."

setup-spoke-uc: setup-spoke-a setup-spoke-b
	@echo "Full UC scope setup complete (spoke-a + spoke-b)."

.PHONY: \
	paladin.deploy-registry-spoke-a paladin.deploy-registry-spoke-b \
	paladin.deploy-zeto-spoke-a paladin.deploy-zeto-spoke-b \
	paladin.deploy-contracts-spoke-a paladin.deploy-contracts-spoke-b \
	paladin.register-nodes-spoke-a paladin.register-nodes-spoke-b \
	paladin.generate-certs-spoke-a paladin.generate-certs-spoke-b \
	paladin.render-configs-spoke-a paladin.render-configs-spoke-b \
	paladin.start-spoke-a paladin.stop-spoke-a paladin.clean-volumes-spoke-a \
	paladin.start-spoke-b paladin.stop-spoke-b paladin.clean-volumes-spoke-b \
	paladin.create-zeto-token-spoke-a paladin.create-zeto-token-spoke-b \
	setup-spoke-a setup-spoke-b setup-spoke-uc
