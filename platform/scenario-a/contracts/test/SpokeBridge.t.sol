// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {SpokeBridge} from "../src/SpokeBridge.sol";
import {ISpokeBridge} from "../src/interfaces/ISpokeBridge.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {DeploySpokeBridge} from "../script/SpokeBridge.s.sol";

/// @title SpokeBridgeTest
/// @notice Unit tests for the SpokeBridge contract (REQ-CAP-005).
/// @dev Covers: lock, release, idempotency, RBAC, clearance gate, getLock.
contract SpokeBridgeTest is Test {
    SpokeBridge public bridge;
    IdentityRegistry public registry;
    TokenizedCentralBankMoney public token;

    /// @dev Mirror events from ISpokeBridge for vm.expectEmit
    event AssetLocked(bytes32 indexed txId, address indexed sender, address token, uint256 amount, uint256 timestamp);
    event AssetReleased(bytes32 indexed txId, address indexed recipient, address token, uint256 amount);

    address public admin = address(0xAD);
    address public centralBank = address(0xCB);
    address public alice = address(0xA1);
    address public bob = address(0xB0);
    address public outsider = address(0x99);

    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");
    bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");
    bytes32 public constant TX_ID_1 = keccak256("tx-001");
    bytes32 public constant TX_ID_2 = keccak256("tx-002");

    uint256 public constant LOCK_AMOUNT = 1_000e18;

    function setUp() public {
        // Deploy registry
        registry = new IdentityRegistry(admin);

        // Register participants
        vm.startPrank(admin);
        registry.registerParticipant(
            alice, "Alice Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        registry.verifyParticipant(alice);
        registry.registerParticipant(
            bob, "Bob Bank", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        registry.verifyParticipant(bob);
        vm.stopPrank();

        // Deploy token
        token = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", admin, centralBank);

        // Deploy bridge
        bridge = new SpokeBridge(address(registry), admin);

        // Mint tokens to Alice and approve bridge
        vm.prank(centralBank);
        token.mint(alice, LOCK_AMOUNT * 10);
        vm.prank(alice);
        token.approve(address(bridge), type(uint256).max);
    }

    function test_Lock_Success() public {
        vm.prank(alice);
        vm.expectEmit(true, true, false, true);
        emit AssetLocked(TX_ID_1, alice, address(token), LOCK_AMOUNT, block.timestamp);
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);

        (address sender, address tkn, uint256 amount, bool released) = bridge.getLock(TX_ID_1);
        assertEq(sender, alice);
        assertEq(tkn, address(token));
        assertEq(amount, LOCK_AMOUNT);
        assertFalse(released);
        assertEq(token.balanceOf(address(bridge)), LOCK_AMOUNT);
    }

    function test_Revert_Lock_DuplicateTxId() public {
        vm.prank(alice);
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);

        vm.prank(alice);
        vm.expectRevert(ISpokeBridge.SB__TxAlreadyProcessed.selector);
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);
    }

    function test_Revert_Lock_UnverifiedSender() public {
        vm.prank(outsider);
        vm.expectRevert(abi.encodeWithSelector(ISpokeBridge.SB__ParticipantNotVerified.selector, outsider));
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);
    }

    function test_Revert_Lock_ZeroAmount() public {
        vm.prank(alice);
        vm.expectRevert(ISpokeBridge.SB__InvalidParameters.selector);
        bridge.lock(address(token), 0, TX_ID_1);
    }

    function test_Revert_Lock_ZeroToken() public {
        vm.prank(alice);
        vm.expectRevert(ISpokeBridge.SB__InvalidParameters.selector);
        bridge.lock(address(0), LOCK_AMOUNT, TX_ID_1);
    }

    function test_Release_Success() public {
        vm.prank(alice);
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);

        uint256 balanceBefore = token.balanceOf(alice);

        vm.prank(admin);
        vm.expectEmit(true, true, false, true);
        emit AssetReleased(TX_ID_1, alice, address(token), LOCK_AMOUNT);
        bridge.release(TX_ID_1);

        (,,, bool released) = bridge.getLock(TX_ID_1);
        assertTrue(released);
        assertEq(token.balanceOf(alice), balanceBefore + LOCK_AMOUNT);
        assertEq(token.balanceOf(address(bridge)), 0);
    }

    function test_Revert_Release_Unauthorized() public {
        vm.prank(alice);
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);

        vm.prank(outsider);
        vm.expectRevert();
        bridge.release(TX_ID_1);
    }

    function test_Revert_Release_TxNotFound() public {
        vm.prank(admin);
        vm.expectRevert(ISpokeBridge.SB__TxNotFound.selector);
        bridge.release(TX_ID_1);
    }

    function test_Revert_Release_AlreadyReleased() public {
        vm.prank(alice);
        bridge.lock(address(token), LOCK_AMOUNT, TX_ID_1);

        vm.prank(admin);
        bridge.release(TX_ID_1);

        vm.prank(admin);
        vm.expectRevert(ISpokeBridge.SB__TxAlreadyProcessed.selector);
        bridge.release(TX_ID_1);
    }

    function test_Revert_GetLock_NotFound() public {
        vm.expectRevert(ISpokeBridge.SB__TxNotFound.selector);
        bridge.getLock(TX_ID_1);
    }

    function test_Revert_Constructor_ZeroRegistry() public {
        vm.expectRevert(ISpokeBridge.SB__InvalidParameters.selector);
        new SpokeBridge(address(0), admin);
    }

    function test_Revert_Constructor_ZeroAdmin() public {
        vm.expectRevert(ISpokeBridge.SB__InvalidParameters.selector);
        new SpokeBridge(address(registry), address(0));
    }

    function test_ScriptRun_Success() public {
        vm.setEnv("DEPLOYER_PRIVATE_KEY", "0x1");
        vm.setEnv("ADMIN_ADDRESS", vm.toString(admin));
        vm.setEnv("IDENTITY_REGISTRY_ADDRESS", vm.toString(address(registry)));

        DeploySpokeBridge script = new DeploySpokeBridge();
        script.setUp();
        script.run();

        assertTrue(address(script.spokeBridge()) != address(0), "SpokeBridge was not deployed");
        assertGt(address(script.spokeBridge()).code.length, 0, "SpokeBridge has no bytecode");
    }
}
