// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

import {IAutomatedMarketMaker} from "./interfaces/IAutomatedMarketMaker.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";

/// @title Automated Market Maker (AMM)
/// @dev Constant Product Liquidity Pool for Scenario B (Exact-Output pricing) with an
///      asymmetric Circuit Breaker (pause = 1-of-N, resume = 2-of-N) per FR-043 / FR-044.
///      All identity and role checks are delegated to the IdentityRegistry (single source of truth).
contract AutomatedMarketMaker is IAutomatedMarketMaker, ReentrancyGuard {
    using SafeERC20 for IERC20;

    /// @notice The ERC20 tokens in the liquidity pool
    IERC20 public immutable TOKEN_A;
    IERC20 public immutable TOKEN_B;

    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @notice Minimum quorum to resume operations after a pause (2-of-N Central Banks).
    uint256 public constant RESUME_QUORUM = 2;

    /// @notice The current reserves of the pool to compute the constant product (x * y = k)
    uint256 public reserveA;
    uint256 public reserveB;

    /// @notice Current paused state of the AMM (asymmetric circuit breaker).
    bool private _paused;

    /// @notice Swap fee in basis points. Default 30 (0.3%). Governance-controlled (FR-005).
    uint256 public feeBps;

    /// @notice Maximum allowed feeBps to prevent griefing (10%).
    uint256 public constant MAX_FEE_BPS = 1000;

    /// @notice Resume proposals awaiting quorum. Key is a hash derived from proposer + block number.
    struct ResumeProposal {
        address proposer;
        uint256 createdAt;
        uint256 signatures;
        mapping(address => bool) signed;
        bool executed;
    }

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
        feeBps = 30;
    }

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    modifier onlyVerified(address account) {
        _onlyVerified(account);
        _;
    }

    function _onlyVerified(address account) internal view {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert AMM__ParticipantNotVerified(account);
        }
    }

    /// @notice Restricts access to accounts with the Liquidity Provider role (005-cooperative-liquidity).
    /// @dev Delegates to IdentityRegistry.isLiquidityProvider — single source of truth.
    modifier onlyLiquidityProvider(address account) {
        if (!IDENTITY_REGISTRY.isLiquidityProvider(account)) {
            revert AMM__NotLiquidityProvider(account);
        }
        _;
    }

    /// @notice Restricts access to accounts with a governance-capable role in the IdentityRegistry.
    modifier onlyGovernance() {
        _onlyGovernance();
        _;
    }

    function _onlyGovernance() internal view {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert AMM__NotGovernance(msg.sender);
        }
    }

    /// @notice Reverts if the AMM is currently paused.
    modifier whenNotPaused() {
        if (_paused) revert AMM__AlreadyPaused();
        _;
    }

    /// @notice Reverts unless the AMM is currently paused.
    modifier whenPaused() {
        if (!_paused) revert AMM__NotPaused();
        _;
    }

    // ============================================================================
    //                    ASYMMETRIC CIRCUIT BREAKER (FR-043 / FR-044)
    // ============================================================================

    /// @notice Emergency pause (1-of-N fail-safe). Any Central Bank with governance rights may pause.
    function pause(string calldata reason) external onlyGovernance whenNotPaused {
        _paused = true;
        emit LogCircuitBreakerPaused(msg.sender, block.timestamp, reason);
    }

    /// @notice Create a resume proposal. The proposer's signature counts as the first signature.
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

    /// @notice Sign a resume proposal. When the signatures count reaches RESUME_QUORUM (2),
    ///         the AMM is auto-resumed atomically in the same transaction.
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
            _paused = false;
            emit LogCircuitBreakerResumed(proposalId, block.timestamp);
        }
    }

    /// @inheritdoc IAutomatedMarketMaker
    function isPaused() external view returns (bool) {
        return _paused;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function resumeQuorum() external pure returns (uint256) {
        return RESUME_QUORUM;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function resumeSignatures(bytes32 proposalId) external view returns (uint256) {
        return _resumeProposals[proposalId].signatures;
    }

    // ============================================================================
    //                         LIQUIDITY AND SWAP
    // ============================================================================

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

        TOKEN_A.safeTransferFrom(msg.sender, address(this), amountA);
        TOKEN_B.safeTransferFrom(msg.sender, address(this), amountB);

        reserveA += amountA;
        reserveB += amountB;

        emit LogLiquidityAdded(msg.sender, amountA, amountB);
    }

    /// @inheritdoc IAutomatedMarketMaker
    /// @notice Deposits a single token side into the pool (cooperative liquidity model).
    /// @dev Caller must hold the Liquidity Provider role in the IdentityRegistry.
    ///      The backend commit-reveal mechanism ensures both sides are matched before
    ///      this is called; this function performs only the on-chain transfer.
    function addSingleSidedLiquidity(bool isTokenA, uint256 amount)
        external
        nonReentrant
        whenNotPaused
        onlyLiquidityProvider(msg.sender)
    {
        if (amount == 0) revert AMM__ZeroAmount();

        if (isTokenA) {
            TOKEN_A.safeTransferFrom(msg.sender, address(this), amount);
            reserveA += amount;
        } else {
            TOKEN_B.safeTransferFrom(msg.sender, address(this), amount);
            reserveB += amount;
        }

        emit LogSingleSidedLiquidityAdded(msg.sender, isTokenA, amount);
    }

    /// @inheritdoc IAutomatedMarketMaker
    /// @notice Removes a single token side proportionally (cooperative withdrawal).
    /// @dev The backend calculates the correct amount from shares_percentage; this
    ///      function only performs the transfer and reserve update.
    function removeSingleSidedLiquidity(bool isTokenA, uint256 amount)
        external
        nonReentrant
        whenNotPaused
        onlyLiquidityProvider(msg.sender)
    {
        if (amount == 0) revert AMM__ZeroAmount();

        if (isTokenA) {
            if (amount > reserveA) revert AMM__InsufficientLiquidity();
            reserveA -= amount;
            TOKEN_A.safeTransfer(msg.sender, amount);
        } else {
            if (amount > reserveB) revert AMM__InsufficientLiquidity();
            reserveB -= amount;
            TOKEN_B.safeTransfer(msg.sender, amount);
        }

        emit LogSingleSidedLiquidityRemoved(msg.sender, isTokenA, amount);
    }

    /// @inheritdoc IAutomatedMarketMaker
    /// @notice Updates the swap fee rate. Restricted to governance roles.
    function setFeeBps(uint256 newFeeBps) external onlyGovernance {
        if (newFeeBps > MAX_FEE_BPS) revert AMM__FeeBpsTooHigh(newFeeBps, MAX_FEE_BPS);
        uint256 old = feeBps;
        feeBps = newFeeBps;
        emit LogFeeRateUpdated(old, newFeeBps);
    }

    /// @inheritdoc IAutomatedMarketMaker
    function removeLiquidity(uint256 amountA, uint256 amountB)
        external
        nonReentrant
        whenNotPaused
        onlyVerified(msg.sender)
    {
        if (amountA == 0 && amountB == 0) {
            revert AMM__ZeroAmount();
        }
        if (amountA > reserveA || amountB > reserveB) {
            revert AMM__InsufficientLiquidity();
        }

        reserveA -= amountA;
        reserveB -= amountB;

        if (amountA > 0) TOKEN_A.safeTransfer(msg.sender, amountA);
        if (amountB > 0) TOKEN_B.safeTransfer(msg.sender, amountB);
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
        // Adding +1 counteracts integer truncation so the invariant (k) never decreases.
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
        if (amountOut == 0) revert AMM__ZeroAmount();
        if (tokenIn == tokenOut) revert AMM__InvalidToken();

        bool isAIn = tokenIn == address(TOKEN_A) && tokenOut == address(TOKEN_B);
        bool isBIn = tokenIn == address(TOKEN_B) && tokenOut == address(TOKEN_A);

        if (!isAIn && !isBIn) revert AMM__InvalidToken();

        uint256 reserveIn = isAIn ? reserveA : reserveB;
        uint256 reserveOut = isAIn ? reserveB : reserveA;

        // Calculate amountIn needed for the desired output (before fee).
        amountIn = getAmountIn(reserveIn, reserveOut, amountOut);

        // Apply fee-in-reserve: user pays grossAmountIn; fee stays in reserves.
        // grossAmountIn = amountIn / (1 - feeBps/10000), ceiling-rounded.
        uint256 grossAmountIn = (amountIn * 10000) / (10000 - feeBps) + 1;

        if (grossAmountIn > maxAmountIn) {
            revert AMM__SlippageExceeded(grossAmountIn, maxAmountIn);
        }

        if (isAIn) {
            reserveA += grossAmountIn;
            reserveB -= amountOut;
        } else {
            reserveB += grossAmountIn;
            reserveA -= amountOut;
        }

        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), grossAmountIn);
        IERC20(tokenOut).safeTransfer(to, amountOut);

        emit LogSwap(msg.sender, tokenIn, tokenOut, grossAmountIn, amountOut);
        return grossAmountIn;
    }
}
