// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// RoleCommercialBank is the uint8 role code for a commercial bank in
// IdentityRegistry. Matches IdentityRegistryLibrary.ParticipantRole: NONE=0,
// TREASURY=1, GOVERNANCE=2, CENTRAL_BANK=3, COMMERCIAL_BANK=4.
const RoleCommercialBank uint8 = 4

// RegisterParticipantParams carries the inputs for an on-chain participant
// registration in the spoke's IdentityRegistry.
type RegisterParticipantParams struct {
	// DataDir is the central bank's data directory; its .deployed-addrs.env holds
	// PARTICIPANT_REGISTRY_ADDRESS (the IdentityRegistry deployed by onboard-registry).
	DataDir string
	// BesuRPCURL is the spoke's Besu JSON-RPC endpoint.
	BesuRPCURL string
	// KeyProvider signs the transaction; SignerKeyID identifies the governance key.
	KeyProvider kp.KeyProvider
	SignerKeyID string
	// Wallet is the participant's on-chain address (the bank's runtime KMS wallet).
	Wallet common.Address
	// Name is the human-readable participant name stored on-chain.
	Name string
	// Role is the IdentityRegistry ParticipantRole code (e.g. RoleCommercialBank).
	Role uint8
	// SpokeID seeds the deterministic zkPointer placeholder, matching onboard-registry.
	SpokeID string
	// Timeout bounds the whole registration (connect + send + receipt wait).
	Timeout time.Duration
}

// RegisterParticipantOnChain whitelists a wallet in the spoke's IdentityRegistry
// via registerParticipant, which is onlyRole(GOVERNANCE_ROLE) — so the transaction
// is signed by the central bank's governance key through the KeyProvider (no raw
// key material here). This mirrors onboard-registry but targets an arbitrary
// wallet/role (e.g. a joining commercial bank's runtime KMS wallet), keeping all
// governance authority and key custody in the engine, never in the CB services.
//
// Idempotent: if the wallet is already a participant, it returns already=true with
// no transaction. Requires that onboard-registry has already deployed the
// IdentityRegistry (PARTICIPANT_REGISTRY_ADDRESS must be present).
func RegisterParticipantOnChain(ctx context.Context, p RegisterParticipantParams) (txHash string, already bool, err error) {
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}

	addrs, err := parseDeployedAddrs(filepath.Join(p.DataDir, ".deployed-addrs.env"))
	if err != nil {
		return "", false, fmt.Errorf("read deployed-addrs: %w", err)
	}
	if addrs.ParticipantRegistryAddress == "" {
		return "", false, fmt.Errorf("IdentityRegistry not deployed (PARTICIPANT_REGISTRY_ADDRESS empty); run the central bank's apply (onboard-registry) first")
	}
	contractAddr := common.HexToAddress(addrs.ParticipantRegistryAddress)

	client, err := ethclient.DialContext(ctx, p.BesuRPCURL)
	if err != nil {
		return "", false, fmt.Errorf("dial besu: %w", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(identityRegistryABI))
	if err != nil {
		return "", false, fmt.Errorf("parse ABI: %w", err)
	}

	// Idempotency: skip if already whitelisted. registerParticipant sets
	// status=Verified, so canTransact returns true once the wallet is registered.
	if isCall, perr := parsedABI.Pack("canTransact", p.Wallet); perr == nil {
		if result, cerr := client.CallContract(ctx, ethereum.CallMsg{To: &contractAddr, Data: isCall}, nil); cerr == nil {
			var registered bool
			if uerr := parsedABI.UnpackIntoInterface(&registered, "canTransact", result); uerr == nil && registered {
				return "", true, nil
			}
		}
	}

	// registerParticipant is onlyRole(GOVERNANCE_ROLE): sign with the governance key.
	govAddr, err := keyProviderAddress(ctx, p.KeyProvider, p.SignerKeyID)
	if err != nil {
		return "", false, fmt.Errorf("resolve governance address: %w", err)
	}

	// Deterministic proof-of-possession placeholder (matches onboard-registry).
	var zkPointer [32]byte
	h := sha256.Sum256([]byte(p.Wallet.Hex() + p.SpokeID))
	copy(zkPointer[:], h[:])

	callData, err := parsedABI.Pack("registerParticipant", p.Wallet, p.Name, p.Role, zkPointer)
	if err != nil {
		return "", false, fmt.Errorf("pack registerParticipant: %w", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", false, fmt.Errorf("get chainID: %w", err)
	}
	nonce, err := client.PendingNonceAt(ctx, govAddr)
	if err != nil {
		return "", false, fmt.Errorf("get nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", false, fmt.Errorf("suggest gas price: %w", err)
	}

	tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), 300000, gasPrice, callData)
	signedTx, err := signTxViaKeyProvider(ctx, p.KeyProvider, p.SignerKeyID, types.NewEIP155Signer(chainID), tx)
	if err != nil {
		return "", false, fmt.Errorf("sign registerParticipant: %w", err)
	}
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return "", false, fmt.Errorf("send registerParticipant tx: %w", err)
	}

	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return "", false, fmt.Errorf("wait for registerParticipant receipt: %w", err)
	}
	if receipt.Status != 1 {
		return "", false, fmt.Errorf("registerParticipant tx reverted")
	}
	return signedTx.Hash().Hex(), false, nil
}
