import { Button, Card, CardContent, CardHeader, CardTitle, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, toast } from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useKyc } from "../hooks";
import type { IssueCredentialPayload } from "../types";

export function KycPage() {
  const { credentials, fetch, issueCredential, status, error } = useKyc();
  const [institutionId, setInstitutionId] = useState("bank-bra-01");
  const [institutionName, setInstitutionName] = useState("Banco Regional A");
  const [credentialType, setCredentialType] = useState<IssueCredentialPayload["credentialType"]>("AUTHORIZED_ISSUER");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const onSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await issueCredential({ institutionId, institutionName, credentialType });
    toast.success("Credential issued");
  };

  return (
    <div className="grid gap-4 xl:grid-cols-[1fr_2fr]">
      <Card>
        <CardHeader>
          <CardTitle>Issue Credential</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="space-y-3" onSubmit={onSubmit}>
            <div className="space-y-2">
              <Label htmlFor="institutionId">Institution ID</Label>
              <Input id="institutionId" value={institutionId} onChange={(event) => setInstitutionId(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="institutionName">Institution Name</Label>
              <Input id="institutionName" value={institutionName} onChange={(event) => setInstitutionName(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Credential Type</Label>
              <Select value={credentialType} onValueChange={(value) => setCredentialType(value as IssueCredentialPayload["credentialType"])}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="AUTHORIZED_ISSUER">AUTHORIZED_ISSUER</SelectItem>
                  <SelectItem value="KYC_VERIFIED">KYC_VERIFIED</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {error ? <p className="text-sm text-destructive">{error}</p> : null}
            <Button type="submit" disabled={status === "loading"}>Issue</Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Issued Credentials</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Institution</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Issued At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {credentials.map((credential) => (
                <TableRow key={credential.id}>
                  <TableCell>{credential.id}</TableCell>
                  <TableCell>{credential.institutionName}</TableCell>
                  <TableCell>{credential.credentialType}</TableCell>
                  <TableCell>{new Date(credential.issuedAt).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
