// SPDX-License-Identifier: Apache-2.0

import { beforeEach, describe, expect, it, vi } from "vitest";
import { LOGIN_ERROR_COPY } from "@cbweb3/ui";
import { useAuthStore } from "../auth.store";

// The mapping itself is tested in @cbweb3/ui. What these pin is the WIRING — that the store calls
// it instead of falling back to axios's message. That single line could be reverted with every
// other suite still green, which is the gap a review caught on the nonce work: a classifier covered
// and the one call site joining it to reality not.

const login = vi.fn();
const me = vi.fn();

vi.mock("../../services/api", () => ({
  authApi: {
    login: (...args: unknown[]) => login(...args),
    me: () => me(),
    logout: vi.fn(),
    refresh: vi.fn(),
  },
}));

vi.mock("../../services/api/token-refresh", () => ({
  scheduleTokenRefresh: vi.fn(),
  cancelTokenRefresh: vi.fn(),
}));

const axiosRejection = (status: number, code: string) => ({
  isAxiosError: true,
  message: `Request failed with status code ${status}`,
  response: { status, data: { error: "whatever the gateway wrote", code } },
});

describe("useAuthStore.login error copy", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthStore.setState({ error: null, status: "idle", isAuthenticated: false, profile: null });
  });

  it("shows the gateway's cause instead of axios's status text", async () => {
    login.mockRejectedValue(axiosRejection(401, "INVALID_CREDENTIALS"));

    await useAuthStore.getState().login("treasury@cb-peru", "wrong");

    const { error } = useAuthStore.getState();
    expect(error).toBe(LOGIN_ERROR_COPY.invalidCredentials);
    expect(error).not.toMatch(/Request failed with status code/);
  });

  it("does not blame the credentials when the auth service is down", async () => {
    login.mockRejectedValue(axiosRejection(503, "AUTH_SERVICE_UNAVAILABLE"));

    await useAuthStore.getState().login("treasury@cb-peru", "correct-password");

    expect(useAuthStore.getState().error).toBe(LOGIN_ERROR_COPY.serviceUnavailable);
  });

  // The BCRP report, at the layer where it would have surfaced: a restore probe answering 400 must
  // read as a finished session, never as a wrong password.
  it("never presents a refresh-token refusal as a rejected credential", async () => {
    login.mockRejectedValue(axiosRejection(400, "MISSING_REFRESH_TOKEN"));

    await useAuthStore.getState().login("treasury@cb-peru", "correct-password");

    const { error } = useAuthStore.getState();
    expect(error).toBe(LOGIN_ERROR_COPY.sessionExpired);
    expect(error).not.toMatch(/refreshToken/);
  });
});
