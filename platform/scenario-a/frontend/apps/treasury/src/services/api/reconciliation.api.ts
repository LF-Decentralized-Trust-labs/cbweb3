// SPDX-License-Identifier: Apache-2.0

import type { SpokeHubDelta, TvlSnapshot } from "../../types";
import { fiatUnitLabel } from "../../types";
import { httpClient } from "./http-client";

type BalanceResponse = { balance: string };

export const reconciliationApi = {
  getTvl: async (): Promise<TvlSnapshot[]> => {
    const [tcebm, fcebm] = await Promise.all([
      httpClient.get<BalanceResponse>("/token/balance"),
      httpClient.get<BalanceResponse>("/token/fiat-balance").catch(() => ({ data: { balance: "0" } })),
    ]);
    const updatedAt = new Date().toISOString();
    return [
      { region: "tCeBM (Zeto)", tvl: tcebm.data.balance, updatedAt },
      { region: `fCeBM (${fiatUnitLabel})`, tvl: fcebm.data.balance, updatedAt },
    ];
  },
  getDelta: async (): Promise<SpokeHubDelta> => {
    const [tcebm, fcebm] = await Promise.all([
      httpClient.get<BalanceResponse>("/token/balance"),
      httpClient.get<BalanceResponse>("/token/fiat-balance").catch(() => ({ data: { balance: "0" } })),
    ]);
    const spoke = BigInt(tcebm.data.balance || "0");
    const hub = BigInt(fcebm.data.balance || "0");
    const delta = spoke - hub;
    const absDelta = delta < 0n ? -delta : delta;
    const severity: SpokeHubDelta["severity"] = absDelta === 0n ? "INFO" : absDelta < 1000n ? "WARNING" : "CRITICAL";
    return {
      spokeSupply: spoke.toString(),
      hubMirror: hub.toString(),
      delta: delta.toString(),
      severity,
      updatedAt: new Date().toISOString(),
    };
  },
};
