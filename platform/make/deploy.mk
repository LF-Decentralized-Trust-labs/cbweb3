TARGET ?= local
DEPLOY_DIR := deploy/$(TARGET)

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

deploy.down-infra:
	@echo "Stopping Compose Services..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml down -v
	@docker network rm cbweb3_network 2>/dev/null || true

deploy.up: deploy.up-besu deploy.up-infra

deploy.up-minimal: deploy.up-hub deploy.up-infra

deploy.down: deploy.down-infra deploy.down-besu

deploy.down-minimal: deploy.down-infra deploy.down-hub

deploy.test-api-gateway:
	@echo "Running api-gateway tests..."
	@cd ./backend/services/api-gateway && go test ./...

deploy.test-identity:
	@echo "Running identity tests (including E2E)..."
	@cd ./backend/services/identity && go test ./...

deploy.test-data-access:
	@echo "Running data-access tests..."
	@cd ./backend/services/data-access && go test ./...

deploy.test-services: test-data-access test-identity test-api-gateway

deploy.up-backend:
	@echo "Starting backend stack (postgres, keycloak, data-access, identity, api-gateway)..."
	@docker compose -f backend/docker-compose-backend.yaml up -d

deploy.down-backend:
	@echo "Stopping backend stack..."
	@docker compose -f backend/docker-compose-backend.yaml down -v

.PHONY: deploy.create-shared-network deploy.up-hub deploy.up-spoke-a deploy.up-spoke-b deploy.down-hub deploy.down-spoke-a deploy.down-spoke-b deploy.up-besu deploy.down-besu deploy.up-infra deploy.down-infra deploy.up deploy.down deploy.test-api-gateway deploy.test-identity deploy.test-data-access deploy.test-services deploy.up-backend deploy.down-backend
