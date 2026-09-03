// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {DeployCBWeb3Spoke} from "../script/CBWeb3Spoke.s.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {SpokeBridge} from "../src/SpokeBridge.sol";
import {FiatCentralBankMoney} from "../src/FiatCentralBankMoney.sol";
import {IIdentityRegistry} from "../src/interfaces/IIdentityRegistry.sol";

/// @title DeployCBWeb3SpokeTest
/// @notice Unit tests for the DeployCBWeb3Spoke deployment script.
/// @dev Tests verify that spoke-specific contracts are deployed correctly.
///      Spokes deploy: IdentityRegistry, tCeBM (domestic currency), HTLC (Scenario A domestic leg),
///      and SpokeBridge (lock-and-mint for Scenario B).
///      Hub-only contracts (AMM) are NOT deployed here.
contract DeployCBWeb3SpokeTest is Test {
    DeployCBWeb3Spoke public deployScript;

    /// @dev Deployment configuration
    uint256 private deployerPrivateKey;
    address private expectedDeployer;

    /// @dev Role identifiers
    bytes32 private constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 private constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");
    bytes32 private constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    /// @dev Spoke token configuration
    string private constant TOKEN_NAME = "Tokenized BRL";
    string private constant TOKEN_SYMBOL = "tCeBM_BRL";
    string private constant FIAT_TOKEN_NAME = "Fiat BRL";
    string private constant FIAT_TOKEN_SYMBOL = "fCeBM_BRL";

    function setUp() public {
        deployScript = new DeployCBWeb3Spoke();
        deployScript.setUp();

        deployerPrivateKey = uint256(0x1);
        expectedDeployer = vm.addr(deployerPrivateKey);
    }

    /// @dev Sets the deployment env. Called inside the test body (not setUp) because vm.setEnv
    ///      mutates a process-global env: a sibling deploy-script suite's setUp() can overwrite
    ///      these keys between this suite's setUp() and run(), causing non-deterministic flakes.
    ///      Setting them immediately before run() guarantees they hold at envAddress() read time.
    function _setDeployEnv() internal {
        vm.setEnv("DEPLOYER_PRIVATE_KEY", vm.toString(deployerPrivateKey));
        // ADMIN_ADDRESS must be the broadcaster (deployer) so it holds DEFAULT_ADMIN_ROLE on
        // the freshly-deployed IdentityRegistry and can grant GOVERNANCE_ROLE to the central
        // bank inside run(); otherwise grantRole reverts with AccessControlUnauthorizedAccount.
        vm.setEnv("ADMIN_ADDRESS", vm.toString(expectedDeployer));
        vm.setEnv("CENTRAL_BANK_ADDRESS", vm.toString(address(0x2345678901234567890123456789012345678901)));
        vm.setEnv("TOKEN_NAME", TOKEN_NAME);
        vm.setEnv("TOKEN_SYMBOL", TOKEN_SYMBOL);
        vm.setEnv("FIAT_TOKEN_NAME", FIAT_TOKEN_NAME);
        vm.setEnv("FIAT_TOKEN_SYMBOL", FIAT_TOKEN_SYMBOL);
    }

    /// @dev The script should deploy IdentityRegistry, a single token, and HTLC.
    function test_ScriptRun_Success() public {
        _setDeployEnv();
        deployScript.run();

        IdentityRegistry identityRegistry = deployScript.identityRegistry();
        TokenizedCentralBankMoney token = deployScript.token();
        HashTimeLockedContract htlc = deployScript.htlc();
        SpokeBridge spokeBridge = deployScript.spokeBridge();
        FiatCentralBankMoney fiatToken = deployScript.fiatToken();

        /// @dev Assert: contracts deployed
        assertTrue(address(identityRegistry) != address(0), "IdentityRegistry was not deployed");
        assertTrue(address(token) != address(0), "Token was not deployed");
        assertTrue(address(htlc) != address(0), "HTLC was not deployed");
        assertTrue(address(spokeBridge) != address(0), "SpokeBridge was not deployed");
        assertTrue(address(fiatToken) != address(0), "FiatToken was not deployed");
        assertGt(address(identityRegistry).code.length, 0, "IdentityRegistry has no bytecode");
        assertGt(address(token).code.length, 0, "Token has no bytecode");
        assertGt(address(htlc).code.length, 0, "HTLC has no bytecode");
        assertGt(address(spokeBridge).code.length, 0, "SpokeBridge has no bytecode");
        assertGt(address(fiatToken).code.length, 0, "FiatToken has no bytecode");

        /// @dev Assert: IdentityRegistry RBAC
        assertTrue(identityRegistry.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()), "Admin role not granted");
        assertTrue(
            identityRegistry.hasRole(GOVERNANCE_ROLE, deployScript.adminAddress()), "Governance role not granted"
        );

        /// @dev Assert: Token configuration
        assertEq(token.name(), TOKEN_NAME, "Token name incorrect");
        assertEq(token.symbol(), TOKEN_SYMBOL, "Token symbol incorrect");
        assertTrue(token.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()), "Token admin role not granted");
        assertTrue(
            token.hasRole(CENTRAL_BANK_ROLE, deployScript.centralBankAddress()), "Token central bank role not granted"
        );

        /// @dev Assert: HTLC linked to spoke's IdentityRegistry
        assertEq(
            address(htlc.IDENTITY_REGISTRY()), address(identityRegistry), "HTLC not linked to spoke IdentityRegistry"
        );

        /// @dev Assert: SpokeBridge linked to spoke's IdentityRegistry
        assertEq(
            address(spokeBridge.IDENTITY_REGISTRY()),
            address(identityRegistry),
            "SpokeBridge not linked to spoke IdentityRegistry"
        );

        /// @dev Assert: SpokeBridge RBAC
        assertTrue(
            spokeBridge.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()), "SpokeBridge admin role not granted"
        );
        assertTrue(
            spokeBridge.hasRole(spokeBridge.GOVERNANCE_ROLE(), deployScript.adminAddress()),
            "SpokeBridge governance role not granted"
        );

        /// @dev Assert: FiatToken configuration
        assertEq(fiatToken.name(), FIAT_TOKEN_NAME, "FiatToken name incorrect");
        assertEq(fiatToken.symbol(), FIAT_TOKEN_SYMBOL, "FiatToken symbol incorrect");
        assertTrue(
            fiatToken.hasRole(DEFAULT_ADMIN_ROLE, deployScript.adminAddress()), "FiatToken admin role not granted"
        );
        assertTrue(
            fiatToken.hasRole(CENTRAL_BANK_ROLE, deployScript.centralBankAddress()),
            "FiatToken central bank role not granted"
        );
    }
}
