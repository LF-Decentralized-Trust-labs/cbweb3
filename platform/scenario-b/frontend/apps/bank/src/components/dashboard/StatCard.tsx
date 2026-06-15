import type { ReactNode } from "react";
import { Card, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";

type StatCardProps = {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  loading?: boolean;
};

// StatCard is a compact KPI tile: a description label, a prominent value, and an
// optional smaller hint line beneath it.
export function StatCard({ label, value, hint, loading = false }: StatCardProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl">{loading ? "…" : value}</CardTitle>
        {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
      </CardHeader>
    </Card>
  );
}
