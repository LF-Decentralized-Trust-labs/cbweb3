/**
 * Configuration loader for the Cacti Liquidity Relay service (Scenario B).
 * All required environment variables are validated at startup; missing values
 * cause an immediate fatal error with a clear message.
 */

export interface SpokeConfig {
  /** Human-readable name used in logs (e.g. "spoke-a"). */
  name: string;
  /** HTTP JSON-RPC URL for the Besu node (e.g. "http://host:8645"). */
  besuRpc: string;
  /** WebSocket JSON-RPC URL for the Besu node (e.g. "ws://host:8655"). */
  besuWs: string;
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
  spokeA: {
    name: "spoke-a",
    besuRpc: requireEnv("SPOKE_A_BESU_RPC"),
    besuWs: optionalEnv("SPOKE_A_BESU_WS", "ws://localhost:8655"),
  } satisfies SpokeConfig,

  spokeB: {
    name: "spoke-b",
    besuRpc: requireEnv("SPOKE_B_BESU_RPC"),
    besuWs: optionalEnv("SPOKE_B_BESU_WS", "ws://localhost:8755"),
  } satisfies SpokeConfig,

  /** Port for the Cacti REST API server (default: 4000). */
  apiPort: parseInt(optionalEnv("CACTI_API_PORT", "4000"), 10),
};
