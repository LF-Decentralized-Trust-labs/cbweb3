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
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { isScenarioB } from "../config/scenario";
import { circuitBreakerV2Api } from "../services/api/circuit-breaker-v2.api";
import { useCircuitBreakerV2Store } from "../features/circuit-breaker/circuit-breaker-v2.store";
import { useCircuitBreaker } from "../hooks";
import { usePolling } from "../hooks/usePolling";

export function CircuitBreakerPage() {
  if (isScenarioB) {
    return <CircuitBreakerPageV2 />;
  }

  return <CircuitBreakerPageScenarioA />;
}

function CircuitBreakerPageScenarioA() {
  const { circuitBreaker, status, error, fetchState, toggle } = useCircuitBreaker();
  const [reason, setReason] = useState("");
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    void fetchState();
  }, [fetchState]);

  const halted = circuitBreaker?.state === "HALTED";

  const onSubmit = async () => {
    if (reason.trim().length < 10) {
      toast.error("Reason must contain at least 10 characters.");
      return;
    }
    await toggle({ pause: !halted, reason });
    toast.success(halted ? "Swaps resumed" : "Swaps halted");
    setReason("");
    setConfirming(false);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Emergency Circuit Breaker</CardTitle>
        <CardDescription>Pause or resume all AMM swap operations across the hub.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-lg border border-border bg-muted/40 p-4">
          <p className="text-xs text-muted-foreground">Current state</p>
          <div className="mt-2 flex items-center gap-3">
            <Badge variant={halted ? "destructive" : "default"} className="text-sm">
              {circuitBreaker?.state ?? "LIVE"}
            </Badge>
            <span className="text-xs text-muted-foreground">
              Last update: {circuitBreaker?.updatedAt ? new Date(circuitBreaker.updatedAt).toLocaleString() : "—"}
            </span>
          </div>
        </div>

        <div className="space-y-2">
          <Label>Reason (required)</Label>
          <Textarea value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Describe why this intervention is required" />
        </div>

        {!confirming ? (
          <Button variant={halted ? "default" : "destructive"} onClick={() => setConfirming(true)}>
            {halted ? "Resume Swaps" : "Pause Swaps"}
          </Button>
        ) : (
          <div className="space-y-2 rounded-md border border-border p-3">
            <p className="text-sm font-medium">Confirm emergency action</p>
            <p className="text-xs text-muted-foreground">
              This action changes the global swap state and will be permanently recorded in audit logs.
            </p>
            <div className="flex gap-2">
              <Button variant={halted ? "default" : "destructive"} onClick={() => void onSubmit()} disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Confirm"}
              </Button>
              <Button variant="outline" onClick={() => setConfirming(false)}>
                Cancel
              </Button>
            </div>
          </div>
        )}

        {error ? <p className="text-sm text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}

function CircuitBreakerPageV2() {
  const cbStatus = useCircuitBreakerV2Store((state) => state.cbStatus);
  const resumeRequestId = useCircuitBreakerV2Store((state) => state.resumeRequestId);
  const isStale = useCircuitBreakerV2Store((state) => state.isStale);
  const status = useCircuitBreakerV2Store((state) => state.status);
  const error = useCircuitBreakerV2Store((state) => state.error);
  const fetchStatus = useCircuitBreakerV2Store((state) => state.fetchStatus);
  const pause = useCircuitBreakerV2Store((state) => state.pause);
  const proposeResume = useCircuitBreakerV2Store((state) => state.proposeResume);
  const signResume = useCircuitBreakerV2Store((state) => state.signResume);

  const [pair, setPair] = useState("BRL-USD");
  const [bankId, setBankId] = useState("central-bank-a");
  const [reasonCode, setReasonCode] = useState("INCIDENT_HIGH_VOLATILITY");
  const [signature, setSignature] = useState("AA==");

  const [resumeRequestToSign, setResumeRequestToSign] = useState("");
  const [resumeSignerId, setResumeSignerId] = useState("central-bank-b");
  const [resumeSignature, setResumeSignature] = useState("AA==");

  useEffect(() => {
    void fetchStatus(pair);
  }, [fetchStatus, pair]);

  usePolling(
    () => {
      void fetchStatus(pair);
    },
    15000,
    true,
  );

  const onPause = async () => {
    await pause({
      pair,
      bank_id: bankId,
      reason_code: reasonCode,
      signature,
    });
  };

  const onProposeResume = async () => {
    await proposeResume({
      pair,
      bank_id: bankId,
      signature,
    });
  };

  const onSignResume = async () => {
    await signResume({
      pair,
      request_id: resumeRequestToSign,
      bank_id: resumeSignerId,
      signature: resumeSignature,
    });
  };

  const operationalStatus = cbStatus
    ? circuitBreakerV2Api.normalizeOperationalStatus(cbStatus, isStale)
    : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Circuit Breaker Governance (Scenario B)</CardTitle>
        <CardDescription>Pause immediately, then resume with multi-party signatures.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-lg border border-border bg-muted/40 p-4">
          <p className="text-xs text-muted-foreground">Current state</p>
          <div className="mt-2 flex items-center gap-3">
            <Badge variant={cbStatus?.state === "HALTED" ? "destructive" : "default"} className="text-sm">
              {cbStatus?.state ?? "LIVE"}
            </Badge>
            <span className="text-xs text-muted-foreground">Pair: {cbStatus?.pair ?? pair}</span>
          </div>
          {cbStatus?.resume_request_id ? (
            <p className="mt-2 text-xs text-muted-foreground">Active resume request: {cbStatus.resume_request_id}</p>
          ) : null}
          {isStale ? (
            <p className="mt-2 text-xs text-amber-700">Showing last known state. Latest status refresh failed.</p>
          ) : null}
          {operationalStatus ? (
            <p className="mt-2 text-xs text-muted-foreground">Guidance: {operationalStatus.guidance}</p>
          ) : null}
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <div className="space-y-2 rounded-md border border-border p-3">
            <p className="text-sm font-medium">Pause Circuit Breaker</p>
            <Label>Pair</Label>
            <Input value={pair} onChange={(event) => setPair(event.target.value)} />
            <Label>Bank ID</Label>
            <Input value={bankId} onChange={(event) => setBankId(event.target.value)} />
            <Label>Reason Code</Label>
            <Input value={reasonCode} onChange={(event) => setReasonCode(event.target.value)} />
            <Label>Institutional Signature (base64)</Label>
            <Input value={signature} onChange={(event) => setSignature(event.target.value)} />
            <Button variant="destructive" onClick={() => void onPause()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Pause"}
            </Button>
          </div>

          <div className="space-y-2 rounded-md border border-border p-3">
            <p className="text-sm font-medium">Propose Resume</p>
            <Label>Pair</Label>
            <Input value={pair} onChange={(event) => setPair(event.target.value)} />
            <Label>Bank ID</Label>
            <Input value={bankId} onChange={(event) => setBankId(event.target.value)} />
            <Label>Institutional Signature (base64)</Label>
            <Input value={signature} onChange={(event) => setSignature(event.target.value)} />
            <Button onClick={() => void onProposeResume()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Propose Resume"}
            </Button>
            {resumeRequestId ? <p className="text-xs text-muted-foreground">Created request: {resumeRequestId}</p> : null}
          </div>
        </div>

        <div className="space-y-2 rounded-md border border-border p-3">
          <p className="text-sm font-medium">Sign Resume Request</p>
          <Label>Pair</Label>
          <Input value={pair} onChange={(event) => setPair(event.target.value)} />
          <Label>Request ID</Label>
          <Input value={resumeRequestToSign} onChange={(event) => setResumeRequestToSign(event.target.value)} />
          <Label>Signer Bank ID</Label>
          <Input value={resumeSignerId} onChange={(event) => setResumeSignerId(event.target.value)} />
          <Label>Institutional Signature (base64)</Label>
          <Input value={resumeSignature} onChange={(event) => setResumeSignature(event.target.value)} />
          <Button variant="secondary" onClick={() => void onSignResume()} disabled={status === "loading" || !resumeRequestToSign}>
            {status === "loading" ? "Submitting..." : "Sign Resume"}
          </Button>
        </div>

        <Button variant="outline" onClick={() => void fetchStatus(pair)} disabled={status === "loading"}>
          Refresh Status
        </Button>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}
