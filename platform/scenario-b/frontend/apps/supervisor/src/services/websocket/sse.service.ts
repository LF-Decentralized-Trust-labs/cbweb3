import type { AuditLogsResponse, SupervisorEvent } from "../../types";
import { apiFetch } from "../api/apiClient";

type EventSubscriber = (event: SupervisorEvent) => void;

interface CircuitBreakerStatus {
  state: string;
  last_toggled_at?: string;
  reason?: string;
}

interface AMMPoolStatus {
  pool_pair: string;
  pool_status: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
  imbalance_flag: boolean;
  current_ratio: number;
}

function auditSeverityToEvent(s: string): SupervisorEvent["severity"] {
  if (s === "ERROR" || s === "CRITICAL") return "HIGH";
  if (s === "WARN") return "MEDIUM";
  return "LOW";
}

const POLL_INTERVAL_MS = 30_000;
const AUDIT_FETCH_LIMIT = 5;
const KNOWN_POOL_PAIR = "W-BRL-ARS";

class PollingEventService {
  private timer: ReturnType<typeof setInterval> | null = null;
  private subscribers = new Set<EventSubscriber>();

  private lastCBState: string | null = null;
  private lastPoolStatus: string | null = null;
  private lastPoolImbalanced: boolean | null = null;
  private seenAuditIds = new Set<string>();

  connect() {
    if (this.timer) return;
    void this.poll(true);
    this.timer = setInterval(() => void this.poll(false), POLL_INTERVAL_MS);
  }

  disconnect() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  subscribe(subscriber: EventSubscriber) {
    this.subscribers.add(subscriber);
    return () => {
      this.subscribers.delete(subscriber);
    };
  }

  private emit(event: SupervisorEvent) {
    this.subscribers.forEach((sub) => sub(event));
  }

  private async poll(isInit: boolean) {
    const [cbResult, poolResult, auditResult] = await Promise.allSettled([
      apiFetch<CircuitBreakerStatus>("/api/v2/governance/circuit-breaker/status"),
      apiFetch<AMMPoolStatus>(`/api/v2/amm/pool/${KNOWN_POOL_PAIR}/status`),
      apiFetch<AuditLogsResponse>(`/api/v1/compliance/audit/logs?limit=${AUDIT_FETCH_LIMIT}`),
    ]);

    if (isInit) {
      // Seed state without emitting — avoids surfacing pre-existing conditions as new events
      if (cbResult.status === "fulfilled") this.lastCBState = cbResult.value.state;
      if (poolResult.status === "fulfilled") {
        this.lastPoolStatus = poolResult.value.pool_status;
        this.lastPoolImbalanced = poolResult.value.imbalance_flag;
      }
      if (auditResult.status === "fulfilled") {
        for (const log of auditResult.value.logs) this.seenAuditIds.add(log.log_id);
      }
      return;
    }

    // Circuit breaker state change
    if (cbResult.status === "fulfilled") {
      const cb = cbResult.value;
      if (this.lastCBState !== null && cb.state !== this.lastCBState) {
        this.emit({
          id: `cb_${Date.now()}`,
          type: "CIRCUIT_BREAKER",
          severity: cb.state === "PAUSED" ? "CRITICAL" : "LOW",
          message: `Circuit breaker: ${this.lastCBState} → ${cb.state}${cb.reason ? ` · ${cb.reason}` : ""}`,
          createdAt: cb.last_toggled_at ?? new Date().toISOString(),
        });
      }
      this.lastCBState = cb.state;
    }

    // Pool imbalance or status change
    if (poolResult.status === "fulfilled") {
      const p = poolResult.value;
      if (p.pool_status !== "EMPTY") {
        if (this.lastPoolImbalanced !== null && p.imbalance_flag !== this.lastPoolImbalanced) {
          this.emit({
            id: `pool_imb_${Date.now()}`,
            type: "POOL_IMBALANCE",
            severity: p.imbalance_flag ? "HIGH" : "LOW",
            message: p.imbalance_flag
              ? `Pool ${p.pool_pair} became imbalanced (ratio ${p.current_ratio.toFixed(4)})`
              : `Pool ${p.pool_pair} returned to balanced state`,
            createdAt: new Date().toISOString(),
          });
        }
        if (this.lastPoolStatus !== null && p.pool_status !== this.lastPoolStatus) {
          this.emit({
            id: `pool_st_${Date.now()}`,
            type: "POOL_IMBALANCE",
            severity: "MEDIUM",
            message: `Pool ${p.pool_pair} status: ${this.lastPoolStatus} → ${p.pool_status}`,
            createdAt: new Date().toISOString(),
          });
        }
      }
      this.lastPoolStatus = p.pool_status;
      this.lastPoolImbalanced = p.imbalance_flag;
    }

    // New audit log entries since last poll
    if (auditResult.status === "fulfilled") {
      for (const log of auditResult.value.logs) {
        if (this.seenAuditIds.has(log.log_id)) continue;
        this.seenAuditIds.add(log.log_id);
        this.emit({
          id: `audit_${log.log_id}`,
          type: "AUDIT_ACTIVITY",
          severity: auditSeverityToEvent(log.severity),
          message: `[${log.category}] ${log.actor}: ${log.action}${log.target_subject ? ` → ${log.target_subject}` : ""} (${log.outcome})`,
          createdAt: log.timestamp,
        });
      }
    }
  }
}

export const sseService = new PollingEventService();
