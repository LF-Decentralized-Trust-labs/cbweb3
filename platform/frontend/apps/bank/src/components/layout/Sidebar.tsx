import { NavLink } from "react-router-dom";
import {
  ArrowDownToLine,
  ArrowRightLeft,
  ArrowUpFromLine,
  ClipboardList,
  Coins,
  LayoutDashboard,
  Lock,
  Scale,
  Settings,
  ShieldCheck,
} from "lucide-react";

const links = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/liquidity", label: "Liquidity & Transfers", icon: Coins },
  { to: "/deposits", label: "Deposits", icon: ArrowDownToLine },
  { to: "/escrows", label: "Pledges", icon: Lock },
  { to: "/redeems", label: "Redeems", icon: ArrowUpFromLine },
  { to: "/htlc", label: "PvP Settlement", icon: ArrowRightLeft },
  { to: "/amm", label: "Automated FX Trading", icon: Scale },
  { to: "/compliance", label: "Compliance", icon: ShieldCheck },
  { to: "/onboarding", label: "Onboarding", icon: ClipboardList },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:border-b-0 md:border-r">
      <p className="mb-1 text-xs uppercase tracking-wide text-muted-foreground">LACnet</p>
      <p className="mb-3 text-sm font-semibold">Bank Portal</p>
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
