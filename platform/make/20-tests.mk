deploy.test-api-gateway:
	@echo "Running api-gateway tests..."
	@cd ./backend/services/api-gateway && go test ./...

deploy.test-auth:
	@echo "Running auth tests (including E2E)..."
	@cd ./backend/services/auth && go test ./...

deploy.test-compliance:
	@echo "Running compliance tests..."
	@cd ./backend/services/compliance && go test ./...

deploy.test-identity:
	@$(MAKE) deploy.test-auth

deploy.test-data-access:
	@$(MAKE) deploy.test-compliance

deploy.test-services: deploy.test-compliance deploy.test-auth deploy.test-api-gateway

.PHONY: deploy.test-api-gateway deploy.test-auth deploy.test-compliance deploy.test-identity deploy.test-data-access deploy.test-services
