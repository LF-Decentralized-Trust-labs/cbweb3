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
import { BalanceWidget } from "../components/common/BalanceWidget";
import { useAuthStore } from "../stores/auth.store";
import { usePaymentStore } from "../stores";
import {
  PaymentStatus,
  displayToBase,
  fiatCurrencyLabel,
  formatFiatUnits,
  formatTokenAmount,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
} from "../types";

const PAGE_SIZE = 10;

const shortHash = (value: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

export function DepositsPage() {
  const fetchAll = usePaymentStore((state) => state.fetchAll);
  const registerDeposit = usePaymentStore((state) => state.registerDeposit);
  const deposits = usePaymentStore((state) => state.deposits);
  const balance = usePaymentStore((state) => state.balance);
  const fiatBalance = usePaymentStore((state) => state.fiatBalance);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const tCeBMSymbol = usePaymentStore((state) => state.tCeBMSymbol);
  const fCeBMDecimals = usePaymentStore((state) => state.fCeBMDecimals);
  const fCeBMSymbol = usePaymentStore((state) => state.fCeBMSymbol);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);
  const profile = useAuthStore((state) => state.profile);
  const fDecimals = fCeBMDecimals ?? 18;

  const walletAddress = profile?.wallet?.trim() ?? "";

  const [amount, setAmount] = useState("0");
  const [confirmRequest, setConfirmRequest] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  const pendingCount = useMemo(
    () => deposits.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length,
    [deposits],
  );

  const total = deposits.length;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Keep the page in range when the list changes.
  useEffect(() => {
    setPage((current) => Math.min(current, totalPages));
  }, [totalPages]);

  const pagedDeposits = useMemo(
    () => deposits.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE),
    [deposits, page],
  );
  const rangeStart = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const rangeEnd = Math.min(page * PAGE_SIZE, total);

  const onSubmit = async () => {
    if (!/^\d+(\.\d+)?$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive number.");
      return;
    }

    try {
      const depositId = await registerDeposit(displayToBase(amount, fDecimals));
      toast.success(`Issuance request submitted: ${depositId}`);
      setAmount("0");
      setConfirmRequest(false);
    } catch (submitError) {
      toast.error(submitError instanceof Error ? submitError.message : "Unable to submit issuance request");
    }
  };

  const onPrepareSubmit = () => {
    if (!/^\d+(\.\d+)?$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive number.");
      return;
    }
    setConfirmRequest(true);
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <BalanceWidget balance={balance} decimals={tCeBMDecimals} symbol={tCeBMSymbol} loading={status === "loading" && balance === null} hideSymbol />
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Fiat Reserve Balance (fCeBM)</CardDescription>
            <CardTitle>{fiatBalance !== null ? formatTokenAmount(fiatBalance, fDecimals) : "—"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Issuance Requests</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Request Token Issuance</CardTitle>
          <CardDescription>Submit fiat collateral proof to request tCeBM issuance by the central bank.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-xs text-muted-foreground">
            Wallet: {walletAddress || "Not available in session"}
          </p>
          <div className="space-y-2">
            <Label htmlFor="deposit-amount">Amount ({fiatCurrencyLabel(fCeBMSymbol)})</Label>
            <Input
              id="deposit-amount"
              type="number"
              min="0"
              step="any"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={onPrepareSubmit} disabled={status === "loading"}>
              Review Issuance Request
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
            <CardTitle>Confirm Issuance Request</CardTitle>
            <CardDescription>{formatFiatUnits(amount, fDecimals, fCeBMSymbol)} will be submitted for central bank approval.</CardDescription>
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
          <CardTitle>Issuance Requests</CardTitle>
          <CardDescription>Total records: {deposits.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Fiat Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Issuance Reference</TableHead>
                <TableHead>Rejection Reason</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pagedDeposits.map((deposit) => (
                <TableRow key={deposit.id}>
                  <TableCell className="font-medium">{deposit.id}</TableCell>
                  <TableCell>{formatFiatUnits(deposit.amount, fDecimals, fCeBMSymbol)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(deposit.status)}>{getPaymentStatusLabel(deposit.status)}</Badge>
                  </TableCell>
                  <TableCell title={deposit.fiat_mint_tx_hash}>{shortHash(deposit.fiat_mint_tx_hash)}</TableCell>
                  <TableCell>{deposit.rejection_reason || "-"}</TableCell>
                  <TableCell>{new Date(deposit.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!total ? <p className="pt-3 text-sm text-muted-foreground">No issuance requests found.</p> : null}

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
