import type {
  AMMPoolStatus,
  AMMQuoteRequest,
  AMMQuoteResponse,
  ComplianceCredential,
  CreateAgreementRequest,
  FXAgreement,
  HTLCLock,
  OnRampRequest,
  OnRampRequestPayload,
  TokenBalance,
  TokenTransaction,
  TransferRequest,
  User,
} from "../../types";

const nowIso = () => new Date().toISOString();

const makeId = (prefix: string) => `${prefix}_${Math.random().toString(36).slice(2, 10)}`;

const currentUser: User = {
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

let agreements: FXAgreement[] = [];
let locks: HTLCLock[] = [];

let pool: AMMPoolStatus = {
  tokenA: "BRL-tCeBM",
  tokenB: "USD-tCeBM",
  reserveA: "700000",
  reserveB: "300000",
  imbalanceFlag: false,
};

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
  async createAgreement(payload: CreateAgreementRequest) {
    await wait(500);
    const agreement: FXAgreement = { id: makeId("agr"), ...payload };
    agreements = [agreement, ...agreements];
    return agreement;
  },
  async getAgreements() {
    await wait(250);
    return agreements;
  },
  async lockFunds(agreementId: string, hashLock: string) {
    await wait(600);
    const agreement = agreements.find((item) => item.id === agreementId);
    if (!agreement) throw new Error("Agreement not found");
    const lock: HTLCLock = {
      id: makeId("lock"),
      agreementId,
      hashLock,
      amount: agreement.notional,
      status: "LOCKED",
      expiryAt: agreement.expiryAt,
    };
    locks = [lock, ...locks];
    return lock;
  },
  async settle(lockId: string, secret: string) {
    await wait(500);
    if (!secret.trim()) {
      throw new Error("Secret is required");
    }
    locks = locks.map((item) => (item.id === lockId ? { ...item, status: "SETTLED" } : item));
    const settled = locks.find((item) => item.id === lockId);
    if (!settled) throw new Error("Lock not found");
    return settled;
  },
  async getLocks() {
    await wait(250);
    return locks;
  },
  async quoteExactOutput(params: AMMQuoteRequest): Promise<AMMQuoteResponse> {
    await wait(200);
    const reserveIn = BigInt(pool.reserveA);
    const reserveOut = BigInt(pool.reserveB);
    const amountOut = BigInt(params.exactOutputAmount || "0");
    if (amountOut <= 0n || amountOut >= reserveOut) {
      throw new Error("Invalid exact output amount");
    }
    const requiredInputAmount = (reserveIn * amountOut) / (reserveOut - amountOut);
    const spot = Number(reserveOut) / Number(reserveIn);
    const effective = Number(amountOut) / Number(requiredInputAmount || 1n);
    const impact = Math.abs(((effective - spot) / spot) * 100);
    return {
      requiredInputAmount: requiredInputAmount.toString(),
      priceImpactPct: Number(impact.toFixed(4)),
      slippagePct: 0.5,
    };
  },
  async getPoolStatus() {
    await wait(250);
    const a = Number(pool.reserveA);
    const b = Number(pool.reserveB);
    const ratio = a / (a + b);
    return {
      ...pool,
      imbalanceFlag: ratio > 0.7 || ratio < 0.3,
    };
  },
  async swapExactOutput(params: AMMQuoteRequest) {
    const quote = await this.quoteExactOutput(params);
    const reserveA = BigInt(pool.reserveA) + BigInt(quote.requiredInputAmount);
    const reserveB = BigInt(pool.reserveB) - BigInt(params.exactOutputAmount);
    pool = { ...pool, reserveA: reserveA.toString(), reserveB: reserveB.toString() };
    return quote;
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
