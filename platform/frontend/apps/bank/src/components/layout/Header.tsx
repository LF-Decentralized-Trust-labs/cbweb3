import { Button } from "@cbweb3/ui";
import { useAuthStore, useWebsocketStore } from "../../stores";

export function Header() {
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const connected = useWebsocketStore((state) => state.connected);
  const events = useWebsocketStore((state) => state.events);

  return (
    <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
      <div>
        <h1 className="text-lg font-semibold">CBWeb3 Bank Portal</h1>
        <p className="text-xs text-muted-foreground">Institution: {user?.institutionId ?? "-"}</p>
      </div>

      <div className="flex items-center gap-3">
        <span className={`rounded px-2 py-1 text-xs ${connected ? "bg-green-100 text-green-800" : "bg-red-100 text-red-800"}`}>
          {connected ? "WS Connected" : "WS Disconnected"}
        </span>
        <span className="text-xs text-muted-foreground">Events: {events.length}</span>
        <Button variant="outline" onClick={() => void logout()}>
          Logout
        </Button>
      </div>
    </header>
  );
}
