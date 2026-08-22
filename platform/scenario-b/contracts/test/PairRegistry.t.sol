// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {PairRegistry} from "../src/PairRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";

/// @title PairRegistryTest
/// @notice Unit tests for PairRegistry bilateral pair approval (D9 — 005-cooperative-liquidity).
/// @dev Verifies: propose, confirm, authorization guards, duplicate prevention, iteration.
contract PairRegistryTest is Test {
    // Re-declare events locally for vm.expectEmit compatibility (Solidity 0.8.20).
    event PairProposed(string indexed pairId, address indexed proposer, address tokenA, address tokenB);
    event PairRegistered(string indexed pairId, address indexed ammAddress, address tokenA, address tokenB);

    PairRegistry public pairRegistry;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenBRL;
    TokenizedCentralBankMoney public tokenUSD;
    TokenizedCentralBankMoney public tokenARS;

    address public admin = makeAddr("admin");
    address public cbBRL = makeAddr("cbBRL"); // Central Bank of Brazil (issues tokenBRL)
    address public cbUSD = makeAddr("cbUSD"); // Federal Reserve (issues tokenUSD)
    address public cbARS = makeAddr("cbARS"); // Central Bank of Argentina (issues tokenARS)
    address public attacker = makeAddr("attacker");
    address public fakeAMM = makeAddr("fakeAMM");

    function setUp() public {
        // Deploy tokens
        tokenBRL = new TokenizedCentralBankMoney("tCeBM BRL", "tBRL", admin, cbBRL);
        tokenUSD = new TokenizedCentralBankMoney("tCeBM USD", "tUSD", admin, cbUSD);
        tokenARS = new TokenizedCentralBankMoney("tCeBM ARS", "tARS", admin, cbARS);

        // Deploy IdentityRegistry and register participants
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            cbBRL, "Banco Central do Brasil", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.verifyParticipant(cbBRL);
        identityRegistry.registerParticipant(
            cbUSD, "Federal Reserve", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.verifyParticipant(cbUSD);
        identityRegistry.registerParticipant(
            cbARS, "Banco Central de Argentina", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.verifyParticipant(cbARS);
        // Map each token to its issuing CB
        identityRegistry.setCentralBankOf(address(tokenBRL), cbBRL);
        identityRegistry.setCentralBankOf(address(tokenUSD), cbUSD);
        identityRegistry.setCentralBankOf(address(tokenARS), cbARS);
        vm.stopPrank();

        // Deploy PairRegistry
        pairRegistry = new PairRegistry(address(identityRegistry));
    }

    // ─────────────────────── proposePair ─────────────────────────────

    /// @notice CB of tokenA can successfully propose a new pair.
    function test_proposePair_emitsPairProposed() public {
        vm.expectEmit(true, true, false, true);
        emit PairProposed("BRL-USD", cbBRL, address(tokenBRL), address(tokenUSD));

        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);

        PairRegistry.PairEntry memory e = pairRegistry.getPair("BRL-USD");
        assertEq(e.pairId, "BRL-USD");
        assertEq(e.ammAddress, fakeAMM);
        assertEq(e.tokenA, address(tokenBRL));
        assertEq(e.tokenB, address(tokenUSD));
        assertEq(uint256(e.status), uint256(PairRegistry.PairStatus.PROPOSED));
        assertEq(e.proposer, cbBRL);
        assertEq(e.confirmer, address(0));
    }

    /// @notice A non-CB address cannot propose a pair for tokenA.
    function test_wrongCB_cannotPropose() public {
        vm.prank(attacker);
        vm.expectRevert(PairRegistry.PairRegistry__Unauthorized.selector);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);
    }

    /// @notice Proposing with a zero tokenA address reverts.
    function test_proposePair_zeroTokenA_reverts() public {
        vm.prank(cbBRL);
        vm.expectRevert(PairRegistry.PairRegistry__ZeroAddress.selector);
        pairRegistry.proposePair("BRL-USD", address(0), address(tokenUSD), fakeAMM);
    }

    /// @notice Proposing with a zero AMM address reverts.
    function test_proposePair_zeroAMM_reverts() public {
        vm.prank(cbBRL);
        vm.expectRevert(PairRegistry.PairRegistry__ZeroAddress.selector);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), address(0));
    }

    /// @notice A duplicate pairId reverts.
    function test_duplicatePairId_reverts() public {
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);

        vm.prank(cbBRL);
        vm.expectRevert(abi.encodeWithSelector(PairRegistry.PairRegistry__AlreadyExists.selector, "BRL-USD"));
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);
    }

    // ─────────────────────── confirmPair ─────────────────────────────

    /// @notice CB of tokenB can confirm a proposed pair; emits PairRegistered.
    function test_confirmPair_emitsPairRegistered() public {
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);

        vm.expectEmit(true, true, false, true);
        emit PairRegistered("BRL-USD", fakeAMM, address(tokenBRL), address(tokenUSD));

        vm.prank(cbUSD);
        pairRegistry.confirmPair("BRL-USD");

        PairRegistry.PairEntry memory e = pairRegistry.getPair("BRL-USD");
        assertEq(uint256(e.status), uint256(PairRegistry.PairStatus.ACTIVE));
        assertEq(e.confirmer, cbUSD);
    }

    /// @notice A non-CB address cannot confirm a pair for tokenB.
    function test_wrongCB_cannotConfirm() public {
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);

        vm.prank(attacker);
        vm.expectRevert(PairRegistry.PairRegistry__Unauthorized.selector);
        pairRegistry.confirmPair("BRL-USD");
    }

    /// @notice Confirming an unknown pair reverts.
    function test_confirmPair_unknownPair_reverts() public {
        vm.expectRevert(abi.encodeWithSelector(PairRegistry.PairRegistry__NotFound.selector, "BRL-USD"));
        vm.prank(cbUSD);
        pairRegistry.confirmPair("BRL-USD");
    }

    /// @notice Confirming an already-ACTIVE pair reverts.
    function test_confirmPair_alreadyActive_reverts() public {
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);
        vm.prank(cbUSD);
        pairRegistry.confirmPair("BRL-USD");

        vm.prank(cbUSD);
        vm.expectRevert(abi.encodeWithSelector(PairRegistry.PairRegistry__AlreadyActive.selector, "BRL-USD"));
        pairRegistry.confirmPair("BRL-USD");
    }

    // ──────────────────── getAllActivePairs ───────────────────────────

    /// @notice getAllActivePairs returns only ACTIVE pairs (not PROPOSED).
    function test_getAllActivePairs_returnsOnlyActive() public {
        address fakeAMM2 = makeAddr("fakeAMM2");

        // Propose BRL-USD and BRL-ARS
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-ARS", address(tokenBRL), address(tokenARS), fakeAMM2);

        // Confirm only BRL-USD
        vm.prank(cbUSD);
        pairRegistry.confirmPair("BRL-USD");

        PairRegistry.PairEntry[] memory active = pairRegistry.getAllActivePairs();
        assertEq(active.length, 1);
        assertEq(active[0].pairId, "BRL-USD");
        assertEq(uint256(active[0].status), uint256(PairRegistry.PairStatus.ACTIVE));
    }

    /// @notice getAllActivePairs returns empty array when no pairs are active.
    function test_getAllActivePairs_emptyWhenNoActive() public view {
        PairRegistry.PairEntry[] memory active = pairRegistry.getAllActivePairs();
        assertEq(active.length, 0);
    }

    /// @notice After confirming all proposed pairs, getAllActivePairs returns all of them.
    function test_getAllActivePairs_multipleActive() public {
        address fakeAMM2 = makeAddr("fakeAMM2");

        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-ARS", address(tokenBRL), address(tokenARS), fakeAMM2);

        vm.prank(cbUSD);
        pairRegistry.confirmPair("BRL-USD");
        vm.prank(cbARS);
        pairRegistry.confirmPair("BRL-ARS");

        PairRegistry.PairEntry[] memory active = pairRegistry.getAllActivePairs();
        assertEq(active.length, 2);
    }

    // ──────────────────────── getAllPairs ─────────────────────────────

    /// @notice getAllPairs returns every pair, including PROPOSED ones (no status filter).
    function test_getAllPairs_includesProposed() public {
        address fakeAMM2 = makeAddr("fakeAMM2");

        // Propose BRL-USD and BRL-ARS
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-USD", address(tokenBRL), address(tokenUSD), fakeAMM);
        vm.prank(cbBRL);
        pairRegistry.proposePair("BRL-ARS", address(tokenBRL), address(tokenARS), fakeAMM2);

        // Confirm only BRL-USD; BRL-ARS stays PROPOSED
        vm.prank(cbUSD);
        pairRegistry.confirmPair("BRL-USD");

        PairRegistry.PairEntry[] memory all = pairRegistry.getAllPairs();
        assertEq(all.length, 2);

        // getAllActivePairs would drop BRL-ARS; getAllPairs must keep it.
        bool sawProposed;
        bool sawActive;
        for (uint256 i; i < all.length; ++i) {
            if (keccak256(bytes(all[i].pairId)) == keccak256("BRL-ARS")) {
                sawProposed = all[i].status == PairRegistry.PairStatus.PROPOSED;
            }
            if (keccak256(bytes(all[i].pairId)) == keccak256("BRL-USD")) {
                sawActive = all[i].status == PairRegistry.PairStatus.ACTIVE;
            }
        }
        assertTrue(sawProposed, "PROPOSED pair must be returned by getAllPairs");
        assertTrue(sawActive, "ACTIVE pair must be returned by getAllPairs");
    }

    /// @notice getAllPairs returns an empty array when no pairs are registered.
    function test_getAllPairs_emptyWhenNone() public view {
        PairRegistry.PairEntry[] memory all = pairRegistry.getAllPairs();
        assertEq(all.length, 0);
    }

    // ─────────────────── getPair / constructor ────────────────────────

    /// @notice getPair reverts for an unknown pairId.
    function test_getPair_unknownPair_reverts() public {
        vm.expectRevert(abi.encodeWithSelector(PairRegistry.PairRegistry__NotFound.selector, "UNKNOWN"));
        pairRegistry.getPair("UNKNOWN");
    }

    /// @notice Constructor reverts with zero registry address.
    function test_constructor_zeroRegistry_reverts() public {
        vm.expectRevert(PairRegistry.PairRegistry__ZeroAddress.selector);
        new PairRegistry(address(0));
    }

    // ──────────────────── setCentralBankOf ───────────────────────────

    /// @notice Admin can set and retrieve the CB of a token via IdentityRegistry.
    function test_setCentralBankOf_and_getCentralBankOf() public {
        address newToken = makeAddr("newToken");
        address newCB = makeAddr("newCB");

        vm.prank(admin);
        identityRegistry.setCentralBankOf(newToken, newCB);

        assertEq(identityRegistry.getCentralBankOf(newToken), newCB);
    }

    /// @notice Non-admin cannot call setCentralBankOf.
    function test_setCentralBankOf_nonAdmin_reverts() public {
        vm.prank(attacker);
        vm.expectRevert();
        identityRegistry.setCentralBankOf(address(tokenBRL), attacker);
    }
}
