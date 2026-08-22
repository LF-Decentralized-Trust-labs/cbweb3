// SPDX-License-Identifier: Apache-2.0
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
    /// @notice A resume proposal raised under an earlier pause is being signed after a later pause.
    error AMM__ProposalExpired(bytes32 proposalId, uint256 proposalEpoch, uint256 currentEpoch);
    /// @notice A second wallet of an institution that already signed this proposal tried to sign.
    error AMM__InstitutionAlreadySigned(bytes32 proposalId, bytes32 institutionId);
    /// @notice The signer carries no institution id in the registry, so its vote cannot be attributed.
    error AMM__InvalidInstitutionId(address account);
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

    // ============================================================================
    //         ESCROW-AND-FINALIZE PAIRED DEPOSIT (decision D6, Phase 2)
    // ============================================================================
    // Each provider deposits ONLY its own side, independently, against a shared commit id
    // (matching the sovereign reality where two central banks deposit from two gateways/keys).
    // Tokens are escrowed — not yet in reserves — until both sides arrive, at which point
    // finalizeCommit moves them into reserves and mints proportional shares to each recipient.

    /// @notice Emitted when one side of a paired deposit is escrowed against a commit.
    event LogCommitDeposit(
        bytes32 indexed commitId, address indexed depositor, address indexed recipient, bool isTokenA, uint256 amount
    );

    /// @notice Emitted when both sides are present and the commit is finalized into reserves + shares.
    event LogCommitFinalized(
        bytes32 indexed commitId, address recipientA, address recipientB, uint256 sharesA, uint256 sharesB
    );

    /// @notice Emitted when an un-finalized escrowed side is reclaimed by its depositor (timeout/refund).
    event LogCommitRefunded(bytes32 indexed commitId, address indexed depositor, bool isTokenA, uint256 amount);

    error AMM__CommitAlreadyFinalized(bytes32 commitId);
    error AMM__CommitIncomplete(bytes32 commitId);
    error AMM__SideAlreadyDeposited(bytes32 commitId, bool isTokenA);
    error AMM__NotDepositor(bytes32 commitId);
    error AMM__NothingToRefund(bytes32 commitId);

    /// @notice Escrows the caller's single side against `commitId`, crediting LP shares (on finalize)
    ///         to `shareRecipient`. Tokens are pulled from `msg.sender`; shares accrue to the recipient.
    /// @param commitId The off-chain matched-commit identifier the two sides share.
    /// @param isTokenA True if depositing TOKEN_A, false for TOKEN_B.
    /// @param amount The amount of the chosen side to escrow.
    /// @param shareRecipient The verified participant that will receive this side's LP shares.
    function depositForCommit(bytes32 commitId, bool isTokenA, uint256 amount, address shareRecipient) external;

    /// @notice Finalizes a fully-deposited commit: moves both escrowed sides into reserves and mints
    ///         proportional LP shares to each recorded recipient (split by contributed value).
    /// @param commitId The commit to finalize (both sides must be present).
    /// @return sharesA Shares minted to the token-A recipient. @return sharesB Shares minted to B.
    function finalizeCommit(bytes32 commitId) external returns (uint256 sharesA, uint256 sharesB);

    /// @notice Reclaims an un-finalized escrowed side (timeout/refund). Only that side's depositor may call.
    /// @param commitId The commit to refund from.
    /// @param isTokenA Which side to reclaim.
    /// @return amount The amount refunded to the depositor.
    function cancelCommitDeposit(bytes32 commitId, bool isTokenA) external returns (uint256 amount);

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

    /// @notice Fee-aware exact-output quote: the EXACT input `swapTokensForExactTokens` will charge
    ///         for `amountOut`, as a SINGLE ceiling division (constant-product input and fee gross-up
    ///         folded, rounded up once in favor of the pool). Off-chain callers size `maxAmountIn`
    ///         from this value to avoid AMM__SlippageExceeded. This is the shared source of truth the
    ///         swap itself uses (R2-H-3).
    /// @param reserveIn The reserve of the input token.
    /// @param reserveOut The reserve of the output token.
    /// @param amountOut The desired exact output amount.
    /// @param feeBps_ The swap fee applied, in basis points.
    /// @return amountIn The fee-inclusive input required.
    function quoteExactOutput(uint256 reserveIn, uint256 reserveOut, uint256 amountOut, uint256 feeBps_)
        external
        pure
        returns (uint256 amountIn);

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
