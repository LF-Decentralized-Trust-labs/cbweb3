export interface TokenBalance {
  publicBalance: string;
  privateBalance: string;
  currency: string;
  updatedAt: string;
}

export interface MintRequest {
  amount: string;
  fiatProofRef: string;
}

export interface TransferRequest {
  toAddress: string;
  amount: string;
  shielded: boolean;
}

export interface TokenTransaction {
  id: string;
  kind: "MINT" | "TRANSFER";
  amount: string;
  status: "PENDING" | "CONFIRMED" | "FAILED";
  createdAt: string;
}
