test.api-gateway:
	@echo "Running api-gateway tests..."
	@cd ./backend/services/api-gateway && go test ./...

test.auth:
	@echo "Running auth tests (including E2E)..."
	@cd ./backend/services/auth && go test ./...

test.compliance:
	@echo "Running compliance tests..."
	@cd ./backend/services/compliance && go test ./...

test.all: test.compliance test.auth test.api-gateway

# ── D6 coverage gate (80% on CORE back-end business logic) ───────────────────
# Each module gates its own core -coverpkg list (see its Makefile). The
# integration_lite lane is a hermetic cross-service smoke suite (<1s, no infra).
# All targets pin CGO_ENABLED=0 so the gate stays reproducible and hermetic.
COVERAGE_MODULES := \
	backend/services/payment-orchestrator \
	backend/services/compliance \
	backend/services/auth \
	backend/shared/identity

scenario-a.test-backend-coverage:
	@set -e; for m in $(COVERAGE_MODULES); do \
		echo "=== coverage gate: $$m ==="; \
		$(MAKE) -C $$m test-coverage; \
	done
	@echo "=== integration_lite lane ==="
	@cd ./tests/integration && CGO_ENABLED=0 go test -tags integration_lite ./...


# ── R1-12.3 performance threshold harness (see scenario-a/docs/performance) ──────────────
# AUTH_TOKEN (access_token JWT) is REQUIRED for write scenarios.
# Fill measured numbers into docs/performance/RESULTS-TEMPLATE.md after a real run.
# Do NOT run the soak in CI.

# Default to the real bank-a API gateway host port (deploy/local/README.md).
API_GW_URL ?= http://localhost:18080

scenario-a.perf-baseline:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] running API latency baseline (read + write p95)..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  RECEIVER=$${RECEIVER:-funded_operator@spoke-a-bank-c} \
	  k6 run tests/performance/scenario-a-perf.js

scenario-a.perf-transfer:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] HTLC token-transfer throughput — 50 TPS target (R1-12.3 threshold 1)..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  RECEIVER=$${RECEIVER:-funded_operator@spoke-a-bank-c} \
	  TRANSFER_TPS=$${TRANSFER_TPS:-50} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/k6/htlc-transfer-throughput.js

scenario-a.perf-zeto:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] Zeto escrow throughput — 15 TPS target (R1-12.3 threshold 2)..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  ZETO_TPS=$${ZETO_TPS:-15} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/k6/zeto-escrow-throughput.js

scenario-a.perf-soak:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] 12-hour SOAK — dedicated infra only, NOT for CI..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  RECEIVER=$${RECEIVER:-funded_operator@spoke-a-bank-c} \
	  DURATION=$${DURATION:-12h} \
	  k6 run tests/performance/k6/soak.js

# ── R1-12.3 zero-config orchestration ───────────────────────────────────────
# perf-all takes NO required args: it stands up the stack (lib/stack.sh), mints
# AUTH_TOKEN via /auth/login (lib/auth.sh), funds the sender (lib/fund.sh), runs
# baseline + 50 TPS HTLC + 15 TPS Zeto, correlates on-chain TTF (lib/ttf.sh), and
# writes docs/performance/RESULTS-<UTC>.md (lib/results.sh).
scenario-a.perf-all:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required"; exit 1; }
	@echo "[scenario-a] R1-12.3 zero-config performance suite (perf-all)..."
	@bash tests/performance/run-all.sh

# Validate the orchestrator end to end with NO infra (CI-safe smoke).
scenario-a.perf-all-dry:
	@command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required"; exit 1; }
	@echo "[scenario-a] perf-all dry-run (no infra)..."
	@PERF_DRY_RUN=1 bash tests/performance/run-all.sh

# SEPARATE opt-in 12h soak — dedicated infra only, NEVER in CI or perf-all.
scenario-a.perf-soak-all:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required"; exit 1; }
	@echo "[scenario-a] R1-12.3 12-hour SOAK (opt-in, dedicated infra only)..."
	@PERF_SOAK=1 bash tests/performance/run-all.sh

.PHONY: test.api-gateway test.auth test.compliance test.all scenario-a.test-backend-coverage \
	scenario-a.perf-baseline scenario-a.perf-transfer scenario-a.perf-zeto scenario-a.perf-soak \
	scenario-a.perf-all scenario-a.perf-all-dry scenario-a.perf-soak-all
