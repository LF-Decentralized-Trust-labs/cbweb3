// SPDX-License-Identifier: Apache-2.0

export interface AMMQuoteRequest {
  tokenIn: string;
  tokenOut: string;
  exactOutputAmount: string;
}

export interface AMMQuoteResponse {
  requiredInputAmount: string;
  priceImpactPct: number;
  slippagePct: number;
}

export interface AMMPoolStatus {
  tokenA: string;
  tokenB: string;
  reserveA: string;
  reserveB: string;
  imbalanceFlag: boolean;
}
