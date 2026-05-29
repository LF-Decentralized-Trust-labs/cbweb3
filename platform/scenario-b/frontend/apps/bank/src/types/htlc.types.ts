export type HTLCState =
  | "HTLC_STATE_INVALID"
  | "HTLC_STATE_PENDING"
  | "HTLC_STATE_LOCKED"
  | "HTLC_STATE_SETTLED"
  | "HTLC_STATE_REFUNDED"
  | "HTLC_STATE_SETTLING"
  | "HTLC_STATE_REFUNDING";

export type HTLCSearchState = "LOCKED" | "SETTLED" | "REFUNDED" | "SETTLING" | "REFUNDING";

export interface HTLCLock {
  contract_id: string;
  sender: string;
  receiver: string;
  hash_lock: string;
  time_lock: number;
  secret?: string;
  state: HTLCState;
}

export interface LockHTLCRequest {
  agreement_id?: string;
  receiver: string;
  amount: string;
  time_lock?: number;
}

export interface LockWithHashHTLCRequest {
  hash_lock: string;
  receiver: string;
  amount: string;
  agreement_id?: string;
}

export interface LockHTLCResponse {
  contract_id: string;
  hash_lock: string;
  htlc_tx_hash?: string;
}

export interface SettleHTLCRequest {
  contract_id: string;
  secret: string;
}

export interface SettleHTLCResponse {
  htlc_tx_hash?: string;
}

export interface RefundHTLCResponse {
  htlc_tx_hash?: string;
}

export interface SearchHTLCParams {
  state?: HTLCSearchState;
  agreement_id?: string;
  sender?: string;
  receiver?: string;
}

export interface SearchHTLCResponse {
  locks: HTLCLock[];
  total: number;
}
