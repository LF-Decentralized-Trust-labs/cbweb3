TARGET ?= local
DEPLOY_DIR := deploy/$(TARGET)
BACKEND_COMPOSE_BANK_A    := backend/docker-compose-backend.bank-a.yaml
BACKEND_COMPOSE_BANK_B    := backend/docker-compose-backend.bank-b.yaml
BACKEND_COMPOSE_CENTRAL_BANK := backend/docker-compose-backend.central-bank.yaml
BACKEND_ENV_BANK_A        := backend/config/.env.infra.bank-a
BACKEND_ENV_BANK_B        := backend/config/.env.infra.bank-b
BACKEND_ENV_CENTRAL_BANK  := backend/config/.env.infra.central-bank

# Keycloak HTTP readiness wait in deploy.up-infra: attempts * sleep = max wall time.
KEYCLOAK_READY_ATTEMPTS ?= 40
KEYCLOAK_WAIT_SLEEP_SEC ?= 3

deploy.create-shared-network:
	@docker network inspect cbweb3_network >/dev/null 2>&1 || docker network create cbweb3_network

deploy.up-spoke-a:
	@echo "Starting Spoke-A Besu (central-bank + bank-a + bank-b nodes)..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./startBesu.sh

deploy.down-spoke-a:
	@echo "Stopping Spoke-A Besu..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./stopBesu.sh

deploy.up-besu: deploy.up-spoke-a

deploy.down-besu: deploy.down-spoke-a

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

deploy.up: deploy.up-besu deploy.up-infra

deploy.up-with-contracts: deploy.up contracts.deploy-all-with-sync

deploy.down: deploy.down-infra deploy.down-besu

deploy.build-backend:
	@echo "Building backend service images..."
	@docker compose -f $(BACKEND_COMPOSE_BANK_A) build
	@docker compose -f $(BACKEND_COMPOSE_BANK_B) build
	@docker compose -f $(BACKEND_COMPOSE_CENTRAL_BANK) build

deploy.up-backend:
	@echo "Starting backend stack for all entities (bank-a, bank-b, central-bank)..."
	@$(MAKE) deploy.up-backend-entities

deploy.down-backend:
	@echo "Stopping backend stack for all entities (bank-a, bank-b, central-bank)..."
	@$(MAKE) deploy.down-backend-entities

deploy.validate-backend-bank-a:
	@docker compose --env-file $(BACKEND_ENV_BANK_A) -f $(BACKEND_COMPOSE_BANK_A) config -q

deploy.validate-backend-bank-b:
	@docker compose --env-file $(BACKEND_ENV_BANK_B) -f $(BACKEND_COMPOSE_BANK_B) config -q

deploy.validate-backend-central-bank:
	@docker compose --env-file $(BACKEND_ENV_CENTRAL_BANK) -f $(BACKEND_COMPOSE_CENTRAL_BANK) config -q

deploy.validate-backend-entities: deploy.validate-backend-bank-a deploy.validate-backend-bank-b deploy.validate-backend-central-bank

deploy.up-backend-bank-a:
	@echo "Starting backend bank-a services..."
	@docker compose --env-file $(BACKEND_ENV_BANK_A) -f $(BACKEND_COMPOSE_BANK_A) up -d

deploy.down-backend-bank-a:
	@echo "Stopping backend bank-a services..."
	@docker compose --env-file $(BACKEND_ENV_BANK_A) -f $(BACKEND_COMPOSE_BANK_A) down -v

deploy.up-backend-bank-b:
	@echo "Starting backend bank-b services..."
	@docker compose --env-file $(BACKEND_ENV_BANK_B) -f $(BACKEND_COMPOSE_BANK_B) up -d

deploy.down-backend-bank-b:
	@echo "Stopping backend bank-b services..."
	@docker compose --env-file $(BACKEND_ENV_BANK_B) -f $(BACKEND_COMPOSE_BANK_B) down -v

deploy.up-backend-central-bank:
	@echo "Starting backend central-bank services..."
	@docker compose --env-file $(BACKEND_ENV_CENTRAL_BANK) -f $(BACKEND_COMPOSE_CENTRAL_BANK) up -d

deploy.down-backend-central-bank:
	@echo "Stopping backend central-bank services..."
	@docker compose --env-file $(BACKEND_ENV_CENTRAL_BANK) -f $(BACKEND_COMPOSE_CENTRAL_BANK) down -v

deploy.up-backend-entities: deploy.up-backend-bank-a deploy.up-backend-bank-b deploy.up-backend-central-bank

deploy.down-backend-entities: deploy.down-backend-central-bank deploy.down-backend-bank-b deploy.down-backend-bank-a

deploy.build-ci-runner:
	@docker build -t cbweb3-act-runner:latest -f ./$(DEPLOY_DIR)/act/Dockerfile .

deploy.ci-local: deploy.build-ci-runner
	@docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(shell pwd):/src \
		cbweb3-act-runner:latest \
		-P ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-22.04 \
		$(ARGS)

.PHONY: deploy.create-shared-network deploy.up-spoke-a deploy.down-spoke-a deploy.up-besu deploy.down-besu deploy.up-infra deploy.down-infra deploy.up deploy.up-with-contracts deploy.down deploy.build-backend deploy.up-backend deploy.down-backend deploy.validate-backend-bank-a deploy.validate-backend-bank-b deploy.validate-backend-central-bank deploy.validate-backend-entities deploy.up-backend-bank-a deploy.down-backend-bank-a deploy.up-backend-bank-b deploy.down-backend-bank-b deploy.up-backend-central-bank deploy.down-backend-central-bank deploy.up-backend-entities deploy.down-backend-entities deploy.build-ci-runner deploy.ci-local
