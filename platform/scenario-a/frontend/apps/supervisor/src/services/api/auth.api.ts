import { Permission, SupervisorRole } from "../../types";
import type { LoginResponse, SupervisorUser } from "../../types";
import { apiFetch } from "./apiClient";

type LoginTokenResponse = {
  accessToken: string;
  expiresIn: number;
  tokenType: string;
  refreshToken?: string;
};

type MeResponse = {
  subject: string;
  issuer?: string;
  roles: string[];
  wallet?: string;
  bankId?: string;
  country?: string;
};

function roleFromClaims(roles: string[]): SupervisorRole {
  if (roles.includes("ROLE_SUPERVISOR")) return SupervisorRole.SUPERVISOR_ROLE;
  if (roles.includes("ROLE_NOC")) return SupervisorRole.COMPLIANCE_OFFICER;
  return SupervisorRole.CENTRAL_BANK_ADMIN;
}

function mapMeToUser(me: MeResponse): SupervisorUser {
  return {
    id: me.subject,
    username: me.subject,
    role: roleFromClaims(me.roles),
    institutionId: me.bankId ?? "",
    institutionName: me.bankId ?? "Central Bank",
    walletAddress: me.wallet ?? "",
    permissions: [
      Permission.VIEW_NETWORK,
      Permission.VIEW_REGISTRY,
      Permission.VIEW_AUDIT_LOGS,
      Permission.DECRYPT_TRANSACTIONS,
      Permission.VIEW_STABILITY,
    ],
    createdAt: new Date().toISOString(),
  };
}

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const token = await apiFetch<LoginTokenResponse>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ clientId: username, clientSecret: password }),
    });
    const me = await apiFetch<MeResponse>("/api/v1/auth/me");
    return {
      user: mapMeToUser(me),
      sessionTimeoutSeconds: token.expiresIn,
    };
  },

  me: async (): Promise<SupervisorUser> => {
    const me = await apiFetch<MeResponse>("/api/v1/auth/me");
    return mapMeToUser(me);
  },

  logout: async (): Promise<void> => {
    await apiFetch<unknown>("/api/v1/auth/logout", { method: "POST" });
  },
};
