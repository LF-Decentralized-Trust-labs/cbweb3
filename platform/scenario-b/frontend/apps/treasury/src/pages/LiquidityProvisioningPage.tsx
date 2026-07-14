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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  toast,
} from "@cbweb3/ui";
import { BadgeCheck, ChevronDown, ChevronRight, Coins, Droplets, FilePlus2, Landmark, RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { hubLiquidityApi } from "../services/api";
import { useAuthStore } from "../stores";
import type { HubCurrency, HubPair, PoolSide } from "../types";
import { displayToBase, parseAmountInput } from "../types/payment.types";

// Hub W-tCeBM tokens use 18 decimals. Operators enter whole-token amounts
// (e.g. "1000"); the backend expects raw base units, so convert on the way out.
// Without this, "1000" reaches the chain as 1000 wei (~0 tokens) and swaps starve.
const HUB_TOKEN_DECIMALS = 18;

const SELECT_CLASS =
  "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

const shortAddress = (value: string) => (value ? `${value.slice(0, 8)}...${value.slice(-6)}` : "-");

const sameAddress = (a: string, b: string) => Boolean(a) && Boolean(b) && a.toLowerCase() === b.toLowerCase();

export function LiquidityProvisioningPage() {
  const user = useAuthStore((state) => state.user);
  const defaultProviderBankId = user?.institutionId || user?.walletAddress || "";

  const [pairs, setPairs] = useState<HubPair[]>([]);
  const [pairsLoading, setPairsLoading] = useState(false);
  const [currencies, setCurrencies] = useState<HubCurrency[]>([]);
  const [currenciesLoading, setCurrenciesLoading] = useState(false);
  // Fallback Hub AMM address when no pair exists yet (from GET /amm/hub-config).

  // --- Register Currency (advanced / optional — currencies are normally pre-registered) ---
  const [registerOpen, setRegisterOpen] = useState(false);
  const [currencySymbol, setCurrencySymbol] = useState("");
  const [currencyCountry, setCurrencyCountry] = useState("");
  const [currencyTokenAddress, setCurrencyTokenAddress] = useState("");
  const [currencyProposerCb, setCurrencyProposerCb] = useState("");
  const [registering, setRegistering] = useState(false);

  // --- Propose Pair — selection-driven, fields auto-derived from the chosen currencies ---
  const [proposeSymbolA, setProposeSymbolA] = useState("");
  const [proposeSymbolB, setProposeSymbolB] = useState("");
  const [proposePairId, setProposePairId] = useState("");
  const [proposeAmmAddress, setProposeAmmAddress] = useState("");
  const [proposeTokenA, setProposeTokenA] = useState("");
  const [proposeTokenB, setProposeTokenB] = useState("");
  const [proposeProposerCb, setProposeProposerCb] = useState("");
  const [proposeAdvanced, setProposeAdvanced] = useState(false);
  const [proposing, setProposing] = useState(false);

  // --- Confirm Pair — pick a proposed pair; confirmer_cb auto-derived from token B currency ---
  const [confirmPairId, setConfirmPairId] = useState("");
  const [confirmConfirmerCb, setConfirmConfirmerCb] = useState("");
  const [confirmAdvanced, setConfirmAdvanced] = useState(false);
  const [confirming, setConfirming] = useState(false);

  // --- Mint & Approve form state ---
  const [mintAmount, setMintAmount] = useState("");
  const [mintPoolPair, setMintPoolPair] = useState("");
  const [mintSide, setMintSide] = useState<PoolSide>("A");
  const [mintRecipient, setMintRecipient] = useState("");
  const [minting, setMinting] = useState(false);

  // --- Seed Liquidity form state ---
  const [seedPoolPair, setSeedPoolPair] = useState("");
  const [seedProviderBankId, setSeedProviderBankId] = useState(defaultProviderBankId);
  const [tokenAAmount, setTokenAAmount] = useState("");
  const [tokenBAmount, setTokenBAmount] = useState("");
  const [seeding, setSeeding] = useState(false);

  const loadPairs = async () => {
    setPairsLoading(true);
    try {
      const response = await hubLiquidityApi.listPairs();
      setPairs(response.pairs ?? []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to load pairs");
    } finally {
      setPairsLoading(false);
    }
  };

  const loadCurrencies = async () => {
    setCurrenciesLoading(true);
    try {
      const response = await hubLiquidityApi.listCurrencies();
      setCurrencies(response.currencies ?? []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to load currencies");
    } finally {
      setCurrenciesLoading(false);
    }
  };

  // Fetch the currencies list once; it is reused across Propose / Confirm.
  useEffect(() => {
    void loadPairs();
    void loadCurrencies();
  }, []);

  // Keep the provider bank id in sync with the logged-in operator once the session loads.
  useEffect(() => {
    if (defaultProviderBankId) {
      setSeedProviderBankId((current) => current || defaultProviderBankId);
    }
  }, [defaultProviderBankId]);

  const currencyBySymbol = (symbol: string) => currencies.find((currency) => currency.symbol === symbol);
  const currencyByAddress = (address: string) =>
    currencies.find((currency) => sameAddress(currency.token_address, address));

  const pairOptions = useMemo(() => pairs.map((pair) => pair.pair_id), [pairs]);
  // Confirm Pair only lists pairs still awaiting confirmation (proposed/pending).
  const confirmablePairs = useMemo(
    () => pairs.filter((pair) => ["PROPOSED", "PENDING"].includes((pair.status ?? "").toUpperCase())),
    [pairs],
  );

  const deriveProposeFields = (symbolA: string, symbolB: string) => {
    const currA = currencyBySymbol(symbolA);
    const currB = currencyBySymbol(symbolB);
    setProposeTokenA(currA?.token_address ?? "");
    setProposeTokenB(currB?.token_address ?? "");
    // proposer_cb is derived from Currency A (the proposing side).
    setProposeProposerCb(currA?.proposer_cb ?? "");
    setProposePairId(symbolA && symbolB ? `${symbolA}-${symbolB}` : "");
  };

  const handleSelectSymbolA = (symbol: string) => {
    setProposeSymbolA(symbol);
    deriveProposeFields(symbol, proposeSymbolB);
  };

  const handleSelectSymbolB = (symbol: string) => {
    setProposeSymbolB(symbol);
    deriveProposeFields(proposeSymbolA, symbol);
  };

  const handleSelectConfirmPair = (pairId: string) => {
    setConfirmPairId(pairId);
    const pair = pairs.find((item) => item.pair_id === pairId);
    // confirmer_cb = the CB that registered token B (the confirming/counterparty side).
    const matching = pair ? currencyByAddress(pair.token_b_address) : undefined;
    setConfirmConfirmerCb(matching?.proposer_cb ?? "");
  };

  const onRegisterCurrency = async () => {
    if (!currencySymbol.trim() || !currencyCountry.trim() || !currencyTokenAddress.trim()) {
      toast.error("Symbol, country and token address are required");
      return;
    }
    if (!currencyProposerCb.trim()) {
      toast.error("Proposer CB is required");
      return;
    }
    setRegistering(true);
    try {
      const result = await hubLiquidityApi.registerCurrency({
        symbol: currencySymbol.trim(),
        country_name: currencyCountry.trim(),
        token_address: currencyTokenAddress.trim(),
        proposer_cb: currencyProposerCb.trim(),
      });
      toast.success(`Currency ${result.symbol} registered — tx ${result.tx_hash}`);
      setCurrencySymbol("");
      setCurrencyCountry("");
      setCurrencyTokenAddress("");
      setCurrencyProposerCb("");
      void loadCurrencies();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to register currency");
    } finally {
      setRegistering(false);
    }
  };

  const onProposePair = async () => {
    if (!proposeTokenA.trim() || !proposeTokenB.trim()) {
      toast.error("Select Currency A and Currency B");
      return;
    }
    if (!proposePairId.trim()) {
      toast.error("Pair ID is required");
      return;
    }
    if (!proposeProposerCb.trim()) {
      toast.error("Proposer CB could not be derived — set it under Advanced");
      return;
    }
    setProposing(true);
    try {
      // amm_address is left empty by default: the gateway deploys a dedicated,
      // per-pair AMM bound to the two W-tokens. Only send an address when the
      // operator explicitly overrides it under Advanced.
      const result = await hubLiquidityApi.proposePair({
        pair_id: proposePairId.trim(),
        token_a_address: proposeTokenA.trim(),
        token_b_address: proposeTokenB.trim(),
        amm_address: proposeAmmAddress.trim(),
        proposer_cb: proposeProposerCb.trim(),
      });
      const ammNote = result.amm_address ? ` — AMM ${result.amm_address}` : "";
      toast.success(`Pair ${result.pair_id} proposed (${result.status})${ammNote}`);
      void loadPairs();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to propose pair");
    } finally {
      setProposing(false);
    }
  };

  const onConfirmPair = async () => {
    if (!confirmPairId.trim()) {
      toast.error("Select a proposed pair");
      return;
    }
    if (!confirmConfirmerCb.trim()) {
      toast.error("Confirmer CB could not be derived — set it under Advanced");
      return;
    }
    setConfirming(true);
    try {
      const result = await hubLiquidityApi.confirmPair({
        pair_id: confirmPairId.trim(),
        confirmer_cb: confirmConfirmerCb.trim(),
      });
      toast.success(`Pair ${result.pair_id} confirmed (${result.status}) — tx ${result.tx_hash}`);
      void loadPairs();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to confirm pair");
    } finally {
      setConfirming(false);
    }
  };

  const onMintAndApprove = async () => {
    if (!mintAmount.trim()) {
      toast.error("Amount is required");
      return;
    }
    if (!/^\d+(\.\d+)?$/.test(parseAmountInput(mintAmount.trim()))) {
      toast.error("Amount must be a non-negative number");
      return;
    }
    if (!mintPoolPair.trim()) {
      toast.error("Pool pair is required");
      return;
    }
    setMinting(true);
    try {
      const result = await hubLiquidityApi.mintAndApprove({
        amount: displayToBase(parseAmountInput(mintAmount.trim()), HUB_TOKEN_DECIMALS),
        pool_pair: mintPoolPair.trim(),
        side: mintSide,
        recipient: mintRecipient.trim() || undefined,
      });
      toast.success(
        `Mint & approve ${result.status ?? "ok"} — ${result.amount}${
          result.recipient ? ` to ${result.recipient}` : " for the AMM"
        }`,
      );
      void loadPairs();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to mint & approve tokens");
    } finally {
      setMinting(false);
    }
  };

  const onSeedLiquidity = async () => {
    if (!seedPoolPair.trim()) {
      toast.error("Pool pair is required");
      return;
    }
    if (!seedProviderBankId.trim()) {
      toast.error("Provider bank ID is required");
      return;
    }
    if (!tokenAAmount.trim() || !tokenBAmount.trim()) {
      toast.error("Both token amounts are required");
      return;
    }
    setSeeding(true);
    try {
      const result = await hubLiquidityApi.addLiquidity({
        pool_pair: seedPoolPair.trim(),
        provider_bank_id: seedProviderBankId.trim(),
        token_a_amount: displayToBase(parseAmountInput(tokenAAmount.trim()), HUB_TOKEN_DECIMALS),
        token_b_amount: displayToBase(parseAmountInput(tokenBAmount.trim()), HUB_TOKEN_DECIMALS),
      });
      const ok = result.success ?? true;
      toast.success(`Liquidity seeded — ${result.status ?? (ok ? "ok" : "submitted")}`);
      void loadPairs();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to seed liquidity");
    } finally {
      setSeeding(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FilePlus2 className="h-5 w-5" />
            Propose Pair
          </CardTitle>
          <CardDescription>
            Pick two registered currencies to propose a new AMM pair. Token addresses and the proposer CB are derived
            automatically; signing is done by this central bank&apos;s gateway.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="propose-currency-a">Currency A</Label>
            <select
              id="propose-currency-a"
              className={SELECT_CLASS}
              value={proposeSymbolA}
              onChange={(event) => handleSelectSymbolA(event.target.value)}
            >
              <option value="">Select a currency...</option>
              {currencies.map((currency) => (
                <option key={`a-${currency.symbol}`} value={currency.symbol}>
                  {currency.symbol} — {currency.country_name}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="propose-currency-b">Currency B</Label>
            <select
              id="propose-currency-b"
              className={SELECT_CLASS}
              value={proposeSymbolB}
              onChange={(event) => handleSelectSymbolB(event.target.value)}
            >
              <option value="">Select a currency...</option>
              {currencies.map((currency) => (
                <option key={`b-${currency.symbol}`} value={currency.symbol}>
                  {currency.symbol} — {currency.country_name}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="propose-pair-id">Pair ID</Label>
            <Input
              id="propose-pair-id"
              value={proposePairId}
              onChange={(event) => setProposePairId(event.target.value)}
              placeholder="auto-derived from the two symbols"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="propose-amm-address">Dedicated AMM address (optional)</Label>
            <Input
              id="propose-amm-address"
              value={proposeAmmAddress}
              onChange={(event) => setProposeAmmAddress(event.target.value)}
              placeholder="leave blank — a dedicated per-pair AMM is deployed automatically"
            />
          </div>

          <div className="md:col-span-2">
            <button
              type="button"
              onClick={() => setProposeAdvanced((value) => !value)}
              className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            >
              {proposeAdvanced ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              Advanced / override
            </button>
          </div>
          {proposeAdvanced ? (
            <>
              <div className="space-y-2">
                <Label htmlFor="propose-token-a">Token A address</Label>
                <Input
                  id="propose-token-a"
                  value={proposeTokenA}
                  onChange={(event) => setProposeTokenA(event.target.value)}
                  placeholder="0x..."
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="propose-token-b">Token B address</Label>
                <Input
                  id="propose-token-b"
                  value={proposeTokenB}
                  onChange={(event) => setProposeTokenB(event.target.value)}
                  placeholder="0x..."
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="propose-proposer-cb">Proposer CB</Label>
                <Input
                  id="propose-proposer-cb"
                  value={proposeProposerCb}
                  onChange={(event) => setProposeProposerCb(event.target.value)}
                  placeholder="derived from Currency A"
                />
              </div>
            </>
          ) : (
            <div className="md:col-span-2 grid gap-1 rounded-md border border-dashed border-border p-3 text-xs text-muted-foreground">
              <span>
                Token A: <span className="font-mono">{proposeTokenA || "-"}</span>
              </span>
              <span>
                Token B: <span className="font-mono">{proposeTokenB || "-"}</span>
              </span>
              <span>
                Proposer CB: <span className="font-mono">{proposeProposerCb || "-"}</span>
              </span>
            </div>
          )}

          <div className="md:col-span-2">
            <Button onClick={() => void onProposePair()} disabled={proposing}>
              {proposing ? "Submitting..." : "Propose Pair"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BadgeCheck className="h-5 w-5" />
            Confirm Pair
          </CardTitle>
          <CardDescription>
            Select a proposed pair to confirm it. The confirmer CB is derived from the pair&apos;s token B currency.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="confirm-pair-id-select">Proposed Pair</Label>
            <select
              id="confirm-pair-id-select"
              className={SELECT_CLASS}
              value={confirmPairId}
              onChange={(event) => handleSelectConfirmPair(event.target.value)}
            >
              <option value="">Select a proposed pair...</option>
              {confirmablePairs.map((pair) => (
                <option key={pair.pair_id} value={pair.pair_id}>
                  {pair.pair_id} ({pair.status})
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-2">
            <Label>Confirmer CB (derived)</Label>
            <div className="flex h-9 items-center rounded-md border border-dashed border-border px-3 text-xs font-mono text-muted-foreground">
              {confirmConfirmerCb || "-"}
            </div>
          </div>

          <div className="md:col-span-2">
            <button
              type="button"
              onClick={() => setConfirmAdvanced((value) => !value)}
              className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            >
              {confirmAdvanced ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              Advanced / override
            </button>
          </div>
          {confirmAdvanced ? (
            <div className="space-y-2">
              <Label htmlFor="confirm-confirmer-cb">Confirmer CB</Label>
              <Input
                id="confirm-confirmer-cb"
                value={confirmConfirmerCb}
                onChange={(event) => setConfirmConfirmerCb(event.target.value)}
                placeholder="derived from token B currency"
              />
            </div>
          ) : null}

          <div className="md:col-span-2">
            <Button onClick={() => void onConfirmPair()} disabled={confirming}>
              {confirming ? "Submitting..." : "Confirm Pair"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Coins className="h-5 w-5" />
            Mint &amp; Approve
          </CardTitle>
          <CardDescription>
            Mint and approve tokens for the AMM for a given pool and side. Leave the recipient blank to mint &amp;
            approve for the AMM itself.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="mint-amount">Amount (whole tokens)</Label>
            <Input
              id="mint-amount"
              value={mintAmount}
              onChange={(event) => setMintAmount(event.target.value)}
              inputMode="decimal"
              placeholder="e.g. 1000"
            />
            <p className="text-xs text-muted-foreground">Whole tokens — converted to 18-decimal base units on submit.</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="mint-side">Side</Label>
            <select
              id="mint-side"
              className={SELECT_CLASS}
              value={mintSide}
              onChange={(event) => setMintSide(event.target.value as PoolSide)}
            >
              <option value="A">A</option>
              <option value="B">B</option>
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="mint-pool-pair-select">Pool Pair</Label>
            <select
              id="mint-pool-pair-select"
              className={SELECT_CLASS}
              value={pairOptions.includes(mintPoolPair) ? mintPoolPair : ""}
              onChange={(event) => setMintPoolPair(event.target.value)}
            >
              <option value="">Select a pair...</option>
              {pairOptions.map((pairId) => (
                <option key={pairId} value={pairId}>
                  {pairId}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="mint-recipient">Recipient (optional)</Label>
            <Input
              id="mint-recipient"
              value={mintRecipient}
              onChange={(event) => setMintRecipient(event.target.value)}
              placeholder="0x... (leave blank to mint for the AMM)"
            />
          </div>
          <div className="md:col-span-2">
            <Button onClick={() => void onMintAndApprove()} disabled={minting}>
              {minting ? "Submitting..." : "Mint & Approve"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Droplets className="h-5 w-5" />
            Seed Liquidity
          </CardTitle>
          <CardDescription>Add initial liquidity to the pool for both sides.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="seed-pool-pair-select">Pool Pair</Label>
            <select
              id="seed-pool-pair-select"
              className={SELECT_CLASS}
              value={pairOptions.includes(seedPoolPair) ? seedPoolPair : ""}
              onChange={(event) => setSeedPoolPair(event.target.value)}
            >
              <option value="">Select a pair...</option>
              {pairOptions.map((pairId) => (
                <option key={pairId} value={pairId}>
                  {pairId}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="seed-provider-bank-id">Provider Bank ID</Label>
            <Input
              id="seed-provider-bank-id"
              value={seedProviderBankId}
              onChange={(event) => setSeedProviderBankId(event.target.value)}
              placeholder="this central bank's identity"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="token-a-amount">Token A Amount (whole tokens)</Label>
            <Input
              id="token-a-amount"
              value={tokenAAmount}
              onChange={(event) => setTokenAAmount(event.target.value)}
              inputMode="decimal"
              placeholder="e.g. 1000"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="token-b-amount">Token B Amount (whole tokens)</Label>
            <Input
              id="token-b-amount"
              value={tokenBAmount}
              onChange={(event) => setTokenBAmount(event.target.value)}
              inputMode="decimal"
              placeholder="e.g. 1000"
            />
            <p className="text-xs text-muted-foreground">Whole tokens — converted to 18-decimal base units on submit.</p>
          </div>
          <div className="md:col-span-2">
            <Button onClick={() => void onSeedLiquidity()} disabled={seeding}>
              {seeding ? "Submitting..." : "Seed Liquidity"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle>Registered Currencies</CardTitle>
            <CardDescription>National currencies registered on the Hub currency registry.</CardDescription>
          </div>
          <Button variant="outline" size="sm" onClick={() => void loadCurrencies()} disabled={currenciesLoading}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {currenciesLoading ? "Refreshing..." : "Refresh"}
          </Button>
        </CardHeader>
        <CardContent>
          {currencies.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {currenciesLoading ? "Loading currencies..." : "No currencies registered."}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Symbol</TableHead>
                  <TableHead>Country</TableHead>
                  <TableHead>Token Address</TableHead>
                  <TableHead>Proposer CB</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {currencies.map((currency) => (
                  <TableRow key={`${currency.symbol}-${currency.token_address}`}>
                    <TableCell className="font-medium">{currency.symbol}</TableCell>
                    <TableCell>{currency.country_name}</TableCell>
                    <TableCell className="font-mono text-xs" title={currency.token_address}>
                      {shortAddress(currency.token_address)}
                    </TableCell>
                    <TableCell className="font-mono text-xs" title={currency.proposer_cb}>
                      {currency.proposer_cb}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}

          <div className="mt-4 border-t border-border pt-4">
            <button
              type="button"
              onClick={() => setRegisterOpen((value) => !value)}
              className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            >
              {registerOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              <Landmark className="h-4 w-4" />
              Register a currency (advanced)
            </button>
            {registerOpen ? (
              <div className="mt-4 grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="currency-symbol">Symbol</Label>
                  <Input
                    id="currency-symbol"
                    value={currencySymbol}
                    onChange={(event) => setCurrencySymbol(event.target.value)}
                    placeholder="e.g. W-tCeBM_BRL"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="currency-country">Country Name</Label>
                  <Input
                    id="currency-country"
                    value={currencyCountry}
                    onChange={(event) => setCurrencyCountry(event.target.value)}
                    placeholder="e.g. Sovereign BRL"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="currency-token-address">Token Address</Label>
                  <Input
                    id="currency-token-address"
                    value={currencyTokenAddress}
                    onChange={(event) => setCurrencyTokenAddress(event.target.value)}
                    placeholder="0x..."
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="currency-proposer-cb">Proposer CB</Label>
                  <Input
                    id="currency-proposer-cb"
                    value={currencyProposerCb}
                    onChange={(event) => setCurrencyProposerCb(event.target.value)}
                    placeholder="e.g. spoke-brl"
                  />
                </div>
                <div className="md:col-span-2">
                  <Button variant="outline" onClick={() => void onRegisterCurrency()} disabled={registering}>
                    {registering ? "Submitting..." : "Register Currency"}
                  </Button>
                </div>
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle>Pairs</CardTitle>
            <CardDescription>Existing AMM pairs on the hub. Select a pair above to provision it.</CardDescription>
          </div>
          <Button variant="outline" size="sm" onClick={() => void loadPairs()} disabled={pairsLoading}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {pairsLoading ? "Refreshing..." : "Refresh"}
          </Button>
        </CardHeader>
        <CardContent>
          {pairs.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {pairsLoading ? "Loading pairs..." : "No pairs found."}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Pair ID</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Token A</TableHead>
                  <TableHead>Token B</TableHead>
                  <TableHead>AMM</TableHead>
                  <TableHead>Proposer CB</TableHead>
                  <TableHead>Confirmer CB</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pairs.map((pair) => (
                  <TableRow key={pair.pair_id}>
                    <TableCell className="font-medium">{pair.pair_id}</TableCell>
                    <TableCell>
                      <Badge variant={pair.status === "ACTIVE" ? "success" : "outline"}>{pair.status}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs" title={pair.token_a_address}>
                      {shortAddress(pair.token_a_address)}
                    </TableCell>
                    <TableCell className="font-mono text-xs" title={pair.token_b_address}>
                      {shortAddress(pair.token_b_address)}
                    </TableCell>
                    <TableCell className="font-mono text-xs" title={pair.amm_address}>
                      {shortAddress(pair.amm_address)}
                    </TableCell>
                    <TableCell className="font-mono text-xs" title={pair.proposer_cb}>
                      {pair.proposer_cb}
                    </TableCell>
                    <TableCell className="font-mono text-xs" title={pair.confirmer_cb}>
                      {pair.confirmer_cb}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
