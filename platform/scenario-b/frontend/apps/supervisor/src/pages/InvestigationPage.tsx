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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  toast,
} from "@cbweb3/ui";
import { useState } from "react";
import type { DisclosureReasonCode, DisclosureRequest } from "../types";
import { oversightApi } from "../services/api";

const REASON_CODES: { value: DisclosureReasonCode; label: string }[] = [
  { value: "AML_ALERT", label: "AML Alert" },
  { value: "CFT_INVESTIGATION", label: "CFT Investigation" },
  { value: "COURT_ORDER", label: "Court Order" },
  { value: "REGULATORY_EXAM", label: "Regulatory Exam" },
];

const STATE_VARIANTS: Record<string, "default" | "secondary" | "destructive" | "warning"> = {
  PENDING: "secondary",
  QUORUM_REACHED: "default",
  EXPIRED: "destructive",
};

function DisclosureCard({ disclosure }: { disclosure: DisclosureRequest }) {
  return (
    <div className="rounded-md border border-border p-3 space-y-1 text-sm">
      <div className="flex items-center gap-2">
        <span className="font-medium">{disclosure.requestId}</span>
        <Badge variant={STATE_VARIANTS[disclosure.state] ?? "secondary"}>{disclosure.state}</Badge>
      </div>
      <p className="text-muted-foreground">Tx: {disclosure.targetTransactionRef}</p>
      <p className="text-muted-foreground">Reason: {disclosure.reasonCode}</p>
      <p className="text-muted-foreground">
        Quorum: {disclosure.quorumReached}/{disclosure.quorumRequired}
      </p>
      <p className="text-muted-foreground">Expires: {new Date(disclosure.expiresAt).toLocaleString()}</p>
    </div>
  );
}

export function InvestigationPage() {
  // Open disclosure form
  const [txRef, setTxRef] = useState("");
  const [requestorId, setRequestorId] = useState("");
  const [reasonCode, setReasonCode] = useState<DisclosureReasonCode>("AML_ALERT");
  const [openStatus, setOpenStatus] = useState<"idle" | "loading" | "error">("idle");
  const [openResult, setOpenResult] = useState<DisclosureRequest | null>(null);

  // Sign disclosure form
  const [signRequestId, setSignRequestId] = useState("");
  const [signerId, setSignerId] = useState("");
  const [signStatus, setSignStatus] = useState<"idle" | "loading" | "error">("idle");

  // Status lookup form
  const [lookupId, setLookupId] = useState("");
  const [lookupStatus, setLookupStatus] = useState<"idle" | "loading" | "error">("idle");
  const [lookupResult, setLookupResult] = useState<DisclosureRequest | null>(null);

  const handleOpen = async () => {
    if (!txRef || !requestorId) {
      toast.error("Missing fields", { description: "Tx reference and requestor ID are required." });
      return;
    }
    setOpenStatus("loading");
    try {
      const result = await oversightApi.openDisclosure({ txRef, requestorId, reasonCode });
      setOpenResult(result);
      setOpenStatus("idle");
      toast.success("Disclosure request created", { description: `Request ID: ${result.requestId}` });
    } catch {
      setOpenStatus("error");
      toast.error("Failed to open disclosure");
    }
  };

  const handleSign = async () => {
    if (!signRequestId || !signerId) {
      toast.error("Missing fields", { description: "Request ID and signer ID are required." });
      return;
    }
    setSignStatus("loading");
    try {
      await oversightApi.signDisclosure({ requestId: signRequestId, signerId });
      setSignStatus("idle");
      toast.success("Signature recorded");
    } catch {
      setSignStatus("error");
      toast.error("Failed to sign disclosure");
    }
  };

  const handleLookup = async () => {
    if (!lookupId) return;
    setLookupStatus("loading");
    setLookupResult(null);
    try {
      const result = await oversightApi.getDisclosureStatus(lookupId);
      setLookupResult(result);
      setLookupStatus("idle");
    } catch {
      setLookupStatus("error");
      toast.error("Request not found or access denied");
    }
  };

  return (
    <div className="space-y-4">
      {/* Open Disclosure */}
      <Card>
        <CardHeader>
          <CardTitle>Open Disclosure Request</CardTitle>
          <CardDescription>Initiate a 2-of-N AML/CFT disclosure workflow. The request expires in 72 hours.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1">
              <Label htmlFor="tx-ref">Transaction Reference</Label>
              <Input id="tx-ref" placeholder="0xabc..." value={txRef} onChange={(e) => setTxRef(e.target.value)} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="requestor-id">Requestor Bank ID</Label>
              <Input id="requestor-id" placeholder="cb_lnet" value={requestorId} onChange={(e) => setRequestorId(e.target.value)} />
            </div>
          </div>
          <div className="space-y-1">
            <Label>Reason Code</Label>
            <Select value={reasonCode} onValueChange={(v) => setReasonCode(v as DisclosureReasonCode)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {REASON_CODES.map((r) => (
                  <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button onClick={() => void handleOpen()} disabled={openStatus === "loading"}>
            {openStatus === "loading" ? "Opening..." : "Open Request"}
          </Button>
          {openResult && <DisclosureCard disclosure={openResult} />}
        </CardContent>
      </Card>

      {/* Sign Disclosure */}
      <Card>
        <CardHeader>
          <CardTitle>Co-sign Disclosure Request</CardTitle>
          <CardDescription>Add your co-signature to an existing PENDING disclosure request.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1">
              <Label htmlFor="sign-req-id">Request ID</Label>
              <Input id="sign-req-id" placeholder="req-uuid" value={signRequestId} onChange={(e) => setSignRequestId(e.target.value)} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="signer-id">Signer Bank ID</Label>
              <Input id="signer-id" placeholder="cb_spoke_a" value={signerId} onChange={(e) => setSignerId(e.target.value)} />
            </div>
          </div>
          <Button onClick={() => void handleSign()} disabled={signStatus === "loading"}>
            {signStatus === "loading" ? "Signing..." : "Sign Request"}
          </Button>
        </CardContent>
      </Card>

      {/* Status Lookup */}
      <Card>
        <CardHeader>
          <CardTitle>Disclosure Status Lookup</CardTitle>
          <CardDescription>Retrieve the current state and quorum progress of a disclosure request.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Input placeholder="Request ID" value={lookupId} onChange={(e) => setLookupId(e.target.value)} />
            <Button onClick={() => void handleLookup()} disabled={lookupStatus === "loading"}>
              {lookupStatus === "loading" ? "Searching..." : "Look up"}
            </Button>
          </div>
          {lookupResult && <DisclosureCard disclosure={lookupResult} />}
        </CardContent>
      </Card>
    </div>
  );
}
