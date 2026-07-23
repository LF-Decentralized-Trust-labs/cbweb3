// SPDX-License-Identifier: Apache-2.0

import { promises as fs } from "fs";
import path from "path";

/**
 * BlockWatermarkStore persists the last processed block per watcher key to a JSON
 * file so a watcher resumes after a restart instead of resetting to its configured
 * start block and re-scanning (or, worse, skipping the gap between the persisted
 * position and the start block). Finding R2-H-11.
 *
 * Self-contained to Scenario B — it must not be shared with Scenario A per the
 * project constitution (scenario isolation).
 */
export class BlockWatermarkStore {
  private state: Record<string, number> = {};

  constructor(
    private readonly filePath: string,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  async init(): Promise<void> {
    try {
      await fs.mkdir(path.dirname(this.filePath), { recursive: true });
      const raw = await fs.readFile(this.filePath, "utf8");
      const parsed = JSON.parse(raw) as Record<string, number>;
      this.state = parsed ?? {};
      this.log.info(
        `[block-watermark-store] loaded watermarks=${Object.keys(this.state).length}`,
      );
    } catch (err) {
      this.log.warn(`[block-watermark-store] starting with empty state: ${String(err)}`);
      this.state = {};
      await this.persist();
    }
  }

  /** Last processed block for key, or undefined if none recorded. */
  get(key: string): number | undefined {
    return this.state[key];
  }

  /** Record the last processed block for key and persist it. */
  async set(key: string, block: number): Promise<void> {
    this.state[key] = block;
    await this.persist();
  }

  private async persist(): Promise<void> {
    await fs.writeFile(this.filePath, JSON.stringify(this.state), "utf8");
  }
}
