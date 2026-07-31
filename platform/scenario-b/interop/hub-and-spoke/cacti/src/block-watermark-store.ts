// SPDX-License-Identifier: Apache-2.0

import { promises as fs } from "fs";
import path from "path";

/**
 * BlockWatermarkStore persists the watcher's durable state to a single JSON file on the
 * relay volume so a watcher resumes exactly where it left off after a restart instead of
 * resetting to its configured start block and re-scanning (or skipping the gap). Finding
 * R2-H-11.
 *
 * It holds three maps in one file so a single atomic write keeps them consistent:
 *   - `blocks`    — last processed block per watcher key (the watermark).
 *   - `delivered` — set of already-delivered event keys (`txHash:logIndex`) so a re-scan
 *                   after a crash-before-watermark-advance does not re-forward an event.
 *   - `meta`      — small per-key strings; used to record the chain's genesis-block hash so
 *                   a chain reset (fresh genesis, stale watermark on the volume) is detected
 *                   instead of stalling the watcher silently.
 *
 * Writes are atomic (temp file + rename) so a torn write can never corrupt the file and send
 * init() into the empty-state fallback, wiping the delivered set. Self-contained to Scenario B
 * — it must not be shared with Scenario A per the project constitution (scenario isolation).
 */

interface WatermarkState {
  blocks: Record<string, number>;
  delivered: Record<string, number>;
  meta: Record<string, string>;
}

const EMPTY: WatermarkState = { blocks: {}, delivered: {}, meta: {} };

export class BlockWatermarkStore {
  private state: WatermarkState = { blocks: {}, delivered: {}, meta: {} };

  constructor(
    private readonly filePath: string,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  async init(): Promise<void> {
    try {
      await fs.mkdir(path.dirname(this.filePath), { recursive: true });
      const raw = await fs.readFile(this.filePath, "utf8");
      const parsed = JSON.parse(raw) as Partial<WatermarkState> & Record<string, unknown>;
      this.state = this.hydrate(parsed);
      this.log.info(
        `[block-watermark-store] loaded blocks=${Object.keys(this.state.blocks).length} ` +
        `delivered=${Object.keys(this.state.delivered).length}`,
      );
    } catch (err) {
      this.log.warn(`[block-watermark-store] starting with empty state: ${String(err)}`);
      this.state = { blocks: {}, delivered: {}, meta: {} };
      await this.persist();
    }
  }

  /**
   * Accept both the current shape and the legacy flat `Record<string, number>` (block-only)
   * shape so an existing on-volume file keeps its watermark across the upgrade.
   */
  private hydrate(parsed: Partial<WatermarkState> & Record<string, unknown>): WatermarkState {
    if (parsed && typeof parsed === "object" && "blocks" in parsed) {
      return {
        blocks: parsed.blocks ?? {},
        delivered: parsed.delivered ?? {},
        meta: parsed.meta ?? {},
      };
    }
    // Legacy: the whole object was the block map.
    const blocks: Record<string, number> = {};
    for (const [k, v] of Object.entries(parsed ?? {})) {
      if (typeof v === "number") blocks[k] = v;
    }
    return { blocks, delivered: {}, meta: {} };
  }

  /** Last processed block for key, or undefined if none recorded. */
  get(key: string): number | undefined {
    return this.state.blocks[key];
  }

  /** Record the last processed block for key and persist it. */
  async set(key: string, block: number): Promise<void> {
    this.state.blocks[key] = block;
    await this.persist();
  }

  /** True when an event key (`txHash:logIndex`) has already been delivered. */
  hasDelivered(eventKey: string): boolean {
    return this.state.delivered[eventKey] !== undefined;
  }

  /** Mark an event key delivered and persist it. */
  async markDelivered(eventKey: string, ts = 0): Promise<void> {
    this.state.delivered[eventKey] = ts;
    await this.persist();
  }

  /** Read a metadata string for key (e.g. the chain genesis hash), or undefined. */
  getMeta(key: string): string | undefined {
    return this.state.meta[key];
  }

  /** Record a metadata string for key and persist it. */
  async setMeta(key: string, value: string): Promise<void> {
    this.state.meta[key] = value;
    await this.persist();
  }

  /**
   * Reset the watermark for key back to `block` (e.g. after a chain reset). Also drops the
   * delivered set — the previous chain's event keys are meaningless on the new chain — and
   * records the new chain identity under meta. Persisted atomically.
   */
  async resetChain(key: string, block: number, genesisHash: string): Promise<void> {
    this.state.blocks[key] = block;
    this.state.delivered = {};
    this.state.meta[key] = genesisHash;
    await this.persist();
  }

  private async persist(): Promise<void> {
    const tmp = `${this.filePath}.tmp`;
    await fs.writeFile(tmp, JSON.stringify(this.state), "utf8");
    await fs.rename(tmp, this.filePath);
  }
}

// Re-exported for tests that want the empty shape.
export const EMPTY_WATERMARK_STATE = EMPTY;
