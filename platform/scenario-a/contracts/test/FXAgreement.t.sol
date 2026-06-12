// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {FXAgreement} from "../src/FXAgreement.sol";
import {IFXAgreement} from "../src/interfaces/IFXAgreement.sol";
import {FXAgreementLibrary} from "../src/libraries/FXAgreementLibrary.sol";
import {IdentityRegistry} from "../src/IdentityRegistry.sol";
import {IdentityRegistryLibrary} from "../src/libraries/IdentityRegistryLibrary.sol";
import {DeployFXAgreement} from "../script/FXAgreement.s.sol";

/// @title FXAgreementTest
/// @notice Unit tests for the FXAgreement bilateral deal registry contract.
contract FXAgreementTest is Test {
    /// @notice FXAgreement contract under test
    FXAgreement public fxAgreement;

    /// @notice Identity Registry for clearance gate tests
    IdentityRegistry public identityRegistry;

    /// @notice Test accounts
    address public admin = makeAddr("admin");
    address public centralBank = makeAddr("centralBank");
    address public counterpartyA = makeAddr("counterpartyA");
    address public counterpartyB = makeAddr("counterpartyB");
    address public unregistered = makeAddr("unregistered");

    /// @notice Test data
    bytes32 public tradeId = keccak256("FX_DEAL_001");
    address public settlementAgent = makeAddr("settlementAgent");
    address public custodian = makeAddr("custodian");
    address public beneficiary = makeAddr("beneficiary");
    uint256 public originAmount = 1_000_000 * 10 ** 18;
    uint256 public counterAmount = 5_500_000 * 10 ** 18;
    bytes32 public originCurrency = bytes32("BRL");
    bytes32 public counterCurrency = bytes32("EUR");
    uint256 public rate = 5_500 * 10 ** 15; // 5.5 BRL/EUR scaled by 1e18
    uint256 public expiryDate;

    function setUp() public {
        /// @dev 1. Deploy IdentityRegistry and register test participants
        identityRegistry = new IdentityRegistry(admin);
        vm.startPrank(admin);
        identityRegistry.registerParticipant(
            counterpartyA, "Commercial Bank A", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            counterpartyB, "Commercial Bank B", IdentityRegistryLibrary.ParticipantRole.COMMERCIAL_BANK, bytes32(0)
        );
        identityRegistry.registerParticipant(
            centralBank, "Central Bank", IdentityRegistryLibrary.ParticipantRole.CENTRAL_BANK, bytes32(0)
        );
        vm.stopPrank();

        /// @dev 2. Deploy FXAgreement
        fxAgreement = new FXAgreement(address(identityRegistry));

        /// @dev 3. Set expiry to 1 day from now
        expiryDate = block.timestamp + 1 days;
    }

    function test_Propose_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.PROPOSED));
        assertEq(agreement.originator, counterpartyA);
        assertEq(agreement.counterpartyB, counterpartyB);
        assertEq(agreement.settlementAgent, settlementAgent);
        assertEq(agreement.custodian, custodian);
        assertEq(agreement.beneficiary, beneficiary);
        assertEq(agreement.originAmount, originAmount);
        assertEq(agreement.counterAmount, counterAmount);
        assertEq(agreement.originCurrency, originCurrency);
        assertEq(agreement.counterCurrency, counterCurrency);
        assertEq(agreement.rate, rate);
        assertEq(agreement.expiryDate, expiryDate);
    }

    function test_Revert_Propose_DuplicateTradeID() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__TradeAlreadyExists.selector);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);
    }

    function test_Revert_Propose_UnverifiedSender() public {
        vm.prank(unregistered);
        vm.expectRevert(abi.encodeWithSelector(IFXAgreement.FXA__ParticipantNotVerified.selector, unregistered));
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);
    }

    function test_Revert_Propose_ZeroOriginAmount() public {
        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__InvalidParameters.selector);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, 0, counterAmount, originCurrency, counterCurrency, rate, expiryDate);
    }

    function test_Revert_Propose_ZeroCounterAmount() public {
        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__InvalidParameters.selector);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, 0, originCurrency, counterCurrency, rate, expiryDate);
    }

    function test_Revert_Propose_ExpiredDate() public {
        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__AgreementExpired.selector);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, block.timestamp - 1);
    }

    function test_Accept_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyB);
        fxAgreement.accept(tradeId);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.ACCEPTED));
    }

    function test_Revert_Accept_WrongCaller() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.accept(tradeId);
    }

    function test_Revert_Accept_NotProposed() public {
        vm.prank(counterpartyB);
        vm.expectRevert(IFXAgreement.FXA__InvalidStateTransition.selector);
        fxAgreement.accept(tradeId);
    }

    function test_Revert_Accept_Expired() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.warp(expiryDate + 1);

        vm.prank(counterpartyB);
        vm.expectRevert(IFXAgreement.FXA__AgreementExpired.selector);
        fxAgreement.accept(tradeId);
    }

    function test_Reject_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyB);
        fxAgreement.reject(tradeId);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.REJECTED));
    }

    function test_Revert_Reject_WrongCaller() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.reject(tradeId);
    }

    function test_Cancel_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyA);
        fxAgreement.cancel(tradeId);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.CANCELLED));
    }

    function test_Revert_Cancel_WrongCaller() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyB);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.cancel(tradeId);
    }

    function test_Revert_Cancel_NotProposed() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyB);
        fxAgreement.accept(tradeId);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__InvalidStateTransition.selector);
        fxAgreement.cancel(tradeId);
    }

    function test_Settle_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyB);
        fxAgreement.accept(tradeId);

        vm.prank(centralBank);
        fxAgreement.settle(tradeId);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.SETTLED));
    }

    function test_Revert_Settle_NotGovernance() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyB);
        fxAgreement.accept(tradeId);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.settle(tradeId);
    }

    function test_Revert_Settle_NotAccepted() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(centralBank);
        vm.expectRevert(IFXAgreement.FXA__InvalidStateTransition.selector);
        fxAgreement.settle(tradeId);
    }

    function test_ProposeOnBehalf_Success() public {
        vm.prank(centralBank);
        fxAgreement.proposeOnBehalf(tradeId, counterpartyA, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.PROPOSED));
        assertEq(agreement.originator, counterpartyA);
        assertEq(agreement.counterpartyB, counterpartyB);
        assertEq(agreement.settlementAgent, settlementAgent);
        assertEq(agreement.custodian, custodian);
        assertEq(agreement.beneficiary, beneficiary);
        assertEq(agreement.originAmount, originAmount);
        assertEq(agreement.counterAmount, counterAmount);
        assertEq(agreement.originCurrency, originCurrency);
        assertEq(agreement.counterCurrency, counterCurrency);
        assertEq(agreement.rate, rate);
        assertEq(agreement.expiryDate, expiryDate);
    }

    function test_Revert_ProposeOnBehalf_NotGovernance() public {
        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.proposeOnBehalf(tradeId, counterpartyA, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);
    }

    function test_AcceptOnBehalf_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(centralBank);
        fxAgreement.acceptOnBehalf(tradeId);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.ACCEPTED));
    }

    function test_Revert_AcceptOnBehalf_NotGovernance() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.acceptOnBehalf(tradeId);
    }

    function test_Revert_AcceptOnBehalf_NotProposed() public {
        vm.prank(centralBank);
        vm.expectRevert(IFXAgreement.FXA__InvalidStateTransition.selector);
        fxAgreement.acceptOnBehalf(tradeId);
    }

    function test_Revert_AcceptOnBehalf_Expired() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.warp(expiryDate + 1);

        vm.prank(centralBank);
        vm.expectRevert(IFXAgreement.FXA__AgreementExpired.selector);
        fxAgreement.acceptOnBehalf(tradeId);
    }

    function test_RejectOnBehalf_Success() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(centralBank);
        fxAgreement.rejectOnBehalf(tradeId);

        FXAgreementLibrary.FxAgreement memory agreement = fxAgreement.getAgreement(tradeId);
        assertEq(uint256(agreement.state), uint256(FXAgreementLibrary.AgreementState.REJECTED));
    }

    function test_Revert_RejectOnBehalf_NotGovernance() public {
        vm.prank(counterpartyA);
        fxAgreement.propose(tradeId, counterpartyB, settlementAgent, custodian, beneficiary, originAmount, counterAmount, originCurrency, counterCurrency, rate, expiryDate);

        vm.prank(counterpartyA);
        vm.expectRevert(IFXAgreement.FXA__Unauthorized.selector);
        fxAgreement.rejectOnBehalf(tradeId);
    }

    function test_Revert_GetAgreement_NotFound() public {
        vm.expectRevert(IFXAgreement.FXA__TradeNotFound.selector);
        fxAgreement.getAgreement(keccak256("NONEXISTENT"));
    }

    function test_Revert_Constructor_ZeroRegistry() public {
        vm.expectRevert(IFXAgreement.FXA__InvalidParameters.selector);
        new FXAgreement(address(0));
    }

    function test_ScriptRun_Success() public {
        DeployFXAgreement deployScript = new DeployFXAgreement();
        deployScript.setUp();

        vm.setEnv("DEPLOYER_PRIVATE_KEY", vm.toString(uint256(0x1)));
        vm.setEnv("IDENTITY_REGISTRY_ADDRESS", vm.toString(address(identityRegistry)));

        deployScript.run();

        assertTrue(address(deployScript.fxAgreement()) != address(0), "FXAgreement was not deployed");
        assertGt(address(deployScript.fxAgreement()).code.length, 0, "FXAgreement has no bytecode");
    }
}
