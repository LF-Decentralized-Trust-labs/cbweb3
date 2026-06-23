// SPDX-License-Identifier: Apache-2.0

import type {
  BridgedAssetPosition,
  BridgeState,
  BurnUnlockRequest,
  LockMintRequest,
} from "../../types/bridge.types";
import { httpClientV2 } from "./http-client";

type BridgePositionsResponse = {
  positions: BridgedAssetPosition[];
};

export const bridgeApi = {
  lockMint: async (payload: LockMintRequest): Promise<BridgedAssetPosition> => {
    const response = await httpClientV2.post<BridgedAssetPosition>("/bridge/lock-mint", payload);
    return response.data;
  },
  burnUnlock: async (
    payload: BurnUnlockRequest,
  ): Promise<{ position_id: string; bridge_state: BridgeState }> => {
    const response = await httpClientV2.post<{ position_id: string; bridge_state: BridgeState }>(
      "/bridge/burn-unlock",
      payload,
    );
    return response.data;
  },
  listPositions: async (): Promise<BridgedAssetPosition[]> => {
    const response = await httpClientV2.get<BridgePositionsResponse | BridgedAssetPosition[]>("/bridge/positions");

    if (Array.isArray(response.data)) {
      return response.data;
    }

    return response.data.positions ?? [];
  },
};
