// SPDX-License-Identifier: Apache-2.0

export type TvlSnapshot = {
  region: string;
  tvl: string;
  updatedAt: string;
};

export type SpokeHubDelta = {
  spokeSupply: string;
  hubMirror: string;
  delta: string;
  severity: "INFO" | "WARNING" | "CRITICAL";
  updatedAt: string;
};
