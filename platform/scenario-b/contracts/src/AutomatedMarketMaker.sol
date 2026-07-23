// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

import {IAutomatedMarketMaker} from "./interfaces/IAutomatedMarketMaker.sol";
import {IIdentityRegistry} from "./interfaces/IIdentityRegistry.sol";
import {IERC20} from "@openzeppelin-contracts/token/ERC20/IERC20.sol";
import {ERC20} from "@openzeppelin-contracts/token/ERC20/ERC20.sol";
import {SafeERC20} from "@openzeppelin-contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin-contracts/utils/ReentrancyGuard.sol";
import {Math} from "@openzeppelin-contracts/utils/math/Math.sol";

/// @title Automated Market Maker (AMM) with on-chain LP-share accounting
/// @dev Constant-product pool for Scenario B. The contract IS its own ERC20 LP-share token
///      (one instance per pair, à la Uniswap V2): pool ownership is recorded on-chain as share
///      balances, so withdrawal is gated by share ownership (you can only burn what you hold) —
///      this closes the prior pool-drain vector (TASK-12). Empty-pool swaps are rejected and the
///      first deposit locks MINIMUM_LIQUIDITY (TASK-13). Withdrawal returns a single "home"
///      currency by zap-swapping the other side (decision D1, specs/013-amm-lp-shares).
///      Includes the asymmetric Circuit Breaker (pause = 1-of-N, resume = 2-of-N; FR-043/FR-044).
///      LP-share transfers are restricted to verified participants (decision D2).
contract AutomatedMarketMaker is IAutomatedMarketMaker, ERC20, ReentrancyGuard {
    using SafeERC20 for IERC20;

    /// @notice The ERC20 tokens in the liquidity pool.
    IERC20 public immutable TOKEN_A;
    IERC20 public immutable TOKEN_B;

    /// @notice The Identity Registry used for participant clearance gates.
    IIdentityRegistry public immutable IDENTITY_REGISTRY;

    /// @notice Minimum quorum to resume operations after a pause (2-of-N Central Banks).
    uint256 public constant RESUME_QUORUM = 2;

    /// @notice Liquidity permanently locked on the first deposit to harden against the
    ///         first-depositor share-inflation / donation attack (Uniswap V2 convention).
    uint256 public constant MINIMUM_LIQUIDITY = 1000;

    /// @notice Sink address holding the locked MINIMUM_LIQUIDITY shares (never recoverable).
    address public constant BURN_ADDRESS = address(0xdEaD);

    /// @notice Maximum allowed fee (swap or withdrawal) in basis points (10%).
    uint256 public constant MAX_FEE_BPS = 1000;

    /// @notice Current reserves used to compute the constant product (x * y = k).
    uint256 public reserveA;
    uint256 public reserveB;

    /// @notice Current paused state of the AMM (asymmetric circuit breaker).
    bool private _paused;

    /// @notice Swap fee in basis points. Default 0 (no fee). Governance-controlled (FR-005).
    uint256 public feeBps;

    /// @notice Withdrawal (zap-out) fee in basis points. Default 0 (no fee). Governance-controlled.
    uint256 public withdrawalFeeBps;

    /// @notice Resume proposals awaiting quorum. Key is a hash derived from proposer + block.
    struct ResumeProposal {
        address proposer;
        uint256 createdAt;
        uint256 signatures;
        mapping(address => bool) signed;
        bool executed;
    }

    mapping(bytes32 => ResumeProposal) private _resumeProposals;

    /// @notice One side of a paired deposit escrowed against an off-chain commit id (decision D6).
    struct Escrow {
        address depositorA; // funded side A (entitled to refund before finalize)
        address depositorB;
        address recipientA; // receives side-A LP shares on finalize
        address recipientB;
        uint256 amountA;
        uint256 amountB;
        bool finalized;
    }

    /// @notice Escrowed paired deposits, keyed by the matched-commit id.
    mapping(bytes32 => Escrow) private _escrows;

    /// @notice Initializes the AMM/LP-share token with the pair and IdentityRegistry.
    constructor(address _tokenA, address _tokenB, address _identityRegistry) ERC20("CBWeb3 Hub AMM LP", "CBW3-LP") {
        if (_tokenA == address(0) || _tokenB == address(0) || _identityRegistry == address(0)) {
            revert AMM__ZeroAddress();
        }

        TOKEN_A = IERC20(_tokenA);
        TOKEN_B = IERC20(_tokenB);
        IDENTITY_REGISTRY = IIdentityRegistry(_identityRegistry);
        feeBps = 0;
        withdrawalFeeBps = 0;
    }

    // ============================================================================
    //                              MODIFIERS
    // ============================================================================

    /// @notice Ensures the given account is a verified participant in the IdentityRegistry.
    modifier onlyVerified(address account) {
        if (!IDENTITY_REGISTRY.canTransact(account)) {
            revert AMM__ParticipantNotVerified(account);
        }
        _;
    }

    /// @notice Restricts access to accounts with a governance-capable role in the IdentityRegistry.
    modifier onlyGovernance() {
        if (!IDENTITY_REGISTRY.canGovern(msg.sender)) {
            revert AMM__NotGovernance(msg.sender);
        }
        _;
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
    //                    LP-SHARE TOKEN — RESTRICTED TRANSFERS (D2)
    // ============================================================================

    /// @dev Mint (`from == 0`) and burn (`to == 0`) are always allowed. Peer-to-peer LP-share
    ///      transfers require both ends to be verified participants (decision D2) — pool ownership
    ///      cannot leak to non-participants.
    function _update(address from, address to, uint256 value) internal override {
        if (from != address(0) && to != address(0)) {
            if (!IDENTITY_REGISTRY.canTransact(from)) revert AMM__ParticipantNotVerified(from);
            if (!IDENTITY_REGISTRY.canTransact(to)) revert AMM__ParticipantNotVerified(to);
        }
        super._update(from, to, value);
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

    /// @notice Sign a resume proposal. When signatures reach RESUME_QUORUM (2), the AMM auto-resumes.
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
    //                              LIQUIDITY
    // ============================================================================

    /// @inheritdoc IAutomatedMarketMaker
    function addLiquidity(uint256 amountA, uint256 amountB)
        external
        nonReentrant
        whenNotPaused
        onlyVerified(msg.sender)
        returns (uint256 shares)
    {
        if (amountA == 0 || amountB == 0) {
            revert AMM__ZeroAmount();
        }

        TOKEN_A.safeTransferFrom(msg.sender, address(this), amountA);
        TOKEN_B.safeTransferFrom(msg.sender, address(this), amountB);

        shares = _liquidityShares(amountA, amountB);

        reserveA += amountA;
        reserveB += amountB;

        _mint(msg.sender, shares);
        emit LogLiquidityAdded(msg.sender, amountA, amountB, shares);
    }

    /// @dev Computes LP shares for a balanced (amountA, amountB) deposit against the *current*
    ///      reserves and mints the MINIMUM_LIQUIDITY lock on the very first deposit (TASK-13).
    ///      MUST be called before `reserveA`/`reserveB` are increased by this deposit.
    function _liquidityShares(uint256 amountA, uint256 amountB) internal returns (uint256 shares) {
        uint256 supply = totalSupply();
        if (supply == 0) {
            uint256 initial = Math.sqrt(amountA * amountB);
            if (initial <= MINIMUM_LIQUIDITY) revert AMM__InsufficientLiquidity();
            shares = initial - MINIMUM_LIQUIDITY;
            _mint(BURN_ADDRESS, MINIMUM_LIQUIDITY);
        } else {
            // Proportional mint; the min() penalizes ratio-inconsistent deposits (TASK-12/13).
            uint256 shareA = (amountA * supply) / reserveA;
            uint256 shareB = (amountB * supply) / reserveB;
            shares = shareA < shareB ? shareA : shareB;
            if (shares == 0) revert AMM__InsufficientLiquidity();
        }
    }

    // ============================================================================
    //         ESCROW-AND-FINALIZE PAIRED DEPOSIT (decision D6, Phase 2)
    // ============================================================================

    /// @inheritdoc IAutomatedMarketMaker
    function depositForCommit(bytes32 commitId, bool isTokenA, uint256 amount, address shareRecipient)
        external
        nonReentrant
        whenNotPaused
        onlyVerified(msg.sender)
    {
        if (amount == 0) revert AMM__ZeroAmount();
        if (!IDENTITY_REGISTRY.canTransact(shareRecipient)) revert AMM__ParticipantNotVerified(shareRecipient);

        Escrow storage e = _escrows[commitId];
        if (e.finalized) revert AMM__CommitAlreadyFinalized(commitId);

        if (isTokenA) {
            if (e.depositorA != address(0)) revert AMM__SideAlreadyDeposited(commitId, true);
            TOKEN_A.safeTransferFrom(msg.sender, address(this), amount);
            e.depositorA = msg.sender;
            e.recipientA = shareRecipient;
            e.amountA = amount;
        } else {
            if (e.depositorB != address(0)) revert AMM__SideAlreadyDeposited(commitId, false);
            TOKEN_B.safeTransferFrom(msg.sender, address(this), amount);
            e.depositorB = msg.sender;
            e.recipientB = shareRecipient;
            e.amountB = amount;
        }

        emit LogCommitDeposit(commitId, msg.sender, shareRecipient, isTokenA, amount);
    }

    /// @inheritdoc IAutomatedMarketMaker
    function finalizeCommit(bytes32 commitId)
        external
        nonReentrant
        whenNotPaused
        returns (uint256 sharesA, uint256 sharesB)
    {
        Escrow storage e = _escrows[commitId];
        if (e.finalized) revert AMM__CommitAlreadyFinalized(commitId);
        if (e.depositorA == address(0) || e.depositorB == address(0)) revert AMM__CommitIncomplete(commitId);

        uint256 amountA = e.amountA;
        uint256 amountB = e.amountB;
        address recipientA = e.recipientA;
        address recipientB = e.recipientB;
        e.finalized = true;

        // Value of each side in token-A terms (pre-update reserves). First deposit → 50/50 split.
        uint256 valueA;
        uint256 valueB;
        if (reserveA == 0 || reserveB == 0) {
            valueA = 1;
            valueB = 1;
        } else {
            valueA = amountA;
            valueB = (amountB * reserveA) / reserveB;
        }

        uint256 shares = _liquidityShares(amountA, amountB);

        reserveA += amountA;
        reserveB += amountB;

        sharesA = (shares * valueA) / (valueA + valueB);
        sharesB = shares - sharesA;

        if (sharesA > 0) _mint(recipientA, sharesA);
        if (sharesB > 0) _mint(recipientB, sharesB);

        emit LogCommitFinalized(commitId, recipientA, recipientB, sharesA, sharesB);
    }

    /// @inheritdoc IAutomatedMarketMaker
    function cancelCommitDeposit(bytes32 commitId, bool isTokenA) external nonReentrant returns (uint256 amount) {
        Escrow storage e = _escrows[commitId];
        if (e.finalized) revert AMM__CommitAlreadyFinalized(commitId);

        if (isTokenA) {
            if (e.depositorA != msg.sender) revert AMM__NotDepositor(commitId);
            amount = e.amountA;
            if (amount == 0) revert AMM__NothingToRefund(commitId);
            e.depositorA = address(0);
            e.recipientA = address(0);
            e.amountA = 0;
            TOKEN_A.safeTransfer(msg.sender, amount);
        } else {
            if (e.depositorB != msg.sender) revert AMM__NotDepositor(commitId);
            amount = e.amountB;
            if (amount == 0) revert AMM__NothingToRefund(commitId);
            e.depositorB = address(0);
            e.recipientB = address(0);
            e.amountB = 0;
            TOKEN_B.safeTransfer(msg.sender, amount);
        }

        emit LogCommitRefunded(commitId, msg.sender, isTokenA, amount);
    }

    /// @notice Reads the escrow state for a commit (recipients, escrowed amounts, finalized flag).
    function getEscrow(bytes32 commitId)
        external
        view
        returns (
            address depositorA,
            address depositorB,
            address recipientA,
            address recipientB,
            uint256 amountA,
            uint256 amountB,
            bool finalized
        )
    {
        Escrow storage e = _escrows[commitId];
        return (e.depositorA, e.depositorB, e.recipientA, e.recipientB, e.amountA, e.amountB, e.finalized);
    }

    /// @inheritdoc IAutomatedMarketMaker
    /// @dev Home-currency ("zap-out") withdrawal — see specs/013-amm-lp-shares §2.1. Ownership is
    ///      enforced by `_burn` reverting on insufficient balance, so a caller can never withdraw
    ///      more than their share (TASK-12 drain fix).
    function removeLiquidity(uint256 shares, address tokenOut, uint256 minAmountOut)
        external
        nonReentrant
        whenNotPaused
        onlyVerified(msg.sender)
        returns (uint256 amountOut)
    {
        if (shares == 0) revert AMM__ZeroAmount();
        if (tokenOut != address(TOKEN_A) && tokenOut != address(TOKEN_B)) {
            revert AMM__InvalidToken();
        }

        uint256 supply = totalSupply();
        uint256 amountA = (shares * reserveA) / supply;
        uint256 amountB = (shares * reserveB) / supply;

        _burn(msg.sender, shares); // reverts (ERC20InsufficientBalance) if caller lacks the shares

        // Pro-rata removal first (price-neutral), then zap the non-home side into the home side.
        reserveA -= amountA;
        reserveB -= amountB;

        if (tokenOut == address(TOKEN_A)) {
            uint256 swapOut = getAmountOut(amountB, reserveB, reserveA, withdrawalFeeBps);
            reserveB += amountB;
            reserveA -= swapOut;
            amountOut = amountA + swapOut;
            TOKEN_A.safeTransfer(msg.sender, amountOut);
        } else {
            uint256 swapOut = getAmountOut(amountA, reserveA, reserveB, withdrawalFeeBps);
            reserveA += amountA;
            reserveB -= swapOut;
            amountOut = amountB + swapOut;
            TOKEN_B.safeTransfer(msg.sender, amountOut);
        }

        if (amountOut < minAmountOut) revert AMM__InsufficientOutputAmount();
        emit LogLiquidityRemoved(msg.sender, shares, tokenOut, amountOut);
    }

    /// @inheritdoc IAutomatedMarketMaker
    /// @dev Paused-only both-sided exit (no swap) so LPs are never trapped during a halt.
    function removeLiquidityEmergency(uint256 shares)
        external
        nonReentrant
        whenPaused
        onlyVerified(msg.sender)
        returns (uint256 amountA, uint256 amountB)
    {
        if (shares == 0) revert AMM__ZeroAmount();

        uint256 supply = totalSupply();
        amountA = (shares * reserveA) / supply;
        amountB = (shares * reserveB) / supply;

        _burn(msg.sender, shares);

        reserveA -= amountA;
        reserveB -= amountB;

        if (amountA > 0) TOKEN_A.safeTransfer(msg.sender, amountA);
        if (amountB > 0) TOKEN_B.safeTransfer(msg.sender, amountB);

        emit LogEmergencyWithdrawal(msg.sender, shares, amountA, amountB);
    }

    // ============================================================================
    //                                SWAP
    // ============================================================================

    /// @inheritdoc IAutomatedMarketMaker
    function getAmountIn(uint256 reserveIn, uint256 reserveOut, uint256 amountOut)
        public
        pure
        returns (uint256 amountIn)
    {
        if (amountOut >= reserveOut) {
            revert AMM__InsufficientLiquidity();
        }

        // Constant Product for Exact Output: dx = (x * dy) / (y - dy); +1 counters truncation.
        uint256 numerator = reserveIn * amountOut;
        uint256 denominator = reserveOut - amountOut;
        amountIn = (numerator / denominator) + 1;
    }

    /// @inheritdoc IAutomatedMarketMaker
    function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 feeBps_)
        public
        pure
        returns (uint256 amountOut)
    {
        if (amountIn == 0 || reserveIn == 0 || reserveOut == 0) {
            return 0;
        }
        // Constant Product for Exact Input with fee in bps: dy = (dx·(1-f)·y) / (x + dx·(1-f)).
        uint256 amountInWithFee = amountIn * (10000 - feeBps_);
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * 10000 + amountInWithFee;
        amountOut = numerator / denominator;
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
        // Empty-pool guard (TASK-13): no swaps against a pool missing either side.
        if (reserveA == 0 || reserveB == 0) revert AMM__InsufficientLiquidity();

        bool isAIn = tokenIn == address(TOKEN_A) && tokenOut == address(TOKEN_B);
        bool isBIn = tokenIn == address(TOKEN_B) && tokenOut == address(TOKEN_A);
        if (!isAIn && !isBIn) revert AMM__InvalidToken();

        uint256 reserveIn = isAIn ? reserveA : reserveB;
        uint256 reserveOut = isAIn ? reserveB : reserveA;

        if (amountOut >= reserveOut) revert AMM__InsufficientLiquidity();

        // R2-H-3: fold the exact-output constant-product quote AND the fee gross-up into a SINGLE
        // ceiling division. The prior two-step form rounded up twice — once in getAmountIn
        // (`+1`) and again in the fee gross-up (`+1`) — over-charging the caller and stranding the
        // excess in the reserves. One ceiling division rounds up exactly once and still favors the
        // pool (k never decreases):
        //   amountIn = ceil( reserveIn * amountOut * 10000 / ((reserveOut - amountOut) * (10000 - feeBps)) )
        // The `amountIn` the caller pays is inclusive of the fee, which stays in the reserves.
        amountIn =
            Math.mulDiv(reserveIn * amountOut, 10000, (reserveOut - amountOut) * (10000 - feeBps), Math.Rounding.Ceil);

        if (amountIn > maxAmountIn) {
            revert AMM__SlippageExceeded(amountIn, maxAmountIn);
        }

        if (isAIn) {
            reserveA += amountIn;
            reserveB -= amountOut;
        } else {
            reserveB += amountIn;
            reserveA -= amountOut;
        }

        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenOut).safeTransfer(to, amountOut);

        emit LogSwap(msg.sender, tokenIn, tokenOut, amountIn, amountOut);
        return amountIn;
    }

    // ============================================================================
    //                       FEES (governance-configurable)
    // ============================================================================

    /// @inheritdoc IAutomatedMarketMaker
    function setFeeBps(uint256 newFeeBps) external onlyGovernance {
        if (newFeeBps > MAX_FEE_BPS) revert AMM__FeeBpsTooHigh(newFeeBps, MAX_FEE_BPS);
        uint256 old = feeBps;
        feeBps = newFeeBps;
        emit LogFeeRateUpdated(old, newFeeBps);
    }

    /// @inheritdoc IAutomatedMarketMaker
    function setWithdrawalFeeBps(uint256 newWithdrawalFeeBps) external onlyGovernance {
        if (newWithdrawalFeeBps > MAX_FEE_BPS) revert AMM__FeeBpsTooHigh(newWithdrawalFeeBps, MAX_FEE_BPS);
        uint256 old = withdrawalFeeBps;
        withdrawalFeeBps = newWithdrawalFeeBps;
        emit LogWithdrawalFeeRateUpdated(old, newWithdrawalFeeBps);
    }
}
