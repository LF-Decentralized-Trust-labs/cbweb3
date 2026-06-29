// SPDX-License-Identifier: Apache-2.0

/**
 * RL-1 — dynamic spoke registry.
 *
 * The relay no longer requires every spoke to be known at startup. A founding
 * central bank registers its freshly-provisioned spoke via
 *   POST /api/v1/spokes
 * and the toolkit's relay registrar later confirms it via
 *   GET  /api/v1/spokes/:id
 *
 * Registrations are persisted to a JSON file so they survive relay restarts.
 * This module owns ONLY the registry (discovery) concern; turning a registered
 * spoke into an actively-polled one still depends on complete connection details
 * (besuWs, grpcEndpoint, internalApiUrl) in the relay's poll configuration.
 */

import { promises as fs } from "fs";
import path from "path";

export interface RegisteredSpoke {
  id: string;
  besuRpc: string;
  besuWs: string;
  htlcAddress: string;
  internalApiUrl: string;
  grpcEndpoint: string;
  /** Unix epoch milliseconds when the spoke was (re)registered. */
  registeredAt: number;
}

/** Thrown when a registration body is missing a required field. */
export class SpokeRegistrationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SpokeRegistrationError";
  }
}

/**
 * normalizeSpokeRegistration maps a registration request body to a RegisteredSpoke.
 * It accepts both the snake_case shape sent by the Go orchestrator's relay
 * registrar (spoke_id, besu_rpc_url, besu_ws_url, htlc_address, grpc_endpoint)
 * and a camelCase shape. `id` and `besuRpc` are required; other fields default
 * to "" so a partial registration (as emitted by the found register-relay step)
 * is accepted.
 */
export function normalizeSpokeRegistration(body: Record<string, unknown>): RegisteredSpoke {
  const pick = (...keys: string[]): string => {
    for (const k of keys) {
      const v = body[k];
      if (typeof v === "string" && v.length > 0) return v;
    }
    return "";
  };

  const id = pick("id", "spoke_id", "spokeId");
  const besuRpc = pick("besuRpc", "besu_rpc_url", "besuRpcUrl");
  if (!id) {
    throw new SpokeRegistrationError("spoke id is required (id / spoke_id)");
  }
  if (!besuRpc) {
    throw new SpokeRegistrationError("besuRpc is required (besuRpc / besu_rpc_url)");
  }

  return {
    id,
    besuRpc,
    besuWs: pick("besuWs", "besu_ws_url", "besuWsUrl"),
    htlcAddress: pick("htlcAddress", "htlc_address"),
    internalApiUrl: pick("internalApiUrl", "internal_api_url"),
    grpcEndpoint: pick("grpcEndpoint", "grpc_endpoint"),
    registeredAt: Date.now(),
  };
}

interface RegistryState {
  spokes: Record<string, RegisteredSpoke>;
}

export class SpokeRegistry {
  private state: RegistryState = { spokes: {} };

  constructor(
    private readonly filePath: string,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  /** load reads the persisted registry; a missing/corrupt file starts empty. */
  async load(): Promise<void> {
    try {
      const raw = await fs.readFile(this.filePath, "utf8");
      const parsed = JSON.parse(raw) as RegistryState;
      this.state = { spokes: parsed.spokes ?? {} };
      this.log.info(`[spoke-registry] loaded ${Object.keys(this.state.spokes).length} spoke(s)`);
    } catch {
      this.state = { spokes: {} };
    }
  }

  list(): RegisteredSpoke[] {
    return Object.values(this.state.spokes);
  }

  get(id: string): RegisteredSpoke | undefined {
    return this.state.spokes[id];
  }

  has(id: string): boolean {
    return this.state.spokes[id] !== undefined;
  }

  /** register upserts the spoke by id and persists the registry. */
  async register(spoke: RegisteredSpoke): Promise<RegisteredSpoke> {
    this.state.spokes[spoke.id] = spoke;
    await this.persist();
    return spoke;
  }

  private async persist(): Promise<void> {
    await fs.mkdir(path.dirname(this.filePath), { recursive: true });
    await fs.writeFile(this.filePath, JSON.stringify(this.state), "utf8");
  }
}
