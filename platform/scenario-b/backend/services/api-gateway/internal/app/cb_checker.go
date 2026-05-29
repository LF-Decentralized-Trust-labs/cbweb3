// Package app — CentralBankChecker implementations for the anti-G5-cross guard (FR-004 / T016).
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ─── On-chain checker (primary) ──────────────────────────────────────────────

// cbCheckerABI is the minimal ABI for the Hub IdentityRegistry canGovern check.
// canGovern returns true for CENTRAL_BANK and GOVERNANCE roles — both are CB-controlled
// addresses that must not receive minted tokens from another CB (anti-G5-cross guard).
const cbCheckerABI = `[
{"type":"function","name":"canGovern","stateMutability":"view",
 "inputs":[{"name":"account","type":"address"}],
 "outputs":[{"name":"","type":"bool"}]}
]`

// onchainCBChecker implements handlers.CentralBankChecker using the Hub IdentityRegistry.
// It has cross-institution visibility: a CB-A gateway can detect CB-B's signer address
// without sharing a compliance DB, because both are registered on the shared Hub chain.
type onchainCBChecker struct {
	contract common.Address
	ec       *ethclient.Client
	timeout  time.Duration
}

// newOnchainCBChecker creates an onchainCBChecker connected to the Hub IdentityRegistry.
func newOnchainCBChecker(rpcURL, contractAddress string) (*onchainCBChecker, error) {
	if rpcURL == "" || contractAddress == "" {
		return nil, fmt.Errorf("rpcURL and contractAddress are required")
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ec, err := evm.Dial(dialCtx, rpcURL, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("onchainCBChecker: dial %s: %w", rpcURL, err)
	}
	// Validate ABI at construction time.
	if _, err := evm.ParseABI(cbCheckerABI); err != nil {
		ec.Close()
		return nil, fmt.Errorf("onchainCBChecker: parse ABI: %w", err)
	}
	return &onchainCBChecker{
		contract: common.HexToAddress(contractAddress),
		ec:       ec,
		timeout:  10 * time.Second,
	}, nil
}

// IsCentralBankAddress returns true if the address has CENTRAL_BANK or GOVERNANCE
// role in the Hub IdentityRegistry (canGovern check). This is the source of truth
// for the anti-G5-cross guard regardless of which CB instance is running.
func (c *onchainCBChecker) IsCentralBankAddress(ctx context.Context, address string) (bool, error) {
	addrNorm := strings.TrimSpace(address)
	if addrNorm == "" {
		return false, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	parsed, err := evm.ParseABI(cbCheckerABI)
	if err != nil {
		return false, fmt.Errorf("onchainCBChecker: parse ABI: %w", err)
	}
	var result bool
	if err := evm.Call(callCtx, c.ec, c.contract, parsed, "canGovern",
		[]interface{}{common.HexToAddress(addrNorm)}, &result); err != nil {
		return false, fmt.Errorf("onchainCBChecker: canGovern(%s): %w", addrNorm, err)
	}
	return result, nil
}

// ─── Off-chain checker (fallback) ────────────────────────────────────────────

// identityCBChecker implements handlers.CentralBankChecker using the compliance gRPC service.
// It only has visibility into the local CB's own participants. Used as fallback when the
// Hub IdentityRegistry address is not configured (non-CB environments).
type identityCBChecker struct {
	userManager interfaces.UserManager
}

// newIdentityCBChecker creates an identityCBChecker backed by the given UserManager.
func newIdentityCBChecker(userManager interfaces.UserManager) *identityCBChecker {
	return &identityCBChecker{userManager: userManager}
}

// IsCentralBankAddress returns true if address is the wallet of any active
// Scenario B central_bank participant (case-insensitive).
func (c *identityCBChecker) IsCentralBankAddress(ctx context.Context, address string) (bool, error) {
	addrNorm := strings.TrimSpace(address)
	if addrNorm == "" {
		return false, nil
	}

	users, _, err := c.userManager.ListUsers(ctx, domain.RoleCentralBankScenarioB, "")
	if err != nil {
		return false, fmt.Errorf("cb_checker: list central bank users: %w", err)
	}

	for _, u := range users {
		if strings.EqualFold(u.WalletAddress, addrNorm) {
			return true, nil
		}
	}
	return false, nil
}
