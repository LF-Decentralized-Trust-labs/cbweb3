// SPDX-License-Identifier: Apache-2.0

import { beforeEach, describe, expect, it, vi } from "vitest";
import { LOGIN_ERROR_COPY } from "@cbweb3/ui";
import { useAuthStore } from "../auth.store";

// The mapping itself is tested in @cbweb3/ui. What these pin is the WIRING — that the store calls
// it instead of falling back to axios's message. That line could be reverted with every other suite
// still green, which is the same shape of gap a reviewer caught on the nonce work: a classifier
// covered, and the one call site joining it to reality not.

const login = vi.fn();
const me = vi.fn();

vi.mock("../../services/api", () => ({
  authApi: {
    login: (...args: unknown[]) => login(...args),
    me: () => me(),
    logout: vi.fn(),
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

    await useAuthStore.getState().login("bank-itau", "wrong");

    const { error } = useAuthStore.getState();
    expect(error).toBe(LOGIN_ERROR_COPY.invalidCredentials);
    expect(error).not.toMatch(/Request failed with status code/);
  });

  it("does not blame the credentials when the auth service is down", async () => {
    login.mockRejectedValue(axiosRejection(503, "AUTH_SERVICE_UNAVAILABLE"));

    await useAuthStore.getState().login("bank-itau", "correct-secret");

    expect(useAuthStore.getState().error).toBe(LOGIN_ERROR_COPY.serviceUnavailable);
  });

  it("keeps a message the portal raised itself", async () => {
    // The bank portal refuses the PKI nonce flow deliberately; that text is better than anything
    // the shared mapping could substitute for it.
    login.mockResolvedValue({ nonce: "abc" });

    await useAuthStore.getState().login("bank-itau", "correct-secret");

    expect(useAuthStore.getState().error).toBe("PKI authentication is not supported in the Bank Portal.");
  });
});
