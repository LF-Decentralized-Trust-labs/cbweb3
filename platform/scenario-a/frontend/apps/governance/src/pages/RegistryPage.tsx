// SPDX-License-Identifier: Apache-2.0

import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Label,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useAuth, useRegistry } from "../hooks";
import { hasAdmissionAccess } from "../auth/authorization";
import {
  approvalReasonIssue,
  approvalReasonLength,
  isApprovalReasonAcceptable,
} from "../features/onboarding/approval-reason";

// const statusVariant = {
//   ACTIVE: "default",
//   PENDING: "warning",
//   CREDENTIAL_REQUESTED: "warning",
//   KYC_APPROVED: "default",
//   REVOKED: "destructive",
//   FROZEN: "secondary",
// } as const;

export function RegistryPage() {
  const {pendingKyc, fetch, fetchPendingKyc, approveKyc, kycStatus, error } = useRegistry();
  // Approving KYC is Admission-exclusive (spec 042 FR-002/FR-003). A governance-only
  // operator keeps the read views but must not be offered the control — the gateway
  // would 403 it anyway, so showing it would only produce a dead button.
  const { profile } = useAuth();
  const canApprove = hasAdmissionAccess(profile);
  // const [search, setSearch] = useState("");
  // const [entityName, setEntityName] = useState("");
  // const [legalEntityId, setLegalEntityId] = useState("");
  // const [scopes, setScopes] = useState("DEPOSIT,TRANSFER,SWAP");
  // const [reason, setReason] = useState("");
  // const [confirming, setConfirming] = useState(false);
  const [approvalReasonBySubject, setApprovalReasonBySubject] = useState<Record<string, string>>({});

  useEffect(() => {
    void fetch();
  }, [fetch]);

  useEffect(() => {
    const interval = window.setInterval(() => {
      void fetchPendingKyc();
    }, 15_000);

    return () => window.clearInterval(interval);
  }, [fetchPendingKyc]);

  // const filtered = useMemo(() => {
  //   const term = search.toLowerCase().trim();
  //   if (!term) return participants;
  //   return participants.filter(
  //     (participant) => participant.name.toLowerCase().includes(term) || participant.status.toLowerCase().includes(term),
  //   );
  // }, [participants, search]);

  // const onIssue = async () => {
  //   if (!entityName || !legalEntityId || !reason || reason.length < 10) {
  //     toast.error("Complete all fields and provide a reason (min 10 chars)");
  //     return;
  //   }
  //   await issueCredential({
  //     entityName,
  //     legalEntityId,
  //     scopes: scopes.split(",").map((scope) => scope.trim()).filter(Boolean),
  //     reason,
  //   });
  //   toast.success("Credential issued successfully");
  //   setEntityName("");
  //   setLegalEntityId("");
  //   setReason("");
  //   setConfirming(false);
  // };

  const onApproveKyc = async (subject: string) => {
    const rowReason = (approvalReasonBySubject[subject] ?? "").trim();
    // The button is disabled while the reason is unacceptable, so this is belt and braces — a
    // disabled control is a hint, not an enforcement. Both paths read the same rule, so the refusal
    // and the button can never describe different requirements.
    const issue = approvalReasonIssue(rowReason);
    if (issue) {
      toast.error(issue);
      return;
    }

    const response = await approveKyc({ subject, reason: rowReason });
    if (!response) return;

    toast.success(`KYC approved for ${subject}`);
    setApprovalReasonBySubject((current) => ({ ...current, [subject]: "" }));
  };

  return (
    <div className="space-y-4">
      {/* <Card>
        <CardHeader>
          <CardTitle>Compliance Registry</CardTitle>
          <CardDescription>Authorize institutions and manage safelist status.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input placeholder="Search participant or status" value={search} onChange={(event) => setSearch(event.target.value)} />
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Participant</TableHead>
                <TableHead>Legal Entity ID</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Credential</TableHead>
                <TableHead>Expiry</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((participant) => (
                <TableRow key={participant.id}>
                  <TableCell className="font-medium">{participant.name}</TableCell>
                  <TableCell>{participant.legalEntityId}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[participant.status]}>{participant.status}</Badge>
                  </TableCell>
                  <TableCell>{participant.credentialId ?? "—"}</TableCell>
                  <TableCell>{participant.credentialExpiry ? new Date(participant.credentialExpiry).toLocaleDateString() : "—"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card> */}

      {/* <Card>
        <CardHeader>
          <CardTitle>Issue Credential</CardTitle>
          <CardDescription>Authorize a new or pending institution.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label>Entity Name</Label>
            <Input value={entityName} onChange={(event) => setEntityName(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>Legal Entity ID</Label>
            <Input value={legalEntityId} onChange={(event) => setLegalEntityId(event.target.value)} />
          </div>
          <div className="space-y-2 md:col-span-2">
            <Label>Scopes (comma separated)</Label>
            <Input value={scopes} onChange={(event) => setScopes(event.target.value)} />
          </div>
          <div className="space-y-2 md:col-span-2">
            <Label>Reason (required)</Label>
            <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
          </div>

          <div className="md:col-span-2">
            {!confirming ? (
              <Button onClick={() => setConfirming(true)}>Continue</Button>
            ) : (
              <div className="space-y-2 rounded-md border border-border p-3">
                <p className="text-sm font-medium">Confirm credential issuance</p>
                <p className="text-xs text-muted-foreground">This action will authorize the institution and create an immutable audit entry.</p>
                <div className="flex gap-2">
                  <Button onClick={() => void onIssue()} disabled={status === "loading"}>
                    {status === "loading" ? "Submitting..." : "Confirm"}
                  </Button>
                  <Button variant="outline" onClick={() => setConfirming(false)}>
                    Cancel
                  </Button>
                </div>
              </div>
            )}
          </div>

          {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card> */}

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle>Pending KYC Approvals</CardTitle>
              <CardDescription>Approve commercial banks that are waiting for Central Bank validation.</CardDescription>
            </div>
            <Button variant="outline" onClick={() => void fetchPendingKyc()} disabled={kycStatus === "loading"}>
              {kycStatus === "loading" ? "Refreshing..." : "Refresh"}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Subject</TableHead>
                <TableHead>Bank Code</TableHead>
                <TableHead>Institution</TableHead>
                <TableHead>Wallet</TableHead>
                <TableHead>Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pendingKyc.flatMap((entry) => [
                <TableRow key={entry.subject}>
                  <TableCell className="max-w-56 truncate font-medium" title={entry.subject}>
                    {entry.subject}
                  </TableCell>
                  <TableCell>{entry.bank_code ?? "—"}</TableCell>
                  <TableCell>{entry.institution_name ?? "—"}</TableCell>
                  <TableCell className="max-w-48 truncate" title={entry.wallet_address}>
                    {entry.wallet_address ?? "—"}
                  </TableCell>
                  <TableCell>
                    {canApprove ? (
                      <Button
                        size="sm"
                        onClick={() => void onApproveKyc(entry.subject)}
                        // Two conditions, two different messages. hasAdmissionAccess decides whether
                        // the control is offered at all (spec 042); the justification rule decides
                        // whether an offered control is usable yet, so the requirement is visible
                        // BEFORE the click rather than discovered by being refused.
                        disabled={kycStatus === "loading" || !isApprovalReasonAcceptable(approvalReasonBySubject[entry.subject] ?? "")}
                      >
                        Approve KYC
                      </Button>
                    ) : (
                      <span className="text-xs text-muted-foreground">Requires the Admission profile</span>
                    )}
                  </TableCell>
                </TableRow>,
                // The justification gets a row of its own, full width: it is the part of this
                // decision a human writes and the audit trail keeps, and it did not fit in a
                // one-line box inside a table cell.
                //
                // Behind the same gate as the button, for the same reason the button is gated: a
                // required field offered to an operator who cannot approve is a dead control with
                // extra steps. A governance-only operator keeps the read view and sees why.
                ...(canApprove
                  ? [
                <TableRow key={`${entry.subject}-reason`}>
                  <TableCell colSpan={5} className="pt-0">
                    <div className="space-y-1.5 rounded-md border border-border bg-muted/30 p-3">
                      <Label htmlFor={`approval-reason-${entry.subject}`}>
                        Approval reason <span className="text-destructive">*</span>
                      </Label>
                      <Textarea
                        id={`approval-reason-${entry.subject}`}
                        rows={3}
                        value={approvalReasonBySubject[entry.subject] ?? ""}
                        onChange={(event) =>
                          setApprovalReasonBySubject((current) => ({
                            ...current,
                            [entry.subject]: event.target.value,
                          }))
                        }
                        placeholder="Why this institution is being approved — what was checked, and against what."
                      />
                      <p className="text-xs text-muted-foreground">
                        {approvalReasonIssue(approvalReasonBySubject[entry.subject] ?? "") ??
                          `${approvalReasonLength(approvalReasonBySubject[entry.subject] ?? "")} characters — recorded in the audit trail.`}
                      </p>
                    </div>
                  </TableCell>
                </TableRow>,
                    ]
                  : []),
              ])}
              {!pendingKyc.length ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-sm text-muted-foreground">
                    No pending KYC requests.
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>

          {error ? <p className="mt-3 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
