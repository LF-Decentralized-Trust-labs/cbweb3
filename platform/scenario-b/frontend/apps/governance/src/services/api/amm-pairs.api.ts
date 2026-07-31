// SPDX-License-Identifier: Apache-2.0

import { httpClientV2 } from "./http-client";

export type AmmPair = { pair_id: string; status: string };

// ammPairsApi lists the AMM pairs registered on the hub (PairRegistry). Public GET —
// used to drive per-pair governance actions (e.g. the circuit breaker) against the
// pairs the Central Banks actually created.
export const ammPairsApi = {
  getPairs: async (): Promise<AmmPair[]> => {
    const res = await httpClientV2.get<{ pairs?: AmmPair[] }>("/amm/pairs");
    return res.data.pairs ?? [];
  },
};
