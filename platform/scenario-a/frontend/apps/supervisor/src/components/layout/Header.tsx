// SPDX-License-Identifier: Apache-2.0

import { Button, Badge, PlatformLogo } from "@cbweb3/ui";
import { useAuthStore, useWebsocketStore } from "../../stores";

export function Header() {
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const connected = useWebsocketStore((state) => state.connected);
  const alerts = useWebsocketStore((state) => state.events);

  return (
    <header className="app-root-header flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div className="flex items-center gap-3">
        <PlatformLogo imageClassName="h-8" />
        <div>
          <h1 className="text-lg font-semibold">CBWeb3 Supervisor Portal</h1>
          <p className="text-xs text-muted-foreground">Institution: {user?.institutionName ?? "-"}</p>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <Badge variant={connected ? "success" : "destructive"}>{connected ? "SSE Connected" : "SSE Disconnected"}</Badge>
        <span className="text-xs text-muted-foreground">Alerts: {alerts.length}</span>
        <Button variant="ghost" onClick={() => void logout()}>
          Logout
        </Button>
      </div>
    </header>
  );
}
