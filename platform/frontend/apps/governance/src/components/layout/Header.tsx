import { Badge, Button } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth, useCircuitBreaker } from "../../hooks";

export function Header() {
  const navigate = useNavigate();
  const { profile, logout } = useAuth();
  const { circuitBreaker } = useCircuitBreaker();
  const portalOwner = (import.meta.env.VITE_PORTAL_OWNER ?? "UNSET_OWNER").trim() || "UNSET_OWNER";

  const isHalted = circuitBreaker?.state === "HALTED";

  const onLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <header className="border-b border-border bg-background/95 px-4 py-3 backdrop-blur">
      <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4">
        <div>
          <h1 className="text-lg font-semibold">Governance Portal</h1>
          <p className="text-xs text-muted-foreground">Central Bank control plane</p>
          <p className="text-xs text-muted-foreground">{portalOwner}</p>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={isHalted ? "destructive" : "default"}>
            Circuit Breaker: {circuitBreaker?.state ?? "LIVE"}
          </Badge>
          <span className="text-xs text-muted-foreground">{profile?.subject ?? "Unknown operator"}</span>
          <Button variant="outline" size="sm" onClick={() => void onLogout()}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
