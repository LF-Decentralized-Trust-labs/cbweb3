// SPDX-License-Identifier: Apache-2.0

package scripts_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TestDeployZetoFactory deploys the Zeto_Anon infrastructure
// (verifiers, impl, factory) and registers the implementation.
// Writes ZETO_FACTORY_ADDRESS to the spoke's .deployed-addrs.env.
//
// Run:
//
//	SPOKE=spoke-a BESU_RPC_URL=http://127.0.0.1:8645 go test ./scripts/ -run TestDeployZetoFactory -v -count=1
func TestDeployZetoFactory(t *testing.T) {
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

	nonce, err := client.PendingNonceAt(ctx, deployer)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}

	adir := artifactsDir()

	deployContract := func(name string, data []byte) common.Address {
		t.Helper()
		tx := types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: big.NewInt(1_000_000_000),
			Gas:      4_500_000,
			To:       nil,
			Value:    big.NewInt(0),
			Data:     data,
		})
		signed, err := types.SignTx(tx, signer, privKey)
		if err != nil {
			t.Fatalf("sign(%s): %v", name, err)
		}
		if err := client.SendTransaction(ctx, signed); err != nil {
			t.Fatalf("send(%s): %v", name, err)
		}
		hash := signed.Hash()
		nonce++
		var addr common.Address
		for i := 0; i < 60; i++ {
			time.Sleep(2 * time.Second)
			r, err := client.TransactionReceipt(ctx, hash)
			if err != nil {
				continue
			}
			if r.Status != 1 {
				t.Fatalf("deploy(%s) failed (status=%d)", name, r.Status)
			}
			addr = r.ContractAddress
			break
		}
		if addr == (common.Address{}) {
			t.Fatalf("timeout waiting for receipt of %s", name)
		}
		t.Logf("deployed %s at %s", name, addr.Hex())
		return addr
	}

	callContract := func(name string, to common.Address, calldata []byte) {
		t.Helper()
		tx := types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: big.NewInt(1_000_000_000),
			Gas:      2_000_000,
			To:       &to,
			Value:    big.NewInt(0),
			Data:     calldata,
		})
		signed, err := types.SignTx(tx, signer, privKey)
		if err != nil {
			t.Fatalf("sign(%s): %v", name, err)
		}
		if err := client.SendTransaction(ctx, signed); err != nil {
			t.Fatalf("send(%s): %v", name, err)
		}
		hash := signed.Hash()
		nonce++
		for i := 0; i < 60; i++ {
			time.Sleep(2 * time.Second)
			r, err := client.TransactionReceipt(ctx, hash)
			if err != nil {
				continue
			}
			if r.Status != 1 {
				t.Fatalf("call(%s) failed (status=%d)", name, r.Status)
			}
			t.Logf("called %s OK", name)
			return
		}
		t.Fatalf("timeout waiting for receipt of %s call", name)
	}

	// Deploy verifiers + factory (no library deps needed for Zeto_Anon)
	g16DepositBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_deposit.yaml")
	g16WithdrawBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_withdraw.yaml")
	g16WithdrawBatchBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_withdraw_batch.yaml")
	g16AnonBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_anon.yaml")
	g16AnonBatchBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_anon_batch.yaml")
	factoryBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_factory.yaml")

	g16DepositAddr := deployContract("G16_Deposit", g16DepositBytecode)
	g16WithdrawAddr := deployContract("G16_Withdraw", g16WithdrawBytecode)
	g16WithdrawBatchAddr := deployContract("G16_WithdrawBatch", g16WithdrawBatchBytecode)
	g16AnonAddr := deployContract("G16_Anon", g16AnonBytecode)
	g16AnonBatchAddr := deployContract("G16_AnonBatch", g16AnonBatchBytecode)
	factoryAddr := deployContract("ZetoFactory", factoryBytecode)

	// Zeto_Anon impl — no library linking required
	implBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_impl_anon.yaml")
	implAddr := deployContract("Zeto_Anon_Impl", implBytecode)

	const registerImplABIJSON = `[{"inputs":[{"internalType":"string","name":"name","type":"string"},{"components":[{"internalType":"address","name":"implementation","type":"address"},{"components":[{"internalType":"address","name":"verifier","type":"address"},{"internalType":"address","name":"depositVerifier","type":"address"},{"internalType":"address","name":"withdrawVerifier","type":"address"},{"internalType":"address","name":"lockVerifier","type":"address"},{"internalType":"address","name":"burnVerifier","type":"address"},{"internalType":"address","name":"batchVerifier","type":"address"},{"internalType":"address","name":"batchWithdrawVerifier","type":"address"},{"internalType":"address","name":"batchLockVerifier","type":"address"},{"internalType":"address","name":"batchBurnVerifier","type":"address"}],"internalType":"struct ZetoTokenFactory.Verifiers","name":"verifiers","type":"tuple"}],"internalType":"struct ZetoTokenFactory.ImplementationInfo","name":"implementation","type":"tuple"}],"name":"registerImplementation","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(registerImplABIJSON))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}

	zero := common.HexToAddress("0x0000000000000000000000000000000000000000")

	calldata, err := parsedABI.Pack("registerImplementation",
		"Zeto_Anon",
		struct {
			Implementation common.Address
			Verifiers      struct {
				Verifier              common.Address
				DepositVerifier       common.Address
				WithdrawVerifier      common.Address
				LockVerifier          common.Address
				BurnVerifier          common.Address
				BatchVerifier         common.Address
				BatchWithdrawVerifier common.Address
				BatchLockVerifier     common.Address
				BatchBurnVerifier     common.Address
			}
		}{
			Implementation: implAddr,
			Verifiers: struct {
				Verifier              common.Address
				DepositVerifier       common.Address
				WithdrawVerifier      common.Address
				LockVerifier          common.Address
				BurnVerifier          common.Address
				BatchVerifier         common.Address
				BatchWithdrawVerifier common.Address
				BatchLockVerifier     common.Address
				BatchBurnVerifier     common.Address
			}{
				Verifier:              g16AnonAddr,
				DepositVerifier:       g16DepositAddr,
				WithdrawVerifier:      g16WithdrawAddr,
				LockVerifier:          zero,
				BurnVerifier:          zero,
				BatchVerifier:         g16AnonBatchAddr,
				BatchWithdrawVerifier: g16WithdrawBatchAddr,
				BatchLockVerifier:     zero,
				BatchBurnVerifier:     zero,
			},
		},
	)
	if err != nil {
		t.Fatalf("encode registerImplementation: %v", err)
	}
	callContract("registerImplementation(Zeto_Anon)", factoryAddr, calldata)

	fmt.Printf("\nZETO_FACTORY_ADDRESS=%s\n", factoryAddr.Hex())
	t.Logf("Zeto factory at: %s", factoryAddr.Hex())
	writeOrUpdateEnvVar(t, addrsEnvFile(), "ZETO_FACTORY_ADDRESS", factoryAddr.Hex())
}
