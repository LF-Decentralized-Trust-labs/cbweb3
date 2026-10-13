// SPDX-License-Identifier: Apache-2.0

/**
 * Persisted spoke store (TK-B5). A durable JSON file on the relay volume, loaded
 * at boot and saved on every registration so registered spokes survive restarts.
 * Replaces the legacy flat `cacti-relay-store.json`.
 */

import { randomBytes } from "crypto";
import { promises as fs } from "fs";
import * as path from "path";
import { Spoke } from "./spoke-registry";

interface StoreShape {
  spokes: Spoke[];
}

export class RelayStore {
  /** Tail of the write chain: each save() waits for the previous one, in order. */
  private writeQueue: Promise<void> = Promise.resolve();
  private writeSeq = 0;
  /** Per-INSTANCE temp-file suffix, so two instances on one path never share a temp name. */
  private readonly writeTag = randomBytes(6).toString("hex");

  constructor(private readonly filePath: string) {}

  /** Load persisted spokes; a missing file yields [] (no error). */
  async load(): Promise<Spoke[]> {
    try {
      const buf = await fs.readFile(this.filePath, "utf8");
      const parsed = JSON.parse(buf) as StoreShape;
      return Array.isArray(parsed?.spokes) ? parsed.spokes : [];
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === "ENOENT") return [];
      throw err;
    }
  }

  /**
   * Atomically persist the spoke set (write tmp + rename), one write at a time. Spokes register at
   * runtime, so two saves can be in flight together; with a shared temp name the second rename
   * found no file and threw. The queue keeps the order of the calls, so the last save wins.
   */
  save(spokes: Spoke[]): Promise<void> {
    const write = async (): Promise<void> => {
      await fs.mkdir(path.dirname(this.filePath), { recursive: true });
      const tmp = `${this.filePath}.${this.writeTag}.${this.writeSeq++}.tmp`;
      try {
        await fs.writeFile(tmp, JSON.stringify({ spokes } satisfies StoreShape, null, 2));
        await fs.rename(tmp, this.filePath);
      } catch (err) {
        await fs.rm(tmp, { force: true }).catch(() => undefined);
        throw err;
      }
    };
    const result = this.writeQueue.then(write, write);
    this.writeQueue = result.then(() => undefined, () => undefined);
    return result;
  }
}
