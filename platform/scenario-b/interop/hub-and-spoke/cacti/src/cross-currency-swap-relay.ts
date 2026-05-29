/**
 * CrossCurrencySwapRelay — Cacti module for 009-commercial-cross-currency-swap.
 *
 * Receives a bridge-out notification from CB-A after the Hub AMM swap and
 * forwards it to the target CB gateway so that CB-B can:
 *   1. burn the W-ARS tokens on the Hub
 *   2. release (mint) tCeBM-ARS to Bank-B on Spoke-B
 *
 * The relay acts as the neutral cross-chain message bus: CB-A does not need to
 * know CB-B's internal URL; CB-B does not expose its URL to CB-A directly.
 *
 * Environment variables consumed:
 *   CB_B_GATEWAY_URL          — Internal URL of CB-B api-gateway
 *                               (e.g. "http://api-gateway-central-bank-b:8080")
 *   INTERNAL_RELAY_AUTH_SECRET — Shared secret for X-Relay-Auth header
 *
 * Endpoint registered in index.ts:
 *   POST /api/v1/cross-currency/bridge-out
 */

import { Request, Response } from "express";

// ---------------------------------------------------------------------------
// Payload types
// ---------------------------------------------------------------------------

export interface CrossCurrencyBridgeOutPayload {
  /** UUID correlating bridge-in + swap + bridge-out across CBs. */
  correlation_id: string;
  /** On-chain tx hash of the Hub AMM swap (proof the swap happened). */
  swap_tx_hash: string;
  /** Sovereign pool pair identifier (e.g. "W-BRL-ARS"). */
  pool_pair: string;
  /** Amount in target currency (wei decimal string). */
  amount_out: string;
  /** ID of the commercial bank that should receive the funds (e.g. "bank-b"). */
  beneficiary_bank_id: string;
  /** Target spoke network (e.g. "spoke-b"). */
  spoke_out: string;
  /** ERC-20 address of the wrapped target token on the Hub (e.g. W-ARS address). */
  wrapped_target_token: string;
  /**
   * Hub address that received W-ARS from the AMM swap (CB-A's signer).
   * CB-B's executor burns from this address — it has CENTRAL_BANK_ROLE which
   * grants burn authority over any address.
   */
  swap_sender_address?: string;
  // Note: beneficiary_spoke_address is intentionally absent from this payload.
  // CB-B resolves the beneficiary on-chain address internally from its participants registry
  // using beneficiary_bank_id — the frontend/CB-A never needs to know on-chain addresses of peers.
}

// ---------------------------------------------------------------------------
// CrossCurrencySwapRelay
// ---------------------------------------------------------------------------

export class CrossCurrencySwapRelay {
  private readonly cbBGatewayUrl: string;
  private readonly relayAuthSecret: string;

  constructor(opts: { cbBGatewayUrl: string; relayAuthSecret: string }) {
    this.cbBGatewayUrl = opts.cbBGatewayUrl.replace(/\/$/, "");
    this.relayAuthSecret = opts.relayAuthSecret;
  }

  /**
   * Express request handler for POST /api/v1/cross-currency/bridge-out.
   * Validates X-Relay-Auth, parses payload, and forwards to CB-B.
   */
  handleBridgeOut = async (req: Request, res: Response): Promise<void> => {
    // Auth validation
    const provided = req.headers["x-relay-auth"];
    if (!provided || provided !== this.relayAuthSecret) {
      res.status(401).json({ error: "X-Relay-Auth invalid or missing" });
      return;
    }

    const payload = req.body as Partial<CrossCurrencyBridgeOutPayload>;

    // Validate required fields
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

    console.log(
      `[CrossCurrencySwapRelay] bridge-out request — correlation_id=${payload.correlation_id} ` +
        `amount_out=${payload.amount_out} beneficiary=${payload.beneficiary_bank_id} spoke=${payload.spoke_out}`,
    );

    try {
      await this.forwardToCBB(payload as CrossCurrencyBridgeOutPayload);
      res.json({ status: "accepted", correlation_id: payload.correlation_id });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error(`[CrossCurrencySwapRelay] forward to CB-B failed: ${msg}`);
      res.status(502).json({ error: `failed to forward to CB-B gateway: ${msg}` });
    }
  };

  private async forwardToCBB(payload: CrossCurrencyBridgeOutPayload): Promise<void> {
    const url = `${this.cbBGatewayUrl}/internal/amm/cross-currency-bridge-out`;
    const resp = await fetch(url, {
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

    const json = (await resp.json().catch(() => ({}))) as Record<string, unknown>;
    console.log(
      `[CrossCurrencySwapRelay] CB-B accepted bridge-out — ` +
        `correlation_id=${payload.correlation_id} position_id=${json["position_id"] ?? "unknown"}`,
    );
  }
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function createCrossCurrencySwapRelayFromEnv(): CrossCurrencySwapRelay | null {
  const cbBGatewayUrl = process.env["CB_B_GATEWAY_URL"] ?? "";
  const relayAuthSecret = process.env["INTERNAL_RELAY_AUTH_SECRET"] ?? "";

  if (!cbBGatewayUrl) {
    console.log(
      "[CrossCurrencySwapRelay] CB_B_GATEWAY_URL not set — cross-currency relay disabled",
    );
    return null;
  }
  if (!relayAuthSecret) {
    console.error(
      "[CrossCurrencySwapRelay] INTERNAL_RELAY_AUTH_SECRET not set — refusing to start (security)",
    );
    return null;
  }

  return new CrossCurrencySwapRelay({ cbBGatewayUrl, relayAuthSecret });
}
