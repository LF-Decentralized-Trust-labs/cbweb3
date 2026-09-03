// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// operatorAddressFromHex derives the 0x-checksummed EVM address from a hex private
// key (no 0x prefix), or "" when the key is empty/invalid. Used to render
// ENTITY_BESU_ADDRESS — the wallet the escrow proxy stamps as requester_besu_address
// and whose fCeBM balance /token/fiat-balance reads.
func operatorAddressFromHex(hexKey string) string {
	if hexKey == "" {
		return ""
	}
	k, err := gethcrypto.HexToECDSA(hexKey)
	if err != nil {
		return ""
	}
	return gethcrypto.PubkeyToAddress(k.PublicKey).Hex()
}

// operatorKeyExporter is implemented by the local KeyProvider only: it exports the
// well-known public dev operator key hex so the engine can wire the backend's
// Besu-layer signing path in local. The prod provider does not implement it, so
// resolveOperatorKeyHex returns "" and the Besu path stays off (KMS wiring is FASE 4).
type operatorKeyExporter interface {
	ExportPrivateKeyHex(id string) (string, error)
}

// resolveOperatorKeyHex returns the local operator key hex when the provider is the
// local emulator, or "" otherwise (prod / export failure). Empty disables the
// backend Besu-signing path — the correct default outside local.
//
// This is the CENTRAL BANK operator (the prefunded deployer/DEFAULT_ADMIN, 0xFE3B557E).
// Commercial banks must NOT share it — see resolveBankOperatorKeyHex.
func resolveOperatorKeyHex(provider kp.KeyProvider) string {
	ex, ok := provider.(operatorKeyExporter)
	if !ok {
		return ""
	}
	hexKey, err := ex.ExportPrivateKeyHex(kp.LocalOperatorKeyID)
	if err != nil {
		return ""
	}
	return hexKey
}

// resolveBankOperatorKeyHex returns a DISTINCT per-bank Besu operator key (hex, no
// 0x) for the local profile, or "" for prod (the Besu path stays off until KMS
// wiring, matching resolveOperatorKeyHex).
//
// Every commercial bank previously reused the single shared dev operator key
// (0xFE3B557E via resolveOperatorKeyHex), so every bank's payment-orchestrator
// signed HTLC/fCeBM transactions — and appeared on-chain — as the SAME identity.
// That makes per-bank participant verification meaningless (one address stands in
// for all banks) and mirrors, on the base ledger, the Paladin nonce collision that
// fundedOperatorKey fixed for the private layer.
//
// The derived address is this bank's HTLC signer and the participant compliance
// registers on KYC approval. Derivation is deterministic (idempotent across
// redeploys, no in-memory GenerateKey state to lose) and the account needs no
// genesis prefunding: the spoke genesis sets zeroBaseFee, so gas is free.
func resolveBankOperatorKeyHex(provider kp.KeyProvider, spokeID, bankCode string) string {
	if _, ok := provider.(operatorKeyExporter); !ok {
		return "" // prod: Besu path off until KMS wiring
	}
	h := sha256.Sum256([]byte("cbweb3/besu_operator/" + spokeID + "/" + bankCode))
	return hex.EncodeToString(h[:])
}

// forgeArtifactWithABI is the subset of a Foundry build artifact needed to deploy
// a contract with constructor arguments: the creation bytecode and the ABI (used to
// encode the constructor inputs). Distinct from forgeArtifact (bytecode only), which
// deployParticipantRegistry uses for its single hand-packed address argument.
type forgeArtifactWithABI struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// encodeDeployData reads a Foundry artifact, ABI-encodes ctorArgs against the
// contract's constructor, and returns creation bytecode || encoded-args. It is
// pure (no I/O beyond reading artifactPath) so the encoding is unit-testable
// without a chain.
func encodeDeployData(artifactPath string, ctorArgs ...interface{}) ([]byte, error) {
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact %s: %w", artifactPath, err)
	}
	var art forgeArtifactWithABI
	if err := json.Unmarshal(raw, &art); err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}
	bytecode, err := hex.DecodeString(strings.TrimPrefix(art.Bytecode.Object, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode bytecode: %w", err)
	}
	if len(bytecode) == 0 {
		return nil, fmt.Errorf("empty bytecode in %s", artifactPath)
	}
	parsedABI, err := abi.JSON(bytes.NewReader(art.ABI))
	if err != nil {
		return nil, fmt.Errorf("parse ABI: %w", err)
	}
	// Pack with an empty method name encodes the constructor inputs (no selector).
	packed, err := parsedABI.Pack("", ctorArgs...)
	if err != nil {
		return nil, fmt.Errorf("pack constructor args: %w", err)
	}
	return append(bytecode, packed...), nil
}

// deployContractFromArtifact deploys the contract in artifactPath with the given
// constructor arguments, signing via the KeyProvider (no raw key material in the
// engine), and returns the deployed address. Generalizes deployParticipantRegistry
// to contracts whose constructors take dynamic types (e.g. fCeBM's name/symbol).
func deployContractFromArtifact(ctx context.Context, rpcURL, artifactPath string, provider kp.KeyProvider, signerKeyID string, ctorArgs ...interface{}) (common.Address, error) {
	data, err := encodeDeployData(artifactPath, ctorArgs...)
	if err != nil {
		return common.Address{}, err
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
			return common.Address{}, fmt.Errorf("contract deploy reverted (%s)", artifactPath)
		}
		if rec.ContractAddress == (common.Address{}) {
			return common.Address{}, fmt.Errorf("no contract address in receipt")
		}
		return rec.ContractAddress, nil
	}
	return common.Address{}, fmt.Errorf("timeout waiting for deploy receipt (%s)", artifactPath)
}
