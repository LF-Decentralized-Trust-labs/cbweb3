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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  toast,
} from "@cbweb3/ui";
import { BadgeCheck, Droplets, FilePlus2, RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { hubLiquidityApi } from "../services/api";
import type { EscrowStatus, HubCurrency, HubPair } from "../types";
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
  const [pairs, setPairs] = useState<HubPair[]>([]);
  const [currencies, setCurrencies] = useState<HubCurrency[]>([]);

  // --- Propose Pair — selection-driven, fields auto-derived from the chosen currencies ---
  const [proposeSymbolA, setProposeSymbolA] = useState("");
  const [proposeSymbolB, setProposeSymbolB] = useState("");
  const [proposePairId, setProposePairId] = useState("");
  const [proposeTokenA, setProposeTokenA] = useState("");
  const [proposeTokenB, setProposeTokenB] = useState("");
  const [proposeProposerCb, setProposeProposerCb] = useState("");
  const [proposing, setProposing] = useState(false);

  // --- Confirm Pair — pick a proposed pair; confirmer_cb auto-derived from token B currency ---
  const [confirmPairId, setConfirmPairId] = useState("");
  const [confirmConfirmerCb, setConfirmConfirmerCb] = useState("");
  const [confirming, setConfirming] = useState(false);

  // --- Sovereign seed state (escrow-and-finalize, no LCR) ---
  const [seedPoolPair, setSeedPoolPair] = useState("");
  const [seedAmount, setSeedAmount] = useState("");
  const [seeding, setSeeding] = useState(false);
  const [finalizing, setFinalizing] = useState(false);
  const [reclaiming, setReclaiming] = useState(false);
  const [escrow, setEscrow] = useState<EscrowStatus | null>(null);

  const loadPairs = async () => {
    try {
      const response = await hubLiquidityApi.listPairs();
      setPairs(response.pairs ?? []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to load pairs");
    }
  };

  const loadCurrencies = async () => {
    try {
      const response = await hubLiquidityApi.listCurrencies();
      setCurrencies(response.currencies ?? []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to load currencies");
    }
  };

  // Fetch the currencies list once; it is reused across Propose / Confirm.
  useEffect(() => {
    void loadPairs();
    void loadCurrencies();
  }, []);

  const loadEscrow = async (poolPair: string) => {
    if (!poolPair) {
      setEscrow(null);
      return;
    }
    try {
      setEscrow(await hubLiquidityApi.getEscrow(poolPair));
    } catch {
      setEscrow(null);
    }
  };

  // Refresh the escrow status whenever the selected pool changes.
  useEffect(() => {
    void loadEscrow(seedPoolPair);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedPoolPair]);

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
      // The gateway always deploys a dedicated, per-pair AMM bound to the two
      // W-tokens; there is no operator-supplied AMM address anymore.
      const result = await hubLiquidityApi.proposePair({
        pair_id: proposePairId.trim(),
        token_a_address: proposeTokenA.trim(),
        token_b_address: proposeTokenB.trim(),
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

  // Deposit ONLY this CB's own side into the pool's escrow (side auto-resolved on-chain).
  const onDepositSide = async () => {
    if (!seedPoolPair.trim()) {
      toast.error("Pool pair is required");
      return;
    }
    if (!/^\d+(\.\d+)?$/.test(parseAmountInput(seedAmount.trim()))) {
      toast.error("Amount must be a non-negative number");
      return;
    }
    setSeeding(true);
    try {
      const result = await hubLiquidityApi.depositSide({
        pool_pair: seedPoolPair.trim(),
        amount: displayToBase(parseAmountInput(seedAmount.trim()), HUB_TOKEN_DECIMALS),
      });
      toast.success(`Deposited side ${result.side} for ${result.pool_pair}`);
      await loadEscrow(seedPoolPair);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to deposit side");
    } finally {
      setSeeding(false);
    }
  };

  // Finalize once BOTH sides are escrowed — funds the pool reserves atomically.
  const onFinalize = async () => {
    if (!seedPoolPair.trim()) {
      return;
    }
    setFinalizing(true);
    try {
      const result = await hubLiquidityApi.finalizeSeed(seedPoolPair.trim());
      toast.success(`Pool finalized — shares A ${result.shares_a} / B ${result.shares_b}`);
      await loadEscrow(seedPoolPair);
      void loadPairs();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Finalize failed (both sides must be deposited)");
    } finally {
      setFinalizing(false);
    }
  };

  // Reclaim this CB's own pending side (before finalize).
  const onReclaim = async () => {
    if (!seedPoolPair.trim()) {
      return;
    }
    setReclaiming(true);
    try {
      const result = await hubLiquidityApi.reclaimSide(seedPoolPair.trim());
      toast.success(`Reclaimed side ${result.side} for ${result.pool_pair}`);
      await loadEscrow(seedPoolPair);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to reclaim side");
    } finally {
      setReclaiming(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle>Registered Currencies</CardTitle>
            <CardDescription>National currencies registered on the Hub currency registry.</CardDescription>
          </div>
          <Button variant="outline" size="sm" onClick={() => void loadCurrencies()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh
          </Button>
        </CardHeader>
        <CardContent>
          {currencies.length === 0 ? (
            <p className="text-sm text-muted-foreground">No currencies registered.</p>
          ) : (
            <div className="overflow-x-auto">
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
            </div>
          )}
        </CardContent>
      </Card>

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
              readOnly
              placeholder="auto-derived from the two symbols"
            />
          </div>

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
            <Button onClick={() => void onConfirmPair()} disabled={confirming}>
              {confirming ? "Submitting..." : "Confirm Pair"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Droplets className="h-5 w-5" />
            Seed Liquidity (sovereign)
          </CardTitle>
          <CardDescription>
            Each central bank deposits ONLY its own currency. Your side is auto-detected from the pair (no picker).
            When both sides are escrowed, anyone finalizes to fund the pool atomically. You can reclaim your own
            pending side any time before finalize.
          </CardDescription>
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
            <Label htmlFor="seed-amount">Amount (whole tokens, your currency)</Label>
            <Input
              id="seed-amount"
              value={seedAmount}
              onChange={(event) => setSeedAmount(event.target.value)}
              inputMode="decimal"
              placeholder="e.g. 1000"
            />
            <p className="text-xs text-muted-foreground">Whole tokens — converted to 18-decimal base units on submit.</p>
          </div>

          {seedPoolPair ? (
            <div className="md:col-span-2 rounded-md border border-border p-3 text-sm">
              <p className="font-medium">Escrow status — {seedPoolPair}</p>
              {escrow ? (
                <div className="mt-1 grid gap-1 text-xs text-muted-foreground md:grid-cols-3">
                  <span>Side A deposited: {escrow.side_a_deposited ? "✓" : "—"}</span>
                  <span>Side B deposited: {escrow.side_b_deposited ? "✓" : "—"}</span>
                  <span>Finalized: {escrow.finalized ? "✓" : "—"}</span>
                </div>
              ) : (
                <p className="mt-1 text-xs text-muted-foreground">No escrow yet — deposit your side to start.</p>
              )}
            </div>
          ) : null}

          <div className="md:col-span-2 flex flex-wrap gap-2">
            <Button onClick={() => void onDepositSide()} disabled={seeding || !seedPoolPair || !seedAmount.trim()}>
              {seeding ? "Depositing..." : "Deposit My Side"}
            </Button>
            <Button
              variant="outline"
              onClick={() => void onFinalize()}
              disabled={finalizing || !escrow || !escrow.side_a_deposited || !escrow.side_b_deposited || escrow.finalized}
              title={escrow && (!escrow.side_a_deposited || !escrow.side_b_deposited) ? "Both sides must be deposited" : undefined}
            >
              {finalizing ? "Finalizing..." : "Finalize Pool"}
            </Button>
            <Button
              variant="outline"
              onClick={() => void onReclaim()}
              disabled={reclaiming || !escrow || escrow.finalized}
            >
              {reclaiming ? "Reclaiming..." : "Reclaim My Side"}
            </Button>
          </div>
        </CardContent>
      </Card>

    </div>
  );
}
