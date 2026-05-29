export type EventType =
  | "htlc.locked"
  | "htlc.settled"
  | "token.transferred"
  | "onramp.request.updated"
  | "amm.pool.updated";

export interface BankEvent<TPayload = unknown> {
  id: string;
  type: EventType;
  occurredAt: string;
  payload: TPayload;
}
