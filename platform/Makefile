TARGET ?= local
DEPLOY_DIR := deploy/$(TARGET)

up:
	@echo "Starting Domestic Besu A..."
	@cd ./$(DEPLOY_DIR)/domestic-besu-a && ./startBesu.sh
	@echo "Starting Domestic Besu B..."
	@cd ./$(DEPLOY_DIR)/domestic-besu-b && ./startBesu.sh
	@echo "Starting International Besu..."
	@cd ./$(DEPLOY_DIR)/international-besu && ./startBesu.sh
	@echo "Starting Compose Services"
	@docker network create cbweb3_network
	@docker compose -f $(DEPLOY_DIR)/compose.yml up -d

down:
	@docker compose -f $(DEPLOY_DIR)/compose.yml down -v
	@docker network rm cbweb3_network
	@echo "Stopping Compose Services"
	@echo "Stopping Domestic Besu A..."
	@cd ./$(DEPLOY_DIR)/domestic-besu-a && ./stopBesu.sh
	@echo "Stopping Domestic Besu B..."
	@cd ./$(DEPLOY_DIR)/domestic-besu-b && ./stopBesu.sh
	@echo "Stopping International Besu..."
	@cd ./$(DEPLOY_DIR)/international-besu && ./stopBesu.sh

ps:
	@docker compose -f $(DEPLOY_DIR)/compose.yml ps -a

.PHONY: up down
