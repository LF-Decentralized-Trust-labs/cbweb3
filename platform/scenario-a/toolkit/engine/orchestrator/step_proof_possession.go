// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// RoleCommercialBank is the uint8 role code for a commercial bank in IdentityRegistry.
const RoleCommercialBank uint8 = 2

// proofPossessionStep proves possession of the commercial bank's blockchain key
// (by signing the registration transaction with it via the KeyProvider) and
// registers the bank's EVM address in the spoke's IdentityRegistry.
type proofPossessionStep struct {
	bankCode        string
	registryAddress string
	besuRPCURL      string
	keyProvider     kp.KeyProvider
	timeout         time.Duration
}

func newProofPossessionStep(bankCode, registryAddress, besuRPCURL string, keyProvider kp.KeyProvider, timeout time.Duration) Step {
	return &proofPossessionStep{
		bankCode:        bankCode,
		registryAddress: registryAddress,
		besuRPCURL:      besuRPCURL,
		keyProvider:     keyProvider,
		timeout:         timeout,
	}
}

func (s *proofPossessionStep) Name() string { return StepProofPossession }

func (s *proofPossessionStep) Check(ctx context.Context) (bool, error) {
	pubkey, err := s.keyProvider.GetPublicKey(ctx, s.bankCode)
	if err != nil {
		return false, nil // key not generated yet → not registered
	}
	evmAddr, err := kp.EVMAddress(pubkey)
	if err != nil {
		return false, fmt.Errorf("derive EVM address: %w", err)
	}

	client, err := ethclient.DialContext(ctx, s.besuRPCURL)
	if err != nil {
		return false, nil
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
	contractAddr := common.HexToAddress(s.registryAddress)
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &contractAddr, Data: callData}, nil)
	if err != nil {
		return false, nil
	}
	var isParticipant bool
	if err := parsedABI.UnpackIntoInterface(&isParticipant, "isParticipant", result); err != nil {
		return false, fmt.Errorf("unpack isParticipant: %w", err)
	}
	return isParticipant, nil
}

func (s *proofPossessionStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// 1. Get or generate the bank's blockchain key (private key never leaves the provider).
	pubkey, err := s.keyProvider.GetPublicKey(ctx, s.bankCode)
	if errors.Is(err, kp.ErrKeyNotFound) {
		pubkey, err = s.keyProvider.GenerateKey(ctx, s.bankCode)
	}
	if err != nil {
		return fmt.Errorf("get or generate bank key: %w", err)
	}
	evmAddr, err := kp.EVMAddress(pubkey)
	if err != nil {
		return fmt.Errorf("derive EVM address: %w", err)
	}

	client, err := ethclient.DialContext(ctx, s.besuRPCURL)
	if err != nil {
		return fmt.Errorf("dial besu: %w", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(identityRegistryABI))
	if err != nil {
		return fmt.Errorf("parse ABI: %w", err)
	}

	contractAddr := common.HexToAddress(s.registryAddress)
	addr := common.HexToAddress(evmAddr)

	// 2. Proof-of-possession nonce as zkPointer placeholder.
	var zkPointer [32]byte
	h := sha256.Sum256([]byte(evmAddr + s.bankCode))
	copy(zkPointer[:], h[:])

	callData, err := parsedABI.Pack("registerParticipant",
		addr,
		"Commercial Bank "+s.bankCode,
		RoleCommercialBank,
		zkPointer,
	)
	if err != nil {
		return fmt.Errorf("pack registerParticipant: %w", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get chainID: %w", err)
	}
	nonce, err := client.PendingNonceAt(ctx, addr)
	if err != nil {
		return fmt.Errorf("get nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("suggest gas price: %w", err)
	}

	tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), 300000, gasPrice, callData)
	signer := types.NewEIP155Signer(chainID)
	txHash := signer.Hash(tx)

	// 3. Sign via KeyProvider (proof of possession).
	sig, err := s.keyProvider.Sign(ctx, s.bankCode, txHash[:])
	if err != nil {
		return fmt.Errorf("sign transaction: %w", err)
	}
	if len(sig) == 65 && sig[64] >= 27 {
		sig[64] -= 27
	}

	signedTx, err := tx.WithSignature(signer, sig)
	if err != nil {
		return fmt.Errorf("attach signature: %w", err)
	}
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
	return nil
}
