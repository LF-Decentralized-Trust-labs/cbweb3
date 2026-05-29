export const SWAP_ERROR = {
  POOL_NOT_ACTIVE: "POOL_NOT_ACTIVE",
  SLIPPAGE_LIMIT_EXCEEDED: "SLIPPAGE_LIMIT_EXCEEDED",
  INSUFFICIENT_POOL_LIQUIDITY: "INSUFFICIENT_POOL_LIQUIDITY",
  ZK_VALIDATION_FAILED: "ZK_VALIDATION_FAILED",
  CIRCUIT_BREAKER_HALTED: "CIRCUIT_BREAKER_HALTED",
  APPROVE_AMM_REQUIRED: "APPROVE_AMM_REQUIRED",
} as const;

export type SwapError = (typeof SWAP_ERROR)[keyof typeof SWAP_ERROR];

export interface AMMQuote {
  pair: string;
  amount_out: string;
  required_input: string;
  price_impact: string;
  quote_timestamp: number;
}

export interface SwapOrder {
  order_id: string;
  tx_hash: string;
  amount_in: string;
  state: string;
  confirmed_at: string;
}

export interface PoolStatus {
  pool_pair: string;
  reserve_a: string;
  reserve_b: string;
  current_ratio: string;
  imbalance_flag: boolean;
  updated_at: string;
  pool_status?: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
  fee_rate_bps?: number;
  total_lp_count?: number;
}

export interface SwapRequest {
  pair: string;
  amount_out: string;
  max_amount_in: string;
  payer_id: string;
  beneficiary_id: string;
}

export interface ApproveAmmRequest {
  amount: string;
  side?: "A" | "B";
}
