// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// runRegisterParticipant implements `cbweb3 register-participant`: the central
// bank's governance action that whitelists a joined commercial bank's runtime
// wallet in the spoke's IdentityRegistry.
//
// Onboarding is split (concat.md): the Governance Portal approves KYC and issues
// the bank's certificate (CompleteOnboarding), while the on-chain whitelisting —
// registerParticipant, onlyRole(GOVERNANCE_ROLE) — is performed here by the engine
// using the CB governance key via the KeyProvider. This keeps every governance
// private key inside the engine and out of the CB services. The operator runs this
// once a bank has completed portal onboarding (its KMS wallet exists).
func runRegisterParticipant(args []string) int {
	fs := flag.NewFlagSet("register-participant", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		manifestFile string
		bankCode     string
		walletHex    string
		apiURL       string
		name         string
		timeout      time.Duration
	)
	fs.StringVar(&manifestFile, "f", "", "path to the central bank's ParticipantDeployment YAML manifest (required)")
	fs.StringVar(&manifestFile, "file", "", "path to the central bank's ParticipantDeployment YAML manifest (required)")
	fs.StringVar(&bankCode, "bank", "", "commercial bank code to register (e.g. bank-itau)")
	fs.StringVar(&walletHex, "wallet", "", "the bank's on-chain wallet address (0x…); if omitted, looked up via the CB api-gateway by --bank")
	fs.StringVar(&apiURL, "api-url", "", "CB api-gateway base URL for wallet lookup (default: http://localhost:<rpc.port+10000>)")
	fs.StringVar(&name, "name", "", "participant name stored on-chain (default: \"Commercial Bank <bank>\")")
	fs.DurationVar(&timeout, "timeout", 2*time.Minute, "registration timeout")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if manifestFile == "" {
		fmt.Fprintln(os.Stderr, "required flag -f (--file) is missing")
		return 1
	}
	if bankCode == "" {
		fmt.Fprintln(os.Stderr, "required flag --bank is missing")
		return 1
	}

	m, err := manifest.Load(manifestFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "manifest file not found: %v\n", err)
		return 1
	}
	if err := manifest.Validate(m); err != nil {
		fmt.Fprintf(os.Stderr, "validation error: %v\n", err)
		return 1
	}
	// Resolve a relative spec.node.dataDir against the current working directory,
	// matching the apply path so register-participant reads the same .deployed-addrs.env.
	if err := manifest.ResolveDataDir(m); err != nil {
		fmt.Fprintf(os.Stderr, "validation error: %v\n", err)
		return 1
	}
	if m.Spec.Mode != "found" {
		fmt.Fprintf(os.Stderr, "register-participant must run against a central bank (mode: found) manifest, got mode: %s\n", m.Spec.Mode)
		return 1
	}
	if m.Spec.Environment != "local" {
		fmt.Fprintf(os.Stderr, "environment %s is not yet supported — only local is available\n", m.Spec.Environment)
		return 1
	}

	rpcPort := 0
	if m.Spec.Node.RPC != nil {
		rpcPort = m.Spec.Node.RPC.Port
	}
	if rpcPort == 0 {
		fmt.Fprintln(os.Stderr, "manifest spec.node.rpc.port is required")
		return 1
	}
	besuRPCURL := fmt.Sprintf("http://localhost:%d", rpcPort)
	dataDir := m.Spec.Node.DataDir

	keyProvider, err := kp.New(m.Spec.KeyProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyProvider URI error: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Resolve the bank wallet: explicit --wallet wins; otherwise query the CB
	// api-gateway my-status endpoint by bank_code (returns the approved wallet).
	if walletHex == "" {
		if apiURL == "" {
			apiURL = fmt.Sprintf("http://localhost:%d", rpcPort+10000)
		}
		walletHex, err = lookupBankWallet(ctx, apiURL, bankCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve bank wallet via %s: %v\n", apiURL, err)
			fmt.Fprintln(os.Stderr, "hint: ensure the bank completed portal onboarding, or pass --wallet 0x…")
			return 1
		}
	}
	if !common.IsHexAddress(walletHex) {
		fmt.Fprintf(os.Stderr, "invalid wallet address: %q\n", walletHex)
		return 1
	}
	if name == "" {
		name = "Commercial Bank " + bankCode
	}

	txHash, already, err := orchestrator.RegisterParticipantOnChain(ctx, orchestrator.RegisterParticipantParams{
		DataDir:     dataDir,
		BesuRPCURL:  besuRPCURL,
		KeyProvider: keyProvider,
		SignerKeyID: kp.LocalOperatorKeyID,
		Wallet:      common.HexToAddress(walletHex),
		Name:        name,
		Role:        orchestrator.RoleCommercialBank,
		SpokeID:     m.Spec.Spoke.ID,
		Timeout:     timeout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "register-participant failed: %v\n", err)
		return 1
	}
	if already {
		fmt.Printf("%s (%s) already registered in IdentityRegistry — no-op\n", bankCode, walletHex)
		return 0
	}
	fmt.Printf("registered %s (%s) as COMMERCIAL_BANK — tx %s\n", bankCode, walletHex, txHash)
	return 0
}

// lookupBankWallet queries the CB api-gateway my-status endpoint by bank_code and
// returns the wallet_address it reports.
func lookupBankWallet(ctx context.Context, apiURL, bankCode string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/onboarding/my-status?bank_code=%s", apiURL, bankCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("my-status returned HTTP %d", resp.StatusCode)
	}
	var body struct {
		WalletAddress string `json:"wallet_address"`
		Status        string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode my-status: %w", err)
	}
	if body.WalletAddress == "" {
		return "", fmt.Errorf("my-status returned no wallet_address (status %q); bank may not have completed onboarding", body.Status)
	}
	return body.WalletAddress, nil
}
