// SPDX-License-Identifier: Apache-2.0

export const CROSS_CURRENCY_SWAP_ERROR = {
  POOL_NOT_ACTIVE: "POOL_NOT_ACTIVE",
  CIRCUIT_BREAKER_HALTED: "CIRCUIT_BREAKER_HALTED",
  SLIPPAGE_LIMIT_EXCEEDED: "SLIPPAGE_LIMIT_EXCEEDED",
  INSUFFICIENT_POOL_LIQUIDITY: "INSUFFICIENT_POOL_LIQUIDITY",
  QUOTE_EXPIRED: "QUOTE_EXPIRED",
  BRIDGE_IN_FAILED: "BRIDGE_IN_FAILED",
  SWAP_FAILED: "SWAP_FAILED",
  BRIDGE_OUT_FAILED: "BRIDGE_OUT_FAILED",
  SWAP_NOT_FOUND: "SWAP_NOT_FOUND",
  INVALID_REQUEST: "INVALID_REQUEST",
  TRANSFER_LIMIT_EXCEEDED: "TRANSFER_LIMIT_EXCEEDED",
} as const;

export type CrossCurrencySwapErrorCode =
  (typeof CROSS_CURRENCY_SWAP_ERROR)[keyof typeof CROSS_CURRENCY_SWAP_ERROR];

export type CrossCurrencySwapStatus =
  | "QUOTING"
  | "BRIDGE_IN_PROGRESS"
  | "SWAP_IN_PROGRESS"
  | "BRIDGE_OUT_PROGRESS"
  | "COMPLETED"
  | "FAILED";

export interface CrossCurrencyQuote {
  quote_id: string;
  amount_in: string;
  amount_out: string;
  effective_rate: string;
  fee_bps: number;
  max_slippage_pct: number;
  pool_pair: string;
  time_remaining_seconds: number;
  valid_until: number;
  created_at: number;
}

// SwapHistoryItem is one row from GET /amm/swap/cross-currency (paginated history).
export interface SwapHistoryItem {
  swap_id: string;
  status: CrossCurrencySwapStatus;
  source_currency: string;
  target_currency: string;
  pool_pair: string;
  amount_in: string;
  amount_out: string;
  effective_rate: number;
  swap_tx_hash?: string;
  beneficiary_bank_id?: string;
  failure_reason?: string;
  // The slippage buffer the bridge-in had to move and the swap did not consume, and what
  // became of its return. RETURN_ENQUEUED is on its way back; RETURN_FAILED is being retried;
  // RETURN_ESCALATED means the retries gave up and the amount is still owed to this bank.
  residue_amount?: string;
  residue_status?: "NONE" | "RETURN_ENQUEUED" | "RETURN_FAILED" | "RETURN_ESCALATED";
  residue_position_id?: string;
  created_at: string;
  completed_at?: string;
}

export interface SwapHistoryPage {
  operations: SwapHistoryItem[];
  page: number;
  page_size: number;
  total: number;
}

// SwapHistoryQuery are the filters for the history endpoint. from/to are ISO dates
// (YYYY-MM-DD accepted by the backend); omit for an unbounded window.
export interface SwapHistoryQuery {
  from?: string;
  to?: string;
  page?: number;
  pageSize?: number;
}

export interface CrossCurrencySwapRequest {
  source_currency: string;
  target_currency: string;
  pool_pair: string;
  amount_out: string;
  max_amount_in: string;
  // Deprecated: backend derives payer bank from authenticated claims.
  payer_bank_id?: string;
  beneficiary_bank_id: string;
  quote_id?: string;
}

export interface CrossCurrencySwapResult {
  swap_id: string;
  status: CrossCurrencySwapStatus;
  correlation_id?: string;
  amount_in?: string;
  amount_out?: string;
  effective_rate?: string;
  bridge_in_position_id?: string;
  swap_tx_hash?: string;
  bridge_out_position_id?: string;
  error_code?: string;
  created_at?: string;
}
