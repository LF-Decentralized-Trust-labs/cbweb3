GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
# D6 gate: 80% statement coverage on CORE back-end business logic.
COVERAGE_THRESHOLD ?= 80

# Core business-logic packages enforced by the gate.
#   internal/grpc/server — auth gRPC handler logic (PoP / nonce / token flows)
#   internal/domain      — domain model + invariants (e2e build-tagged tests)
# Excluded from the denominator (infra):
#   cmd/                          — process bootstrap / wiring
#   internal/blockchain, internal/cbclient, internal/complianceclient,
#   internal/keycloak, internal/kms, internal/noncestore — live-backend / KMS infra
#   internal/grpc/contract        — generated protobuf contract
#
# -tags e2e: the auth domain suite is build-tagged `//go:build e2e`; the tag is
# included so those hermetic domain tests count toward the gate.
CORE_COVERPKG := ./internal/grpc/server/...,./internal/domain/...
CORE_TESTPKG  := ./internal/grpc/server/... ./internal/domain/...
TEST_TAGS     := e2e

.PHONY: test test-coverage vet security ci

test:
	$(GO) test -tags '$(TEST_TAGS)' ./internal/...

test-coverage:
	$(GO) test -tags '$(TEST_TAGS)' $(CORE_TESTPKG) -coverpkg=$(CORE_COVERPKG) -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(COVERAGE_THRESHOLD)%%\n", $$3 } }'

vet:
	$(GO) vet ./...

security:
	@test -x "$(GOSEC)" || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GOSEC) -severity high -exclude=G706 ./...

ci: vet test-coverage security
