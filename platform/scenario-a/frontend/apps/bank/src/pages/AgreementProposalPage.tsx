// SPDX-License-Identifier: Apache-2.0

import {
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
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useFxAgreementStore } from "../stores/fx-agreement.store";
import { useIdentityStore } from "../stores/identity.store";

// Dropdown of string options, shared by the party and spoke fields so a typo
// or a non-member value can no longer reach the on-chain propose call. Options
// are derived from the available Paladin identities (single source of truth).
function OptionSelect({
  id,
  value,
  onChange,
  options,
  placeholder,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  options: string[];
  placeholder: string;
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger id={id}>
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {options.length ? (
          options.map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))
        ) : (
          <SelectItem value="__none__" disabled>
            No options available
          </SelectItem>
        )}
      </SelectContent>
    </Select>
  );
}

// Derive the spoke id (e.g. "spoke-brl") from a Paladin identity such as
// "funded_operator@spoke-brl-bank-itau".
function spokeFromIdentity(identity: string): string {
  const node = identity.includes("@") ? identity.slice(identity.indexOf("@") + 1) : identity;
  const parts = node.split("-");
  return parts.length >= 2 ? `${parts[0]}-${parts[1]}` : node;
}

// Latin American settlement currencies (ISO 4217) selectable on the receive leg.
const LATAM_CURRENCIES = [
  "ARS", "BOB", "BRL", "CLP", "COP", "CRC", "CUP", "DOP", "GTQ",
  "HNL", "MXN", "NIO", "PAB", "PEN", "PYG", "USD", "UYU", "VES",
];

// The send leg is fixed to this portal's own spoke currency, baked at build time
// as VITE_FIAT_SYMBOL (e.g. BRL for a Brazilian spoke).
const SPOKE_CURRENCY = (import.meta.env.VITE_FIAT_SYMBOL ?? "BRL").trim() || "BRL";

const defaultExpiry = () => {
  const d = new Date();
  d.setHours(d.getHours() + 24);
  return d.toISOString().slice(0, 16);
};

export function AgreementProposalPage() {
  const navigate = useNavigate();
  const propose = useFxAgreementStore((s) => s.propose);
  const status = useFxAgreementStore((s) => s.status);
  const identities = useIdentityStore((s) => s.identities);
  const fetchIdentities = useIdentityStore((s) => s.fetchAll);
  const identityStatus = useIdentityStore((s) => s.status);
  const identityError = useIdentityStore((s) => s.error);
  const rosterConfigured = useIdentityStore((s) => s.configured);

  useEffect(() => {
    void fetchIdentities();
  }, [fetchIdentities]);

  // Distinguish a failed/unconfigured roster from a genuinely empty one so the
  // operator is not left with a silent empty dropdown and no explanation.
  const rosterUnavailable = identityStatus === "error" || !rosterConfigured;
  const rosterMessage =
    identityStatus === "error"
      ? identityError ?? "Unable to load the identity roster. Check the API gateway and try again."
      : "The Paladin identity roster is not configured. Set PALADIN_IDENTITIES or enable Pente group membership on the API gateway, then reload.";

  // Unique spoke ids derived from the identity roster, sorted.
  const spokes = useMemo(
    () => Array.from(new Set(identities.map(spokeFromIdentity))).sort(),
    [identities],
  );

  const [counterpartyB, setCounterpartyB] = useState("");
  const [settlementAgent, setSettlementAgent] = useState("");
  const [custodian, setCustodian] = useState("");
  const [beneficiary, setBeneficiary] = useState("");
  const [sourceSpokeId, setSourceSpokeId] = useState("");
  const [destSpokeId, setDestSpokeId] = useState("");
  const [sourceReceiver, setSourceReceiver] = useState("");
  const [destReceiver, setDestReceiver] = useState("");
  const [originAmount, setOriginAmount] = useState("");
  const originCurrency = SPOKE_CURRENCY;
  const [counterAmount, setCounterAmount] = useState("");
  const [counterCurrency, setCounterCurrency] = useState("COP");
  const [expiryDateTime, setExpiryDateTime] = useState(defaultExpiry);
  const [showConfirm, setShowConfirm] = useState(false);

  const rate = useMemo(() => {
    const o = parseFloat(originAmount);
    const c = parseFloat(counterAmount);
    if (o > 0 && c > 0) return (c / o).toFixed(6);
    return "";
  }, [originAmount, counterAmount]);

  const validate = () => {
    if (!counterpartyB.trim()) {
      toast.error("Counterparty identity is required.");
      return false;
    }
    if (!settlementAgent.trim()) {
      toast.error("Settlement agent identity is required.");
      return false;
    }
    if (!custodian.trim()) {
      toast.error("Custodian identity is required.");
      return false;
    }
    if (!beneficiary.trim()) {
      toast.error("Beneficiary identity is required.");
      return false;
    }
    if (!originAmount || parseFloat(originAmount) <= 0) {
      toast.error("Send amount must be a positive number.");
      return false;
    }
    if (!counterAmount || parseFloat(counterAmount) <= 0) {
      toast.error("Receive amount must be a positive number.");
      return false;
    }
    if (!expiryDateTime) {
      toast.error("Expiry date is required.");
      return false;
    }
    const expiryUnix = Math.floor(new Date(expiryDateTime).getTime() / 1000);
    if (expiryUnix <= Math.floor(Date.now() / 1000) + 300) {
      toast.error("Expiry must be at least 5 minutes in the future.");
      return false;
    }
    return true;
  };

  const onPrepare = () => {
    if (!validate()) return;
    setShowConfirm(true);
  };

  const onConfirm = async () => {
    try {
      const result = await propose({
        counterparty_b: counterpartyB.trim(),
        settlement_agent: settlementAgent.trim(),
        custodian: custodian.trim(),
        beneficiary: beneficiary.trim(),
        origin_amount: originAmount,
        counter_amount: counterAmount,
        origin_currency: originCurrency,
        counter_currency: counterCurrency,
        rate: rate,
        expiry_date: Math.floor(new Date(expiryDateTime).getTime() / 1000),
        source_spoke_id: sourceSpokeId.trim() || undefined,
        dest_spoke_id: destSpokeId.trim() || undefined,
        source_receiver: sourceReceiver.trim() || undefined,
        dest_receiver: destReceiver.trim() || undefined,
      });
      toast.success("Trade agreement proposed successfully.");
      navigate(`/agreements/${result.trade_id}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to propose trade agreement.");
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">New Trade Agreement</h1>
          <p className="text-sm text-muted-foreground">Propose a new FX trade agreement with a counterparty.</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/agreements">Back to agreements</Link>
        </Button>
      </div>

      {rosterUnavailable ? (
        <Card className="border-destructive">
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-destructive">Identity roster unavailable</CardTitle>
              <CardDescription>{rosterMessage}</CardDescription>
            </div>
            <Button variant="outline" onClick={() => void fetchIdentities()} disabled={identityStatus === "loading"}>
              {identityStatus === "loading" ? "Loading..." : "Retry"}
            </Button>
          </CardHeader>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Parties</CardTitle>
          <CardDescription>Select the counterparty and intermediary identities.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="counterparty-b">Counterparty B Identity</Label>
            <OptionSelect id="counterparty-b" value={counterpartyB} onChange={setCounterpartyB} options={identities} placeholder="Select a Paladin identity" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="settlement-agent">Settlement Agent Identity</Label>
            <OptionSelect id="settlement-agent" value={settlementAgent} onChange={setSettlementAgent} options={identities} placeholder="Select a Paladin identity" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="custodian">Custodian Identity</Label>
            <OptionSelect id="custodian" value={custodian} onChange={setCustodian} options={identities} placeholder="Select a Paladin identity" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="beneficiary">Beneficiary Identity</Label>
            <OptionSelect id="beneficiary" value={beneficiary} onChange={setBeneficiary} options={identities} placeholder="Select a Paladin identity" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="source-spoke-id">Source Spoke ID</Label>
            <OptionSelect id="source-spoke-id" value={sourceSpokeId} onChange={setSourceSpokeId} options={spokes} placeholder="Select a spoke" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="dest-spoke-id">Destination Spoke ID</Label>
            <OptionSelect id="dest-spoke-id" value={destSpokeId} onChange={setDestSpokeId} options={spokes} placeholder="Select a spoke" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="source-receiver">Source Receiver</Label>
            <OptionSelect id="source-receiver" value={sourceReceiver} onChange={setSourceReceiver} options={identities} placeholder="Select a Paladin identity" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="dest-receiver">Destination Receiver</Label>
            <OptionSelect id="dest-receiver" value={destReceiver} onChange={setDestReceiver} options={identities} placeholder="Select a Paladin identity" />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Trade Terms</CardTitle>
          <CardDescription>Define the amounts, currencies, and exchange rate.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="origin-amount">Send Amount</Label>
              <Input
                id="origin-amount"
                type="number"
                min="0"
                step="any"
                placeholder="1000"
                value={originAmount}
                onChange={(e) => setOriginAmount(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="origin-currency">Send Currency</Label>
              <Input id="origin-currency" value={originCurrency} readOnly disabled />
              <p className="text-xs text-muted-foreground">Fixed to this spoke&apos;s currency.</p>
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="counter-amount">Receive Amount</Label>
              <Input
                id="counter-amount"
                type="number"
                min="0"
                step="any"
                placeholder="5000"
                value={counterAmount}
                onChange={(e) => setCounterAmount(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="counter-currency">Receive Currency</Label>
              <Select value={counterCurrency} onValueChange={setCounterCurrency}>
                <SelectTrigger>
                  <SelectValue placeholder="Currency" />
                </SelectTrigger>
                <SelectContent>
                  {LATAM_CURRENCIES.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <Label>Exchange Rate</Label>
            <p className="text-sm text-muted-foreground">
              {rate ? (
                <>
                  1 {originCurrency} = {rate} {counterCurrency}
                </>
              ) : (
                "Enter both amounts to calculate the exchange rate."
              )}
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="expiry-date">Expiry Date</Label>
            <Input
              id="expiry-date"
              type="datetime-local"
              value={expiryDateTime}
              onChange={(e) => setExpiryDateTime(e.target.value)}
            />
          </div>

          <Button onClick={onPrepare} disabled={status === "loading" || rosterUnavailable}>
            {status === "loading" ? "Submitting..." : "Review Agreement"}
          </Button>
          {rosterUnavailable ? (
            <p className="text-xs text-destructive">
              Resolve the identity roster above before proposing an agreement.
            </p>
          ) : null}
        </CardContent>
      </Card>

      {showConfirm ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Trade Agreement</CardTitle>
            <CardDescription>Review the agreement details before submitting.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm">Counterparty: {counterpartyB}</p>
            <p className="text-sm">Settlement Agent: {settlementAgent}</p>
            <p className="text-sm">Custodian: {custodian}</p>
            <p className="text-sm">Beneficiary: {beneficiary}</p>
            <p className="text-sm">Source Spoke: {sourceSpokeId || "—"}</p>
            <p className="text-sm">Destination Spoke: {destSpokeId || "—"}</p>
            <p className="text-sm">Source Receiver: {sourceReceiver || "—"}</p>
            <p className="text-sm">Destination Receiver: {destReceiver || "—"}</p>
            <p className="text-sm">
              Send: {originAmount} {originCurrency}
            </p>
            <p className="text-sm">
              Receive: {counterAmount} {counterCurrency}
            </p>
            <p className="text-sm">Exchange Rate: {rate}</p>
            <p className="text-sm">Expiry: {new Date(expiryDateTime).toLocaleString()}</p>
            <div className="flex gap-2">
              <Button onClick={() => void onConfirm()} disabled={status === "loading"}>
                {status === "loading" ? "Proposing..." : "Confirm Proposal"}
              </Button>
              <Button variant="outline" onClick={() => setShowConfirm(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
