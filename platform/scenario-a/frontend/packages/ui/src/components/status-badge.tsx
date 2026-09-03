// SPDX-License-Identifier: Apache-2.0

import { Badge } from "./badge";

type StatusBadgeProps = {
  status: string;
};

const variants: Record<string, "warning" | "default" | "success" | "destructive"> = {
  PENDING: "warning",
  LOCKED: "default",
  SETTLED: "success",
  FAILED: "destructive",
  CONFIRMED: "success",
};

export function StatusBadge({ status }: StatusBadgeProps) {
  return <Badge variant={variants[status] ?? "outline"}>{status}</Badge>;
}
