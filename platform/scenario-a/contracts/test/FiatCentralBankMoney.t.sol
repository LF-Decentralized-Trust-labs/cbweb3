// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.20;

import {Test} from "forge-std/Test.sol";
import {FiatCentralBankMoney} from "../src/FiatCentralBankMoney.sol";

import {DeployFiatCBM} from "../script/FiatCentralBankMoney.s.sol";
import {IAccessControl} from "@openzeppelin-contracts/access/IAccessControl.sol";

/// @title FiatCentralBankMoneyTest
/// @notice Unit tests for the FiatCentralBankMoney (fCeBM) ERC-20 contract.
contract FiatCentralBankMoneyTest is Test {
    /// @notice The fiat central bank money contract under test.
    FiatCentralBankMoney public fiat;

    /// @notice Test accounts.
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public commercialBank = makeAddr("commercialBank");
    address public unauthorized = makeAddr("unauthorized");

    /// @notice Roles used in the contract.
    bytes32 public constant DEFAULT_ADMIN_ROLE = 0x0;
    bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    /// @dev Mirror events from the contract for expectEmit assertions.
    event FiatMinted(address indexed to, uint256 amount);
    event FiatBurned(address indexed from, uint256 amount);

    function setUp() public {
        fiat = new FiatCentralBankMoney("Fiat BRL", "fCeBM_BRL", admin, centralBank);
    }

    // ─── Initial State ───────────────────────────────────────────────────

    /// @dev Test the initial state of the contract after deployment.
    function test_InitialState() public view {
        assertEq(fiat.name(), "Fiat BRL");
        assertEq(fiat.symbol(), "fCeBM_BRL");
        assertTrue(fiat.hasRole(DEFAULT_ADMIN_ROLE, admin));
        assertTrue(fiat.hasRole(CENTRAL_BANK_ROLE, centralBank));
        assertEq(fiat.totalSupply(), 0);
    }

    // ─── Mint ────────────────────────────────────────────────────────────

    /// @dev Test that minting tokens succeeds when called by the Central Bank.
    function test_Mint_Success() public {
        uint256 mintAmount = 1000 * 10 ** 18;

        vm.prank(centralBank);
        fiat.mint(commercialBank, mintAmount);

        assertEq(fiat.balanceOf(commercialBank), mintAmount);
        assertEq(fiat.totalSupply(), mintAmount);
    }

    /// @dev Test that minting emits the FiatMinted event.
    function test_Mint_EmitsEvent() public {
        uint256 mintAmount = 500 * 10 ** 18;

        vm.expectEmit(true, false, false, true);
        emit FiatMinted(commercialBank, mintAmount);

        vm.prank(centralBank);
        fiat.mint(commercialBank, mintAmount);
    }

    /// @dev Test that minting reverts when called by an unauthorized account.
    function test_Mint_RevertUnauthorized() public {
        uint256 mintAmount = 1000 * 10 ** 18;

        vm.expectRevert(
            abi.encodeWithSelector(
                IAccessControl.AccessControlUnauthorizedAccount.selector, unauthorized, CENTRAL_BANK_ROLE
            )
        );

        vm.prank(unauthorized);
        fiat.mint(commercialBank, mintAmount);
    }

    // ─── Burn ────────────────────────────────────────────────────────────

    /// @dev Test that burning tokens succeeds when called by the Central Bank.
    function test_Burn_Success() public {
        uint256 amount = 500 * 10 ** 18;

        /// @dev Setup: Mint tokens to the commercial bank first.
        vm.prank(centralBank);
        fiat.mint(commercialBank, amount);
        assertEq(fiat.balanceOf(commercialBank), amount);

        /// @dev Execute: Burn tokens from the commercial bank.
        vm.prank(centralBank);
        fiat.burn(commercialBank, amount);

        assertEq(fiat.balanceOf(commercialBank), 0);
        assertEq(fiat.totalSupply(), 0);
    }

    /// @dev Test that burning emits the FiatBurned event.
    function test_Burn_EmitsEvent() public {
        uint256 amount = 250 * 10 ** 18;

        vm.prank(centralBank);
        fiat.mint(commercialBank, amount);

        vm.expectEmit(true, false, false, true);
        emit FiatBurned(commercialBank, amount);

        vm.prank(centralBank);
        fiat.burn(commercialBank, amount);
    }

    /// @dev Test that burning reverts when called by an unauthorized account.
    function test_Burn_RevertUnauthorized() public {
        uint256 amount = 500 * 10 ** 18;

        /// @dev Setup: Mint tokens first.
        vm.prank(centralBank);
        fiat.mint(commercialBank, amount);

        vm.expectRevert(
            abi.encodeWithSelector(
                IAccessControl.AccessControlUnauthorizedAccount.selector, unauthorized, CENTRAL_BANK_ROLE
            )
        );

        /// @dev Execute: Attempt to burn using an unauthorized account.
        vm.prank(unauthorized);
        fiat.burn(commercialBank, amount);
    }

    /// @dev Test that the Central Bank can burn tokens from any account (sovereign authority).
    function test_Burn_FromAnyAddress() public {
        uint256 amount = 1000 * 10 ** 18;
        address recipient = makeAddr("arbitraryHolder");

        /// @dev Setup: Mint to an arbitrary address.
        vm.prank(centralBank);
        fiat.mint(recipient, amount);

        /// @dev Execute: CB burns directly from that address without an allowance.
        vm.prank(centralBank);
        fiat.burn(recipient, amount);

        assertEq(fiat.balanceOf(recipient), 0);
    }

    // ─── Fuzz ────────────────────────────────────────────────────────────

    /// @dev Fuzz test: mint an arbitrary amount and verify balance consistency.
    function testFuzz_MintAndBurn(uint256 amount) public {
        /// @dev Bound amount to avoid overflow — max 10^30 tokens (reasonable upper limit).
        amount = bound(amount, 1, 10 ** 30);

        vm.prank(centralBank);
        fiat.mint(commercialBank, amount);
        assertEq(fiat.balanceOf(commercialBank), amount);

        vm.prank(centralBank);
        fiat.burn(commercialBank, amount);
        assertEq(fiat.balanceOf(commercialBank), 0);
        assertEq(fiat.totalSupply(), 0);
    }
}

/// @title DeployFiatCBMTest
/// @notice Integration test for the FiatCentralBankMoney deployment script.
contract DeployFiatCBMTest is Test {
    DeployFiatCBM public deployScript;

    string private constant TOKEN_NAME = "Fiat BRL";
    string private constant TOKEN_SYMBOL = "fCeBM_BRL";

    bytes32 private constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 private constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    uint256 private deployerPrivateKey;
    address private expectedDeployer;
    address private expectedAdmin;
    address private expectedCentralBank;

    function setUp() public {
        deployScript = new DeployFiatCBM();

        deployerPrivateKey = 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80;
        expectedDeployer = vm.addr(deployerPrivateKey);
        expectedAdmin = makeAddr("admin");
        expectedCentralBank = makeAddr("centralBank");

        vm.setEnv("DEPLOYER_PRIVATE_KEY", vm.toString(deployerPrivateKey));
        vm.setEnv("ADMIN_ADDRESS", vm.toString(expectedAdmin));
        vm.setEnv("CENTRAL_BANK_ADDRESS", vm.toString(expectedCentralBank));
        vm.setEnv("FIAT_TOKEN_NAME", TOKEN_NAME);
        vm.setEnv("FIAT_TOKEN_SYMBOL", TOKEN_SYMBOL);
    }

    /// @dev Verify the deployment script configures name, symbol, and roles correctly.
    function test_DeployScript_ConfiguresCorrectly() public {
        deployScript.run();

        FiatCentralBankMoney fiat = deployScript.fiatToken();

        assertEq(fiat.name(), TOKEN_NAME);
        assertEq(fiat.symbol(), TOKEN_SYMBOL);
        assertTrue(fiat.hasRole(DEFAULT_ADMIN_ROLE, expectedAdmin));
        assertTrue(fiat.hasRole(CENTRAL_BANK_ROLE, expectedCentralBank));
        assertEq(fiat.totalSupply(), 0);
    }
}
