import { Badge, Button, PlatformLogo } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { useAlertStore, useWebsocketStore } from "../../stores";

export function Header() {
  const navigate = useNavigate();
  const { user, logout, status } = useAuth();
  const isConnected = useWebsocketStore((state) => state.isConnected);
  const stale = useWebsocketStore((state) => state.stale);
  const criticalCount = useAlertStore((state) => state.alerts.filter((a) => a.severity === "CRITICAL").length);

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
          <Badge variant={isConnected ? "success" : "destructive"} className="hidden sm:inline-flex">
            {isConnected ? "WS" : "WS OFF"}
          </Badge>
          <Badge variant={stale ? "warning" : "secondary"} className="hidden sm:inline-flex">
            {stale ? "STALE" : "LIVE"}
          </Badge>
          {criticalCount > 0 && (
            <Badge variant="destructive">{criticalCount} CRITICAL</Badge>
          )}
          <p className="hidden text-sm text-muted-foreground md:block">{user?.name ?? "Unknown user"}</p>
          <Button variant="ghost" size="sm" onClick={() => void onLogout()} disabled={status === "loading"}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
