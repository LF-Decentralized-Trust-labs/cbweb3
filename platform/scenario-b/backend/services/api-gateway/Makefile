GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
# Project requirement: 80% statement coverage on ./internal/...
# The suite currently covers only ~19%, so this gate fails until coverage is
# raised — tracked as a follow-up task (increase test coverage to 80%). This is
# the real target, not a ratchet floor; do not lower it to mask the gap.
COVERAGE_THRESHOLD ?= 80

.PHONY: test test-coverage vet security ci

test:
	$(GO) test ./internal/...

test-coverage:
	$(GO) test ./internal/... -coverpkg=./internal/... -coverprofile=coverage.out -covermode=atomic
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

