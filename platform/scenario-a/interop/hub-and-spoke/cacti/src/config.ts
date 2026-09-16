// SPDX-License-Identifier: Apache-2.0

/**
 * Configuration loader for the Cacti HTLC relay service.
 * All required environment variables are validated at startup; missing values
 * cause an immediate fatal error with a clear message.
 */

import { loadSpokesConfig } from "./spokes-config";

export interface SpokeConfig {
  /** Unique identifier for this spoke (e.g. "spoke-a"). */
  id: string;
  /** HTTP JSON-RPC URL for the Besu node (e.g. "http://host:8645"). */
  besuRpc: string;
  /** WebSocket JSON-RPC URL for the Besu node (e.g. "ws://host:8655"). */
  besuWs: string;
  /** Deployed HTLC contract address on this spoke (0x-prefixed). */
  htlcAddress: string;
  /** Internal api-gateway URL for polling FX agreements (e.g. "http://host:18080"). */
  internalApiUrl: string;
  /**
   * gRPC target for this spoke's payment-orchestrator — the founding central bank's (e.g.
   * "host:19094"). Used to forward FX AGREEMENT actions, which are spoke-level objects in a
   * shared Pente group, so reaching the CB reaches the spoke.
   *
   * Not a settlement path. An HTLC leg is settled by its owner, via transferLocked on the
   * owner's own Paladin node; the relay journals the claim and the owner acts on it.
   */
  grpcEndpoint: string;
}

function requireEnv(key: string): string {
  const val = process.env[key];
  if (!val) {
    throw new Error(`Fatal: required environment variable "${key}" is not set`);
  }
  return val;
}

function optionalEnv(key: string, fallback: string): string {
  return process.env[key] ?? fallback;
}

export const config = {
  // Spoke registry — loaded from CACTI_SPOKES_CONFIG YAML or legacy SPOKE_A/SPOKE_B env vars.
  spokes: loadSpokesConfig(console),

  /** Port for the Cacti REST API server (default: 4000). */
  apiPort: parseInt(optionalEnv("CACTI_API_PORT", "4000"), 10),

  /** How often to poll each Besu node for new HTLC events (default: 3000 ms). */
  pollIntervalMs: parseInt(optionalEnv("POLL_INTERVAL_MS", "3000"), 10),

  /** Shared secret sent to internal FX endpoints via X-Relay-Auth header. */
  relayAuthSecret: requireEnv("INTERNAL_RELAY_AUTH_SECRET"),

  /** Comma-separated allowed origins for Socket.IO browser connections. Empty = block all browser cross-origin requests. */
  socketIoAllowedOrigins: optionalEnv("SOCKET_IO_ALLOWED_ORIGINS", "").split(",").filter(Boolean),

  /** JSON file path used to persist relay dedup/retry state across restarts. */
  relayStorePath: optionalEnv("RELAY_STORE_PATH", "/tmp/cacti-relay-store.json"),
  /**
   * Entries retained per journal kind before the oldest are dropped.
   *
   * A retention policy, so it belongs in configuration: a network with heavier traffic, or one
   * whose entities can be down longer, needs more headroom, and until now changing it meant
   * editing a constant and rebuilding. Dropping an entry a consumer has not read loses a
   * settlement — the destination leg's owner is the only party that can settle it, and this
   * journal is how it finds out — so the relay records what it dropped and the consumer reports
   * it (X-Relay-Journal-Trimmed-Through). Raise this rather than rely on that report.
   */
  journalMaxEntries: parseInt(optionalEnv("JOURNAL_MAX_ENTRIES", "10000"), 10),

  /** JSON file path used to persist the dynamic spoke registry (RL-1) across restarts. */
  spokeRegistryPath: optionalEnv("CACTI_SPOKES_REGISTRY", "/data/cacti-spoke-registry.json"),

  /**
   * Absolute path to the payment-orchestrator proto file.
   * In the Docker image this is copied to /app/apis/proto/…
   */
  protoPath: optionalEnv(
    "PROTO_PATH",
    "/app/apis/proto/payment_orchestrator/v1/payment_orchestrator.proto",
  ),
};
