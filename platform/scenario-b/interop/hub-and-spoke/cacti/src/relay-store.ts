// SPDX-License-Identifier: Apache-2.0

/**
 * Persisted spoke store (TK-B5). A durable JSON file on the relay volume, loaded
 * at boot and saved on every registration so registered spokes survive restarts.
 * Replaces the legacy flat `cacti-relay-store.json`.
 */

import { promises as fs } from "fs";
import * as path from "path";
import { Spoke } from "./spoke-registry";

interface StoreShape {
  spokes: Spoke[];
}

export class RelayStore {
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

  /** Atomically persist the spoke set (write tmp + rename). */
  async save(spokes: Spoke[]): Promise<void> {
    await fs.mkdir(path.dirname(this.filePath), { recursive: true });
    const tmp = `${this.filePath}.tmp`;
    await fs.writeFile(tmp, JSON.stringify({ spokes } satisfies StoreShape, null, 2));
    await fs.rename(tmp, this.filePath);
  }
}
