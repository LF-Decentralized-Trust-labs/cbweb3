// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, afterEach, vi } from "vitest";
import { createRequire } from "node:module";
import http from "node:http";
import type { Socket } from "node:net";
import { createHash, randomUUID } from "node:crypto";
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import {
  ClosingBesuConnector,
  closeConnectors,
  sharedBesuPrometheusExporter,
} from "./closing-besu-connector";

// A Besu WebSocket slot is held until the client closes its socket.
//
// An implicit default of 80 connections was exhausted in production: once every slot was taken,
// Besu refused new WebSocket clients, including the local Paladin node. One source of those
// slots was this relay, which rebuilds its Besu connector every few seconds while poll cycles
// fail. Cacti's connector-besu 2.0.0 opens a WebSocket in its constructor and its shutdown()
// only writes a log line, so every rebuild left the previous socket open on Besu.
//
// These tests run the REAL connector against a WebSocket server that counts open sockets, so
// what they assert is what Besu would see: a slot taken, and a slot given back.

// Minimal RFC 6455 server: completes the handshake, tracks each upgraded socket until it closes,
// and ends the socket when the client sends a close frame. Built on node:http so the test needs
// no WebSocket library of its own.
function startCountingWsServer(): Promise<{
  url: string;
  open: () => number;
  stop: () => Promise<void>;
}> {
  const sockets = new Set<Socket>();
  const server = http.createServer();
  server.on("upgrade", (req, socket: Socket) => {
    const key = req.headers["sec-websocket-key"];
    const accept = createHash("sha1")
      .update(`${key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`)
      .digest("base64");
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

describe("Besu connector shutdown releases its WebSocket", () => {
  let server: Awaited<ReturnType<typeof startCountingWsServer>> | undefined;
  afterEach(async () => {
    await server?.stop();
    server = undefined;
  });

  // The defect this file works around, pinned so a library upgrade that fixes it is noticed: if
  // this starts failing after bumping connector-besu, upstream shutdown() closes the socket and
  // ClosingBesuConnector can be deleted. It also proves the harness sees an open socket at all,
  // so the passing test below is not vacuous.
  it("the library connector keeps its socket open after shutdown()", async () => {
    server = await startCountingWsServer();
    const connector = new PluginLedgerConnectorBesu(options(server.url));
    await connector.onPluginInit();
    expect(await waitFor(() => server!.open() === 1, 3_000)).toBe(true);

    await connector.shutdown();
    await new Promise((r) => setTimeout(r, 500));
    expect(server.open()).toBe(1);
  });

  it("ClosingBesuConnector closes its socket on shutdown()", async () => {
    server = await startCountingWsServer();
    const connector = new ClosingBesuConnector(options(server.url));
    await connector.onPluginInit();
    expect(await waitFor(() => server!.open() === 1, 3_000)).toBe(true);

    await connector.shutdown();
    expect(await waitFor(() => server!.open() === 0, 3_000)).toBe(true);
  });

  // The fix reaches into a private field of a third-party class. If an upgrade renames it, the
  // failure must be loud: a shutdown that quietly does nothing is the original leak again.
  it("fails loudly when the provider can no longer be reached", async () => {
    server = await startCountingWsServer();
    const connector = new ClosingBesuConnector(options(server.url));
    (connector as unknown as { web3Provider: unknown }).web3Provider = {};

    await expect(connector.shutdown()).rejects.toThrow(/WebSocket/);
  });

  it("closing twice is harmless", async () => {
    server = await startCountingWsServer();
    const connector = new ClosingBesuConnector(options(server.url));
    await connector.onPluginInit();
    expect(await waitFor(() => server!.open() === 1, 3_000)).toBe(true);

    await connector.shutdown();
    await expect(connector.shutdown()).resolves.toBeUndefined();
    expect(await waitFor(() => server!.open() === 0, 3_000)).toBe(true);
  });
});

// The socket is opened by the constructor, before onPluginInit() runs. A connector whose
// initialisation fails must still give its slot back, or every failed rebuild leaks one.
describe("ClosingBesuConnector.open", () => {
  let server: Awaited<ReturnType<typeof startCountingWsServer>> | undefined;
  afterEach(async () => {
    vi.restoreAllMocks();
    await server?.stop();
    server = undefined;
  });

  it("returns an initialised connector", async () => {
    server = await startCountingWsServer();
    const connector = await ClosingBesuConnector.open(options(server.url));
    expect(await waitFor(() => server!.open() === 1, 3_000)).toBe(true);
    await connector.shutdown();
  });

  it("closes the socket when initialisation fails, then rethrows", async () => {
    server = await startCountingWsServer();
    const srv = server;
    // Fail only once the socket is really open on the server, so the test proves a slot taken
    // and given back rather than a connection that never started.
    vi.spyOn(PluginLedgerConnectorBesu.prototype, "onPluginInit").mockImplementation(async () => {
      await waitFor(() => srv.open() === 1, 3_000);
      throw new Error("init boom");
    });

    await expect(ClosingBesuConnector.open(options(server.url))).rejects.toThrow("init boom");
    expect(await waitFor(() => server!.open() === 0, 3_000)).toBe(true);
  });
});

// The library builds a PrometheusExporter per connector and starts prom-client's default metrics
// on it, which never stop (an event-loop-delay histogram and a GC observer each time). The relay
// rebuilds connectors for as long as Besu is unreachable, so those must not multiply.
describe("connector metrics are started once per process", () => {
  // prom-client as the connector library resolves it, so the spy sees the library's calls.
  const promClient = createRequire(
    require.resolve("@hyperledger/cactus-plugin-ledger-connector-besu"),
  )("prom-client") as { collectDefaultMetrics: (...args: unknown[]) => void };

  let server: Awaited<ReturnType<typeof startCountingWsServer>> | undefined;
  afterEach(async () => {
    vi.restoreAllMocks();
    await server?.stop();
    server = undefined;
  });

  it("the library starts a new set for every connector", async () => {
    server = await startCountingWsServer();
    const a = new PluginLedgerConnectorBesu(options(server.url));
    const b = new PluginLedgerConnectorBesu(options(server.url));
    expect(a.getPrometheusExporter()).not.toBe(b.getPrometheusExporter());
  });

  it("every ClosingBesuConnector shares one exporter and starts no new metrics", async () => {
    server = await startCountingWsServer();
    const first = new ClosingBesuConnector(options(server.url)); // the shared set exists from here on
    const spy = vi.spyOn(promClient, "collectDefaultMetrics");

    const rebuilt = Array.from({ length: 5 }, () => new ClosingBesuConnector(options(server!.url)));

    expect(spy).not.toHaveBeenCalled();
    for (const c of rebuilt) expect(c.getPrometheusExporter()).toBe(sharedBesuPrometheusExporter());
    expect(first.getPrometheusExporter()).toBe(sharedBesuPrometheusExporter());
    await closeConnectors(new Map([first, ...rebuilt].map((c, i) => [`s${i}`, c])));
  });
});

// The exit path awaits every close before the process exits, and reports each failure.
describe("closeConnectors", () => {
  it("waits for every close and returns the ones that failed", async () => {
    const order: string[] = [];
    const slow = {
      shutdown: async () => {
        await new Promise((r) => setTimeout(r, 50));
        order.push("slow");
      },
    };
    const failing = { shutdown: async () => { throw new Error("provider gone"); } };

    const failures = await closeConnectors(new Map([["spoke-a", slow], ["spoke-b", failing]]));

    expect(order).toEqual(["slow"]);
    expect(failures).toHaveLength(1);
    expect(failures[0].spokeId).toBe("spoke-b");
    expect(String(failures[0].error)).toContain("provider gone");
  });
});
