// SPDX-License-Identifier: Apache-2.0

import { BackToLauncherButton, Badge, Button, PlatformLogo } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { useAlertStore, useUiStore } from "../../stores";

export function Header() {
  const navigate = useNavigate();
  const { user, logout, status } = useAuth();
  const criticalCount = useAlertStore((state) => state.alerts.filter((a) => a.severity === "CRITICAL").length);
  const { fallbackPollingSeconds } = useUiStore();

  // Sign out lands on the portal's own login screen. Redirecting to the launcher instead
  // strands the operator on a browser error whenever the launcher is not deployed on this
  // host (the URL is baked at build time and never probed) — "Back to launcher" stays as
  // the explicit way to leave the portal.
  const onLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <header className="app-root-header">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        <div className="flex items-center gap-3">
          <PlatformLogo imageClassName="h-7" />
          <div>
            <p className="text-xs uppercase tracking-wide text-muted-foreground">LNET</p>
            <h1 className="text-sm font-semibold">NOC Portal</h1>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Badge variant="secondary" className="hidden sm:inline-flex">POLLING {fallbackPollingSeconds}s</Badge>
          <Badge variant="default" className="hidden sm:inline-flex">LIVE</Badge>
          {criticalCount > 0 && (
            <Badge variant="destructive">{criticalCount} CRITICAL</Badge>
          )}
          <p className="hidden text-sm text-muted-foreground md:block">{user?.name ?? "Unknown user"}</p>
          <BackToLauncherButton />
          <Button variant="ghost" size="sm" onClick={() => void onLogout()} disabled={status === "loading"}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
