GO ?= go
GOSEC ?= $(shell $(GO) env GOPATH)/bin/gosec
COVERAGE_THRESHOLD ?= 90

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
	$(GOSEC) ./...

ci: vet test-coverage security

