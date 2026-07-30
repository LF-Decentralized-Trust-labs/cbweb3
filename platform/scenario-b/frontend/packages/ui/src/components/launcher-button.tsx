// SPDX-License-Identifier: Apache-2.0

import { LayoutGrid } from "lucide-react";
import { Button } from "./button";
import { cn } from "../lib/utils";
import { getLauncherUrl, goToLauncher } from "../lib/launcher";

// BackToLauncherButton renders a "back to launcher" affordance on portal login screens.
// It self-hides when no launcher is configured (VITE_LAUNCHER_URL absent), so portals
// deployed without a launcher are unaffected. The launcher URL is never hardcoded — it is
// baked in at build time by each scenario's toolkit.
export function BackToLauncherButton({ className }: { className?: string }) {
  if (!getLauncherUrl()) return null;
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      className={cn("text-muted-foreground", className)}
      onClick={() => goToLauncher()}
    >
      <LayoutGrid />
      Back to launcher
    </Button>
  );
}
