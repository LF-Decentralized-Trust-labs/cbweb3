// SPDX-License-Identifier: Apache-2.0

/**
 * Configuration loader for the generalized Cacti relay (Scenario B, TK-B5).
 *
 * The relay boots NEUTRAL: with zero spokes and no fatal SPOKE_A/B_BESU_RPC.
 * Spokes come from the persisted RelayStore (hydrated at boot) and/or the
 * optional SPOKES_JSON env, and are added at runtime via POST /api/v1/spokes.
 */

import { Spoke } from "./spoke-registry";

function optionalEnv(key: string, fallback: string, env = process.env): string {
  return env[key] ?? fallback;
}

/** Validate one spoke object parsed from external input (SPOKES_JSON / API). */
export function validateSpoke(value: unknown): Spoke {
  const v = value as Record<string, unknown>;
  for (const key of ["spokeId", "besuRpc", "besuWs", "gatewayUrl"]) {
    if (typeof v?.[key] !== "string" || (v[key] as string).trim() === "") {
      throw new Error(`invalid spoke: "${key}" is required`);
    }
  }
  return {
    spokeId: v.spokeId as string,
    besuRpc: v.besuRpc as string,
    besuWs: v.besuWs as string,
    gatewayUrl: v.gatewayUrl as string,
  };
}

/**
 * Load the optional seed list of spokes from SPOKES_JSON (a JSON array). Returns
 * [] when unset/empty — boot neutro. Never throws on missing (only on malformed).
 */
export function loadSpokesFromEnv(env = process.env): Spoke[] {
  const raw = env.SPOKES_JSON;
  if (!raw || raw.trim() === "") return [];
  const parsed: unknown = JSON.parse(raw);
  if (!Array.isArray(parsed)) {
    throw new Error("SPOKES_JSON must be a JSON array");
  }
  return parsed.map(validateSpoke);
}

export const config = {
  /** Port for the Cacti REST API server (default: 4000). */
  apiPort: parseInt(optionalEnv("CACTI_API_PORT", "4000"), 10),
};
