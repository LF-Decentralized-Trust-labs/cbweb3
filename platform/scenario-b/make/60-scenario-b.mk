# make/60-scenario-b.mk — Scenario B (Hub-and-Spoke Liquidity Pool) orchestration.
#
# Scenario B reuses the transverse infrastructure of Scenario A:
#   * Keycloak + Postgres + Redis   (deploy.up-infra)
#   * Hub Besu     (independent network, chain 1337, RPC port 8845)
#   * Spoke-A Besu  (deploy.up-spoke-a, chain 1338, RPC port 8645)
#   * Spoke-B Besu  (deploy.up-spoke-b, chain 1339, RPC port 8745)
#   * Hyperledger Cacti Relayer    (scenario-b.up-relayer)
#
# The Hub runs on its own Besu network (chain 1337) for AMM contracts, fully
# isolated from the spokes. Override BESU_HUB_RPC in the env to point at a
# different Hub endpoint.

# ── Feature toggle: MLP Path B ──────────────────────────────────────────────────
# Loaded from deploy/local/.env (gitignored). Copy from deploy/local/.env.example.
-include deploy/local/.env
export ENABLE_MLP
export MLP_ADDRESS

# ── RPC defaults (override via env) ──────────────────────────────────────────
BESU_HUB_RPC ?= http://localhost:8845
SPOKE_A_RPC  ?= http://localhost:8645
SPOKE_B_RPC  ?= http://localhost:8745
KEYCLOAK_URL    ?= http://localhost:8081
API_GW_URL      ?= http://localhost:18080
API_GW_BANK_A_URL         ?= http://localhost:18080
API_GW_BANK_B_URL         ?= http://localhost:28080
API_GW_CENTRAL_BANK_A_URL ?= http://localhost:38080
API_GW_CENTRAL_BANK_B_URL ?= http://localhost:60080
CACTI_RELAYER_URL ?= http://localhost:4000

SCENARIO_B_ENV := \
	BESU_HUB_RPC=$(BESU_HUB_RPC) \
	SPOKE_A_RPC=$(SPOKE_A_RPC) \
	SPOKE_B_RPC=$(SPOKE_B_RPC) \
	KEYCLOAK_URL=$(KEYCLOAK_URL) \
	API_GW_URL=$(API_GW_URL) \
	CACTI_RELAYER_URL=$(CACTI_RELAYER_URL)

# ── Infrastructure (reuses Scenario A infra) ─────────────────────────────────

scenario-b.prepare-pki: pki.gen-all
	@echo "[scenario-b] PKI prepared (idempotent; use FORCE=1 to regenerate)"

scenario-b.up-infra: deploy.up-infra deploy.up-besu
	@echo "[scenario-b] shared infra + besu spokes up"

scenario-b.down-infra: deploy.down-besu deploy.down-infra
	@echo "[scenario-b] shared infra + besu spokes down"

# ── Relayer (Cacti) ──────────────────────────────────────────────────────────

scenario-b.up-relayer: cacti-up
	@echo "[scenario-b] Cacti Relayer up at $(CACTI_RELAYER_URL)"

scenario-b.down-relayer: cacti-down
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

scenario-b.up-backend: scenario-b.build-backend-images deploy.up-backend-entities
	@echo "[scenario-b] backend services (api-gateway v2, payment-orchestrator, compliance) up"

scenario-b.down-backend: deploy.down-backend-entities
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
	  DURATION_SECS=0 nohup ./deploy/local/tools/mock-fx-feeder.sh > $(FX_FEEDER_LOG) 2>&1 & \
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
scenario-b.up: scenario-b.down-fx-feeder scenario-b.prepare-pki scenario-b.up-infra scenario-b.deploy-contracts scenario-b.up-relayer scenario-b.up-backend scenario-b.up-fx-feeder noc.setup-keycloak noc.up noc.setup-agents
	@echo "[scenario-b] full stack up — ready for tryout (bash tryouts/tryout-scenario-b-e2e.sh)"

# Perf-lean bring-up: the full settlement stack (infra + contracts + relayer + backend +
# fx-feeder) WITHOUT the NOC operations portal (noc.setup-keycloak/noc.up/noc.setup-agents).
# The NOC portal is a monitoring frontend and is not on the perf path; excluding it keeps the
# R1-12.3 perf harness (scenario-b.perf-all) from depending on the NOC frontend build.
scenario-b.up-perf: scenario-b.down-fx-feeder scenario-b.prepare-pki scenario-b.up-infra scenario-b.deploy-contracts scenario-b.up-relayer scenario-b.up-backend scenario-b.up-fx-feeder
	@echo "[scenario-b] perf stack up (no NOC portal) — ready for make scenario-b.perf-all"

scenario-b.down: scenario-b.down-fx-feeder scenario-b.down-backend scenario-b.down-relayer scenario-b.down-infra noc.down
	@echo "[scenario-b] full stack down"

scenario-b.restart: scenario-b.down scenario-b.up

# ── Full wipe ────────────────────────────────────────────────────────────────
# scenario-b.nuke — best-effort total teardown for a guaranteed clean slate.
# Unlike `scenario-b.down`, this also removes the frontend dev-server containers
# and force-clears anything a partial/failed down may have left behind, including
# the `local_postgres_data` Postgres volume (the one a bare re-`up` never drops,
# which otherwise keeps stale pool_commits / bridged_asset_positions across deploys).
# All steps are best-effort (errors ignored) so it always reaches the force-clean.
scenario-b.nuke:
	@echo "[scenario-b] NUKE — tearing down the entire stack + volumes..."
	-@$(MAKE) scenario-b.down
	-@$(MAKE) noc.down
	-@$(MAKE) frontend-scenario-b-down
	@echo "[scenario-b] force-removing any leftover containers..."
	-@docker rm -f $$(docker ps -aq --filter name=cbweb3 --filter name=backend-) 2>/dev/null || true
	@echo "[scenario-b] removing Postgres volume (local_postgres_data)..."
	-@docker volume rm local_postgres_data 2>/dev/null || true
	@echo "[scenario-b] verifying clean state..."
	@docker ps -a --format '{{.Names}}' | grep -E 'cbweb3|backend-' && echo "  WARN: containers still present (see above)" || echo "  OK: no cbweb3 containers"
	@docker volume ls --format '{{.Name}}' | grep -E 'local_postgres_data' && echo "  WARN: postgres volume still present" || echo "  OK: no postgres volume"
	@ls -d deploy/local/*/nodes/*/data 2>/dev/null && echo "  WARN: besu chain data still present" || echo "  OK: no besu chain data"
	@echo "[scenario-b] nuke complete — run 'make scenario-b.up' for a fresh stack"

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

# ── Integration test (full happy-path API test) ──────────────────────────────

SKIP_UP   ?= 1
SKIP_DOWN ?= 1

scenario-b.test-integration: ## Run full happy-path API integration test against a live stack
	@echo "[scenario-b] running integration test (full happy path)..."
ifeq ($(SKIP_UP),0)
	@echo "[scenario-b] bringing stack up (SKIP_UP=0)..."
	@$(MAKE) scenario-b.up
endif
	@cd tests/integration && \
	  SKIP_UP=1 \
	  SKIP_DOWN=$(SKIP_DOWN) \
	  KEYCLOAK_URL=$(KEYCLOAK_URL) \
	  API_GW_BANK_A_URL=$(API_GW_BANK_A_URL) \
	  API_GW_BANK_B_URL=$(API_GW_BANK_B_URL) \
	  API_GW_CENTRAL_BANK_A_URL=$(API_GW_CENTRAL_BANK_A_URL) \
	  API_GW_CENTRAL_BANK_B_URL=$(API_GW_CENTRAL_BANK_B_URL) \
	  $(if $(EVIDENCE_DIR),EVIDENCE_DIR=$(EVIDENCE_DIR),) \
	  BESU_HUB_RPC=$(BESU_HUB_RPC) \
	  BESU_SPOKE_B_RPC=$(BESU_SPOKE_B_RPC) \
	  go test -v -count=1 -tags integration -timeout 30m -run TestFullHappyPath ./...

# ── On-chain evidence capture (D12 P0-D12-1) ─────────────────────────────────
# Run the instrumented happy path against a live stack so the harness records each
# step's tx_hash + block_number + gas_used from the Besu RPC, then regenerate the
# machine-readable evidence bundle with those populated on-chain fields.
BESU_HUB_RPC     ?= http://localhost:8845
BESU_SPOKE_B_RPC ?= http://localhost:8745
EVIDENCE_DIR     ?= $(CURDIR)/../evidence-bundles/_harness

evidence.e2e-b: ## Capture live on-chain evidence (tx_hash/block/gas) and regenerate the Scenario B bundle
	@echo "[scenario-b] capturing on-chain evidence to $(EVIDENCE_DIR)..."
	@mkdir -p "$(EVIDENCE_DIR)"
	@$(MAKE) scenario-b.test-integration \
	  EVIDENCE_DIR="$(EVIDENCE_DIR)" \
	  BESU_HUB_RPC=$(BESU_HUB_RPC) \
	  BESU_SPOKE_B_RPC=$(BESU_SPOKE_B_RPC)
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
	scenario-b.test-integration evidence.e2e-b \
	scenario-b.perf-baseline scenario-b.validate-openapi \
	scenario-b.gen-postman scenario-b.check-postman \
	scenario-b.perf-amm-throughput scenario-b.perf-transfer \
	scenario-b.perf-zeto scenario-b.perf-soak scenario-b.perf-all scenario-b.perf-smoke
