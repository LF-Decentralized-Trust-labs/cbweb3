// SPDX-License-Identifier: Apache-2.0

package scripts_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// penteFactoryArtifact holds the ABI+bytecode for the PenteFactory contract,
// extracted from pente.jar (contracts/domains/pente/PenteFactory.sol/PenteFactory.json).
type penteFactoryArtifact struct {
	ABI      []interface{} `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// TestDeployPenteFactory deploys the PenteFactory contract on-chain and writes
// PENTE_FACTORY_ADDRESS to the spoke's .deployed-addrs.env.
//
// The PenteFactory contract is extracted from pente.jar and stored at:
//
//	deploy/local/paladin/contracts/PenteFactory.json
//
// Run:
//
//	SPOKE=spoke-a BESU_RPC_URL=http://127.0.0.1:8645 go test ./... -run TestDeployPenteFactory -v -count=1
func TestDeployPenteFactory(t *testing.T) {
	ctx := context.Background()

	client, err := ethclient.DialContext(ctx, besuRPCURL())
	if err != nil {
		t.Fatalf("dial besu: %v", err)
	}
	chainID, _ := client.ChainID(ctx)

	privKeyBytes, _ := hex.DecodeString(deployerPrivKeyHex)
	privKey, _ := crypto.ToECDSA(privKeyBytes)
	deployer := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))
	signer := types.NewEIP155Signer(chainID)

	t.Logf("Deployer: %s", deployer.Hex())

	nonce, err := client.PendingNonceAt(ctx, deployer)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}

	// Load PenteFactory artifact (bytecode extracted from pente.jar).
	// Path relative to this file: ../contracts/PenteFactory.json
	artifactPath := artifactsDir() + "/PenteFactory.json"
	absPath, err := filepath.Abs(artifactPath)
	if err != nil {
		t.Fatalf("resolve artifact path: %v", err)
	}
	t.Logf("Loading PenteFactory artifact from %s...", absPath)

	rawJSON, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read PenteFactory artifact: %v", err)
	}

	var artifact penteFactoryArtifact
	if err := json.Unmarshal(rawJSON, &artifact); err != nil {
		t.Fatalf("parse PenteFactory artifact: %v", err)
	}

	bytecodeHex := artifact.Bytecode.Object
	if bytecodeHex == "" || bytecodeHex == "0x" {
		t.Fatalf("PenteFactory bytecode is empty")
	}

	// Strip 0x prefix for decoding.
	bytecodeTrimmed := bytecodeHex
	if len(bytecodeTrimmed) > 2 && bytecodeTrimmed[:2] == "0x" {
		bytecodeTrimmed = bytecodeTrimmed[2:]
	}
	bytecode, err := hex.DecodeString(bytecodeTrimmed)
	if err != nil {
		t.Fatalf("decode bytecode hex: %v", err)
	}

	t.Logf("PenteFactory bytecode: %d bytes", len(bytecode))

	// Deploy PenteFactory via Besu directly (no constructor arguments needed).
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      4_500_000,
		To:       nil,
		Value:    big.NewInt(0),
		Data:     bytecode,
	})
	signed, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	if err := client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("send tx: %v", err)
	}
	hash := signed.Hash()
	t.Logf("PenteFactory deploy tx sent: %s", hash.Hex())

	var factoryAddr common.Address
	for i := 0; i < 60; i++ {
		time.Sleep(2 * time.Second)
		r, err := client.TransactionReceipt(ctx, hash)
		if err != nil {
			continue
		}
		if r.Status != 1 {
			t.Fatalf("PenteFactory deploy failed (status=%d)", r.Status)
		}
		factoryAddr = r.ContractAddress
		break
	}
	if factoryAddr == (common.Address{}) {
		t.Fatalf("timeout waiting for PenteFactory deploy receipt")
	}

	t.Logf("✓ PenteFactory deployed at %s", factoryAddr.Hex())
	fmt.Printf("\nPENTE_FACTORY_ADDRESS=%s\n", factoryAddr.Hex())

	writeOrUpdateEnvVar(t, addrsEnvFile(), "PENTE_FACTORY_ADDRESS", factoryAddr.Hex())
}
