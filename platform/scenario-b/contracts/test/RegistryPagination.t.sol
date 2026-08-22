// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {CurrencyRegistry} from "../src/CurrencyRegistry.sol";
import {ICurrencyRegistry} from "../src/interfaces/ICurrencyRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";

/// @title RegistryPaginationTest
/// @notice Bounded reads for CurrencyRegistry (finding R2-M-14).
/// @dev getAllCurrencies() walks the whole array twice and returns every active entry. It is
///      `external view`, so nothing pays gas for it on-chain — the cost is the response size and
///      the RPC round trip as the set grows, which is why this is a scalability bound rather
///      than a denial-of-service fix. These tests pin the paged reader that gives callers a way
///      to ask for a slice, and pin that a caller cannot ask for an unbounded one.
contract RegistryPaginationTest is Test {
    CurrencyRegistry public registry;
    IdentityRegistry public identityRegistry;

    address public admin = makeAddr("admin");
    address public cb = makeAddr("central_bank");

    uint256 constant SEEDED = 5;

    function setUp() public {
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            cb, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0), bytes32("inst-cb")
        );
        identityRegistry.verifyParticipant(cb);
        vm.stopPrank();

        registry = new CurrencyRegistry(address(identityRegistry));

        for (uint256 i; i < SEEDED; ++i) {
            string memory sym = string.concat("tCeBM_", vm.toString(i));
            TokenizedCentralBankMoney tok = new TokenizedCentralBankMoney(sym, sym, admin, cb);
            vm.prank(admin);
            identityRegistry.setCentralBankOf(address(tok), cb);
            vm.prank(cb);
            registry.registerCurrency(sym, string.concat("Country ", vm.toString(i)), address(tok), "cb");
        }
    }

    function test_currencyCount_countsActiveEntries() public view {
        assertEq(registry.currencyCount(), SEEDED, "count must match the seeded set");
    }

    function test_getCurrenciesPaged_returnsTheRequestedSlice() public view {
        (ICurrencyRegistry.CurrencyEntry[] memory page, uint256 total) = registry.getCurrenciesPaged(0, 2);
        assertEq(page.length, 2, "page size honoured");
        assertEq(total, SEEDED, "total reports the whole set, not the page");

        (ICurrencyRegistry.CurrencyEntry[] memory second,) = registry.getCurrenciesPaged(2, 2);
        assertEq(second.length, 2, "second page size honoured");
        assertTrue(
            keccak256(bytes(second[0].symbol)) != keccak256(bytes(page[0].symbol)), "offset must advance the window"
        );
    }

    /// A caller must not be able to defeat the bound by asking for a huge page.
    function test_getCurrenciesPaged_clampsAnOversizedLimit() public view {
        (ICurrencyRegistry.CurrencyEntry[] memory page,) = registry.getCurrenciesPaged(0, type(uint256).max);
        assertLe(page.length, registry.MAX_PAGE_SIZE(), "page must never exceed MAX_PAGE_SIZE");
        assertLe(page.length, SEEDED, "and never exceed what exists");
    }

    /// An offset past the end is an empty page, not a revert: paging loops end on a short read.
    function test_getCurrenciesPaged_offsetPastEndIsEmpty() public view {
        (ICurrencyRegistry.CurrencyEntry[] memory page, uint256 total) = registry.getCurrenciesPaged(SEEDED + 10, 5);
        assertEq(page.length, 0, "past the end is empty");
        assertEq(total, SEEDED, "total still reported so a caller can tell it overshot");
    }

    /// A zero limit means "the default page", never "everything".
    function test_getCurrenciesPaged_zeroLimitUsesTheDefault() public view {
        (ICurrencyRegistry.CurrencyEntry[] memory page,) = registry.getCurrenciesPaged(0, 0);
        assertGt(page.length, 0, "zero limit must still return a page");
        assertLe(page.length, registry.MAX_PAGE_SIZE(), "and stay bounded");
    }

    /// Paging must cover the set exactly once — no gaps, no repeats.
    /// @dev By identity, not by arithmetic. Summing page lengths cannot tell a repeated entry from
    ///      a fresh one, so an implementation that returned the same window for every offset would
    ///      satisfy a count-only check.
    function test_getCurrenciesPaged_walksTheWholeSetExactlyOnce() public {
        _assertPagedWalkMatches(_seededSymbols(), 2);
    }

    /// The offset counts ACTIVE entries, not array slots, and removeCurrency tombstones in place
    /// rather than compacting `_symbols`. Until a hole exists the two indices are identical, so
    /// every other test in this file passes against an implementation that offsets by raw array
    /// position. This is the test that separates them.
    function test_getCurrenciesPaged_walksCorrectlyAcrossATombstone() public {
        string memory removed = "tCeBM_1";
        vm.prank(cb);
        registry.removeCurrency(removed);

        assertEq(registry.currencyCount(), SEEDED - 1, "the removal must drop the active count");

        string[] memory expected = new string[](SEEDED - 1);
        uint256 n;
        for (uint256 i; i < SEEDED; ++i) {
            string memory sym = string.concat("tCeBM_", vm.toString(i));
            if (keccak256(bytes(sym)) == keccak256(bytes(removed))) continue;
            expected[n++] = sym;
        }
        _assertPagedWalkMatches(expected, 2);
    }

    /// A tombstoned entry must not surface in any page.
    function test_getCurrenciesPaged_skipsTombstonedEntries() public {
        vm.prank(cb);
        registry.removeCurrency("tCeBM_0");

        (ICurrencyRegistry.CurrencyEntry[] memory page, uint256 total) =
            registry.getCurrenciesPaged(0, registry.MAX_PAGE_SIZE());
        assertEq(total, SEEDED - 1, "total must exclude the tombstone");
        for (uint256 i; i < page.length; ++i) {
            assertTrue(
                keccak256(bytes(page[i].symbol)) != keccak256(bytes("tCeBM_0")), "a removed entry must not be paged"
            );
        }
    }

    // ─────────────────────────────── helpers ───────────────────────────────

    function _seededSymbols() internal view returns (string[] memory out) {
        out = new string[](SEEDED);
        for (uint256 i; i < SEEDED; ++i) {
            out[i] = string.concat("tCeBM_", vm.toString(i));
        }
    }

    /// Walks the registry in pages of `pageSize` and asserts the pages together hold exactly
    /// `expected`: every symbol present, each exactly once, and nothing extra.
    function _assertPagedWalkMatches(string[] memory expected, uint256 pageSize) internal view {
        uint256 total = registry.currencyCount();
        assertEq(total, expected.length, "currencyCount must match the expected active set");

        // Oversized on purpose: an implementation that over-returns must overflow the expected
        // count and be caught below, not silently write past the end.
        string[] memory seen = new string[](total + pageSize + 1);
        uint256 n;
        for (uint256 off; off < total; off += pageSize) {
            (ICurrencyRegistry.CurrencyEntry[] memory page,) = registry.getCurrenciesPaged(off, pageSize);
            for (uint256 j; j < page.length && n < seen.length; ++j) {
                seen[n++] = page[j].symbol;
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
