// SPDX-License-Identifier: Apache-2.0

import { test, mock } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import type { Socket } from "node:net";
import { createHash, randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { ClosingBesuConnector, sharedBesuPrometheusExporter } from "./closing-besu-connector";

// A Besu WebSocket slot is held until the client closes its socket. Cacti's connector-besu
// 2.0.0 opens a WebSocket in its constructor and its shutdown() only writes a log line, so a
// stopped spoke runtime would keep its slot on Besu for the life of the process.
//
// These tests run the REAL connector against a WebSocket server that counts open sockets, so
// what they assert is what Besu would see: a slot taken, and a slot given back.

// Minimal RFC 6455 server: completes the handshake, tracks each upgraded socket until it closes,
// and ends the socket when the client sends a close frame.
function startCountingWsServer(): Promise<{ url: string; open: () => number; stop: () => Promise<void> }> {
  const sockets = new Set<Socket>();
  const server = http.createServer();
  server.on("upgrade", (req, socket: Socket) => {
    const key = req.headers["sec-websocket-key"];
    const accept = createHash("sha1").update(`${key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`).digest("base64");
    socket.write(
      "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
        `Sec-WebSocket-Accept: ${accept}\r\n\r\n`,
    );
    sockets.add(socket);
    socket.on("data", (buf: Buffer) => {
      if ((buf[0] & 0x0f) === 0x8) socket.end(); // close frame
    });
    socket.on("close", () => sockets.delete(socket));
    socket.on("error", () => sockets.delete(socket));
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address() as { port: number };
      resolve({
        url: `ws://127.0.0.1:${port}`,
        open: () => sockets.size,
        stop: () =>
          new Promise<void>((done) => {
            for (const s of sockets) s.destroy();
            server.close(() => done());
          }),
      });
    });
  });
}

async function waitFor(cond: () => boolean, ms: number): Promise<boolean> {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (cond()) return true;
    await new Promise((r) => setTimeout(r, 20));
  }
  return cond();
}

function options(wsUrl: string) {
  return {
    instanceId: `test-${randomUUID()}`,
    rpcApiHttpHost: "http://127.0.0.1:1",
    rpcApiWsHost: wsUrl,
    pluginRegistry: new PluginRegistry(),
    logLevel: "ERROR" as const,
  };
}

// Pins the library defect: if this starts failing after a connector-besu upgrade, upstream
// shutdown() closes the socket and ClosingBesuConnector can be deleted. It also proves the
// harness sees an open socket, so the tests below are not vacuous.
test("the library connector keeps its socket open after shutdown()", async () => {
  const server = await startCountingWsServer();
  try {
    const connector = new PluginLedgerConnectorBesu(options(server.url));
    await connector.onPluginInit();
    assert.ok(await waitFor(() => server.open() === 1, 3_000));
    await connector.shutdown();
    await new Promise((r) => setTimeout(r, 500));
    assert.equal(server.open(), 1);
  } finally {
    await server.stop();
  }
});

test("ClosingBesuConnector closes its socket on shutdown(), and closing twice is harmless", async () => {
  const server = await startCountingWsServer();
  try {
    const connector = new ClosingBesuConnector(options(server.url));
    await connector.onPluginInit();
    assert.ok(await waitFor(() => server.open() === 1, 3_000));
    await connector.shutdown();
    await connector.shutdown();
    assert.ok(await waitFor(() => server.open() === 0, 3_000));
  } finally {
    await server.stop();
  }
});

// The fix reaches into a private field of a third-party class. If an upgrade renames it, the
// failure must be loud: a shutdown that quietly does nothing is the original leak again.
test("ClosingBesuConnector fails loudly when the provider can no longer be reached", async () => {
  const server = await startCountingWsServer();
  try {
    const connector = new ClosingBesuConnector(options(server.url));
    (connector as unknown as { web3Provider: unknown }).web3Provider = {};
    await assert.rejects(connector.shutdown(), /WebSocket/);
  } finally {
    await server.stop();
  }
});

// The socket is opened by the constructor, before onPluginInit() runs.
test("ClosingBesuConnector.open closes the socket when initialisation fails, then rethrows", async () => {
  const server = await startCountingWsServer();
  const init = mock.method(PluginLedgerConnectorBesu.prototype, "onPluginInit", async () => {
    await waitFor(() => server.open() === 1, 3_000);
    throw new Error("init boom");
  });
  try {
    await assert.rejects(ClosingBesuConnector.open(options(server.url)), /init boom/);
    assert.ok(await waitFor(() => server.open() === 0, 3_000));
  } finally {
    init.mock.restore();
    await server.stop();
  }
});

// The library starts prom-client's default metrics per connector, and they never stop.
test("every ClosingBesuConnector shares one exporter and starts no new metrics", async () => {
  const promClient = createRequire(require.resolve("@hyperledger/cactus-plugin-ledger-connector-besu"))(
    "prom-client",
  ) as { collectDefaultMetrics: (...args: unknown[]) => void };
  const server = await startCountingWsServer();
  const built: ClosingBesuConnector[] = [new ClosingBesuConnector(options(server.url))];
  const spy = mock.method(promClient, "collectDefaultMetrics");
  try {
    for (let i = 0; i < 5; i++) built.push(new ClosingBesuConnector(options(server.url)));
    assert.equal(spy.mock.callCount(), 0);
    for (const c of built) assert.equal(c.getPrometheusExporter(), sharedBesuPrometheusExporter());
  } finally {
    spy.mock.restore();
    await Promise.allSettled(built.map((c) => c.shutdown()));
    await server.stop();
  }
});
