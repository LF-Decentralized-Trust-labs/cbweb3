import type {
  AccountEntry,
  ApproveKycPayload,
  ApproveKycResponse,
  AuditFilter,
  CircuitBreakerPayload,
  CircuitBreakerState,
  FreezePayload,
  GovernanceAuditEntry,
  GovernanceParameters,
  IssueCredentialPayload,
  KycStatusEntry,
  Participant,
  UpdateParametersPayload,
} from "../../types";
import {
  generateAccounts,
  generateAudit,
  generateParameters,
  generateParticipants,
} from "./data-generators";

let participants: Participant[] = generateParticipants();
let pendingKyc: KycStatusEntry[] = [
  {
    subject: "2f3a37ba-f2a7-4d8e-9ca9-f9052f5df151",
    status: "CREDENTIAL_REQUESTED",
    institution_name: "Banco Horizonte S.A.",
    bank_code: "horizonte",
    country: "BR",
    wallet_address: "0x2AbF9Cde14ea1e39f6Aa9f7b4fA5AfA01B86bF7a",
    created_at: new Date(Date.now() - 20 * 60_000).toISOString(),
  },
  {
    subject: "1300d183-5b6e-4a4d-b26a-88bd8576e4bb",
    status: "CREDENTIAL_REQUESTED",
    institution_name: "Banco Nascente S.A.",
    bank_code: "nascente",
    country: "BR",
    wallet_address: "0xB3197e08F8fA9F0Aaf1A73a0f6Da957d1D8f8695",
    created_at: new Date(Date.now() - 8 * 60_000).toISOString(),
  },
];
let accounts: AccountEntry[] = generateAccounts();
let parameters: GovernanceParameters = generateParameters();
let circuitBreaker: CircuitBreakerState = {
  state: "LIVE",
  updatedAt: new Date().toISOString(),
  updatedBy: "system",
};
let auditLogs: GovernanceAuditEntry[] = generateAudit();

const appendAudit = (entry: Omit<GovernanceAuditEntry, "id" | "createdAt">) => {
  const audit: GovernanceAuditEntry = {
    id: `audit-${Math.random().toString(16).slice(2, 10)}`,
    createdAt: new Date().toISOString(),
    ...entry,
  };
  auditLogs = [audit, ...auditLogs];
};

export const mockDb = {
  listParticipants: async () => {
    await new Promise((resolve) => setTimeout(resolve, 160));
    return participants;
  },
  issueCredential: async (payload: IssueCredentialPayload) => {
    await new Promise((resolve) => setTimeout(resolve, 180));
    const existing = participants.find((participant) => participant.legalEntityId === payload.legalEntityId);
    if (existing) {
      const updated: Participant = {
        ...existing,
        status: "ACTIVE",
        credentialId: `cred-${existing.id}-${Date.now()}`,
        credentialExpiry: new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString(),
      };
      participants = participants.map((participant) => (participant.id === existing.id ? updated : participant));
      appendAudit({
        actor: "governance.admin",
        action: "Credential issued",
        category: "CREDENTIAL",
        severity: "INFO",
        outcome: "SUCCESS",
        metadata: JSON.stringify({ participantId: existing.id, scopes: payload.scopes }),
      });
      return {
        participantId: existing.id,
        credentialId: updated.credentialId!,
        credentialExpiry: updated.credentialExpiry!,
      };
    }

    const participantId = `pt-${Math.random().toString(16).slice(2, 8)}`;
    const created: Participant = {
      id: participantId,
      name: payload.entityName,
      legalEntityId: payload.legalEntityId,
      status: "ACTIVE",
      credentialId: `cred-${participantId}`,
      credentialExpiry: new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString(),
      createdAt: new Date().toISOString(),
    };
    participants = [created, ...participants];

    appendAudit({
      actor: "governance.admin",
      action: "Credential issued",
      category: "CREDENTIAL",
      severity: "INFO",
      outcome: "SUCCESS",
      metadata: JSON.stringify({ participantId, scopes: payload.scopes }),
    });

    return {
      participantId,
      credentialId: created.credentialId!,
      credentialExpiry: created.credentialExpiry!,
    };
  },
  listPendingKyc: async () => {
    await new Promise((resolve) => setTimeout(resolve, 150));
    return pendingKyc.filter((entry) => entry.status === "CREDENTIAL_REQUESTED");
  },
  getKycStatus: async (subject: string) => {
    await new Promise((resolve) => setTimeout(resolve, 100));
    const found = pendingKyc.find((entry) => entry.subject === subject);
    if (!found) {
      throw new Error("KYC request not found");
    }
    return found;
  },
  approveKyc: async (payload: ApproveKycPayload): Promise<ApproveKycResponse> => {
    await new Promise((resolve) => setTimeout(resolve, 220));
    const found = pendingKyc.find((entry) => entry.subject === payload.subject);
    if (!found) {
      throw new Error("KYC request not found");
    }

    const popNonce = Math.random().toString(16).slice(2).padEnd(64, "0").slice(0, 64);
    pendingKyc = pendingKyc.map((entry) =>
      entry.subject === payload.subject
        ? {
            ...entry,
            status: "KYC_APPROVED",
          }
        : entry,
    );

    appendAudit({
      actor: "governance.admin",
      action: "KYC approved",
      category: "CREDENTIAL",
      severity: "INFO",
      outcome: "SUCCESS",
      metadata: JSON.stringify({ subject: payload.subject, reason: payload.reason }),
    });

    return {
      status: "KYC_APPROVED",
      pop_nonce: popNonce,
    };
  },
  getCircuitBreaker: async () => {
    await new Promise((resolve) => setTimeout(resolve, 120));
    return circuitBreaker;
  },
  setCircuitBreaker: async (payload: CircuitBreakerPayload) => {
    await new Promise((resolve) => setTimeout(resolve, 200));
    circuitBreaker = {
      state: payload.pause ? "HALTED" : "LIVE",
      updatedAt: new Date().toISOString(),
      updatedBy: "governance.admin",
    };

    appendAudit({
      actor: "governance.admin",
      action: payload.pause ? "Circuit breaker paused" : "Circuit breaker resumed",
      category: "CIRCUIT_BREAKER",
      severity: "CRITICAL",
      outcome: "SUCCESS",
      metadata: JSON.stringify(payload),
    });

    return {
      success: true,
      state: circuitBreaker.state,
      updatedAt: circuitBreaker.updatedAt,
      updatedBy: circuitBreaker.updatedBy,
    };
  },
  listAccounts: async () => {
    await new Promise((resolve) => setTimeout(resolve, 160));
    return accounts;
  },
  freezeAccount: async (payload: FreezePayload) => {
    await new Promise((resolve) => setTimeout(resolve, 220));
    const account = accounts.find((item) => item.id === payload.accountId);
    if (!account) {
      throw new Error("Account not found");
    }
    const frozenAt = new Date().toISOString();
    accounts = accounts.map((item) =>
      item.id === payload.accountId
        ? {
            ...item,
            frozen: true,
            frozenAt,
            frozenReason: payload.reason,
          }
        : item,
    );
    participants = participants.map((participant) =>
      participant.id === account.participantId ? { ...participant, status: "FROZEN" } : participant,
    );

    appendAudit({
      actor: "governance.admin",
      action: "Account frozen",
      category: "FREEZE",
      severity: "CRITICAL",
      outcome: "SUCCESS",
      metadata: JSON.stringify(payload),
    });

    return {
      success: true,
      accountId: payload.accountId,
      frozenAt,
    };
  },
  getParameters: async () => {
    await new Promise((resolve) => setTimeout(resolve, 100));
    return parameters;
  },
  updateParameters: async (payload: UpdateParametersPayload) => {
    await new Promise((resolve) => setTimeout(resolve, 180));
    parameters = {
      txLimitMin: payload.txLimitMin,
      txLimitMax: payload.txLimitMax,
      slippageTolerance: payload.slippageTolerance,
      settlementWindowSeconds: payload.settlementWindowSeconds,
    };

    appendAudit({
      actor: "governance.admin",
      action: "Parameters updated",
      category: "PARAMETER",
      severity: "CRITICAL",
      outcome: "SUCCESS",
      metadata: JSON.stringify(payload),
    });

    return parameters;
  },
  listAuditLogs: async (filter?: AuditFilter) => {
    await new Promise((resolve) => setTimeout(resolve, 140));
    return auditLogs.filter((entry) => {
      const categoryMatch = !filter?.category || filter.category === "ALL" || entry.category === filter.category;
      const severityMatch = !filter?.severity || filter.severity === "ALL" || entry.severity === filter.severity;
      const fromMatch = !filter?.dateFrom || new Date(entry.createdAt) >= new Date(filter.dateFrom);
      const toMatch = !filter?.dateTo || new Date(entry.createdAt) <= new Date(filter.dateTo);
      return categoryMatch && severityMatch && fromMatch && toMatch;
    });
  },
};
