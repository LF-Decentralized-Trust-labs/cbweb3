// SPDX-License-Identifier: Apache-2.0

export type MovementDirection = "credit" | "debit";
export type MovementToken = "fCeBM" | "tCeBM";
export type MovementKind = "deposit" | "tokenisation" | "redeem" | "pvp_settlement";

// A single credit/debit line on the commercial bank statement (extrato).
export interface Movement {
  id: string;
  timestamp: string; // RFC3339
  direction: MovementDirection;
  token: MovementToken;
  amount: string; // integer units, as persisted
  kind: MovementKind | string;
  reference?: string;
}

export interface StatementResponse {
  movements: Movement[];
  total: number;
}
