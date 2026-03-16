import { Badge } from "@cbweb3/ui";
import { Link, useLocation } from "react-router-dom";
import { useCircuitBreaker } from "../../hooks";

const navItems = [
  { to: "/", label: "Dashboard" },
  { to: "/registry", label: "Registry" },
  { to: "/circuit-breaker", label: "Circuit Breaker" },
  { to: "/accounts", label: "Accounts" },
  { to: "/parameters", label: "Parameters" },
  { to: "/audit", label: "Audit" },
  { to: "/settings", label: "Settings" },
];

export function Sidebar() {
  const location = useLocation();
  const { circuitBreaker } = useCircuitBreaker();
  const isHalted = circuitBreaker?.state === "HALTED";

  return (
    <aside className="w-full border-r border-border bg-card p-3 md:w-64 md:p-4">
      <div className="mb-4 flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">System State</span>
        <Badge variant={isHalted ? "destructive" : "default"}>{circuitBreaker?.state ?? "LIVE"}</Badge>
      </div>
      <nav className="space-y-1">
        {navItems.map((item) => {
          const active = location.pathname === item.to;
          return (
            <Link
              key={item.to}
              to={item.to}
              className={`block rounded-md px-3 py-2 text-sm transition-colors ${
                active ? "bg-primary text-primary-foreground" : "hover:bg-muted"
              }`}
            >
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
