// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

/// @title IAutomatedMarketMaker
/// @dev Interface for the Constant Product AMM with ERC20 LP-share accounting and an asymmetric
///      Circuit Breaker (FR-030 / FR-043 / FR-044 / SC-017 / SC-026). The AMM contract IS the
///      LP-share token (one instance per pair); pool ownership is tracked on-chain via ERC20
///      balances — see specs/013-amm-lp-shares.
interface IAutomatedMarketMaker {
    /// @notice Emitted when liquidity is added and LP shares are minted to the provider.
    event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB, uint256 sharesMinted);

    /// @notice Emitted when LP shares are burned and a single-currency (home) amount is paid out.
    event LogLiquidityRemoved(
        address indexed provider, uint256 sharesBurned, address indexed tokenOut, uint256 amountOut
    );

    /// @notice Emitted when LP shares are burned via the paused emergency path (both sides, no swap).
    event LogEmergencyWithdrawal(
        address indexed provider, uint256 sharesBurned, uint256 amountTokenA, uint256 amountTokenB
    );

    /// @notice Emitted when the swap fee rate is updated by governance.
    event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps);

    /// @notice Emitted when the withdrawal (zap-out) fee rate is updated by governance.
    event LogWithdrawalFeeRateUpdated(uint256 oldWithdrawalFeeBps, uint256 newWithdrawalFeeBps);

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

    /// @dev Custom errors for pricing, pool interactions, and circuit breaker governance.
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
    /// @dev New fee value (swap or withdrawal) exceeds the allowed maximum (1000 = 10%).
    error AMM__FeeBpsTooHigh(uint256 provided, uint256 max);

    // ============================================================================
    //                              LIQUIDITY
    // ============================================================================

    /// @notice Adds balanced liquidity and mints proportional LP shares to the caller.
    /// @dev First deposit mints `sqrt(amountA*amountB) - MINIMUM_LIQUIDITY` and permanently locks
    ///      MINIMUM_LIQUIDITY; subsequent deposits mint `min(amountA*ts/reserveA, amountB*ts/reserveB)`.
    /// @param amountA The amount of token A to add.
    /// @param amountB The amount of token B to add.
    /// @return shares The amount of LP shares minted to the caller.
    function addLiquidity(uint256 amountA, uint256 amountB) external returns (uint256 shares);

    /// @notice Burns LP shares and returns a single (home) currency, zap-swapping the other side.
    /// @param shares The amount of LP shares to burn.
    /// @param tokenOut The token the caller wants to receive in full (TOKEN_A or TOKEN_B).
    /// @param minAmountOut Slippage bound; reverts if the realized output is below this.
    /// @return amountOut The amount of `tokenOut` transferred to the caller.
    function removeLiquidity(uint256 shares, address tokenOut, uint256 minAmountOut)
        external
        returns (uint256 amountOut);

    /// @notice Paused-only exit: burns LP shares and returns the pro-rata of BOTH sides (no swap),
    ///         so providers are never trapped while the circuit breaker is engaged.
    /// @param shares The amount of LP shares to burn.
    /// @return amountA The token A returned. @return amountB The token B returned.
    function removeLiquidityEmergency(uint256 shares) external returns (uint256 amountA, uint256 amountB);

    /// @notice Swaps tokens aiming for an EXACT output amount (Exact-Output pricing).
    function swapTokensForExactTokens(
        address tokenIn,
        address tokenOut,
        uint256 amountOut,
        uint256 maxAmountIn,
        address to
    ) external returns (uint256 amountIn);

    /// @notice Exact-output pricing: required input for a desired output (constant product, pre-fee).
    function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
        external
        pure
        returns (uint256 amountIn);

    /// @notice Exact-input pricing: output for a given input, charging `feeBps_` (used by the zap-out).
    /// @param amountIn The input amount being sold into the pool.
    /// @param reserveIn The reserve of the input token.
    /// @param reserveOut The reserve of the output token.
    /// @param feeBps_ The fee applied, in basis points.
    /// @return amountOut The output amount after fee and price impact.
    function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 feeBps_)
        external
        pure
        returns (uint256 amountOut);

    // ============================================================================
    //                    ASYMMETRIC CIRCUIT BREAKER (FR-043 / FR-044)
    // ============================================================================

    /// @notice Emergency pause invoked by a single Central Bank (1-of-N fail-safe).
    function pause(string calldata reason) external;

    /// @notice Propose a resume action; creates a proposal awaiting 2-of-N signatures.
    function proposeResume() external returns (bytes32 proposalId);

    /// @notice Central Bank signs a resume proposal; auto-executes resume when quorum is reached.
    function signResume(bytes32 proposalId) external;

    /// @notice Returns the current pause state of the AMM.
    function isPaused() external view returns (bool);

    /// @notice Returns the minimum quorum required to resume operations (2-of-N).
    function resumeQuorum() external view returns (uint256);

    /// @notice Returns the number of signatures collected for a resume proposal.
    function resumeSignatures(bytes32 proposalId) external view returns (uint256);

    // ============================================================================
    //                       FEES (governance-configurable)
    // ============================================================================

    /// @notice Returns the current swap fee rate in basis points (default 30 = 0.3%).
    function feeBps() external view returns (uint256);

    /// @notice Returns the current withdrawal (zap-out) fee rate in basis points (default 30 = 0.3%).
    function withdrawalFeeBps() external view returns (uint256);

    /// @notice Updates the swap fee rate. Restricted to governance. Max 1000 (10%).
    function setFeeBps(uint256 newFeeBps) external;

    /// @notice Updates the withdrawal (zap-out) fee rate. Restricted to governance. Max 1000 (10%).
    function setWithdrawalFeeBps(uint256 newWithdrawalFeeBps) external;
}
