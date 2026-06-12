// SPDX-License-Identifier: Apache-2.0

// Package app — startup bootstrap for Hub IdentityRegistry LP role.
// On every api-gateway start, if HUB_IDENTITY_REGISTRY_ADDRESS and CB_PRIVATE_KEY
// are set, ensures LOCAL_CB_HUB_SIGNER is registered as LiquidityProvider on the Hub
// IdentityRegistry (007-bridge-based-cb-liquidity / FR-004).
package app

import (
	"context"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/common"
)

// identityRegistryBootstrapABI — minimal ABI for LP role check + grant.
const identityRegistryBootstrapABI = `[
{"type":"function","name":"isLiquidityProvider","stateMutability":"view",
 "inputs":[{"name":"account","type":"address"}],
 "outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"grantLiquidityProvider","stateMutability":"nonpayable",
 "inputs":[{"name":"account","type":"address"}],
 "outputs":[]}
]`

// bootstrapLiquidityProviderRole ensures the CB's Hub signer has the LiquidityProvider
// role in the on-chain IdentityRegistry. It is idempotent: if the signer is already
// registered the function returns immediately without sending a transaction.
//
// Required env vars (all others are silently skipped):
//   - HUB_IDENTITY_REGISTRY_ADDRESS — Hub IdentityRegistry contract address
//   - LOCAL_CB_HUB_SIGNER           — CB's Hub Ethereum signer address
//   - CB_PRIVATE_KEY                — DEFAULT_ADMIN_ROLE holder key (governance admin)
//   - HUB_BESU_RPC_URL              — Hub JSON-RPC endpoint
//   - HUB_CHAIN_ID                  — Hub chain ID (integer string)
func bootstrapLiquidityProviderRole(ctx context.Context) {
	registryAddr := os.Getenv("HUB_IDENTITY_REGISTRY_ADDRESS")
	signerAddr := os.Getenv("LOCAL_CB_HUB_SIGNER")
	adminKey := os.Getenv("CB_PRIVATE_KEY")
	rpcURL := os.Getenv("HUB_BESU_RPC_URL")

	if registryAddr == "" || signerAddr == "" || adminKey == "" || rpcURL == "" {
		// Not a CB environment or vars not yet populated — skip silently.
		return
	}

	chainID := resolveBootstrapHubChainID(log.New(os.Stderr, "[identity-bootstrap] ", 0))

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ec, err := evm.Dial(dialCtx, rpcURL, 10*time.Second)
	if err != nil {
		log.Printf("[identity-bootstrap] dial Hub RPC (%s): %v — skipping LP grant", rpcURL, err)
		return
	}
	defer ec.Close()

	parsedABI, err := evm.ParseABI(identityRegistryBootstrapABI)
	if err != nil {
		log.Printf("[identity-bootstrap] parse ABI: %v", err)
		return
	}

	contract := common.HexToAddress(registryAddr)
	target := common.HexToAddress(signerAddr)

	// Check idempotently: skip grant if already a LP.
	checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
	defer checkCancel()

	var isLP bool
	if err := evm.Call(checkCtx, ec, contract, parsedABI, "isLiquidityProvider",
		[]interface{}{target}, &isLP); err != nil {
		log.Printf("[identity-bootstrap] isLiquidityProvider check: %v — skipping LP grant", err)
		return
	}
	if isLP {
		log.Printf("[identity-bootstrap] %s is already a LiquidityProvider — no action needed", signerAddr)
		return
	}

	// Grant LP role using the admin key (DEFAULT_ADMIN_ROLE on IdentityRegistry).
	signer, err := evm.NewSigner(adminKey, big.NewInt(chainID))
	if err != nil {
		log.Printf("[identity-bootstrap] build signer from CB_PRIVATE_KEY: %v", err)
		return
	}

	grantCtx, grantCancel := context.WithTimeout(ctx, 30*time.Second)
	defer grantCancel()

	txHash, err := evm.SubmitTx(grantCtx, ec, signer, contract, parsedABI,
		"grantLiquidityProvider", target)
	if err != nil {
		log.Printf("[identity-bootstrap] grantLiquidityProvider(%s): %v", signerAddr, err)
		return
	}

	log.Printf("[identity-bootstrap] grantLiquidityProvider(%s) OK — tx=%s", signerAddr, txHash)
}

// resolveBootstrapHubChainID reads HUB_CHAIN_ID from the environment, defaulting
// to 1337. Emits a warning when absent so operators see a visible signal rather
// than a silent assumption (FR-009 / Constitution Principle VI).
func resolveBootstrapHubChainID(warnLogger *log.Logger) int64 {
	raw := strings.TrimSpace(os.Getenv("HUB_CHAIN_ID"))
	if raw == "" {
		warnLogger.Printf("warning: HUB_CHAIN_ID not set; defaulting to 1337 (set HUB_CHAIN_ID to suppress this warning)")
		return 1337
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		warnLogger.Printf("warning: HUB_CHAIN_ID=%q invalid; defaulting to 1337", raw)
		return 1337
	}
	return id
}
