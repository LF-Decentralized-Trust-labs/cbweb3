// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
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
import { PaymentStatus, formatCeBM, getPaymentStatusLabel, getPaymentStatusVariant, normalizePaymentStatus } from "../types";

const shortHash = (value: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

export function EscrowsApprovalPage() {
  const escrows = usePaymentStore((state) => state.escrows);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);
  const fetchEscrows = usePaymentStore((state) => state.fetchEscrows);
  const approveEscrow = usePaymentStore((state) => state.approveEscrow);
  const rejectEscrow = usePaymentStore((state) => state.rejectEscrow);

  const [approveTargetId, setApproveTargetId] = useState<string | null>(null);
  const [rejectTargetId, setRejectTargetId] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  useEffect(() => {
    void fetchEscrows();
  }, [fetchEscrows]);

  const pendingCount = useMemo(
    () => escrows.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length,
    [escrows],
  );

  const onApprove = async () => {
    if (!approveTargetId) return;

    try {
      const result = await approveEscrow(approveTargetId);
      toast.success(`Tokenisation approved. Redemption: ${shortHash(result.burn_tx_hash)}, Issuance: ${shortHash(result.mint_tx_hash)}`);
      setApproveTargetId(null);
    } catch (approveError) {
      toast.error(approveError instanceof Error ? approveError.message : "Unable to approve tokenisation request");
    }
  };

  const onReject = async () => {
    if (!rejectTargetId) return;
    if (reason.trim().length < 3) {
      toast.error("Please provide a rejection reason.");
      return;
    }

    try {
      await rejectEscrow(rejectTargetId, reason.trim());
      toast.success("Tokenisation request rejected.");
      setRejectTargetId(null);
      setReason("");
    } catch (rejectError) {
      toast.error(rejectError instanceof Error ? rejectError.message : "Unable to reject tokenisation request");
    }
  };

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Tokenisation Requests</CardDescription>
            <CardTitle>{escrows.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Tokenisation Requests</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Tokenisation Request Queue</CardTitle>
          <CardDescription>Approve tCeBM issuance or reject with reason.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-3">
            <Button variant="outline" onClick={() => void fetchEscrows()} disabled={status === "loading"}>
              Refresh
            </Button>
          </div>
          <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Requester</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Redemption ID</TableHead>
                <TableHead>Issuance Ref</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {escrows.map((escrow) => (
                <TableRow key={escrow.id}>
                  <TableCell className="font-mono font-medium" title={escrow.id}>{shortHash(escrow.id)}</TableCell>
                  <TableCell className="font-mono" title={escrow.requester_id}>{shortHash(escrow.requester_id)}</TableCell>
                  <TableCell>{formatCeBM(escrow.amount)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(escrow.status)}>{getPaymentStatusLabel(escrow.status)}</Badge>
                  </TableCell>
                  <TableCell title={escrow.burn_tx_hash}>{shortHash(escrow.burn_tx_hash)}</TableCell>
                  <TableCell title={escrow.mint_tx_hash}>{shortHash(escrow.mint_tx_hash)}</TableCell>
                  <TableCell>{new Date(escrow.created_at).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    {normalizePaymentStatus(escrow.status) === PaymentStatus.PENDING ? (
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          onClick={() => {
                            setApproveTargetId(escrow.id);
                            setRejectTargetId(null);
                            setReason("");
                          }}
                          disabled={status === "loading"}
                        >
                          Approve
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setRejectTargetId(escrow.id);
                            setApproveTargetId(null);
                            setReason("");
                          }}
                        >
                          Reject
                        </Button>
                      </div>
                    ) : (
                      <span className="text-xs text-muted-foreground">No action</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!escrows.length ? <p className="pt-3 text-sm text-muted-foreground">No tokenisation requests found.</p> : null}
          {error ? <p className="pt-3 text-sm text-destructive">{error}</p> : null}
          </div>
        </CardContent>
      </Card>

      {approveTargetId ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Tokenisation Approval</CardTitle>
            <CardDescription>Request ID: {approveTargetId}</CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button onClick={() => void onApprove()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Confirm Approve"}
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
            <CardTitle>Reject Tokenisation Request</CardTitle>
            <CardDescription>Request ID: {rejectTargetId}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
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
