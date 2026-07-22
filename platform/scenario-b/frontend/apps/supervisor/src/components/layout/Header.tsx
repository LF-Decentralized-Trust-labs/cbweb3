// SPDX-License-Identifier: Apache-2.0

import { Button, PlatformLogo, goToLauncher } from "@cbweb3/ui";
import { useAuthStore } from "../../stores";

export function Header() {
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);

  return (
    <header className="app-root-header px-4 py-3 backdrop-blur">
      <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <PlatformLogo imageClassName="h-8" />
          <div>
            <h1 className="text-lg font-semibold">CBWeb3 Supervisor Portal · {import.meta.env.VITE_SPOKE_NAME}</h1>
            <p className="text-xs text-muted-foreground">Institution: {user?.institutionName ?? "-"}</p>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={() => void logout().then(() => goToLauncher())}>
          Sign out
        </Button>
      </div>
    </header>
  );
}
