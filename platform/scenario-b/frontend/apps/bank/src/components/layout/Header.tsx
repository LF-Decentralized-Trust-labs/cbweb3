// SPDX-License-Identifier: Apache-2.0

import { Button, PlatformLogo, goToLauncher } from "@cbweb3/ui";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";

export function Header() {
  const navigate = useNavigate();
  const { profile, logout } = useAuth();
  const institutionName =
    (import.meta.env.VITE_INSTITUTION_NAME ?? "Bank Portal").trim() ||
    "Bank Portal";

  const onLogout = async () => {
    await logout();
    if (!goToLauncher()) navigate("/login", { replace: true });
  };

  return (
    <header className="app-root-header px-4 py-3 backdrop-blur">
      <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <PlatformLogo imageClassName="h-8" />
          <div>
            <h1 className="text-lg font-semibold">{institutionName} Portal</h1>
            <p className="text-xs text-muted-foreground">
              Institution: {profile?.bankId ?? profile?.subject ?? "-"}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <Button variant="ghost" onClick={() => void onLogout()}>
            Logout
          </Button>
        </div>
      </div>
    </header>
  );
}
