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

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TestDeployZetoFactory deploys the full Zeto_AnonNullifier infrastructure
// (PoseidonUnits, verifiers, SmtLib, impl, factory) and registers the
// implementation. Writes ZETO_FACTORY_ADDRESS to the spoke's .deployed-addrs.env.
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

	// Tier 1 — no library deps
	poseidon2lBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_poseidon_unit2l.yaml")
	poseidon3lBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_poseidon_unit3l.yaml")
	g16DepositBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_deposit.yaml")
	g16WnBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_withdraw_nullifier.yaml")
	g16WnBatchBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_withdraw_nullifier_batch.yaml")
	g16AntBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_anon_nullifier_transfer.yaml")
	g16AntBatchBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_g16_anon_nullifier_transfer_batch.yaml")
	factoryBytecode, _ := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_factory.yaml")

	poseidon2lAddr := deployContract("PoseidonUnit2L", poseidon2lBytecode)
	poseidon3lAddr := deployContract("PoseidonUnit3L", poseidon3lBytecode)
	g16DepositAddr := deployContract("G16_Deposit", g16DepositBytecode)
	g16WnAddr := deployContract("G16_WithdrawNullifier", g16WnBytecode)
	g16WnBatchAddr := deployContract("G16_WithdrawNullifierBatch", g16WnBatchBytecode)
	g16AntAddr := deployContract("G16_AnonNullifierTransfer", g16AntBytecode)
	g16AntBatchAddr := deployContract("G16_AnonNullifierTransferBatch", g16AntBatchBytecode)
	factoryAddr := deployContract("ZetoFactory", factoryBytecode)

	// Tier 2 — SmtLib needs Poseidon2L + Poseidon3L
	smtlibBytecode, smtlibRefs := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_smt_lib.yaml")
	smtlibLinked := linkBytecode(t, smtlibBytecode, smtlibRefs, map[string]common.Address{
		"PoseidonUnit2L": poseidon2lAddr,
		"PoseidonUnit3L": poseidon3lAddr,
	})
	smtlibAddr := deployContract("SmtLib", smtlibLinked)

	// Tier 3 — AnonNullifier impl needs Poseidon3L + SmtLib
	implBytecode, implRefs := readArtifact(t, adir+"/core_v1alpha1_smartcontractdeployment_zeto_impl_anon_nullifier.yaml")
	implLinked := linkBytecode(t, implBytecode, implRefs, map[string]common.Address{
		"PoseidonUnit3L": poseidon3lAddr,
		"SmtLib":         smtlibAddr,
	})
	implAddr := deployContract("Zeto_AnonNullifier_Impl", implLinked)

	const registerImplABIJSON = `[{"inputs":[{"internalType":"string","name":"name","type":"string"},{"components":[{"internalType":"address","name":"implementation","type":"address"},{"components":[{"internalType":"address","name":"verifier","type":"address"},{"internalType":"address","name":"depositVerifier","type":"address"},{"internalType":"address","name":"withdrawVerifier","type":"address"},{"internalType":"address","name":"lockVerifier","type":"address"},{"internalType":"address","name":"burnVerifier","type":"address"},{"internalType":"address","name":"batchVerifier","type":"address"},{"internalType":"address","name":"batchWithdrawVerifier","type":"address"},{"internalType":"address","name":"batchLockVerifier","type":"address"},{"internalType":"address","name":"batchBurnVerifier","type":"address"}],"internalType":"struct ZetoTokenFactory.Verifiers","name":"verifiers","type":"tuple"}],"internalType":"struct ZetoTokenFactory.ImplementationInfo","name":"implementation","type":"tuple"}],"name":"registerImplementation","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(registerImplABIJSON))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}

	zero := common.HexToAddress("0x0000000000000000000000000000000000000000")

	// Lock verifiers: deploy from artifacts when available, otherwise use zero.
	// The lock/transferLocked operations require on-chain Groth16 verifiers
	// compiled from the Zeto ZKP circuits. When the artifact YAML files
	// are provided, deploy them here. Until then, lock operations will revert.
	lockVerifier := zero
	batchLockVerifier := zero
	lockArtifactPath := adir + "/core_v1alpha1_smartcontractdeployment_zeto_g16_lock.yaml"
	if _, err := os.Stat(lockArtifactPath); err == nil {
		lockBytecode, _ := readArtifact(t, lockArtifactPath)
		lockVerifier = deployContract("G16_Lock", lockBytecode)
	} else {
		t.Log("WARN: G16_Lock artifact not found — LockVerifier set to zero address. " +
			"Zeto lock operations will NOT work until the artifact is provided.")
	}
	batchLockArtifactPath := adir + "/core_v1alpha1_smartcontractdeployment_zeto_g16_lock_batch.yaml"
	if _, err := os.Stat(batchLockArtifactPath); err == nil {
		batchLockBytecode, _ := readArtifact(t, batchLockArtifactPath)
		batchLockVerifier = deployContract("G16_BatchLock", batchLockBytecode)
	} else {
		t.Log("WARN: G16_BatchLock artifact not found — BatchLockVerifier set to zero address.")
	}

	calldata, err := parsedABI.Pack("registerImplementation",
		"Zeto_AnonNullifier",
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
				Verifier:              g16AntAddr,
				DepositVerifier:       g16DepositAddr,
				WithdrawVerifier:      g16WnAddr,
				LockVerifier:          lockVerifier,
				BurnVerifier:          zero,
				BatchVerifier:         g16AntBatchAddr,
				BatchWithdrawVerifier: g16WnBatchAddr,
				BatchLockVerifier:     batchLockVerifier,
				BatchBurnVerifier:     zero,
			},
		},
	)
	if err != nil {
		t.Fatalf("encode registerImplementation: %v", err)
	}
	callContract("registerImplementation(Zeto_AnonNullifier)", factoryAddr, calldata)

	fmt.Printf("\nZETO_FACTORY_ADDRESS=%s\n", factoryAddr.Hex())
	t.Logf("Zeto factory at: %s", factoryAddr.Hex())
	writeOrUpdateEnvVar(t, addrsEnvFile(), "ZETO_FACTORY_ADDRESS", factoryAddr.Hex())
}
