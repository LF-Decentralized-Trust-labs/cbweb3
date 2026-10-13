// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "fs";
import * as os from "os";
import * as path from "path";
import { RelayStore } from "./relay-store";
import { Spoke } from "./spoke-registry";

const mk = (id: string): Spoke => ({
  spokeId: id, besuRpc: "http://h", besuWs: "ws://h", gatewayUrl: "http://gw",
});

async function tmpFile(): Promise<string> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "relay-store-"));
  return path.join(dir, "spokes.json");
}

test("load returns [] when file is absent", async () => {
  const fp = await tmpFile();
  assert.deepEqual(await new RelayStore(fp).load(), []);
});

test("save then load round-trips the spoke set", async () => {
  const fp = await tmpFile();
  const store = new RelayStore(fp);
  await store.save([mk("spoke-a"), mk("spoke-b")]);
  const loaded = await store.load();
  assert.equal(loaded.length, 2);
  assert.deepEqual(loaded.map((s) => s.spokeId).sort(), ["spoke-a", "spoke-b"]);
});

test("save is atomic (no leftover temp file; file present)", async () => {
  const fp = await tmpFile();
  await new RelayStore(fp).save([mk("x")]);
  assert.ok(await fs.stat(fp)); // final file exists
  // Read the directory rather than stat one predicted name: a fixed `${fp}.tmp` check passes no
  // matter what is left behind once temp names carry a per-instance suffix.
  const leftovers = (await fs.readdir(path.dirname(fp))).filter(f => f !== path.basename(fp));
  assert.deepEqual(leftovers, []);
});

// Spokes register at runtime through POST /api/v1/spokes, so two registrations can reach save()
// together. A fixed temp name with no serialisation made the second rename find no temp file and
// throw ENOENT into the request handler.
test("concurrent saves all succeed and the last one wins", async () => {
  const fp = await tmpFile();
  const store = new RelayStore(fp);

  const failures: string[] = [];
  await Promise.all(Array.from({ length: 40 }, (_, i) =>
    store.save([mk(`spoke-${i}`)]).catch((e: Error) => failures.push(e.message))));

  assert.deepEqual(failures, []);
  const loaded = await new RelayStore(fp).load();
  assert.deepEqual(loaded.map(s => s.spokeId), ["spoke-39"], "the last write to be queued is the one on disk");
});

test("two RelayStore instances on the same file do not collide", async () => {
  const fp = await tmpFile();
  await new RelayStore(fp).save([mk("seed")]);
  const a = new RelayStore(fp);
  const b = new RelayStore(fp);

  const failures: string[] = [];
  await Promise.all(Array.from({ length: 40 }, (_, i) =>
    (i % 2 ? a : b).save([mk(`spoke-${i}`)]).catch((e: Error) => failures.push(e.message))));

  assert.deepEqual(failures, []);
});

// A write that fails part-way (a full disk) leaves a partly written temp file. Temp names are
// unique per write, so nothing later overwrites it: unless the failure path removes it, each
// failed save during a disk-pressure event leaves a full copy of the spoke set behind.
test("save removes its temp file when a write fails part-way, and the store keeps working", async () => {
  const fp = await tmpFile();
  const store = new RelayStore(fp);

  const realWriteFile = fs.writeFile.bind(fs);
  const original = fs.writeFile;
  (fs as { writeFile: unknown }).writeFile = async (...args: unknown[]) => {
    await (realWriteFile as (...a: unknown[]) => Promise<void>)(...args); // the partial file exists …
    throw new Error("ENOSPC: no space left on device");                    // … then the write fails
  };
  try {
    await assert.rejects(() => store.save([mk("x")]), /ENOSPC/);
  } finally {
    (fs as { writeFile: unknown }).writeFile = original;
  }

  const leftovers = (await fs.readdir(path.dirname(fp))).filter(f => f !== path.basename(fp));
  assert.deepEqual(leftovers, [], "a failed write must not leave its temp file behind");
  await store.save([mk("y")]);
  assert.deepEqual((await store.load()).map(s => s.spokeId), ["y"]);
});
