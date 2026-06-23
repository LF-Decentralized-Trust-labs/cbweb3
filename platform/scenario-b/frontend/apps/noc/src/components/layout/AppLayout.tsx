// SPDX-License-Identifier: Apache-2.0

import { useEffect } from "react";
import { Outlet } from "react-router-dom";
import { useWebsocketStore } from "../../stores";
import { StaleDataBanner } from "../common/StaleDataBanner";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";
import { TopStatusBar } from "./TopStatusBar";

export function AppLayout() {
  const connect = useWebsocketStore((state) => state.connect);
  const disconnect = useWebsocketStore((state) => state.disconnect);
  const stale = useWebsocketStore((state) => state.stale);

  useEffect(() => {
    connect();
    return () => disconnect();
  }, [connect, disconnect]);

  return (
    <div className="flex min-h-screen flex-col bg-muted/30">
      <TopStatusBar />
      <Header />
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col md:flex-row">
        <Sidebar />
        <main className="flex-1 p-4">
          <StaleDataBanner stale={stale} />
          <Outlet />
        </main>
      </div>
    </div>
  );
}
