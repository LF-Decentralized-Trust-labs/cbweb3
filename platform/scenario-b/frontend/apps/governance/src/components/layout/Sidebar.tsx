// SPDX-License-Identifier: Apache-2.0

import { Badge } from "@cbweb3/ui";
import {
  ClipboardCheck,
  Eye,
  LayoutDashboard,
  ListChecks,
  Settings,
  ShieldAlert,
  SlidersHorizontal,
  Users,
} from "lucide-react";
import { NavLink } from "react-router-dom";
import { isScenarioB } from "../../config/scenario";
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

const scenarioANavItems: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/registry", label: "Registry", icon: ClipboardCheck, required: GOVERNANCE_OR_ADMISSION },
  { to: "/circuit-breaker", label: "Circuit Breaker", icon: ShieldAlert },
  { to: "/transfer-limits", label: "Transfer Limits", icon: SlidersHorizontal },
  { to: "/accounts", label: "Accounts", icon: Users },
  { to: "/audit", label: "Audit", icon: ClipboardCheck },
  { to: "/settings", label: "Settings", icon: Settings },
];

const scenarioBNavItems: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/registry", label: "Registry", icon: ClipboardCheck, required: GOVERNANCE_OR_ADMISSION },
  { to: "/accounts", label: "Accounts", icon: Users },
  { to: "/approvals-overview", label: "Approvals Overview", icon: ListChecks },
  { to: "/circuit-breaker", label: "Circuit Breaker", icon: ShieldAlert },
  { to: "/transfer-limits", label: "Transfer Limits", icon: SlidersHorizontal },
  { to: "/oversight", label: "Oversight", icon: Eye },
  { to: "/audit", label: "Audit", icon: ClipboardCheck },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  const { circuitBreaker } = useCircuitBreaker();
  const { profile } = useAuth();
  const isHalted = circuitBreaker?.state === "HALTED";
  const navItems = (isScenarioB ? scenarioBNavItems : scenarioANavItems).filter((item) =>
    isAllowedOnRoute(profile, item.required ?? GOVERNANCE_ONLY),
  );

  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:shrink-0 md:border-b-0 md:border-r">
      <div className="mb-3 space-y-2">
        {/* <PlatformLogo imageClassName="h-7" /> */}
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-muted-foreground">System State</span>
          <Badge variant={isHalted ? "destructive" : "default"}>{circuitBreaker?.state ?? "LIVE"}</Badge>
        </div>
      </div>
      <nav className="grid gap-1">
        {navItems.map((item) => (
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
