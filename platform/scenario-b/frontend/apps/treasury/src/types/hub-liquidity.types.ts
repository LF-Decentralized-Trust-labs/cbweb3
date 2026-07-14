// SPDX-License-Identifier: Apache-2.0

// Hub AMM pool provisioning types (Mint & Approve + Seed liquidity + list pairs).
// These mirror the payment-orchestrator /amm endpoints consumed by the Central Bank
// treasury operator.

export type PoolSide = "A" | "B";

// --- Currency registry (Hub) ---

export interface RegisterCurrencyRequest {
  symbol: string;
  country_name: string;
  token_address: string;
  proposer_cb: string;
}

export interface RegisterCurrencyResponse {
  symbol: string;
  tx_hash: string;
}

export interface HubCurrency {
  symbol: string;
  country_name: string;
  token_address: string;
  proposer_cb: string;
}

export interface HubCurrenciesResponse {
  currencies: HubCurrency[];
}

// --- Pair proposal / confirmation (Hub AMM) ---

export interface ProposePairRequest {
  pair_id: string;
  token_a_address: string;
  token_b_address: string;
  /** Optional override. Leave empty to let the gateway deploy a dedicated per-pair AMM. */
  amm_address?: string;
  proposer_cb: string;
}

export interface ProposePairResponse {
  pair_id: string;
  status: string;
  tx_hash: string;
  /** Address of the dedicated AMM the gateway deployed (or the supplied override). */
  amm_address?: string;
}

export interface ConfirmPairRequest {
  pair_id: string;
  confirmer_cb: string;
}

export interface ConfirmPairResponse {
  pair_id: string;
  status: string;
  tx_hash: string;
}

export interface MintAndApproveRequest {
  /** Non-negative integer amount, as a decimal string. */
  amount: string;
  pool_pair: string;
  side: PoolSide;
  /** Optional — omit to mint & approve for the AMM itself; if set, mints to that address. */
  recipient?: string;
}

export interface MintAndApproveResponse {
  status: string;
  amount: string;
  recipient?: string;
}

export interface AddLiquidityRequest {
  pool_pair: string;
  provider_bank_id: string;
  token_a_amount: string;
  token_b_amount: string;
}

/** Liquidity result object — treated as opaque; only status/success are surfaced. */
export interface AddLiquidityResponse {
  status?: string;
  success?: boolean;
  [key: string]: unknown;
}

export interface HubPair {
  pair_id: string;
  status: string;
  token_a_address: string;
  token_b_address: string;
  amm_address: string;
  proposer_cb: string;
  confirmer_cb: string;
  proposed_at?: string;
  confirmed_at?: string;
}

// Hub configuration — canonical on-chain addresses (values may be empty if unset).
export interface HubConfig {
  amm_address: string;
  pair_registry: string;
  currency_registry: string;
}

export interface HubPairsResponse {
  pairs: HubPair[];
}
