TARGET ?= local
DEPLOY_DIR := deploy/$(TARGET)
BACKEND_COMPOSE_SPOKE_A := backend/docker-compose-backend.spoke-a.yaml
BACKEND_COMPOSE_SPOKE_B := backend/docker-compose-backend.spoke-b.yaml
BACKEND_COMPOSE_HUB := backend/docker-compose-backend.hub.yaml

deploy.create-shared-network:
	@docker network inspect cbweb3_network >/dev/null 2>&1 || docker network create cbweb3_network

deploy.up-hub:
	@echo "Starting Hub Besu..."
	@cd ./$(DEPLOY_DIR)/hub-besu && ./startBesu.sh

deploy.up-spoke-a:
	@echo "Starting Spoke Besu A..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./startBesu.sh

deploy.up-spoke-b:
	@echo "Starting Spoke Besu B..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-b && ./startBesu.sh

deploy.up-besu: deploy.up-hub deploy.up-spoke-a deploy.up-spoke-b

deploy.down-hub:
	@echo "Stopping Hub Besu..."
	@cd ./$(DEPLOY_DIR)/hub-besu && ./stopBesu.sh

deploy.down-spoke-a:
	@echo "Stopping Spoke Besu A..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./stopBesu.sh

deploy.down-spoke-b:
	@echo "Stopping Spoke Besu B..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-b && ./stopBesu.sh

deploy.down-besu: deploy.down-hub deploy.down-spoke-a deploy.down-spoke-b

deploy.up-infra: deploy.create-shared-network
	@echo "Starting Compose Services (Keycloak, Postgres)..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml up -d
	@echo "Waiting for Keycloak readiness (timeout: 120s)..."
	@keycloak_port="$${KEYCLOAK_PORT:-8081}"; \
	keycloak_url="http://localhost:$$keycloak_port/realms/master"; \
	max_attempts=40; \
	attempt=1; \
	until curl -fsS "$$keycloak_url" >/dev/null 2>&1; do \
		if [ "$$attempt" -ge "$$max_attempts" ]; then \
			echo "ERROR: Keycloak did not become ready at $$keycloak_url within 120 seconds."; \
			exit 1; \
		fi; \
		echo "  [$$attempt/$$max_attempts] waiting for $$keycloak_url..."; \
		attempt=$$((attempt + 1)); \
		/bin/sleep 3; \
	done; \
	echo "Keycloak is ready at $$keycloak_url."
	@echo "Waiting for Keycloak init script completion (timeout: 180s)..."
	@keycloak_container="$${KEYCLOAK_CONTAINER_NAME:-cbweb3-keycloak}"; \
	init_done_marker="KEYCLOAK_INIT_DONE"; \
	max_attempts=60; \
	attempt=1; \
	until docker logs "$$keycloak_container" 2>&1 | rg -q "$$init_done_marker"; do \
		if [ "$$attempt" -ge "$$max_attempts" ]; then \
			echo "ERROR: Keycloak init script did not finish within 180 seconds. Check logs: docker logs $$keycloak_container"; \
			exit 1; \
		fi; \
		echo "  [$$attempt/$$max_attempts] waiting for init completion in $$keycloak_container logs..."; \
		attempt=$$((attempt + 1)); \
		/bin/sleep 3; \
	done; \
	echo "Keycloak init script finished."

deploy.down-infra:
	@echo "Stopping Compose Services..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml down -v
	@docker network rm cbweb3_network 2>/dev/null || true

deploy.up: deploy.up-besu deploy.up-infra

deploy.up-minimal: deploy.up-hub deploy.up-infra

deploy.down: deploy.down-infra deploy.down-besu

deploy.down-minimal: deploy.down-infra deploy.down-hub

deploy.up-backend:
	@echo "Starting backend stack for all domains (spoke-a, spoke-b, hub)..."
	@$(MAKE) deploy.up-backend-domains

deploy.down-backend:
	@echo "Stopping backend stack for all domains (spoke-a, spoke-b, hub)..."
	@$(MAKE) deploy.down-backend-domains

deploy.validate-backend-spoke-a:
	@docker compose -f $(BACKEND_COMPOSE_SPOKE_A) config -q

deploy.validate-backend-spoke-b:
	@docker compose -f $(BACKEND_COMPOSE_SPOKE_B) config -q

deploy.validate-backend-hub:
	@docker compose -f $(BACKEND_COMPOSE_HUB) config -q

deploy.validate-backend-domains: deploy.validate-backend-spoke-a deploy.validate-backend-spoke-b deploy.validate-backend-hub

deploy.up-backend-spoke-a:
	@echo "Starting backend spoke-a services..."
	@docker compose -f $(BACKEND_COMPOSE_SPOKE_A) up -d

deploy.down-backend-spoke-a:
	@echo "Stopping backend spoke-a services..."
	@docker compose -f $(BACKEND_COMPOSE_SPOKE_A) down -v

deploy.up-backend-spoke-b:
	@echo "Starting backend spoke-b services..."
	@docker compose -f $(BACKEND_COMPOSE_SPOKE_B) up -d

deploy.down-backend-spoke-b:
	@echo "Stopping backend spoke-b services..."
	@docker compose -f $(BACKEND_COMPOSE_SPOKE_B) down -v

deploy.up-backend-hub:
	@echo "Starting backend hub services..."
	@docker compose -f $(BACKEND_COMPOSE_HUB) up -d

deploy.down-backend-hub:
	@echo "Stopping backend hub services..."
	@docker compose -f $(BACKEND_COMPOSE_HUB) down -v

deploy.up-backend-domains: deploy.up-backend-spoke-a deploy.up-backend-spoke-b deploy.up-backend-hub

deploy.down-backend-domains: deploy.down-backend-hub deploy.down-backend-spoke-b deploy.down-backend-spoke-a

deploy.build-ci-runner:
	@docker build -t cbweb3-act-runner:latest -f ./$(DEPLOY_DIR)/act/Dockerfile .

deploy.ci-local: deploy.build-ci-runner
	@docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(shell pwd):/src \
		cbweb3-act-runner:latest \
		-P ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-22.04 \
		$(ARGS)

.PHONY: deploy.create-shared-network deploy.up-hub deploy.up-spoke-a deploy.up-spoke-b deploy.down-hub deploy.down-spoke-a deploy.down-spoke-b deploy.up-besu deploy.down-besu deploy.up-infra deploy.down-infra deploy.up deploy.down deploy.up-minimal deploy.down-minimal deploy.up-backend deploy.down-backend deploy.validate-backend-spoke-a deploy.validate-backend-spoke-b deploy.validate-backend-hub deploy.validate-backend-domains deploy.up-backend-spoke-a deploy.down-backend-spoke-a deploy.up-backend-spoke-b deploy.down-backend-spoke-b deploy.up-backend-hub deploy.down-backend-hub deploy.up-backend-domains deploy.down-backend-domains deploy.build-ci-runner deploy.ci-local
