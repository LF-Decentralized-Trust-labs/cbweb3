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
# Drives the real REST API of a running toolkit stack (samples/deploy-all.sh) through the
# Scenario A core workflow (login → mint → FX propose/accept → HTLC lock both
# legs → settle → relay-settle → verify). Counterpart to the hermetic
# integration_lite lane; gated behind the `integration` build tag so the two
# never compile together.
#
#   make scenario-a.test-integration              # run against a live toolkit stack
#   SKIP_DOWN=0 make scenario-a.test-integration  # tear the stack down afterwards
#
# This target does NOT provision. It used to call `spoke-all`, the legacy deploy/local
# bring-up, which was removed in favour of the toolkit as the single provisioning path.
# Bring a stack up first:  cd samples && ./deploy-all.sh
#
# Endpoints, operator logins and Paladin identities are DERIVED from the toolkit
# manifests by tests/integration/toolkit-env.sh — defaults follow samples/deploy-all.sh
# (Brazil = spoke-a, Colombia = spoke-b). Any value passed on the command line or
# exported wins over the derived one, so a different topology only has to name what
# differs, e.g.
#
#   make scenario-a.test-integration API_GW_BANK_A_URL=http://localhost:18646
#   CB_A_MANIFEST=/path/to/cb.yaml make scenario-a.test-integration
#
# SKIP_UP is gone: the suite can no longer create or destroy a stack.
SKIP_DOWN ?= 1

TOOLKIT_ENV := tests/integration/toolkit-env.sh

scenario-a.test-integration: ## Run the happy-path test against an ALREADY-RUNNING toolkit stack
	@echo "[scenario-a] deriving endpoints from the toolkit manifests..."
	@# The script is run under BASH and its `export …` output eval'd, rather than
	@# sourced: recipes run under /bin/sh (dash here), which rejects the script's
	@# `set -o pipefail`. Values are printf %q-quoted, so the eval is safe.
	@#
	@# Caller-supplied make variables become environment FIRST, so the script keeps
	@# them (it only fills in what is unset). evidence.e2e-a relies on this.
	@$(if $(BESU_SPOKE_A_RPC),export BESU_SPOKE_A_RPC="$(BESU_SPOKE_A_RPC)";) \
	 $(if $(BESU_SPOKE_B_RPC),export BESU_SPOKE_B_RPC="$(BESU_SPOKE_B_RPC)";) \
	 eval "$$(bash ./$(TOOLKIT_ENV))"; \
	  if ! curl -sf -o /dev/null --max-time 5 "$$API_GW_BANK_A_URL/healthz"; then \
	    echo "[scenario-a] no stack answering at $$API_GW_BANK_A_URL."; \
	    echo "[scenario-a] Bring one up with the toolkit first:  cd samples && ./deploy-all.sh"; \
	    exit 1; \
	  fi; \
	  echo "[scenario-a] stack detected — running the full happy path..."; \
	  cd tests/integration && \
	  SKIP_UP=1 SKIP_DOWN=$(SKIP_DOWN) \
	  $(if $(EVIDENCE_DIR),EVIDENCE_DIR=$(EVIDENCE_DIR),) \
	  $(if $(ONBOARD),ONBOARD=$(ONBOARD),) \
	  go test -v -count=1 -tags integration -timeout 30m -run TestFullHappyPath ./...

scenario-a.test-integration-env: ## Print the endpoints/credentials the happy-path test would use
	@bash $(TOOLKIT_ENV)

# ── On-chain evidence capture (D12 P0-D12-1) ─────────────────────────────────
# Run the instrumented happy path against a live stack so the harness records each
# step's tx_hash + block_number + gas_used from the Besu RPC, then regenerate the
# machine-readable evidence bundle with those populated on-chain fields.
EVIDENCE_DIR     ?= $(CURDIR)/../evidence-bundles/_harness

evidence.e2e-a: ## Capture live on-chain evidence (tx_hash/block/gas) and regenerate the Scenario A bundle
	@echo "[scenario-a] capturing on-chain evidence to $(EVIDENCE_DIR)..."
	@mkdir -p "$(EVIDENCE_DIR)"
	@# The Besu RPCs are no longer forwarded here: toolkit-env.sh derives them from
	@# the same manifests as the gateways, so a hand-passed pair could disagree with
	@# the topology under test. A command-line override still reaches the sub-make.
	@$(MAKE) scenario-a.test-integration EVIDENCE_DIR="$(EVIDENCE_DIR)"
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

# Endpoints come from the toolkit manifests (tests/integration/toolkit-env.sh), the same
# source the integration suite uses. The default here used to be :18080 — a port of the
# removed legacy stack — so every perf target silently probed a gateway nothing serves
# (DEF-022). PERF_GW_ENV derives them and still lets an explicit override win:
# `make scenario-a.perf-baseline API_GW_URL=...` takes precedence.
PERF_GW_ENV = eval "$$(bash ./$(TOOLKIT_ENV))"; \
	  API_GW_URL="$(if $(API_GW_URL),$(API_GW_URL),$$API_GW_BANK_A_URL)"; \
	  PERF_CUSTODIAN_GW_URL="$(if $(PERF_CUSTODIAN_GW_URL),$(PERF_CUSTODIAN_GW_URL),$$API_GW_BANK_D_URL)"; \
	  export API_GW_URL PERF_CUSTODIAN_GW_URL API_GW_CENTRAL_BANK_A_URL;

scenario-a.perf-baseline:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] running API latency baseline (read + write p95)..."
	@$(PERF_GW_ENV) AUTH_TOKEN=$(AUTH_TOKEN) \
	  RECEIVER=$${RECEIVER:-funded_operator@spoke-a-bank-c} \
	  k6 run tests/performance/scenario-a-perf.js

scenario-a.perf-transfer:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] HTLC token-transfer throughput — 50 TPS target (R1-12.3 threshold 1)..."
	@$(PERF_GW_ENV) AUTH_TOKEN=$(AUTH_TOKEN) \
	  RECEIVER=$${RECEIVER:-funded_operator@spoke-a-bank-c} \
	  TRANSFER_TPS=$${TRANSFER_TPS:-50} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/k6/htlc-transfer-throughput.js

scenario-a.perf-zeto:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] Zeto escrow throughput — 15 TPS target (R1-12.3 threshold 2)..."
	@$(PERF_GW_ENV) AUTH_TOKEN=$(AUTH_TOKEN) \
	  ZETO_TPS=$${ZETO_TPS:-15} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/k6/zeto-escrow-throughput.js

scenario-a.perf-soak:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-a] 12-hour SOAK — dedicated infra only, NOT for CI..."
	@$(PERF_GW_ENV) AUTH_TOKEN=$(AUTH_TOKEN) \
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
	@$(PERF_GW_ENV) PERF_ONLY_HAPPY=1 bash tests/performance/run-all.sh

# ── R1-12.3 zero-config orchestration ───────────────────────────────────────
# perf-all takes NO required args: it stands up the stack (lib/stack.sh), mints
# AUTH_TOKEN via /auth/login (lib/auth.sh), funds the sender (lib/fund.sh), runs
# baseline + 50 TPS HTLC + 15 TPS Zeto, correlates on-chain TTF (lib/ttf.sh), and
# writes docs/performance/RESULTS-<UTC>.md (lib/results.sh).
scenario-a.perf-all:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required"; exit 1; }
	@echo "[scenario-a] R1-12.3 performance suite (perf-all)..."
	@$(PERF_GW_ENV) bash tests/performance/run-all.sh

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

.PHONY: test.api-gateway test.auth test.compliance test.all scenario-a.test-integration scenario-a.test-integration-env evidence.e2e-a scenario-a.test-backend-coverage \
	scenario-a.perf-baseline scenario-a.perf-transfer scenario-a.perf-zeto scenario-a.perf-soak \
	scenario-a.perf-happy-path scenario-a.perf-all scenario-a.perf-all-dry scenario-a.perf-soak-all
