// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label } from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { ammPairsApi, circuitBreakerV2Api, type AmmPair } from "../services/api";
import { useCircuitBreakerV2Store } from "../features/circuit-breaker/circuit-breaker-v2.store";
import { useAuthStore } from "../stores";
import { usePolling } from "../hooks/usePolling";

const SELECT_CLASS =
  "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

// The circuit breaker is a Central Bank control, per-pair (one AMM instance per pair),
// with an asymmetric quorum: 1-of-N to pause, 2-of-N to resume. This portal drives the
// real /api/v2/governance/circuit-breaker/* endpoints against the pairs the CBs created.
export function CircuitBreakerPage() {
  const cbStatus = useCircuitBreakerV2Store((state) => state.cbStatus);
  const resumeRequestId = useCircuitBreakerV2Store((state) => state.resumeRequestId);
  const isStale = useCircuitBreakerV2Store((state) => state.isStale);
  const status = useCircuitBreakerV2Store((state) => state.status);
  const error = useCircuitBreakerV2Store((state) => state.error);
  const fetchStatus = useCircuitBreakerV2Store((state) => state.fetchStatus);
  const pause = useCircuitBreakerV2Store((state) => state.pause);
  const proposeResume = useCircuitBreakerV2Store((state) => state.proposeResume);
  const signResume = useCircuitBreakerV2Store((state) => state.signResume);
  const profile = useAuthStore((state) => state.profile);

  // This Central Bank's own identity, recorded as the pause/resume initiator. Derived
  // from the session (falls back to the baked institution name) — never free-typed.
  const bankId =
    (profile?.bankId ?? "").trim() || (import.meta.env.VITE_INSTITUTION_NAME ?? "").trim() || "central-bank";

  const [pairs, setPairs] = useState<AmmPair[]>([]);
  const [pair, setPair] = useState("");
  const [reasonCode, setReasonCode] = useState("INCIDENT_HIGH_VOLATILITY");
  const [resumeRequestToSign, setResumeRequestToSign] = useState("");

  // Load the real pairs once; default the selection to the first one.
  useEffect(() => {
    ammPairsApi
      .getPairs()
      .then((list) => {
        setPairs(list);
        setPair((current) => current || list[0]?.pair_id || "");
      })
      .catch(() => setPairs([]));
  }, []);

  useEffect(() => {
    if (pair) void fetchStatus(pair);
  }, [fetchStatus, pair]);

  // Auto-fill the resume request id discovered on-chain, so a Central Bank that did NOT
  // propose can co-sign the 2-of-N resume in one click (no id to look up out-of-band).
  useEffect(() => {
    if (cbStatus?.resume_request_id) setResumeRequestToSign(cbStatus.resume_request_id);
  }, [cbStatus?.resume_request_id]);

  usePolling(
    () => {
      if (pair) void fetchStatus(pair);
    },
    15000,
    true,
  );

  const onPause = async () => {
    await pause({ pair, bank_id: bankId, reason_code: reasonCode });
  };

  const onProposeResume = async () => {
    await proposeResume({ pair, bank_id: bankId });
  };

  const onSignResume = async () => {
    await signResume({ pair, request_id: resumeRequestToSign, bank_id: bankId });
  };

  const operationalStatus = cbStatus ? circuitBreakerV2Api.normalizeOperationalStatus(cbStatus, isStale) : null;
  const noPairs = pairs.length === 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Circuit Breaker</CardTitle>
        <CardDescription>
          Pause a pair immediately (1-of-N), then resume with the 2-of-N Central Bank quorum. Actions apply to the
          selected pair.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Plain-language explanation of what this control does */}
        <div className="rounded-lg border border-border bg-blue-50/60 p-4 text-sm text-muted-foreground">
          <p className="font-medium text-foreground">What is the circuit breaker?</p>
          <p className="mt-1">
            It is an emergency stop for a trading pair. If something looks wrong (fraud, extreme volatility, an
            incident), any single Central Bank can pause the pair right away — no one else has to agree first.
          </p>
          <p className="mt-2">
            Turning it back on is deliberately harder: it takes two different Central Banks to resume. One proposes the
            resume, a second one signs it, and only then does trading restart. The state you see here is read directly
            from the blockchain, so every Central Bank sees the same thing at the same time.
          </p>
        </div>

        {/* Pair selector + live status */}
        <div className="rounded-lg border border-border bg-muted/40 p-4">
          <div className="max-w-sm space-y-1">
            <Label htmlFor="cb-pair">Pair</Label>
            <select id="cb-pair" className={SELECT_CLASS} value={pair} onChange={(e) => setPair(e.target.value)}>
              {noPairs ? <option value="">No pairs created yet</option> : null}
              {pairs.map((p) => (
                <option key={p.pair_id} value={p.pair_id}>
                  {p.pair_id} ({p.status})
                </option>
              ))}
            </select>
          </div>
          <div className="mt-3 flex items-center gap-3">
            <p className="text-xs text-muted-foreground">Current state</p>
            <Badge
              variant={
                cbStatus?.state === "HALTED"
                  ? "destructive"
                  : cbStatus?.state === "RESUME_PENDING"
                    ? "warning"
                    : "default"
              }
              className="text-sm"
            >
              {cbStatus?.state ?? "LIVE"}
            </Badge>
          </div>
          {cbStatus?.resume_request_id ? (
            <p className="mt-2 text-xs text-muted-foreground">
              Resume proposal: <span className="font-mono">{cbStatus.resume_request_id.slice(0, 12)}…</span>
              {typeof cbStatus.resume_quorum === "number" && cbStatus.resume_quorum > 0
                ? ` · ${cbStatus.resume_signatures ?? 0}/${cbStatus.resume_quorum} signatures`
                : ""}
            </p>
          ) : null}
          {isStale ? (
            <p className="mt-2 text-xs text-amber-700">Showing last known state. Latest status refresh failed.</p>
          ) : null}
          {operationalStatus ? (
            <p className="mt-2 text-xs text-muted-foreground">Guidance: {operationalStatus.guidance}</p>
          ) : null}
          <p className="mt-2 text-xs text-muted-foreground">Acting as: {bankId}</p>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <div className="space-y-2 rounded-md border border-border p-3">
            <p className="text-sm font-medium">Pause (1-of-N)</p>
            <Label>Reason Code</Label>
            <Input value={reasonCode} onChange={(event) => setReasonCode(event.target.value)} />
            <Button variant="destructive" onClick={() => void onPause()} disabled={status === "loading" || !pair}>
              {status === "loading" ? "Submitting..." : "Pause"}
            </Button>
          </div>

          <div className="space-y-2 rounded-md border border-border p-3">
            <p className="text-sm font-medium">Propose Resume</p>
            <p className="text-xs text-muted-foreground">Opens a resume request (counts as the 1st of the 2-of-N).</p>
            <Button onClick={() => void onProposeResume()} disabled={status === "loading" || !pair}>
              {status === "loading" ? "Submitting..." : "Propose Resume"}
            </Button>
            {resumeRequestId ? <p className="text-xs text-muted-foreground">Created request: {resumeRequestId}</p> : null}
          </div>
        </div>

        <div className="space-y-2 rounded-md border border-border p-3">
          <p className="text-sm font-medium">Sign Resume Request (2-of-N)</p>
          <Label>Request ID</Label>
          <Input value={resumeRequestToSign} onChange={(event) => setResumeRequestToSign(event.target.value)} />
          <Button
            variant="secondary"
            onClick={() => void onSignResume()}
            disabled={status === "loading" || !pair || !resumeRequestToSign}
          >
            {status === "loading" ? "Submitting..." : "Sign Resume"}
          </Button>
          <p className="text-xs text-muted-foreground">Resume needs a second Central Bank to sign to reach quorum.</p>
        </div>

        <Button variant="outline" onClick={() => void fetchStatus(pair)} disabled={status === "loading" || !pair}>
          Refresh Status
        </Button>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}
