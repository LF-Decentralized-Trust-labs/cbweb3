// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../src/IdentityRegistry.sol";

contract IdentityRegistryLPTest is Test {
    IdentityRegistry registry;
    address admin;
    address lp1;
    address lp2;
    address nonAdmin;

    event LogLiquidityProviderGranted(address indexed account);
    event LogLiquidityProviderRevoked(address indexed account);

    function setUp() public {
        admin = makeAddr("admin");
        lp1 = makeAddr("lp1");
        lp2 = makeAddr("lp2");
        nonAdmin = makeAddr("nonAdmin");

        vm.startPrank(admin);
        registry = new IdentityRegistry(admin);
        vm.stopPrank();
    }

    // --- grantLiquidityProvider ---

    function test_grantLP_emitsEvent() public {
        vm.expectEmit(true, false, false, false);
        emit LogLiquidityProviderGranted(lp1);

        vm.prank(admin);
        registry.grantLiquidityProvider(lp1);
    }

    function test_grantLP_setsFlag() public {
        assertFalse(registry.isLiquidityProvider(lp1));

        vm.prank(admin);
        registry.grantLiquidityProvider(lp1);

        assertTrue(registry.isLiquidityProvider(lp1));
    }

    function test_grantLP_idempotent() public {
        vm.startPrank(admin);
        registry.grantLiquidityProvider(lp1);
        registry.grantLiquidityProvider(lp1); // should not revert
        vm.stopPrank();
        assertTrue(registry.isLiquidityProvider(lp1));
    }

    function test_nonAdmin_cannotGrant() public {
        vm.prank(nonAdmin);
        vm.expectRevert();
        registry.grantLiquidityProvider(lp1);
    }

    // --- revokeLiquidityProvider ---

    function test_revokeLP_emitsEvent() public {
        vm.prank(admin);
        registry.grantLiquidityProvider(lp1);

        vm.expectEmit(true, false, false, false);
        emit LogLiquidityProviderRevoked(lp1);

        vm.prank(admin);
        registry.revokeLiquidityProvider(lp1);
    }

    function test_revokeLP_clearsFlag() public {
        vm.startPrank(admin);
        registry.grantLiquidityProvider(lp1);
        registry.revokeLiquidityProvider(lp1);
        vm.stopPrank();
        assertFalse(registry.isLiquidityProvider(lp1));
    }

    function test_nonAdmin_cannotRevoke() public {
        vm.prank(admin);
        registry.grantLiquidityProvider(lp1);

        vm.prank(nonAdmin);
        vm.expectRevert();
        registry.revokeLiquidityProvider(lp1);
    }

    // --- isLiquidityProvider ---

    function test_isLP_returnsFalseForUnknown() public view {
        assertFalse(registry.isLiquidityProvider(address(0xdead)));
    }
}
