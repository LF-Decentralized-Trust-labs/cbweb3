// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";

type StatCardProps = {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  footer?: ReactNode;
  loading?: boolean;
};

// StatCard is a compact KPI tile: a description label, a prominent value, an optional
// hint line, and an optional footer slot (e.g. a review link).
export function StatCard({ label, value, hint, footer, loading = false }: StatCardProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl">{loading ? "…" : value}</CardTitle>
        {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
      </CardHeader>
      {footer ? <CardContent>{footer}</CardContent> : null}
    </Card>
  );
}
