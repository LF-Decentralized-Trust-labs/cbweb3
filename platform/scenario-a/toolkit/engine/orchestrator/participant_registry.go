// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// forgeArtifact is the subset of a Foundry build artifact we need: the creation
// bytecode under bytecode.object.
type forgeArtifact struct {
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// deployParticipantRegistry deploys contracts/src/IdentityRegistry.sol (the
// participant whitelist) from the Foundry artifact at artifactPath, with the
// constructor admin set to the deployer (which thereby holds GOVERNANCE_ROLE).
// Returns the deployed contract address.
//
// This is distinct from the Paladin node registry (REGISTRY_CONTRACT_ADDRESS):
// IdentityRegistry.sol exposes registerParticipant(onlyRole(GOVERNANCE_ROLE)),
// which onboard-registry calls to whitelist the central bank.
func deployParticipantRegistry(ctx context.Context, rpcURL, artifactPath string, provider kp.KeyProvider, signerKeyID string) (common.Address, error) {
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, fmt.Errorf("read artifact %s: %w", artifactPath, err)
	}
	var art forgeArtifact
	if err := json.Unmarshal(raw, &art); err != nil {
		return common.Address{}, fmt.Errorf("parse artifact: %w", err)
	}
	bytecode, err := hex.DecodeString(strings.TrimPrefix(art.Bytecode.Object, "0x"))
	if err != nil {
		return common.Address{}, fmt.Errorf("decode bytecode: %w", err)
	}
	if len(bytecode) == 0 {
		return common.Address{}, fmt.Errorf("empty bytecode in %s", artifactPath)
	}

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return common.Address{}, fmt.Errorf("dial besu: %w", err)
	}
	defer client.Close()

	deployerAddr, err := keyProviderAddress(ctx, provider, signerKeyID)
	if err != nil {
		return common.Address{}, err
	}
	// constructor(address admin) — admin = the operator (governance authority).
	ctorArg := common.LeftPadBytes(deployerAddr.Bytes(), 32)
	data := append(bytecode, ctorArg...)

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return common.Address{}, fmt.Errorf("chain id: %w", err)
	}
	nonce, err := client.PendingNonceAt(ctx, deployerAddr)
	if err != nil {
		return common.Address{}, fmt.Errorf("nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return common.Address{}, fmt.Errorf("gas price: %w", err)
	}

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      3_000_000,
		To:       nil, // contract creation
		Value:    big.NewInt(0),
		Data:     data,
	})
	signed, err := signTxViaKeyProvider(ctx, provider, signerKeyID, types.NewEIP155Signer(chainID), tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("sign deploy tx: %w", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		return common.Address{}, fmt.Errorf("send deploy tx: %w", err)
	}

	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return common.Address{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
		rec, err := client.TransactionReceipt(ctx, signed.Hash())
		if err != nil {
			continue
		}
		if rec.Status != 1 {
			return common.Address{}, fmt.Errorf("IdentityRegistry deploy reverted")
		}
		if rec.ContractAddress == (common.Address{}) {
			return common.Address{}, fmt.Errorf("no contract address in receipt")
		}
		return rec.ContractAddress, nil
	}
	return common.Address{}, fmt.Errorf("timeout waiting for IdentityRegistry deploy receipt")
}
