// SPDX-License-Identifier: Apache-2.0

import type {
  ComplianceCredential,
  OnRampRequest,
  OnRampRequestPayload,
  TokenBalance,
  TokenTransaction,
  TransferRequest,
} from "../../types";

type MockUser = {
  id: string;
  name: string;
  institutionId: string;
  role: "COMMERCIAL_BANK_OPERATOR";
  walletAddress?: string;
};

const nowIso = () => new Date().toISOString();

const makeId = (prefix: string) => `${prefix}_${Math.random().toString(36).slice(2, 10)}`;

const currentUser: MockUser = {
  id: "user_bank_operator",
  name: "Bank Operator",
  institutionId: "bank-bra",
  role: "COMMERCIAL_BANK_OPERATOR",
  walletAddress: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
};

let tokenBalance: TokenBalance = {
  publicBalance: "1250000",
  privateBalance: "410000",
  currency: "tCeBM",
  updatedAt: nowIso(),
};

let tokenTransactions: TokenTransaction[] = [
  { id: makeId("tx"), kind: "TRANSFER", amount: "75000", status: "PENDING", createdAt: nowIso() },
];

let onRampRequests: OnRampRequest[] = [
  {
    id: makeId("req"),
    type: "ON_RAMP",
    amount: "200000",
    fiatProofRef: "reserve-proof-2026-03",
    justification: "Monthly settlement window liquidity allocation.",
    status: "PENDING",
    createdAt: nowIso(),
  },
];

const credentials: ComplianceCredential[] = [
  { id: "cred_kyc_001", type: "KYC_VERIFIED", issuer: "CB Compliance Node", expiresAt: "2027-12-31T23:59:59.000Z" },
  { id: "cred_aml_001", type: "AML_CLEARED", issuer: "CB Compliance Node", expiresAt: "2027-12-31T23:59:59.000Z" },
];

const wait = async (ms = 400) => new Promise((resolve) => setTimeout(resolve, ms));

export const mockDb = {
  wait,
  currentUser,
  async login(username: string, password: string) {
    await wait();
    if (!username || !password) {
      throw new Error("Invalid credentials");
    }
    return { user: currentUser };
  },
  async bindWallet(walletAddress: string) {
    await wait();
    currentUser.walletAddress = walletAddress;
    return currentUser;
  },
  async logout() {
    await wait(150);
    return { ok: true };
  },
  async getBalance() {
    await wait(250);
    return tokenBalance;
  },
  async transfer(params: TransferRequest) {
    await wait(500);
    const amount = BigInt(params.amount);
    const current = BigInt(tokenBalance.publicBalance);
    if (amount > current) {
      throw new Error("Insufficient public balance");
    }
    tokenBalance = {
      ...tokenBalance,
      publicBalance: String(current - amount),
      updatedAt: nowIso(),
    };
    const tx: TokenTransaction = {
      id: makeId("tx"),
      kind: "TRANSFER",
      amount: params.amount,
      status: "CONFIRMED",
      createdAt: nowIso(),
    };
    tokenTransactions = [tx, ...tokenTransactions];
    return tx;
  },
  async submitOnRampRequest(payload: OnRampRequestPayload) {
    await wait(500);
    const request: OnRampRequest = {
      id: makeId("req"),
      type: payload.type,
      amount: payload.amount,
      fiatProofRef: payload.fiatProofRef,
      justification: payload.justification,
      status: "PENDING",
      createdAt: nowIso(),
    };
    onRampRequests = [request, ...onRampRequests];

    const tx: TokenTransaction = {
      id: makeId("tx"),
      kind: "ON_RAMP_REQUEST",
      amount: payload.amount,
      status: "CONFIRMED",
      createdAt: nowIso(),
    };
    tokenTransactions = [tx, ...tokenTransactions];
    return request;
  },
  async getOnRampRequests() {
    await wait(250);
    return onRampRequests;
  },
  async getTransactions() {
    await wait(250);
    return tokenTransactions;
  },
  async getCredentials() {
    await wait(250);
    return credentials;
  },
  async attachCredentials(transactionId: string, credentialIds: string[]) {
    await wait(400);
    return { transactionId, credentialIds };
  },
};
