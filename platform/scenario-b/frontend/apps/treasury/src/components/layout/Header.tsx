import { Badge, Button, PlatformLogo } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";

export function Header() {
  const navigate = useNavigate();
  const { user, logout, status } = useAuth();
  const institutionName = (import.meta.env.VITE_INSTITUTION_NAME ?? "Central Bank").trim() || "Central Bank";

  const onLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <header className="app-root-header px-4 py-3 backdrop-blur">
      <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <PlatformLogo imageClassName="h-8" />
          <div>
            <h1 className="text-lg font-semibold">{institutionName} Portal</h1>
            <p className="text-xs text-muted-foreground">Treasury operations</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant="secondary">TREASURY</Badge>
          <span className="text-xs text-muted-foreground">{user?.name ?? "Unknown operator"}</span>
          <Button variant="ghost" size="sm" onClick={() => void onLogout()} disabled={status === "loading"}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
