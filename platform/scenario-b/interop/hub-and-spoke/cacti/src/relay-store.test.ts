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

test("save is atomic (no leftover .tmp; file present)", async () => {
  const fp = await tmpFile();
  await new RelayStore(fp).save([mk("x")]);
  assert.ok(await fs.stat(fp)); // final file exists
  await assert.rejects(() => fs.stat(`${fp}.tmp`)); // tmp gone (renamed)
});
