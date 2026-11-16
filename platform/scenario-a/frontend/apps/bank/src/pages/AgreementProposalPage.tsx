// SPDX-License-Identifier: Apache-2.0

import {
  amountRefusalMessage,
  apiErrorMessage,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  parseCurrencyAmount,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  centralBankOfSpoke,
  counterpartyOptions,
  ownIdentity,
  spokeMembers,
  spokeOfIdentity,
} from "../features/fx/proposal-parties";
import { spokeIdsFromRoster } from "../features/fx/spoke-roster";
import { displayToBase } from "../types";
import { usePaymentStore } from "../stores";
import { useAuthStore } from "../stores/auth.store";
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
  // The FX leg carries the same unit as the HTLC leg that settles it (ADR-009).
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const status = useFxAgreementStore((s) => s.status);
  const identities = useIdentityStore((s) => s.identities);
  const fetchIdentities = useIdentityStore((s) => s.fetchAll);
  const identityStatus = useIdentityStore((s) => s.status);
  const identityError = useIdentityStore((s) => s.error);
  const rosterConfigured = useIdentityStore((s) => s.configured);
  const profile = useAuthStore((s) => s.profile);

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

  // Spoke ids from the roster's central-bank entries. Derived there and not from
  // the bank entries because a bank node carries no separator that tells the
  // spoke id from the bank id — see features/fx/spoke-roster.ts.
  const spokes = useMemo(() => spokeIdsFromRoster(identities), [identities]);

  // Only three parties are asked for; the rest are derived from who is proposing and to whom
  // (features/fx/proposal-parties.ts). A derived value can still be overridden, and an override
  // is kept as its own state so the default keeps following its source until then.
  const [counterpartyB, setCounterpartyBValue] = useState("");
  const [sourceReceiverChoice, setSourceReceiver] = useState("");
  const [destReceiverOverride, setDestReceiver] = useState<string | null>(null);
  const [settlementAgentOverride, setSettlementAgent] = useState<string | null>(null);
  const [custodianOverride, setCustodian] = useState<string | null>(null);
  const [beneficiaryOverride, setBeneficiary] = useState<string | null>(null);
  const [sourceSpokeFallback, setSourceSpokeFallback] = useState("");
  const [showAdvanced, setShowAdvanced] = useState(false);

  // The proposer's own identity and spoke. When the roster does not let them be told apart
  // (no bank id in the profile, or the bank is not in the roster) the spoke is asked for.
  const me = useMemo(() => ownIdentity(identities, profile?.bankId ?? ""), [identities, profile]);
  const ownSpoke = useMemo(() => (me ? spokeOfIdentity(me, spokes) : null), [me, spokes]);
  const sourceSpokeId = ownSpoke ?? sourceSpokeFallback;
  const destSpokeId = useMemo(
    () => (counterpartyB ? spokeOfIdentity(counterpartyB, spokes) ?? "" : ""),
    [counterpartyB, spokes],
  );

  const counterparties = useMemo(
    () => counterpartyOptions(identities, spokes, sourceSpokeId || null),
    [identities, spokes, sourceSpokeId],
  );
  const sourceReceivers = useMemo(
    () => spokeMembers(identities, spokes, sourceSpokeId || null, me),
    [identities, spokes, sourceSpokeId, me],
  );
  const destReceivers = useMemo(
    () => spokeMembers(identities, spokes, destSpokeId || null),
    [identities, spokes, destSpokeId],
  );

  // A single candidate is selected for the operator rather than left as a one-option dropdown.
  const sourceReceiver = sourceReceiverChoice || (sourceReceivers.length === 1 ? sourceReceivers[0] : "");
  const destReceiver = destReceiverOverride ?? counterpartyB;
  const settlementAgent = settlementAgentOverride ?? centralBankOfSpoke(sourceSpokeId, identities) ?? "";
  const custodian = custodianOverride ?? counterpartyB;
  const beneficiary = beneficiaryOverride ?? destReceiver;

  // The destination receiver and custodian belong to the counterparty's spoke, so an override
  // chosen for one counterparty must not carry over to another.
  const setCounterpartyB = (value: string) => {
    setCounterpartyBValue(value);
    setDestReceiver(null);
    setCustodian(null);
  };
  const [originAmount, setOriginAmount] = useState("");
  const originCurrency = SPOKE_CURRENCY;
  const [counterAmount, setCounterAmount] = useState("");
  const [counterCurrency, setCounterCurrency] = useState("COP");
  const [expiryDateTime, setExpiryDateTime] = useState(defaultExpiry);
  const [showConfirm, setShowConfirm] = useState(false);

  // Parsed once and reused, so the rate on screen, the validation message and
  // the value posted can never disagree about what the operator typed.
  // Each leg is bounded by ITS OWN declared currency, which is where the table earns
  // its keep: the counter leg can be CLP or PYG, and neither has a subunit.
  const parsedOrigin = useMemo(
    () => parseCurrencyAmount(originAmount, originCurrency),
    [originAmount, originCurrency],
  );
  const parsedCounter = useMemo(
    () => parseCurrencyAmount(counterAmount, counterCurrency),
    [counterAmount, counterCurrency],
  );

  const rate = useMemo(() => {
    if (!parsedOrigin.ok || !parsedCounter.ok) return "";
    return (parsedCounter.value / parsedOrigin.value).toFixed(6);
  }, [parsedOrigin, parsedCounter]);

  // What the confirmation shows: the normalised figure that will actually be
  // sent, not the keystrokes. An operator who typed "1000,10" is asked to
  // approve "1000.10", which is the number the orchestrator will receive.
  const originDisplay = parsedOrigin.ok ? parsedOrigin.canonical : originAmount;
  const counterDisplay = parsedCounter.ok ? parsedCounter.canonical : counterAmount;

  const validate = () => {
    if (!counterpartyB.trim()) {
      toast.error("Counterparty bank is required.");
      return false;
    }
    // The backend refuses a proposal without both spokes and both receivers. They used to be
    // optional here and the refusal only arrived after the confirmation step.
    if (!sourceSpokeId) {
      toast.error("Your spoke could not be determined. Select it under Parties.");
      return false;
    }
    if (!destSpokeId) {
      toast.error("The counterparty's spoke could not be determined. Choose another counterparty.");
      return false;
    }
    if (!sourceReceiver.trim()) {
      toast.error("Receiver on your spoke is required.");
      return false;
    }
    if (!destReceiver.trim()) {
      toast.error("Receiver on the counterparty's spoke is required.");
      return false;
    }
    if (!settlementAgent.trim() || !custodian.trim() || !beneficiary.trim()) {
      setShowAdvanced(true);
      toast.error("Settlement agent, custodian and beneficiary are required. Review them under Advanced.");
      return false;
    }
    if (!parsedOrigin.ok) {
      toast.error(amountRefusalMessage("Send amount", parsedOrigin.refusal));
      return false;
    }
    if (!parsedCounter.ok) {
      toast.error(amountRefusalMessage("Receive amount", parsedCounter.refusal));
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
    // Unreachable in the UI: the confirmation card only renders after validate()
    // accepted both amounts. Kept because it is what narrows the union, and
    // because an amount edited while the card is open must not slip past.
    if (!parsedOrigin.ok || !parsedCounter.ok) {
      setShowConfirm(false);
      return;
    }
    try {
      const result = await propose({
        counterparty_b: counterpartyB.trim(),
        settlement_agent: settlementAgent.trim(),
        custodian: custodian.trim(),
        beneficiary: beneficiary.trim(),
        // Scaled with the SAME decimals the HTLC leg uses. The two must stay equal:
        // sample-tryout locks an HTLC of the origin amount against this very field.
        origin_amount: displayToBase(parsedOrigin.canonical, tCeBMDecimals),
        counter_amount: displayToBase(parsedCounter.canonical, tCeBMDecimals),
        origin_currency: originCurrency,
        counter_currency: counterCurrency,
        rate: rate,
        expiry_date: Math.floor(new Date(expiryDateTime).getTime() / 1000),
        source_spoke_id: sourceSpokeId,
        dest_spoke_id: destSpokeId,
        source_receiver: sourceReceiver.trim(),
        dest_receiver: destReceiver.trim(),
      });
      toast.success("Trade agreement proposed successfully.");
      navigate(`/agreements/${result.trade_id}`);
    } catch (error) {
      toast.error(apiErrorMessage(error, "Unable to propose trade agreement."));
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
          <CardDescription>
            Choose the counterparty and who receives each leg. The spokes and intermediaries are filled in for you.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="source-spoke-id">Your Spoke</Label>
              {ownSpoke ? (
                <Input id="source-spoke-id" value={ownSpoke} readOnly disabled />
              ) : (
                <>
                  <OptionSelect id="source-spoke-id" value={sourceSpokeFallback} onChange={setSourceSpokeFallback} options={spokes} placeholder="Select your spoke" />
                  <p className="text-xs text-muted-foreground">Your bank was not found in the identity roster, so your spoke cannot be filled in.</p>
                </>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="dest-spoke-id">Counterparty&apos;s Spoke</Label>
              <Input id="dest-spoke-id" value={destSpokeId || "Select a counterparty"} readOnly disabled />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="counterparty-b">Counterparty Bank</Label>
            <OptionSelect id="counterparty-b" value={counterpartyB} onChange={setCounterpartyB} options={counterparties} placeholder="Select the bank you are trading with" />
            <p className="text-xs text-muted-foreground">Banks on other spokes.</p>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="source-receiver">Receiver on Your Spoke</Label>
              <OptionSelect id="source-receiver" value={sourceReceiver} onChange={setSourceReceiver} options={sourceReceivers} placeholder="Who receives your payment" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="dest-receiver">Receiver on Counterparty&apos;s Spoke</Label>
              <OptionSelect id="dest-receiver" value={destReceiver} onChange={setDestReceiver} options={destReceivers} placeholder="Select a counterparty first" />
              <p className="text-xs text-muted-foreground">Defaults to the counterparty bank.</p>
            </div>
          </div>

          <div className="space-y-3 rounded-md border p-3">
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-sm font-medium">Advanced</p>
                <p className="text-xs text-muted-foreground">
                  Settlement agent: {settlementAgent || "—"} · Custodian: {custodian || "—"} · Beneficiary: {beneficiary || "—"}
                </p>
              </div>
              <Button type="button" variant="outline" size="sm" onClick={() => setShowAdvanced((v) => !v)}>
                {showAdvanced ? "Hide" : "Change"}
              </Button>
            </div>
            {showAdvanced ? (
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="settlement-agent">Settlement Agent</Label>
                  <OptionSelect id="settlement-agent" value={settlementAgent} onChange={setSettlementAgent} options={identities} placeholder="Select a Paladin identity" />
                  <p className="text-xs text-muted-foreground">Defaults to your spoke&apos;s central bank.</p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="custodian">Custodian</Label>
                  <OptionSelect id="custodian" value={custodian} onChange={setCustodian} options={destReceivers} placeholder="Select a counterparty first" />
                  <p className="text-xs text-muted-foreground">Defaults to the counterparty bank. Must be on the counterparty&apos;s spoke.</p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="beneficiary">Beneficiary</Label>
                  <OptionSelect id="beneficiary" value={beneficiary} onChange={setBeneficiary} options={identities} placeholder="Select a Paladin identity" />
                  <p className="text-xs text-muted-foreground">Defaults to the receiver on the counterparty&apos;s spoke.</p>
                </div>
              </div>
            ) : null}
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
                type="text"
                inputMode="decimal"
                autoComplete="off"
                placeholder="1000.00"
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
                type="text"
                inputMode="decimal"
                autoComplete="off"
                placeholder="5000.00"
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
            <p className="text-sm">Spokes: {sourceSpokeId} → {destSpokeId}</p>
            <p className="text-sm">Receiver on your spoke: {sourceReceiver}</p>
            <p className="text-sm">Receiver on counterparty&apos;s spoke: {destReceiver}</p>
            <p className="text-sm">Settlement Agent: {settlementAgent}</p>
            <p className="text-sm">Custodian: {custodian}</p>
            <p className="text-sm">Beneficiary: {beneficiary}</p>
            <p className="text-sm">
              Send: {originDisplay} {originCurrency}
            </p>
            <p className="text-sm">
              Receive: {counterDisplay} {counterCurrency}
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
