import type {
  AMMConfigRequest,
  AuditLogEntry,
  CircuitBreakerRequest,
  CredentialRevocationRequest,
  CredentialRevocationResponse,
  DecryptTransactionRequest,
  DecryptTransactionResponse,
  InstitutionalOnboardingRequest,
  LoginResponse,
  NetworkOverview,
  Participant,
  PoolStatus,
  SanctionsCheckResponse,
  SanctionsListEntry,
  StabilityAlert,
  SupervisorUser,
} from "../../types";
import { CredentialStatus, InstitutionType, Permission, SupervisorRole } from "../../types";

const nowIso = () => new Date().toISOString();
const makeId = (prefix: string) => `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
const wait = async (ms = 300) => new Promise((resolve) => setTimeout(resolve, ms));

const supervisorUser: SupervisorUser = {
  id: "sup_001",
  username: "supervisor@centralbank.gov",
  role: SupervisorRole.SUPERVISOR_ROLE,
  institutionId: "cb_lacnet",
  institutionName: "LAC Regional Central Bank",
  walletAddress: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
  permissions: [
    Permission.VIEW_NETWORK,
    Permission.MANAGE_PARTICIPANTS,
    Permission.REVOKE_CREDENTIALS,
    Permission.DECRYPT_TRANSACTIONS,
    Permission.CIRCUIT_BREAKER,
    Permission.MANAGE_AMM,
    Permission.CHECK_SANCTIONS,
  ],
  createdAt: nowIso(),
};

let sessionActive = false;

const sanctionsList: SanctionsListEntry[] = [
  {
    address: "0x1111111111111111111111111111111111111111",
    entityName: "Sanctioned Finance Group",
    reason: "AML investigation",
    addedAt: "2026-02-01T12:00:00.000Z",
    source: "OFAC",
  },
];

let participants: Participant[] = [
  {
    id: "ptc_001",
    institutionName: "Banco Andino",
    evmAddress: "0xA4B1D0d4A8c7E4B8D3E9A7B6C5D4E3F2A1B0C9D8",
    publicKey: "pk_banco_andino",
    credentialStatus: CredentialStatus.ACTIVE,
    jurisdictionCode: "PE",
    institutionType: InstitutionType.COMMERCIAL_BANK,
    onboardedAt: "2026-01-10T08:00:00.000Z",
    lastActivityAt: nowIso(),
  },
  {
    id: "ptc_002",
    institutionName: "Banco del Sur",
    evmAddress: "0x1111111111111111111111111111111111111111",
    publicKey: "pk_banco_sur",
    credentialStatus: CredentialStatus.ACTIVE,
    jurisdictionCode: "CL",
    institutionType: InstitutionType.COMMERCIAL_BANK,
    onboardedAt: "2026-01-15T08:00:00.000Z",
    lastActivityAt: nowIso(),
  },
];

const pools: PoolStatus[] = [
  {
    pair: "BRL/USD",
    reserveA: 710_000,
    reserveB: 290_000,
    ratioA: 71,
    ratioB: 29,
    isImbalanced: true,
    updatedAt: nowIso(),
  },
  {
    pair: "COP/USD",
    reserveA: 510_000,
    reserveB: 490_000,
    ratioA: 51,
    ratioB: 49,
    isImbalanced: false,
    updatedAt: nowIso(),
  },
];

let circuitBreakerActive = false;
let feeBps = 30;
let slippageBps = 50;

let auditLogs: AuditLogEntry[] = [
  {
    id: "audit_001",
    actor: "supervisor@centralbank.gov",
    action: "POOL_STATUS_VIEW",
    target: "BRL/USD",
    timestamp: nowIso(),
    status: "SUCCESS",
  },
];

const addAudit = (action: string, target: string, status: "SUCCESS" | "FAILED" = "SUCCESS") => {
  auditLogs = [
    {
      id: makeId("audit"),
      actor: supervisorUser.username,
      action,
      target,
      timestamp: nowIso(),
      status,
    },
    ...auditLogs,
  ];
};

export const mockDb = {
  async login(username: string, password: string): Promise<LoginResponse> {
    await wait();
    if (username !== supervisorUser.username || password.length < 6) {
      throw new Error("Invalid institutional credentials");
    }
    sessionActive = true;
    return { user: supervisorUser, sessionTimeoutSeconds: 900 };
  },

  async me(): Promise<LoginResponse> {
    await wait(120);
    if (!sessionActive) {
      throw new Error("Session expired");
    }
    return { user: supervisorUser, sessionTimeoutSeconds: 900 };
  },

  async logout() {
    await wait(80);
    sessionActive = false;
    return { ok: true };
  },

  async getNetworkOverview(): Promise<NetworkOverview> {
    await wait();
    return {
      totalSupply: 4_250_000,
      activeInstitutions: participants.filter((item) => item.credentialStatus === CredentialStatus.ACTIVE).length,
      activeAgreements: 18,
      healthyPools: pools.filter((item) => !item.isImbalanced).length,
      imbalancedPools: pools.filter((item) => item.isImbalanced).length,
      lastUpdatedAt: nowIso(),
    };
  },

  async listParticipants(): Promise<Participant[]> {
    await wait();
    return participants;
  },

  async onboardInstitution(payload: InstitutionalOnboardingRequest): Promise<Participant> {
    await wait(450);
    const participant: Participant = {
      id: makeId("ptc"),
      institutionName: payload.institutionName,
      evmAddress: payload.evmAddress,
      publicKey: payload.publicKey,
      credentialStatus: CredentialStatus.ACTIVE,
      jurisdictionCode: payload.jurisdictionCode,
      institutionType: payload.institutionType,
      onboardedAt: nowIso(),
      lastActivityAt: nowIso(),
    };
    participants = [participant, ...participants];
    addAudit("INSTITUTION_ONBOARDED", payload.evmAddress);
    return participant;
  },

  async revokeCredential(payload: CredentialRevocationRequest): Promise<CredentialRevocationResponse> {
    await wait(400);
    participants = participants.map((participant) =>
      participant.evmAddress.toLowerCase() === payload.address.toLowerCase()
        ? { ...participant, credentialStatus: CredentialStatus.REVOKED, lastActivityAt: nowIso() }
        : participant,
    );
    addAudit("CREDENTIAL_REVOKED", payload.address);
    return {
      address: payload.address,
      revokedAt: nowIso(),
      reason: payload.reason,
      effectiveDate: payload.effectiveDate ?? nowIso(),
    };
  },

  async checkSanctions(address: string): Promise<SanctionsCheckResponse> {
    await wait(200);
    const matches = sanctionsList.filter((item) => item.address.toLowerCase() === address.toLowerCase());
    addAudit("SANCTIONS_CHECK_PERFORMED", address);
    return {
      address,
      isSanctioned: matches.length > 0,
      matches,
      checkedAt: nowIso(),
    };
  },

  async getPoolStatuses(): Promise<PoolStatus[]> {
    await wait(220);
    return pools;
  },

  async setCircuitBreaker(payload: CircuitBreakerRequest) {
    await wait(250);
    circuitBreakerActive = payload.action === "PAUSE";
    addAudit(`CIRCUIT_BREAKER_${payload.action}`, payload.reason);
    return {
      isActive: circuitBreakerActive,
      updatedAt: nowIso(),
      reason: payload.reason,
    };
  },

  async updateAmmConfig(payload: AMMConfigRequest) {
    await wait(250);
    feeBps = payload.feeBps;
    slippageBps = payload.slippageBps;
    addAudit("AMM_CONFIG_UPDATED", payload.reason);
    return {
      feeBps,
      slippageBps,
      updatedAt: nowIso(),
    };
  },

  async getStabilityAlerts(): Promise<StabilityAlert[]> {
    await wait(150);
    return pools
      .filter((pool) => pool.isImbalanced)
      .map((pool) => ({
        id: makeId("alert"),
        pair: pool.pair,
        severity: "CRITICAL" as const,
        message: `Liquidity imbalance detected in ${pool.pair} (${pool.ratioA}/${pool.ratioB})`,
        createdAt: nowIso(),
      }));
  },

  async decryptTransaction(payload: DecryptTransactionRequest): Promise<DecryptTransactionResponse> {
    await wait(350);
    addAudit("TRANSACTION_DECRYPTED", payload.txHash);
    return {
      txHash: payload.txHash,
      amount: "145000",
      currency: "tCeBM",
      sender: "0x2D16A7cD9B8e10f0017D5Ce10093A6EfBc004123",
      receiver: "0x5DcbE8A2d18F88E7aCb43199d0f660f7eAB40f10",
      decryptedAt: nowIso(),
    };
  },

  async getAuditLogs(): Promise<AuditLogEntry[]> {
    await wait(140);
    return auditLogs.slice(0, 50);
  },
};
