GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
COVERAGE_THRESHOLD := 80

MODULE := github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance

# D6 gate scope — CORE back-end business logic only.
# Enforced packages (the -coverpkg denominator):
#   internal/grpc/server, internal/domain
# Excluded from the gate (infra / not hermetically unit-testable):
#   cmd/                   — process entrypoint wiring
#   internal/bootstrap     — DI/wiring
#   internal/grpc/contract — generated protobuf surface
#   internal/pki           — cert material loading
#   internal/repository    — mixed package: the in-memory impl (repository.go)
#                            is hermetically covered, but models.go +
#                            transfer_limit_repository.go are Postgres-only GORM
#                            infra, holding the package at ~50%. Per the D6 gate
#                            policy compliance's enforced scope is grpc/server +
#                            domain only, so the whole package is excluded.
CORE_PKGS := \
	$(MODULE)/internal/grpc/server \
	$(MODULE)/internal/domain
COVERPKGS := $(shell echo '$(CORE_PKGS)' | tr ' ' ',' | sed 's/,,*/,/g; s/^,//; s/,$$//')

.PHONY: test test-coverage vet security ci

test:
	$(GO) test ./internal/...

test-coverage:
	CGO_ENABLED=0 $(GO) test $(CORE_PKGS) -coverpkg=$(COVERPKGS) -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(COVERAGE_THRESHOLD)%%\n", $$3 } }'

vet:
	$(GO) vet ./...

security:
	@test -x "$(GOSEC)" || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOSEC) -severity high ./...

ci: vet test-coverage security
