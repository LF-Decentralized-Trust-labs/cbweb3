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
  toast,
} from "@cbweb3/ui";
import { useState } from "react";
import { apiFetch } from "../../services/api/apiClient";

interface KYCStatusResponse {
  subject: string;
  status: string;
  last_updated_at?: string;
  freeze_reason?: string;
}

const STATUS_VARIANT: Record<string, "default" | "secondary" | "destructive" | "warning"> = {
  ACTIVE: "default",
  APPROVED: "default",
  PENDING: "secondary",
  FROZEN: "destructive",
  REVOKED: "destructive",
  REJECTED: "destructive",
};

export function KYCStatusPanel() {
  const [subject, setSubject] = useState("");
  const [result, setResult] = useState<KYCStatusResponse | null>(null);
  const [loading, setLoading] = useState(false);

  const handleLookup = async () => {
    const id = subject.trim();
    if (!id) return;
    setLoading(true);
    setResult(null);
    try {
      const data = await apiFetch<KYCStatusResponse>(
        `/api/v1/compliance/kyc/status/${encodeURIComponent(id)}`,
      );
      setResult(data);
    } catch {
      toast.error("Subject not found or access denied");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Credential Verification</CardTitle>
        <CardDescription>Look up a participant's KYC/credential lifecycle status by subject ID.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="space-y-1">
          <Label htmlFor="kyc-subject">Subject ID</Label>
          <div className="flex gap-2">
            <Input
              id="kyc-subject"
              placeholder="user-uuid or DID"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") void handleLookup(); }}
            />
            <Button onClick={() => void handleLookup()} disabled={loading || !subject.trim()}>
              {loading ? "Checking..." : "Verify"}
            </Button>
          </div>
        </div>

        {result && (
          <div className="rounded-md border border-border p-3 space-y-2 text-sm">
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">Subject:</span>
              <span className="font-mono text-xs">{result.subject}</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">Status:</span>
              <Badge variant={STATUS_VARIANT[result.status] ?? "secondary"}>{result.status}</Badge>
            </div>
            {result.last_updated_at && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">Last updated:</span>
                <span>{new Date(result.last_updated_at).toLocaleString()}</span>
              </div>
            )}
            {result.freeze_reason && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">Freeze reason:</span>
                <span className="text-destructive">{result.freeze_reason}</span>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
