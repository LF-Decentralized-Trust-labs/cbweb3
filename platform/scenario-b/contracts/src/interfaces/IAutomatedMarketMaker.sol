// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title IAutomatedMarketMaker
/// @dev Interface for the Constant Product AMM Liquidity Pool with asymmetric Circuit Breaker
///      governance (FR-030 / FR-043 / FR-044 / SC-017 / SC-026).
interface IAutomatedMarketMaker {
    /// @notice Emitted when liquidity is added to the pool.
    event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB);

    /// @notice Emitted when a single-sided liquidity deposit is made (005-cooperative-liquidity).
    event LogSingleSidedLiquidityAdded(address indexed provider, bool isTokenA, uint256 amount);

    /// @notice Emitted when single-sided liquidity is removed (005-cooperative-liquidity).
    event LogSingleSidedLiquidityRemoved(address indexed provider, bool isTokenA, uint256 amount);

    /// @notice Emitted when the fee rate is updated by governance (005-cooperative-liquidity).
    event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps);

    /// @notice Emitted when a swap is successfully executed.
    event LogSwap(
        address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut
    );

    /// @notice Emitted when a Central Bank triggers the Circuit Breaker pause (1-of-N).
    event LogCircuitBreakerPaused(address indexed pausedBy, uint256 timestamp, string reason);

    /// @notice Emitted when a Central Bank proposes to resume operations.
    event LogResumeProposed(bytes32 indexed proposalId, address indexed proposedBy, uint256 timestamp);

    /// @notice Emitted when a Central Bank signs a resume proposal.
    event LogResumeSigned(bytes32 indexed proposalId, address indexed signer, uint256 signaturesCount);

    /// @notice Emitted when a resume proposal reaches quorum (2-of-N) and operations restart.
    event LogCircuitBreakerResumed(bytes32 indexed proposalId, uint256 timestamp);

    /// @dev Custom errors for exact-output pricing, pool interactions, and circuit breaker governance.
    error AMM__ZeroAddress();
    error AMM__ZeroAmount();
    error AMM__InvalidToken();
    error AMM__InsufficientLiquidity();
    error AMM__InsufficientOutputAmount();
    error AMM__SlippageExceeded(uint256 requiredAmountIn, uint256 maxAmountIn);
    error AMM__ParticipantNotVerified(address account);
    error AMM__NotGovernance(address account);
    error AMM__NotPaused();
    error AMM__AlreadyPaused();
    error AMM__ProposalNotFound(bytes32 proposalId);
    error AMM__AlreadySigned(bytes32 proposalId, address signer);
    error AMM__ProposalQuorumIncomplete(bytes32 proposalId, uint256 signatures, uint256 required);
    /// @dev Caller is not a registered Liquidity Provider (005-cooperative-liquidity).
    error AMM__NotLiquidityProvider(address account);
    /// @dev New feeBps value exceeds the allowed maximum (1000 = 10%).
    error AMM__FeeBpsTooHigh(uint256 provided, uint256 max);

    /// @notice Adds initial or subsequent liquidity to the pool.
    /// @param amountA The amount of token A to add.
    /// @param amountB The amount of token B to add.
    function addLiquidity(uint256 amountA, uint256 amountB) external;

    /// @notice Removes liquidity proportionally from the pool.
    /// @param amountA The amount of token A to withdraw.
    /// @param amountB The amount of token B to withdraw.
    function removeLiquidity(uint256 amountA, uint256 amountB) external;

    /// @notice Swaps tokens aiming for an EXACT output amount (Exact-Output pricing).
    /// @dev Implements slippage protection via maxAmountIn.
    /// @param tokenIn The address of the token the user is paying.
    /// @param tokenOut The address of the token the user wants to receive.
    /// @param amountOut The exact amount of tokenOut the user wants.
    /// @param maxAmountIn The maximum amount of tokenIn the user is willing to pay (Slippage protection).
    /// @param to The address that will receive the output tokens.
    /// @return amountIn The calculated amount of tokenIn actually deducted.
    function swapTokensForExactTokens(
        address tokenIn,
        address tokenOut,
        uint256 amountOut,
        uint256 maxAmountIn,
        address to
    ) external returns (uint256 amountIn);

    /// @notice Calculates the required input amount for a desired output amount.
    /// @param reserveIn The current reserve of the input token.
    /// @param reserveOut The current reserve of the output token.
    /// @param amountOut The exact amount of output tokens desired.
    /// @return amountIn The mathematically required amount of input tokens.
    function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
        external
        pure
        returns (uint256 amountIn);

    // ============================================================================
    //                    ASYMMETRIC CIRCUIT BREAKER (FR-043 / FR-044)
    // ============================================================================

    /// @notice Emergency pause invoked by a single Central Bank (1-of-N fail-safe).
    /// @param reason Free-text justification recorded in the event log for audit.
    function pause(string calldata reason) external;

    /// @notice Propose a resume action; creates a proposal awaiting 2-of-N signatures.
    /// @return proposalId Stable identifier for the resume proposal.
    function proposeResume() external returns (bytes32 proposalId);

    /// @notice Central Bank signs a resume proposal; auto-executes resume when quorum is reached.
    /// @param proposalId Identifier returned by proposeResume().
    function signResume(bytes32 proposalId) external;

    /// @notice Returns the current pause state of the AMM.
    function isPaused() external view returns (bool);

    /// @notice Returns the minimum quorum required to resume operations (2-of-N).
    function resumeQuorum() external view returns (uint256);

    /// @notice Returns the number of signatures collected for a resume proposal.
    function resumeSignatures(bytes32 proposalId) external view returns (uint256);

    // =========================================================================
    //             COOPERATIVE LIQUIDITY (005-cooperative-liquidity)
    // =========================================================================

    /// @notice Returns the current fee rate in basis points (default 30 = 0.3%).
    function feeBps() external view returns (uint256);

    /// @notice Deposits a single token side into the pool (cooperative model).
    /// @dev Caller must be a registered Liquidity Provider (isLiquidityProvider = true).
    ///      Funds are only meaningful once both sides are present; the pool status
    ///      transitions EMPTY → PENDING_COUNTERPART → ACTIVE in the backend layer.
    /// @param isTokenA True to deposit TOKEN_A; false to deposit TOKEN_B.
    /// @param amount   The amount to deposit (in the token's base unit).
    function addSingleSidedLiquidity(bool isTokenA, uint256 amount) external;

    /// @notice Removes a single token side from the pool (proportional withdrawal).
    /// @dev Caller must be a registered Liquidity Provider.
    /// @param isTokenA True to withdraw TOKEN_A; false to withdraw TOKEN_B.
    /// @param amount   The amount to withdraw.
    function removeSingleSidedLiquidity(bool isTokenA, uint256 amount) external;

    /// @notice Updates the swap fee rate. Restricted to governance (onlyPauser equivalent).
    /// @param newFeeBps New fee in basis points. Maximum 1000 (10%).
    function setFeeBps(uint256 newFeeBps) external;
}

