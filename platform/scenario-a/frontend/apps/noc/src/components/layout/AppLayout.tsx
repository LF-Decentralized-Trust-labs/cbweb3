// SPDX-License-Identifier: Apache-2.0

import { Outlet } from "react-router-dom";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";

export function AppLayout() {
  return (
    <div className="flex min-h-screen flex-col bg-muted/30">
      <Header />
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col md:flex-row">
        <Sidebar />
        <main className="min-w-0 flex-1 p-4">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
