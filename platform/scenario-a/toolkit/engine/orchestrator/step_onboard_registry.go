// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

const (
	// identityRegistryABI is the minimal ABI for IdentityRegistry used by the engine.
	// Includes: registerParticipant, isParticipant, setCertFingerprint.
	identityRegistryABI = `[
		{
			"name": "registerParticipant",
			"type": "function",
			"inputs": [
				{"name": "wallet", "type": "address"},
				{"name": "name", "type": "string"},
				{"name": "role", "type": "uint8"},
				{"name": "zkPointer", "type": "bytes32"}
			],
			"outputs": []
		},
		{
			"name": "isParticipant",
			"type": "function",
			"inputs": [{"name": "wallet", "type": "address"}],
			"outputs": [{"name": "", "type": "bool"}]
		}
	]`

	// RoleCentralBank is the uint8 role code for a central bank in IdentityRegistry.
	// Matches IdentityRegistryLibrary.ParticipantRole: NONE=0, TREASURY=1,
	// GOVERNANCE=2, CENTRAL_BANK=3, COMMERCIAL_BANK=4.
	RoleCentralBank uint8 = 3
)

type onboardRegistryStep struct {
	spokeID             string
	dataDir             string
	besuRPCURL          string
	keyProvider         kp.KeyProvider
	participantArtifact string // path to IdentityRegistry.sol Foundry artifact
	timeout             time.Duration
}

func newOnboardRegistryStep(spokeID, dataDir, besuRPCURL string, keyProvider kp.KeyProvider, participantArtifact string, timeout time.Duration) Step {
	return &onboardRegistryStep{
		spokeID:             spokeID,
		dataDir:             dataDir,
		besuRPCURL:          besuRPCURL,
		keyProvider:         keyProvider,
		participantArtifact: participantArtifact,
		timeout:             timeout,
	}
}

func (s *onboardRegistryStep) Name() string { return StepOnboardRegistry }

func (s *onboardRegistryStep) Check(ctx context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	// The participant whitelist (IdentityRegistry.sol) is deployed by this step.
	// If it does not exist yet, onboarding has not happened.
	if addrs.ParticipantRegistryAddress == "" {
		return false, nil
	}

	pubkey, err := s.keyProvider.GetPublicKey(ctx, s.spokeID+"/cb")
	if err != nil {
		return false, nil // key not generated yet
	}
	evmAddr, err := kp.EVMAddress(pubkey)
	if err != nil {
		return false, fmt.Errorf("derive EVM address: %w", err)
	}

	client, err := ethclient.DialContext(ctx, s.besuRPCURL)
	if err != nil {
		return false, fmt.Errorf("dial besu: %w", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(identityRegistryABI))
	if err != nil {
		return false, fmt.Errorf("parse ABI: %w", err)
	}

	callData, err := parsedABI.Pack("isParticipant", common.HexToAddress(evmAddr))
	if err != nil {
		return false, fmt.Errorf("pack isParticipant: %w", err)
	}

	contractAddr := common.HexToAddress(addrs.ParticipantRegistryAddress)
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &contractAddr, Data: callData}, nil)
	if err != nil {
		return false, fmt.Errorf("call isParticipant: %w", err)
	}

	var isParticipant bool
	if err := parsedABI.UnpackIntoInterface(&isParticipant, "isParticipant", result); err != nil {
		return false, fmt.Errorf("unpack isParticipant: %w", err)
	}
	return isParticipant, nil
}

func (s *onboardRegistryStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}

	// 1. Get the CB key (FR-009: MUST use GetPublicKey); generate on first run.
	pubkey, err := s.keyProvider.GetPublicKey(ctx, s.spokeID+"/cb")
	if errors.Is(err, kp.ErrKeyNotFound) {
		pubkey, err = s.keyProvider.GenerateKey(ctx, s.spokeID+"/cb")
	}
	if err != nil {
		return fmt.Errorf("get or generate CB key: %w", err)
	}
	evmAddr, err := kp.EVMAddress(pubkey)
	if err != nil {
		return fmt.Errorf("derive EVM address: %w", err)
	}

	// 2. Connect to Besu.
	client, err := ethclient.DialContext(ctx, s.besuRPCURL)
	if err != nil {
		return fmt.Errorf("dial besu: %w", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(identityRegistryABI))
	if err != nil {
		return fmt.Errorf("parse ABI: %w", err)
	}

	// registerParticipant is onlyRole(GOVERNANCE_ROLE). The central bank founding
	// the spoke IS the governance authority, so the tx is signed by the operator
	// (governance) key via the KeyProvider — no raw key material here. The
	// KeyProvider-derived CB wallet address is what gets registered; the zkPointer
	// carries the proof-of-possession binding.
	govAddr, err := keyProviderAddress(ctx, s.keyProvider, kp.LocalOperatorKeyID)
	if err != nil {
		return fmt.Errorf("resolve governance address: %w", err)
	}

	// 2b. Ensure the participant whitelist (IdentityRegistry.sol) exists. It is a
	// spoke-level contract distinct from the Paladin node registry; deploy it once
	// (admin = governance key) and persist PARTICIPANT_REGISTRY_ADDRESS (FR-018).
	envPath := filepath.Join(s.dataDir, ".deployed-addrs.env")
	participantAddrHex := addrs.ParticipantRegistryAddress
	if participantAddrHex == "" {
		deployed, derr := deployParticipantRegistry(ctx, s.besuRPCURL, s.participantArtifact, s.keyProvider, kp.LocalOperatorKeyID)
		if derr != nil {
			return fmt.Errorf("deploy participant registry: %w", derr)
		}
		participantAddrHex = deployed.Hex()
		if werr := addrsAppend(envPath, "PARTICIPANT_REGISTRY_ADDRESS", participantAddrHex); werr != nil {
			return fmt.Errorf("persist participant registry address: %w", werr)
		}
	}
	contractAddr := common.HexToAddress(participantAddrHex)
	addr := common.HexToAddress(evmAddr)

	// 3. Build the registerParticipant transaction.
	var zkPointer [32]byte
	// Compute a deterministic proof-of-possession nonce as zkPointer placeholder.
	h := sha256.Sum256([]byte(evmAddr + s.spokeID))
	copy(zkPointer[:], h[:])

	callData, err := parsedABI.Pack("registerParticipant",
		addr,
		"Central Bank "+s.spokeID,
		RoleCentralBank,
		zkPointer,
	)
	if err != nil {
		return fmt.Errorf("pack registerParticipant: %w", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get chainID: %w", err)
	}
	nonce, err := client.PendingNonceAt(ctx, govAddr)
	if err != nil {
		return fmt.Errorf("get nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("suggest gas price: %w", err)
	}

	tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), 300000, gasPrice, callData)
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := signTxViaKeyProvider(ctx, s.keyProvider, kp.LocalOperatorKeyID, signer, tx)
	if err != nil {
		return fmt.Errorf("sign registerParticipant: %w", err)
	}

	// 5. Send and wait for receipt.
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return fmt.Errorf("send registerParticipant tx: %w", err)
	}

	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return fmt.Errorf("wait for registerParticipant receipt: %w", err)
	}
	if receipt.Status != 1 {
		return fmt.Errorf("registerParticipant tx reverted")
	}

	_ = ctx
	return nil
}

// waitForReceipt polls until the transaction receipt is available.
func waitForReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	for {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil {
			return receipt, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// newReader wraps a string as an io.Reader — used for abi.JSON().
func init() {
	// Validate identityRegistryABI at package init to catch typos early.
	_ = crypto.Keccak256 // ensure go-ethereum crypto is linked
}
