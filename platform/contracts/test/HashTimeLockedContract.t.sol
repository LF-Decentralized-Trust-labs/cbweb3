// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {HashTimeLockedContract} from "../src/HashTimeLockedContract.sol";
import {HashTimeLockedContractLibrary} from "../src/libraries/HashTimeLockedContractLibrary.sol";
import {TokenizedCentralBankMoney} from "../src/TokenizedCentralBankMoney.sol";
import {DeployHTLC} from "../script/HashTimeLockedContract.s.sol";

contract HashTimeLockedContractTest is Test {
    /// @notice HTLC contract under test
    HashTimeLockedContract public htlc;

    /// @notice Mock token used as escrowed asset in tests
    TokenizedCentralBankMoney public tCeBm;

    /// @notice Test accounts
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public sender = makeAddr("sender");
    address public receiver = makeAddr("receiver");

    /// @notice HTLC test variables
    bytes32 public contractId = keccak256("FX_AGREEMENT_001");
    uint256 public lockAmount = 100 * 10 ** 18;
    bytes32 public secret = "super_secure_secret_preimage";
    bytes32 public hashLock = sha256(abi.encodePacked(secret));
    uint256 public timeLock;

    /// @notice Deploys and configures test fixtures for HTLC flows.
    /// @dev Mints tokens to sender and sets allowance for the escrow contract.
    function setUp() public {
        /// @dev 1. Deploy the tCeBm mock asset
        tCeBm = new TokenizedCentralBankMoney("Tokenized BRL", "tCeBM_BRL", admin, centralBank);

        /// @dev 2. Deploy the HTLC Escrow
        htlc = new HashTimeLockedContract();

        /// @dev 3. Mint tokens to the sender and approve the HTLC to spend them
        vm.startPrank(centralBank);
        tCeBm.mint(sender, lockAmount);
        vm.stopPrank();

        vm.prank(sender);
        tCeBm.approve(address(htlc), lockAmount);

        /// @dev Set the timeLock to 1 hour from the current block timestamp
        timeLock = block.timestamp + 1 hours;
    }

    /// @dev Test that `lock` succeeds and funds are escrowed with state set to `LOCKED`.
    function test_Lock_Success() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        /// @dev Verify state and balances
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.LOCKED));
        assertEq(tCeBm.balanceOf(address(htlc)), lockAmount);
        assertEq(tCeBm.balanceOf(sender), 0);
    }

    /// @dev Test that `settle` succeeds with the correct secret preimage.
    function test_Settle_Success() public {
        /// @dev Setup: lock the funds
        vm.prank(sender);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);

        /// @dev Act: receiver (or anyone) settles with the valid secret
        htlc.settle(contractId, secret);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        /// @dev Verify state and balances
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.SETTLED));
        assertEq(details.secret, secret);
        assertEq(tCeBm.balanceOf(address(htlc)), 0);
        assertEq(tCeBm.balanceOf(receiver), lockAmount);
    }

    /// @dev Test that `refund` succeeds only after `timeLock` expiry.
    function test_Refund_Success() public {
        /// @dev Setup: lock the funds
        vm.prank(sender);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);

        /// @dev Act: fast-forward chain time beyond `timeLock`
        vm.warp(timeLock + 1 seconds);

        /// @dev Sender executes the refund
        vm.prank(sender);
        htlc.refund(contractId);

        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(contractId);

        /// @dev Verify state and balances
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.REFUNDED));
        assertEq(tCeBm.balanceOf(address(htlc)), 0);
        assertEq(tCeBm.balanceOf(sender), lockAmount);
    }

    /// @dev Test that `lock` reverts when attempting to reuse an existing `contractId`.
    function test_Revert_Lock_AlreadyExists() public {
        vm.startPrank(sender);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);

        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__ContractAlreadyExists.selector);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);
        vm.stopPrank();
    }

    /// @dev Test that `lock` reverts when `amount` is zero.
    function test_Revert_Lock_InvalidAmount() public {
        bytes32 newContractId = keccak256("FX_AGREEMENT_INVALID_AMOUNT");

        vm.prank(sender);
        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__InvalidAmount.selector);
        htlc.lock(newContractId, receiver, address(tCeBm), 0, hashLock, timeLock);
    }

    /// @dev Test that `lock` reverts when `timeLock` is already expired.
    function test_Revert_Lock_TimeLockExpired() public {
        bytes32 newContractId = keccak256("FX_AGREEMENT_EXPIRED_TIMELOCK");
        uint256 expiredTimeLock = block.timestamp;

        vm.prank(sender);
        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__TimeLockExpired.selector);
        htlc.lock(newContractId, receiver, address(tCeBm), lockAmount, hashLock, expiredTimeLock);
    }

    /// @dev Test that `settle` reverts when the provided secret does not match `hashLock`.
    function test_Revert_Settle_InvalidSecret() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);

        bytes32 wrongSecret = "wrong_secret";

        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__InvalidSecret.selector);
        htlc.settle(contractId, wrongSecret);
    }

    /// @dev Test that `settle` reverts when the contract is not in `LOCKED` state.
    function test_Revert_Settle_ContractNotLocked() public {
        bytes32 nonExistingContractId = keccak256("FX_AGREEMENT_NON_EXISTING_SETTLE");

        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__ContractNotLocked.selector);
        htlc.settle(nonExistingContractId, secret);
    }

    /// @dev Test that `refund` reverts before `timeLock` expiry.
    function test_Revert_Refund_TimeLockNotExpired() public {
        vm.prank(sender);
        htlc.lock(contractId, receiver, address(tCeBm), lockAmount, hashLock, timeLock);

        /// @dev Attempt to refund immediately (before `vm.warp`)
        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__TimeLockNotExpired.selector);
        htlc.refund(contractId);
    }

    /// @dev Test that `refund` reverts when the contract is not in `LOCKED` state.
    function test_Revert_Refund_ContractNotLocked() public {
        bytes32 nonExistingContractId = keccak256("FX_AGREEMENT_NON_EXISTING_REFUND");

        vm.expectRevert(HashTimeLockedContractLibrary.HTLC__ContractNotLocked.selector);
        htlc.refund(nonExistingContractId);
    }
}

contract DeployHTLCTest is Test {
    /// @notice Test deployment of HashTimeLockedContract via script
    DeployHTLC public deployScript;

    /// @dev Deployment configuration
    uint256 private deployerPrivateKey;
    address private expectedDeployer;

    string private constant ENV_DEPLOYER_PRIVATE_KEY = "DEPLOYER_PRIVATE_KEY";

    function setUp() public {
        deployScript = new DeployHTLC();
        deployScript.setUp();

        /// @dev Read script input from .env or use default test value
        deployerPrivateKey = vm.envOr(ENV_DEPLOYER_PRIVATE_KEY, uint256(0x1));
        expectedDeployer = vm.addr(deployerPrivateKey);

        /// @dev Set environment variable for the script if not already set
        vm.setEnv(ENV_DEPLOYER_PRIVATE_KEY, vm.toString(deployerPrivateKey));
    }

    ///
    /// @dev The script should successfully deploy the HTLC contract using env vars.
    ///
    function test_ScriptRun_Success() public {
        /// @dev Act
        deployScript.run();
        HashTimeLockedContract htlc = deployScript.htlc();

        /// @dev Assert: contract deployment
        assertTrue(address(htlc) != address(0), "Contract was not deployed");
        assertGt(address(htlc).code.length, 0, "Deployed contract has no runtime bytecode");

        /// @dev Assert: default lock details for non-existing contractId
        bytes32 unknownContractId = keccak256("UNKNOWN_CONTRACT_ID");
        HashTimeLockedContractLibrary.LockDetails memory details = htlc.getLockDetails(unknownContractId);

        assertEq(details.sender, address(0), "Default sender should be zero address");
        assertEq(details.receiver, address(0), "Default receiver should be zero address");
        assertEq(details.token, address(0), "Default token should be zero address");
        assertEq(details.amount, 0, "Default amount should be zero");
        assertEq(details.hashLock, bytes32(0), "Default hashLock should be zero");
        assertEq(details.timeLock, 0, "Default timeLock should be zero");
        assertEq(details.secret, bytes32(0), "Default secret should be zero");
        assertEq(uint256(details.state), uint256(HashTimeLockedContractLibrary.HTLCState.INVALID));

        /// @dev Assert: ensure env-derived deployer address is valid
        assertTrue(expectedDeployer != address(0), "Expected deployer should not be zero address");
    }
}
