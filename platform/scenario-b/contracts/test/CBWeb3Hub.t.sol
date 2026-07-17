// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {DeployCBWeb3Hub} from "../script/CBWeb3Hub.s.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary} from "../src/libraries/HashTimeLockedContractLibrary.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {ManualOracle} from "../src/ManualOracle.sol";
import {PairRegistry} from "../src/PairRegistry.sol";
import {CurrencyRegistry} from "../src/CurrencyRegistry.sol";
import {LiquidityCommitRegistry} from "../src/LiquidityCommitRegistry.sol";

/// @title DeployCBWeb3HubTest
/// @notice Unit tests for the DeployCBWeb3Hub hub deployment script.
/// @dev tCeBM tokens are intentionally NOT deployed by the hub script.
contract DeployCBWeb3HubTest is Test {
    DeployCBWeb3Hub public deployScript;

    uint256 private deployerPrivateKey;
    address private expectedDeployer;

    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_ADMIN_ADDRESS = "ADMIN_ADDRESS";
    string private constant ENV_CENTRAL_BANK_ADDRESS = "CENTRAL_BANK_ADDRESS";

    bytes32 private constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 private constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");

    function setUp() public {
        deployScript = new DeployCBWeb3Hub();
        deployScript.setUp();
        deployerPrivateKey = uint256(0x1);
        expectedDeployer = vm.addr(deployerPrivateKey);
    }

    function _setDeployEnv() internal {
        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
        vm.setEnv(ENV_ADMIN_ADDRESS, vm.toString(address(0x1234567890123456789012345678901234567890)));
        vm.setEnv(ENV_CENTRAL_BANK_ADDRESS, vm.toString(address(0x2345678901234567890123456789012345678901)));
        vm.setEnv("ADMIN_PRIVATE_KEY", "0");
    }

    function test_ScriptRun_Success() public {
        _setDeployEnv();
        deployScript.run();

        IdentityRegistry identityRegistry = deployScript.identityRegistry();
        HashTimeLockedContract htlc = deployScript.htlc();
        FXAgreement fxAgreement = deployScript.fxAgreement();
        ManualOracle oracle = deployScript.oracle();
        PairRegistry pairRegistry = deployScript.pairRegistry();
        CurrencyRegistry currencyRegistry = deployScript.currencyRegistry();
        LiquidityCommitRegistry lcr = deployScript.liquidityCommitRegistry();

        assertTrue(address(identityRegistry) != address(0), "IdentityRegistry was not deployed");
        assertTrue(address(htlc) != address(0), "HTLC was not deployed");
        assertTrue(address(fxAgreement) != address(0), "FXAgreement was not deployed");
        assertTrue(address(oracle) != address(0), "ManualOracle was not deployed");
        assertTrue(address(pairRegistry) != address(0), "PairRegistry was not deployed");
        assertTrue(address(currencyRegistry) != address(0), "CurrencyRegistry was not deployed");
        assertTrue(address(lcr) != address(0), "LiquidityCommitRegistry was not deployed");

        assertGt(address(identityRegistry).code.length, 0, "IdentityRegistry has no runtime bytecode");
        assertGt(address(htlc).code.length, 0, "HTLC has no runtime bytecode");
        assertGt(address(fxAgreement).code.length, 0, "FXAgreement has no runtime bytecode");
        assertGt(address(oracle).code.length, 0, "ManualOracle has no runtime bytecode");

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

        assertEq(
            address(htlc.IDENTITY_REGISTRY()),
            address(identityRegistry),
            "HTLC should reference the deployed IdentityRegistry"
        );
        assertEq(
            address(fxAgreement.IDENTITY_REGISTRY()),
            address(identityRegistry),
            "FXAgreement should reference the deployed IdentityRegistry"
        );

        assertTrue(
            oracle.hasRole(oracle.CENTRAL_BANK_ROLE(), deployScript.centralBankAddress()),
            "ManualOracle central bank role not granted"
        );
        assertTrue(
            oracle.hasRole(oracle.DEFAULT_ADMIN_ROLE(), deployScript.adminAddress()),
            "ManualOracle admin role not granted"
        );

        bytes32 unknownContractId = keccak256("UNKNOWN_CONTRACT");
        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(unknownContractId);
        assertEq(details.sender, address(0), "HTLC default sender should be zero address");
        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
