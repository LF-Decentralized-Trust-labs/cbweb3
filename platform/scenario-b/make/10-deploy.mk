TARGET ?= local
DEPLOY_DIR := deploy/$(TARGET)

SPOKE_A_ENTITIES := central-bank-a bank-a
SPOKE_B_ENTITIES := central-bank-b bank-b
ALL_ENTITIES     := $(SPOKE_A_ENTITIES) $(SPOKE_B_ENTITIES)

BACKEND_DIR     := backend
BACKEND_ENV_DIR := $(BACKEND_DIR)/config

KEYCLOAK_READY_ATTEMPTS ?= 40
KEYCLOAK_WAIT_SLEEP_SEC ?= 3

# ── Shared network ───────────────────────────────────────────────────────────

deploy.create-shared-network:
	@docker network inspect cbweb3_network >/dev/null 2>&1 || docker network create cbweb3_network

# ── Besu spokes ──────────────────────────────────────────────────────────────

deploy.up-spoke-a:
	@echo "Starting Spoke-A Besu (central-bank-a + bank-a nodes)..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./startBesu.sh

deploy.down-spoke-a:
	@echo "Stopping Spoke-A Besu..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./stopBesu.sh

deploy.up-spoke-b:
	@echo "Starting Spoke-B Besu (central-bank-b + bank-b nodes)..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-b && ./startBesu.sh

deploy.down-spoke-b:
	@echo "Stopping Spoke-B Besu..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-b && ./stopBesu.sh

# ── Besu hub (independent neutral network — chain 1337) ──────────────────────

deploy.up-hub-besu:
	@echo "Starting International Hub Besu (single-validator sandbox, chain 1337)..."
	@cd ./$(DEPLOY_DIR)/hub-besu && ./startBesu.sh

deploy.down-hub-besu:
	@echo "Stopping International Hub Besu..."
	@cd ./$(DEPLOY_DIR)/hub-besu && ./stopBesu.sh

deploy.up-besu: deploy.up-hub-besu deploy.up-spoke-a deploy.up-spoke-b
deploy.down-besu: deploy.down-spoke-b deploy.down-spoke-a deploy.down-hub-besu

# ── Infra (Keycloak, Postgres, Redis) ────────────────────────────────────────

deploy.up-infra: deploy.create-shared-network
	@echo "Starting Compose Services (Keycloak, Postgres, Redis)..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml up -d
	@ready_max=$$(( $(KEYCLOAK_READY_ATTEMPTS) * $(KEYCLOAK_WAIT_SLEEP_SEC) )); \
	echo "Waiting for Keycloak HTTP readiness (max ~$$ready_max seconds, $(KEYCLOAK_READY_ATTEMPTS) attempts × $(KEYCLOAK_WAIT_SLEEP_SEC)s)..."; \
	keycloak_port="$${KEYCLOAK_PORT:-8081}"; \
	keycloak_url="http://localhost:$$keycloak_port/realms/master"; \
	max_attempts=$(KEYCLOAK_READY_ATTEMPTS); \
	attempt=1; \
	until curl -fsS "$$keycloak_url" >/dev/null 2>&1; do \
		if [ "$$attempt" -ge "$$max_attempts" ]; then \
			echo "ERROR: Keycloak did not become ready at $$keycloak_url within ~$$ready_max seconds ($$max_attempts attempts). Try: docker logs $${KEYCLOAK_CONTAINER_NAME:-cbweb3-keycloak}"; \
			exit 1; \
		fi; \
		echo "  [$$attempt/$$max_attempts] waiting for $$keycloak_url..."; \
		attempt=$$((attempt + 1)); \
		/bin/sleep $(KEYCLOAK_WAIT_SLEEP_SEC); \
	done; \
	echo "Keycloak is ready at $$keycloak_url."
	@echo "Waiting for Keycloak init (KEYCLOAK_INIT_DONE, up to 10 min)..."; \
	end=$$(( $$(date +%s) + 600 )); \
	while [ "$$(date +%s)" -lt "$$end" ]; do \
		if docker logs cbweb3-keycloak 2>&1 | grep -Fq "KEYCLOAK_INIT_DONE"; then \
			echo "Keycloak init script finished."; \
			exit 0; \
		fi; \
		/bin/sleep 5; \
	done; \
	echo "ERROR: Keycloak init did not finish within 10 minutes."; \
	docker logs --tail 50 cbweb3-keycloak 2>&1 || true; \
	exit 1

deploy.down-infra:
	@echo "Stopping Compose Services..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml down -v
	@docker network rm cbweb3_network 2>/dev/null || true

# ── Full environment ─────────────────────────────────────────────────────────

deploy.up: deploy.up-besu deploy.up-infra
deploy.up-with-contracts: deploy.up contracts.deploy-all-with-sync
deploy.down: deploy.down-infra deploy.down-besu

# ── Backend services (pattern rules) ─────────────────────────────────────────

deploy.up-backend-%:
	@echo "Starting backend $* services..."
	@docker compose --env-file $(BACKEND_ENV_DIR)/.env.infra.$* \
		-f $(BACKEND_DIR)/docker-compose-backend.$*.yaml up -d --build

deploy.down-backend-%:
	@echo "Stopping backend $* services..."
	@docker compose --env-file $(BACKEND_ENV_DIR)/.env.infra.$* \
		-f $(BACKEND_DIR)/docker-compose-backend.$*.yaml down -v

deploy.validate-backend-%:
	@docker compose --env-file $(BACKEND_ENV_DIR)/.env.infra.$* \
		-f $(BACKEND_DIR)/docker-compose-backend.$*.yaml config -q

deploy.build-backend:
	@echo "Building backend service images ($(ALL_ENTITIES))..."
	@$(foreach e,$(ALL_ENTITIES),docker compose -f $(BACKEND_DIR)/docker-compose-backend.$(e).yaml build &&) true

deploy.up-backend-spoke-a: $(addprefix deploy.up-backend-,$(SPOKE_A_ENTITIES))
deploy.down-backend-spoke-a: $(addprefix deploy.down-backend-,$(SPOKE_A_ENTITIES))
deploy.up-backend-spoke-b: $(addprefix deploy.up-backend-,$(SPOKE_B_ENTITIES))
deploy.down-backend-spoke-b: $(addprefix deploy.down-backend-,$(SPOKE_B_ENTITIES))

deploy.up-backend-entities: $(addprefix deploy.up-backend-,$(ALL_ENTITIES))
deploy.down-backend-entities: $(addprefix deploy.down-backend-,$(ALL_ENTITIES))
deploy.validate-backend-entities: $(addprefix deploy.validate-backend-,$(ALL_ENTITIES))

deploy.up-backend: deploy.up-backend-entities
deploy.down-backend: deploy.down-backend-entities

# ── CI ────────────────────────────────────────────────────────────────────────

deploy.build-ci-runner:
	@docker build -t cbweb3-act-runner:latest -f ./$(DEPLOY_DIR)/act/Dockerfile .

deploy.ci-local: deploy.build-ci-runner
	@docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(shell pwd):/src \
		cbweb3-act-runner:latest \
		-P ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-22.04 \
		$(ARGS)

.PHONY: deploy.create-shared-network \
	deploy.up-spoke-a deploy.down-spoke-a deploy.up-spoke-b deploy.down-spoke-b \
	deploy.up-hub-besu deploy.down-hub-besu \
	deploy.up-besu deploy.down-besu deploy.up-infra deploy.down-infra \
	deploy.up deploy.up-with-contracts deploy.down \
	deploy.build-backend deploy.up-backend deploy.down-backend \
	deploy.up-backend-spoke-a deploy.down-backend-spoke-a \
	deploy.up-backend-spoke-b deploy.down-backend-spoke-b \
	deploy.up-backend-entities deploy.down-backend-entities deploy.validate-backend-entities \
	deploy.build-ci-runner deploy.ci-local
