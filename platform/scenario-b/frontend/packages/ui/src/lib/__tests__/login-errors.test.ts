// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { LOGIN_ERROR_COPY, loginErrorMessage } from "../login-errors";

// The defect these pin: every portal rendered axios's `error.message`, so an operator saw "Request
// failed with status code 400" and could not tell a wrong secret from an empty field from a gateway
// that was down. The gateway already wrote a usable message and a code; nothing read either.

const axiosResponse = (status: number, data?: unknown) => ({
  isAxiosError: true,
  message: `Request failed with status code ${status}`,
  response: { status, data },
});

describe("loginErrorMessage", () => {
  it("explains a refused credential without naming a field", () => {
    const message = loginErrorMessage(axiosResponse(401, { error: "invalid credentials", code: "INVALID_CREDENTIALS" }));

    expect(message).toBe(LOGIN_ERROR_COPY.invalidCredentials);
    // User enumeration: an operator must not learn from the copy whether the ID exists.
    expect(message).not.toMatch(/user|username|account|exists|unknown client/i);
  });

  it("tells a missing field apart from a rejected credential", () => {
    expect(loginErrorMessage(axiosResponse(400, { error: "clientSecret is required", code: "MISSING_CREDENTIALS" }))).toBe(
      LOGIN_ERROR_COPY.missingCredentials,
    );
  });

  it("does not blame the credentials when the service is down", () => {
    // Saying "wrong secret" here sends an operator to rotate a secret that was fine.
    expect(
      loginErrorMessage(axiosResponse(503, { error: "authentication service unavailable", code: "AUTH_SERVICE_UNAVAILABLE" })),
    ).toBe(LOGIN_ERROR_COPY.serviceUnavailable);
  });

  it("reads a malformed request as a client fault, not a credential one", () => {
    expect(loginErrorMessage(axiosResponse(400, { error: "invalid body", code: "INVALID_REQUEST" }))).toBe(
      LOGIN_ERROR_COPY.invalidRequest,
    );
  });

  // The report behind this: an operator saw "refreshToken is required" on a LOGIN screen. It is a
  // session-restore probe on a page with no session — not a rejected credential, and it must never
  // read as one.
  it("treats a refresh-token refusal as a session, not a credential, problem", () => {
    for (const code of ["MISSING_REFRESH_TOKEN", "INVALID_REFRESH_TOKEN"]) {
      const message = loginErrorMessage(axiosResponse(code === "MISSING_REFRESH_TOKEN" ? 400 : 401, { code }));
      expect(message).toBe(LOGIN_ERROR_COPY.sessionExpired);
      expect(message).not.toBe(LOGIN_ERROR_COPY.invalidCredentials);
    }
  });

  it("still refuses a 401 that carries no code", () => {
    // Older gateways, or a body that never reached us: the status alone still says "refused".
    expect(loginErrorMessage(axiosResponse(401, undefined))).toBe(LOGIN_ERROR_COPY.invalidCredentials);
  });

  it("keeps the status visible for a response it cannot classify", () => {
    expect(loginErrorMessage(axiosResponse(500, { error: "boom" }))).toBe(LOGIN_ERROR_COPY.unexpected(500));
  });

  it("names an unreachable gateway as such", () => {
    expect(loginErrorMessage({ isAxiosError: true, message: "Network Error" })).toBe(LOGIN_ERROR_COPY.unreachable);
  });

  // The portals throw their own errors — a bank portal refusing a PKI flow it does not implement,
  // for one. Those messages are deliberate and better than anything this mapping could substitute.
  it("passes through an error the app raised itself", () => {
    expect(loginErrorMessage(new Error("PKI authentication is not supported in the Bank Portal."))).toBe(
      "PKI authentication is not supported in the Bank Portal.",
    );
  });

  it("has something to say about a value that is not an error at all", () => {
    expect(loginErrorMessage(undefined)).toBe(LOGIN_ERROR_COPY.fallback);
    expect(loginErrorMessage("boom")).toBe(LOGIN_ERROR_COPY.fallback);
  });

  it("never surfaces axios's own status text", () => {
    // The regression in one line: this string is what operators were reading.
    for (const status of [400, 401, 500, 503]) {
      expect(loginErrorMessage(axiosResponse(status, { code: "WHATEVER" }))).not.toMatch(/Request failed with status code/);
    }
  });
});
