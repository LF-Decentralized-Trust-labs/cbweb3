import type {
  AccountEntry,
  AuditFilter,
  CircuitBreakerPayload,
  CircuitBreakerState,
  FreezePayload,
  GovernanceAuditEntry,
  GovernanceParameters,
  IssueCredentialPayload,
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
  login: async (username: string, password: string) => {
    await new Promise((resolve) => setTimeout(resolve, 250));
    if (username !== "governance.admin" || password !== "GovAdmin2026!") {
      throw new Error("Invalid credentials");
    }
    return {
      token: "mock-governance-token",
      user: {
        id: "gov-001",
        username,
        displayName: "Governance Committee Operator",
        role: "CENTRAL_BANK_ADMIN" as const,
      },
    };
  },
  listParticipants: async () => {
    await new Promise((resolve) => setTimeout(resolve, 160));
    return participants;
  },
  issueCredential: async (payload: IssueCredentialPayload) => {
    await new Promise((resolve) => setTimeout(resolve, 180));
    const existing = participants.find((participant) => participant.cnpj === payload.cnpj);
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
        category: "REGISTRY",
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
      cnpj: payload.cnpj,
      status: "ACTIVE",
      credentialId: `cred-${participantId}`,
      credentialExpiry: new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString(),
      createdAt: new Date().toISOString(),
    };
    participants = [created, ...participants];

    appendAudit({
      actor: "governance.admin",
      action: "Credential issued",
      category: "REGISTRY",
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
