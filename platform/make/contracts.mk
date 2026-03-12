include contracts/.env

contracts.fmt:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge fmt --check

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

contracts.deploy-besu:
	@cd contracts && FOUNDRY_PROFILE=${FOUNDRY_PROFILE} forge script script/TokenizedCentralBankMoney.s.sol:DeployTCeBM --rpc-url ${BESU_RPC_URL} --broadcast

.PHONY: contracts.fmt contracts.lint contracts.test contracts.coverage contracts.build contracts.clean contracts.gen-doc contracts.serve-doc contracts.deploy-besu
