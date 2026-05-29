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
  zeto_lock_ref: string;
  state: HTLCState;
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
