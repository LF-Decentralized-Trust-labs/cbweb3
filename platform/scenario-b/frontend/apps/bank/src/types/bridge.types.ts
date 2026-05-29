export const BRIDGE_STATE = {
  LOCKING: "LOCKING",
  ACTIVE: "ACTIVE",
  BURNING: "BURNING",
  BURNED: "BURNED",
  RELEASED: "RELEASED",
  RECONCILIATION_REQUIRED: "RECONCILIATION_REQUIRED",
} as const;

export type BridgeState = (typeof BRIDGE_STATE)[keyof typeof BRIDGE_STATE];

export interface BridgedAssetPosition {
  position_id: string;
  owner_bank_id: string;
  spoke_network: string;
  native_asset: string;
  mirrored_asset: string;
  mirrored_amount: string;
  bridge_state: BridgeState;
  relayer_retries: number;
  relayer_error_log: string | null;
  created_at: string;
  updated_at: string;
}

export interface LockMintRequest {
  amount: string;
}

export interface BurnUnlockRequest {
  position_id: string;
}
