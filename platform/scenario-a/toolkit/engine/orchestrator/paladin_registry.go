// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
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

// identityRegistryABIJSON is the minimal ABI of the IdentityRegistry needed to
// register a Paladin node identity and publish its gRPC transport. Mirrors the
// ABI used by the reference deploy/local/paladin/scripts (not imported, to keep
// the reference network untouched).
const identityRegistryABIJSON = `[
	{"inputs":[{"name":"parentIdentityHash","type":"bytes32"},{"name":"name","type":"string"},{"name":"owner","type":"address"}],"name":"registerIdentity","outputs":[],"stateMutability":"nonpayable","type":"function"},
	{"inputs":[{"name":"identityHash","type":"bytes32"},{"name":"name","type":"string"},{"name":"value","type":"string"}],"name":"setIdentityProperty","outputs":[],"stateMutability":"nonpayable","type":"function"},
	{"anonymous":false,"inputs":[{"name":"parentIdentityHash","type":"bytes32","indexed":false},{"name":"identityHash","type":"bytes32","indexed":false},{"name":"name","type":"string","indexed":false},{"name":"owner","type":"address","indexed":false}],"name":"IdentityRegistered","type":"event"}
]`

// paladinNodeRegistration carries the inputs to register one Paladin node.
// All signing goes through the KeyProvider (no raw key material here): the operator
// key (LocalOperatorKeyID locally; a real KMS key in prod) is the registry owner
// and the node owner, and signs both on-chain phases.
type paladinNodeRegistration struct {
	registry     common.Address
	nodeName     string // e.g. "spoke-brl-cb"
	grpcHostname string // dial host for dns:///<host>:9000 — container name (single-host) or routable advertisedHost (cross-VM); see paladinDialHost
	certPEM      []byte
	provider     kp.KeyProvider
	signerKeyID  string
}

// registerPaladinNode registers a single Paladin node identity on-chain and
// publishes its gRPC transport endpoint + TLS cert. Parametrized per node — no
// hardcoded spoke/bank topology, no raw keys. Two phases (mirroring the reference
// flow), both signed by the operator key via the KeyProvider:
//  1. registerIdentity(0, name, ownerAddr)
//  2. setIdentityProperty(hash, transport.grpc, …)
func registerPaladinNode(ctx context.Context, rpcURL string, r paladinNodeRegistration) error {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("dial besu: %w", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("chain id: %w", err)
	}
	signer := types.NewEIP155Signer(chainID)

	parsedABI, err := abi.JSON(strings.NewReader(identityRegistryABIJSON))
	if err != nil {
		return fmt.Errorf("parse ABI: %w", err)
	}
	evt := parsedABI.Events["IdentityRegistered"]

	code, err := client.CodeAt(ctx, r.registry, nil)
	if err != nil {
		return fmt.Errorf("registry code: %w", err)
	}
	if len(code) == 0 {
		return fmt.Errorf("IdentityRegistry not deployed at %s", r.registry.Hex())
	}

	operatorAddr, err := keyProviderAddress(ctx, r.provider, r.signerKeyID)
	if err != nil {
		return err
	}

	send := func(label string, nonce uint64, data []byte) (*types.Receipt, error) {
		tx := types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: big.NewInt(1_000_000_000),
			Gas:      2_000_000,
			To:       &r.registry,
			Value:    big.NewInt(0),
			Data:     data,
		})
		signed, err := signTxViaKeyProvider(ctx, r.provider, r.signerKeyID, signer, tx)
		if err != nil {
			return nil, fmt.Errorf("sign %s: %w", label, err)
		}
		if err := client.SendTransaction(ctx, signed); err != nil {
			return nil, fmt.Errorf("send %s: %w", label, err)
		}
		for i := 0; i < 60; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			rec, err := client.TransactionReceipt(ctx, signed.Hash())
			if err != nil {
				continue
			}
			if rec.Status != 1 {
				return nil, fmt.Errorf("%s reverted (status=%d)", label, rec.Status)
			}
			return rec, nil
		}
		return nil, fmt.Errorf("timeout waiting for %s receipt", label)
	}

	// Phase 1 — registerIdentity (owner = the operator address).
	// Idempotent: a prior attempt may have mined registerIdentity successfully
	// while the step still failed (e.g. interrupted before state mark, or
	// setIdentityProperty failed). On "Name already taken", recover the hash
	// from IdentityRegistered logs and continue to phase 2.
	nonce, err := client.PendingNonceAt(ctx, operatorAddr)
	if err != nil {
		return fmt.Errorf("operator nonce: %w", err)
	}
	var zeroHash [32]byte
	regData, err := parsedABI.Pack("registerIdentity", zeroHash, r.nodeName, operatorAddr)
	if err != nil {
		return fmt.Errorf("encode registerIdentity(%s): %w", r.nodeName, err)
	}

	var nodeHash [32]byte
	receipt, err := send("registerIdentity("+r.nodeName+")", nonce, regData)
	if err != nil {
		existing, lookupErr := lookupIdentityHashByName(ctx, client, r.registry, evt, r.nodeName)
		if lookupErr != nil {
			return fmt.Errorf("%w (and lookup existing identity: %v)", err, lookupErr)
		}
		nodeHash = existing
	} else {
		for _, lg := range receipt.Logs {
			if len(lg.Topics) == 0 || lg.Topics[0] != evt.ID {
				continue
			}
			ed, unpackErr := evt.Inputs.Unpack(lg.Data)
			if unpackErr == nil && len(ed) >= 2 {
				if h, ok := ed[1].([32]byte); ok {
					nodeHash = h
					break
				}
			}
		}
		if nodeHash == [32]byte{} {
			existing, lookupErr := lookupIdentityHashByName(ctx, client, r.registry, evt, r.nodeName)
			if lookupErr != nil {
				return fmt.Errorf("registerIdentity(%s) mined but identity hash not found: %w", r.nodeName, lookupErr)
			}
			nodeHash = existing
		}
	}

	// Phase 2 — setIdentityProperty(transport.grpc), signed by the same operator.
	nonce2, err := client.PendingNonceAt(ctx, operatorAddr)
	if err != nil {
		return fmt.Errorf("operator nonce (2): %w", err)
	}
	transport := struct {
		Endpoint string `json:"endpoint"`
		Issuers  string `json:"issuers"`
	}{
		Endpoint: fmt.Sprintf("dns:///%s:9000", r.grpcHostname),
		Issuers:  string(r.certPEM),
	}
	transportJSON, err := json.Marshal(&transport)
	if err != nil {
		return fmt.Errorf("marshal transport: %w", err)
	}
	propData, err := parsedABI.Pack("setIdentityProperty", nodeHash, "transport.grpc", string(transportJSON))
	if err != nil {
		return fmt.Errorf("encode setIdentityProperty(%s): %w", r.nodeName, err)
	}
	if _, err := send("setIdentityProperty("+r.nodeName+")", nonce2, propData); err != nil {
		return err
	}
	return nil
}

// lookupIdentityHashByName scans IdentityRegistered logs for nodeName and
// returns the most recent identityHash. Used to resume after a partial
// registerIdentity (name already taken on-chain).
func lookupIdentityHashByName(ctx context.Context, client *ethclient.Client, registry common.Address, evt abi.Event, nodeName string) ([32]byte, error) {
	var zero [32]byte
	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{registry},
		Topics:    [][]common.Hash{{evt.ID}},
	})
	if err != nil {
		return zero, fmt.Errorf("filter IdentityRegistered: %w", err)
	}
	var found [32]byte
	ok := false
	for _, lg := range logs {
		ed, unpackErr := evt.Inputs.Unpack(lg.Data)
		if unpackErr != nil || len(ed) < 3 {
			continue
		}
		name, _ := ed[2].(string)
		if name != nodeName {
			continue
		}
		h, hashOK := ed[1].([32]byte)
		if !hashOK {
			continue
		}
		found = h
		ok = true
	}
	if !ok {
		return zero, fmt.Errorf("no IdentityRegistered event for %q", nodeName)
	}
	return found, nil
}

// cbNodeName / cbGrpcHostname derive the central-bank Paladin node identity from
// the spoke id — parametric, never a hardcoded spoke/bank list.
func cbNodeName(spokeID string) string     { return spokeID + "-cb" }
func cbGrpcHostname(spokeID string) string { return "paladin-" + spokeID + "-cb" }

// bankNodeName / bankGrpcHostname derive a commercial-bank Paladin node identity
// from the spoke id and bank id — parametric, never a hardcoded bank list.
func bankNodeName(spokeID, bankID string) string     { return spokeID + "-" + bankID }
func bankGrpcHostname(spokeID, bankID string) string { return "paladin-" + spokeID + "-" + bankID }
