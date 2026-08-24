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
  toast,
} from "@cbweb3/ui";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { RecordsEmptyState } from "../components/common/RecordsEmptyState";
import { usePaymentStore } from "../stores";
import {
  EscrowStatus,
  displayToBase,
  fiatCurrencyLabel,
  formatCeBM,
  formatFiatUnits,
  tokenUnitPrefix,
} from "../types";

const shortHash = (value: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

const PAGE_SIZE = 10;

function getEscrowStatusLabel(status: unknown): string {
  if (typeof status === "number") {
    if (status === EscrowStatus.APPROVED) return "APPROVED";
    if (status === EscrowStatus.REJECTED) return "REJECTED";
    return "PENDING";
  }
  if (typeof status === "string") {
    const normalized = status.toUpperCase().replace("ESCROW_STATUS_", "");
    if (normalized === "APPROVED") return "APPROVED";
    if (normalized === "REJECTED") return "REJECTED";
    return "PENDING";
  }
  return "PENDING";
}

function getEscrowStatusVariant(
  status: unknown,
): "warning" | "success" | "destructive" | "outline" {
  const label = getEscrowStatusLabel(status);
  if (label === "APPROVED") return "success";
  if (label === "REJECTED") return "destructive";
  return "warning";
}

export function EscrowsPage() {
  const fetchAll = usePaymentStore((state) => state.fetchAll);
  const requestEscrow = usePaymentStore((state) => state.requestEscrow);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const fiatBalance = usePaymentStore((state) => state.fiatBalance);
  const fCeBMDecimals = usePaymentStore((state) => state.fCeBMDecimals);
  const fCeBMSymbol = usePaymentStore((state) => state.fCeBMSymbol);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const tCeBMSymbol = usePaymentStore((state) => state.tCeBMSymbol);
  // Instrument names follow the on-chain symbols (custom per spoke).
  const tCeBMName = tokenUnitPrefix(tCeBMSymbol);
  const fCeBMName = tokenUnitPrefix(fCeBMSymbol, "fCeBM");
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);
  const fDecimals = fCeBMDecimals ?? 18;
  const tDecimals = tCeBMDecimals ?? 18;

  const [depositId, setDepositId] = useState("");
  const [amount, setAmount] = useState("0");
  const [confirmRequest, setConfirmRequest] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  const pendingCount = useMemo(
    () => escrows.filter((e) => getEscrowStatusLabel(e.status) === "PENDING").length,
    [escrows],
  );

  const total = escrows.length;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  useEffect(() => {
    setPage((current) => Math.min(current, totalPages));
  }, [totalPages]);

  const pagedEscrows = useMemo(() => escrows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [escrows, page]);
  const rangeStart = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const rangeEnd = Math.min(page * PAGE_SIZE, total);

  const approvedDeposits = useMemo(
    () => deposits.filter((d) => {
      const s = typeof d.status === "string"
        ? d.status.toUpperCase().replace("DEPOSIT_STATUS_", "")
        : "";
      return s === "APPROVED" || d.status === 1;
    }),
    [deposits],
  );

  const onPrepareSubmit = () => {
    if (!depositId.trim()) {
      toast.error("Deposit ID is required.");
      return;
    }
    if (!/^\d+(\.\d+)?$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive number.");
      return;
    }
    setConfirmRequest(true);
  };

  const onSubmit = async () => {
    try {
      const escrowId = await requestEscrow(depositId, displayToBase(amount, fDecimals));
      toast.success(`Tokenisation request submitted: ${escrowId}`);
      setDepositId("");
      setAmount("0");
      setConfirmRequest(false);
    } catch (submitError) {
      toast.error(
        submitError instanceof Error ? submitError.message : "Unable to submit tokenisation request",
      );
    }
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Fiat Reserve Balance ({fCeBMName})</CardDescription>
            <CardTitle>
              {fiatBalance !== null ? formatFiatUnits(fiatBalance, fDecimals, fCeBMSymbol) : "—"}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Tokenisation Requests</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Request Tokenisation ({fCeBMName} → {tCeBMName})</CardTitle>
          <CardDescription>
            Convert your fiat reserve ({fCeBMName}) into tokenised central bank money ({tCeBMName}).
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="escrow-deposit-id">Deposit ID</Label>
            <Input
              id="escrow-deposit-id"
              placeholder="Select an approved deposit ID"
              value={depositId}
              onChange={(e) => setDepositId(e.target.value)}
              list="approved-deposits"
            />
            <datalist id="approved-deposits">
              {approvedDeposits.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.id} — {formatFiatUnits(d.amount, fDecimals, fCeBMSymbol)}
                </option>
              ))}
            </datalist>
          </div>
          <div className="space-y-2">
            <Label htmlFor="escrow-amount">Amount ({fiatCurrencyLabel(fCeBMSymbol)})</Label>
            <Input
              id="escrow-amount"
              type="number"
              min="0"
              step="any"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={onPrepareSubmit} disabled={status === "loading"}>
              Review Tokenisation Request
            </Button>
            <Button variant="outline" onClick={() => void fetchAll()} disabled={status === "loading"}>
              Refresh
            </Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {confirmRequest ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Tokenisation Request</CardTitle>
            <CardDescription>
              {formatFiatUnits(amount, fDecimals, fCeBMSymbol)} held as {fCeBMName} will be submitted to the central bank for conversion to{" "}
              {formatCeBM(amount, tDecimals, tCeBMSymbol)}.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button onClick={() => void onSubmit()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Confirm Request"}
            </Button>
            <Button variant="outline" onClick={() => setConfirmRequest(false)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Tokenisation Requests</CardTitle>
          <CardDescription>Total records: {escrows.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Deposit ID</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Burn Tx ({fCeBMName})</TableHead>
                <TableHead>Mint Tx ({tCeBMName})</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pagedEscrows.map((escrow) => (
                <TableRow key={escrow.id}>
                  <TableCell className="font-medium">{escrow.id}</TableCell>
                  <TableCell>{escrow.deposit_id}</TableCell>
                  <TableCell>{formatFiatUnits(escrow.amount, fDecimals, fCeBMSymbol)}</TableCell>
                  <TableCell>
                    <Badge variant={getEscrowStatusVariant(escrow.status)}>
                      {getEscrowStatusLabel(escrow.status)}
                    </Badge>
                  </TableCell>
                  <TableCell title={escrow.burn_tx_hash}>{shortHash(escrow.burn_tx_hash)}</TableCell>
                  <TableCell title={escrow.mint_tx_hash}>{shortHash(escrow.mint_tx_hash)}</TableCell>
                  <TableCell>{new Date(escrow.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!total ? <RecordsEmptyState emptyLabel="No tokenisation requests found." /> : null}

          {/* Pagination */}
          <div className="flex items-center justify-between gap-2 pt-4">
            <span className="text-xs text-muted-foreground">
              {total === 0 ? "—" : `Showing ${rangeStart}–${rangeEnd} of ${total}`}
            </span>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page <= 1}>
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
                disabled={page >= totalPages}
              >
                Next
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
