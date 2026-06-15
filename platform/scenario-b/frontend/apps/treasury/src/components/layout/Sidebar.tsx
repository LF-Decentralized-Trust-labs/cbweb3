import { NavLink } from "react-router-dom";
import { LayoutDashboard, HandCoins, Coins, Flame, ShieldAlert, ScrollText, Settings, Gauge, Landmark, Wallet, ListChecks } from "lucide-react";

const links = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/funding-requests", label: "Funding Requests", icon: HandCoins },
  { to: "/issuance", label: "Issuance", icon: Coins },
  { to: "/redemption", label: "Redemption", icon: Flame },
  { to: "/deposits-approval", label: "Issuance Approvals", icon: Wallet },
  { to: "/escrows-approval", label: "Tokenisation Approvals", icon: ListChecks },
  { to: "/redeems-approval", label: "Redeem Approvals", icon: Gauge },
  { to: "/reconciliation", label: "Reconciliation", icon: ShieldAlert },
  { to: "/liquidity", label: "Liquidity Management", icon: Landmark },
  { to: "/transfer-limits", label: "Transfer Limits", icon: Gauge },
  { to: "/audit", label: "Audit", icon: ScrollText },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:border-b-0 md:border-r">
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
