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
import { useParticipantsStore } from "../stores";
import { InstitutionType } from "../types";

export function ParticipantManagementPage() {
  const { participants, lastSanctionsCheck, status, error, fetch, onboard, revokeCredential, checkSanctions } = useParticipantsStore();
  const [institutionName, setInstitutionName] = useState("New Bank Institution");
  const [evmAddress, setEvmAddress] = useState("0x8A7e4D1F2B3c4A5d6E7f8C9a0B1D2E3F4A5B6C7D");
  const [publicKey, setPublicKey] = useState("pk_new_institution");
  const [jurisdictionCode, setJurisdictionCode] = useState("BR");
  const [institutionType, setInstitutionType] = useState<InstitutionType>(InstitutionType.COMMERCIAL_BANK);
  const [revokeAddress, setRevokeAddress] = useState("");
  const [revokeReason, setRevokeReason] = useState("Compliance violation");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const activeCount = useMemo(() => participants.filter((participant) => participant.credentialStatus === "ACTIVE").length, [participants]);

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Participants</CardDescription>
            <CardTitle>{participants.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Credentials</CardDescription>
            <CardTitle>{activeCount}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Trusted List</CardTitle>
          <CardDescription>Institution onboarding, sanctions check, and credential lifecycle.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Institution</TableHead>
                <TableHead>EVM Address</TableHead>
                <TableHead>Jurisdiction</TableHead>
                <TableHead>Credential</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {participants.map((participant) => (
                <TableRow key={participant.id}>
                  <TableCell>{participant.institutionName}</TableCell>
                  <TableCell className="font-mono text-xs">{participant.evmAddress}</TableCell>
                  <TableCell>{participant.jurisdictionCode}</TableCell>
                  <TableCell>
                    <Badge variant={participant.credentialStatus === "ACTIVE" ? "success" : "destructive"}>
                      {participant.credentialStatus}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <section className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Institutional Onboarding</CardTitle>
            <CardDescription>POST /compliance/onboard</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="institution-name">Institution name</Label>
              <Input id="institution-name" value={institutionName} onChange={(event) => setInstitutionName(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="evm-address">EVM address</Label>
              <Input id="evm-address" value={evmAddress} onChange={(event) => setEvmAddress(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="public-key">Public key</Label>
              <Input id="public-key" value={publicKey} onChange={(event) => setPublicKey(event.target.value)} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="jurisdiction">Jurisdiction</Label>
                <Input id="jurisdiction" value={jurisdictionCode} onChange={(event) => setJurisdictionCode(event.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Institution type</Label>
                <Select value={institutionType} onValueChange={(value) => setInstitutionType(value as InstitutionType)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={InstitutionType.COMMERCIAL_BANK}>Commercial Bank</SelectItem>
                    <SelectItem value={InstitutionType.CENTRAL_BANK}>Central Bank</SelectItem>
                    <SelectItem value={InstitutionType.CLEARING_HOUSE}>Clearing House</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <Button
              disabled={status === "loading"}
              onClick={async () => {
                await onboard({ institutionName, evmAddress, publicKey, jurisdictionCode, institutionType });
                toast("Institution onboarded");
              }}
            >
              Onboard Institution
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Credential Revocation</CardTitle>
            <CardDescription>DELETE /auth/identity/{"{"}address{"}"}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="revoke-address">Target address</Label>
              <Input id="revoke-address" value={revokeAddress} onChange={(event) => setRevokeAddress(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="revoke-reason">Reason</Label>
              <Textarea id="revoke-reason" value={revokeReason} onChange={(event) => setRevokeReason(event.target.value)} />
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="destructive"
                disabled={status === "loading" || revokeAddress.length < 10 || revokeReason.length < 5}
                onClick={async () => {
                  await revokeCredential({ address: revokeAddress, reason: revokeReason });
                  toast("Credential revoked");
                }}
              >
                Revoke Credential
              </Button>
              <Button
                variant="outline"
                disabled={status === "loading" || revokeAddress.length < 10}
                onClick={async () => {
                  await checkSanctions(revokeAddress);
                  toast("Sanctions check completed");
                }}
              >
                Check Sanctions
              </Button>
            </div>

            {lastSanctionsCheck ? (
              <div className="rounded-md border border-border bg-muted/30 p-2 text-sm">
                <p>
                  Sanctions result for <span className="font-mono text-xs">{lastSanctionsCheck.address}</span>: {" "}
                  <Badge variant={lastSanctionsCheck.isSanctioned ? "destructive" : "success"}>
                    {lastSanctionsCheck.isSanctioned ? "MATCH" : "CLEAR"}
                  </Badge>
                </p>
              </div>
            ) : null}

            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
