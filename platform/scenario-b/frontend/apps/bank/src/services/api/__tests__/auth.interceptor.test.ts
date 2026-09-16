// SPDX-License-Identifier: Apache-2.0

import axios, { AxiosHeaders, type AxiosAdapter } from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { attachAuthInterceptor } from "../interceptors/auth.interceptor";
import { RELAY_AUTH_INVALID, RELAY_AUTH_NOT_CONFIGURED, RELAY_AUTH_REQUIRED, RELAY_SIGNATURE_INVALID } from "../trust-errors";

const forceLogout = vi.fn();
const reportRejection = vi.fn();
const noteSuccess = vi.fn();
const clear = vi.fn();
const reportConfigurationFault = vi.fn();

vi.mock("../../../stores", () => ({
  useAuthStore: { getState: () => ({ forceLogout }) },
}));

vi.mock("../../../stores/trust.store", () => ({
  useTrustStore: { getState: () => ({ reportRejection, noteSuccess, clear, reportConfigurationFault }) },
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
    // Path, method and a way to ask again. The method is what lets the store refuse to repeat a
    // write, and the probe is what lets Recheck exercise the channel instead of only re-reading the
    // reason — re-reading can never clear the notice, because every status classifies to one.
    expect(reportRejection).toHaveBeenCalledWith("/payments/escrows", "get", expect.any(Function));
  });

  it("hands the store a probe that replays the refused request through the same client", async () => {
    // Same instance matters: the replay inherits the baseURL, credentials and interceptors of the
    // request that was refused. A request rebuilt by hand would drop them and prove nothing.
    const { client, seen } = clientRespondingWith([
      { status: 401, data: { code: RELAY_SIGNATURE_INVALID } },
      { status: 200, data: { ok: true } },
    ]);

    await expect(client.get("/payments/escrows")).rejects.toThrow();
    const probe = reportRejection.mock.calls[0]?.[2] as () => Promise<unknown>;
    expect(probe).toBeTypeOf("function");

    await probe();

    expect(seen).toEqual(["/payments/escrows", "/payments/escrows"]);
  });

  it("marks a refused write as a write, so the store never replays it", async () => {
    const { client } = clientRespondingWith([{ status: 401, data: { code: RELAY_SIGNATURE_INVALID } }]);

    await expect(client.post("/payments/deposits", { amount: "100" })).rejects.toThrow();

    expect(reportRejection).toHaveBeenCalledWith("/payments/deposits", "post", expect.any(Function));
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

  // --- the relay credential, which is neither the session nor the identity ---
  //
  // The bank's gateway authenticates itself to the central bank's with INTERNAL_RELAY_AUTH_SECRET,
  // and PaymentProxyHandler relays the central bank's refusal verbatim. With that secret absent or
  // divergent the refusal reached the browser uncoded, took the session branch, and threw the
  // operator back to the login screen over a configuration error nobody could see from there.

  it.each([RELAY_AUTH_REQUIRED, RELAY_AUTH_INVALID])(
    "does not treat %s as an expired session",
    async (code) => {
      const refresh = vi.spyOn(axios, "post").mockResolvedValue({ data: {} });
      const { client, seen } = clientRespondingWith([{ status: 401, data: { code, error: "relay auth" } }]);

      await expect(client.get("/payments/deposits")).rejects.toThrow();

      expect(forceLogout).not.toHaveBeenCalled();
      // Refreshing would succeed — the session is fine — and prove nothing, so it is not attempted.
      expect(refresh).not.toHaveBeenCalled();
      expect(seen).toEqual(["/payments/deposits"]);
      refresh.mockRestore();
    },
  );

  it("reports a configuration fault rather than a trust rejection", async () => {
    // The distinction the operator sees: a trust rejection points at onboarding, and this must not.
    const { client } = clientRespondingWith([{ status: 401, data: { code: RELAY_AUTH_INVALID } }]);

    await expect(client.get("/payments/escrows")).rejects.toThrow();

    expect(reportConfigurationFault).toHaveBeenCalledWith("/payments/escrows", "get", expect.any(Function));
    expect(reportRejection).not.toHaveBeenCalled();
  });

  it("reports the 503 raised when the server has no relay credential at all", async () => {
    // Not a 401, so it never reached the session branch — but it is the same operator-facing fault
    // and used to surface as a raw error with no explanation.
    const { client } = clientRespondingWith([{ status: 503, data: { code: RELAY_AUTH_NOT_CONFIGURED } }]);

    await expect(client.get("/payments/deposits")).rejects.toThrow();

    expect(reportConfigurationFault).toHaveBeenCalledWith("/payments/deposits", "get", expect.any(Function));
    expect(forceLogout).not.toHaveBeenCalled();
  });

  it("marks a refused write as a write, so the store never replays it", async () => {
    const { client } = clientRespondingWith([{ status: 401, data: { code: RELAY_AUTH_INVALID } }]);

    await expect(client.post("/payments/deposits", { amount: "100" })).rejects.toThrow();

    expect(reportConfigurationFault).toHaveBeenCalledWith("/payments/deposits", "post", expect.any(Function));
  });
});
