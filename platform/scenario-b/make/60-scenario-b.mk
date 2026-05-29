# make/60-scenario-b.mk — Scenario B (Hub-and-Spoke Liquidity Pool) orchestration.
#
# Scenario B reuses the transverse infrastructure of Scenario A:
#   * Keycloak + Postgres + Redis   (deploy.up-infra)
#   * Spoke-A Besu  (deploy.up-spoke-a, also playing the Hub role in local dev)
#   * Spoke-B Besu  (deploy.up-spoke-b)
#   * Hyperledger Cacti Relayer    (scenario-b.up-relayer)
#
# Design decision: to keep the local footprint small we use Spoke-A as the
# "Hub" for AMM contracts in dev. Production deployments may split the Hub
# into its own Besu network by setting BESU_HUB_RPC in the env.

# ── Feature toggle: MLP Path B ──────────────────────────────────────────────────
# Loaded from deploy/local/.env (gitignored). Copy from deploy/local/.env.example.
-include deploy/local/.env
export ENABLE_MLP
export MLP_ADDRESS

# ── RPC defaults (override via env) ──────────────────────────────────────────
BESU_HUB_RPC ?= http://localhost:8645
SPOKE_A_RPC  ?= http://localhost:8645
SPOKE_B_RPC  ?= http://localhost:8745
KEYCLOAK_URL    ?= http://localhost:8081
API_GW_URL      ?= http://localhost:3000
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

scenario-b.deploy-contracts: contracts.setup contracts.build contracts.deploy-hub contracts.deploy-spoke-a contracts.deploy-spoke-b contracts.sync-addresses contracts.register-participants scenario-b.seed-sovereign-pair
	@echo "[scenario-b] deployed Hub (IdentityRegistry + Tokens + AMM) + Spokes + Participants + LiquidityCommitRegistry"
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
	@test -n "$(CENTRAL_BANK_PRIVATE_KEY)"   || (echo "ERROR: CENTRAL_BANK_PRIVATE_KEY not set — check contracts/.env"; exit 1)
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
	   CB_A_HUB_PRIVATE_KEY=$(CENTRAL_BANK_PRIVATE_KEY) \
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
# ── Stack targets ────────────────────────────────────────────────────────────

scenario-b.up: scenario-b.prepare-pki scenario-b.up-infra scenario-b.deploy-contracts scenario-b.up-relayer scenario-b.up-backend
	@echo "[scenario-b] full stack up — ready for tryout (bash tryouts/tryout-scenario-b-e2e.sh)"

scenario-b.down: scenario-b.down-backend scenario-b.down-relayer scenario-b.down-infra
	@echo "[scenario-b] full stack down"

scenario-b.restart: scenario-b.down scenario-b.up

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

# ── Performance baseline (T105) ──────────────────────────────────────────────

scenario-b.perf-baseline:
	@command -v k6 >/dev/null 2>&1 || { echo "ERROR: k6 is required (https://k6.io)"; exit 1; }
	@echo "[scenario-b] running performance baseline (quote + swap p95)..."
	@API_GW_URL=$(API_GW_URL) k6 run tests/performance/scenario-b-perf.js

# ── OpenAPI validation (T108) ────────────────────────────────────────────────

scenario-b.validate-openapi:
	@command -v redocly >/dev/null 2>&1 || npx --yes @redocly/cli@latest lint \
		backend/services/api-gateway/openapi/v2/scenario-b.yaml

.PHONY: \
	scenario-b.up-infra scenario-b.down-infra \
	scenario-b.prepare-pki \
	scenario-b.up-relayer scenario-b.down-relayer \
	scenario-b.deploy-contracts scenario-b.seed-sovereign-pair \
	scenario-b.build-backend-images \
	scenario-b.up-backend scenario-b.down-backend \
	scenario-b.up-backend-mlp scenario-b.down-backend-mlp scenario-b.tryout-us2-mlp \
	scenario-b.up scenario-b.down scenario-b.restart \
	scenario-b.test-contracts scenario-b.test-backend scenario-b.test \
	scenario-b.tryout scenario-b.tryout-us1 scenario-b.tryout-us2 scenario-b.tryout-us3 \
	scenario-b.perf-baseline scenario-b.validate-openapi
