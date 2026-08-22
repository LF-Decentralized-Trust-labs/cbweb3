# make/60-scenario-b.mk — Scenario B (Hub-and-Spoke Liquidity Pool) orchestration.
#
# The stack is brought up by the TOOLKIT — `samples/deploy-all.sh`, which runs
# `cbweb3b apply` per entity. The legacy `deploy/local` bring-up that used to live in
# make/10-deploy.mk and make/15-dev.mk was removed: it duplicated the toolkit and kept
# falling behind it (the Besu pin never reached it, credentials and Redis auth only did
# because one PR touched both trees).
#
# What remains here are the targets that act ON a running stack rather than create one:
# contracts, tryouts, performance baselines, evidence capture, tests and Postman. They
# find the stack through URLs and ports, so they work against whatever provisioned it —
# but their DEFAULTS still point at the ports the legacy stack published (hub 8845,
# spokes 8645/8745). Pointing them at the toolkit's ports is tracked in the migration
# card for this suite; until then, pass the URLs explicitly.
#
# The Hub runs on its own Besu network for the AMM contracts, isolated from the spokes —
# see docs/decisions/ADR-006. Override BESU_HUB_RPC to point at a different Hub.

# ── Feature toggle: MLP Path B ──────────────────────────────────────────────────
export ENABLE_MLP
export MLP_ADDRESS

# ── Endpoints for the tryouts ────────────────────────────────────────────────
# The defaults that used to live here were the legacy deploy/local ports
# (18080/28080/38080/60080, spokes 8645/8745, Keycloak 8081, relay 4000). That
# bring-up was removed, so they injected addresses nothing answers on — and because
# they were injected, they OVERRODE whatever the tryout derived for itself.
#
# The tryout now sources tests/integration/toolkit-env.sh, which reads the same
# manifests the toolkit was applied with. Anything set on the command line still wins,
# so only the values that actually differ have to be named:
#
#   make scenario-b.tryout-us1 API_GW_BANK_A_URL=http://localhost:41646
#
# SCENARIO_B_ENV forwards only what a caller explicitly set; unset variables are left
# alone so the deriver can fill them.
SCENARIO_B_ENV := \
	$(if $(BESU_HUB_RPC),BESU_HUB_RPC=$(BESU_HUB_RPC),) \
	$(if $(SPOKE_A_RPC),SPOKE_A_RPC=$(SPOKE_A_RPC),) \
	$(if $(SPOKE_B_RPC),SPOKE_B_RPC=$(SPOKE_B_RPC),) \
	$(if $(API_GW_BANK_A_URL),API_GW_BANK_A_URL=$(API_GW_BANK_A_URL),) \
	$(if $(API_GW_CENTRAL_BANK_A_URL),API_GW_CENTRAL_BANK_A_URL=$(API_GW_CENTRAL_BANK_A_URL),) \
	$(if $(API_GW_CENTRAL_BANK_B_URL),API_GW_CENTRAL_BANK_B_URL=$(API_GW_CENTRAL_BANK_B_URL),) \
	$(if $(CACTI_RELAYER_URL),CACTI_RELAYER_URL=$(CACTI_RELAYER_URL),) \
	$(if $(POOL_PAIR),POOL_PAIR=$(POOL_PAIR),)

# ── Infrastructure (reuses Scenario A infra) ─────────────────────────────────

scenario-b.prepare-pki: pki.gen-all
	@echo "[scenario-b] PKI prepared (idempotent; use FORCE=1 to regenerate)"



# ── Relayer (Cacti) ──────────────────────────────────────────────────────────

scenario-b.up-relayer:
	@bash provisioning/scripts/start-cacti.sh
	@echo "[scenario-b] Cacti Relayer up at $(CACTI_RELAYER_URL)"

scenario-b.down-relayer:
	@docker compose -p cacti down -v --remove-orphans
	@echo "[scenario-b] Cacti Relayer down"

# ── Contracts (AMM on Hub, SpokeBridge on each Spoke) ────────────────────────

scenario-b.deploy-contracts: contracts.setup contracts.build contracts.deploy-hub contracts.deploy-spoke-a contracts.deploy-spoke-b contracts.sync-addresses contracts.register-participants scenario-b.seed-sovereign-pair contracts.grant-liquidity-providers
	@echo "[scenario-b] deployed Hub (IdentityRegistry + Tokens + AMM) + Spokes + Participants + LiquidityCommitRegistry + LP grants"
	@# contracts.seed-hub removido: pool seeding é feito via commit-reveal cooperativo (step4b)

# ── Backend services (Scenario B v2 API) ────────────────────────────────────

# ── Sovereign CB liquidity pair seeding (008-fix-cb-liquidity) ───────────────
# Deploys LiquidityCommitRegistry + W-tCeBM sovereign tokens + sovereign AMM,
# registers the W-BRL-ARS pair on PairRegistry, then re-runs contracts.sync-addresses
# so LIQUIDITY_COMMIT_REGISTRY_ADDRESS is propagated to all backend .env.infra.* files
# and to interop/hub-and-spoke/cacti/.env (enabling the CommitMatched watcher).
#
# Keys come from contracts/.env (loaded via make/30-contracts.mk -include).
# Hub addresses are read from backend/config/.env.infra.central-bank-a (written by
# contracts.sync-addresses, which must run before this target).
SOVEREIGN_TOKEN_A ?= BRL
SOVEREIGN_TOKEN_B ?= ARS
SOVEREIGN_PAIR_ID ?= W-BRL-ARS

scenario-b.seed-sovereign-pair:
	@echo "[scenario-b] Seeding sovereign pair $(SOVEREIGN_PAIR_ID) on Hub..."
	@test -n "$(ADMIN_PRIVATE_KEY)"          || (echo "ERROR: ADMIN_PRIVATE_KEY not set — check contracts/.env"; exit 1)
	@test -n "$(CENTRAL_BANK_A_PRIVATE_KEY)" || (echo "ERROR: CENTRAL_BANK_A_PRIVATE_KEY not set — check contracts/.env"; exit 1)
	@test -n "$(CENTRAL_BANK_B_PRIVATE_KEY)" || (echo "ERROR: CENTRAL_BANK_B_PRIVATE_KEY not set — check contracts/.env"; exit 1)
	@test -n "$(ADMIN_ADDRESS)"              || (echo "ERROR: ADMIN_ADDRESS not set — check contracts/.env"; exit 1)
	@HUB_IR=$$(grep '^HUB_IDENTITY_REGISTRY_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2-) && \
	 PAIR_REG=$$(grep '^PAIR_REGISTRY_CONTRACT_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2-) && \
	 test -n "$$HUB_IR"   || (echo "ERROR: HUB_IDENTITY_REGISTRY_ADDRESS missing from backend/config/.env.infra.central-bank-a — run contracts.sync-addresses first"; exit 1) && \
	 test -n "$$PAIR_REG" || (echo "ERROR: PAIR_REGISTRY_CONTRACT_ADDRESS missing from backend/config/.env.infra.central-bank-a — run contracts.sync-addresses first"; exit 1) && \
	 $(MAKE) contracts.seed-sovereign-pair \
	   TOKEN_SYMBOL_A=$(SOVEREIGN_TOKEN_A) \
	   TOKEN_SYMBOL_B=$(SOVEREIGN_TOKEN_B) \
	   PAIR_ID=$(SOVEREIGN_PAIR_ID) \
	   CB_A_HUB_PRIVATE_KEY=$(CENTRAL_BANK_A_PRIVATE_KEY) \
	   CB_B_HUB_PRIVATE_KEY=$(CENTRAL_BANK_B_PRIVATE_KEY) \
	   ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
	   ADMIN_ADDRESS=$(ADMIN_ADDRESS) \
	   HUB_IDENTITY_REGISTRY=$$HUB_IR \
	   PAIR_REGISTRY_ADDRESS=$$PAIR_REG
	@echo "[scenario-b] Propagating LIQUIDITY_COMMIT_REGISTRY_ADDRESS to env files and Cacti..."
	@$(MAKE) contracts.sync-addresses SOVEREIGN_PAIR_ID=$(SOVEREIGN_PAIR_ID)
	@echo "[scenario-b] Sovereign pair $(SOVEREIGN_PAIR_ID) seeded — LCR address propagated"


scenario-b.build-backend-images:
	@echo "[scenario-b] building backend Docker images (compliance, auth, payment-orchestrator, api-gateway)..."
	@bash $(CURDIR)/build-backend-images.sh $(or $(TAG),local) $(or $(VERSION),latest)



# ── MLP backend services (opt-in: ENABLE_MLP=true) ───────────────────────────

scenario-b.up-backend-mlp:
	@echo "[scenario-b] MLP backend stack up (compliance-mlp, auth-mlp, payment-orchestrator-mlp, api-gateway-mlp)"
	@docker compose -f backend/docker-compose-backend.mlp.yaml up -d

scenario-b.down-backend-mlp:
	@echo "[scenario-b] MLP backend stack down"
	@docker compose -f backend/docker-compose-backend.mlp.yaml down --remove-orphans

scenario-b.tryout-us2-mlp:
	@echo "[scenario-b] E2E tryout — US2 MLP (requires ENABLE_MLP=true and MLP stack running)"
	@ENABLE_MLP=true $(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh us2
# ── Mock FX-rate feeder (system-managed oracle) ──────────────────────────────
# Pushes a real-life-like, slightly-jittered BRL/ARS rate into the Hub ManualOracle
# on an interval, standing in for an external price feed. The api-gateway reads this
# rate to suggest a counterpart match amount during liquidity coordination. Runs as a
# detached host process (DURATION_SECS=0 = until stopped) so it lives with the stack;
# launching never blocks or fails `up` (the oracle/cast may not be ready — that's fine).
FX_FEEDER_PID := /tmp/cbweb3-mock-fx-feeder.pid
FX_FEEDER_LOG := /tmp/cbweb3-mock-fx-feeder.log

scenario-b.up-fx-feeder:
	@echo "[scenario-b] starting mock FX-rate feeder (BRL/ARS → Hub ManualOracle)..."
	@if [ -f $(FX_FEEDER_PID) ] && kill -0 $$(cat $(FX_FEEDER_PID)) 2>/dev/null; then \
	  echo "  already running (pid $$(cat $(FX_FEEDER_PID)))"; \
	else \
	  DURATION_SECS=0 nohup ./tools/mock-fx-feeder.sh > $(FX_FEEDER_LOG) 2>&1 & \
	  echo $$! > $(FX_FEEDER_PID); \
	  echo "  started (pid $$(cat $(FX_FEEDER_PID))) — log: $(FX_FEEDER_LOG)"; \
	fi

scenario-b.down-fx-feeder:
	@if [ -f $(FX_FEEDER_PID) ]; then \
	  kill $$(cat $(FX_FEEDER_PID)) 2>/dev/null || true; rm -f $(FX_FEEDER_PID); \
	  echo "[scenario-b] mock FX-rate feeder stopped"; \
	fi

# ── Stack targets ────────────────────────────────────────────────────────────

# down-fx-feeder runs first: the feeder signs setRate with the admin/deployer key, the
# same account forge uses in deploy-contracts — a stale feeder from a prior `up` would
# race the deploy's nonce. up-fx-feeder restarts it fresh at the end.

# Perf-lean bring-up: the full settlement stack (infra + contracts + relayer + backend +
# fx-feeder) WITHOUT the NOC operations portal (noc.setup-keycloak/noc.up/noc.setup-agents).
# The NOC portal is a monitoring frontend and is not on the perf path; excluding it keeps the
# R1-12.3 perf harness (scenario-b.perf-all) from depending on the NOC frontend build.



# ── Full wipe ────────────────────────────────────────────────────────────────
# scenario-b.nuke — best-effort total teardown for a guaranteed clean slate.
# Unlike `scenario-b.down`, this also removes the frontend dev-server containers
# and force-clears anything a partial/failed down may have left behind, including
# the `local_postgres_data` Postgres volume (the one a bare re-`up` never drops,
# which otherwise keeps stale pool_commits / bridged_asset_positions across deploys).
# All steps are best-effort (errors ignored) so it always reaches the force-clean.
scenario-b.nuke:
	@echo "[scenario-b] NUKE — tearing down the entire stack + volumes..."
	-@$(MAKE) frontend-scenario-b-down
	@echo "[scenario-b] force-removing any leftover containers..."
	-@docker rm -f $$(docker ps -aq --filter name=cbweb3 --filter name=backend-) 2>/dev/null || true
	@echo "[scenario-b] removing Postgres volume (local_postgres_data)..."
	-@docker volume rm local_postgres_data 2>/dev/null || true
	@echo "[scenario-b] verifying clean state..."
	@docker ps -a --format '{{.Names}}' | grep -E 'cbweb3|backend-' && echo "  WARN: containers still present (see above)" || echo "  OK: no cbweb3 containers"
	@docker volume ls --format '{{.Name}}' | grep -E 'local_postgres_data' && echo "  WARN: postgres volume still present" || echo "  OK: no postgres volume"
	@docker volume ls --format '{{.Name}}' | grep -E '_besu_data|_genesis' && echo "  WARN: besu volumes still present (toolkit-provisioned state)" || echo "  OK: no besu volumes"
	@echo "[scenario-b] nuke complete — bring a fresh stack up with: cd samples && ./deploy-all.sh"

# ── Tests ────────────────────────────────────────────────────────────────────

scenario-b.test-contracts:
	@echo "[scenario-b] forge test for AMM + Circuit Breaker asymmetric..."
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge test -vv \
		--match-contract "AutomatedMarketMakerTest|CBWeb3HubTest|SpokeBridgeTest"

scenario-b.test-backend:
	@echo "[scenario-b] go test for backend services..."
	@cd backend/services/api-gateway && go test ./...
	@cd backend/services/payment-orchestrator && go test ./...
	@cd backend/services/compliance && go test ./...

scenario-b.test: scenario-b.test-contracts scenario-b.test-backend

# ── End-to-end tryout ────────────────────────────────────────────────────────

scenario-b.tryout:
	@echo "[scenario-b] running E2E tryout (all stories)..."
	@$(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh all

scenario-b.tryout-us1:
	@$(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh us1

scenario-b.tryout-us2:
	@$(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh us2

scenario-b.tryout-us3:
	@$(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh us3

# us5 and us6 have always been dispatchable in the script — its own help names them —
# but no target invoked them, so the PairRegistry and CurrencyRegistry stories were
# unreachable through make and never ran.
scenario-b.tryout-us5:
	@$(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh us5

scenario-b.tryout-us6:
	@$(SCENARIO_B_ENV) bash tryouts/tryout-scenario-b-e2e.sh us6

# ── Integration test (full happy-path API test) ──────────────────────────────
# This target does NOT provision. Bring a stack up first:
#   cd samples && ./deploy-all.sh
#
# Endpoints, Besu RPCs and operator logins are DERIVED from the toolkit manifests by
# tests/integration/toolkit-env.sh — defaults follow samples/deploy-all.sh (Brazil =
# spoke-a, Argentina = spoke-b, through the neutral hub). Anything passed on the
# command line or exported wins over the derived value, e.g.
#
#   make scenario-b.test-integration API_GW_BANK_A_URL=http://localhost:41646
#   CB_B_MANIFEST=/path/to/cb.yaml make scenario-b.test-integration
#
# SKIP_UP is gone: the suite can no longer create or destroy a stack.
SKIP_DOWN ?= 1

TOOLKIT_ENV := tests/integration/toolkit-env.sh

scenario-b.test-integration: ## Run the happy-path test against an ALREADY-RUNNING toolkit stack
	@echo "[scenario-b] deriving endpoints from the toolkit manifests..."
	@# The script runs under BASH and its `export …` output is eval'd, rather than
	@# sourced: recipes run under /bin/sh (dash here), which rejects the script's
	@# `set -o pipefail`. Values are printf %q-quoted, so the eval is safe.
	@eval "$$(bash ./$(TOOLKIT_ENV))"; \
	  if ! curl -sf -o /dev/null --max-time 5 "$$API_GW_BANK_A_URL/healthz"; then \
	    echo "[scenario-b] no stack answering at $$API_GW_BANK_A_URL."; \
	    echo "[scenario-b] Bring one up with the toolkit first:  cd samples && ./deploy-all.sh"; \
	    exit 1; \
	  fi; \
	  echo "[scenario-b] stack detected — running the full happy path..."; \
	  cd tests/integration && \
	  SKIP_UP=1 SKIP_DOWN=$(SKIP_DOWN) \
	  $(if $(EVIDENCE_DIR),EVIDENCE_DIR=$(EVIDENCE_DIR),) \
	  go test -v -count=1 -tags integration -timeout 30m -run TestFullHappyPath ./...

scenario-b.test-integration-env: ## Print the endpoints/credentials the happy-path test would use
	@bash $(TOOLKIT_ENV)

# ── On-chain evidence capture (D12 P0-D12-1) ─────────────────────────────────
# Run the instrumented happy path against a live stack so the harness records each
# step's tx_hash + block_number + gas_used from the Besu RPC, then regenerate the
# machine-readable evidence bundle with those populated on-chain fields.
EVIDENCE_DIR     ?= $(CURDIR)/../evidence-bundles/_harness

evidence.e2e-b: ## Capture live on-chain evidence (tx_hash/block/gas) and regenerate the Scenario B bundle
	@echo "[scenario-b] capturing on-chain evidence to $(EVIDENCE_DIR)..."
	@mkdir -p "$(EVIDENCE_DIR)"
	@# The Besu RPCs are no longer forwarded here: toolkit-env.sh derives them from the
	@# same manifests as the gateways, so a hand-passed value could disagree with the
	@# topology under test. A command-line override still reaches the sub-make.
	@$(MAKE) scenario-b.test-integration EVIDENCE_DIR="$(EVIDENCE_DIR)"
	@echo "[scenario-b] folding capture into the Scenario B evidence bundle..."
	@python3 ../tools/gen_evidence_bundles.py e2e-scenario-b

# ── Performance baseline (T105) ──────────────────────────────────────────────

scenario-b.perf-baseline:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] running performance baseline (quote + swap p95)..."
	@API_GW_URL=$(API_GW_URL) k6 run tests/performance/scenario-b-perf.js

# ── R1-12.3 threshold harness (see scenario-b/docs/performance) ───────────────
# These drive the Report 1 / Finding 12.3 thresholds. AUTH_TOKEN (commercial_bank
# JWT) is REQUIRED for swap/transfer scenarios. Fill measured numbers into
# docs/performance/RESULTS-TEMPLATE.md after a real run — do not run the soak in CI.

scenario-b.perf-amm-throughput:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] AMM swap throughput — validating DRAFT 30 TPS target..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  LOAD_MODEL=rate SWAP_TPS=$${SWAP_TPS:-30} QUOTE_TPS=$${QUOTE_TPS:-60} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/scenario-b-perf.js

scenario-b.perf-transfer:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] value-transfer throughput — 50 TPS target..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  TRANSFER_TPS=$${TRANSFER_TPS:-50} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/k6/bridge-transfer-throughput.js

scenario-b.perf-zeto:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] Zeto privacy-transfer throughput — 15 TPS target..."
	@API_GW_URL=$(API_GW_URL) AUTH_TOKEN=$(AUTH_TOKEN) \
	  TOKEN_KIND=zeto TRANSFER_TPS=$${TRANSFER_TPS:-15} DURATION=$${DURATION:-10m} \
	  k6 run tests/performance/k6/bridge-transfer-throughput.js

# ── R1-12.3 ZERO-CONFIG orchestration ────────────────────────────────────────
# scenario-b.perf-all — ONE command, NO config, NO manual steps. Stands up the stack if
# needed, mints the auth tokens via the gateway login, applies the perf profile (clears
# transfer limits + asserts the circuit breaker is RESUMED before swaps), seeds 30-TPS-sized
# liquidity, runs every threshold benchmark with --summary-export, does on-chain TTF
# correlation, and writes measured numbers + PASS/FAIL into docs/performance/RESULTS.md.
# The 12h soak is SEPARATE (scenario-b.perf-soak). Override DURATION/SWAP_TPS/etc. if desired.
# scenario-b.perf-smoke — static + logic smoke test for the perf harness helpers. No infra, no
# k6 run; safe for CI. Validates syntax, JSON logging, and write-results PASS/FAIL/VALIDATED/REVISE.
scenario-b.perf-smoke:
	@echo "[scenario-b] perf harness smoke test (no infra)..."
	@bash tests/performance/lib/smoke_test.sh

scenario-b.perf-all:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] R1-12.3 full perf suite (zero-config) — see docs/performance/RESULTS.md..."
	@PERF_MAKE_DIR=$(CURDIR) \
	 API_GW_URL=$(API_GW_URL) \
	 API_GW_CENTRAL_BANK_A_URL=$(API_GW_CENTRAL_BANK_A_URL) \
	 bash tests/performance/run-all.sh

scenario-b.perf-soak:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] 12-hour SOAK — dedicated infra only, NOT for CI..."
	@PERF_MAKE_DIR=$(CURDIR) \
	 API_GW_URL=$(API_GW_URL) \
	 API_GW_CENTRAL_BANK_A_URL=$(API_GW_CENTRAL_BANK_A_URL) \
	 DURATION=$${DURATION:-12h} \
	 bash tests/performance/run-soak.sh

# ── API artifacts: OpenAPI validation + Postman generation (T108, R1-§8) ─────
#
# apis/postman/generate.sh is the supported entry point and the one CI runs —
# these three targets are thin aliases for its modes, kept because the surrounding
# make surface still documents them. It pins redocly and the Postman converter to
# exact versions; the previous target linted an unwired fragment with @latest and,
# because of a `||` guard, skipped linting entirely whenever redocly was installed.

scenario-b.validate-openapi:
	@bash apis/postman/generate.sh --lint-only

scenario-b.gen-postman:
	@bash apis/postman/generate.sh

# Fails when the committed collection no longer matches the served spec.
scenario-b.check-postman:
	@bash apis/postman/generate.sh --check

.PHONY: \
	scenario-b.up-infra scenario-b.down-infra \
	scenario-b.prepare-pki \
	scenario-b.up-relayer scenario-b.down-relayer \
	scenario-b.deploy-contracts scenario-b.seed-sovereign-pair \
	scenario-b.build-backend-images \
	scenario-b.up-backend scenario-b.down-backend \
	scenario-b.up-backend-mlp scenario-b.down-backend-mlp scenario-b.tryout-us2-mlp \
	scenario-b.up scenario-b.up-perf scenario-b.down scenario-b.restart scenario-b.nuke \
	scenario-b.test-contracts scenario-b.test-backend scenario-b.test \
	scenario-b.tryout scenario-b.tryout-us1 scenario-b.tryout-us2 scenario-b.tryout-us3 \
	scenario-b.tryout-us5 scenario-b.tryout-us6 \
	scenario-b.test-integration scenario-b.test-integration-env evidence.e2e-b \
	scenario-b.perf-baseline scenario-b.validate-openapi \
	scenario-b.gen-postman scenario-b.check-postman \
	scenario-b.perf-amm-throughput scenario-b.perf-transfer \
	scenario-b.perf-zeto scenario-b.perf-soak scenario-b.perf-all scenario-b.perf-smoke
