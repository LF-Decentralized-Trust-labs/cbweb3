/**
 * Configuration loader for the Cacti HTLC relay service.
 * All required environment variables are validated at startup; missing values
 * cause an immediate fatal error with a clear message.
 */

export interface SpokeConfig {
  /** Human-readable name used in logs (e.g. "spoke-a"). */
  name: string;
  /** HTTP JSON-RPC URL for the Besu node (e.g. "http://host:8645"). */
  besuRpc: string;
  /** Deployed HTLC contract address on this spoke (0x-prefixed). */
  htlcAddress: string;
  /** Internal api-gateway URL for polling FX agreements (e.g. "http://host:18080"). */
  internalApiUrl: string;
  /** gRPC target for the counterpart spoke's payment-orchestrator (e.g. "host:29094"). */
  counterpartGrpc: string;
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
    htlcAddress: requireEnv("SPOKE_A_HTLC_ADDRESS"),
    internalApiUrl: requireEnv("SPOKE_A_INTERNAL_API"),
    counterpartGrpc: requireEnv("SPOKE_B_PAYMENT_GRPC"),
  } satisfies SpokeConfig,

  spokeB: {
    name: "spoke-b",
    besuRpc: requireEnv("SPOKE_B_BESU_RPC"),
    htlcAddress: requireEnv("SPOKE_B_HTLC_ADDRESS"),
    internalApiUrl: requireEnv("SPOKE_B_INTERNAL_API"),
    counterpartGrpc: requireEnv("SPOKE_A_PAYMENT_GRPC"),
  } satisfies SpokeConfig,

  /** Port for the Cacti REST API server (default: 4000). */
  apiPort: parseInt(optionalEnv("CACTI_API_PORT", "4000"), 10),

  /** How often to poll each Besu node for new HTLC events (default: 3000 ms). */
  pollIntervalMs: parseInt(optionalEnv("POLL_INTERVAL_MS", "3000"), 10),

  /**
   * Absolute path to the payment-orchestrator proto file.
   * In the Docker image this is copied to /app/apis/proto/…
   */
  protoPath: optionalEnv(
    "PROTO_PATH",
    "/app/apis/proto/payment_orchestrator/v1/payment_orchestrator.proto",
  ),
};
