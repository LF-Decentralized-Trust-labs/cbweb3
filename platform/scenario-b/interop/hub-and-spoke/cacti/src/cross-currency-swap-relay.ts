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
  registry: SpokeRegistry;
  relayAuthSecret: string;
  notPaused: NotPausedGate;
  /** Injectable fetch for tests (defaults to global fetch). */
  fetchFn?: typeof fetch;
}

export class CrossCurrencySwapRelay {
  private readonly registry: SpokeRegistry;
  private readonly relayAuthSecret: string;
  private readonly notPaused: NotPausedGate;
  private readonly fetchFn: typeof fetch;

  constructor(deps: CrossCurrencyRelayDeps) {
    this.registry = deps.registry;
    this.relayAuthSecret = deps.relayAuthSecret;
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
    const url = `${gatewayUrl}/internal/amm/cross-currency-bridge-out`;
    const resp = await this.fetchFn(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Relay-Auth": this.relayAuthSecret,
      },
      body: JSON.stringify(payload),
    });
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
  return new CrossCurrencySwapRelay({ registry, relayAuthSecret, notPaused });
}
