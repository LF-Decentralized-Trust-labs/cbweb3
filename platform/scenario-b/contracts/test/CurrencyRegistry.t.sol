// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {CurrencyRegistry} from "../src/CurrencyRegistry.sol";
import {ICurrencyRegistry} from "../src/interfaces/ICurrencyRegistry.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";

/// @title CurrencyRegistryTest
/// @notice Unit tests for CurrencyRegistry.sol (006-hub-currency-registry).
/// @dev Covers registerCurrency, removeCurrency, getCurrency, and getAllCurrencies.
contract CurrencyRegistryTest is Test {
    // Re-declare events locally for vm.expectEmit compatibility (Solidity 0.8.20).
    event CurrencyRegistered(
        string indexed symbol, address indexed tokenAddress, string countryName, string proposerCB
    );
    event CurrencyRemoved(string indexed symbol, address indexed tokenAddress);
    CurrencyRegistry public registry;
    IdentityRegistry public identityRegistry;
    TokenizedCentralBankMoney public tokenBRL;
    TokenizedCentralBankMoney public tokenEUR;

    address public admin = makeAddr("admin");
    address public cbA = makeAddr("central_bank_a");
    address public cbB = makeAddr("central_bank_b");
    address public attacker = makeAddr("attacker");

    function setUp() public {
        // Deploy IdentityRegistry and register both central banks.
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            cbA, "Central Bank A", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0), bytes32("inst-cbA")
        );
        identityRegistry.verifyParticipant(cbA);
        identityRegistry.registerParticipant(
            cbB, "Central Bank B", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0), bytes32("inst-cbB")
        );
        identityRegistry.verifyParticipant(cbB);
        vm.stopPrank();

        // Deploy tCeBM tokens (minter = admin for setup simplicity).
        tokenBRL = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", admin, cbA);
        tokenEUR = new TokenizedCentralBankMoney("Tokenized EUR", "tCeBM_EUR", admin, cbB);

        // Map tokens to their issuing CBs in IdentityRegistry.
        vm.startPrank(admin);
        identityRegistry.setCentralBankOf(address(tokenBRL), cbA);
        identityRegistry.setCentralBankOf(address(tokenEUR), cbB);
        vm.stopPrank();

        // Deploy CurrencyRegistry.
        registry = new CurrencyRegistry(address(identityRegistry));
    }

    // ──────────────────────── registerCurrency ────────────────────────────

    function test_registerCurrency_success() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        ICurrencyRegistry.CurrencyEntry memory entry = registry.getCurrency("BRL");
        assertEq(entry.symbol, "BRL");
        assertEq(entry.countryName, "Brazil");
        assertEq(entry.tokenAddress, address(tokenBRL));
        assertEq(entry.proposerCB, "central_bank_a");
    }

    function test_registerCurrency_emitsCurrencyRegistered() public {
        vm.expectEmit(true, true, false, true);
        emit CurrencyRegistered("BRL", address(tokenBRL), "Brazil", "central_bank_a");

        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");
    }

    function test_registerCurrency_duplicateSymbol_reverts() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(ICurrencyRegistry.CurrencyRegistry__AlreadyExists.selector, "BRL"));
        registry.registerCurrency("BRL", "Brasil", address(tokenBRL), "central_bank_a");
    }

    function test_registerCurrency_duplicateToken_reverts() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        // Same token address, different symbol — must revert.
        vm.prank(cbA);
        vm.expectRevert(
            abi.encodeWithSelector(
                ICurrencyRegistry.CurrencyRegistry__TokenAlreadyRegistered.selector, address(tokenBRL)
            )
        );
        registry.registerCurrency("BRL2", "Brazil2", address(tokenBRL), "central_bank_a");
    }

    function test_registerCurrency_unauthorized_reverts() public {
        vm.prank(attacker);
        vm.expectRevert(ICurrencyRegistry.CurrencyRegistry__Unauthorized.selector);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "attacker");
    }

    function test_registerCurrency_zeroAddress_reverts() public {
        vm.prank(cbA);
        vm.expectRevert(ICurrencyRegistry.CurrencyRegistry__ZeroAddress.selector);
        registry.registerCurrency("BRL", "Brazil", address(0), "central_bank_a");
    }

    function test_registerCurrency_emptySymbol_reverts() public {
        vm.prank(cbA);
        vm.expectRevert(ICurrencyRegistry.CurrencyRegistry__EmptyString.selector);
        registry.registerCurrency("", "Brazil", address(tokenBRL), "central_bank_a");
    }

    // ──────────────────────── getAllCurrencies ────────────────────────────

    function test_getAllCurrencies_returnsRegistered() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(cbB);
        registry.registerCurrency("EUR", "Germany", address(tokenEUR), "central_bank_b");

        ICurrencyRegistry.CurrencyEntry[] memory all = registry.getAllCurrencies();
        assertEq(all.length, 2);
        assertEq(all[0].symbol, "BRL");
        assertEq(all[1].symbol, "EUR");
    }

    function test_getAllCurrencies_empty() public view {
        ICurrencyRegistry.CurrencyEntry[] memory all = registry.getAllCurrencies();
        assertEq(all.length, 0);
    }

    // ──────────────────────── removeCurrency ─────────────────────────────

    function test_removeCurrency_success() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(cbA);
        registry.removeCurrency("BRL");

        vm.expectRevert(abi.encodeWithSelector(ICurrencyRegistry.CurrencyRegistry__NotFound.selector, "BRL"));
        registry.getCurrency("BRL");
    }

    function test_removeCurrency_emitsCurrencyRemoved() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.expectEmit(true, true, false, false);
        emit CurrencyRemoved("BRL", address(tokenBRL));

        vm.prank(cbA);
        registry.removeCurrency("BRL");
    }

    function test_removeCurrency_excludedFromGetAll() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(cbB);
        registry.registerCurrency("EUR", "Germany", address(tokenEUR), "central_bank_b");

        vm.prank(cbA);
        registry.removeCurrency("BRL");

        ICurrencyRegistry.CurrencyEntry[] memory all = registry.getAllCurrencies();
        assertEq(all.length, 1);
        assertEq(all[0].symbol, "EUR");
    }

    /// @notice Re-registering a removed symbol must leave ONE entry, not two.
    /// @dev removeCurrency tombstones the entry (`_symbolExists = false`) and deliberately leaves
    ///      `_symbols` untouched so paging offsets stay stable. Registering the symbol again then
    ///      pushed a second copy of the same string, and every reader that walks `_symbols` and
    ///      filters on `_symbolExists` — currencyCount, getCurrenciesPaged, getAllCurrencies —
    ///      counted the live entry once per copy.
    function test_registerCurrency_afterRemoval_doesNotDuplicateTheSymbol() public {
        vm.startPrank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");
        registry.removeCurrency("BRL");
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");
        vm.stopPrank();

        assertEq(registry.currencyCount(), 1, "a re-registered symbol must be counted once");

        ICurrencyRegistry.CurrencyEntry[] memory all = registry.getAllCurrencies();
        assertEq(all.length, 1, "getAllCurrencies must not list the symbol twice");
        assertEq(all[0].symbol, "BRL");

        (ICurrencyRegistry.CurrencyEntry[] memory page,) = registry.getCurrenciesPaged(0, 10);
        assertEq(page.length, 1, "a page must not list the symbol twice");
    }

    function test_removeCurrency_notFound_reverts() public {
        vm.prank(cbA);
        vm.expectRevert(abi.encodeWithSelector(ICurrencyRegistry.CurrencyRegistry__NotFound.selector, "XYZ"));
        registry.removeCurrency("XYZ");
    }

    function test_removeCurrency_unauthorized_reverts() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(attacker);
        vm.expectRevert(ICurrencyRegistry.CurrencyRegistry__Unauthorized.selector);
        registry.removeCurrency("BRL");
    }

    function test_removeCurrency_symbolFreeAfterRemove() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(cbA);
        registry.removeCurrency("BRL");

        // Same symbol can be re-registered after removal.
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        ICurrencyRegistry.CurrencyEntry memory entry = registry.getCurrency("BRL");
        assertEq(entry.symbol, "BRL");
    }

    function test_removeCurrency_tokenFreeAfterRemove() public {
        vm.prank(cbA);
        registry.registerCurrency("BRL", "Brazil", address(tokenBRL), "central_bank_a");

        vm.prank(cbA);
        registry.removeCurrency("BRL");

        // Same tokenAddress can be re-registered under a different symbol.
        vm.prank(cbA);
        registry.registerCurrency("BRL2", "Brazil v2", address(tokenBRL), "central_bank_a");

        ICurrencyRegistry.CurrencyEntry memory entry = registry.getCurrency("BRL2");
        assertEq(entry.tokenAddress, address(tokenBRL));
    }
}
