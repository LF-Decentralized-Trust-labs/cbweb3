// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, PlatformLogo } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth, useCircuitBreaker } from "../../hooks";

export function Header() {
  const navigate = useNavigate();
  const { profile, logout } = useAuth();
  const { circuitBreaker } = useCircuitBreaker();
  const institutionName = (import.meta.env.VITE_INSTITUTION_NAME ?? "Central Bank").trim() || "Central Bank";

  const isHalted = circuitBreaker?.state === "HALTED";

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
            <p className="text-xs text-muted-foreground">Central Bank control plane</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={isHalted ? "destructive" : "default"}>
            Circuit Breaker: {circuitBreaker?.state ?? "LIVE"}
          </Badge>
          <span className="text-xs text-muted-foreground">{profile?.subject ?? "Unknown operator"}</span>
          <Button variant="ghost" size="sm" onClick={() => void onLogout()}>
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
