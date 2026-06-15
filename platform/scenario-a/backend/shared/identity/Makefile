GO ?= go
COVERAGE_THRESHOLD := 80

MODULE := github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity

# D6 gate scope — shared/identity is a single self-contained business-logic
# package with no infra files, so the whole module is the gate denominator.
CORE_PKGS := $(MODULE)
COVERPKGS := $(MODULE)

.PHONY: test test-coverage vet

test:
	$(GO) test ./...

test-coverage:
	CGO_ENABLED=0 $(GO) test $(CORE_PKGS) -coverpkg=$(COVERPKGS) -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | awk '/total:/ { gsub("%","",$$3); if ($$3+0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.2f%% is below threshold $(COVERAGE_THRESHOLD)%%\n", $$3; exit 1 } else { printf "coverage %.2f%% meets threshold $(COVERAGE_THRESHOLD)%%\n", $$3 } }'

vet:
	$(GO) vet ./...
