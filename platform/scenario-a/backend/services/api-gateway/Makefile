GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
COVERAGE_THRESHOLD ?= 80

# Tests are hermetic: no live backends, no chain, no Keycloak/Postgres.
export CGO_ENABLED ?= 0

.PHONY: test test-coverage vet security ci

test:
	$(GO) test ./internal/...

# COVERPKGS is the set of CORE business-logic packages the 80% gate applies to.
# Excluded (infrastructure / integration surface, intentionally not gated):
#   - internal/adapters/*  : thin gRPC/HTTP clients dialing live backends
#                            (payment-orchestrator, compliance, identity).
#   - internal/app         : composition root that dials live gRPC connections;
#                            only its early-return error paths are hermetically
#                            reachable (covered by app_test.go, not gated here).
#   - cmd/*                : process entrypoint (no business logic).
# internal/interfaces has no statements (pure contracts) and is harmless to keep.
COVERPKGS := $(shell $(GO) list ./internal/... | grep -v '/adapters/' | grep -v '/internal/app$$' | tr '\n' ',' | sed 's/,$$//')

test-coverage:
	$(GO) test ./internal/... -coverpkg=$(COVERPKGS) -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(COVERAGE_THRESHOLD)%%\n", $$3 } }'

vet:
	$(GO) vet ./...

security:
	@test -x "$(GOSEC)" || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOSEC) -severity high ./...

ci: vet test-coverage security

