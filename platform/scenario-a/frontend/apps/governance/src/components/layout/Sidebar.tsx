// SPDX-License-Identifier: Apache-2.0

import { Badge } from "@cbweb3/ui";
import { ClipboardCheck, LayoutDashboard, ScrollText, Settings, Users } from "lucide-react";
import { NavLink } from "react-router-dom";
import { useAuth, useCircuitBreaker } from "../../hooks";
import {
  GOVERNANCE_ONLY,
  GOVERNANCE_OR_ADMISSION,
  isAllowedOnRoute,
  type RouteRequirement,
} from "../../auth/authorization";

// Each item carries the same requirement its route declares in routes/index.tsx, so the
// nav can never offer a link the router would bounce (spec 042 FR-008). Default is
// governance-only: an item added without a requirement stays hidden from Admission.
type NavItem = {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
  required?: RouteRequirement;
};

const navItems: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/registry", label: "Registry", icon: ClipboardCheck, required: GOVERNANCE_OR_ADMISSION },
  { to: "/accounts", label: "Accounts", icon: Users },
  { to: "/audit", label: "Audit", icon: ScrollText },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  const { circuitBreaker } = useCircuitBreaker();
  const { profile } = useAuth();
  const isHalted = circuitBreaker?.state === "HALTED";
  // Hide links the router would bounce for this profile (spec 042 FR-008).
  const visibleNavItems = navItems.filter((item) =>
    isAllowedOnRoute(profile, item.required ?? GOVERNANCE_ONLY),
  );

  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:shrink-0 md:border-b-0 md:border-r">
      <div className="mb-3 space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-muted-foreground">System State</span>
          <Badge variant={isHalted ? "destructive" : "default"}>{circuitBreaker?.state ?? "LIVE"}</Badge>
        </div>
      </div>
      <nav className="grid gap-1">
        {visibleNavItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors ${
                isActive ? "bg-primary text-primary-foreground shadow-sm" : "text-muted-foreground hover:bg-accent hover:text-foreground"
              }`
            }
          >
            <item.icon className="h-4 w-4" />
            {item.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
