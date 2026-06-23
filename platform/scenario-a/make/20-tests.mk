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

# ── Live happy-path integration test (full FX + cross-spoke HTLC flow) ────────
# Drives the real REST API of a running `make spoke-all` stack through the
# Scenario A core workflow (login → mint → FX propose/accept → HTLC lock both
# legs → settle → relay-settle → verify). Counterpart to the hermetic
# integration_lite lane; gated behind the `integration` build tag so the two
# never compile together.
#
#   make scenario-a.test-integration              # auto-detect: brings the stack up only if it isn't already running
#   SKIP_UP=0 make scenario-a.test-integration    # force a fresh bring-up first (WIPES the chain)
#   SKIP_UP=1 make scenario-a.test-integration    # never bring up; fail fast if the stack is down
#   SKIP_DOWN=0 make scenario-a.test-integration  # tear the stack down afterwards
#
# SKIP_UP unset → auto: a healthz probe decides whether to run `spoke-all` first.
SKIP_UP   ?= auto
SKIP_DOWN ?= 1

API_GW_BANK_A_URL         ?= http://localhost:18080
API_GW_BANK_B_URL         ?= http://localhost:28080
API_GW_BANK_D_URL         ?= http://localhost:58080
API_GW_CENTRAL_BANK_A_URL ?= http://localhost:38080
API_GW_CENTRAL_BANK_B_URL ?= http://localhost:60080

scenario-a.test-integration: ## Run the happy-path test; brings the stack up automatically if it isn't already running
	@echo "[scenario-a] running integration test (full happy path)..."
	@if [ "$(SKIP_UP)" = "0" ]; then \
	  echo "[scenario-a] SKIP_UP=0 — forcing a fresh bring-up (regenerates genesis, WIPES the chain)..."; \
	  $(MAKE) spoke-all; \
	elif [ "$(SKIP_UP)" = "auto" ] && ! curl -sf -o /dev/null --max-time 5 "$(API_GW_BANK_A_URL)/healthz"; then \
	  echo "[scenario-a] stack not detected at $(API_GW_BANK_A_URL) — bringing it up via spoke-all..."; \
	  $(MAKE) spoke-all; \
	else \
	  echo "[scenario-a] stack detected — running against the live stack..."; \
	fi
	@cd tests/integration && \
	  SKIP_UP=1 \
	  SKIP_DOWN=$(SKIP_DOWN) \
	  API_GW_BANK_A_URL=$(API_GW_BANK_A_URL) \
	  API_GW_BANK_B_URL=$(API_GW_BANK_B_URL) \
	  API_GW_BANK_D_URL=$(API_GW_BANK_D_URL) \
	  API_GW_CENTRAL_BANK_A_URL=$(API_GW_CENTRAL_BANK_A_URL) \
	  API_GW_CENTRAL_BANK_B_URL=$(API_GW_CENTRAL_BANK_B_URL) \
	  $(if $(EVIDENCE_DIR),EVIDENCE_DIR=$(EVIDENCE_DIR),) \
	  $(if $(ONBOARD),ONBOARD=$(ONBOARD),) \
	  BESU_SPOKE_A_RPC=$(BESU_SPOKE_A_RPC) \
	  BESU_SPOKE_B_RPC=$(BESU_SPOKE_B_RPC) \
	  go test -v -count=1 -tags integration -timeout 30m -run TestFullHappyPath ./...

scenario-a.test-integration-up: ## Bring the full stack up via `make spoke-all`, then run the happy-path test
	@$(MAKE) scenario-a.test-integration SKIP_UP=0 SKIP_DOWN=$(SKIP_DOWN)

# ── On-chain evidence capture (D12 P0-D12-1) ─────────────────────────────────
# Run the instrumented happy path against a live stack so the harness records each
# step's tx_hash + block_number + gas_used from the Besu RPC, then regenerate the
# machine-readable evidence bundle with those populated on-chain fields.
BESU_SPOKE_A_RPC ?= http://localhost:8645
BESU_SPOKE_B_RPC ?= http://localhost:8745
EVIDENCE_DIR     ?= $(CURDIR)/../evidence-bundles/_harness

evidence.e2e-a: ## Capture live on-chain evidence (tx_hash/block/gas) and regenerate the Scenario A bundle
	@echo "[scenario-a] capturing on-chain evidence to $(EVIDENCE_DIR)..."
	@mkdir -p "$(EVIDENCE_DIR)"
	@$(MAKE) scenario-a.test-integration \
	  EVIDENCE_DIR="$(EVIDENCE_DIR)" \
	  BESU_SPOKE_A_RPC=$(BESU_SPOKE_A_RPC) \
	  BESU_SPOKE_B_RPC=$(BESU_SPOKE_B_RPC)
	@echo "[scenario-a] folding capture into the Scenario A evidence bundle..."
	@python3 ../tools/gen_evidence_bundles.py e2e-scenario-a

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

# Full happy-path settlement benchmark ONLY (FX + cross-spoke HTLC, end to end) —
# the Scenario A analogue of scenario-b's cross-currency measurement. Zero-config:
# mints originator (bank-a) + custodian (bank-d) tokens, provisions tCeBM to both,
# then drives the complete flow. Relay-bound; set HAPPY_VUS / HAPPY_DURATION to tune.
scenario-a.perf-happy-path:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required"; exit 1; }
	@echo "[scenario-a] full happy-path settlement benchmark (FX + cross-spoke HTLC)..."
	@PERF_ONLY_HAPPY=1 bash tests/performance/run-all.sh

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

.PHONY: test.api-gateway test.auth test.compliance test.all scenario-a.test-integration scenario-a.test-integration-up evidence.e2e-a scenario-a.test-backend-coverage \
	scenario-a.perf-baseline scenario-a.perf-transfer scenario-a.perf-zeto scenario-a.perf-soak \
	scenario-a.perf-happy-path scenario-a.perf-all scenario-a.perf-all-dry scenario-a.perf-soak-all
