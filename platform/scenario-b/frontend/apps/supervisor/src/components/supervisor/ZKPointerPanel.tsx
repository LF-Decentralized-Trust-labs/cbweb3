// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
  Label,
} from "@cbweb3/ui";
import { useState } from "react";
import { zkPointerApi } from "../../services/api";
import { ApiError } from "../../services/api/apiClient";
import type { ZKPointerVerification } from "../../types";

const STATE_VARIANT: Record<string, "success" | "destructive" | "warning" | "default"> = {
  VALID: "success",
  EXPIRED: "warning",
  INVALID: "destructive",
};

export function ZKPointerPanel() {
  const [bankId, setBankId] = useState("");
  const [commitmentHash, setCommitmentHash] = useState("");
  const [result, setResult] = useState<ZKPointerVerification | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleVerify = async () => {
    setError(null);
    setResult(null);
    setLoading(true);
    try {
      const data = await zkPointerApi.verify(bankId.trim(), commitmentHash.trim());
      setResult(data);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setResult({ bankId: bankId.trim(), pointerId: "", commitmentHash: commitmentHash.trim(), state: "INVALID", expiresAt: null });
      } else {
        setError(err instanceof Error ? err.message : "Verification failed");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>ZK Pointer Verification</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="zk-bank-id">Bank ID</Label>
            <Input
              id="zk-bank-id"
              placeholder="e.g. cb_spoke_a"
              value={bankId}
              onChange={(e) => setBankId(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="zk-commitment">Commitment Hash</Label>
            <Input
              id="zk-commitment"
              placeholder="0x..."
              value={commitmentHash}
              onChange={(e) => setCommitmentHash(e.target.value)}
            />
          </div>
        </div>

        <Button
          onClick={() => void handleVerify()}
          disabled={loading || bankId.trim() === "" || commitmentHash.trim() === ""}
        >
          {loading ? "Verifying…" : "Verify"}
        </Button>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        {result ? (
          <dl className="rounded-md border border-border bg-muted/20 p-3 text-sm space-y-2">
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">State</dt>
              <dd>
                <Badge variant={STATE_VARIANT[result.state] ?? "default"}>{result.state}</Badge>
              </dd>
            </div>
            {result.pointerId ? (
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Pointer ID</dt>
                <dd className="font-mono text-xs">{result.pointerId}</dd>
              </div>
            ) : null}
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">Commitment</dt>
              <dd className="font-mono text-xs truncate max-w-[200px]" title={result.commitmentHash}>
                {result.commitmentHash}
              </dd>
            </div>
            {result.expiresAt ? (
              <div className="flex justify-between gap-4">
                <dt className="text-muted-foreground">Expires At</dt>
                <dd>{new Date(result.expiresAt).toLocaleString()}</dd>
              </div>
            ) : null}
          </dl>
        ) : null}
      </CardContent>
    </Card>
  );
}
