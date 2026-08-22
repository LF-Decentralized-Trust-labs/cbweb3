// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {CommitmentHashRegistry} from "../src/CommitmentHashRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title CommitmentHashRegistryTest
/// @notice Unit tests for the CommitmentHashRegistry fallback gating contract.
contract CommitmentHashRegistryTest is Test {
    CommitmentHashRegistry public commitmentHashRegistry;
    IdentityRegistry public identityRegistry;

    address public admin = makeAddr("admin");
    address public governance = makeAddr("governance");
    address public bankA = makeAddr("bankA");
    address public bankB = makeAddr("bankB");
    address public unauthorized = makeAddr("unauthorized");

    bytes32 public tradeId = keccak256("FX_TRADE_001");
    uint256 public originAmount = 1_000_000 * 10 ** 18;
    uint256 public counterAmount = 5_500_000 * 10 ** 18;
    uint256 public rate = 5_500 * 10 ** 15; // 5.5 BRL/EUR scaled by 1e18

    function setUp() public {
        // Deploy IdentityRegistry
        identityRegistry = new IdentityRegistry(admin);

        // Register governance participant
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            governance,
            "Governance",
            IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK,
            bytes32(0),
            bytes32("inst-governance")
        );
        identityRegistry.verifyParticipant(governance);
        vm.stopPrank();

        // Deploy CommitmentHashRegistry
        commitmentHashRegistry = new CommitmentHashRegistry(address(identityRegistry));
    }

    function test_Revert_Constructor_ZeroRegistry() public {
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        new CommitmentHashRegistry(address(0));
    }

    function test_RegisterCommitment_Success() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        CommitmentHashRegistry.CommitmentState state = commitmentHashRegistry.getCommitmentState(expectedHash);
        assertEq(uint256(state), uint256(CommitmentHashRegistry.CommitmentState.PENDING));

        CommitmentHashRegistry.Commitment memory commitment = commitmentHashRegistry.getCommitment(expectedHash);
        assertEq(commitment.tradeId, tradeId);
        assertEq(commitment.originator, bankA);
        assertEq(commitment.counterpartyB, bankB);
        assertEq(commitment.originAmount, originAmount);
        assertEq(commitment.counterAmount, counterAmount);
        assertEq(commitment.rate, rate);
    }

    function test_Revert_RegisterCommitment_Unauthorized() public {
        vm.prank(unauthorized);
        vm.expectRevert(CommitmentHashRegistry.CRG__Unauthorized.selector);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);
    }

    function test_Revert_RegisterCommitment_InvalidParameters() public {
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        commitmentHashRegistry.registerCommitment(bytes32(0), bankA, bankB, originAmount, counterAmount, rate);
    }

    function test_Revert_RegisterCommitment_DuplicateHash() public {
        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__CommitmentAlreadyExists.selector);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);
    }

    function test_AcceptCommitment_Success() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        vm.prank(governance);
        commitmentHashRegistry.acceptCommitment(expectedHash);

        CommitmentHashRegistry.CommitmentState state = commitmentHashRegistry.getCommitmentState(expectedHash);
        assertEq(uint256(state), uint256(CommitmentHashRegistry.CommitmentState.ACCEPTED));

        bool isAccepted = commitmentHashRegistry.isAccepted(expectedHash);
        assertTrue(isAccepted);
    }

    function test_SettleCommitment_Success() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        vm.prank(governance);
        commitmentHashRegistry.acceptCommitment(expectedHash);

        vm.prank(governance);
        commitmentHashRegistry.settleCommitment(expectedHash);

        CommitmentHashRegistry.CommitmentState state = commitmentHashRegistry.getCommitmentState(expectedHash);
        assertEq(uint256(state), uint256(CommitmentHashRegistry.CommitmentState.SETTLED));

        bool isAccepted = commitmentHashRegistry.isAccepted(expectedHash);
        assertFalse(isAccepted);
    }

    function test_CancelCommitment_Success() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        vm.prank(governance);
        commitmentHashRegistry.cancelCommitment(expectedHash);

        CommitmentHashRegistry.CommitmentState state = commitmentHashRegistry.getCommitmentState(expectedHash);
        assertEq(uint256(state), uint256(CommitmentHashRegistry.CommitmentState.CANCELLED));
    }

    function test_GetCommitmentHashByTradeId_Success() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        bytes32 retrievedHash = commitmentHashRegistry.getCommitmentHashByTradeId(tradeId);
        assertEq(retrievedHash, expectedHash);
    }

    function test_Revert_AcceptCommitment_InvalidState() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        vm.prank(governance);
        commitmentHashRegistry.acceptCommitment(expectedHash);

        // Try to accept again
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidStateTransition.selector);
        commitmentHashRegistry.acceptCommitment(expectedHash);
    }

    // --- registerCommitment parameter-validation branches ---

    function test_Revert_RegisterCommitment_ZeroOriginator() public {
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        commitmentHashRegistry.registerCommitment(tradeId, address(0), bankB, originAmount, counterAmount, rate);
    }

    function test_Revert_RegisterCommitment_ZeroCounterparty() public {
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, address(0), originAmount, counterAmount, rate);
    }

    function test_Revert_RegisterCommitment_ZeroOriginAmount() public {
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, 0, counterAmount, rate);
    }

    function test_Revert_RegisterCommitment_ZeroCounterAmount() public {
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, 0, rate);
    }

    function test_Revert_RegisterCommitment_ZeroRate() public {
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidParameters.selector);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, 0);
    }

    // --- settleCommitment branch ---

    function test_Revert_SettleCommitment_NotAccepted() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.prank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);

        // Still PENDING, not ACCEPTED
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidStateTransition.selector);
        commitmentHashRegistry.settleCommitment(expectedHash);
    }

    // --- cancelCommitment terminal-state branches ---

    function test_Revert_CancelCommitment_Invalid() public {
        // never registered → INVALID
        vm.prank(governance);
        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidStateTransition.selector);
        commitmentHashRegistry.cancelCommitment(keccak256("unknown"));
    }

    function test_Revert_CancelCommitment_Settled() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.startPrank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);
        commitmentHashRegistry.acceptCommitment(expectedHash);
        commitmentHashRegistry.settleCommitment(expectedHash);

        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidStateTransition.selector);
        commitmentHashRegistry.cancelCommitment(expectedHash);
        vm.stopPrank();
    }

    function test_Revert_CancelCommitment_AlreadyCancelled() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.startPrank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);
        commitmentHashRegistry.cancelCommitment(expectedHash);

        vm.expectRevert(CommitmentHashRegistry.CRG__InvalidStateTransition.selector);
        commitmentHashRegistry.cancelCommitment(expectedHash);
        vm.stopPrank();
    }

    function test_CancelCommitment_FromAccepted() public {
        bytes32 expectedHash = keccak256(abi.encodePacked(tradeId, originAmount, counterAmount, rate));

        vm.startPrank(governance);
        commitmentHashRegistry.registerCommitment(tradeId, bankA, bankB, originAmount, counterAmount, rate);
        commitmentHashRegistry.acceptCommitment(expectedHash);
        commitmentHashRegistry.cancelCommitment(expectedHash);
        vm.stopPrank();

        assertEq(
            uint256(commitmentHashRegistry.getCommitmentState(expectedHash)),
            uint256(CommitmentHashRegistry.CommitmentState.CANCELLED)
        );
    }

    // --- view not-found branches ---

    function test_Revert_GetCommitment_NotFound() public {
        vm.expectRevert(CommitmentHashRegistry.CRG__CommitmentNotFound.selector);
        commitmentHashRegistry.getCommitment(keccak256("missing"));
    }

    function test_Revert_GetCommitmentHashByTradeId_NotFound() public {
        vm.expectRevert(CommitmentHashRegistry.CRG__CommitmentNotFound.selector);
        commitmentHashRegistry.getCommitmentHashByTradeId(keccak256("missing-trade"));
    }
}
