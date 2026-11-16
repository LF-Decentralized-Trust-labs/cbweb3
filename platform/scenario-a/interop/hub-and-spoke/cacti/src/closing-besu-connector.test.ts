// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, afterEach } from "vitest";
import http from "node:http";
import type { Socket } from "node:net";
import { createHash, randomUUID } from "node:crypto";
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { ClosingBesuConnector } from "./closing-besu-connector";

// A Besu WebSocket slot is held until the client closes its socket.
//
// SUP-152: the central bank Besu ran out of all 80 WebSocket slots, the local Paladin node could
// not open its block listener, and the spoke settled nothing for fifteen days. One source of
// those slots was this relay, which rebuilds its Besu connector every few seconds while poll
// cycles fail. Cacti's connector-besu 2.0.0 opens a WebSocket in its constructor and its
// shutdown() only writes a log line, so every rebuild left the previous socket open on Besu.
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
});
