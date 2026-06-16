test.api-gateway:
	@echo "Running api-gateway tests..."
	@cd ./backend/services/api-gateway && go test ./...

test.auth:
	@echo "Running auth tests (including E2E)..."
	@cd ./backend/services/auth && go test ./...

test.compliance:
	@echo "Running compliance tests..."
	@cd ./backend/services/compliance && go test ./...

test.all: test.compliance test.auth test.api-gateway

# ── D6 coverage gate (80% on CORE back-end business logic) ───────────────────
# Each module gates its own core -coverpkg list (see its Makefile). The
# integration_lite lane is a hermetic cross-service smoke suite (<1s, no infra).
# All targets pin CGO_ENABLED=0 so the gate stays reproducible and hermetic.
COVERAGE_MODULES := \
	backend/services/payment-orchestrator \
	backend/services/compliance \
	backend/services/auth \
	backend/shared/identity

scenario-a.test-backend-coverage:
	@set -e; for m in $(COVERAGE_MODULES); do \
		echo "=== coverage gate: $$m ==="; \
		$(MAKE) -C $$m test-coverage; \
	done
	@echo "=== integration_lite lane ==="
	@cd ./tests/integration && CGO_ENABLED=0 go test -tags integration_lite ./...

.PHONY: test.api-gateway test.auth test.compliance test.all scenario-a.test-backend-coverage
