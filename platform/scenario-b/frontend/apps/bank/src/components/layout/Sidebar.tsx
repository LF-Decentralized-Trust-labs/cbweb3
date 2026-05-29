import { NavLink } from "react-router-dom";
import {
  ArrowDownToLine,
  ArrowRightLeft,
  ArrowUpFromLine,
  ClipboardList,
  Coins,
  Handshake,
  LayoutDashboard,
  Lock,
  Scale,
  Settings,
  ShieldCheck,
} from "lucide-react";
// import { PlatformLogo } from "@cbweb3/ui";
import { isScenarioB } from "../../config/scenario";

const scenarioALinks = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/liquidity", label: "Liquidity & Transfers", icon: Coins },
  { to: "/deposits", label: "Issuance Requests", icon: ArrowDownToLine },
  { to: "/escrows", label: "Reserve Tokenisation", icon: Lock },
  { to: "/redeems", label: "Redeems", icon: ArrowUpFromLine },
  { to: "/agreements", label: "Trade Agreements", icon: Handshake },
  { to: "/htlc", label: "PvP Settlement", icon: ArrowRightLeft },
  { to: "/amm", label: "Automated FX Trading", icon: Scale },
  { to: "/compliance", label: "Compliance", icon: ShieldCheck },
  { to: "/onboarding", label: "Onboarding", icon: ClipboardList },
  { to: "/settings", label: "Settings", icon: Settings },
];

const scenarioBLinks = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/transfer", label: "Transfer", icon: ArrowRightLeft },
  { to: "/deposits", label: "Issuance Requests", icon: ArrowDownToLine },
  { to: "/escrows", label: "Reserve Tokenisation", icon: Lock },
  { to: "/approve-amm", label: "Approve AMM", icon: Scale },
  { to: "/redeems", label: "Redeems", icon: ArrowUpFromLine },
  { to: "/bridge", label: "Bridge", icon: ArrowRightLeft },
  { to: "/compliance", label: "Compliance", icon: ShieldCheck },
  { to: "/onboarding", label: "Onboarding", icon: ClipboardList },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  const links = isScenarioB ? scenarioBLinks : scenarioALinks;

  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:border-b-0 md:border-r">
      <div className="mb-3 space-y-1">
        {/* <PlatformLogo imageClassName="h-7" /> */}
        <p className="text-xs uppercase tracking-wide text-muted-foreground">LNET</p>
        <p className="text-sm font-semibold">Bank Portal</p>
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
