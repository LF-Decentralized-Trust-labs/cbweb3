// SPDX-License-Identifier: Apache-2.0

/**
 * CrossCurrencySwapRelay — generalized for N spokes (TK-B5).
 *
 * Receives a bridge-out notification after the Hub AMM swap and forwards it to
 * the target spoke's gateway, resolved by **lookup of `spoke_out`** in the
 * dynamic SpokeRegistry (no fixed CB-B). Before forwarding, it validates the
 * pair's circuit breaker by reading `isPaused()` ON-CHAIN on `amm_address`
 * (fail-safe: paused/unavailable ⇒ refuse). Auth remains the shared secret
 * (`X-Relay-Auth`); per-CB auth is out of scope (§14.D, pending project-lead).
 */

import { Request, Response } from "express";
import { SpokeRegistry } from "./spoke-registry";
import { RelaySigner } from "./relay-auth";

export interface CrossCurrencyBridgeOutPayload {
  correlation_id: string;
  swap_tx_hash: string;
  pool_pair: string;
  amount_out: string;
  beneficiary_bank_id: string;
  /** Target spoke id — used to look up the destination gateway. */
  spoke_out: string;
  wrapped_target_token: string;
  /** Address of the pair's AMM — read on-chain for the isPaused() gate (TK-B5). */
  amm_address: string;
  swap_sender_address?: string;
}

/** Gate: returns true only when it is SAFE to forward (AMM not paused). */
export type NotPausedGate = (ammAddress: string | undefined) => Promise<boolean>;

export interface CrossCurrencyRelayDeps {
  /**
   * signer, when present, signs each forwarded request with the relay's own identity. Without it the
   * only credential is relayAuthSecret, which is identical in every entity — and the relay was the
   * last caller on it, which is what blocks enabling RELAY_REQUIRE_SIGNATURE on the CBs.
   */
  signer?: RelaySigner;
  registry: SpokeRegistry;
  relayAuthSecret: string;
  notPaused: NotPausedGate;
  /** Injectable fetch for tests (defaults to global fetch). */
  fetchFn?: typeof fetch;
}

export class CrossCurrencySwapRelay {
  private readonly registry: SpokeRegistry;
  private readonly relayAuthSecret: string;
  private readonly signer?: RelaySigner;
  private readonly notPaused: NotPausedGate;
  private readonly fetchFn: typeof fetch;

  constructor(deps: CrossCurrencyRelayDeps) {
    this.registry = deps.registry;
    this.relayAuthSecret = deps.relayAuthSecret;
    this.signer = deps.signer;
    this.notPaused = deps.notPaused;
    this.fetchFn = deps.fetchFn ?? fetch;
  }

  /** Resolve the destination gateway by spoke id; throws if not registered. */
  resolveGateway(spokeOut: string): string {
    const spoke = this.registry.get(spokeOut);
    if (!spoke) {
      throw new Error(`unknown spoke_out: "${spokeOut}" not registered`);
    }
    return spoke.gatewayUrl.replace(/\/$/, "");
  }

  handleBridgeOut = async (req: Request, res: Response): Promise<void> => {
    const provided = req.headers["x-relay-auth"];
    if (!provided || provided !== this.relayAuthSecret) {
      res.status(401).json({ error: "X-Relay-Auth invalid or missing" });
      return;
    }

    const payload = req.body as Partial<CrossCurrencyBridgeOutPayload>;
    if (
      !payload.correlation_id ||
      !payload.swap_tx_hash ||
      !payload.amount_out ||
      !payload.beneficiary_bank_id ||
      !payload.spoke_out
    ) {
      res.status(400).json({
        error:
          "correlation_id, swap_tx_hash, amount_out, beneficiary_bank_id, spoke_out are required",
      });
      return;
    }

    // Circuit breaker (Constitution III): read isPaused() on-chain on amm_address.
    if (!(await this.notPaused(payload.amm_address))) {
      console.warn(
        JSON.stringify({
          ts: new Date().toISOString(),
          service: "cacti-relay",
          severity: "WARN",
          event: "bridge_out_refused",
          reason: "circuit_breaker",
          correlation_id: payload.correlation_id,
          spoke_out: payload.spoke_out,
        }),
      );
      res
        .status(409)
        .json({ error: "circuit breaker: pair paused or unavailable (fail-safe)" });
      return;
    }

    // Route by spoke_out → gateway lookup.
    let gatewayUrl: string;
    try {
      gatewayUrl = this.resolveGateway(payload.spoke_out);
    } catch (err) {
      res.status(400).json({ error: (err as Error).message });
      return;
    }

    try {
      await this.forward(gatewayUrl, payload as CrossCurrencyBridgeOutPayload);
      res.json({ status: "accepted", correlation_id: payload.correlation_id });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error(`[CrossCurrencySwapRelay] forward failed: ${msg}`);
      res.status(502).json({ error: `failed to forward to spoke gateway: ${msg}` });
    }
  };

  private async forward(
    gatewayUrl: string,
    payload: CrossCurrencyBridgeOutPayload,
  ): Promise<void> {
    const path = "/internal/amm/cross-currency-bridge-out";
    const url = `${gatewayUrl}${path}`;
    // Serialized ONCE. The signature covers these exact bytes, so serializing again for the request
    // body is how a mismatch gets introduced — the receiver would reject every call.
    const body = JSON.stringify(payload);
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      // Kept alongside the signature for the migration window: a CB that has not pinned the relay
      // yet still accepts the call, instead of the bridge-out leg failing closed.
      "X-Relay-Auth": this.relayAuthSecret,
    };
    if (this.signer) {
      Object.assign(headers, this.signer.headersFor("POST", path, body));
    }
    const resp = await this.fetchFn(url, { method: "POST", headers, body });
    if (!resp.ok) {
      const body = await resp.text().catch(() => "");
      throw new Error(`HTTP ${resp.status} from ${url}: ${body}`);
    }
    console.log(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: "cacti-relay",
        severity: "INFO",
        event: "bridge_out_forwarded",
        correlation_id: payload.correlation_id,
        spoke_out: payload.spoke_out,
      }),
    );
  }
}

/**
 * Factory: the registry and the isPaused gate are injected by index.ts (they are
 * runtime state). Only the shared auth secret comes from env here.
 */
export function createCrossCurrencySwapRelay(
  registry: SpokeRegistry,
  notPaused: NotPausedGate,
): CrossCurrencySwapRelay | null {
  const relayAuthSecret = process.env["INTERNAL_RELAY_AUTH_SECRET"] ?? "";
  if (!relayAuthSecret) {
    console.error(
      "[CrossCurrencySwapRelay] INTERNAL_RELAY_AUTH_SECRET not set — refusing to start (security)",
    );
    return null;
  }
  // The relay's own service identity. Optional on purpose: a deployment that has not provisioned it
  // keeps working on the shared secret, which is the migration state. It is required only once a CB
  // sets RELAY_REQUIRE_SIGNATURE — and that CB refuses to start unless it has pinned peers, so the
  // two ends cannot be enabled out of order without the operator being told.
  const keyId = process.env["RELAY_KEY_ID"] ?? "cacti-relay";
  const signer = RelaySigner.fromOptional(keyId, process.env["RELAY_SIGNING_KEY_FILE"]);
  if (signer) {
    console.log(`[CrossCurrencySwapRelay] per-entity signature enabled (key-id=${keyId})`);
  } else {
    console.log(
      "[CrossCurrencySwapRelay] no signing key configured (RELAY_SIGNING_KEY_FILE) — forwarding with the shared secret only",
    );
  }
  return new CrossCurrencySwapRelay({ registry, relayAuthSecret, notPaused, signer });
}
