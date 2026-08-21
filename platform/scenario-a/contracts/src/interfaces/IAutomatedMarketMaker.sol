// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

/// @title IAutomatedMarketMaker
/// @dev Interface for the Constant Product AMM Liquidity Pool.
interface IAutomatedMarketMaker {
    /// @notice Emitted when liquidity is added to the pool.
    event LogLiquidityAdded(address indexed provider, uint256 amountTokenA, uint256 amountTokenB);

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

    /// @notice Emitted when the swap fee rate is updated by governance.
    event LogFeeRateUpdated(uint256 oldFeeBps, uint256 newFeeBps);

    /// @dev Custom errors for exact-output pricing and pool interactions.
    error AMM__ZeroAddress();
    error AMM__ZeroAmount();
    error AMM__InvalidToken();
    error AMM__InsufficientLiquidity();
    error AMM__InsufficientOutputAmount();
    error AMM__SlippageExceeded(uint256 requiredAmountIn, uint256 maxAmountIn);
    error AMM__ParticipantNotVerified(address account);
    error AMM__NotGovernance(address account);
    /// @dev Resume proposal id has never been created (or was mistyped).
    error AMM__ProposalNotFound(bytes32 proposalId);
    /// @dev The signer has already signed this resume proposal (no self-quorum).
    error AMM__AlreadySigned(bytes32 proposalId, address signer);
    /// @dev Another wallet of the same institution has already signed this resume proposal. The quorum
    ///      counts distinct institutions, so a single institution cannot reach it with two keys.
    error AMM__InstitutionAlreadySigned(bytes32 proposalId, bytes32 institutionId);
    /// @dev The signer carries no institution identifier in the IdentityRegistry, so its signature cannot
    ///      be attributed to an institution and cannot be counted towards the quorum.
    error AMM__InvalidInstitutionId(address account);
    /// @dev The resume proposal was raised against a stale pause epoch; a new pause has since occurred,
    ///      so signatures collected against the old pause can no longer form quorum (R2-H-2 epoch binding).
    error AMM__ProposalExpired(bytes32 proposalId, uint256 proposalEpoch, uint256 currentEpoch);
    /// @dev New fee value exceeds the allowed maximum (1000 = 10%).
    error AMM__FeeBpsTooHigh(uint256 provided, uint256 max);

    /// @notice Adds initial or subsequent liquidity to the pool.
    /// @param amountA The amount of token A to add.
    /// @param amountB The amount of token B to add.
    function addLiquidity(uint256 amountA, uint256 amountB) external;

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

    /// @notice Fee-inclusive exact-output quote in a SINGLE ceiling division (R2-H-3 parity).
    /// @dev The input a caller sizes `maxAmountIn` against IS the exact charge — swap and quote share
    ///      this one ceiling division, so the caller is never over-charged by a second rounding step.
    /// @param reserveIn The current reserve of the input token.
    /// @param reserveOut The current reserve of the output token.
    /// @param amountOut The exact amount of output tokens desired.
    /// @param feeBps_ The swap fee in basis points to fold into the quote.
    /// @return amountIn The fee-inclusive input amount (the fee stays in the reserves).
    function quoteExactOutput(uint256 reserveIn, uint256 reserveOut, uint256 amountOut, uint256 feeBps_)
        external
        pure
        returns (uint256 amountIn);

    // ============================================================================
    //                    ASYMMETRIC CIRCUIT BREAKER (FR-043 / FR-044)
    // ============================================================================

    /// @notice Emergency pause invoked by a single Central Bank (1-of-N fail-safe).
    /// @param reason Free-text justification recorded on-chain for auditability.
    function pause(string calldata reason) external;

    /// @notice Propose a resume action; creates a proposal awaiting 2-of-N signatures.
    /// @return proposalId The identifier of the created resume proposal (proposer counts as 1 signature).
    function proposeResume() external returns (bytes32 proposalId);

    /// @notice Central Bank signs a resume proposal; auto-executes resume when quorum is reached.
    function signResume(bytes32 proposalId) external;

    /// @notice Returns the current pause state of the AMM.
    function isPaused() external view returns (bool);

    /// @notice Returns the minimum quorum required to resume operations (2-of-N).
    function resumeQuorum() external view returns (uint256);

    /// @notice Returns the number of signatures collected for a resume proposal.
    function resumeSignatures(bytes32 proposalId) external view returns (uint256);

    /// @notice Monotonic counter incremented on every {pause}. A resume proposal is bound to the epoch
    ///         in which it was raised; signatures cannot carry across a later pause (R2-H-2 epoch binding).
    function pauseEpoch() external view returns (uint256);

    // ============================================================================
    //                       FEE MODEL (governance-configurable)
    // ============================================================================

    /// @notice Returns the current swap fee rate in basis points (default 30 = 0.3%).
    function feeBps() external view returns (uint256);

    /// @notice Updates the swap fee rate. Restricted to governance. Max 1000 (10%).
    function setFeeBps(uint256 newFeeBps) external;
}
