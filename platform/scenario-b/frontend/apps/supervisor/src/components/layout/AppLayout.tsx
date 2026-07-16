// SPDX-License-Identifier: Apache-2.0

import { Outlet } from "react-router-dom";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";

export function AppLayout() {
  return (
    <div className="min-h-screen bg-muted/30">
      <Header />
      <div className="mx-auto flex min-h-[calc(100vh-65px)] w-full max-w-7xl flex-col md:flex-row">
        <Sidebar />
        <main className="min-w-0 min-h-full flex-1 p-4">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
