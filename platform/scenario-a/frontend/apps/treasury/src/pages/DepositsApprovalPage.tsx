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
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { usePaymentStore } from "../stores";
import {
  PaymentStatus,
  formatFiatUnits,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
} from "../types";

const shortHash = (value: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

export function DepositsApprovalPage() {
  const deposits = usePaymentStore((state) => state.deposits);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);
  const fetchDeposits = usePaymentStore((state) => state.fetchDeposits);
  const approveDeposit = usePaymentStore((state) => state.approveDeposit);
  const rejectDeposit = usePaymentStore((state) => state.rejectDeposit);
  const requestFiatExchange = usePaymentStore((state) => state.requestFiatExchange);

  const [requesterFilter, setRequesterFilter] = useState("");
  const [approveTargetId, setApproveTargetId] = useState<string | null>(null);
  const [rejectTargetId, setRejectTargetId] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  useEffect(() => {
    void fetchDeposits();
  }, [fetchDeposits]);

  const pendingCount = useMemo(
    () => deposits.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length,
    [deposits],
  );

  const onApplyFilter = async () => {
    await fetchDeposits(requesterFilter.trim() || undefined);
  };

  const onApproveDeposit = async () => {
    if (!approveTargetId) return;

    try {
      await approveDeposit(approveTargetId);
      const exchangeResult = await requestFiatExchange(approveTargetId);
      toast.success(`Issuance request approved and fiat exchanged. Tx: ${shortHash(exchangeResult.tx_hash)}`);
      setApproveTargetId(null);
    } catch (approveError) {
      toast.error(approveError instanceof Error ? approveError.message : "Unable to approve and exchange issuance request");
    }
  };

  const onRetryFiatExchange = async (depositId: string) => {
    try {
      const exchangeResult = await requestFiatExchange(depositId);
      toast.success(`Fiat exchange retried successfully. Tx: ${shortHash(exchangeResult.tx_hash)}`);
    } catch (retryError) {
      toast.error(retryError instanceof Error ? retryError.message : "Unable to retry fiat exchange");
    }
  };

  const onReject = async () => {
    if (!rejectTargetId) return;
    if (reason.trim().length < 3) {
      toast.error("Please provide a rejection reason.");
      return;
    }

    try {
      await rejectDeposit(rejectTargetId, reason.trim());
      toast.success("Issuance request rejected.");
      setRejectTargetId(null);
      setReason("");
    } catch (rejectError) {
      toast.error(rejectError instanceof Error ? rejectError.message : "Unable to reject issuance request");
    }
  };

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Issuance Requests</CardDescription>
            <CardTitle>{deposits.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Issuance Requests</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Filter Issuance Requests</CardTitle>
          <CardDescription>Optional filter by requester_id.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="requester-filter">Requester ID</Label>
            <Input
              id="requester-filter"
              placeholder="bank-a-client"
              value={requesterFilter}
              onChange={(event) => setRequesterFilter(event.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => void onApplyFilter()} disabled={status === "loading"}>
              {status === "loading" ? "Filtering..." : "Apply Filter"}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setRequesterFilter("");
                void fetchDeposits();
              }}
              disabled={status === "loading"}
            >
              Clear Filter
            </Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Issuance Request Queue</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Requester</TableHead>
                <TableHead>Fiat Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Issuance Reference</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deposits.map((deposit) => (
                <TableRow key={deposit.id}>
                  <TableCell className="font-mono font-medium" title={deposit.id}>{shortHash(deposit.id)}</TableCell>
                  <TableCell className="font-mono" title={deposit.requester_id}>{shortHash(deposit.requester_id)}</TableCell>
                  <TableCell>{formatFiatUnits(deposit.amount)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(deposit.status)}>{getPaymentStatusLabel(deposit.status)}</Badge>
                  </TableCell>
                  <TableCell title={deposit.mint_tx_hash}>{shortHash(deposit.mint_tx_hash)}</TableCell>
                  <TableCell>{new Date(deposit.created_at).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    {normalizePaymentStatus(deposit.status) === PaymentStatus.PENDING ? (
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          onClick={() => {
                            setApproveTargetId(deposit.id);
                            setRejectTargetId(null);
                          }}
                          disabled={status === "loading"}
                        >
                          Approve Issuance
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setRejectTargetId(deposit.id);
                            setApproveTargetId(null);
                            setReason("");
                          }}
                        >
                          Reject
                        </Button>
                      </div>
                    ) : normalizePaymentStatus(deposit.status) === PaymentStatus.MINT_FAILED ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void onRetryFiatExchange(deposit.id)}
                        disabled={status === "loading"}
                      >
                        Retry Mint
                      </Button>
                    ) : (
                      <span className="text-xs text-muted-foreground">No action</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!deposits.length ? <p className="pt-3 text-sm text-muted-foreground">No issuance requests found.</p> : null}
          </div>
        </CardContent>
      </Card>

      {approveTargetId ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Issuance Approval + Fiat Exchange</CardTitle>
            <CardDescription>Request ID: {approveTargetId}</CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button onClick={() => void onApproveDeposit()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Confirm Action"}
            </Button>
            <Button variant="outline" onClick={() => setApproveTargetId(null)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {rejectTargetId ? (
        <Card>
          <CardHeader>
            <CardTitle>Reject Issuance Request</CardTitle>
            <CardDescription>Request ID: {rejectTargetId}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="deposit-reason">Reason</Label>
              <Textarea id="deposit-reason" value={reason} onChange={(event) => setReason(event.target.value)} />
            </div>
            <div className="flex gap-2">
              <Button variant="destructive" onClick={() => void onReject()} disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Confirm Reject"}
              </Button>
              <Button variant="outline" onClick={() => setRejectTargetId(null)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
