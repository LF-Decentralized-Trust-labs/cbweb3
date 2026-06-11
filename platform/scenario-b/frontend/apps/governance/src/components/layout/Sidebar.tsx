import { Badge } from "@cbweb3/ui";
import {
  ClipboardCheck,
  Gauge,
  Eye,
  LayoutDashboard,
  Landmark,
  ListChecks,
  Lock,
  Scale,
  Settings,
  ShieldAlert,
  Users,
  Wallet,
} from "lucide-react";
import { NavLink } from "react-router-dom";
import { isScenarioB } from "../../config/scenario";
import { useCircuitBreaker } from "../../hooks";

const scenarioANavItems = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/registry", label: "Registry", icon: ClipboardCheck },
  { to: "/circuit-breaker", label: "Circuit Breaker", icon: ShieldAlert },
  { to: "/htlc-monitor", label: "PvP Settlement", icon: Lock },
  { to: "/accounts", label: "Accounts", icon: Users },
  { to: "/parameters", label: "Parameters", icon: Scale },
  { to: "/deposits-approval", label: "Issuance Approvals", icon: Wallet },
  { to: "/escrows-approval", label: "Tokenisation Approvals", icon: ListChecks },
  { to: "/redeems-approval", label: "Redeem Approvals", icon: Gauge },
  { to: "/audit", label: "Audit", icon: ClipboardCheck },
  { to: "/settings", label: "Settings", icon: Settings },
];

const scenarioBNavItems = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/registry", label: "Registry", icon: ClipboardCheck },
  { to: "/liquidity", label: "Liquidity Management", icon: Landmark },
  { to: "/deposits-approval", label: "Issuance Approvals", icon: Wallet },
  { to: "/escrows-approval", label: "Tokenisation Approvals", icon: ListChecks },
  { to: "/redeems-approval", label: "Redeem Approvals", icon: Gauge },
  { to: "/swap-monitor", label: "Swap Monitor", icon: ListChecks },
  { to: "/circuit-breaker", label: "Circuit Breaker", icon: ShieldAlert },
  { to: "/oversight", label: "Oversight", icon: Eye },
  { to: "/transfer-limits", label: "Transfer Limits", icon: Scale },
  { to: "/audit", label: "Audit", icon: ClipboardCheck },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  const { circuitBreaker } = useCircuitBreaker();
  const isHalted = circuitBreaker?.state === "HALTED";
  const navItems = isScenarioB ? scenarioBNavItems : scenarioANavItems;

  return (
    <aside className="w-full border-b border-border bg-card p-3 md:min-h-full md:w-64 md:border-b-0 md:border-r">
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
