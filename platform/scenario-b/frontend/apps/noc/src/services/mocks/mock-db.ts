// SPDX-License-Identifier: Apache-2.0

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
  roles: ["ROLE_NOC_ADMIN"],
  role: "SYS_ADMIN",
  institutionId: "central-ops-bra",
};

const infrastructure: InfrastructureNode[] = [
  { id: "besu-hub-1", network: "Regional Hub", region: "HUB-SA", component: "BESU", uptimePct: 99.99, syncLagBlocks: 0, status: "HEALTHY", updatedAt: nowIso() },
  { id: "paladin-hub", network: "Regional Hub", region: "HUB-SA", component: "PALADIN", uptimePct: 99.97, syncLagBlocks: 0, status: "HEALTHY", updatedAt: nowIso() },
  { id: "besu-a-1", network: "Spoke A", region: "BR-SP", component: "BESU", uptimePct: 99.95, syncLagBlocks: 0, status: "HEALTHY", updatedAt: nowIso() },
  { id: "besu-b-1", network: "Spoke B", region: "BR-RJ", component: "BESU", uptimePct: 99.72, syncLagBlocks: 2, status: "DEGRADED", updatedAt: nowIso() },
  { id: "paladin-a", network: "Spoke A", region: "BR-SP", component: "PALADIN", uptimePct: 99.88, syncLagBlocks: 0, status: "HEALTHY", updatedAt: nowIso() },
  { id: "paladin-b", network: "Spoke B", region: "BR-RJ", component: "PALADIN", uptimePct: 99.31, syncLagBlocks: 1, status: "DEGRADED", updatedAt: nowIso() },
];

const relays: RelayStatus[] = [
  { id: "relay-a-hub", route: "Spoke A ↔ Regional Hub", latencyP50Ms: 82, latencyP95Ms: 168, proofSuccessRatePct: 99.4, status: "HEALTHY", updatedAt: nowIso() },
  { id: "relay-b-hub", route: "Spoke B ↔ Regional Hub", latencyP50Ms: 95, latencyP95Ms: 241, proofSuccessRatePct: 97.6, status: "DEGRADED", updatedAt: nowIso() },
];

const pools: PoolStatus[] = [
  { pair: "BRL-tCeBM/ARS-tCeBM", reserveA: "720000", reserveB: "280000", ratioA: 72, ratioB: 28, breached7030: true, severity: "CRITICAL", updatedAt: nowIso() },
  { pair: "BRL-tCeBM/EUR-tCeBM", reserveA: "580000", reserveB: "420000", ratioA: 58, ratioB: 42, breached7030: false, severity: "INFO", updatedAt: nowIso() },
];

const topologyNodes: TopologyNode[] = [
  { id: "hub", label: "Regional Hub (Besu)", kind: "HUB", spoke_id: "hub", spoke_name: "Regional Hub", redundant: true, status: "HEALTHY" },
  { id: "paladin-hub", label: "Paladin Hub", kind: "PALADIN", spoke_id: "hub", spoke_name: "Regional Hub", redundant: true, status: "HEALTHY" },
  { id: "besu-a", label: "Besu Spoke A", kind: "BESU", spoke_id: "spoke-a", spoke_name: "Spoke A", redundant: true, status: "HEALTHY" },
  { id: "paladin-a", label: "Paladin Spoke A", kind: "PALADIN", spoke_id: "spoke-a", spoke_name: "Spoke A", redundant: true, status: "HEALTHY" },
  { id: "cacti-a", label: "Cacti Relay A↔Hub", kind: "CACTI", spoke_id: "spoke-a", spoke_name: "Spoke A", redundant: false, status: "HEALTHY" },
  { id: "besu-b", label: "Besu Spoke B", kind: "BESU", spoke_id: "spoke-b", spoke_name: "Spoke B", redundant: false, status: "DEGRADED" },
  { id: "paladin-b", label: "Paladin Spoke B", kind: "PALADIN", spoke_id: "spoke-b", spoke_name: "Spoke B", redundant: false, status: "DEGRADED" },
  { id: "cacti-b", label: "Cacti Relay B↔Hub", kind: "CACTI", spoke_id: "spoke-b", spoke_name: "Spoke B", redundant: false, status: "DEGRADED" },
];

const topologyEdges: TopologyEdge[] = [
  { id: "e1", from: "besu-a", to: "hub", healthy: true },
  { id: "e2", from: "paladin-a", to: "besu-a", healthy: true },
  { id: "e3", from: "cacti-a", to: "hub", healthy: true },
  { id: "e4", from: "paladin-hub", to: "hub", healthy: true },
  { id: "e5", from: "besu-b", to: "hub", healthy: false },
  { id: "e6", from: "paladin-b", to: "besu-b", healthy: true },
  { id: "e7", from: "cacti-b", to: "hub", healthy: false },
];

const auditLogs: AuditLogEntry[] = [
  { id: "audit-1", actor: "system", action: "MONITORING_CYCLE_START", target_id: "noc", target_type: "SYSTEM", detail: "NOC monitoring cycle started", created_at: nowIso() },
  { id: "audit-2", actor: "system", action: "RELAY_LATENCY_WARNING", target_id: "cacti", target_type: "CACTI", detail: "Relay latency exceeded warning threshold", created_at: nowIso() },
  { id: "audit-3", actor: "system", action: "NODE_SYNC_LAG_CRITICAL", target_id: "besu", target_type: "BESU", detail: "Node sync lag crossed critical threshold", created_at: nowIso() },
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
