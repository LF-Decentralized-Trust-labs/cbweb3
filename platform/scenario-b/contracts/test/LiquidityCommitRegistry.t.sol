// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {LiquidityCommitRegistry} from "../src/LiquidityCommitRegistry.sol";
import {ILiquidityCommitRegistry} from "../src/interfaces/ILiquidityCommitRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";

/// @title LiquidityCommitRegistryTest
/// @notice 8 Foundry unit tests for LiquidityCommitRegistry.sol (007-bridge-based-cb-liquidity).
/// @dev Covers registerCommit, cancelCommit, expireCommit, and CommitMatched emission.
contract LiquidityCommitRegistryTest is Test {
    // Re-declare events locally for vm.expectEmit compatibility.
    event CommitRegistered(
        bytes32 indexed commitId,
        string  poolPair,
        ILiquidityCommitRegistry.CommitSide side,
        address indexed signer,
        address wTokenAddr,
        uint256 amount,
        uint256 expiresAt
    );
    event CommitMatched(
        string  indexed poolPair,
        bytes32 commitIdA,
        address signerA,
        uint256 amountA,
        bytes32 commitIdB,
        address signerB,
        uint256 amountB
    );
    event CommitExpired(
        bytes32 indexed commitId,
        string  poolPair,
        ILiquidityCommitRegistry.CommitSide side
    );
    event CommitCancelled(bytes32 indexed commitId);

    LiquidityCommitRegistry public lcr;
    IdentityRegistry         public identityRegistry;
    TokenizedCentralBankMoney public tokenBRL;
    TokenizedCentralBankMoney public tokenARS;

    address public admin    = makeAddr("admin");
    address public cbA      = makeAddr("central_bank_a");
    address public cbB      = makeAddr("central_bank_b");
    address public attacker = makeAddr("attacker");

    string  constant POOL_PAIR = "W-BRL-ARS";
    uint256 constant AMOUNT_A  = 100_000e18;
    uint256 constant AMOUNT_B  = 200_000e18;

    function setUp() public {
        // Deploy IdentityRegistry and register both central banks.
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            cbA, "Central Bank A", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            cbB, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );

        // Deploy W-tCeBM tokens with respective CBs as CENTRAL_BANK_ROLE holders.
        tokenBRL = new TokenizedCentralBankMoney("Wrapped tCeBM BRL", "W-tCeBM_BRL", admin, cbA);
        tokenARS = new TokenizedCentralBankMoney("Wrapped tCeBM ARS", "W-tCeBM_ARS", admin, cbB);

        // Map each token to its issuing CB.
        identityRegistry.setCentralBankOf(address(tokenBRL), cbA);
        identityRegistry.setCentralBankOf(address(tokenARS), cbB);
        vm.stopPrank();

        // Deploy LiquidityCommitRegistry.
        lcr = new LiquidityCommitRegistry(address(identityRegistry));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 1. test_registerCommit_singleSide
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice Registers side A; no match yet (side B absent). Verifies storage via getCommit.
    function test_registerCommit_singleSide() public {
        vm.prank(cbA);
        bytes32 commitId = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        assertNotEq(commitId, bytes32(0), "commitId must be non-zero");

        (address signer, address wToken, uint256 amount,, ILiquidityCommitRegistry.CommitStatus status, ILiquidityCommitRegistry.CommitSide side)
            = lcr.getCommit(commitId);

        assertEq(signer, cbA);
        assertEq(wToken, address(tokenBRL));
        assertEq(amount, AMOUNT_A);
        assertEq(uint8(status), uint8(ILiquidityCommitRegistry.CommitStatus.PENDING));
        assertEq(uint8(side), uint8(ILiquidityCommitRegistry.CommitSide.A));

        // getPendingCommit should return this commitId.
        assertEq(lcr.getPendingCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A), commitId);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 2. test_registerCommit_triggerMatch
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice CB-A registers side A, then CB-B registers side B → CommitMatched emitted.
    function test_registerCommit_triggerMatch() public {
        vm.prank(cbA);
        bytes32 commitIdA = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        // Pre-compute commitIdB using the same derivation as the contract.
        // Both registerCommit calls happen in the same block (timestamp unchanged).
        bytes32 expectedCommitIdB = keccak256(abi.encode(
            POOL_PAIR,
            ILiquidityCommitRegistry.CommitSide.B,
            cbB,
            AMOUNT_B,
            address(tokenARS),
            block.timestamp
        ));

        vm.expectEmit(true, false, false, true);
        emit CommitMatched(POOL_PAIR, commitIdA, cbA, AMOUNT_A, expectedCommitIdB, cbB, AMOUNT_B);

        vm.prank(cbB);
        bytes32 commitIdB = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B, AMOUNT_B, address(tokenARS));

        assertEq(commitIdB, expectedCommitIdB, "commitIdB mismatch");
        assertNotEq(commitIdB, bytes32(0));

        // Both commits should now be MATCHED.
        (,,,,ILiquidityCommitRegistry.CommitStatus statusA,) = lcr.getCommit(commitIdA);
        (,,,,ILiquidityCommitRegistry.CommitStatus statusB,) = lcr.getCommit(commitIdB);

        assertEq(uint8(statusA), uint8(ILiquidityCommitRegistry.CommitStatus.MATCHED));
        assertEq(uint8(statusB), uint8(ILiquidityCommitRegistry.CommitStatus.MATCHED));

        // Pending slots must be cleared after match.
        assertEq(lcr.getPendingCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A), bytes32(0));
        assertEq(lcr.getPendingCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B), bytes32(0));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 3. test_registerCommit_revertsIfNotCB
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice Reverts when caller is NOT getCentralBankOf(wTokenAddress).
    function test_registerCommit_revertsIfNotCB() public {
        vm.prank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(ILiquidityCommitRegistry.LCR__NotTokenCentralBank.selector, attacker, address(tokenBRL))
        );
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 4. test_registerCommit_revertsIfAlreadyPending
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice Reverts when a second PENDING commit is attempted for the same (poolPair, side).
    function test_registerCommit_revertsIfAlreadyPending() public {
        vm.prank(cbA);
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        vm.prank(cbA);
        vm.expectRevert(
            abi.encodeWithSelector(ILiquidityCommitRegistry.LCR__CommitAlreadyPending.selector, POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A)
        );
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 5. test_cancelCommit
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice CB-A registers side A then cancels it. Pending slot is cleared.
    function test_cancelCommit() public {
        vm.prank(cbA);
        bytes32 commitId = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        vm.expectEmit(true, false, false, false);
        emit CommitCancelled(commitId);

        vm.prank(cbA);
        lcr.cancelCommit(commitId);

        // Status must be CANCELLED.
        (,,,,ILiquidityCommitRegistry.CommitStatus status,) = lcr.getCommit(commitId);
        assertEq(uint8(status), uint8(ILiquidityCommitRegistry.CommitStatus.CANCELLED));

        // Pending slot cleared — a new commit can now be registered.
        assertEq(lcr.getPendingCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A), bytes32(0));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 6. test_cancelCommit_revertsIfNotOwner
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice Only the original signer can cancel a commit.
    function test_cancelCommit_revertsIfNotOwner() public {
        vm.prank(cbA);
        bytes32 commitId = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        vm.prank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(ILiquidityCommitRegistry.LCR__NotCommitOwner.selector, commitId, attacker)
        );
        lcr.cancelCommit(commitId);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 7. test_expireCommit (vm.warp)
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice After 72h + 1s, any address can expire a PENDING commit.
    function test_expireCommit() public {
        vm.prank(cbA);
        bytes32 commitId = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        // Warp past the 72-hour TTL.
        vm.warp(block.timestamp + 72 hours + 1);

        vm.expectEmit(true, false, false, true);
        emit CommitExpired(commitId, POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A);

        // attacker (or any address) can call expireCommit — permissionless GC.
        vm.prank(attacker);
        lcr.expireCommit(commitId);

        (,,,,ILiquidityCommitRegistry.CommitStatus status,) = lcr.getCommit(commitId);
        assertEq(uint8(status), uint8(ILiquidityCommitRegistry.CommitStatus.EXPIRED));

        // Pending slot cleared.
        assertEq(lcr.getPendingCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A), bytes32(0));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 8. test_expireCommit_revertsIfNotExpired
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice Reverts if the commit TTL has not elapsed.
    function test_expireCommit_revertsIfNotExpired() public {
        vm.prank(cbA);
        bytes32 commitId = lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        // Only 1 hour has passed — commit not yet expired.
        vm.warp(block.timestamp + 1 hours);

        vm.prank(attacker);
        vm.expectRevert(); // LCR__CommitNotExpired
        lcr.expireCommit(commitId);
    }
}
