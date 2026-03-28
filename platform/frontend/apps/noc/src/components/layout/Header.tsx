import { Badge, Button } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";

export function Header() {
  const navigate = useNavigate();
  const { user, logout, status } = useAuth();

  const onLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <header className="border-b border-border bg-card">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        <div>
          <p className="text-xs uppercase tracking-wide text-muted-foreground">CBWeb3</p>
          <h1 className="text-sm font-semibold">NOC Portal</h1>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant="outline">SYS_ADMIN</Badge>
          <p className="hidden text-sm text-muted-foreground md:block">{user?.name ?? "Unknown user"}</p>
          <Button variant="outline" size="sm" onClick={() => void onLogout()} disabled={status === "loading"}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
