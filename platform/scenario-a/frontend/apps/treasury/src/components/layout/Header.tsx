import { Badge, Button, PlatformLogo } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";

export function Header() {
  const navigate = useNavigate();
  const { profile, logout, status } = useAuth();
  const institutionName = (import.meta.env.VITE_INSTITUTION_NAME ?? "Central Bank").trim() || "Central Bank";

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
            <p className="text-xs uppercase tracking-wide text-muted-foreground">{institutionName}</p>
            <h1 className="text-sm font-semibold">Treasury Portal</h1>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant="outline">TREASURY</Badge>
          <p className="hidden text-sm text-muted-foreground md:block">{profile?.subject ?? "Unknown operator"}</p>
          <Button variant="ghost" size="sm" onClick={() => void onLogout()} disabled={status === "loading"}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
