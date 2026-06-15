// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {DeployCBWeb3Hub} from "../script/CBWeb3Hub.s.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IIdentityRegistry} from "../src/interfaces/IIdentityRegistry.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary} from "../src/libraries/HashTimeLockedContractLibrary.sol";
import {AutomatedMarketMaker} from "../src/AutomatedMarketMaker.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {ManualOracle} from "../src/ManualOracle.sol";

/// @title DeployCBWeb3HubTest
/// @notice Unit tests for the DeployCBWeb3Hub hub deployment script.
/// @dev Tests verify that all core hub contracts are deployed correctly with proper configuration.
contract DeployCBWeb3HubTest is Test {
    /// @notice Test deployment of the entire CBWeb3 hub via script
    DeployCBWeb3Hub public deployScript;

    /// @dev Deployment configuration
    uint256 private deployerPrivateKey;
    address private expectedDeployer;

    /// @dev Environment variable names
    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_ADMIN_ADDRESS = "ADMIN_ADDRESS";
    string private constant ENV_CENTRAL_BANK_ADDRESS = "CENTRAL_BANK_ADDRESS";

    /// @dev Role identifiers for RBAC
    bytes32 private constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 private constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");
    bytes32 private constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    /// @dev Token details for assertions
    string private constant TOKEN_BRL_NAME = "Tokenized BRL";
    string private constant TOKEN_BRL_SYMBOL = "tCeBM_BRL";
    string private constant TOKEN_EUR_NAME = "Tokenized EUR";
    string private constant TOKEN_EUR_SYMBOL = "tCeBM_EUR";

    /// @notice Sets up the test environment by instantiating the deployment script.
    /// @dev Reads deployment parameters from environment variables.
    function setUp() public {
        deployScript = new DeployCBWeb3Hub();
        deployScript.setUp();

        /// @dev Always set explicit defaults to avoid env contamination from other test suites
        deployerPrivateKey = uint256(0x1);
        expectedDeployer = vm.addr(deployerPrivateKey);
    }

    /// @dev Sets the deployment env. Called inside the test body (not setUp) because vm.setEnv
    ///      mutates a process-global env: a sibling deploy-script suite's setUp() can overwrite
    ///      these keys between this suite's setUp() and run(), causing non-deterministic flakes.
    function _setDeployEnv() internal {
        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
        vm.setEnv(ENV_ADMIN_ADDRESS, vm.toString(address(0x1234567890123456789012345678901234567890)));
        vm.setEnv(ENV_CENTRAL_BANK_ADDRESS, vm.toString(address(0x2345678901234567890123456789012345678901)));
        // Ensure ADMIN_PRIVATE_KEY is absent so grantLiquidityProvider is skipped in the test context.
        // (The deployer ≠ admin in this test; granting requires the admin key which is not available here.)
        vm.setEnv("ADMIN_PRIVATE_KEY", "0");
    }

    /// @dev The script should successfully deploy all hub contracts using env vars.
    function test_ScriptRun_Success() public {
        /// @dev Act
        _setDeployEnv();
        deployScript.run();

        IdentityRegistry identityRegistry = deployScript.identityRegistry();
        TokenizedCentralBankMoney tokenBrl = deployScript.tokenBrl();
        TokenizedCentralBankMoney tokenEur = deployScript.tokenEur();
        HashTimeLockedContract htlc = deployScript.htlc();
        AutomatedMarketMaker amm = deployScript.amm();
        FXAgreement fxAgreement = deployScript.fxAgreement();
        ManualOracle oracle = deployScript.oracle();

        /// @dev Assert: All contracts deployed
        assertTrue(address(identityRegistry) != address(0), "IdentityRegistry was not deployed");
        assertTrue(address(tokenBrl) != address(0), "TokenBRL was not deployed");
        assertTrue(address(tokenEur) != address(0), "TokenEUR was not deployed");
        assertTrue(address(htlc) != address(0), "HTLC was not deployed");
        assertTrue(address(amm) != address(0), "AMM was not deployed");
        assertTrue(address(fxAgreement) != address(0), "FXAgreement was not deployed");
        assertTrue(address(oracle) != address(0), "ManualOracle was not deployed");

        /// @dev Assert: Contracts have bytecode
        assertGt(address(identityRegistry).code.length, 0, "IdentityRegistry has no runtime bytecode");
        assertGt(address(tokenBrl).code.length, 0, "TokenBRL has no runtime bytecode");
        assertGt(address(tokenEur).code.length, 0, "TokenEUR has no runtime bytecode");
        assertGt(address(htlc).code.length, 0, "HTLC has no runtime bytecode");
        assertGt(address(amm).code.length, 0, "AMM has no runtime bytecode");
        assertGt(address(fxAgreement).code.length, 0, "FXAgreement has no runtime bytecode");
        assertGt(address(oracle).code.length, 0, "ManualOracle has no runtime bytecode");

        /// @dev Assert: IdentityRegistry RBAC configuration
        assertTrue(
            identityRegistry.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()),
            "IdentityRegistry admin role not granted"
        );
        assertTrue(
            identityRegistry.hasRole(GOVERNANCE_ROLE, deployScript.adminAddress()),
            "IdentityRegistry governance role not granted"
        );
        assertFalse(
            identityRegistry.hasRole(DEFAULT_ADMIN_ROLE, expectedDeployer),
            "IdentityRegistry deployer should not be admin"
        );

        /// @dev Assert: TokenBRL configuration
        assertEq(tokenBrl.name(), TOKEN_BRL_NAME, "TokenBRL name incorrect");
        assertEq(tokenBrl.symbol(), TOKEN_BRL_SYMBOL, "TokenBRL symbol incorrect");
        assertTrue(tokenBrl.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()), "TokenBRL admin role not granted");
        assertTrue(
            tokenBrl.hasRole(CENTRAL_BANK_ROLE, deployScript.centralBankAddress()),
            "TokenBRL central bank role not granted"
        );

        /// @dev Assert: TokenEUR configuration
        assertEq(tokenEur.name(), TOKEN_EUR_NAME, "TokenEUR name incorrect");
        assertEq(tokenEur.symbol(), TOKEN_EUR_SYMBOL, "TokenEUR symbol incorrect");
        assertTrue(tokenEur.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()), "TokenEUR admin role not granted");
        assertTrue(
            tokenEur.hasRole(CENTRAL_BANK_ROLE, deployScript.centralBankAddress()),
            "TokenEUR central bank role not granted"
        );

        /// @dev Assert: AMM configuration with deployed tokens
        assertEq(address(amm.TOKEN_A()), address(tokenBrl), "AMM TokenA should be TokenBRL");
        assertEq(address(amm.TOKEN_B()), address(tokenEur), "AMM TokenB should be TokenEUR");

        /// @dev Assert: AMM initial state
        assertEq(amm.reserveA(), 0, "AMM initial reserveA should be zero");
        assertEq(amm.reserveB(), 0, "AMM initial reserveB should be zero");
        assertFalse(amm.isPaused(), "AMM should not be paused initially");

        /// @dev Assert: AMM and HTLC are wired to the IdentityRegistry
        assertEq(
            address(amm.IDENTITY_REGISTRY()),
            address(identityRegistry),
            "AMM should reference the deployed IdentityRegistry"
        );
        assertEq(
            address(htlc.IDENTITY_REGISTRY()),
            address(identityRegistry),
            "HTLC should reference the deployed IdentityRegistry"
        );

        /// @dev Assert: FXAgreement linked to IdentityRegistry
        assertEq(
            address(fxAgreement.IDENTITY_REGISTRY()),
            address(identityRegistry),
            "FXAgreement should reference the deployed IdentityRegistry"
        );

        /// @dev Assert: ManualOracle RBAC
        assertTrue(
            oracle.hasRole(oracle.CENTRAL_BANK_ROLE(), deployScript.centralBankAddress()),
            "ManualOracle central bank role not granted"
        );
        assertTrue(
            oracle.hasRole(oracle.DEFAULT_ADMIN_ROLE(), deployScript.adminAddress()),
            "ManualOracle admin role not granted"
        );

        /// @dev Assert: HTLC default state for non-existing contract
        bytes32 unknownContractId = keccak256("UNKNOWN_CONTRACT");
        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(unknownContractId);
        assertEq(details.sender, address(0), "HTLC default sender should be zero address");

        /// @dev Assert: Deployer addresses are valid
        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
