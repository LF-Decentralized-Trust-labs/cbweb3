// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { apiErrorMessage, REQUEST_ERROR_COPY } from "../request-errors";

/**
 * The defect these pin, measured on `develop` before this module existed.
 *
 * Every store and page in Scenario A's bank portal surfaced `error.message`, which for axios is the
 * string "Request failed with status code NNN". The gateway's own explanation sat in
 * `response.data.error` and was thrown away — 31 call sites across 13 files.
 *
 * The case that found it: a divergent INTERNAL_RELAY_AUTH_SECRET makes the central bank answer 401,
 * the bank's gateway wraps that into a 502 whose body says exactly what happened, and the operator
 * read "Request failed with status code 502". Neither they nor support could tell a bad relay
 * credential from a central bank that was down.
 */

/** Shapes the real client produces, built here so the tests do not depend on axios itself. */
const axiosFailure = (status: number, data: unknown) => ({
  isAxiosError: true,
  message: `Request failed with status code ${status}`,
  response: { status, data },
});

const networkFailure = () => ({
  isAxiosError: true,
  message: "Network Error",
  response: undefined,
});

const FALLBACK = "Unable to load the statement";

describe("apiErrorMessage", () => {
  it("returns the gateway's own explanation instead of axios's", () => {
    const error = axiosFailure(502, {
      error: "statement: load deposits: central bank returned 401: invalid relay auth secret",
    });
    expect(apiErrorMessage(error, FALLBACK)).toBe(
      "statement: load deposits: central bank returned 401: invalid relay auth secret",
    );
  });

  // The regression guard proper: the whole point is NOT to render this string.
  it("never returns axios's own message when the response carries one", () => {
    const error = axiosFailure(502, { error: "the central bank refused the relay signature" });
    expect(apiErrorMessage(error, FALLBACK)).not.toContain("Request failed with status code");
  });

  // 214 handlers in this gateway answer with `error`; two answer with `message`. Reading only the
  // first would leave those two rendering the fallback and look like the defect never got fixed.
  it("reads `message` for the handlers that use it", () => {
    const error = axiosFailure(400, { message: "escrow amount must be positive" });
    expect(apiErrorMessage(error, FALLBACK)).toBe("escrow amount must be positive");
  });

  it("prefers `error` when a body carries both", () => {
    const error = axiosFailure(400, { error: "the specific one", message: "the generic one" });
    expect(apiErrorMessage(error, FALLBACK)).toBe("the specific one");
  });

  it("accepts a body that is a bare string", () => {
    expect(apiErrorMessage(axiosFailure(503, "upstream is draining"), FALLBACK)).toBe(
      "upstream is draining",
    );
  });

  // A body with nothing usable must still say more than the caller's fallback alone: the status is
  // the one fact support can act on, and dropping it is how "something went wrong" screens happen.
  it("keeps the status visible when the body explains nothing", () => {
    for (const body of [{}, { error: "" }, { error: 42 }, null, undefined]) {
      const message = apiErrorMessage(axiosFailure(500, body), FALLBACK);
      expect(message).toContain(FALLBACK);
      expect(message).toContain("500");
    }
  });

  // Not the same failure, and it must not be described as one. There is no gateway answer to quote.
  it("says the gateway was unreachable when there is no response at all", () => {
    expect(apiErrorMessage(networkFailure(), FALLBACK)).toBe(REQUEST_ERROR_COPY.unreachable);
  });

  // An Error the app raised itself is deliberate and carries its own explanation; replacing it with
  // a network story would be a regression of a different kind.
  it("passes through an Error the app threw", () => {
    expect(apiErrorMessage(new Error("Select a beneficiary before continuing"), FALLBACK)).toBe(
      "Select a beneficiary before continuing",
    );
  });

  it("falls back for anything it cannot read", () => {
    for (const thrown of [undefined, null, 7, {}, new Error("")]) {
      expect(apiErrorMessage(thrown, FALLBACK)).toBe(FALLBACK);
    }
  });

  // Whitespace-only prose is not an explanation; treating it as one renders a blank error box.
  it("does not treat blank prose as an explanation", () => {
    const message = apiErrorMessage(axiosFailure(400, { error: "   " }), FALLBACK);
    expect(message).toContain(FALLBACK);
  });
});
