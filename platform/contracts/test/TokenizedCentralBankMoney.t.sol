// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {DeployTCeBM} from "../script/TokenizedCentralBankMoney.s.sol";
import {IAccessControl} from "@openzeppelin-contracts/access/IAccessControl.sol";

contract TokenizedCentralBankMoneyTest is Test {
    /// @notice The tokenized central bank money contract
    TokenizedCentralBankMoney public tCeBm;

    /// @notice Test accounts
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public userA = makeAddr("userA");

    /// @notice Roles used in the contract
    bytes32 public constant DEFAULT_ADMIN_ROLE = 0x0;
    bytes32 public constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    function setUp() public {
        tCeBm = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", admin, centralBank);
    }

    /// @dev Test the initial state of the contract
    function test_InitialState() public view {
        assertEq(tCeBm.name(), "Tokenized BRL");
        assertEq(tCeBm.symbol(), "tCeBM_BRL");
        assertTrue(tCeBm.hasRole(DEFAULT_ADMIN_ROLE, admin));
        assertTrue(tCeBm.hasRole(CENTRAL_BANK_ROLE, centralBank));
        assertEq(tCeBm.totalSupply(), 0);
    }

    /// @dev Test that minting tokens succeeds when called by the central bank
    function test_Mint_Success() public {
        uint256 mintAmount = 1000 * 10 ** 18;

        /// @dev Prank simulates the next call coming from the centralBank address
        vm.prank(centralBank);
        tCeBm.mint(userA, mintAmount);

        assertEq(tCeBm.balanceOf(userA), mintAmount);
        assertEq(tCeBm.totalSupply(), mintAmount);
    }

    /// @dev Test that minting tokens reverts when called by an unauthorized account
    function test_Mint_RevertUnauthorized() public {
        uint256 mintAmount = 1000 * 10 ** 18;

        /// @dev Expect the OpenZeppelin v5 custom error
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, userA, CENTRAL_BANK_ROLE)
        );

        /// @dev Prank as userA (who does not have the CB role)
        vm.prank(userA);
        tCeBm.mint(userA, mintAmount);
    }

    /// @dev Test that burning tokens succeeds when called by the central bank
    function test_Burn_Success() public {
        uint256 amount = 500 * 10 ** 18;

        /// @dev Setup: Mint tokens first
        vm.prank(centralBank);
        tCeBm.mint(userA, amount);
        assertEq(tCeBm.balanceOf(userA), amount);

        /// @dev Execute: Burn tokens directly from userA
        vm.prank(centralBank);
        tCeBm.burn(userA, amount);

        assertEq(tCeBm.balanceOf(userA), 0);
        assertEq(tCeBm.totalSupply(), 0);
    }

    /// @dev Test that burning tokens reverts when called by an unauthorized account
    function test_Burn_RevertUnauthorized() public {
        uint256 amount = 500 * 10 ** 18;

        /// @dev Setup: Mint tokens first
        vm.prank(centralBank);
        tCeBm.mint(userA, amount);

        /// @dev Expect the OpenZeppelin v5 custom error
        vm.expectRevert(
            abi.encodeWithSelector(IAccessControl.AccessControlUnauthorizedAccount.selector, userA, CENTRAL_BANK_ROLE)
        );

        /// @dev Execute: Attempt to burn using an unauthorized account
        vm.prank(userA);
        tCeBm.burn(userA, amount);
    }
}

contract DeployTCeBMTest is Test {
    /// @notice Test deployment and RBAC configuration of TokenizedCentralBankMoney
    DeployTCeBM public deployScript;

    /// @dev Token details for assertions
    string private constant TOKEN_NAME = "Tokenized BRL";
    string private constant TOKEN_SYMBOL = "tCeBM_BRL";

    /// @dev Role identifiers for RBAC
    bytes32 private constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 private constant CENTRAL_BANK_ROLE = keccak256("CENTRAL_BANK_ROLE");

    /// @dev Deployment and role configuration details
    uint256 private deployerPrivateKey;
    address private expectedDeployer;
    address private expectedAdmin;
    address private expectedCentralBank;

    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";
    string private constant ENV_ADMIN_ADDRESS = "ADMIN_ADDRESS";
    string private constant ENV_CENTRAL_BANK_ADDRESS = "CENTRAL_BANK_ADDRESS";

    function setUp() public {
        deployScript = new DeployTCeBM();
        deployScript.setUp();

        /// @dev Read script inputs directly from .env / environment
        deployerPrivateKey = vm.envUint(ENV_DEPLOYER_PRIVATE_KEY);
        expectedAdmin = vm.envAddress(ENV_ADMIN_ADDRESS);
        expectedCentralBank = vm.envAddress(ENV_CENTRAL_BANK_ADDRESS);
        expectedDeployer = vm.addr(deployerPrivateKey);
    }

    /**
     * @dev The script should successfully deploy the contract and configure RBAC using env vars.
     */
    function test_ScriptRun_Success() public {
        /// @dev Act
        deployScript.run();
        TokenizedCentralBankMoney tCeBm = deployScript.tCeBm();

        /// @dev Assert: contract deployment
        assertTrue(address(tCeBm) != address(0), "Contract was not deployed");

        /// @dev Assert: constructor state
        assertEq(tCeBm.name(), TOKEN_NAME, "Incorrect token name");
        assertEq(tCeBm.symbol(), TOKEN_SYMBOL, "Incorrect token symbol");

        /// @dev Assert: RBAC seeded from env values
        assertTrue(tCeBm.hasRole(DEFAULT_ADMIN_ROLE, expectedAdmin), "Admin role not granted to expected address");
        assertTrue(
            tCeBm.hasRole(CENTRAL_BANK_ROLE, expectedCentralBank), "Central Bank role not granted to expected address"
        );

        /// @dev Assert: deployer does not receive privileged roles by default
        assertFalse(tCeBm.hasRole(DEFAULT_ADMIN_ROLE, expectedDeployer), "Deployer should not be admin");
        assertFalse(tCeBm.hasRole(CENTRAL_BANK_ROLE, expectedDeployer), "Deployer should not be central bank");
    }
}
