// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {PairRegistry} from "../src/PairRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";

/// @title PairRegistryPaginationTest
/// @notice Bounded reads for PairRegistry (finding R2-M-14).
/// @dev The mirror of RegistryPaginationTest, and not a formality: getActivePairsPaged filters on
///      `status == ACTIVE` where CurrencyRegistry filters on `_symbolExists`, so it is a separate
///      predicate over a separate array and shares none of the other contract's coverage.
///
///      The fixture deliberately leaves one pair PROPOSED, in the middle of `_pairIds`. Without a
///      non-ACTIVE entry the active index and the raw array index are identical, and an
///      implementation that offsets by array position passes every test that does not have a hole
///      to trip over.
contract PairRegistryPaginationTest is Test {
    PairRegistry public registry;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney[] public tokens;

    address public admin = makeAddr("admin");
    address public cb = makeAddr("central_bank");
    address public amm = makeAddr("amm");

    /// Number of ACTIVE pairs seeded. One further pair is left PROPOSED.
    uint256 constant SEEDED = 5;
    string constant PROPOSED_ONLY = "PAIR_PROPOSED";

    function setUp() public {
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            cb, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        identityRegistry.verifyParticipant(cb);
        vm.stopPrank();

        // One CB issues every token, so it is both proposer and confirmer. PairRegistry does not
        // require those to differ; the pagination logic does not care either way.
        for (uint256 i; i <= SEEDED; ++i) {
            string memory sym = string.concat("tCeBM_", vm.toString(i));
            TokenizedCentralBankMoney tok = new TokenizedCentralBankMoney(sym, sym, admin, cb);
            vm.prank(admin);
            identityRegistry.setCentralBankOf(address(tok), cb);
            tokens.push(tok);
        }

        registry = new PairRegistry(address(identityRegistry));

        for (uint256 i; i < SEEDED; ++i) {
            // Drop the PROPOSED-only pair into the middle of the array, not at either end, so an
            // off-by-one at a boundary cannot be mistaken for correct handling.
            if (i == 2) {
                vm.prank(cb);
                registry.proposePair(PROPOSED_ONLY, address(tokens[0]), address(tokens[1]), amm);
            }
            string memory id = _pairId(i);
            vm.startPrank(cb);
            registry.proposePair(id, address(tokens[i]), address(tokens[i + 1]), amm);
            registry.confirmPair(id);
            vm.stopPrank();
        }
    }

    function test_activePairCount_countsOnlyActive() public view {
        assertEq(registry.activePairCount(), SEEDED, "the PROPOSED pair must not be counted");
    }

    function test_getActivePairsPaged_returnsTheRequestedSlice() public view {
        (PairRegistry.PairEntry[] memory page, uint256 total) = registry.getActivePairsPaged(0, 2);
        assertEq(page.length, 2, "page size honoured");
        assertEq(total, SEEDED, "total reports the whole active set, not the page");

        (PairRegistry.PairEntry[] memory second,) = registry.getActivePairsPaged(2, 2);
        assertEq(second.length, 2, "second page size honoured");
        assertTrue(
            keccak256(bytes(second[0].pairId)) != keccak256(bytes(page[0].pairId)), "offset must advance the window"
        );
    }

    /// A caller must not be able to defeat the bound by asking for a huge page.
    function test_getActivePairsPaged_clampsAnOversizedLimit() public view {
        (PairRegistry.PairEntry[] memory page,) = registry.getActivePairsPaged(0, type(uint256).max);
        assertLe(page.length, registry.MAX_PAGE_SIZE(), "page must never exceed MAX_PAGE_SIZE");
        assertLe(page.length, SEEDED, "and never exceed what exists");
    }

    /// An offset past the end is an empty page, not a revert: paging loops end on a short read.
    function test_getActivePairsPaged_offsetPastEndIsEmpty() public view {
        (PairRegistry.PairEntry[] memory page, uint256 total) = registry.getActivePairsPaged(SEEDED + 10, 5);
        assertEq(page.length, 0, "past the end is empty");
        assertEq(total, SEEDED, "total still reported so a caller can tell it overshot");
    }

    /// A zero limit means "the default page", never "everything".
    function test_getActivePairsPaged_zeroLimitUsesTheDefault() public view {
        (PairRegistry.PairEntry[] memory page,) = registry.getActivePairsPaged(0, 0);
        assertGt(page.length, 0, "zero limit must still return a page");
        assertLe(page.length, registry.MAX_PAGE_SIZE(), "and stay bounded");
    }

    /// A PROPOSED pair must not surface in any page.
    function test_getActivePairsPaged_skipsProposedPairs() public view {
        (PairRegistry.PairEntry[] memory page,) = registry.getActivePairsPaged(0, registry.MAX_PAGE_SIZE());
        for (uint256 i; i < page.length; ++i) {
            assertTrue(
                keccak256(bytes(page[i].pairId)) != keccak256(bytes(PROPOSED_ONLY)),
                "a PROPOSED pair must never be paged"
            );
            assertTrue(page[i].status == PairRegistry.PairStatus.ACTIVE, "every paged entry must be ACTIVE");
        }
    }

    /// Paging must cover the ACTIVE set exactly once, by identity: the offset counts ACTIVE
    /// entries, not array slots, and the PROPOSED pair in the middle is what makes those differ.
    function test_getActivePairsPaged_walksTheActiveSetExactlyOnce() public view {
        string[] memory expected = new string[](SEEDED);
        for (uint256 i; i < SEEDED; ++i) {
            expected[i] = _pairId(i);
        }
        _assertPagedWalkMatches(expected, 2);
    }

    // ─────────────────────────────── helpers ───────────────────────────────

    function _pairId(uint256 i) internal pure returns (string memory) {
        return string.concat("PAIR_", vm.toString(i));
    }

    /// Walks the registry in pages of `pageSize` and asserts the pages together hold exactly
    /// `expected`: every pair present, each exactly once, and nothing extra.
    function _assertPagedWalkMatches(string[] memory expected, uint256 pageSize) internal view {
        uint256 total = registry.activePairCount();
        assertEq(total, expected.length, "activePairCount must match the expected active set");

        // Oversized on purpose: an implementation that over-returns must overflow the expected
        // count and be caught below, not silently write past the end.
        string[] memory seen = new string[](total + pageSize + 1);
        uint256 n;
        for (uint256 off; off < total; off += pageSize) {
            (PairRegistry.PairEntry[] memory page,) = registry.getActivePairsPaged(off, pageSize);
            for (uint256 j; j < page.length && n < seen.length; ++j) {
                seen[n++] = page[j].pairId;
            }
        }
        assertEq(n, total, "paging must yield exactly the active count: no gaps, no extras");

        for (uint256 i; i < expected.length; ++i) {
            uint256 hits;
            for (uint256 j; j < n; ++j) {
                if (keccak256(bytes(seen[j])) == keccak256(bytes(expected[i]))) ++hits;
            }
            assertEq(hits, 1, string.concat("must appear exactly once across the pages: ", expected[i]));
        }
    }
}
