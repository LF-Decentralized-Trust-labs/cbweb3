// SPDX-License-Identifier: Apache-2.0

import { NavLink } from "react-router-dom";
import { LayoutDashboard, Server, Shuffle, TriangleAlert, Network, ScrollText, Settings } from "lucide-react";
import { PlatformLogo } from "@cbweb3/ui";

const links = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/infrastructure", label: "Infrastructure", icon: Server },
  { to: "/relays", label: "Relays", icon: Shuffle },
  { to: "/pool-stability", label: "Pool Stability", icon: TriangleAlert },
  { to: "/topology", label: "Topology", icon: Network },
  { to: "/audit", label: "Audit", icon: ScrollText },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:shrink-0 md:border-b-0 md:border-r">
      <div className="mb-3 space-y-1">
        <PlatformLogo imageClassName="h-7" />
        <p className="text-xs uppercase tracking-wide text-muted-foreground">Operations</p>
        <p className="text-sm font-semibold">NOC Command Center</p>
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
