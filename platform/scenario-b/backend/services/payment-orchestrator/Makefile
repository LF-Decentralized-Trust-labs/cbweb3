GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
# D6 gate: 80% statement coverage on CORE back-end business logic.
COVERAGE_THRESHOLD ?= 80

# Core business-logic packages enforced by the gate.
#   internal/services    — settlement / escrow orchestration use cases
#   internal/domain      — domain model + invariants (bridge atomicity, FX)
#   internal/grpc/server — gRPC handler logic
#   internal/workers     — relayer & FX-expiration workers
# Excluded from the denominator:
#   cmd/                                 — process bootstrap / wiring
#   internal/adapters (besu, cacti)      — live-chain / Cacti relay adapters
#   internal/db, internal/repository GORM — Postgres-only wiring
#   internal/ports, internal/identity    — interfaces / live identity wiring
#   internal/workers/besu_relayer_executor.go — requires live Besu RPC; not
#     hermetically unit-testable, so its statements are filtered out of the
#     coverage denominator below (the rest of internal/workers IS gated).
CORE_COVERPKG := ./internal/services/...,./internal/domain/...,./internal/grpc/server/...,./internal/workers/...
CORE_TESTPKG  := ./internal/services/... ./internal/domain/... ./internal/grpc/server/... ./internal/workers/...
COVERAGE_EXCLUDE := internal/workers/besu_relayer_executor.go

.PHONY: test test-coverage vet security ci

test:
	$(GO) test ./internal/...

test-coverage:
	$(GO) test $(CORE_TESTPKG) -count=1 -coverpkg=$(CORE_COVERPKG) -coverprofile=coverage.raw.out -covermode=atomic
	@grep -v -E '$(COVERAGE_EXCLUDE)' coverage.raw.out > coverage.out
	$(GO) tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(COVERAGE_THRESHOLD)%%\n", $$3 } }'

vet:
	$(GO) vet ./...

security:
	@test -x "$(GOSEC)" || $(GO) install github.com/securego/gosec/v2/cmd/gosec@v2.22.9
	$(GOSEC) -severity high -exclude=G706 ./...

ci: vet test-coverage security
