// SPDX-License-Identifier: Apache-2.0

import { test } from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "fs";
import * as os from "os";
import * as path from "path";
import type { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";

import { BlockWatermarkStore } from "./block-watermark-store";
import { LiquidityCommitWatcher } from "./liquidity-commit-watcher";

const CONTRACT = "0x00000000000000000000000000000000000000aa";

// Minimal fake Cacti connector: reports a fixed latest block and no logs. It also
// records the fromBlock of the first getPastLogs call so the test can assert the
// watcher resumed from the persisted watermark rather than the start block.
function fakeConnector(latestBlock: number, capture: { firstFromBlock?: string }) {
  return {
    async getBlock() {
      return { block: { number: latestBlock } };
    },
    async getPastLogs(args: { fromBlock: string }) {
      if (capture.firstFromBlock === undefined) {
        capture.firstFromBlock = args.fromBlock;
      }
      return { logs: [] };
    },
  } as unknown as PluginLedgerConnectorBesu;
}

function sleep(ms: number): Promise<void> {
  return new Promise(r => setTimeout(r, ms));
}

async function tmpFile(): Promise<string> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "lcr-watcher-resume-"));
  return path.join(dir, "store.json");
}

test("LiquidityCommitWatcher resumes from the persisted block instead of startBlock", async () => {
  const file = await tmpFile();

  // Persist a watermark as if a previous process had processed up to block 500.
  const seed = new BlockWatermarkStore(file);
  await seed.init();
  await seed.set(CONTRACT, 500);

  const capture: { firstFromBlock?: string } = {};
  const controller = new AbortController();
  const watcher = new LiquidityCommitWatcher({
    contractAddress: CONTRACT,
    gatewayUrls: ["http://gateway:1/"],
    relayAuthSecret: "secret",
    pollIntervalMs: 5,
    startBlock: 0, // deliberately different from the persisted watermark
    connector: fakeConnector(600, capture),
    watermarkStore: new BlockWatermarkStore(file),
  });

  watcher.start(controller.signal);
  // Allow one poll cycle to run.
  await sleep(80);
  controller.abort();
  watcher.stop();
  await sleep(20);

  // The first getPastLogs must scan from persisted+1 (0x1f5 = 501), not from startBlock+1 (1).
  assert.equal(capture.firstFromBlock, "0x" + (501).toString(16));

  // After processing up to the latest block, the watermark must be persisted.
  const reloaded = new BlockWatermarkStore(file);
  await reloaded.init();
  assert.equal(reloaded.get(CONTRACT), 600);
});
