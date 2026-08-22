// SPDX-License-Identifier: Apache-2.0

package scripts_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// besuRPCURL returns the Besu JSON-RPC URL.
// Controlled by BESU_RPC_URL env var; default is the spoke-a bootnode.
func besuRPCURL() string {
	if u := os.Getenv("BESU_RPC_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:8645"
}

// addrsEnvFile returns the path to the .deployed-addrs.env for the spoke.
func addrsEnvFile() string {
	return "../" + spokeName() + "/.deployed-addrs.env"
}

// artifactsDir returns the path to the contract artifact YAML files.
func artifactsDir() string {
	if d := os.Getenv("ARTIFACTS_DIR"); d != "" {
		return d
	}
	return "../contracts"
}

// deployerPrivKeyHex is a Besu dev account pre-funded in all spoke genesis allocs.
// Address: 0x627306090abaB3A6e1400e9345bC60c78a8BEf57
const deployerPrivKeyHex = "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"

// writeOrUpdateEnvVar writes KEY=VALUE into an env file, replacing an existing
// KEY= line or appending a new one.
func writeOrUpdateEnvVar(t *testing.T, path, key, value string) {
	t.Helper()
	var lines []string
	if existing, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(existing), "\n"), "\n")
	}
	updated := false
	prefix := key + "="
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			lines[i] = prefix + value
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, prefix+value)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write env file %s: %v", path, err)
	}
	t.Logf("Written %s=%s → %s", key, value, path)
}

// TestDeployEVMRegistry deploys a fresh Paladin IdentityRegistry to the target
// spoke and writes REGISTRY_CONTRACT_ADDRESS to .deployed-addrs.env.
//
// Run:
//
//	SPOKE=spoke-a BESU_RPC_URL=http://127.0.0.1:8645 go test ./scripts/ -run TestDeployEVMRegistry -v -count=1
func TestDeployEVMRegistry(t *testing.T) {
	ctx := context.Background()

	client, err := ethclient.DialContext(ctx, besuRPCURL())
	if err != nil {
		t.Fatalf("dial Besu: %v", err)
	}
	defer client.Close()

	privKeyBytes, _ := hex.DecodeString(deployerPrivKeyHex)
	privKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		t.Fatalf("parse privkey: %v", err)
	}
	deployer := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))
	t.Logf("Deployer: %s", deployer.Hex())

	balance, _ := client.BalanceAt(ctx, deployer, nil)
	t.Logf("Balance: %s wei", balance.String())
	if balance.Sign() == 0 {
		t.Fatal("deployer has no funds — check genesis alloc")
	}

	chainID, _ := client.ChainID(ctx)
	nonce, _ := client.PendingNonceAt(ctx, deployer)

	artifactRaw, err := os.ReadFile(artifactsDir() + "/core_v1alpha1_smartcontractdeployment_registry.yaml")
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	bytecodeHex := ""
	for _, line := range strings.Split(string(artifactRaw), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "bytecode:") {
			if parts := strings.SplitN(trimmed, "0x", 2); len(parts) == 2 {
				bytecodeHex = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	if bytecodeHex == "" {
		t.Fatal("bytecode not found in artifact YAML")
	}
	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		t.Fatalf("decode bytecode: %v", err)
	}
	t.Logf("Bytecode: %d bytes", len(bytecode))

	ctorArgs, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	data := append(bytecode, ctorArgs...)

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      3_000_000,
		To:       nil,
		Value:    big.NewInt(0),
		Data:     data,
	})
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privKey)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		t.Fatalf("send tx: %v", err)
	}
	t.Logf("TX: %s", signedTx.Hash().Hex())

	var contractAddr common.Address
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		if receipt, err := client.TransactionReceipt(ctx, signedTx.Hash()); err == nil && receipt != nil {
			if receipt.Status == 0 {
				t.Fatal("tx reverted")
			}
			contractAddr = receipt.ContractAddress
			break
		}
	}
	if contractAddr == (common.Address{}) {
		t.Fatal("timeout waiting for receipt")
	}

	fmt.Printf("\nREGISTRY_CONTRACT_ADDRESS=%s\n", contractAddr.Hex())
	t.Logf("Registry deployed at: %s", contractAddr.Hex())
	writeOrUpdateEnvVar(t, addrsEnvFile(), "REGISTRY_CONTRACT_ADDRESS", contractAddr.Hex())
}
