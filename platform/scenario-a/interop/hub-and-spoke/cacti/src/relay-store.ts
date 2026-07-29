// SPDX-License-Identifier: Apache-2.0

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

/** A durable, sequenced record of one served relay event (finding R2-H-11 — cursor composition). */
export interface JournalEntry {
  /** Monotonic, gap-tolerant sequence assigned once by the relay and never reused. */
  seq: number;
  /** Stable chain identity `${spoke}:${txHash}:${logIndex}` — dedup key across re-decodes. */
  id: string;
  /** The event payload served over REST (shape defined by the relay accessor). */
  event: Record<string, unknown>;
}

export type JournalKind = "lock" | "settle";

interface RelayStoreState {
  delivered: Record<string, number>;
  retries: RelayRetryItem[];
  // Last processed block per spoke id. Lets the poller resume after a restart
  // instead of starting at the chain head and skipping events mined during
  // downtime (finding R2-H-11).
  watermarks: Record<string, number>;
  // Small per-spoke strings. Used to record the spoke chain's genesis-block hash so a
  // chain reset (fresh genesis, stale watermark on the volume) is detected instead of the
  // relay stalling silently with head below watermark (finding R2-H-11).
  meta: Record<string, string>;
  // Durable, monotonically-sequenced journal of lock/settle events served to the Go poller.
  // Replaces the old in-memory ring + wall-clock-ms cursor whose coordinates did not survive a
  // relay restart: on re-decode the ring reassigned fresh timestamps (duplicate delivery) and a
  // dropped ring lost events below the block watermark (silent loss). The journal gives each
  // event a stable seq (assigned once, deduped by chain id) that survives restart, so the Go
  // poller's persisted seq composes exactly with the relay (finding R2-H-11, cursor composition).
  journal: { lock: JournalEntry[]; settle: JournalEntry[] };
  // Next sequence to assign. Monotonic across restarts (never reset except on a fresh store).
  seq: number;
}

/** Fresh empty state. A factory — NOT a shared literal — so instances never alias each other's maps/arrays. */
function emptyState(): RelayStoreState {
  return { delivered: {}, retries: [], watermarks: {}, meta: {}, journal: { lock: [], settle: [] }, seq: 1 };
}

/** Max journal entries retained per kind. Older entries are trimmed; a consumer lagging beyond
 * this many events would miss the trimmed tail (logged when it happens). */
const MAX_JOURNAL = 10_000;

export class RelayStore {
  private state: RelayStoreState = emptyState();

  constructor(
    private readonly filePath: string,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  async init(): Promise<void> {
    try {
      await fs.mkdir(path.dirname(this.filePath), { recursive: true });
      const raw = await fs.readFile(this.filePath, "utf8");
      const parsed = JSON.parse(raw) as Partial<RelayStoreState>;
      const journal = parsed.journal ?? { lock: [], settle: [] };
      this.state = {
        delivered: parsed.delivered ?? {},
        retries: parsed.retries ?? [],
        watermarks: parsed.watermarks ?? {},
        meta: parsed.meta ?? {},
        journal: { lock: journal.lock ?? [], settle: journal.settle ?? [] },
        // Resume seq monotonically. On a legacy file (no seq) start past whatever the journal
        // already holds so a reused seq can never collide with a delivered one.
        seq: parsed.seq ?? (maxSeq(journal.lock) > maxSeq(journal.settle) ? maxSeq(journal.lock) : maxSeq(journal.settle)) + 1,
      };
      this.log.info(
        `[relay-store] loaded delivered=${Object.keys(this.state.delivered).length} retries=${this.state.retries.length} watermarks=${Object.keys(this.state.watermarks).length} journal=${this.state.journal.lock.length}+${this.state.journal.settle.length} nextSeq=${this.state.seq}`,
      );
    } catch (err) {
      this.log.warn(`[relay-store] starting with empty state: ${String(err)}`);
      this.state = emptyState();
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

  /**
   * Last processed block for a spoke, or undefined if none has been recorded.
   * The poller uses this to resume after a restart (finding R2-H-11).
   */
  getWatermark(spokeId: string): number | undefined {
    return this.state.watermarks[spokeId];
  }

  /** Record the last processed block for a spoke and persist it. */
  async setWatermark(spokeId: string, block: number): Promise<void> {
    this.state.watermarks[spokeId] = block;
    await this.persist();
  }

  /** Chain-identity string previously recorded for a spoke (its genesis hash), or undefined. */
  getMeta(spokeId: string): string | undefined {
    return this.state.meta[spokeId];
  }

  /** Record a chain-identity string for a spoke and persist it. */
  async setMeta(spokeId: string, value: string): Promise<void> {
    this.state.meta[spokeId] = value;
    await this.persist();
  }

  /**
   * Reset a spoke after a detected chain reset: rewind its watermark to `block`, record the new
   * genesis hash, and drop that spoke's HTLC dedup keys — both the per-event guard
   * (`htlc-evt:${spokeId}:`) and the echo guard (`htlc-settled:${spokeId}:`) — since the previous
   * chain's tx hashes and contract ids are meaningless on the new chain. FX delivered keys and
   * other spokes are untouched. Persisted atomically (finding R2-H-11).
   */
  async resetSpokeChain(spokeId: string, block: number, genesisHash: string): Promise<void> {
    this.state.watermarks[spokeId] = block;
    this.state.meta[spokeId] = genesisHash;
    const prefixes = [`htlc-evt:${spokeId}:`, `htlc-settled:${spokeId}:`];
    for (const key of Object.keys(this.state.delivered)) {
      if (prefixes.some((p) => key.startsWith(p))) delete this.state.delivered[key];
    }
    await this.persist();
  }

  /**
   * Append an observed lock/settle event to the durable journal and return the seq assigned to it.
   * Idempotent by chain identity `id`: if the same event was already journaled (e.g. re-decoded
   * after a restart), returns the existing seq WITHOUT appending a duplicate — this is what lets
   * the Go poller's persisted seq suppress re-delivery (finding R2-H-11). Persisted atomically.
   */
  async appendEvent(kind: JournalKind, id: string, event: Record<string, unknown>): Promise<number> {
    const entries = this.state.journal[kind];
    const existing = entries.find((e) => e.id === id);
    if (existing) {
      return existing.seq;
    }
    const seq = this.state.seq++;
    entries.push({ seq, id, event: { ...event, seq } });
    if (entries.length > MAX_JOURNAL) {
      const dropped = entries.splice(0, entries.length - MAX_JOURNAL);
      this.log.warn(
        `[relay-store] journal '${kind}' trimmed ${dropped.length} oldest entries (cap ${MAX_JOURNAL}); a consumer lagging past this will miss them`,
      );
    }
    await this.persist();
    return seq;
  }

  /**
   * Return journaled events of a kind with seq strictly greater than sinceSeq, in seq order.
   * The returned payloads carry their `seq` so the consumer can advance its cursor. Read-only,
   * synchronous (serves the REST poll off in-memory state).
   */
  getEventsSince(kind: JournalKind, sinceSeq: number): Record<string, unknown>[] {
    return this.state.journal[kind]
      .filter((e) => e.seq > sinceSeq)
      .sort((a, b) => a.seq - b.seq)
      .map((e) => e.event);
  }

  private computeBackoffMs(attempt: number): number {
    const base = 5_000;
    const max = 5 * 60_000;
    return Math.min(max, base * 2 ** Math.max(0, attempt - 1));
  }

  private async persist(): Promise<void> {
    // Atomic write: a torn in-place write would send init() into the empty-state fallback,
    // wiping the delivered map and retry queue. Write to a temp file then rename (finding R2-H-11).
    const tmp = `${this.filePath}.tmp`;
    await fs.writeFile(tmp, JSON.stringify(this.state), "utf8");
    await fs.rename(tmp, this.filePath);
  }
}

/** Highest seq present in a journal slice (0 when empty). Used to resume seq on a legacy file. */
function maxSeq(entries: JournalEntry[] | undefined): number {
  let m = 0;
  for (const e of entries ?? []) {
    if (e.seq > m) m = e.seq;
  }
  return m;
}
