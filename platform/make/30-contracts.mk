include contracts/.env

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
	@echo "Deploying CBWeb3 hub contracts to chain 1337..."
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Hub.s.sol:DeployCBWeb3Hub --rpc-url ${HUB_RPC_URL} --broadcast

contracts.deploy-spoke-a:
	@echo "Deploying CBWeb3 spoke-a contracts to chain 1338..."
	@cd contracts && TOKEN_NAME="Tokenized BRL" TOKEN_SYMBOL="tCeBM_BRL" \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url ${SPOKE_A_RPC_URL} --broadcast

contracts.deploy-spoke-b:
	@echo "Deploying CBWeb3 spoke-b contracts to chain 1339..."
	@cd contracts && TOKEN_NAME="Tokenized EUR" TOKEN_SYMBOL="tCeBM_EUR" \
		FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url ${SPOKE_B_RPC_URL} --broadcast

contracts.deploy-all: contracts.setup contracts.deploy-hub contracts.deploy-spoke-a contracts.deploy-spoke-b

contracts.deploy-cbweb3-besu: contracts.deploy-hub

contracts.sync-addresses:
	@./deploy/local/tools/sync-contracts.sh

contracts.deploy-all-with-sync: contracts.deploy-all contracts.sync-addresses

contracts.deploy-cbweb3-with-sync: contracts.deploy-all-with-sync

contracts.slither:
	@docker run --rm -v $(CURDIR)/contracts:/share -w /share trailofbits/eth-security-toolbox slither . --config-file slither.config.json

contracts.full-check: contracts.build contracts.fmt contracts.lint contracts.test contracts.coverage contracts.slither

.PHONY: contracts.setup contracts.fmt contracts.lint contracts.test contracts.coverage contracts.build contracts.clean contracts.gen-doc contracts.serve-doc contracts.deploy-tcebm-besu contracts.deploy-htlc-besu contracts.deploy-amm-besu contracts.deploy-identity-registry-besu contracts.deploy-hub contracts.deploy-spoke-a contracts.deploy-spoke-b contracts.deploy-all contracts.deploy-cbweb3-besu contracts.sync-addresses contracts.deploy-all-with-sync contracts.deploy-cbweb3-with-sync contracts.slither contracts.full-check
