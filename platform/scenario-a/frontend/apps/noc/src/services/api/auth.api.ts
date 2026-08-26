// SPDX-License-Identifier: Apache-2.0

import type { LoginResponse, SysAdminUser } from "../../types";
import { clearTokens, decodeJwtPayload, ensureFreshToken, keycloakConfig, storeTokens } from "./token";

function userFromToken(token: string, fallbackName: string): SysAdminUser {
  const payload = decodeJwtPayload(token);
  const realmAccess = payload["realm_access"] as { roles?: string[] } | undefined;
  return {
    id: (payload["sub"] as string) ?? "",
    name: (payload["preferred_username"] as string) ?? fallbackName,
    roles: realmAccess?.roles ?? [],
    role: "SYS_ADMIN",
    institutionId: (payload["bank_id"] as string) ?? "",
  };
}

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const body = new URLSearchParams({
      grant_type: "password",
      client_id: keycloakConfig.clientId,
      username,
      password,
    });

    const res = await fetch(
      `${keycloakConfig.url}/realms/${keycloakConfig.realm}/protocol/openid-connect/token`,
      { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body },
    );

    if (!res.ok) {
      const err = await res.json().catch(() => ({})) as Record<string, string>;
      throw new Error(err["error_description"] ?? "Invalid credentials");
    }

    const tokens = await res.json() as { access_token: string; refresh_token: string };
    storeTokens(tokens.access_token, tokens.refresh_token);

    return { user: userFromToken(tokens.access_token, username) };
  },

  // Restores the session on reload. ensureFreshToken renews an expired access token from
  // the stored refresh token, so reopening the portal after the 5-minute access lifespan
  // keeps the operator signed in instead of bouncing to /login.
  me: async (): Promise<LoginResponse> => {
    const token = await ensureFreshToken();
    if (!token) throw new Error("No session");
    return { user: userFromToken(token, "NOC User") };
  },

  logout: async () => {
    clearTokens();
    return { ok: true };
  },
};
