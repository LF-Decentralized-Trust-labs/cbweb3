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
	@echo "Deploying CBWeb3 hub contracts to chain 1337 (legacy — hub-besu removed)..."
	@echo "WARNING: hub-besu was removed. Use contracts.deploy-spoke-a for the spoke-a chain (1338)."
	@exit 1

contracts.deploy-spoke-a:
	@test -n "$(CENTRAL_BANK_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_ADDRESS is not set — check contracts/.env"; exit 1)
	@echo "Deploying CBWeb3 spoke-a contracts to chain 1338 (bank-a, bank-c, central-bank-a)..."
	@cd contracts && TOKEN_NAME="Tokenized BRL" TOKEN_SYMBOL="tCeBM_BRL" \
		FIAT_TOKEN_NAME="Fiat BRL" FIAT_TOKEN_SYMBOL="fCeBM_BRL" \
		CENTRAL_BANK_ADDRESS=$(CENTRAL_BANK_ADDRESS) \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url ${SPOKE_A_RPC_URL} --broadcast

contracts.deploy-spoke-b:
	@test -n "$(CENTRAL_BANK_ADDRESS)" || (echo "ERROR: CENTRAL_BANK_ADDRESS is not set — check contracts/.env"; exit 1)
	@echo "Deploying CBWeb3 spoke-b contracts (bank-b, bank-d, central-bank-b)..."
	@cd contracts && TOKEN_NAME="Tokenized BRL" TOKEN_SYMBOL="tCeBM_BRL" \
		FIAT_TOKEN_NAME="Fiat BRL" FIAT_TOKEN_SYMBOL="fCeBM_BRL" \
		CENTRAL_BANK_ADDRESS=$(CENTRAL_BANK_ADDRESS) \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url ${SPOKE_B_RPC_URL} --broadcast

contracts.deploy-all: contracts.setup contracts.deploy-spoke-a contracts.deploy-spoke-b

contracts.register-participants-spoke-a:
	@echo "Registering participants in compliance IdentityRegistry on spoke-a..."
	@REGISTRY=$$(grep PARTICIPANT_REGISTRY_ADDRESS backend/config/.env.infra.bank-a | cut -d= -f2-) && \
	 cd contracts && ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
		IDENTITY_REGISTRY=$$REGISTRY \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/RegisterParticipants.s.sol:RegisterParticipants \
		--rpc-url ${SPOKE_A_RPC_URL} --broadcast

contracts.register-participants-spoke-b:
	@echo "Registering participants in compliance IdentityRegistry on spoke-b..."
	@REGISTRY=$$(grep PARTICIPANT_REGISTRY_ADDRESS backend/config/.env.infra.bank-b | cut -d= -f2-) && \
	 cd contracts && ADMIN_PRIVATE_KEY=$(ADMIN_PRIVATE_KEY) \
		IDENTITY_REGISTRY=$$REGISTRY \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/RegisterParticipants.s.sol:RegisterParticipants \
		--rpc-url ${SPOKE_B_RPC_URL} --broadcast

contracts.register-participants: contracts.register-participants-spoke-a contracts.register-participants-spoke-b

contracts.deploy-cbweb3-besu: contracts.deploy-hub

contracts.sync-addresses:
	@./deploy/local/tools/sync-contracts.sh

contracts.deploy-all-with-sync: contracts.deploy-all contracts.sync-addresses contracts.register-participants

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

.PHONY: contracts.setup contracts.fmt contracts.lint contracts.test contracts.coverage contracts.build contracts.clean contracts.gen-doc contracts.serve-doc contracts.deploy-tcebm-besu contracts.deploy-htlc-besu contracts.deploy-amm-besu contracts.deploy-identity-registry-besu contracts.deploy-hub contracts.deploy-spoke-a contracts.deploy-spoke-b contracts.deploy-all contracts.deploy-cbweb3-besu contracts.sync-addresses contracts.deploy-all-with-sync contracts.deploy-cbweb3-with-sync contracts.slither contracts.full-check contracts.abigen contracts.register-participants contracts.register-participants-spoke-a contracts.register-participants-spoke-b
