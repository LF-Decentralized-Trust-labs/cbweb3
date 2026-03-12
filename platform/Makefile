TARGET ?= local
DEPLOY_DIR := deploy/$(TARGET)

create-shared-network:
	@docker network inspect cbweb3_network >/dev/null 2>&1 || docker network create cbweb3_network

up-besu:
	@echo "Starting Hub Besu..."
	@cd ./$(DEPLOY_DIR)/hub-besu && ./startBesu.sh
	@echo "Starting Spoke Besu A..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./startBesu.sh
	@echo "Starting Spoke Besu B..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-b && ./startBesu.sh

down-besu:
	@echo "Stopping Hub Besu..."
	@cd ./$(DEPLOY_DIR)/hub-besu && ./stopBesu.sh
	@echo "Stopping Spoke Besu A..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-a && ./stopBesu.sh
	@echo "Stopping Spoke Besu B..."
	@cd ./$(DEPLOY_DIR)/spoke-besu-b && ./stopBesu.sh

up-infra: create-shared-network
	@echo "Starting Compose Services (Keycloak, Postgres)..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml up -d

down-infra:
	@echo "Stopping Compose Services..."
	@docker compose -f $(DEPLOY_DIR)/compose.yml down -v
	@docker network rm cbweb3_network 2>/dev/null || true

up: up-besu up-infra

down: down-infra down-besu

test-api-gateway:
	@echo "Running api-gateway tests..."
	@cd ./backend/services/api-gateway && go test ./...

test-identity:
	@echo "Running identity tests (including E2E)..."
	@cd ./backend/services/identity && go test ./...

test-data-access:
	@echo "Running data-access tests..."
	@cd ./backend/services/data-access && go test ./...

test-services: test-data-access test-identity test-api-gateway

up-backend:
	@echo "Starting backend stack (postgres, keycloak, data-access, identity, api-gateway)..."
	@docker compose -f backend/docker-compose-backend.yaml up -d

down-backend:
	@echo "Stopping backend stack..."
	@docker compose -f backend/docker-compose-backend.yaml down -v

.PHONY: create-shared-network up-besu down-besu up-infra down-infra up down \
	test-api-gateway test-identity test-data-access test-services \
	up-backend down-backend
