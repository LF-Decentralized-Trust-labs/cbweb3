// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Label,
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useCircuitBreaker } from "../hooks";

export function CircuitBreakerPage() {
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
