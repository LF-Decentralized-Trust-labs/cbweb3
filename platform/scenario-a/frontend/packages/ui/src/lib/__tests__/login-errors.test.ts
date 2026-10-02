// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { LOGIN_ERROR_COPY, loginErrorMessage } from "../login-errors";

// The defect these pin: every portal rendered axios's `error.message`, so an operator saw "Request
// failed with status code 400" and could not tell a wrong password from an empty field from a
// gateway that was down. The gateway already wrote a message and now a code; nothing read either.

const axiosResponse = (status: number, data?: unknown) => ({
  isAxiosError: true,
  message: `Request failed with status code ${status}`,
  response: { status, data },
});

describe("loginErrorMessage", () => {
  it("explains a refused credential without naming a field", () => {
    const message = loginErrorMessage(axiosResponse(401, { error: "invalid credentials", code: "INVALID_CREDENTIALS" }));

    expect(message).toBe(LOGIN_ERROR_COPY.invalidCredentials);
    // User enumeration: the copy must not reveal whether the account exists.
    expect(message).not.toMatch(/exists|unknown user|no such|account not found/i);
  });

  it("speaks the language of this portal's login form", () => {
    // Scenario A signs in with a username and a password. The gateway still calls the same fields
    // clientId and clientSecret in its prose, which is why the operator-facing copy lives here.
    expect(LOGIN_ERROR_COPY.invalidCredentials).toMatch(/username/i);
    expect(LOGIN_ERROR_COPY.invalidCredentials).toMatch(/password/i);
    expect(LOGIN_ERROR_COPY.invalidCredentials).not.toMatch(/client id|client secret|clientId|clientSecret/i);
    expect(LOGIN_ERROR_COPY.missingCredentials).not.toMatch(/client/i);
  });

  it("tells a missing field apart from a rejected credential", () => {
    expect(loginErrorMessage(axiosResponse(400, { error: "clientSecret is required", code: "MISSING_CREDENTIALS" }))).toBe(
      LOGIN_ERROR_COPY.missingCredentials,
    );
  });

  it("does not blame the credentials when the service is down", () => {
    // Saying "wrong password" here sends an operator to reset a password that was fine.
    expect(
      loginErrorMessage(axiosResponse(503, { error: "authentication service unavailable", code: "AUTH_SERVICE_UNAVAILABLE" })),
    ).toBe(LOGIN_ERROR_COPY.serviceUnavailable);
  });

  it("reads a malformed request as a client fault, not a credential one", () => {
    expect(loginErrorMessage(axiosResponse(400, { error: "invalid body", code: "INVALID_REQUEST" }))).toBe(
      LOGIN_ERROR_COPY.invalidRequest,
    );
  });

  // The BCRP report: "refreshToken is required" seen on a LOGIN screen. It is a session-restore
  // probe on a page with no session — not a rejected credential, and it must never read as one.
  it("treats a refresh-token refusal as a session, not a credential, problem", () => {
    for (const [status, code] of [
      [400, "MISSING_REFRESH_TOKEN"],
      [401, "INVALID_REFRESH_TOKEN"],
    ] as const) {
      const message = loginErrorMessage(axiosResponse(status, { error: "refreshToken is required", code }));
      expect(message).toBe(LOGIN_ERROR_COPY.sessionExpired);
      expect(message).not.toBe(LOGIN_ERROR_COPY.invalidCredentials);
      // And the raw backend string never reaches the operator.
      expect(message).not.toMatch(/refreshToken/);
    }
  });

  it("still refuses a 401 that carries no code", () => {
    expect(loginErrorMessage(axiosResponse(401, undefined))).toBe(LOGIN_ERROR_COPY.invalidCredentials);
  });

  it("keeps the status visible for a response it cannot classify", () => {
    expect(loginErrorMessage(axiosResponse(500, { error: "boom" }))).toBe(LOGIN_ERROR_COPY.unexpected(500));
  });

  it("names an unreachable gateway as such", () => {
    expect(loginErrorMessage({ isAxiosError: true, message: "Network Error" })).toBe(LOGIN_ERROR_COPY.unreachable);
  });

  it("passes through an error the app raised itself", () => {
    expect(loginErrorMessage(new Error("PKI authentication is not supported in this portal."))).toBe(
      "PKI authentication is not supported in this portal.",
    );
  });

  it("has something to say about a value that is not an error at all", () => {
    expect(loginErrorMessage(undefined)).toBe(LOGIN_ERROR_COPY.fallback);
    expect(loginErrorMessage("boom")).toBe(LOGIN_ERROR_COPY.fallback);
  });

  it("never surfaces axios's own status text", () => {
    for (const status of [400, 401, 500, 503]) {
      expect(loginErrorMessage(axiosResponse(status, { code: "WHATEVER" }))).not.toMatch(/Request failed with status code/);
    }
  });
});
