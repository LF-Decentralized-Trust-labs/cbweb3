// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { ArrowRight, ChevronLeft, ChevronRight, RotateCcw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { crossCurrencySwapApi } from "../services/api/cross-currency-swap.api";
import { usePaymentStore } from "../stores";
import type { CrossCurrencySwapStatus, SwapHistoryItem } from "../types/cross-currency-swap.types";

const PAGE_SIZE = 20;

// formatAmount2 renders a base-unit (wei) amount with exactly 2 decimal places and
// thousands separators (e.g. "1,234,567.89"). The fraction is TRUNCATED (not rounded)
// to 2 digits, computed in BigInt to avoid float precision loss on large balances.
// Returns "—" for invalid input.
function formatAmount2(wei: string, decimals: number): string {
  if (!wei || !/^\d+$/.test(wei)) return "—";
  const divisor = 10n ** BigInt(decimals);
  const value = BigInt(wei);
  const whole = value / divisor;
  // Truncated two-decimal fraction (0..99): integer division drops the rest.
  const frac = ((value % divisor) * 100n) / divisor;
  return `${whole.toLocaleString()}.${frac.toString().padStart(2, "0")}`;
}

type BadgeVariant = "success" | "warning" | "destructive" | "outline" | "default";

// statusView maps the end-to-end swap lifecycle to a badge. COMPLETED = done,
// FAILED = errored, everything else is an in-flight stage.
function statusView(status: CrossCurrencySwapStatus): { variant: BadgeVariant; label: string } {
  switch (status) {
    case "COMPLETED":
      return { variant: "success", label: "Completed" };
    case "FAILED":
      return { variant: "destructive", label: "Failed" };
    case "QUOTING":
      return { variant: "warning", label: "Quoting" };
    case "BRIDGE_IN_PROGRESS":
      return { variant: "warning", label: "Bridging in" };
    case "SWAP_IN_PROGRESS":
      return { variant: "warning", label: "Swapping" };
    case "BRIDGE_OUT_PROGRESS":
      return { variant: "warning", label: "Bridging out" };
    default:
      return { variant: "outline", label: status };
  }
}

function truncateHash(value?: string): string {
  if (!value) return "—";
  return value.length <= 14 ? value : `${value.slice(0, 8)}…${value.slice(-6)}`;
}

export function BridgeHistoryPage() {
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const tokenDecimals = tCeBMDecimals ?? 18;

  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [page, setPage] = useState(1);
  const [items, setItems] = useState<SwapHistoryItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const fetchHistory = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await crossCurrencySwapApi.listHistory({
        from: from || undefined,
        to: to || undefined,
        page,
        pageSize: PAGE_SIZE,
      });
      setItems(res.operations ?? []);
      setTotal(res.total ?? 0);
    } catch {
      setItems([]);
      setTotal(0);
      setError("Unable to load bridge operations. Please try again.");
    } finally {
      setLoading(false);
    }
  }, [from, to, page]);

  useEffect(() => {
    void fetchHistory();
  }, [fetchHistory]);

  // Changing the date window resets to the first page.
  const onFromChange = (v: string) => {
    setFrom(v);
    setPage(1);
  };
  const onToChange = (v: string) => {
    setTo(v);
    setPage(1);
  };
  const clearFilters = () => {
    setFrom("");
    setTo("");
    setPage(1);
  };

  const rangeStart = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const rangeEnd = Math.min(page * PAGE_SIZE, total);

  return (
    <div className="space-y-4">
      {/* Header */}
      <Card>
        <CardHeader>
          <CardTitle>Bridge History</CardTitle>
          <CardDescription>
            Cross-currency bridge operations initiated by your institution. Read-only view.
          </CardDescription>
        </CardHeader>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="flex flex-wrap items-end gap-4 py-4">
          <div className="space-y-1">
            <Label htmlFor="from">From</Label>
            <Input
              id="from"
              type="date"
              value={from}
              max={to || undefined}
              onChange={(e) => onFromChange(e.target.value)}
              className="w-44"
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="to">To</Label>
            <Input
              id="to"
              type="date"
              value={to}
              min={from || undefined}
              onChange={(e) => onToChange(e.target.value)}
              className="w-44"
            />
          </div>
          <Button variant="outline" size="sm" onClick={clearFilters} disabled={!from && !to}>
            <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
            Clear
          </Button>
          <div className="ml-auto text-sm text-muted-foreground">
            {loading ? "Loading…" : `${total} operation${total === 1 ? "" : "s"}`}
          </div>
        </CardContent>
      </Card>

      {/* Table */}
      <Card>
        <CardContent className="pt-6">
          {error ? (
            <p className="py-6 text-sm text-destructive">{error}</p>
          ) : (
            <>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>When</TableHead>
                      <TableHead>Route</TableHead>
                      <TableHead>Sent</TableHead>
                      <TableHead>Received</TableHead>
                      <TableHead>Rate</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Tx</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.map((op) => {
                      const sv = statusView(op.status);
                      return (
                        <TableRow key={op.swap_id}>
                          <TableCell className="whitespace-nowrap text-muted-foreground">
                            {new Date(op.created_at).toLocaleString()}
                          </TableCell>
                          <TableCell>
                            <span className="flex items-center gap-1.5 font-medium">
                              {op.source_currency}
                              <ArrowRight className="h-3.5 w-3.5 text-muted-foreground" />
                              {op.target_currency}
                            </span>
                            {op.beneficiary_bank_id ? (
                              <span className="text-xs text-muted-foreground">to {op.beneficiary_bank_id}</span>
                            ) : null}
                          </TableCell>
                          <TableCell className="font-mono">
                            {formatAmount2(op.amount_in, tokenDecimals)}{" "}
                            <span className="text-muted-foreground">{op.source_currency}</span>
                          </TableCell>
                          <TableCell className="font-mono">
                            {formatAmount2(op.amount_out, tokenDecimals)}{" "}
                            <span className="text-muted-foreground">{op.target_currency}</span>
                          </TableCell>
                          <TableCell className="font-mono">
                            {op.effective_rate ? op.effective_rate.toFixed(6) : "—"}
                          </TableCell>
                          <TableCell>
                            <Badge variant={sv.variant} title={op.failure_reason ?? undefined}>
                              {sv.label}
                            </Badge>
                          </TableCell>
                          <TableCell className="font-mono text-xs text-muted-foreground" title={op.swap_tx_hash}>
                            {truncateHash(op.swap_tx_hash)}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>

              {!loading && items.length === 0 ? (
                <p className="py-8 text-center text-sm text-muted-foreground">
                  No bridge operations{from || to ? " in the selected period" : " yet"}.
                </p>
              ) : null}

              {/* Pagination */}
              <div className="flex items-center justify-between gap-2 pt-4">
                <span className="text-xs text-muted-foreground">
                  {total === 0 ? "—" : `Showing ${rangeStart}–${rangeEnd} of ${total}`}
                </span>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={loading || page <= 1}
                  >
                    <ChevronLeft className="h-4 w-4" />
                    Prev
                  </Button>
                  <span className="text-xs text-muted-foreground">
                    Page {page} of {totalPages}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    disabled={loading || page >= totalPages}
                  >
                    Next
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
