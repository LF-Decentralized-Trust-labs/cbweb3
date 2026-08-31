// SPDX-License-Identifier: Apache-2.0

import type {
  ApproveKycPayload,
  ApproveKycResponse,
  CredentialIssuanceResult,
  IssueCredentialPayload,
  KycStatusEntry,
  PendingKycApiResponse,
  Participant,
} from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

export const registryApi = {
  // Lightweight participant options for pickers: the spoke's registered banks, keyed by
  // bank_code (the identifier matched against the payer's BankID at enforcement).
  // Deduped by bank_code; entries without a bank_code (non-bank participants) are skipped.
  listParticipants: async (): Promise<{ bankCode: string; name: string; status: string }[]> => {
    const response = await httpClient.get<PendingKycApiResponse>("/compliance/participants");
    const seen = new Set<string>();
    const out: { bankCode: string; name: string; status: string }[] = [];
    for (const p of response.data.participants ?? []) {
      const bankCode = (p.bank_code ?? "").trim();
      if (!bankCode || seen.has(bankCode)) continue;
      seen.add(bankCode);
      out.push({ bankCode, name: p.institution_name ?? bankCode, status: p.status });
    }
    return out;
  },
  list: async (): Promise<Participant[]> => {
    const response = await httpClient.get<PendingKycApiResponse>(
      "/compliance/participants",
    );

    const participants = response.data.participants ?? [];

    return participants.map((entry) => ({
      id: entry.user_id,
      name: entry.institution_name ?? entry.user_id,
      legalEntityId: entry.legal_entity_id ?? "—",
      status: entry.status,
      credentialId: entry.certificate_data ? "Issued" : null,
      credentialExpiry: entry.certificate_expiry ?? null,
      createdAt: "",
    }));
  },
  issueCredential: async (
    payload: IssueCredentialPayload,
  ): Promise<CredentialIssuanceResult> => {
    if (useMocks) {
      return mockDb.issueCredential(payload);
    }
    const response = await httpClient.post<CredentialIssuanceResult>(
      "/compliance/kyc/issue-credential",
      payload,
    );
    return response.data;
  },
  listPendingKyc: async (): Promise<KycStatusEntry[]> => {
    const response = await httpClient.get<PendingKycApiResponse>(
      "/compliance/participants",
    );

    const participants = response.data.participants ?? [];

    return participants
      .filter((entry) => entry.status === "CREDENTIAL_REQUESTED")
      .map((entry) => ({
        subject: entry.user_id,
        status: entry.status as KycStatusEntry["status"],
        institution_name: entry.institution_name,
        bank_code: entry.bank_code,
        country: entry.country_code,
        wallet_address: entry.wallet_address,
      }));
  },
  getKycStatus: async (subject: string): Promise<KycStatusEntry> => {
    if (useMocks) {
      return mockDb.getKycStatus(subject);
    }

    const response = await httpClient.get<KycStatusEntry>(
      `/compliance/kyc/status/${subject}`,
    );
    return response.data;
  },
  approveKyc: async (
    payload: ApproveKycPayload,
  ): Promise<ApproveKycResponse> => {
    const response = await httpClient.post<ApproveKycResponse>(
      "/compliance/approve-kyc",
      payload,
    );
    return response.data;
  },
};
