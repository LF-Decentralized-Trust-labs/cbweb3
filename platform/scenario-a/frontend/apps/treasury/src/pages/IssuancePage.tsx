import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, toast } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useFundingRequests, useIssuance } from "../hooks";

export function IssuancePage() {
  const { requests, fetchRequests } = useFundingRequests();
  const { mint, validateBurnToMint, validation, status, error } = useIssuance();
  const [requestId, setRequestId] = useState("");
  const [amount, setAmount] = useState("");
  const [reserveProofRef, setReserveProofRef] = useState("");

  useEffect(() => {
    void fetchRequests();
  }, [fetchRequests]);

  const approvedRequests = useMemo(() => requests.filter((request) => request.status === "APPROVED"), [requests]);
  const selectedRequest = approvedRequests.find((request) => request.id === requestId);

  const onValidate = async () => {
    if (!requestId || !amount) {
      toast.error("Select request and amount first");
      return;
    }
    await validateBurnToMint(requestId, amount);
  };

  const onMint = async () => {
    if (!selectedRequest || !validation?.isValid || !reserveProofRef.trim()) {
      toast.error("Validation and reserve proof are required");
      return;
    }
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
        <CardDescription>Minting is enabled only after approved request and burn-to-mint validation.</CardDescription>
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
          <Button variant="outline" onClick={() => void onValidate()} disabled={status === "loading"}>
            Validate burn-to-mint
          </Button>
          <Button onClick={() => void onMint()} disabled={status === "loading" || !validation?.isValid}>
            Execute mint
          </Button>
          <p className={`text-sm ${validation?.isValid ? "text-emerald-600" : "text-muted-foreground"}`}>
            {validation ? (validation.isValid ? "Validation passed" : validation.reason) : "Validation not run"}
          </p>
        </div>
        {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}
