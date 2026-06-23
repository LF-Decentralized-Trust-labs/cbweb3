// SPDX-License-Identifier: Apache-2.0

import { NavLink } from "react-router-dom";
import { Activity, ShieldCheck, Users, Search, SlidersHorizontal, Settings, FileSearch } from "lucide-react";

const links = [
  { to: "/", label: "Dashboard", icon: Activity },
  { to: "/liquidity", label: "Liquidity Monitor", icon: SlidersHorizontal },
  { to: "/participants", label: "Compliance Registry", icon: Users },
  { to: "/audit", label: "Audit Vault", icon: Search },
  { to: "/stability", label: "Stability Insights", icon: ShieldCheck },
  { to: "/investigation", label: "Investigation", icon: FileSearch },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:border-b-0 md:border-r">
      <div className="mb-3 px-1">
        <p className="text-sm font-semibold text-foreground">Supervisor Portal</p>
      </div>
      <nav className="grid gap-1">
        {links.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            end={link.to === "/"}
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors ${
                isActive ? "bg-primary text-primary-foreground shadow-sm" : "text-muted-foreground hover:bg-accent hover:text-foreground"
              }`
            }
          >
            <link.icon className="h-4 w-4" />
            {link.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
