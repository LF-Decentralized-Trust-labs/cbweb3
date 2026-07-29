// SPDX-License-Identifier: Apache-2.0

import type { LoginResponse, SysAdminUser } from "../../types";

const KEYCLOAK_URL = import.meta.env.VITE_KEYCLOAK_URL ?? "http://localhost:8080";
const REALM = import.meta.env.VITE_KEYCLOAK_REALM ?? "cbweb3";
const CLIENT_ID = import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? "cbweb3-noc";

function parseJwtPayload(token: string): Record<string, unknown> {
  try {
    const base64 = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
    return JSON.parse(atob(base64)) as Record<string, unknown>;
  } catch {
    return {};
  }
}

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const body = new URLSearchParams({
      grant_type: "password",
      client_id: CLIENT_ID,
      username,
      password,
    });

    const res = await fetch(
      `${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/token`,
      { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body },
    );

    if (!res.ok) {
      const err = await res.json().catch(() => ({})) as Record<string, string>;
      throw new Error(err["error_description"] ?? "Invalid credentials");
    }

    const tokens = await res.json() as { access_token: string; refresh_token: string };
    localStorage.setItem("noc_access_token", tokens.access_token);
    localStorage.setItem("noc_refresh_token", tokens.refresh_token);

    const payload = parseJwtPayload(tokens.access_token);
    const user: SysAdminUser = {
      id: (payload["sub"] as string) ?? "",
      name: (payload["preferred_username"] as string) ?? username,
      role: "SYS_ADMIN",
      institutionId: (payload["bank_id"] as string) ?? "",
    };
    return { user };
  },

  me: async (): Promise<LoginResponse> => {
    const token = localStorage.getItem("noc_access_token");
    if (!token) throw new Error("No session");
    const payload = parseJwtPayload(token);
    const exp = payload["exp"] as number | undefined;
    if (exp && Date.now() / 1000 > exp) {
      throw new Error("Token expired");
    }
    const user: SysAdminUser = {
      id: (payload["sub"] as string) ?? "",
      name: (payload["preferred_username"] as string) ?? "NOC User",
      role: "SYS_ADMIN",
      institutionId: (payload["bank_id"] as string) ?? "",
    };
    return { user };
  },

  logout: async () => {
    localStorage.removeItem("noc_access_token");
    localStorage.removeItem("noc_refresh_token");
    return { ok: true };
  },
};

