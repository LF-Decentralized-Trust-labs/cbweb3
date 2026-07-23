// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IAutomatedMarketMaker} from "./interfaces/IAutomatedMarketMaker.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";
import {Pausable} from "@openzeppelin-contracts/utils/Pausable.sol";

/// @title Automated Market Maker (AMM)
/// @dev Constant Product Liquidity Pool for Scenario A (Exact-Output pricing).
///      All identity and role checks are delegated to the IdentityRegistry (single source of truth).
///      Governance is protected by an asymmetric Circuit Breaker (pause = 1-of-N, resume = 2-of-N;
///      FR-043 / FR-044) — a single Central Bank may halt trading, but restarting requires a quorum.
///      Swaps charge a governance-configurable fee (default 0.3%) that accrues to the pool reserves.
///      This mirrors the Scenario B AMM's breaker and fee model; it does NOT create a runtime
///      dependency across scenarios (the port is source-local to scenario-a/contracts).
contract AutomatedMarketMaker is IAutomatedMarketMaker, ReentrancyGuard, Pausable {
    using SafeERC20 for IERC20;

    /// @notice The ERC20 tokens in the liquidity pool
    IERC20 public immutable TOKEN_A;
    IERC20 public immutable TOKEN_B;

    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @notice Minimum quorum to resume operations after a pause (2-of-N Central Banks).
    uint256 public constant RESUME_QUORUM = 2;

    /// @notice Maximum allowed swap fee in basis points (10%).
    uint256 public constant MAX_FEE_BPS = 1000;

    /// @notice Default swap fee in basis points applied on construction (30 = 0.3%).
    uint256 public constant DEFAULT_FEE_BPS = 30;

    /// @notice The current reserves of the pool to compute the constant product (x * y = k)
    uint256 public reserveA;
    uint256 public reserveB;

    /// @notice Swap fee in basis points. Governance-controlled; defaults to 0.3%. The fee stays in
    ///         the reserves (accrues to liquidity providers).
    uint256 public feeBps;

    /// @notice A resume proposal awaiting the 2-of-N quorum.
    struct ResumeProposal {
        address proposer;
        uint256 createdAt;
        uint256 signatures;
        mapping(address => bool) signed;
        bool executed;
    }

    /// @notice Resume proposals keyed by a hash derived from proposer + block.
    mapping(bytes32 => ResumeProposal) private _resumeProposals;

    /// @notice Initializes the AMM with the token pair and IdentityRegistry.
    /// @param _tokenA Address of the first token (e.g., tCeBM_BRL).
    /// @param _tokenB Address of the second token (e.g., tCeBM_EUR).
    /// @param _identityRegistry Address of the IdentityRegistry (single source of truth for roles).
    constructor(address _tokenA, address _tokenB, address _identityRegistry) {
        if (_tokenA == address(0) || _tokenB == address(0) || _identityRegistry == address(0)) {
            revert AMM__ZeroAddress();
        }

        TOKEN_A = IERC20(_tokenA);
        TOKEN_B = IERC20(_tokenB);
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
        feeBps = DEFAULT_FEE_BPS;
    }

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    /// @param account The address to verify.
    modifier onlyVerified(address account) {
        _onlyVerified(account);
        _;
    }

    /// @dev Internal check extracted from the modifier to reduce bytecode duplication at call sites.
    function _onlyVerified(address account) internal view {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert AMM__ParticipantNotVerified(account);
        }
    }

    /// @notice Restricts access to accounts with a governance-capable role in the IdentityRegistry.
    modifier onlyGovernance() {
        _onlyGovernance();
        _;
    }

    /// @dev Internal governance check — delegates to the IdentityRegistry (single source of truth).
    function _onlyGovernance() internal view {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert AMM__NotGovernance(msg.sender);
        }
    }

    // ============================================================================
    //                    ASYMMETRIC CIRCUIT BREAKER (FR-043 / FR-044)
    // ============================================================================

    /// @notice Emergency pause (1-of-N fail-safe). Any governance-capable participant may halt trading.
    /// @dev Symmetric single-key unpause is intentionally removed; resuming requires a 2-of-N quorum
    ///      via {proposeResume}/{signResume}. Reverts (OZ EnforcedPause) if already paused.
    /// @param reason Free-text justification recorded on-chain for auditability.
    function pause(string calldata reason) external onlyGovernance whenNotPaused {
        _pause();
        emit LogCircuitBreakerPaused(msg.sender, block.timestamp, reason);
    }

    /// @notice Create a resume proposal. The proposer's signature counts as the first signature.
    /// @dev A single proposer is never enough to resume: quorum is {RESUME_QUORUM} (2-of-N).
    /// @return proposalId The identifier of the created resume proposal.
    function proposeResume() external onlyGovernance whenPaused returns (bytes32 proposalId) {
        proposalId = keccak256(abi.encode(msg.sender, block.number, block.timestamp));
        ResumeProposal storage proposal = _resumeProposals[proposalId];
        proposal.proposer = msg.sender;
        proposal.createdAt = block.timestamp;
        proposal.signatures = 1;
        proposal.signed[msg.sender] = true;
        emit LogResumeProposed(proposalId, msg.sender, block.timestamp);
        emit LogResumeSigned(proposalId, msg.sender, 1);
    }

    /// @notice Sign a resume proposal. When signatures reach {RESUME_QUORUM}, the AMM auto-resumes.
    /// @dev A given signer may only sign once (no self-quorum). Reverts if the proposal is unknown.
    function signResume(bytes32 proposalId) external onlyGovernance whenPaused {
        ResumeProposal storage proposal = _resumeProposals[proposalId];
        if (proposal.createdAt == 0) {
            revert AMM__ProposalNotFound(proposalId);
        }
        if (proposal.signed[msg.sender]) {
            revert AMM__AlreadySigned(proposalId, msg.sender);
        }
        proposal.signed[msg.sender] = true;
        proposal.signatures += 1;
        emit LogResumeSigned(proposalId, msg.sender, proposal.signatures);

        if (proposal.signatures >= RESUME_QUORUM && !proposal.executed) {
            proposal.executed = true;
            _unpause();
            emit LogCircuitBreakerResumed(proposalId, block.timestamp);
        }
    }

    /// @inheritdoc IAutomatedMarketMaker
    function isPaused() external view returns (bool) {
        return paused();
    }

    /// @inheritdoc IAutomatedMarketMaker
    function resumeQuorum() external pure returns (uint256) {
        return RESUME_QUORUM;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function resumeSignatures(bytes32 proposalId) external view returns (uint256) {
        return _resumeProposals[proposalId].signatures;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function addLiquidity(uint256 amountA, uint256 amountB)
        external
        nonReentrant
        whenNotPaused
        onlyVerified(msg.sender)
    {
        if (amountA == 0 || amountB == 0) {
            revert AMM__ZeroAmount();
        }

        // [INTERACTIONS] - Pull tokens from liquidity provider
        TOKEN_A.safeTransferFrom(msg.sender, address(this), amountA);
        TOKEN_B.safeTransferFrom(msg.sender, address(this), amountB);

        // [EFFECTS] - Update internal reserves
        reserveA += amountA;
        reserveB += amountB;

        emit LogLiquidityAdded(msg.sender, amountA, amountB);
    }

    /// @inheritdoc IAutomatedMarketMaker
    function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
        public
        pure
        returns (uint256 amountIn)
    {
        if (amountOut >= reserveOut) {
            revert AMM__InsufficientLiquidity();
        }

        // Constant Product Formula for Exact Output: dx = (x * dy) / (y - dy)
        // We add +1 to counteract integer truncation, guaranteeing the invariant (k) never decreases.
        uint256 numerator = reserveIn * amountOut;
        uint256 denominator = reserveOut - amountOut;

        amountIn = (numerator / denominator) + 1;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function swapTokensForExactTokens(
        address tokenIn,
        address tokenOut,
        uint256 amountOut,
        uint256 maxAmountIn,
        address to
    ) external nonReentrant whenNotPaused onlyVerified(msg.sender) onlyVerified(to) returns (uint256 amountIn) {
        // [CHECKS]
        if (amountOut == 0) revert AMM__ZeroAmount();
        if (tokenIn == tokenOut) revert AMM__InvalidToken();

        bool isAIn = tokenIn == address(TOKEN_A) && tokenOut == address(TOKEN_B);
        bool isBIn = tokenIn == address(TOKEN_B) && tokenOut == address(TOKEN_A);

        if (!isAIn && !isBIn) revert AMM__InvalidToken();

        uint256 reserveIn = isAIn ? reserveA : reserveB;
        uint256 reserveOut = isAIn ? reserveB : reserveA;

        // Pre-fee constant-product input for the exact output requested.
        uint256 baseAmountIn = getAmountIn(reserveIn, reserveOut, amountOut);

        // Fee-in-reserve: the user pays a grossed-up input; the 0.3% fee stays in the reserves and
        // accrues to liquidity providers. grossIn = base / (1 - fee); +1 rounds in favour of the pool.
        amountIn = (baseAmountIn * 10000) / (10000 - feeBps) + 1;

        // Slippage Protection (checked against the fee-inclusive amount actually charged).
        if (amountIn > maxAmountIn) {
            revert AMM__SlippageExceeded(amountIn, maxAmountIn);
        }

        // [EFFECTS] - Optimistically update state; the full gross input (fee included) enters reserves.
        if (isAIn) {
            reserveA += amountIn;
            reserveB -= amountOut;
        } else {
            reserveB += amountIn;
            reserveA -= amountOut;
        }

        // [INTERACTIONS] - Transfer assets
        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenOut).safeTransfer(to, amountOut);

        emit LogSwap(msg.sender, tokenIn, tokenOut, amountIn, amountOut);
    }

    // ============================================================================
    //                       FEE MODEL (governance-configurable)
    // ============================================================================

    /// @inheritdoc IAutomatedMarketMaker
    function setFeeBps(uint256 newFeeBps) external onlyGovernance {
        if (newFeeBps > MAX_FEE_BPS) revert AMM__FeeBpsTooHigh(newFeeBps, MAX_FEE_BPS);
        uint256 old = feeBps;
        feeBps = newFeeBps;
        emit LogFeeRateUpdated(old, newFeeBps);
    }
}
