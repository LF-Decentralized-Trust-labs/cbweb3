// SPDX-License-Identifier: Apache-2.0

import type { LoginResponse, SysAdminUser } from "../../types";
import { httpClient } from "./http-client";

// The profile the backend returns. It comes from the server rather than from a decoded
// token because the session is now an HttpOnly cookie: the browser cannot read it, which is
// the entire point — the old portal built this by decoding the token itself, and that only
// worked because any script on the page could read it too.
type BackendProfile = {
  id: string;
  name: string;
  roles: string[];
};

function toSysAdminUser(profile: BackendProfile): SysAdminUser {
  return {
    id: profile.id,
    name: profile.name,
    roles: profile.roles ?? [],
    role: "SYS_ADMIN",
    institutionId: "",
  };
}

export const authApi = {
  // The grant happens on the backend, which sets the session cookie, the refresh cookie and
  // the readable XSRF-TOKEN beside them. The portal never sees a token.
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const response = await httpClient.post<{ user: BackendProfile }>("/auth/login", {
      username,
      password,
    });
    return { user: toSysAdminUser(response.data.user) };
  },

  // Restores the session on reload: the browser still holds the cookie, but the page has no
  // profile. A 401 here is the honest answer that there is no session, and the caller sends
  // the operator to /login.
  me: async (): Promise<LoginResponse> => {
    const response = await httpClient.get<{ user: BackendProfile }>("/auth/me");
    return { user: toSysAdminUser(response.data.user) };
  },

  // Server-side, because only the server can expire the cookies it set. Clearing state in
  // the page would leave the session alive on the next request.
  logout: async () => {
    await httpClient.post("/auth/logout", {});
    return { ok: true };
  },
};
