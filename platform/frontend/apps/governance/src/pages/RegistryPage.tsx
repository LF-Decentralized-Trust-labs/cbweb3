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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useRegistry } from "../hooks";

const statusVariant = {
  ACTIVE: "default",
  PENDING: "warning",
  REVOKED: "destructive",
  FROZEN: "secondary",
} as const;

export function RegistryPage() {
  const { participants, fetch, issueCredential, status, error } = useRegistry();
  const [search, setSearch] = useState("");
  const [entityName, setEntityName] = useState("");
  const [cnpj, setCnpj] = useState("");
  const [scopes, setScopes] = useState("DEPOSIT,TRANSFER,SWAP");
  const [reason, setReason] = useState("");
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const filtered = useMemo(() => {
    const term = search.toLowerCase().trim();
    if (!term) return participants;
    return participants.filter(
      (participant) => participant.name.toLowerCase().includes(term) || participant.status.toLowerCase().includes(term),
    );
  }, [participants, search]);

  const onIssue = async () => {
    if (!entityName || !cnpj || !reason || reason.length < 10) {
      toast.error("Complete all fields and provide a reason (min 10 chars)");
      return;
    }
    await issueCredential({
      entityName,
      cnpj,
      scopes: scopes.split(",").map((scope) => scope.trim()).filter(Boolean),
      reason,
    });
    toast.success("Credential issued successfully");
    setEntityName("");
    setCnpj("");
    setReason("");
    setConfirming(false);
  };

  return (
    <div className="space-y-4">
      <Card>
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
                <TableHead>CNPJ</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Credential</TableHead>
                <TableHead>Expiry</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((participant) => (
                <TableRow key={participant.id}>
                  <TableCell className="font-medium">{participant.name}</TableCell>
                  <TableCell>{participant.cnpj}</TableCell>
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
      </Card>

      <Card>
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
            <Label>CNPJ</Label>
            <Input value={cnpj} onChange={(event) => setCnpj(event.target.value)} />
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
      </Card>
    </div>
  );
}
