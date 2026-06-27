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
  /** gRPC target for this spoke's payment-orchestrator (e.g. "host:19094"). Each spoke stores its own endpoint; the relay routes by dest_spoke_id. */
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
   * Absolute path to the payment-orchestrator proto file.
   * In the Docker image this is copied to /app/apis/proto/…
   */
  protoPath: optionalEnv(
    "PROTO_PATH",
    "/app/apis/proto/payment_orchestrator/v1/payment_orchestrator.proto",
  ),
};
