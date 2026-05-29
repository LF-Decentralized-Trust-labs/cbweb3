import type {
  AuditLogEntry,
  BurnPayload,
  FundingDecisionPayload,
  FundingRequest,
  IssueCredentialPayload,
  IssuedCredential,
  MintPayload,
  SpokeHubDelta,
  SupplySnapshot,
  TreasuryOperation,
  TreasuryUser,
  TvlSnapshot,
} from "../../types";

const nowIso = () => new Date().toISOString();
const makeId = (prefix: string) => `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
const wait = async (ms = 300) => new Promise((resolve) => setTimeout(resolve, ms));

const currentUser: TreasuryUser = {
  id: "treasury-user-1",
  name: "Treasury Operator",
  institutionId: "central-bank-bra",
  role: "TREASURY",
  walletAddress: "0x1111111111111111111111111111111111111111",
  authorizedIssuer: true,
};

let supply: SupplySnapshot = {
  circulatingSupply: "15000000",
  updatedAt: nowIso(),
};

let fundingRequests: FundingRequest[] = [
  {
    id: "req_1001",
    institutionId: "bank-bra-01",
    institutionName: "Banco Regional A",
    amount: "250000",
    fiatProofRef: "proof-2026-03-001",
    justification: "Quarterly liquidity settlement window",
    status: "PENDING",
    submittedAt: nowIso(),
  },
  {
    id: "req_1002",
    institutionId: "bank-bra-02",
    institutionName: "Banco Regional B",
    amount: "120000",
    fiatProofRef: "proof-2026-03-002",
    justification: "Cross-border settlement buffer",
    status: "APPROVED",
    submittedAt: nowIso(),
    reviewedAt: nowIso(),
    reviewedBy: "Treasury Operator",
    reviewReason: "Validated reserve deposit",
  },
];

let operations: TreasuryOperation[] = [
  {
    id: makeId("op"),
    kind: "MINT",
    amount: "120000",
    status: "CONFIRMED",
    createdAt: nowIso(),
    reference: "req_1002",
  },
];

const tvlSnapshots: TvlSnapshot[] = [
  { region: "BR-SP", tvl: "14750000", updatedAt: nowIso() },
  { region: "BR-RJ", tvl: "15200000", updatedAt: nowIso() },
];

const delta: SpokeHubDelta = {
  spokeSupply: "15000000",
  hubMirror: "14980000",
  delta: "20000",
  severity: "WARNING",
  updatedAt: nowIso(),
};

let auditLogs: AuditLogEntry[] = [
  { id: makeId("audit"), category: "AUTH", message: "Treasury operator logged in", severity: "INFO", createdAt: nowIso() },
  { id: makeId("audit"), category: "TREASURY", message: "Mint operation confirmed for req_1002", severity: "INFO", createdAt: nowIso() },
  { id: makeId("audit"), category: "RECONCILIATION", message: "Spoke-Hub delta exceeded info threshold", severity: "WARNING", createdAt: nowIso() },
];

let credentials: IssuedCredential[] = [];

export const mockDb = {
  async login(username: string, password: string) {
    await wait();
    if (!username || !password) {
      throw new Error("Invalid credentials");
    }
    return { user: currentUser };
  },
  async me() {
    await wait(120);
    return { user: currentUser };
  },
  async logout() {
    await wait(120);
    return { ok: true };
  },

  async getFundingRequests() {
    await wait();
    return fundingRequests;
  },
  async approveFundingRequest(payload: FundingDecisionPayload, reviewer: string) {
    await wait(350);
    fundingRequests = fundingRequests.map((request) =>
      request.id === payload.requestId
        ? {
            ...request,
            status: "APPROVED",
            reviewedAt: nowIso(),
            reviewedBy: reviewer,
            reviewReason: payload.reason || "Approved",
          }
        : request,
    );
    auditLogs = [
      {
        id: makeId("audit"),
        category: "FUNDING",
        message: `Funding request ${payload.requestId} approved`,
        severity: "INFO",
        createdAt: nowIso(),
      },
      ...auditLogs,
    ];
    return fundingRequests.find((request) => request.id === payload.requestId)!;
  },
  async rejectFundingRequest(payload: FundingDecisionPayload, reviewer: string) {
    await wait(350);
    if (!payload.reason?.trim()) {
      throw new Error("Rejection reason is required");
    }
    fundingRequests = fundingRequests.map((request) =>
      request.id === payload.requestId
        ? {
            ...request,
            status: "REJECTED",
            reviewedAt: nowIso(),
            reviewedBy: reviewer,
            reviewReason: payload.reason,
          }
        : request,
    );
    auditLogs = [
      {
        id: makeId("audit"),
        category: "FUNDING",
        message: `Funding request ${payload.requestId} rejected`,
        severity: "WARNING",
        createdAt: nowIso(),
      },
      ...auditLogs,
    ];
    return fundingRequests.find((request) => request.id === payload.requestId)!;
  },

  async getSupplySnapshot() {
    await wait(180);
    return supply;
  },
  async getOperations() {
    await wait(180);
    return operations;
  },
  async validateBurnToMint(requestId: string, amount: string) {
    await wait(180);
    const request = fundingRequests.find((item) => item.id === requestId);
    if (!request) {
      return { isValid: false, reason: "Funding request not found" };
    }
    if (request.status !== "APPROVED") {
      return { isValid: false, reason: "Funding request must be approved" };
    }
    if (BigInt(request.amount) < BigInt(amount)) {
      return { isValid: false, reason: "Requested amount exceeds approved allocation" };
    }
    return { isValid: true };
  },
  async mint(payload: MintPayload) {
    await wait(450);
    const validation = await this.validateBurnToMint(payload.requestId, payload.amount);
    if (!validation.isValid) {
      throw new Error(validation.reason ?? "Burn-to-mint validation failed");
    }
    supply = {
      circulatingSupply: String(BigInt(supply.circulatingSupply) + BigInt(payload.amount)),
      updatedAt: nowIso(),
    };
    operations = [
      {
        id: makeId("op"),
        kind: "MINT",
        amount: payload.amount,
        status: "CONFIRMED",
        createdAt: nowIso(),
        reference: payload.requestId,
      },
      ...operations,
    ];
    auditLogs = [
      {
        id: makeId("audit"),
        category: "TREASURY",
        message: `Mint executed for ${payload.amount} from ${payload.requestId}`,
        severity: "INFO",
        createdAt: nowIso(),
      },
      ...auditLogs,
    ];
    fundingRequests = fundingRequests.map((request) =>
      request.id === payload.requestId
        ? {
            ...request,
            status: "COMPLETED",
          }
        : request,
    );
    return operations[0];
  },
  async burn(payload: BurnPayload) {
    await wait(450);
    if (BigInt(payload.amount) > BigInt(supply.circulatingSupply)) {
      throw new Error("Burn amount exceeds circulating supply");
    }
    supply = {
      circulatingSupply: String(BigInt(supply.circulatingSupply) - BigInt(payload.amount)),
      updatedAt: nowIso(),
    };
    operations = [
      {
        id: makeId("op"),
        kind: "BURN",
        amount: payload.amount,
        status: "CONFIRMED",
        createdAt: nowIso(),
        reference: payload.sourceAccount,
      },
      ...operations,
    ];
    auditLogs = [
      {
        id: makeId("audit"),
        category: "TREASURY",
        message: `Burn executed for ${payload.amount} from ${payload.sourceAccount}`,
        severity: "INFO",
        createdAt: nowIso(),
      },
      ...auditLogs,
    ];
    return operations[0];
  },

  async getTvlSnapshots() {
    await wait(200);
    return tvlSnapshots;
  },
  async getSpokeHubDelta() {
    await wait(200);
    return delta;
  },

  async getAuditLogs() {
    await wait(220);
    return auditLogs;
  },

  async issueCredential(payload: IssueCredentialPayload, issuer: string) {
    await wait(350);
    const credential: IssuedCredential = {
      id: makeId("cred"),
      institutionId: payload.institutionId,
      institutionName: payload.institutionName,
      credentialType: payload.credentialType,
      issuedAt: nowIso(),
      issuedBy: issuer,
    };
    credentials = [credential, ...credentials];
    auditLogs = [
      {
        id: makeId("audit"),
        category: "KYC",
        message: `Credential ${payload.credentialType} issued to ${payload.institutionName}`,
        severity: "INFO",
        createdAt: nowIso(),
      },
      ...auditLogs,
    ];
    return credential;
  },
  async getIssuedCredentials() {
    await wait(220);
    return credentials;
  },
};
