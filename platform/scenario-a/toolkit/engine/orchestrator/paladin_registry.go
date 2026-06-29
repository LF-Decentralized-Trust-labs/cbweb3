// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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

// IMPORTANT (local-profile bootstrap keys): these are well-known Hyperledger Besu
// dev keys, pre-funded in the spoke genesis alloc (see start-besu). They are used
// ONLY for the local profile to satisfy the on-chain registration's gas + owner
// requirements, exactly as the reference network does for the CB node.
//
// The IdentityRegistry is deployed (by deploy-contracts) from registryDeployerKey,
// which is therefore the registry owner authorized to call registerIdentity.
// FR-009 (node owner key sourced from the KeyProvider) is deferred until the
// research phase (T001) resolves how a KeyProvider-managed key is funded for gas;
// it is NOT a hardcoded production key.
const (
	// registryDeployerKey — 0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73 (registry owner).
	registryDeployerKey = "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"
	// cbNodeOwnerKey — 0x627306090abaB3A6e1400e9345bC60c78a8BEf57 (CB Paladin node owner).
	cbNodeOwnerKey = "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3"
	// bankNodeOwnerKey — 0xf17f52151EbEF6C7334FAD080c5704D77216b732 (commercial-bank
	// Paladin node owner, local profile; genesis-funded). One owner key serves all
	// local bank nodes (identities are keyed by name). keyProvider-owner is deferred
	// (FR-009) until the funding model is resolved.
	bankNodeOwnerKey = "ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f"
)

// paladinNodeRegistration carries the inputs to register one Paladin node.
type paladinNodeRegistration struct {
	registry     common.Address
	nodeName     string // e.g. "spoke-brl-cb"
	grpcHostname string // e.g. "paladin-spoke-brl-cb"
	certPEM      []byte
	deployerKey  *ecdsa.PrivateKey // registry owner; sends registerIdentity
	ownerKey     *ecdsa.PrivateKey // node owner; sends setIdentityProperty
}

// registerPaladinNode registers a single Paladin node identity on-chain and
// publishes its gRPC transport endpoint + TLS cert. Parametrized per node — no
// hardcoded spoke/bank topology. Two phases (mirroring the reference flow):
//  1. registerIdentity(0, name, ownerAddr)         — sent by the registry owner
//  2. setIdentityProperty(hash, transport.grpc, …) — sent by the node owner
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

	send := func(label string, key *ecdsa.PrivateKey, nonce uint64, data []byte) (*types.Receipt, error) {
		tx := types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: big.NewInt(1_000_000_000),
			Gas:      2_000_000,
			To:       &r.registry,
			Value:    big.NewInt(0),
			Data:     data,
		})
		signed, err := types.SignTx(tx, signer, key)
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

	// Phase 1 — registerIdentity by the registry owner (deployer).
	deployerAddr := crypto.PubkeyToAddress(r.deployerKey.PublicKey)
	deployerNonce, err := client.PendingNonceAt(ctx, deployerAddr)
	if err != nil {
		return fmt.Errorf("deployer nonce: %w", err)
	}
	ownerAddr := crypto.PubkeyToAddress(r.ownerKey.PublicKey)
	var zeroHash [32]byte
	regData, err := parsedABI.Pack("registerIdentity", zeroHash, r.nodeName, ownerAddr)
	if err != nil {
		return fmt.Errorf("encode registerIdentity(%s): %w", r.nodeName, err)
	}
	receipt, err := send("registerIdentity("+r.nodeName+")", r.deployerKey, deployerNonce, regData)
	if err != nil {
		return err
	}

	var nodeHash [32]byte
	for _, lg := range receipt.Logs {
		if len(lg.Topics) == 0 || lg.Topics[0] != evt.ID {
			continue
		}
		ed, err := evt.Inputs.Unpack(lg.Data)
		if err == nil && len(ed) >= 2 {
			if h, ok := ed[1].([32]byte); ok {
				nodeHash = h
				break
			}
		}
	}

	// Phase 2 — setIdentityProperty(transport.grpc) by the node owner.
	ownerNonce, err := client.PendingNonceAt(ctx, ownerAddr)
	if err != nil {
		return fmt.Errorf("owner nonce: %w", err)
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
	if _, err := send("setIdentityProperty("+r.nodeName+")", r.ownerKey, ownerNonce, propData); err != nil {
		return err
	}
	return nil
}

// devKey decodes one of the local-profile funded dev keys above into an ECDSA key.
func devKey(hexKey string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	return crypto.ToECDSA(b)
}

// cbNodeName / cbGrpcHostname derive the central-bank Paladin node identity from
// the spoke id — parametric, never a hardcoded spoke/bank list.
func cbNodeName(spokeID string) string     { return spokeID + "-cb" }
func cbGrpcHostname(spokeID string) string { return "paladin-" + spokeID + "-cb" }

// bankNodeName / bankGrpcHostname derive a commercial-bank Paladin node identity
// from the spoke id and bank id — parametric, never a hardcoded bank list.
func bankNodeName(spokeID, bankID string) string     { return spokeID + "-" + bankID }
func bankGrpcHostname(spokeID, bankID string) string { return "paladin-" + spokeID + "-" + bankID }
