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
import { usePaymentStore } from "../stores";
import {
  PaymentStatus,
  displayToBase,
  fiatCurrencyLabel,
  formatCeBM,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
  tokenUnitPrefix,
} from "../types";

const shortHash = (value: string) =>
  value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-";

const PAGE_SIZE = 10;

export function RedeemsPage() {
  const fetchAll = usePaymentStore((state) => state.fetchAll);
  const requestRedeem = usePaymentStore((state) => state.requestRedeem);
  const redeems = usePaymentStore((state) => state.redeems);
  const balance = usePaymentStore((state) => state.balance);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const tCeBMSymbol = usePaymentStore((state) => state.tCeBMSymbol);
  const fCeBMSymbol = usePaymentStore((state) => state.fCeBMSymbol);
  // Instrument name follows the on-chain symbol (custom per spoke). The fiat side of a
  // redeem is stated as the currency code (fiatCurrencyLabel), not as the fCeBM name.
  const tCeBMName = tokenUnitPrefix(tCeBMSymbol);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);
  const decimals = tCeBMDecimals ?? 18;

  const [amount, setAmount] = useState("0");
  const [confirmRequest, setConfirmRequest] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  const pendingCount = useMemo(
    () =>
      redeems.filter(
        (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
      ).length,
    [redeems],
  );

  const total = redeems.length;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  useEffect(() => {
    setPage((current) => Math.min(current, totalPages));
  }, [totalPages]);

  const pagedRedeems = useMemo(() => redeems.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [redeems, page]);
  const rangeStart = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const rangeEnd = Math.min(page * PAGE_SIZE, total);

  const onSubmit = async () => {
    if (!/^\d+(\.\d+)?$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive number.");
      return;
    }

    try {
      const redeemId = await requestRedeem(displayToBase(amount, decimals));
      toast.success(`Redeem request submitted: ${redeemId}`);
      setAmount("0");
      setConfirmRequest(false);
    } catch (submitError) {
      toast.error(
        submitError instanceof Error
          ? submitError.message
          : "Unable to request redeem",
      );
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
        <BalanceWidget
          balance={balance}
          decimals={tCeBMDecimals}
          symbol={tCeBMSymbol}
          loading={status === "loading" && balance === null}
        />
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Redeem {tCeBMName} ({tCeBMName} to {fiatCurrencyLabel(fCeBMSymbol)})</CardTitle>
          <CardDescription>
            Submit a redemption request to the central bank.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="redeem-amount">Amount ({tCeBMName})</Label>
            <Input
              id="redeem-amount"
              type="number"
              min="0"
              step="any"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={onPrepareSubmit} disabled={status === "loading"}>
              Review Redeem Request
            </Button>
            <Button
              variant="outline"
              onClick={() => void fetchAll()}
              disabled={status === "loading"}
            >
              Refresh
            </Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {confirmRequest ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Redeem Request</CardTitle>
            <CardDescription>
              {formatCeBM(amount, decimals, tCeBMSymbol)} will be submitted for central bank fiat
              reserve release approval.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button
              onClick={() => void onSubmit()}
              disabled={status === "loading"}
            >
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
          <CardTitle>Redeem Requests</CardTitle>
          <CardDescription>Total records: {redeems.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Redemption Tx</TableHead>
                <TableHead>Rejection Reason</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pagedRedeems.map((redeem) => (
                <TableRow key={redeem.id}>
                  <TableCell className="font-medium">{redeem.id}</TableCell>
                  <TableCell>{formatCeBM(redeem.amount, decimals, tCeBMSymbol)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(redeem.status)}>
                      {getPaymentStatusLabel(redeem.status)}
                    </Badge>
                  </TableCell>
                  <TableCell title={redeem.mint_tx_hash}>
                    {shortHash(redeem.mint_tx_hash)}
                  </TableCell>
                  <TableCell>{redeem.rejection_reason || "-"}</TableCell>
                  <TableCell>
                    {new Date(redeem.created_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!total ? (
            <p className="pt-3 text-sm text-muted-foreground">
              No redeems found.
            </p>
          ) : null}

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
