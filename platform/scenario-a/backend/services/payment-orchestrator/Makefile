GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
COVERAGE_THRESHOLD := 80

MODULE := github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator

# D6 gate scope — CORE back-end business logic only.
# Enforced packages (the -coverpkg denominator):
#   internal/grpc/server, internal/domain, internal/workers, internal/repository
# Excluded from the gate (infra / not hermetically unit-testable):
#   cmd/                       — process entrypoint wiring
#   internal/adapters/{besu,cacti,paladin} — live-chain/relay RPC adapters
#   internal/identity, internal/ports — thin client/interface glue
CORE_PKGS := \
	$(MODULE)/internal/grpc/server \
	$(MODULE)/internal/domain \
	$(MODULE)/internal/workers \
	$(MODULE)/internal/repository
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
	@test -x "$(GOSEC)" || $(GO) install github.com/securego/gosec/v2/cmd/gosec@v2.22.9
	$(GOSEC) -severity high ./...

ci: vet test-coverage security
