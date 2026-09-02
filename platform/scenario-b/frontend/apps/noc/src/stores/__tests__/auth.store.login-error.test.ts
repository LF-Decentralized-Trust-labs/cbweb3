// SPDX-License-Identifier: Apache-2.0

import { beforeEach, describe, expect, it, vi } from "vitest";
import { LOGIN_ERROR_COPY } from "@cbweb3/ui";
import { useAuthStore } from "../auth.store";

// The mapping itself is tested in @cbweb3/ui. What this pins is the WIRING — that this store
// calls it instead of falling back to axios's message. Review of #188 measured the gap: reverting
// each store in turn, only the bank portal had a test that noticed. Four of five were the same
// byte-identical line with nothing guarding it, in a PR whose own argument is that an untested
// call site is how a fix silently disappears.

const login = vi.fn();
const me = vi.fn();

// The store calls setSessionExpiredHandler at module load, so the handler has to be captured by a
// hoisted holder — vi.mock factories run before any top-level declaration in this file.
const captured = vi.hoisted(() => ({ sessionExpired: null as (() => void) | null }));

vi.mock("../../services/api/token", () => ({
  setSessionExpiredHandler: (h: () => void) => {
    captured.sessionExpired = h;
  },
}));

vi.mock("../../services/api", () => ({
  authApi: {
    login: (...args: unknown[]) => login(...args),
    me: () => me(),
    logout: vi.fn(),
    refresh: vi.fn(),
  },
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

  // Finding 4 of the #188 review: this store held the ONLY session-expiry message in Scenario B,
  // worded differently from the shared copy — one condition, two sentences. The store registers its
  // handler at module load, so invoking the captured one is what proves the state it writes.
  it("renders session expiry with the shared copy, not a second wording", () => {
    expect(captured.sessionExpired).toBeTypeOf("function");
    captured.sessionExpired!();

    const { error, isAuthenticated } = useAuthStore.getState();
    expect(error).toBe(LOGIN_ERROR_COPY.sessionExpired);
    expect(error).not.toBe("Session expired. Sign in again.");
    expect(isAuthenticated).toBe(false);
  });

  it("does not blame the credentials when the auth service is down", async () => {
    login.mockRejectedValue(axiosRejection(503, "AUTH_SERVICE_UNAVAILABLE"));

    await useAuthStore.getState().login("operator", "correct-secret");

    expect(useAuthStore.getState().error).toBe(LOGIN_ERROR_COPY.serviceUnavailable);
  });
});
