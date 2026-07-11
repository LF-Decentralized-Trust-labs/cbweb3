package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
)

// sovereignPairSteps builds the soft sovereign-pair tail of found-spoke (TK-B9):
// open-sovereign-pair → commit-liquidity → seed-oracle. All Soft: a failure or a
// pending sovereign act never blocks found-spoke nor the spoke bundle. Strict
// sovereignty: each run signs ONLY the current CB's act (no counterparty key).
func sovereignPairSteps(c SpokeConfig) []Step {
	p := c.Pair
	pid := pairID(p.SymbolA, p.SymbolB)
	isProposer := p.CurrentCB == p.ProposerCB
	isConfirmer := p.CurrentCB == p.ConfirmerCB

	statusFn := c.PairStatus
	if statusFn == nil {
		statusFn = func(ctx context.Context, id string) (string, bool, error) {
			return pairStatus(ctx, c.Runner, c.HubRPC, c.HubPairRegistry, id)
		}
	}

	// side/token/amount for the current CB's own currency (cooperative liquidity).
	side := "0"
	sym := p.SymbolA
	amount := p.AmountA
	if isConfirmer {
		side, sym, amount = "1", p.SymbolB, p.AmountB
	}
	wTokenKey := "WTOKEN_" + sym + "_ADDRESS"

	return []Step{
		{
			Name: "open-sovereign-pair",
			Deps: []string{"add-noc-agent"},
			Soft: true,
			Check: func(ctx context.Context) (bool, error) {
				status, _, _ := statusFn(ctx, pid)
				if status == "ACTIVE" {
					return true, nil // corridor already open
				}
				if status == "PROPOSED" && isProposer {
					return true, nil // I already proposed; awaiting the counterparty
				}
				return false, nil
			},
			Run: func(ctx context.Context) error {
				status, exists, _ := statusFn(ctx, pid)
				switch {
				case isProposer:
					if !exists {
						if err := c.scaffoldPair(ctx, pid); err != nil {
							return err
						}
						return c.proposePair(ctx, pid)
					}
					return nil // PROPOSED already (Check normally skips this)
				case isConfirmer:
					if status == "PROPOSED" {
						return c.confirmPair(ctx, pid)
					}
					// The proposer (CB-A) has not proposed yet: pending (soft, non-fatal).
					return fmt.Errorf("open-sovereign-pair: pair %q not yet PROPOSED by %s (pending)", pid, p.ProposerCB)
				default:
					return fmt.Errorf("open-sovereign-pair: %s is neither proposer nor confirmer of %q", p.CurrentCB, pid)
				}
			},
		},
		{
			Name: "commit-liquidity",
			Deps: []string{"open-sovereign-pair"},
			Soft: true,
			Check: func(ctx context.Context) (bool, error) {
				// Skip if a commit for this CB's side is already pending/matched.
				out, err := c.Runner.Run(ctx, "cast", "call", c.lcrAddr(), "getPendingCommit(string,uint8)", pid, side, "--rpc-url", c.HubRPC)
				if err != nil {
					return false, nil
				}
				return commitExists(string(out)), nil
			},
			Run: func(ctx context.Context) error {
				wToken := c.envValue(wTokenKey)
				_, err := c.Runner.Run(ctx, "cast", "send", c.lcrAddr(),
					"registerCommit(string,uint8,uint256,address)", pid, side, amount, wToken,
					"--rpc-url", c.HubRPC, "--private-key", c.CBHubKey)
				return err
			},
		},
		{
			Name: "seed-oracle",
			Deps: []string{"open-sovereign-pair"},
			Soft: true,
			Check: func(context.Context) (bool, error) {
				// local-only convenience (FR-009): skip outside local.
				return c.Environment != "local", nil
			},
			Run: func(ctx context.Context) error {
				tokenA := c.envValue("WTOKEN_" + p.SymbolA + "_ADDRESS")
				tokenB := c.envValue("WTOKEN_" + p.SymbolB + "_ADDRESS")
				_, err := c.Runner.Run(ctx, "cast", "send", c.HubManualOracle,
					"setRate(address,address,uint256)", tokenA, tokenB, p.Rate,
					"--rpc-url", c.HubRPC, "--private-key", c.CBHubKey)
				return err
			},
		},
	}
}

// scaffoldPair deploys/dedups the W-tokens (per currency), the pair AMM and the
// LiquidityCommitRegistry, then maps setCentralBankOf and grants the relayer —
// all signed with the hub admin key (an admin act, not a sovereign one).
func (c SpokeConfig) scaffoldPair(ctx context.Context, pid string) error {
	p := c.Pair
	// W-tokens, deduplicated by currency (reused across corridors).
	for _, sym := range []string{p.SymbolA, p.SymbolB} {
		key := "WTOKEN_" + sym + "_ADDRESS"
		if addrs.HasAddrKey(c.SpokeEnvFile, key) {
			continue // currency already has a W-token — reuse
		}
		out, err := c.Runner.Run(ctx, "forge", "create",
			"src/TokenizedCentralBankMoney.sol:TokenizedCentralBankMoney",
			"--root", c.ContractsDir, "--rpc-url", c.HubRPC, "--private-key", c.HubAdminKey,
			"--constructor-args", "W-"+sym, "W-"+sym, c.HubIdentityRegistry)
		if err != nil {
			return err
		}
		if err := addrs.AppendAddr(c.SpokeEnvFile, key, parseForgeCreateAddr(string(out))); err != nil {
			return err
		}
	}
	tokenA := c.envValue("WTOKEN_" + p.SymbolA + "_ADDRESS")
	tokenB := c.envValue("WTOKEN_" + p.SymbolB + "_ADDRESS")

	// AMM for the pair (carries its own circuit breaker).
	ammOut, err := c.Runner.Run(ctx, "forge", "create",
		"src/AutomatedMarketMaker.sol:AutomatedMarketMaker",
		"--root", c.ContractsDir, "--rpc-url", c.HubRPC, "--private-key", c.HubAdminKey,
		"--constructor-args", tokenA, tokenB, c.HubIdentityRegistry)
	if err != nil {
		return err
	}
	if err := addrs.AppendAddr(c.SpokeEnvFile, "AMM_"+pid+"_ADDRESS", parseForgeCreateAddr(string(ammOut))); err != nil {
		return err
	}

	// LiquidityCommitRegistry: reuse if known, else deploy.
	if !addrs.HasAddrKey(c.SpokeEnvFile, "LIQUIDITY_COMMIT_REGISTRY_ADDRESS") {
		lcrOut, err := c.Runner.Run(ctx, "forge", "create",
			"src/LiquidityCommitRegistry.sol:LiquidityCommitRegistry",
			"--root", c.ContractsDir, "--rpc-url", c.HubRPC, "--private-key", c.HubAdminKey,
			"--constructor-args", c.HubIdentityRegistry)
		if err != nil {
			return err
		}
		if err := addrs.AppendAddr(c.SpokeEnvFile, "LIQUIDITY_COMMIT_REGISTRY_ADDRESS", parseForgeCreateAddr(string(lcrOut))); err != nil {
			return err
		}
	}

	// Map EACH W-token to its CB (FR-003): tokenA→proposer, tokenB→confirmer.
	// This is an admin act (hub admin key) — declaring the registry mapping — so
	// no counterparty key is used and strict sovereignty is preserved. The LCR
	// gates registerCommit by these mappings, so BOTH are required for each CB to
	// commit its own side.
	lcr := c.lcrAddr()
	proposerAddr := firstNonEmptyStr(c.Pair.ProposerCBAddress, c.CBAddress)
	if _, err := c.Runner.Run(ctx, "cast", "send", lcr, "setCentralBankOf(address,address)", tokenA, proposerAddr, "--rpc-url", c.HubRPC, "--private-key", c.HubAdminKey); err != nil {
		return err
	}
	if c.Pair.ConfirmerCBAddress != "" {
		if _, err := c.Runner.Run(ctx, "cast", "send", lcr, "setCentralBankOf(address,address)", tokenB, c.Pair.ConfirmerCBAddress, "--rpc-url", c.HubRPC, "--private-key", c.HubAdminKey); err != nil {
			return err
		}
	}
	if c.RelayerAddr != "" {
		for _, tok := range []string{tokenA, tokenB} {
			if _, err := c.Runner.Run(ctx, "cast", "send", tok, "grantRole(bytes32,address)", "CENTRAL_BANK_ROLE", c.RelayerAddr, "--rpc-url", c.HubRPC, "--private-key", c.HubAdminKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c SpokeConfig) proposePair(ctx context.Context, pid string) error {
	p := c.Pair
	tokenA := c.envValue("WTOKEN_" + p.SymbolA + "_ADDRESS")
	tokenB := c.envValue("WTOKEN_" + p.SymbolB + "_ADDRESS")
	amm := c.envValue("AMM_" + pid + "_ADDRESS")
	_, err := c.Runner.Run(ctx, "cast", "send", c.HubPairRegistry,
		"proposePair(string,address,address,address)", pid, tokenA, tokenB, amm,
		"--rpc-url", c.HubRPC, "--private-key", c.CBHubKey)
	return err
}

func (c SpokeConfig) confirmPair(ctx context.Context, pid string) error {
	_, err := c.Runner.Run(ctx, "cast", "send", c.HubPairRegistry,
		"confirmPair(string)", pid, "--rpc-url", c.HubRPC, "--private-key", c.CBHubKey)
	return err
}

func (c SpokeConfig) lcrAddr() string {
	if a := c.envValue("LIQUIDITY_COMMIT_REGISTRY_ADDRESS"); a != "" {
		return a
	}
	return c.HubManualOracle // never used when unset; keeps calls well-formed in dry paths
}

// envValue returns the value for key in the spoke env file, or "".
func (c SpokeConfig) envValue(key string) string {
	b, err := os.ReadFile(c.SpokeEnvFile)
	if err != nil {
		return ""
	}
	prefix := key + "="
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

var forgeDeployedRE = regexp.MustCompile(`(?i)Deployed to:\s*(0x[0-9a-fA-F]{40})`)

// parseForgeCreateAddr extracts the deployed address from `forge create` output.
func parseForgeCreateAddr(out string) string {
	if m := forgeDeployedRE.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return strings.TrimSpace(out)
}

// firstNonEmptyStr returns the first non-empty string.
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// commitExists reports whether a getPendingCommit result denotes an existing
// (non-zero) commit id.
func commitExists(out string) bool {
	s := strings.TrimSpace(out)
	if s == "" {
		return false
	}
	// A zero commitId (all-zero bytes32) means no pending commit.
	zero := "0x" + strings.Repeat("0", 64)
	return !strings.Contains(s, zero) && strings.Contains(s, "0x")
}
