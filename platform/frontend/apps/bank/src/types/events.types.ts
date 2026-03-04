export type EventType =
  | "htlc.locked"
  | "htlc.settled"
  | "token.minted"
  | "token.transferred"
  | "amm.pool.updated";

export interface BankEvent<TPayload = unknown> {
  id: string;
  type: EventType;
  occurredAt: string;
  payload: TPayload;
}
