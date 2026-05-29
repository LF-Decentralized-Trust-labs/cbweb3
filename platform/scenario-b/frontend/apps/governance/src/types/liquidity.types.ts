export const LP_STATUS = {
  ACTIVE: "ACTIVE",
  WITHDRAWN: "WITHDRAWN",
} as const;

export type LpStatus = (typeof LP_STATUS)[keyof typeof LP_STATUS];

export interface PoolStatus {
  pool_pair: string;
  reserve_a: string;
  reserve_b: string;
  current_ratio: string;
  imbalance_flag: boolean;
  updated_at: string;
  pool_status?: PoolLifecycleStatus;
  fee_rate_bps?: number;
  total_lp_count?: number;
  pending_commits?: PendingCommit[];
}

export interface LiquidityPosition {
  lp_id: string;
  pool_pair: string;
  provider_bank_id: string;
  token_a_contributed: string;
  token_b_contributed: string;
  lp_shares: string;
  status: LpStatus;
  added_at: string;
  deposit_side: string;
  commit_id: string;
}

export interface RemoveLiquidityRequest {
  lp_id: string;
  pool_pair: string;
  provider_bank_id: string;
}

export interface BridgeLockMintRequest {
  amount: string;
}

export interface BridgeLockMintResponse {
  position_id: string;
  bridge_state: string;
  spoke_network: string;
  native_asset: string;
  mirrored_asset: string;
  amount: string;
}

export interface LpPositionsResponse {
  pool_pair: string;
  positions: LiquidityPosition[];
  count: number;
}

export interface ApproveAmmRequest {
  amount: string;
  side?: "A" | "B";
}

export const LIQUIDITY_OPERATIONAL_ERROR = {
  CROSS_CB_MINT_PROHIBITED: "CROSS_CB_MINT_PROHIBITED",
  BRIDGE_POSITION_NOT_ACTIVE: "BRIDGE_POSITION_NOT_ACTIVE",
} as const;

export type LiquidityOperationalError =
  (typeof LIQUIDITY_OPERATIONAL_ERROR)[keyof typeof LIQUIDITY_OPERATIONAL_ERROR];

export const SOVEREIGN_FLOW_PHASE = {
  LOCK_MINT: "LOCK_MINT",
  BRIDGE_ACTIVE_WAIT: "BRIDGE_ACTIVE_WAIT",
  COMMIT_PENDING: "COMMIT_PENDING",
  COMMIT_EXECUTED: "COMMIT_EXECUTED",
  POOL_ACTIVE: "POOL_ACTIVE",
  CANCELLED: "CANCELLED",
  FAILED: "FAILED",
  TIMEOUT: "TIMEOUT",
} as const;

export type SovereignFlowPhase =
  (typeof SOVEREIGN_FLOW_PHASE)[keyof typeof SOVEREIGN_FLOW_PHASE];

export const SOVEREIGN_FLOW_POLICY = {
  bridgePollingIntervalMs: 5_000,
  bridgeTimeoutMs: 120_000,
  commitPollingIntervalMs: 3_000,
  commitWarnAfterMs: 30_000,
  commitTimeoutMs: 180_000,
} as const;

// --- Commit-Reveal protocol types (006-cooperative-liquidity-wizard) ---

export type PoolLifecycleStatus = "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
export type CommitSide = "A" | "B";
export type CommitStatus = "PENDING" | "EXECUTED" | "CANCELLED";

export interface PendingCommit {
  commit_id: string;
  provider_id: string;
  side: CommitSide;
  amount: string;
  expires_at: string;
}

export interface CommitRequest {
  pool_pair: string;
  amount: string;
}

export interface CommitResult {
  commit_id: string;
  on_chain_commit_id: string | null;
  pool_pair: string;
  side: CommitSide;
  amount: string;
  status: CommitStatus;
  expires_at: string;
  lp_ids: string[] | null;
}

export interface CommitsListResponse {
  commits: CommitListItem[];
  count: number;
}

export interface CommitListItem {
  CommitID: string;
  PoolPair: string;
  ProviderID: string;
  Side: string;
  Amount: string;
  Status: string;
  OnChainCommitID: string | null;
  CounterpartCommitID: string | null;
  CreatedAt: string;
  ExpiresAt: string;
}
