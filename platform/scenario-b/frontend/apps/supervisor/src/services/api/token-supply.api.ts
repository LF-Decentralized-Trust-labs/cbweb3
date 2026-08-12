// SPDX-License-Identifier: Apache-2.0

import type { SovereignSupply } from "../../types";
import { apiFetch } from "./apiClient";

interface SovereignSupplyResponse {
  symbol: string;
  token_address: string;
  total_supply: string;
  decimals: number;
}

// getSovereignSupply reads this Central Bank's own wrapped token outstanding on the
// hub (GET /api/v2/hub/token/supply) — the amount of its domestic reserves currently
// bridged in. It is deliberately one currency, not a network-wide aggregate: tCeBM is
// deployed per currency and per layer, so summing across them would mix currencies and
// double-count the same backing.
//
// Throws on failure. Callers must render the card as unavailable rather than 0 — a
// fabricated zero reads as "nothing is bridged".
export async function getSovereignSupply(): Promise<SovereignSupply> {
  const response = await apiFetch<SovereignSupplyResponse>("/api/v2/hub/token/supply");
  return {
    symbol: response.symbol,
    tokenAddress: response.token_address,
    totalSupply: response.total_supply,
    decimals: response.decimals,
  };
}
