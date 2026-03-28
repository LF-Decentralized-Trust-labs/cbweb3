import type { TransactionLifecycle } from "./common.types";

export interface FXAgreement {
  id: string;
  counterparty: string;
  rate: string;
  notional: string;
  expiryAt: string;
}

export interface HTLCLock {
  id: string;
  agreementId: string;
  hashLock: string;
  amount: string;
  status: TransactionLifecycle;
  expiryAt: string;
}

export interface CreateAgreementRequest {
  counterparty: string;
  rate: string;
  notional: string;
  expiryAt: string;
}
