// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

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
            cb, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
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
            keccak256(bytes(second[0].symbol)) != keccak256(bytes(page[0].symbol)),
            "offset must advance the window"
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
    function test_getCurrenciesPaged_walksTheWholeSetExactlyOnce() public view {
        uint256 seen;
        for (uint256 off; off < SEEDED; off += 2) {
            (ICurrencyRegistry.CurrencyEntry[] memory page,) = registry.getCurrenciesPaged(off, 2);
            seen += page.length;
        }
        assertEq(seen, SEEDED, "walking in pages of 2 must visit every entry once");
    }
}
