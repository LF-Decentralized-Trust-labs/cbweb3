// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

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
        string poolPair,
        ILiquidityCommitRegistry.CommitSide side,
        address indexed signer,
        address wTokenAddr,
        uint256 amount,
        uint256 expiresAt
    );
    event CommitMatched(
        string indexed poolPair,
        bytes32 commitIdA,
        address signerA,
        uint256 amountA,
        bytes32 commitIdB,
        address signerB,
        uint256 amountB
    );
    event CommitExpired(bytes32 indexed commitId, string poolPair, ILiquidityCommitRegistry.CommitSide side);
    event CommitCancelled(bytes32 indexed commitId);

    LiquidityCommitRegistry public lcr;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenBRL;
    TokenizedCentralBankMoney public tokenARS;

    address public admin = makeAddr("admin");
    address public cbA = makeAddr("central_bank_a");
    address public cbB = makeAddr("central_bank_b");
    address public attacker = makeAddr("attacker");

    string constant POOL_PAIR = "W-BRL-ARS";
    uint256 constant AMOUNT_A = 100_000e18;
    uint256 constant AMOUNT_B = 200_000e18;

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
        bytes32 commitId =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        assertNotEq(commitId, bytes32(0), "commitId must be non-zero");

        (
            address signer,
            address wToken,
            uint256 amount,,
            ILiquidityCommitRegistry.CommitStatus status,
            ILiquidityCommitRegistry.CommitSide side
        ) = lcr.getCommit(commitId);

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
        bytes32 commitIdA =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        // Pre-compute commitIdB using the same derivation as the contract.
        // Both registerCommit calls happen in the same block (timestamp unchanged).
        bytes32 expectedCommitIdB = keccak256(
            abi.encode(
                POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B, cbB, AMOUNT_B, address(tokenARS), block.timestamp
            )
        );

        vm.expectEmit(true, false, false, true);
        emit CommitMatched(POOL_PAIR, commitIdA, cbA, AMOUNT_A, expectedCommitIdB, cbB, AMOUNT_B);

        vm.prank(cbB);
        bytes32 commitIdB =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B, AMOUNT_B, address(tokenARS));

        assertEq(commitIdB, expectedCommitIdB, "commitIdB mismatch");
        assertNotEq(commitIdB, bytes32(0));

        // Both commits should now be MATCHED.
        (,,,, ILiquidityCommitRegistry.CommitStatus statusA,) = lcr.getCommit(commitIdA);
        (,,,, ILiquidityCommitRegistry.CommitStatus statusB,) = lcr.getCommit(commitIdB);

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
            abi.encodeWithSelector(
                ILiquidityCommitRegistry.LCR__NotTokenCentralBank.selector, attacker, address(tokenBRL)
            )
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
            abi.encodeWithSelector(
                ILiquidityCommitRegistry.LCR__CommitAlreadyPending.selector,
                POOL_PAIR,
                ILiquidityCommitRegistry.CommitSide.A
            )
        );
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 5. test_cancelCommit
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice CB-A registers side A then cancels it. Pending slot is cleared.
    function test_cancelCommit() public {
        vm.prank(cbA);
        bytes32 commitId =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        vm.expectEmit(true, false, false, false);
        emit CommitCancelled(commitId);

        vm.prank(cbA);
        lcr.cancelCommit(commitId);

        // Status must be CANCELLED.
        (,,,, ILiquidityCommitRegistry.CommitStatus status,) = lcr.getCommit(commitId);
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
        bytes32 commitId =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

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
        bytes32 commitId =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        // Warp past the 72-hour TTL.
        vm.warp(block.timestamp + 72 hours + 1);

        vm.expectEmit(true, false, false, true);
        emit CommitExpired(commitId, POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A);

        // attacker (or any address) can call expireCommit — permissionless GC.
        vm.prank(attacker);
        lcr.expireCommit(commitId);

        (,,,, ILiquidityCommitRegistry.CommitStatus status,) = lcr.getCommit(commitId);
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
        bytes32 commitId =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        // Only 1 hour has passed — commit not yet expired.
        vm.warp(block.timestamp + 1 hours);

        vm.prank(attacker);
        vm.expectRevert(); // LCR__CommitNotExpired
        lcr.expireCommit(commitId);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 9. Constructor zero-address guard
    // ─────────────────────────────────────────────────────────────────────────

    function test_Revert_Constructor_ZeroRegistry() public {
        vm.expectRevert(ILiquidityCommitRegistry.LCR__InvalidParameters.selector);
        new LiquidityCommitRegistry(address(0));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 10. registerCommit input validation branches
    // ─────────────────────────────────────────────────────────────────────────

    function test_Revert_registerCommit_ZeroAmount() public {
        vm.prank(cbA);
        vm.expectRevert(ILiquidityCommitRegistry.LCR__InvalidParameters.selector);
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, 0, address(tokenBRL));
    }

    function test_Revert_registerCommit_EmptyPoolPair() public {
        vm.prank(cbA);
        vm.expectRevert(ILiquidityCommitRegistry.LCR__InvalidParameters.selector);
        lcr.registerCommit("", ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
    }

    function test_Revert_registerCommit_ZeroToken() public {
        vm.prank(cbA);
        vm.expectRevert(ILiquidityCommitRegistry.LCR__InvalidParameters.selector);
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(0));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 11. Match triggered by side B registering first (covers the else-ordering branch)
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice CB-B registers side B first, then CB-A registers side A → CommitMatched emitted with
    ///         A/B ordering resolved via the `side == CommitSide.A` false branch.
    function test_registerCommit_triggerMatch_BFirst() public {
        vm.prank(cbB);
        bytes32 commitIdB =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B, AMOUNT_B, address(tokenARS));

        bytes32 expectedCommitIdA = keccak256(
            abi.encode(
                POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, cbA, AMOUNT_A, address(tokenBRL), block.timestamp
            )
        );

        vm.expectEmit(true, false, false, true);
        emit CommitMatched(POOL_PAIR, expectedCommitIdA, cbA, AMOUNT_A, commitIdB, cbB, AMOUNT_B);

        vm.prank(cbA);
        bytes32 commitIdA =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));

        assertEq(commitIdA, expectedCommitIdA);

        (,,,, ILiquidityCommitRegistry.CommitStatus statusA,) = lcr.getCommit(commitIdA);
        (,,,, ILiquidityCommitRegistry.CommitStatus statusB,) = lcr.getCommit(commitIdB);
        assertEq(uint8(statusA), uint8(ILiquidityCommitRegistry.CommitStatus.MATCHED));
        assertEq(uint8(statusB), uint8(ILiquidityCommitRegistry.CommitStatus.MATCHED));
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 12. cancelCommit / expireCommit not-found and not-pending branches
    // ─────────────────────────────────────────────────────────────────────────

    function test_Revert_cancelCommit_NotFound() public {
        bytes32 bogus = keccak256("does-not-exist");
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(ILiquidityCommitRegistry.LCR__CommitNotFound.selector, bogus));
        lcr.cancelCommit(bogus);
    }

    function test_Revert_cancelCommit_NotPending() public {
        // Register and match both sides → MATCHED, no longer PENDING.
        vm.prank(cbA);
        bytes32 commitIdA =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
        vm.prank(cbB);
        lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B, AMOUNT_B, address(tokenARS));

        vm.prank(cbA);
        vm.expectRevert(
            abi.encodeWithSelector(
                ILiquidityCommitRegistry.LCR__CommitNotPending.selector,
                commitIdA,
                ILiquidityCommitRegistry.CommitStatus.MATCHED
            )
        );
        lcr.cancelCommit(commitIdA);
    }

    function test_Revert_expireCommit_NotFound() public {
        bytes32 bogus = keccak256("does-not-exist");
        vm.prank(attacker);
        vm.expectRevert(abi.encodeWithSelector(ILiquidityCommitRegistry.LCR__CommitNotFound.selector, bogus));
        lcr.expireCommit(bogus);
    }

    function test_Revert_expireCommit_NotPending() public {
        vm.prank(cbA);
        bytes32 commitId =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
        vm.prank(cbA);
        lcr.cancelCommit(commitId);

        vm.warp(block.timestamp + 72 hours + 1);
        vm.prank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                ILiquidityCommitRegistry.LCR__CommitNotPending.selector,
                commitId,
                ILiquidityCommitRegistry.CommitStatus.CANCELLED
            )
        );
        lcr.expireCommit(commitId);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 13. getCommit not-found branch
    // ─────────────────────────────────────────────────────────────────────────

    function test_Revert_getCommit_NotFound() public {
        bytes32 bogus = keccak256("does-not-exist");
        vm.expectRevert(abi.encodeWithSelector(ILiquidityCommitRegistry.LCR__CommitNotFound.selector, bogus));
        lcr.getCommit(bogus);
    }

    // ─────────────────────────────────────────────────────────────────────────
    // 14. Re-register after cancel does not re-trigger a stale match (counter not PENDING)
    // ─────────────────────────────────────────────────────────────────────────

    /// @notice After side A is cancelled, registering side B finds no PENDING counterpart, so the
    ///         match block's `counter.status == PENDING` check is exercised on the false path via
    ///         the cleared slot (counterCommitId == 0). This keeps side B PENDING (unmatched).
    function test_registerCommit_NoMatchAfterCounterCancelled() public {
        vm.prank(cbA);
        bytes32 commitIdA =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.A, AMOUNT_A, address(tokenBRL));
        vm.prank(cbA);
        lcr.cancelCommit(commitIdA);

        vm.prank(cbB);
        bytes32 commitIdB =
            lcr.registerCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B, AMOUNT_B, address(tokenARS));

        (,,,, ILiquidityCommitRegistry.CommitStatus statusB,) = lcr.getCommit(commitIdB);
        assertEq(uint8(statusB), uint8(ILiquidityCommitRegistry.CommitStatus.PENDING), "B stays PENDING, no match");
        assertEq(lcr.getPendingCommit(POOL_PAIR, ILiquidityCommitRegistry.CommitSide.B), commitIdB);
    }
}
