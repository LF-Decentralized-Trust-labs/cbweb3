// SPDX-License-Identifier: Apache-2.0

import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, ConfirmActionDialog, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, toast } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useFundingRequests, useIssuance } from "../hooks";

export function IssuancePage() {
  const { requests, fetchRequests } = useFundingRequests();
  const { mint, status, error } = useIssuance();
  const [requestId, setRequestId] = useState("");
  const [amount, setAmount] = useState("");
  const [reserveProofRef, setReserveProofRef] = useState("");
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    void fetchRequests();
  }, [fetchRequests]);

  const approvedRequests = useMemo(() => requests.filter((request) => request.status === "APPROVED"), [requests]);
  const selectedRequest = approvedRequests.find((request) => request.id === requestId);

  // The "Validate burn-to-mint" step that used to live here was removed: its
  // implementation always returned isValid:true while this screen presented it
  // as a real check and gated the mint button on it (finding R2-M-8). The
  // preconditions that remain are real ones — an approved request must be
  // selected and a reserve proof reference supplied — and the operator now
  // confirms the exact request that will be sent.
  const onRequestMint = () => {
    if (!selectedRequest || !amount || !reserveProofRef.trim()) {
      toast.error("Approved request, amount and reserve proof are required");
      return;
    }
    setConfirming(true);
  };

  const onConfirmMint = async () => {
    if (!selectedRequest) {
      return;
    }
    setConfirming(false);
    await mint({
      requestId,
      targetInstitutionId: selectedRequest.institutionId,
      amount,
      reserveProofRef,
    });
    toast.success("Mint operation submitted");
    setAmount("");
    setReserveProofRef("");
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Issuance (Mint)</CardTitle>
        <CardDescription>Minting requires an approved funding request and a reserve proof reference, both recorded in the audit trail.</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label>Approved request</Label>
          <Select value={requestId} onValueChange={setRequestId}>
            <SelectTrigger>
              <SelectValue placeholder="Select request" />
            </SelectTrigger>
            <SelectContent>
              {approvedRequests.map((request) => (
                <SelectItem key={request.id} value={request.id}>
                  {request.id} · {request.institutionName}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="amount">Amount</Label>
          <Input id="amount" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="e.g. 100000" />
        </div>
        <div className="space-y-2 md:col-span-2">
          <Label htmlFor="proof">Reserve proof reference</Label>
          <Input id="proof" value={reserveProofRef} onChange={(event) => setReserveProofRef(event.target.value)} placeholder="proof-2026-03-999" />
        </div>
        <div className="md:col-span-2 flex flex-wrap items-center gap-2">
          <Button onClick={onRequestMint} disabled={status === "loading"}>
            Execute mint
          </Button>
        </div>
        {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}
      </CardContent>
      <ConfirmActionDialog
        open={confirming}
        onOpenChange={setConfirming}
        title="Confirm issuance"
        description="This issues new tCeBM to the institution below. The operation is recorded in the audit trail."
        fields={[
          { label: "Funding request", value: requestId },
          { label: "Institution", value: selectedRequest?.institutionName ?? selectedRequest?.institutionId ?? "-" },
          { label: "Amount", value: amount },
          { label: "Reserve proof", value: reserveProofRef },
        ]}
        confirmLabel="Confirm mint"
        busy={status === "loading"}
        onConfirm={() => void onConfirmMint()}
      />
    </Card>
  );
}
