// SPDX-License-Identifier: Apache-2.0

import { beforeEach, describe, expect, it, vi } from "vitest";
import { useStatementStore } from "../statement.store";

// The mapping itself is tested in @cbweb3/ui. What this pins is the WIRING — that a store reaches
// for the gateway's own words instead of axios's status text.
//
// The statement store is the one measured in the guard audit: with a divergent
// INTERNAL_RELAY_AUTH_SECRET the central bank answers 401, this portal's gateway wraps that into a
// 502 whose body names the cause, and the operator read "Request failed with status code 502".
// Neither they nor support could tell a bad relay credential from a central bank that was down.
//
// One store rather than all fifteen, deliberately: the source guard in
// __tests__/api-error-surfacing.test.ts is what covers every call site. This proves the helper
// actually works through a real store, which a source scan cannot.

const list = vi.fn();

vi.mock("../../services/api/statement.api", () => ({
  statementApi: {
    list: () => list(),
  },
}));

/** The exact 502 the api-gateway produces for this path (handlers/statement.go:135). */
const relayAuthRejection = () => ({
  isAxiosError: true,
  message: "Request failed with status code 502",
  response: {
    status: 502,
    data: {
      error:
        'statement: load deposits: central bank returned 401: {"error":"invalid relay auth secret"}',
    },
  },
});

describe("useStatementStore error surfacing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useStatementStore.setState({ movements: [], status: "idle", error: null });
  });

  it("shows the gateway's cause instead of axios's status text", async () => {
    list.mockRejectedValue(relayAuthRejection());

    await useStatementStore.getState().fetchAll();

    const { error, status } = useStatementStore.getState();
    expect(status).toBe("error");
    expect(error).toContain("invalid relay auth secret");
    expect(error).not.toMatch(/Request failed with status code/);
  });

  it("keeps the status visible when the gateway explains nothing", async () => {
    list.mockRejectedValue({
      isAxiosError: true,
      message: "Request failed with status code 500",
      response: { status: 500, data: {} },
    });

    await useStatementStore.getState().fetchAll();

    const { error } = useStatementStore.getState();
    expect(error).toContain("Unable to load statement");
    expect(error).toContain("500");
  });
});
