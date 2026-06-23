// SPDX-License-Identifier: Apache-2.0

import { Button, Card, CardContent, CardHeader, CardTitle, Input, Label, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, Textarea, toast } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useFundingRequests } from "../hooks";
import type { FundingRequest } from "../types";

const formatAmount = (value: string) => Number(value).toLocaleString();

export function FundingRequestsPage() {
  const { requests, fetchRequests, approveRequest, rejectRequest, status, error } = useFundingRequests();
  const [query, setQuery] = useState("");
  const [selectedRequest, setSelectedRequest] = useState<FundingRequest | null>(null);
  const [reason, setReason] = useState("");

  useEffect(() => {
    void fetchRequests();
  }, [fetchRequests]);

  const filtered = useMemo(
    () => requests.filter((request) => request.institutionName.toLowerCase().includes(query.toLowerCase()) || request.id.includes(query)),
    [requests, query],
  );

  const onApprove = async () => {
    if (!selectedRequest) {
      return;
    }
    await approveRequest(selectedRequest.id, reason || "Approved by Treasury review");
    toast.success("Funding request approved");
    setReason("");
  };

  const onReject = async () => {
    if (!selectedRequest || !reason.trim()) {
      toast.error("Rejection reason is required");
      return;
    }
    await rejectRequest(selectedRequest.id, reason);
    toast.success("Funding request rejected");
    setReason("");
  };

  return (
    <div className="grid gap-4 xl:grid-cols-[2fr_1fr]">
      <Card>
        <CardHeader>
          <CardTitle>Funding Request Queue</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="max-w-sm space-y-2">
            <Label htmlFor="search">Search by institution or request id</Label>
            <Input id="search" value={query} onChange={(event) => setQuery(event.target.value)} />
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Request</TableHead>
                <TableHead>Institution</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((request) => (
                <TableRow key={request.id} className="cursor-pointer" onClick={() => setSelectedRequest(request)}>
                  <TableCell>{request.id}</TableCell>
                  <TableCell>{request.institutionName}</TableCell>
                  <TableCell>{formatAmount(request.amount)}</TableCell>
                  <TableCell>{request.status}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Review</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">{selectedRequest ? `${selectedRequest.id} · ${selectedRequest.institutionName}` : "Select a request"}</p>
          <div className="space-y-2">
            <Label htmlFor="reason">Review reason</Label>
            <Textarea id="reason" rows={5} value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Required for rejection" />
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          <div className="flex gap-2">
            <Button onClick={() => void onApprove()} disabled={!selectedRequest || status === "loading"}>
              Approve
            </Button>
            <Button variant="destructive" onClick={() => void onReject()} disabled={!selectedRequest || status === "loading"}>
              Reject
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
