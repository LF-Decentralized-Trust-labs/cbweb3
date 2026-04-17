import { promises as fs } from "fs";
import path from "path";

export type FXAction = "propose" | "accept" | "reject" | "cancel" | "settle";

export interface RelayRetryItem {
  key: string;
  action: FXAction;
  spokeName: string;
  tradeId: string;
  payload?: Record<string, unknown>;
  attemptCount: number;
  nextAttemptAt: number;
  lastError?: string;
  updatedAt: number;
}

interface RelayStoreState {
  delivered: Record<string, number>;
  retries: RelayRetryItem[];
}

const DEFAULT_STATE: RelayStoreState = { delivered: {}, retries: [] };

export class RelayStore {
  private state: RelayStoreState = { ...DEFAULT_STATE };

  constructor(
    private readonly filePath: string,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  async init(): Promise<void> {
    try {
      await fs.mkdir(path.dirname(this.filePath), { recursive: true });
      const raw = await fs.readFile(this.filePath, "utf8");
      const parsed = JSON.parse(raw) as RelayStoreState;
      this.state = {
        delivered: parsed.delivered ?? {},
        retries: parsed.retries ?? [],
      };
      this.log.info(
        `[relay-store] loaded delivered=${Object.keys(this.state.delivered).length} retries=${this.state.retries.length}`,
      );
    } catch (err) {
      this.log.warn(`[relay-store] starting with empty state: ${String(err)}`);
      this.state = { ...DEFAULT_STATE };
      await this.persist();
    }
  }

  hasDelivered(key: string): boolean {
    return this.state.delivered[key] !== undefined;
  }

  async markDelivered(key: string): Promise<void> {
    this.state.delivered[key] = Date.now();
    this.state.retries = this.state.retries.filter((r) => r.key !== key);
    await this.persist();
  }

  async scheduleRetry(
    key: string,
    action: FXAction,
    spokeName: string,
    tradeId: string,
    error: string,
    payload?: Record<string, unknown>,
  ): Promise<void> {
    const now = Date.now();
    const existing = this.state.retries.find((r) => r.key === key);
    if (!existing) {
      this.state.retries.push({
        key,
        action,
        spokeName,
        tradeId,
        payload,
        attemptCount: 1,
        nextAttemptAt: now + this.computeBackoffMs(1),
        lastError: error,
        updatedAt: now,
      });
    } else {
      existing.attemptCount += 1;
      existing.nextAttemptAt = now + this.computeBackoffMs(existing.attemptCount);
      existing.lastError = error;
      existing.updatedAt = now;
      if (payload) {
        existing.payload = payload;
      }
    }
    await this.persist();
  }

  getDueRetries(spokeName: string, now = Date.now()): RelayRetryItem[] {
    return this.state.retries.filter(
      (r) => r.spokeName === spokeName && r.nextAttemptAt <= now,
    );
  }

  getRetryStats(now = Date.now()): { pending: number; maxLagMs: number } {
    let maxLagMs = 0;
    for (const r of this.state.retries) {
      if (r.nextAttemptAt < now) {
        maxLagMs = Math.max(maxLagMs, now - r.nextAttemptAt);
      }
    }
    return { pending: this.state.retries.length, maxLagMs };
  }

  private computeBackoffMs(attempt: number): number {
    const base = 5_000;
    const max = 5 * 60_000;
    return Math.min(max, base * 2 ** Math.max(0, attempt - 1));
  }

  private async persist(): Promise<void> {
    await fs.writeFile(this.filePath, JSON.stringify(this.state), "utf8");
  }
}
