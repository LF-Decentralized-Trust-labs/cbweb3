GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
# D6 gate: 80% statement coverage on CORE back-end business logic.
COVERAGE_THRESHOLD ?= 80

# Core business-logic packages enforced by the gate.
#   internal/services  — request orchestration / use cases
#   internal/domain    — domain model + invariants
#   internal/audit     — audit-trail logic (compliance-relevant)
# Excluded from the denominator (infra — covered by the integration tier, not the
# unit gate):
#   internal/http/handlers — thin HTTP proxies to live backends
#   internal/adapters      — on-chain EVM / gRPC client adapters (live wiring)
#   internal/app           — process bootstrap / wiring
#   internal/config, internal/db, internal/bootstrap — config & GORM/Postgres wiring
CORE_COVERPKG := ./internal/services/...,./internal/domain/...,./internal/audit/...
CORE_TESTPKG  := ./internal/services/... ./internal/domain/... ./internal/audit/...

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
	# G706 (log injection, LOW) excluded globally: the flagged sites log
	# config/on-chain-derived values (addresses, tx hashes), not user input.
	# Relay-controlled fields that ARE logged are sanitized at the call site.
	$(GOSEC) -exclude=G706 ./...

ci: vet test-coverage security
