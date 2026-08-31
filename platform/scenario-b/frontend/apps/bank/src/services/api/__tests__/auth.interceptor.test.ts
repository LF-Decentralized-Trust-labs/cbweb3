// SPDX-License-Identifier: Apache-2.0

import axios, { AxiosHeaders, type AxiosAdapter } from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { attachAuthInterceptor } from "../interceptors/auth.interceptor";
import { RELAY_SIGNATURE_INVALID } from "../trust-errors";

const forceLogout = vi.fn();
const reportRejection = vi.fn();
const noteSuccess = vi.fn();
const clear = vi.fn();

vi.mock("../../../stores", () => ({
  useAuthStore: { getState: () => ({ forceLogout }) },
}));

vi.mock("../../../stores/trust.store", () => ({
  useTrustStore: { getState: () => ({ reportRejection, noteSuccess, clear }) },
}));

/**
 * A client whose transport is scripted per call, so a retry can be told apart from the first attempt.
 */
const clientRespondingWith = (responses: Array<{ status: number; data?: unknown }>) => {
  const seen: string[] = [];
  const adapter: AxiosAdapter = async (config) => {
    seen.push(config.url ?? "");
    const next = responses.shift() ?? { status: 200, data: {} };
    const headers = new AxiosHeaders();
    const response = { status: next.status, statusText: "", data: next.data ?? {}, headers, config };
    if (next.status >= 400) {
      throw Object.assign(new axios.AxiosError("failed", "ERR_BAD_REQUEST", config, undefined, response), {
        isAxiosError: true,
      });
    }
    return response;
  };
  const client = axios.create({ baseURL: "http://gateway.test/api/v1", adapter });
  attachAuthInterceptor(client);
  return { client, seen };
};

describe("attachAuthInterceptor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("refreshes and retries an expired session", async () => {
    const refresh = vi.spyOn(axios, "post").mockResolvedValue({ data: {} });
    const { client } = clientRespondingWith([{ status: 401, data: { error: "invalid or expired token" } }, { status: 200, data: { ok: true } }]);

    const response = await client.get("/payments/deposits");

    expect(response.data).toEqual({ ok: true });
    expect(refresh).toHaveBeenCalledOnce();
    expect(forceLogout).not.toHaveBeenCalled();
    refresh.mockRestore();
  });

  it("logs out when the refresh does not fix the session", async () => {
    const refresh = vi.spyOn(axios, "post").mockResolvedValue({ data: {} });
    const { client } = clientRespondingWith([
      { status: 401, data: { error: "invalid or expired token" } },
      { status: 401, data: { error: "invalid or expired token" } },
    ]);

    await expect(client.get("/payments/deposits")).rejects.toThrow();

    expect(forceLogout).toHaveBeenCalled();
    refresh.mockRestore();
  });

  it("does not treat the central bank rejecting our identity as an expired session", async () => {
    // The regression this guards: the session cookie is perfectly valid, so refreshing succeeds, the
    // retry is rejected again, and the second 401 used to reach the logout branch — throwing the
    // operator back to the login screen for a failure that has nothing to do with their session.
    const refresh = vi.spyOn(axios, "post").mockResolvedValue({ data: {} });
    const { client, seen } = clientRespondingWith([
      { status: 401, data: { code: RELAY_SIGNATURE_INVALID, error: "relay signature verification failed" } },
    ]);

    await expect(client.get("/payments/deposits")).rejects.toThrow();

    expect(forceLogout).not.toHaveBeenCalled();
    expect(refresh).not.toHaveBeenCalled();
    expect(seen).toEqual(["/payments/deposits"]);
    refresh.mockRestore();
  });

  it("reports the rejection with its path, so a later success on that path can retire the notice", async () => {
    const { client } = clientRespondingWith([
      { status: 401, data: { code: RELAY_SIGNATURE_INVALID, error: "relay signature verification failed" } },
    ]);

    await expect(client.get("/payments/escrows")).rejects.toThrow();

    expect(reportRejection).toHaveBeenCalledOnce();
    expect(reportRejection).toHaveBeenCalledWith("/payments/escrows");
  });

  it("tells the store about every success, letting it decide whether trust was restored", async () => {
    const { client } = clientRespondingWith([{ status: 200, data: { deposits: [] } }]);

    await client.get("/payments/deposits");

    expect(noteSuccess).toHaveBeenCalledWith("/payments/deposits");
    // The interceptor does not decide: only the store knows which paths were rejected.
    expect(clear).not.toHaveBeenCalled();
  });

  it("compares paths without the query string, so a scoped call still retires its own notice", async () => {
    const { client } = clientRespondingWith([{ status: 200, data: {} }]);

    await client.get("/payments/deposits?status=PENDING");

    expect(noteSuccess).toHaveBeenCalledWith("/payments/deposits");
  });
});
