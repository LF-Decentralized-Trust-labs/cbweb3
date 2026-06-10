-include contracts/.env

# ── Protobuf / gRPC code generation ──────────────────────────────────────────
# Prerequisites (install once):
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#   go install github.com/bufbuild/buf/cmd/buf@latest
proto-gen:
	@cd apis/proto && buf generate

proto-lint:
	@cd apis/proto && buf lint

proto-breaking:
	@cd apis/proto && buf breaking --against '.git#branch=main'

# ── Solidity contracts ────────────────────────────────────────────────────────
contracts.setup:
	@cd contracts && forge soldeer install

contracts.fmt:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge fmt

contracts.lint:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge lint

contracts.test:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge test -vvv

contracts.coverage:
	@cd contracts && bash tools/validate-coverage.sh

contracts.build:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge build --sizes

contracts.clean:
	@cd contracts && forge clean

contracts.gen-doc:
	@cd contracts && forge doc

contracts.serve-doc:
	@cd contracts && forge doc --serve

contracts.deploy-tcebm-besu:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/TokenizedCentralBankMoney.s.sol:DeployTCeBM --rpc-url ${BESU_RPC_URL} --broadcast

contracts.deploy-htlc-besu:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/HashTimeLockedContract.s.sol:DeployHTLC --rpc-url ${BESU_RPC_URL} --broadcast

contracts.deploy-amm-besu:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/AutomatedMarketMaker.s.sol:DeployAMM --rpc-url ${BESU_RPC_URL} --broadcast

contracts.deploy-identity-registry-besu:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/IdentityRegistry.s.sol:DeployIdentityRegistry --rpc-url ${BESU_RPC_URL} --broadcast

contracts.deploy-hub:
	@test -n "$(CENTRAL_BANK_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_ADDRESS is not set — check contracts/.env"; exit 1)
	@echo "Deploying Scenario B Hub contracts (IdentityRegistry + Tokens + AMM + HTLC + Oracle) to the independent Hub network (chain 1337)..."
	@./deploy/local/tools/wait-rpc.sh "$${BESU_HUB_RPC:-http://127.0.0.1:8845}"
	@BESU_HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 HUB_CHAIN_ID="$${HUB_CHAIN_ID:-1337}" && \
	 cd contracts && CENTRAL_BANK_ADDRESS=$(CENTRAL_BANK_ADDRESS) FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script \
	   script/CBWeb3Hub.s.sol:DeployCBWeb3Hub \
	   --rpc-url "$$BESU_HUB_RPC" --broadcast

contracts.deploy-spoke-a:
	@test -n "$(CENTRAL_BANK_A_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_A_ADDRESS is not set — check contracts/.env"; exit 1)
	@echo "Deploying CBWeb3 spoke-a contracts to chain 1338 (bank-a, bank-c, central-bank-a)..."
	@./deploy/local/tools/wait-rpc.sh "${SPOKE_A_RPC_URL}"
	@cd contracts && TOKEN_NAME="Tokenized BRL" TOKEN_SYMBOL="tCeBM_BRL" \
		FIAT_TOKEN_NAME="Fiat BRL" FIAT_TOKEN_SYMBOL="fCeBM_BRL" \
		CENTRAL_BANK_ADDRESS=$(CENTRAL_BANK_A_ADDRESS) \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url ${SPOKE_A_RPC_URL} --broadcast

contracts.deploy-spoke-b:
	@test -n "$(CENTRAL_BANK_B_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_B_ADDRESS is not set — check contracts/.env"; exit 1)
	@echo "Deploying CBWeb3 spoke-b contracts (bank-b, bank-d, central-bank-b)..."
	@./deploy/local/tools/wait-rpc.sh "${SPOKE_B_RPC_URL}"
	@cd contracts && TOKEN_NAME="Tokenized ARS" TOKEN_SYMBOL="tCeBM_ARS" \
		FIAT_TOKEN_NAME="Fiat ARS" FIAT_TOKEN_SYMBOL="fCeBM_ARS" \
		CENTRAL_BANK_ADDRESS=$(CENTRAL_BANK_B_ADDRESS) \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url ${SPOKE_B_RPC_URL} --broadcast

contracts.deploy-all: contracts.setup contracts.deploy-spoke-a contracts.deploy-spoke-b

contracts.register-participants-spoke-a:
	@echo "Registering participants in compliance IdentityRegistry on spoke-a..."
	@REGISTRY=$$(grep PARTICIPANT_REGISTRY_ADDRESS backend/config/.env.infra.bank-a | cut -d= -f2-) && \
	 HUB_REG=$$(grep HUB_IDENTITY_REGISTRY_ADDRESS backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2- || true) && \
	 cd contracts && ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
		IDENTITY_REGISTRY=$$REGISTRY \
		HUB_IDENTITY_REGISTRY=$$HUB_REG \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/RegisterParticipants.s.sol:RegisterParticipants \
		--rpc-url ${SPOKE_A_RPC_URL} --broadcast

contracts.register-participants-spoke-b:
	@echo "Registering participants in compliance IdentityRegistry on spoke-b..."
	@REGISTRY=$$(grep PARTICIPANT_REGISTRY_ADDRESS backend/config/.env.infra.bank-b | cut -d= -f2-) && \
	 HUB_REG=$$(grep HUB_IDENTITY_REGISTRY_ADDRESS backend/config/.env.infra.bank-b 2>/dev/null | cut -d= -f2- || true) && \
	 cd contracts && ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
		IDENTITY_REGISTRY=$$REGISTRY \
		HUB_IDENTITY_REGISTRY=$$HUB_REG \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/RegisterParticipants.s.sol:RegisterParticipants \
		--rpc-url ${SPOKE_B_RPC_URL} --broadcast

contracts.register-participants-hub:
	@echo "Registering participants in Hub IdentityRegistry (AMM governance)..."
	@HUB_CHAIN_ID="$${HUB_CHAIN_ID:-1337}" && \
	 HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 REGISTRY=$$(jq -r '[.transactions[] | select(.transactionType=="CREATE" and .contractName=="IdentityRegistry")] | .[0].contractAddress' \
	   contracts/broadcast/CBWeb3Hub.s.sol/$$HUB_CHAIN_ID/run-latest.json) && \
	 echo "  Hub IdentityRegistry : $$REGISTRY (chain $$HUB_CHAIN_ID)" && \
	 cd contracts && ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
		IDENTITY_REGISTRY=$$REGISTRY \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/RegisterParticipants.s.sol:RegisterParticipants \
		--rpc-url $$HUB_RPC --broadcast

contracts.register-participants: contracts.register-participants-spoke-a contracts.register-participants-spoke-b contracts.register-participants-hub

contracts.seed-hub:
	@echo "Seeding hub: minting tokens to participants and initialising AMM liquidity..."
	@TOKEN_BRL=$$(grep '^HUB_TOKEN_A_ADDRESS=' backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2-) && \
	 TOKEN_EUR=$$(grep '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2-) && \
	 AMM=$$(grep '^AMM_CONTRACT_ADDRESS=' backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2-) && \
	 HUB_REG=$$(grep '^HUB_IDENTITY_REGISTRY_ADDRESS=' backend/config/.env.infra.bank-a 2>/dev/null | cut -d= -f2- || true) && \
	 BESU_HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} \
	   ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
	   CENTRAL_BANK_PRIVATE_KEY=$(CENTRAL_BANK_PRIVATE_KEY) \
	   TOKEN_BRL=$$TOKEN_BRL \
	   TOKEN_EUR=$$TOKEN_EUR \
	   AMM_ADDRESS=$$AMM \
	   HUB_IDENTITY_REGISTRY=$$HUB_REG \
	   forge script script/SeedHub.s.sol:SeedHub \
	   --rpc-url "$$BESU_HUB_RPC" --broadcast

contracts.deploy-cbweb3-besu: contracts.deploy-hub

# Recovery target: grants CENTRAL_BANK_ROLE to CENTRAL_BANK_ADDRESS on already-deployed
# hub tokens. Run this when hub tokens were deployed with a wrong centralBank address.
# Requires: ADMIN_PRIVATE_KEY (holds DEFAULT_ADMIN_ROLE), HUB_TOKEN_A_ADDRESS,
#           HUB_TOKEN_B_ADDRESS, BESU_HUB_RPC (defaults to http://127.0.0.1:8845).
contracts.grant-central-bank-role:
	@test -n "$(CENTRAL_BANK_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_ADDRESS is not set — check contracts/.env"; exit 1)
	@test -n "$(ADMIN_PRIVATE_KEY)" || (echo "ERROR: ADMIN_PRIVATE_KEY is not set — check contracts/.env"; exit 1)
	@TOKEN_A=$$(grep '^HUB_TOKEN_A_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2-) && \
	 TOKEN_B=$$(grep '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2-) && \
	 test -n "$$TOKEN_A" || (echo "ERROR: HUB_TOKEN_A_ADDRESS not found in backend/config/.env.infra.central-bank-a"; exit 1) && \
	 test -n "$$TOKEN_B" || (echo "ERROR: HUB_TOKEN_B_ADDRESS not found in backend/config/.env.infra.central-bank-a"; exit 1) && \
	 BESU_HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 ROLE=$$(cast keccak "CENTRAL_BANK_ROLE") && \
	 echo "Granting CENTRAL_BANK_ROLE to $(CENTRAL_BANK_ADDRESS) on HUB_TOKEN_A ($$TOKEN_A)..." && \
	 cast send $$TOKEN_A "grantRole(bytes32,address)" $$ROLE $(CENTRAL_BANK_ADDRESS) \
	   --private-key $(ADMIN_PRIVATE_KEY) --rpc-url $$BESU_HUB_RPC && \
	 echo "Granting CENTRAL_BANK_ROLE to $(CENTRAL_BANK_ADDRESS) on HUB_TOKEN_B ($$TOKEN_B)..." && \
	 cast send $$TOKEN_B "grantRole(bytes32,address)" $$ROLE $(CENTRAL_BANK_ADDRESS) \
	   --private-key $(ADMIN_PRIVATE_KEY) --rpc-url $$BESU_HUB_RPC && \
	 echo "Verifying roles..." && \
	 cast call $$TOKEN_A "hasRole(bytes32,address)(bool)" $$ROLE $(CENTRAL_BANK_ADDRESS) --rpc-url $$BESU_HUB_RPC && \
	 cast call $$TOKEN_B "hasRole(bytes32,address)(bool)" $$ROLE $(CENTRAL_BANK_ADDRESS) --rpc-url $$BESU_HUB_RPC && \
	 echo "Done — CENTRAL_BANK_ROLE granted and verified on both hub tokens."

# Recovery target: grants CENTRAL_BANK_ROLE to CENTRAL_BANK_B_ADDRESS on HUB_TOKEN_B only.
# CB-B issues token_b (tCeBM_EUR/USD); without this role mint() reverts on-chain.
# Run once after deploy if CENTRAL_BANK_B_ADDRESS was not set during contracts.deploy-hub.
# Requires: ADMIN_PRIVATE_KEY, CENTRAL_BANK_B_ADDRESS, BESU_HUB_RPC.
contracts.grant-central-bank-b-role:
	@test -n "$(CENTRAL_BANK_B_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_B_ADDRESS is not set"; exit 1)
	@test -n "$(ADMIN_PRIVATE_KEY)" || (echo "ERROR: ADMIN_PRIVATE_KEY is not set — check contracts/.env"; exit 1)
	@TOKEN_B=$$(grep '^HUB_TOKEN_B_ADDRESS=' backend/config/.env.infra.central-bank-b 2>/dev/null | cut -d= -f2-) && \
	 test -n "$$TOKEN_B" || (echo "ERROR: HUB_TOKEN_B_ADDRESS not found in backend/config/.env.infra.central-bank-b"; exit 1) && \
	 BESU_HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 ROLE=$$(cast keccak "CENTRAL_BANK_ROLE") && \
	 echo "Granting CENTRAL_BANK_ROLE to CB-B ($(CENTRAL_BANK_B_ADDRESS)) on HUB_TOKEN_B ($$TOKEN_B)..." && \
	 cast send $$TOKEN_B "grantRole(bytes32,address)" $$ROLE $(CENTRAL_BANK_B_ADDRESS) \
	   --private-key $(ADMIN_PRIVATE_KEY) --rpc-url $$BESU_HUB_RPC && \
	 echo "Verifying..." && \
	 cast call $$TOKEN_B "hasRole(bytes32,address)(bool)" $$ROLE $(CENTRAL_BANK_B_ADDRESS) --rpc-url $$BESU_HUB_RPC && \
	 echo "Done — CENTRAL_BANK_ROLE granted to CB-B on token_b."

# Grants the LiquidityProvider role on the Hub IdentityRegistry to each CB's hub identity
# (CENTRAL_BANK_A_ADDRESS = 0xfe3b..., CENTRAL_BANK_B_ADDRESS = 0xf17f...), signed by the
# ADMIN key which holds DEFAULT_ADMIN_ROLE on the registry. The CB hub keys are NOT registry
# admins, so they cannot self-grant LP at api-gateway startup — this step does it for them
# (the bootstrap then sees LP already granted and skips). Idempotent: skips addresses that
# are already LiquidityProviders.
# Run after contracts.deploy-hub + contracts.register-participants-hub (the registry must exist).
# Requires: ADMIN_PRIVATE_KEY, CENTRAL_BANK_A_ADDRESS, CENTRAL_BANK_B_ADDRESS, BESU_HUB_RPC.
contracts.grant-liquidity-providers:
	@test -n "$(CENTRAL_BANK_A_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_A_ADDRESS is not set — check contracts/.env"; exit 1)
	@test -n "$(CENTRAL_BANK_B_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_B_ADDRESS is not set — check contracts/.env"; exit 1)
	@test -n "$(ADMIN_PRIVATE_KEY)" || (echo "ERROR: ADMIN_PRIVATE_KEY is not set — check contracts/.env"; exit 1)
	@HUB_REG=$$(grep '^HUB_IDENTITY_REGISTRY_ADDRESS=' backend/config/.env.infra.central-bank-a 2>/dev/null | cut -d= -f2-) && \
	 test -n "$$HUB_REG" || (echo "ERROR: HUB_IDENTITY_REGISTRY_ADDRESS not found in backend/config/.env.infra.central-bank-a"; exit 1) && \
	 BESU_HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 for CB in $(CENTRAL_BANK_A_ADDRESS) $(CENTRAL_BANK_B_ADDRESS); do \
	   IS_LP=$$(cast call $$HUB_REG "isLiquidityProvider(address)(bool)" $$CB --rpc-url $$BESU_HUB_RPC 2>/dev/null || echo error); \
	   if [ "$$IS_LP" = "true" ]; then \
	     echo "  Already LiquidityProvider: $$CB"; \
	   else \
	     echo "  Granting LiquidityProvider to $$CB..." && \
	     cast send $$HUB_REG "grantLiquidityProvider(address)" $$CB \
	       --private-key $(ADMIN_PRIVATE_KEY) --rpc-url $$BESU_HUB_RPC >/dev/null && \
	     cast call $$HUB_REG "isLiquidityProvider(address)(bool)" $$CB --rpc-url $$BESU_HUB_RPC; \
	   fi; \
	 done && \
	 echo "Done — LiquidityProvider roles granted to both CB hub identities."

contracts.sync-addresses:
	@SOVEREIGN_PAIR_ID="$(SOVEREIGN_PAIR_ID)" ./deploy/local/tools/sync-contracts.sh

# ── 007-bridge-based-cb-liquidity: Sovereign Pair Seeding ───────────────────
# Seeds a new sovereign CB liquidity pair on the Hub: deploys W-tCeBM tokens (A + B),
# optionally deploys LiquidityCommitRegistry, deploys AMM, and registers the pair
# in PairRegistry via CB-A proposePair + CB-B confirmPair.
#
# Required env vars:
#   TOKEN_SYMBOL_A           — e.g. BRL
#   TOKEN_SYMBOL_B           — e.g. ARS
#   PAIR_ID                  — e.g. W-BRL-ARS
#   CB_A_HUB_PRIVATE_KEY     — CB-A signer private key (hex)
#   CB_B_HUB_PRIVATE_KEY     — CB-B signer private key (hex)
#   ADMIN_PRIVATE_KEY        — admin private key (hex)
#   ADMIN_ADDRESS            — admin Ethereum address
#   HUB_IDENTITY_REGISTRY    — IdentityRegistry address on Hub
#   PAIR_REGISTRY_ADDRESS    — PairRegistry address on Hub
#   BESU_HUB_RPC             — Hub Besu RPC URL
#
# Optional env vars:
#   LIQUIDITY_COMMIT_REGISTRY_ADDRESS  — reuse existing LCR (deploy new if absent)
#   RELAYER_ADDR                       — Cacti watcher address to grant CENTRAL_BANK_ROLE
#   HUB_CHAIN_ID                       — defaults to 1337
#
# Usage:
#   make contracts.seed-sovereign-pair \
#     TOKEN_SYMBOL_A=BRL TOKEN_SYMBOL_B=ARS PAIR_ID=W-BRL-ARS \
#     CB_A_HUB_PRIVATE_KEY=<hex> CB_B_HUB_PRIVATE_KEY=<hex> \
#     ADMIN_PRIVATE_KEY=<hex> ADMIN_ADDRESS=<addr> \
#     HUB_IDENTITY_REGISTRY=<addr> PAIR_REGISTRY_ADDRESS=<addr>
contracts.seed-sovereign-pair:
	@test -n "$(TOKEN_SYMBOL_A)"       || (echo "ERROR: TOKEN_SYMBOL_A is required"; exit 1)
	@test -n "$(TOKEN_SYMBOL_B)"       || (echo "ERROR: TOKEN_SYMBOL_B is required"; exit 1)
	@test -n "$(PAIR_ID)"              || (echo "ERROR: PAIR_ID is required"; exit 1)
	@test -n "$(CB_A_HUB_PRIVATE_KEY)" || (echo "ERROR: CB_A_HUB_PRIVATE_KEY is required"; exit 1)
	@test -n "$(CB_B_HUB_PRIVATE_KEY)" || (echo "ERROR: CB_B_HUB_PRIVATE_KEY is required"; exit 1)
	@test -n "$(ADMIN_PRIVATE_KEY)"    || (echo "ERROR: ADMIN_PRIVATE_KEY is required"; exit 1)
	@test -n "$(ADMIN_ADDRESS)"        || (echo "ERROR: ADMIN_ADDRESS is required"; exit 1)
	@test -n "$(HUB_IDENTITY_REGISTRY)" || (echo "ERROR: HUB_IDENTITY_REGISTRY is required"; exit 1)
	@test -n "$(PAIR_REGISTRY_ADDRESS)" || (echo "ERROR: PAIR_REGISTRY_ADDRESS is required"; exit 1)
	@BESU_HUB_RPC="$${BESU_HUB_RPC:-http://127.0.0.1:8845}" && \
	 HUB_CHAIN_ID="$${HUB_CHAIN_ID:-1337}" && \
	 echo "Seeding sovereign pair $(PAIR_ID) on Hub (RPC: $$BESU_HUB_RPC chain $$HUB_CHAIN_ID)..." && \
	 cd contracts && \
	 TOKEN_SYMBOL_A=$(TOKEN_SYMBOL_A) \
	 TOKEN_SYMBOL_B=$(TOKEN_SYMBOL_B) \
	 PAIR_ID=$(PAIR_ID) \
	 CB_A_HUB_PRIVATE_KEY=$(CB_A_HUB_PRIVATE_KEY) \
	 CB_B_HUB_PRIVATE_KEY=$(CB_B_HUB_PRIVATE_KEY) \
	 ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
	 ADMIN_ADDRESS=$(ADMIN_ADDRESS) \
	 HUB_IDENTITY_REGISTRY=$(HUB_IDENTITY_REGISTRY) \
	 PAIR_REGISTRY_ADDRESS=$(PAIR_REGISTRY_ADDRESS) \
	 LIQUIDITY_COMMIT_REGISTRY_ADDRESS="$${LIQUIDITY_COMMIT_REGISTRY_ADDRESS:-}" \
	 RELAYER_ADDR="$${RELAYER_ADDR:-}" \
	 FOUNDRY_PROFILE=${FOUNDRY_PROFILE} \
	 forge script script/SeedNewSovereignPair.s.sol:SeedNewSovereignPair \
	   --rpc-url $$BESU_HUB_RPC \
	   --chain-id $$HUB_CHAIN_ID \
	   --broadcast \
	   -vvv

# ── Run LiquidityCommitRegistry Foundry tests ───────────────────────────────
contracts.test-sovereign:
	@echo "Running LiquidityCommitRegistry Foundry tests..."
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge test \
	  --match-contract LiquidityCommitRegistryTest -vvv

contracts.deploy-all-with-sync: contracts.deploy-all contracts.sync-addresses contracts.register-participants contracts.seed-hub

contracts.deploy-cbweb3-with-sync: contracts.deploy-all-with-sync

contracts.slither:
	@docker run --rm -v $(CURDIR)/contracts:/share -w /share trailofbits/eth-security-toolbox slither . --config-file slither.config.json

contracts.full-check: contracts.build contracts.fmt contracts.lint contracts.test contracts.coverage contracts.slither

# ── Go bindings (abigen) ─────────────────────────────────────────────────────
# Prerequisites (install once):
#   go install github.com/ethereum/go-ethereum/cmd/abigen@v1.17.1
ABIGEN_OUT_DIR := backend/shared/blockchain/registry/bindings

contracts.abigen: contracts.build
	@echo "Generating Go bindings for IdentityRegistry..."
	@mkdir -p $(ABIGEN_OUT_DIR)
	@cd contracts && forge inspect IdentityRegistry abi --json > /tmp/IdentityRegistry.abi
	@abigen --abi /tmp/IdentityRegistry.abi \
		--pkg bindings \
		--type IdentityRegistry \
		--out $(ABIGEN_OUT_DIR)/identity_registry.go
	@rm -f /tmp/IdentityRegistry.abi
	@echo "Go bindings generated at $(ABIGEN_OUT_DIR)/identity_registry.go"

.PHONY: contracts.setup contracts.fmt contracts.lint contracts.test contracts.coverage contracts.build contracts.clean contracts.gen-doc contracts.serve-doc contracts.deploy-tcebm-besu contracts.deploy-htlc-besu contracts.deploy-amm-besu contracts.deploy-identity-registry-besu contracts.deploy-hub contracts.deploy-spoke-a contracts.deploy-spoke-b contracts.deploy-all contracts.deploy-cbweb3-besu contracts.grant-central-bank-role contracts.grant-central-bank-b-role contracts.grant-liquidity-providers contracts.sync-addresses contracts.deploy-all-with-sync contracts.deploy-cbweb3-with-sync contracts.slither contracts.full-check contracts.abigen contracts.register-participants contracts.register-participants-spoke-a contracts.register-participants-spoke-b contracts.register-participants-hub contracts.seed-hub contracts.seed-sovereign-pair contracts.test-sovereign
