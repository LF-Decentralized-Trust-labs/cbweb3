GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
# D6 gate: 80% statement coverage on CORE back-end business logic.
COVERAGE_THRESHOLD ?= 80

# Core business-logic packages enforced by the gate.
#   internal/services    — compliance use cases (AML / sanctions / KYC)
#   internal/grpc/server — gRPC handler logic
#   internal/domain      — domain model + invariants
#   internal/repository  — persistence logic (hermetic sqlite-backed unit tests)
# Excluded from the denominator (infra):
#   cmd/, internal/bootstrap — process bootstrap / wiring
#   internal/db, internal/pki — config & Postgres / PKI infra
#   internal/grpc/contract — generated protobuf contract
CORE_COVERPKG := ./internal/services/...,./internal/grpc/server/...,./internal/domain/...,./internal/repository/...
CORE_TESTPKG  := ./internal/services/... ./internal/grpc/server/... ./internal/domain/... ./internal/repository/...

.PHONY: test test-coverage vet security ci

test:
	$(GO) test ./internal/...

test-coverage:
	$(GO) test $(CORE_TESTPKG) -coverpkg=$(CORE_COVERPKG) -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(COVERAGE_THRESHOLD)%%\n", $$3 } }'

vet:
	$(GO) vet ./...

security:
	@test -x "$(GOSEC)" || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOSEC) -exclude=G706 ./...

ci: vet test-coverage security
