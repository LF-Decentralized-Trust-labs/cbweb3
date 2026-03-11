import type {
  AuditLogEntry,
  InfrastructureNode,
  LoginResponse,
  PoolStatus,
  RelayStatus,
  SysAdminUser,
  TelemetryFrame,
  TopologyEdge,
  TopologyNode,
} from "../../types";
import { generateTelemetryFrame } from "./data-generators";

const nowIso = () => new Date().toISOString();
const wait = async (ms = 250) => new Promise((resolve) => setTimeout(resolve, ms));

const currentUser: SysAdminUser = {
  id: "noc-admin-1",
  name: "NOC Administrator",
  role: "SYS_ADMIN",
  institutionId: "central-ops-bra",
};

const infrastructure: InfrastructureNode[] = [
  { id: "besu-a-1", network: "Network A", region: "BR-SP", component: "BESU", uptimePct: 99.95, syncLagBlocks: 0, status: "HEALTHY", updatedAt: nowIso() },
  { id: "besu-b-1", network: "Network B", region: "BR-RJ", component: "BESU", uptimePct: 99.72, syncLagBlocks: 2, status: "DEGRADED", updatedAt: nowIso() },
  { id: "paladin-a", network: "Network A", region: "BR-SP", component: "PALADIN", uptimePct: 99.88, syncLagBlocks: 0, status: "HEALTHY", updatedAt: nowIso() },
  { id: "paladin-b", network: "Network B", region: "BR-RJ", component: "PALADIN", uptimePct: 99.31, syncLagBlocks: 1, status: "DEGRADED", updatedAt: nowIso() },
];

const relays: RelayStatus[] = [
  { id: "relay-a-hub", route: "Network A ↔ Hub", latencyP50Ms: 82, latencyP95Ms: 168, proofSuccessRatePct: 99.4, status: "HEALTHY", updatedAt: nowIso() },
  { id: "relay-b-hub", route: "Network B ↔ Hub", latencyP50Ms: 95, latencyP95Ms: 241, proofSuccessRatePct: 97.6, status: "DEGRADED", updatedAt: nowIso() },
  { id: "relay-a-b", route: "Network A ↔ Network B", latencyP50Ms: 112, latencyP95Ms: 284, proofSuccessRatePct: 96.8, status: "DEGRADED", updatedAt: nowIso() },
];

const pools: PoolStatus[] = [
  { pair: "BRL-tCeBM/USD-tCeBM", reserveA: "720000", reserveB: "280000", ratioA: 72, ratioB: 28, breached7030: true, severity: "CRITICAL", updatedAt: nowIso() },
  { pair: "BRL-tCeBM/EUR-tCeBM", reserveA: "580000", reserveB: "420000", ratioA: 58, ratioB: 42, breached7030: false, severity: "INFO", updatedAt: nowIso() },
];

const topologyNodes: TopologyNode[] = [
  { id: "hub", label: "Regional Hub", kind: "HUB", redundant: true, status: "HEALTHY" },
  { id: "besu-a", label: "Besu A", kind: "BESU", redundant: true, status: "HEALTHY" },
  { id: "besu-b", label: "Besu B", kind: "BESU", redundant: false, status: "DEGRADED" },
  { id: "paladin-a", label: "Paladin A", kind: "PALADIN", redundant: true, status: "HEALTHY" },
  { id: "cacti", label: "Cacti Relay", kind: "CACTI", redundant: false, status: "DEGRADED" },
];

const topologyEdges: TopologyEdge[] = [
  { id: "e1", from: "besu-a", to: "hub", healthy: true },
  { id: "e2", from: "besu-b", to: "hub", healthy: true },
  { id: "e3", from: "paladin-a", to: "besu-a", healthy: true },
  { id: "e4", from: "cacti", to: "hub", healthy: false },
];

const auditLogs: AuditLogEntry[] = [
  { id: "audit-1", component: "SYSTEM", severity: "INFO", message: "NOC monitoring cycle started", createdAt: nowIso() },
  { id: "audit-2", component: "CACTI", severity: "WARNING", message: "Relay latency exceeded warning threshold", createdAt: nowIso() },
  { id: "audit-3", component: "BESU", severity: "CRITICAL", message: "Node sync lag crossed critical threshold", createdAt: nowIso() },
];

export const mockDb = {
  async login(username: string, password: string): Promise<LoginResponse> {
    await wait();
    if (!username || !password) {
      throw new Error("Invalid credentials");
    }
    return { user: currentUser };
  },
  async me(): Promise<LoginResponse> {
    await wait(100);
    return { user: currentUser };
  },
  async logout() {
    await wait(100);
    return { ok: true };
  },
  async getInfrastructure() {
    await wait(180);
    return infrastructure;
  },
  async getRelays() {
    await wait(180);
    return relays;
  },
  async getPools() {
    await wait(180);
    return pools;
  },
  async getTopology() {
    await wait(180);
    return { nodes: topologyNodes, edges: topologyEdges };
  },
  async getAuditLogs() {
    await wait(180);
    return auditLogs;
  },
  async getTelemetrySnapshot(): Promise<TelemetryFrame[]> {
    await wait(100);
    return [
      generateTelemetryFrame("BESU", "besu-a-1"),
      generateTelemetryFrame("BESU", "besu-b-1"),
      generateTelemetryFrame("PALADIN", "paladin-a"),
      generateTelemetryFrame("CACTI", "relay-a-hub"),
    ];
  },
};
