// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from "node:fs";
import * as yaml from "js-yaml";
import type { SpokeConfig } from "./config";

// ---------------------------------------------------------------------------
// T007 — loadSpokesFromYaml
// ---------------------------------------------------------------------------

export function loadSpokesFromYaml(filePath: string): unknown[] {
  let raw: string;
  try {
    raw = readFileSync(filePath, "utf8");
  } catch {
    throw new Error(`Fatal: cannot read spokes config: ${filePath}: no such file`);
  }
  const doc = yaml.load(raw) as Record<string, unknown> | null;
  const spokes = doc?.["spokes"];
  if (!Array.isArray(spokes)) {
    throw new Error("Fatal: at least one spoke must be configured");
  }
  return spokes;
}

// ---------------------------------------------------------------------------
// T008 — validateSpokesConfig
// ---------------------------------------------------------------------------

const REQUIRED_FIELDS = ["id", "besuRpc", "besuWs", "htlcAddress", "internalApiUrl", "grpcEndpoint"] as const;

export function validateSpokesConfig(spokes: unknown[]): SpokeConfig[] {
  if (spokes.length === 0) {
    throw new Error("Fatal: at least one spoke must be configured");
  }

  const seen = new Set<string>();
  const result: SpokeConfig[] = [];

  for (let i = 0; i < spokes.length; i++) {
    const entry = spokes[i] as Record<string, unknown>;
    for (const field of REQUIRED_FIELDS) {
      if (!entry[field]) {
        throw new Error(`Fatal: spoke[${i}].${field} is required`);
      }
    }
    const id = entry["id"] as string;
    if (seen.has(id)) {
      throw new Error(`Fatal: duplicate spoke id "${id}"`);
    }
    seen.add(id);
    result.push({
      id,
      besuRpc: entry["besuRpc"] as string,
      besuWs: entry["besuWs"] as string,
      htlcAddress: entry["htlcAddress"] as string,
      internalApiUrl: entry["internalApiUrl"] as string,
      grpcEndpoint: entry["grpcEndpoint"] as string,
    });
  }

  return result;
}

// ---------------------------------------------------------------------------
// T009 — buildLegacyShim
// ---------------------------------------------------------------------------

function requireLegacyEnv(key: string): string {
  const val = process.env[key];
  if (!val) throw new Error(`Fatal: ${key} is required`);
  return val;
}

function optionalLegacyEnv(key: string, fallback: string): string {
  return process.env[key] ?? fallback;
}

export function buildLegacyShim(log: Pick<Console, "warn">): SpokeConfig[] {
  const spokeABesuRpc = requireLegacyEnv("SPOKE_A_BESU_RPC");
  const spokeABesuWs = optionalLegacyEnv("SPOKE_A_BESU_WS", "ws://localhost:8655");
  const spokeAHtlc = requireLegacyEnv("SPOKE_A_HTLC_ADDRESS");
  const spokeAApi = requireLegacyEnv("SPOKE_A_INTERNAL_API");
  const spokeAGrpc = requireLegacyEnv("SPOKE_A_PAYMENT_GRPC");

  const spokeBBesuRpc = requireLegacyEnv("SPOKE_B_BESU_RPC");
  const spokeBBesuWs = optionalLegacyEnv("SPOKE_B_BESU_WS", "ws://localhost:8755");
  const spokeBHtlc = requireLegacyEnv("SPOKE_B_HTLC_ADDRESS");
  const spokeBApi = requireLegacyEnv("SPOKE_B_INTERNAL_API");
  const spokeBGrpc = requireLegacyEnv("SPOKE_B_PAYMENT_GRPC");

  log.warn(
    "[relay] DEPRECATION: SPOKE_A_*/SPOKE_B_* env vars are deprecated — use CACTI_SPOKES_CONFIG instead",
  );

  return [
    {
      id: "spoke-a",
      besuRpc: spokeABesuRpc,
      besuWs: spokeABesuWs,
      htlcAddress: spokeAHtlc,
      internalApiUrl: spokeAApi,
      grpcEndpoint: spokeAGrpc,
    },
    {
      id: "spoke-b",
      besuRpc: spokeBBesuRpc,
      besuWs: spokeBBesuWs,
      htlcAddress: spokeBHtlc,
      internalApiUrl: spokeBApi,
      grpcEndpoint: spokeBGrpc,
    },
  ];
}

// ---------------------------------------------------------------------------
// T010 — loadSpokesConfig
// ---------------------------------------------------------------------------

export function loadSpokesConfig(log: Pick<Console, "warn">): SpokeConfig[] {
  const yamlPath = process.env["CACTI_SPOKES_CONFIG"];
  if (yamlPath) {
    const raw = loadSpokesFromYaml(yamlPath);
    return validateSpokesConfig(raw);
  }

  if (process.env["SPOKE_A_BESU_RPC"] && process.env["SPOKE_B_BESU_RPC"]) {
    return buildLegacyShim(log);
  }

  // No static config: the relay is driven purely by the dynamic spoke registry
  // (POST /api/v1/spokes). This is the normal path — founding central banks register their
  // spokes at deploy time and watchers are reconciled from the persisted registry on startup.
  log.warn(
    "[relay] no static spoke config (CACTI_SPOKES_CONFIG / SPOKE_A_*+SPOKE_B_*) — " +
      "watchers will be driven entirely by the dynamic /api/v1/spokes registry",
  );
  return [];
}
