export type ZKPointerState = "VALID" | "INVALID" | "EXPIRED";

export interface ZKPointerVerification {
  bankId: string;
  pointerId: string;
  commitmentHash: string;
  state: ZKPointerState;
  expiresAt: string | null;
}
