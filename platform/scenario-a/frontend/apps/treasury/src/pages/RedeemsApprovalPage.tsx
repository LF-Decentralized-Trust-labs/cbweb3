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

export function RedeemsApprovalPage() {
  const redeems = usePaymentStore((state) => state.redeems);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);
  const fetchRedeems = usePaymentStore((state) => state.fetchRedeems);
  const approveRedeem = usePaymentStore((state) => state.approveRedeem);
  const rejectRedeem = usePaymentStore((state) => state.rejectRedeem);

  const [approveTargetId, setApproveTargetId] = useState<string | null>(null);
  const [rejectTargetId, setRejectTargetId] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  useEffect(() => {
    void fetchRedeems();
  }, [fetchRedeems]);

  const pendingCount = useMemo(
    () => redeems.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length,
    [redeems],
  );

  const onApprove = async () => {
    if (!approveTargetId) return;

    try {
      const result = await approveRedeem(approveTargetId);
      toast.success(`Redeem approved. Redemption ID: ${shortHash(result.fiat_mint_tx_hash)}`);
      setApproveTargetId(null);
    } catch (approveError) {
      toast.error(approveError instanceof Error ? approveError.message : "Unable to approve redeem");
    }
  };

  const onReject = async () => {
    if (!rejectTargetId) return;
    if (reason.trim().length < 3) {
      toast.error("Please provide a rejection reason.");
      return;
    }

    try {
      await rejectRedeem(rejectTargetId, reason.trim());
      toast.success("Redeem rejected.");
      setRejectTargetId(null);
      setReason("");
    } catch (rejectError) {
      toast.error(rejectError instanceof Error ? rejectError.message : "Unable to reject redeem");
    }
  };

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Redeems</CardDescription>
            <CardTitle>{redeems.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Redeem Queue</CardTitle>
          <CardDescription>Approve fiat reserve release or reject with reason.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-3">
            <Button variant="outline" onClick={() => void fetchRedeems()} disabled={status === "loading"}>
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
                <TableHead>Zeto Transfer Tx Hash</TableHead>
                <TableHead>Redemption ID</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {redeems.map((redeem) => (
                <TableRow key={redeem.id}>
                  <TableCell className="font-mono font-medium" title={redeem.id}>{shortHash(redeem.id)}</TableCell>
                  <TableCell className="font-mono" title={redeem.requester_id}>{shortHash(redeem.requester_id)}</TableCell>
                  <TableCell>{formatCeBM(redeem.amount)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(redeem.status)}>{getPaymentStatusLabel(redeem.status)}</Badge>
                  </TableCell>
                  <TableCell title={redeem.zeto_transfer_tx_hash}>{shortHash(redeem.zeto_transfer_tx_hash)}</TableCell>
                  <TableCell title={redeem.fiat_mint_tx_hash}>{shortHash(redeem.fiat_mint_tx_hash)}</TableCell>
                  <TableCell>{new Date(redeem.created_at).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    {normalizePaymentStatus(redeem.status) === PaymentStatus.PENDING ? (
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          onClick={() => {
                            setApproveTargetId(redeem.id);
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
                            setRejectTargetId(redeem.id);
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
          {!redeems.length ? <p className="pt-3 text-sm text-muted-foreground">No redeems found.</p> : null}
          {error ? <p className="pt-3 text-sm text-destructive">{error}</p> : null}
          </div>
        </CardContent>
      </Card>

      {approveTargetId ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Redeem Approval</CardTitle>
            <CardDescription>Redeem ID: {approveTargetId}</CardDescription>
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
            <CardTitle>Reject Redeem</CardTitle>
            <CardDescription>Redeem ID: {rejectTargetId}</CardDescription>
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
