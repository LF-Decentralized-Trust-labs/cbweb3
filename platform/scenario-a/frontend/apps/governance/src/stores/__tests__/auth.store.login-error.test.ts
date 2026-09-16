// SPDX-License-Identifier: Apache-2.0

import { beforeEach, describe, expect, it, vi } from "vitest";
import { LOGIN_ERROR_COPY } from "@cbweb3/ui";
import { useAuthStore } from "../auth.store";

// The mapping itself is tested in @cbweb3/ui. What this pins is the WIRING — that this store
// calls it instead of falling back to axios's message. Review of the Scenario B twin (#188)
// measured that gap by reverting each store in turn and watching only one notice; the same
// applies here, where the line is byte-identical across five portals.

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
    useAuthStore.setState({ error: null, status: "idle", isAuthenticated: false });
  });

  it("shows the gateway's cause instead of axios's status text", async () => {
    login.mockRejectedValue(axiosRejection(401, "INVALID_CREDENTIALS"));

    await useAuthStore.getState().login("operator", "wrong");

    const { error } = useAuthStore.getState();
    expect(error).toBe(LOGIN_ERROR_COPY.invalidCredentials);
    expect(error).not.toMatch(/Request failed with status code/);
  });

  it("does not blame the credentials when the auth service is down", async () => {
    login.mockRejectedValue(axiosRejection(503, "AUTH_SERVICE_UNAVAILABLE"));

    await useAuthStore.getState().login("operator", "correct-password");

    expect(useAuthStore.getState().error).toBe(LOGIN_ERROR_COPY.serviceUnavailable);
  });
});
