// SPDX-License-Identifier: Apache-2.0

// Pins what the treasury client actually sends (finding R2-M-8).
//
// The redemption screen has always forced the operator to type a reason, and the
// issuance screen to type a reserve proof reference — and both were dropped in
// the browser: the request bodies carried only the account and the amount. The
// server therefore held no record of WHY money was destroyed. These tests fail
// if any of those fields stops reaching the API again.

import { beforeEach, describe, expect, it, vi } from "vitest";

const post = vi.fn();
const get = vi.fn();

vi.mock("./http-client", () => ({
  httpClient: {
    post: (...args: unknown[]) => post(...args),
    get: (...args: unknown[]) => get(...args),
  },
  useMocks: false,
}));

const { treasuryApi } = await import("./treasury.api");

describe("treasuryApi.burn", () => {
  beforeEach(() => {
    post.mockReset();
    post.mockResolvedValue({ data: { tx_hash: "0xabc" } });
  });

  it("sends the operator's reason to the server", async () => {
    await treasuryApi.burn({ sourceAccount: "vault-01", amount: "50000", reason: "quarterly redemption" });

    expect(post).toHaveBeenCalledTimes(1);
    const [path, body] = post.mock.calls[0] as [string, Record<string, unknown>];
    expect(path).toBe("/token/burn");
    expect(body).toMatchObject({ from: "vault-01", amount: "50000", reason: "quarterly redemption" });
  });
});

describe("treasuryApi.mint", () => {
  beforeEach(() => {
    post.mockReset();
    post.mockResolvedValue({ data: { tx_hash: "0xabc" } });
  });

  it("sends the funding request and the reserve proof reference", async () => {
    await treasuryApi.mint({
      requestId: "dep-77",
      targetInstitutionId: "bank-a",
      amount: "100000",
      reserveProofRef: "proof-2026-08-19",
    });

    const [path, body] = post.mock.calls[0] as [string, Record<string, unknown>];
    expect(path).toBe("/token/mint");
    expect(body).toMatchObject({
      to: "bank-a",
      amount: "100000",
      request_id: "dep-77",
      reserve_proof_ref: "proof-2026-08-19",
    });
  });
});

describe("treasuryApi", () => {
  it("no longer exposes a burn-to-mint validation", () => {
    // It was a stub that always returned isValid:true while the screen presented
    // it as a real check and gated the mint button on it. Removed rather than
    // reimplemented: the disciplined issuance path lives in the deposit flow.
    expect("validateBurnToMint" in treasuryApi).toBe(false);
  });
});

describe("treasuryApi.getOperations", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("maps the audit record shape the gateway actually serves", async () => {
    // AuditRecord marshals as log_id / action / details / timestamp. This mapper
    // used to read id / metadata / created_at, so every row lost its id, its
    // amount and its date. The bug was invisible while nothing wrote TREASURY
    // entries; writing them made it reachable.
    get.mockResolvedValue({
      data: {
        logs: [
          {
            log_id: "log-1",
            action: "TOKEN_BURN",
            details: '{"amount":"50000","from":"gateway-paladin-identity","reason":"quarterly"}',
            timestamp: "2026-08-19T12:00:00Z",
          },
        ],
      },
    });

    const [operation] = await treasuryApi.getOperations();

    expect(operation.id).toBe("log-1");
    expect(operation.kind).toBe("BURN");
    expect(operation.amount).toBe("50000");
    expect(operation.createdAt).toBe("2026-08-19T12:00:00Z");
    expect(Number.isNaN(new Date(operation.createdAt).getTime())).toBe(false);
  });

  it("reads a mint the same way", async () => {
    get.mockResolvedValue({
      data: {
        logs: [
          {
            log_id: "log-2",
            action: "TOKEN_MINT",
            details: '{"amount":"100000","to":"bank-a","request_id":"dep-77","reserve_proof_ref":"proof-1"}',
            timestamp: "2026-08-19T13:00:00Z",
          },
        ],
      },
    });

    const [operation] = await treasuryApi.getOperations();
    expect(operation.kind).toBe("MINT");
    expect(operation.amount).toBe("100000");
  });
});
