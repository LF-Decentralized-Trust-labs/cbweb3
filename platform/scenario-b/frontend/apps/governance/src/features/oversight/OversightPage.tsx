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
} from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useMemo, useState } from "react";
import { DISCLOSURE_STATE } from "../../types/oversight.types";
import { useOversightStore } from "./oversight.store";

type BadgeVariant = "default" | "secondary" | "outline" | "destructive" | "success" | "warning";

function disclosureBadgeVariant(state: string): BadgeVariant {
  if (state === DISCLOSURE_STATE.QUORUM_REACHED) return "success";
  if (state === DISCLOSURE_STATE.EXPIRED) return "outline";
  if (state === DISCLOSURE_STATE.REJECTED) return "destructive";
  return "default";
}

export function OversightPage() {
  const disclosures = useOversightStore((state) => state.disclosures);
  const currentDisclosure = useOversightStore((state) => state.currentDisclosure);
  const status = useOversightStore((state) => state.status);
  const error = useOversightStore((state) => state.error);
  const openDisclosure = useOversightStore((state) => state.openDisclosure);
  const signDisclosure = useOversightStore((state) => state.signDisclosure);
  const fetchDisclosureStatus = useOversightStore((state) => state.fetchDisclosureStatus);

  const [txRef, setTxRef] = useState("");
  const [requestorId, setRequestorId] = useState("central-bank-a");
  const [reasonCode, setReasonCode] = useState("AML_INVESTIGATION");

  const [requestIdToSign, setRequestIdToSign] = useState("");
  const [signerId, setSignerId] = useState("central-bank-b");

  const [requestIdToFetch, setRequestIdToFetch] = useState("");

  const latestDisclosure = useMemo(() => currentDisclosure ?? disclosures[0] ?? null, [currentDisclosure, disclosures]);

  const handleOpenDisclosure = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await openDisclosure({
      tx_ref: txRef,
      requestor_id: requestorId,
      reason_code: reasonCode,
    });
  };

  const handleSignDisclosure = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await signDisclosure({
      request_id: requestIdToSign,
      signer_id: signerId,
    });
  };

  const handleFetchStatus = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await fetchDisclosureStatus(requestIdToFetch);
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Open Disclosure Request</CardTitle>
            <CardDescription>Create a new AML/CFT disclosure request.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleOpenDisclosure}>
              <div className="space-y-1">
                <Label htmlFor="tx_ref">Transaction Reference</Label>
                <Input id="tx_ref" value={txRef} onChange={(event) => setTxRef(event.target.value)} required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="requestor_id">Requestor ID</Label>
                <Input id="requestor_id" value={requestorId} onChange={(event) => setRequestorId(event.target.value)} required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="reason_code">Reason Code</Label>
                <Input id="reason_code" value={reasonCode} onChange={(event) => setReasonCode(event.target.value)} required />
              </div>
              <Button type="submit" disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Open Request"}
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Sign Disclosure</CardTitle>
            <CardDescription>Submit a co-signature for an active disclosure request.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleSignDisclosure}>
              <div className="space-y-1">
                <Label htmlFor="request_id_sign">Request ID</Label>
                <Input id="request_id_sign" value={requestIdToSign} onChange={(event) => setRequestIdToSign(event.target.value)} required />
              </div>
              <div className="space-y-1">
                <Label htmlFor="signer_id">Signer ID</Label>
                <Input id="signer_id" value={signerId} onChange={(event) => setSignerId(event.target.value)} required />
              </div>
              <Button type="submit" variant="outline" disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Submit Signature"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Disclosure Status</CardTitle>
          <CardDescription>Lookup request state and quorum progress.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <form className="flex flex-col gap-2 md:flex-row" onSubmit={handleFetchStatus}>
            <Input value={requestIdToFetch} onChange={(event) => setRequestIdToFetch(event.target.value)} placeholder="Request ID" required />
            <Button type="submit" variant="secondary" disabled={status === "loading"}>
              {status === "loading" ? "Loading..." : "Fetch Status"}
            </Button>
          </form>

          {latestDisclosure ? (
            <div className="space-y-2 rounded-lg border border-border p-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Request</span>
                <Badge variant={disclosureBadgeVariant(latestDisclosure.state)}>{latestDisclosure.state}</Badge>
              </div>
              <p className="text-sm">Request ID: {latestDisclosure.request_id}</p>
              <p className="text-sm">Quorum: {latestDisclosure.quorum_reached}/{latestDisclosure.quorum_required}</p>
              <p className="text-sm">Expires At: {new Date(latestDisclosure.expires_at).toLocaleString()}</p>
              <p className="text-sm">Closed At: {latestDisclosure.closed_at ? new Date(latestDisclosure.closed_at).toLocaleString() : "-"}</p>
            </div>
          ) : null}

          {latestDisclosure ? (
            <pre className="max-h-72 overflow-auto rounded-lg border border-border bg-muted/40 p-3 text-xs">
              {JSON.stringify(latestDisclosure, null, 2)}
            </pre>
          ) : null}

          {error ? <p className="text-sm text-destructive">{error === "Already signed by this signer or request is closed" ? "Already signed or invalid request" : error}</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
