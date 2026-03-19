include contracts/.env

contracts.fmt:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge fmt

contracts.lint:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge lint

contracts.test:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge test -vvvv

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

contracts.deploy-cbweb3-besu:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/CBWeb3.s.sol:DeployCBWeb3 --rpc-url ${BESU_RPC_URL} --broadcast

contracts.slither:
	@docker run --rm -v $(CURDIR)/contracts:/share -w /share trailofbits/eth-security-toolbox slither . --config-file slither.config.json

contracts.full-check: contracts.build contracts.fmt contracts.lint contracts.test contracts.coverage contracts.slither

.PHONY: contracts.fmt contracts.lint contracts.test contracts.coverage contracts.build contracts.clean contracts.gen-doc contracts.serve-doc contracts.deploy-tcebm-besu contracts.deploy-htlc-besu contracts.deploy-amm-besu contracts.deploy-cbweb3-besu contracts.slither contracts.full-check
