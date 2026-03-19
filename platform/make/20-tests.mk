deploy.test-api-gateway:
	@echo "Running api-gateway tests..."
	@cd ./backend/services/api-gateway && go test ./...

deploy.test-identity:
	@echo "Running identity tests (including E2E)..."
	@cd ./backend/services/identity && go test ./...

deploy.test-data-access:
	@echo "Running data-access tests..."
	@cd ./backend/services/data-access && go test ./...

deploy.test-services: deploy.test-data-access deploy.test-identity deploy.test-api-gateway

.PHONY: deploy.test-api-gateway deploy.test-identity deploy.test-data-access deploy.test-services
