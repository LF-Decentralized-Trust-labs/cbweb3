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
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const identityRegistryABIJSON = `[
	{
		"inputs":[
			{"name":"parentIdentityHash","type":"bytes32"},
			{"name":"name","type":"string"},
			{"name":"owner","type":"address"}
		],
		"name":"registerIdentity",
		"outputs":[],
		"stateMutability":"nonpayable",
		"type":"function"
	},
	{
		"inputs":[
			{"name":"identityHash","type":"bytes32"},
			{"name":"name","type":"string"},
			{"name":"value","type":"string"}
		],
		"name":"setIdentityProperty",
		"outputs":[],
		"stateMutability":"nonpayable",
		"type":"function"
	},
	{
		"inputs":[
			{"name":"parentIdentityHash","type":"bytes32","indexed":false},
			{"name":"identityHash","type":"bytes32","indexed":false},
			{"name":"name","type":"string","indexed":false},
			{"name":"owner","type":"address","indexed":false}
		],
		"name":"IdentityRegistered",
		"type":"event"
	}
]`

// nodeConfig holds per-node data for Paladin node registration.
type nodeConfig struct {
	Name     string
	Hostname string // Docker container hostname for gRPC endpoint
	OwnerKey string // funded_operator private key (hex)
	CertDir  string // relative path to config dir containing tls.crt
}

// spokeNodes returns the node configs for the target spoke.
func spokeNodes() []nodeConfig {
	s := spokeName()
	switch s {
	case "spoke-b":
		return []nodeConfig{
			{Name: "spoke-b-cb", Hostname: "paladin-spoke-b-cb", OwnerKey: "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", CertDir: "../spoke-b/config/central-bank"},
			{Name: "spoke-b-bank-b", Hostname: "paladin-spoke-b-bank-b", OwnerKey: "ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f", CertDir: "../spoke-b/config/bank-b"},
			{Name: "spoke-b-bank-d", Hostname: "paladin-spoke-b-bank-d", OwnerKey: "5b02fc9b19facc6de72b514a03db7f7fbd9f46b6ad6ffb24b8df575593ae87f1", CertDir: "../spoke-b/config/bank-d"},
		}
	default: // spoke-a
		return []nodeConfig{
			{Name: "spoke-a-cb", Hostname: "paladin-spoke-a-cb", OwnerKey: "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", CertDir: "../spoke-a/config/central-bank"},
			{Name: "spoke-a-bank-a", Hostname: "paladin-spoke-a-bank-a", OwnerKey: "ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f", CertDir: "../spoke-a/config/bank-a"},
			{Name: "spoke-a-bank-c", Hostname: "paladin-spoke-a-bank-c", OwnerKey: "5b02fc9b19facc6de72b514a03db7f7fbd9f46b6ad6ffb24b8df575593ae87f1", CertDir: "../spoke-a/config/bank-c"},
		}
	}
}

// registryAddressFromEnv reads REGISTRY_CONTRACT_ADDRESS from the spoke env file.
func registryAddressFromEnv(t *testing.T) common.Address {
	t.Helper()
	data, err := os.ReadFile(addrsEnvFile())
	if err != nil {
		t.Fatalf("read %s: %v (run TestDeployEVMRegistry first)", addrsEnvFile(), err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "REGISTRY_CONTRACT_ADDRESS=") {
			addr := strings.TrimSpace(strings.TrimPrefix(line, "REGISTRY_CONTRACT_ADDRESS="))
			if addr != "" {
				return common.HexToAddress(addr)
			}
		}
	}
	t.Fatalf("REGISTRY_CONTRACT_ADDRESS not found in %s", addrsEnvFile())
	return common.Address{}
}

// TestRegisterPaladinNodes registers all spoke Paladin nodes in the on-chain
// IdentityRegistry and publishes their gRPC transport endpoints.
//
// Run:
//
//	SPOKE=spoke-a BESU_RPC_URL=http://127.0.0.1:8645 go test ./scripts/ -run TestRegisterPaladinNodes -v -count=1
func TestRegisterPaladinNodes(t *testing.T) {
	ctx := context.Background()

	client, err := ethclient.DialContext(ctx, besuRPCURL())
	if err != nil {
		t.Fatalf("dial besu: %v", err)
	}
	defer client.Close()

	chainID, _ := client.ChainID(ctx)
	signer := types.NewEIP155Signer(chainID)

	parsedABI, err := abi.JSON(strings.NewReader(identityRegistryABIJSON))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}

	registry := registryAddressFromEnv(t)
	t.Logf("IdentityRegistry: %s", registry.Hex())

	// Validate registry contract exists by checking bytecode
	code, err := client.CodeAt(ctx, registry, nil)
	if err != nil {
		t.Fatalf("failed to get contract code: %v", err)
	}
	if len(code) == 0 {
		t.Fatal("registry contract not deployed at specified address")
	}

	identityRegisteredEvent := parsedABI.Events["IdentityRegistered"]

	sendTx := func(label string, privKey *ecdsa.PrivateKey, nonce *uint64, to common.Address, calldata []byte) *types.Receipt {
		t.Helper()
		tx := types.NewTx(&types.LegacyTx{
			Nonce:    *nonce,
			GasPrice: big.NewInt(1_000_000_000),
			Gas:      2_000_000,
			To:       &to,
			Value:    big.NewInt(0),
			Data:     calldata,
		})
		signed, err := types.SignTx(tx, signer, privKey)
		if err != nil {
			t.Fatalf("sign(%s): %v", label, err)
		}
		if err := client.SendTransaction(ctx, signed); err != nil {
			t.Fatalf("send(%s): %v", label, err)
		}
		*nonce++
		hash := signed.Hash()
		for i := 0; i < 60; i++ {
			time.Sleep(2 * time.Second)
			r, err := client.TransactionReceipt(ctx, hash)
			if err != nil {
				continue
			}
			if r.Status != 1 {
				t.Fatalf("%s reverted (status=%d)", label, r.Status)
			}
			return r
		}
		t.Fatalf("timeout waiting for %s receipt", label)
		return nil
	}

	deployerKeyBytes, _ := hex.DecodeString(deployerPrivKeyHex)
	deployerKey, _ := crypto.ToECDSA(deployerKeyBytes)
	deployerAddr := crypto.PubkeyToAddress(*deployerKey.Public().(*ecdsa.PublicKey))
	deployerNonce, _ := client.PendingNonceAt(ctx, deployerAddr)

	nodes := spokeNodes()
	nodeHashes := make(map[string][32]byte)
	var zeroHash [32]byte

	// Step 1 — register each node identity
	for _, node := range nodes {
		ownerKeyBytes, _ := hex.DecodeString(node.OwnerKey)
		ownerKey, _ := crypto.ToECDSA(ownerKeyBytes)
		ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))

		calldata, err := parsedABI.Pack("registerIdentity", zeroHash, node.Name, ownerAddr)
		if err != nil {
			t.Fatalf("encode registerIdentity(%s): %v", node.Name, err)
		}
		receipt := sendTx(fmt.Sprintf("registerIdentity(%s)", node.Name), deployerKey, &deployerNonce, registry, calldata)

		var nodeHash [32]byte
		for _, log := range receipt.Logs {
			if log.Topics[0] == identityRegisteredEvent.ID {
				ed, err := identityRegisteredEvent.Inputs.Unpack(log.Data)
				if err != nil {
					continue
				}
				nodeHash = ed[1].([32]byte)
				break
			}
		}
		nodeHashes[node.Name] = nodeHash
		t.Logf("registered '%s' owner=%s hash=0x%x", node.Name, ownerAddr.Hex(), nodeHash)
	}

	// Step 2 — each owner publishes gRPC transport endpoint + TLS cert
	for _, node := range nodes {
		ownerKeyBytes, _ := hex.DecodeString(node.OwnerKey)
		ownerKey, _ := crypto.ToECDSA(ownerKeyBytes)
		ownerAddr := crypto.PubkeyToAddress(*ownerKey.Public().(*ecdsa.PublicKey))
		ownerNonce, _ := client.PendingNonceAt(ctx, ownerAddr)

		certPEM, err := os.ReadFile(node.CertDir + "/tls.crt")
		if err != nil {
			t.Fatalf("read cert for %s at %s/tls.crt: %v", node.Name, node.CertDir, err)
		}

		transportDetails := struct {
			Endpoint string `json:"endpoint"`
			Issuers  string `json:"issuers"`
		}{
			Endpoint: fmt.Sprintf("dns:///%s:9000", node.Hostname),
			Issuers:  string(certPEM),
		}
		transportJSON, _ := json.Marshal(&transportDetails)

		nodeHash := nodeHashes[node.Name]
		calldata, err := parsedABI.Pack("setIdentityProperty", nodeHash, "transport.grpc", string(transportJSON))
		if err != nil {
			t.Fatalf("encode setIdentityProperty(%s): %v", node.Name, err)
		}
		sendTx(fmt.Sprintf("setTransport(%s)", node.Name), ownerKey, &ownerNonce, registry, calldata)
		t.Logf("published transport for '%s': endpoint=dns:///%s:9000, cert=%d bytes", node.Name, node.Hostname, len(certPEM))
	}

	t.Logf("All %d Paladin nodes registered for %s", len(nodes), spokeName())
}
