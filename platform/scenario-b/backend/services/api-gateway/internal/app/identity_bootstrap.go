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
		log.Printf("[identity-bootstrap] grantLiquidityProvider(%s): %v", signerAddr, err) // #nosec G706 -- signerAddr is a blockchain address from config, not user input
		return
	}

	log.Printf("[identity-bootstrap] grantLiquidityProvider(%s) OK — tx=%s", signerAddr, txHash) // #nosec G706 -- signerAddr/txHash are blockchain values, not user input
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

// wTokenRoleABI is the minimal AccessControl surface of a sovereign W-token needed to grant the
// relayer its issuance role.
const wTokenRoleABI = `[
{"type":"function","name":"CENTRAL_BANK_ROLE","stateMutability":"view",
 "inputs":[],"outputs":[{"name":"","type":"bytes32"}]},
{"type":"function","name":"hasRole","stateMutability":"view",
 "inputs":[{"name":"role","type":"bytes32"},{"name":"account","type":"address"}],
 "outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"grantRole","stateMutability":"nonpayable",
 "inputs":[{"name":"role","type":"bytes32"},{"name":"account","type":"address"}],
 "outputs":[]}
]`

// bootstrapRelayerIssuanceRole grants this central bank's bridge relayer CENTRAL_BANK_ROLE on
// the CB's own sovereign W-token, so it can mint on bridge-in and burn on bridge-out.
//
// Why the relayer needs its own identity: go-ethereum tracks nonces per process, so a gateway
// and a relayer sharing one key each keep their own counter and concurrent submissions claim
// the same nonce. One transaction is then replaced — and because the relayer persists a
// burn/mint hash as an intent the moment it broadcasts, a replaced transaction leaves the
// position waiting on a hash that will never be mined.
//
// Why this gateway may grant it: the currency handover made this CB the token's
// DEFAULT_ADMIN_ROLE holder (before it, the hub kept administration and could have granted
// issuance back to itself at any time).
//
// Idempotent and best-effort: it returns silently when not applicable, and logs rather than
// failing startup — the grant is retried on every boot, and a missing grant surfaces as an
// explicit AccessControl revert on the relayer's first mint rather than as silent corruption.
func bootstrapRelayerIssuanceRole(ctx context.Context) {
	tokenAddr := strings.TrimSpace(os.Getenv("W_TOKEN_ADDRESS"))
	relayerAddr := strings.TrimSpace(os.Getenv("HUB_RELAYER_ADDRESS"))
	adminKey := strings.TrimSpace(os.Getenv("SIGNER_PRIVATE_KEY"))
	rpcURL := strings.TrimSpace(os.Getenv("HUB_BESU_RPC_URL"))

	// A commercial bank has no W-token, no relayer and no hub key — nothing to do.
	if tokenAddr == "" || relayerAddr == "" || adminKey == "" || rpcURL == "" {
		return
	}

	chainID := resolveBootstrapHubChainID(log.New(os.Stderr, "[relayer-role] ", 0))

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ec, err := evm.Dial(dialCtx, rpcURL, 10*time.Second)
	if err != nil {
		log.Printf("[relayer-role] dial Hub RPC (%s): %v — relayer issuance grant skipped", rpcURL, err)
		return
	}
	defer ec.Close()

	parsedABI, err := evm.ParseABI(wTokenRoleABI)
	if err != nil {
		log.Printf("[relayer-role] parse W-token ABI: %v", err)
		return
	}

	token := common.HexToAddress(tokenAddr)
	relayer := common.HexToAddress(relayerAddr)

	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readCancel()

	var role [32]byte
	if err := evm.Call(readCtx, ec, token, parsedABI, "CENTRAL_BANK_ROLE", nil, &role); err != nil {
		log.Printf("[relayer-role] read CENTRAL_BANK_ROLE on %s: %v — grant skipped", tokenAddr, err)
		return
	}
	var already bool
	if err := evm.Call(readCtx, ec, token, parsedABI, "hasRole",
		[]interface{}{role, relayer}, &already); err != nil {
		log.Printf("[relayer-role] read relayer role: %v — grant skipped", err)
		return
	}
	if already {
		log.Printf("[relayer-role] %s already holds CENTRAL_BANK_ROLE on %s — no action needed", relayerAddr, tokenAddr) // #nosec G706 -- config-sourced blockchain addresses
		return
	}

	signer, err := evm.NewSigner(adminKey, big.NewInt(chainID))
	if err != nil {
		log.Printf("[relayer-role] build signer from SIGNER_PRIVATE_KEY: %v", err)
		return
	}

	grantCtx, grantCancel := context.WithTimeout(ctx, 30*time.Second)
	defer grantCancel()
	txHash, err := evm.SubmitTx(grantCtx, ec, signer, token, parsedABI, "grantRole", role, relayer)
	if err != nil {
		// The likeliest cause is this CB not holding the token's DEFAULT_ADMIN_ROLE, i.e. the
		// currency handover has not completed. Say so instead of leaving a bare revert.
		log.Printf("[relayer-role] grantRole(CENTRAL_BANK_ROLE, %s) on %s failed: %v — the relayer cannot mint or burn until this succeeds; check that the currency authority handover completed and this gateway holds the token's DEFAULT_ADMIN_ROLE", relayerAddr, tokenAddr, err) // #nosec G706 -- config-sourced blockchain addresses
		return
	}
	log.Printf("[relayer-role] granted CENTRAL_BANK_ROLE to the relayer %s on %s — tx=%s", relayerAddr, tokenAddr, txHash) // #nosec G706 -- config-sourced blockchain values
}
