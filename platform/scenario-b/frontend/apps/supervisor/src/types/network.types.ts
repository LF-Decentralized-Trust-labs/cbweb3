// SPDX-License-Identifier: Apache-2.0

// SovereignSupply is this Central Bank's own wrapped token outstanding on the hub
// (W-tCeBM_<CUR>) — how much of its domestic reserves is currently bridged in.
export interface SovereignSupply {
  symbol: string;
  tokenAddress: string;
  totalSupply: string; // base units (wei), decimal string
  decimals: number;
}

export interface NetworkOverview {
  // null when the hub token supply could not be read. Render it as unavailable,
  // never as 0 — a fabricated zero reads as "nothing is bridged".
  sovereignSupply: SovereignSupply | null;
  activeInstitutions: number;
  healthyPools: number;
  imbalancedPools: number;
  lastUpdatedAt: string;
}
