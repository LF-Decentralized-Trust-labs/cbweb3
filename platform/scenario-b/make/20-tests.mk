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

# ── D6 coverage gate (80% on core back-end business logic) ───────────────────
# Runs each module's gated `test-coverage` target, the shared/identity gate, and
# both hermetic integration_lite lanes. All hermetic, CGO_ENABLED=0, < ~2 min.
#
# Per-module gate scope is documented in each service Makefile (explicit core
# -coverpkg lists; infra excluded). shared/identity is gated inline here as it
# has no Makefile. integration_lite is the PR-gate integration tier; the heavy
# //go:build integration suite stays on a separate nightly lane.
IDENTITY_COVERAGE_THRESHOLD ?= 80

scenario-b.test-backend-coverage:
	@echo ">> api-gateway coverage gate"
	@cd ./backend/services/api-gateway && CGO_ENABLED=0 $(MAKE) test-coverage
	@echo ">> auth coverage gate"
	@cd ./backend/services/auth && CGO_ENABLED=0 $(MAKE) test-coverage
	@echo ">> compliance coverage gate"
	@cd ./backend/services/compliance && CGO_ENABLED=0 $(MAKE) test-coverage
	@echo ">> payment-orchestrator coverage gate"
	@cd ./backend/services/payment-orchestrator && CGO_ENABLED=0 $(MAKE) test-coverage
	@echo ">> shared/identity coverage gate"
	@cd ./backend/shared/identity && CGO_ENABLED=0 go test ./... -coverpkg=./... -coverprofile=coverage.out -covermode=atomic && \
		go tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(IDENTITY_COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(IDENTITY_COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(IDENTITY_COVERAGE_THRESHOLD)%%\n", $$3 } }'
	@echo ">> integration_lite lanes"
	@cd ./tests/integration-lite/orchestrator && CGO_ENABLED=0 go test -tags integration_lite ./...
	@cd ./tests/integration-lite/compliance-gate && CGO_ENABLED=0 go test -tags integration_lite ./...
	@echo ">> all scenario-b backend coverage gates passed"

.PHONY: test.api-gateway test.auth test.compliance test.all scenario-b.test-backend-coverage
