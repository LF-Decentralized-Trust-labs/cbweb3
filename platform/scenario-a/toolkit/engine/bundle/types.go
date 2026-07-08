// SPDX-License-Identifier: Apache-2.0

// Package bundle implements the TK-6 join bundle emitter for Scenario A.
// EmitBundle reads provisioning artifacts from SPOKE_DATA_DIR and writes
// a self-contained, public-safe JoinBundle YAML consumed by TK-9 (mode: join).
// The bundle never contains private key material.
package bundle

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

const (
	// APIVersion is the fixed apiVersion field for all JoinBundle documents.
	APIVersion = "cbweb3/v1"
	// Kind is the fixed kind field for all JoinBundle documents.
	Kind = "JoinBundle"
)

// BundleInput carries all inputs required by EmitBundle.
// No field may be nil or empty; EmitBundle returns ErrInvalidInput if any
// mandatory field is absent.
type BundleInput struct {
	Manifest      *manifest.Manifest // mode must be "found"
	DataDir       string             // absolute path to SPOKE_DATA_DIR
	OutputDir     string             // root where bundles/ is created; "." is valid
	EnodeProvider EnodeProvider      // never nil; use NewBesuEnodeProvider in production
	// Validators is the existing QBFT validator set to embed (mode:join needs it).
	// Typically the founding CB validator(s); rpcUrl uses the advertised host.
	Validators []ValidatorSpec
	// CBEndpoint is the central bank's credential-request URL embedded in the bundle.
	CBEndpoint string
	// GenesisVolume, if set, sources spec.genesis.content from this named Docker
	// volume's genesis.json (engine/dockervolume) instead of
	// <DataDir>/genesis/genesis.json. Production sets this to "${spokeID}_cb_genesis"
	// because genesis-init writes genesis.json directly into that volume — it never
	// touches the host filesystem (deviation from the original SPOKE_DATA_DIR
	// bind-mount design; see specs/026-tk4-compose-central-bank/plan.md addendum).
	// Left empty, EmitBundle falls back to the DataDir path — used by tests that
	// don't need Docker.
	GenesisVolume string
	// TLSVolume, if set, sources spec.trust.caCertPEM from this named Docker
	// volume's central-bank.crt (engine/dockervolume) instead of
	// <DataDir>/tls/central-bank.crt. Production sets this to "${spokeID}_cb_tls"
	// because gen-tls writes central-bank.crt directly into that volume. Left
	// empty, EmitBundle falls back to the DataDir path — used by tests that don't
	// need Docker.
	TLSVolume string
}

// JoinBundle is the top-level YAML document emitted by EmitBundle.
type JoinBundle struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   BundleMetadata `yaml:"metadata"`
	Spec       BundleSpec     `yaml:"spec"`
}

// BundleMetadata holds the bundle's identifying fields.
type BundleMetadata struct {
	Name        string `yaml:"name"`
	GeneratedAt string `yaml:"generatedAt"`
}

// BundleSpec contains all public artifacts needed by a commercial bank to join the spoke.
type BundleSpec struct {
	SpokeID    string          `yaml:"spokeId"`
	ChainID    int             `yaml:"chainId"`
	Currency   string          `yaml:"currency"`
	Bootnode   BootnodeSpec    `yaml:"bootnode"`
	Genesis    GenesisSpec     `yaml:"genesis"`
	Contracts  ContractsSpec   `yaml:"contracts"`
	Relay      *RelaySpec      `yaml:"relay,omitempty"`
	Trust      TrustSpec       `yaml:"trust"`
	Validators []ValidatorSpec `yaml:"validators"`
	// CBEndpoint is the central bank's credential-request URL. Consumed by the
	// mode:join engine to submit the commercial bank's CSR. Required for mode:join.
	CBEndpoint string `yaml:"cbEndpoint,omitempty"`
}

// ValidatorSpec identifies an existing QBFT validator in the spoke. The mode:join
// engine casts a validator vote (qbft_proposeValidatorVote) against each entry to
// promote the joining node to validator. Contains only public information.
type ValidatorSpec struct {
	Address string `yaml:"address"` // Ethereum address (0x-prefixed)
	RPCURL  string `yaml:"rpcUrl"`  // JSON-RPC HTTP endpoint of the validator
}

// BootnodeSpec identifies the Besu node that a joining bank must dial first.
// Enode always uses manifest.spec.node.advertisedHost and manifest.spec.node.p2p.port —
// never the container IP returned directly by Besu.
type BootnodeSpec struct {
	Enode          string `yaml:"enode"`
	AdvertisedHost string `yaml:"advertisedHost"`
	P2PPort        int    `yaml:"p2pPort"`
}

// GenesisSpec carries the spoke's genesis block, self-contained so TK-9 needs no
// side-channel to retrieve it.
type GenesisSpec struct {
	Hash    string `yaml:"hash"`    // "sha256:<hex-lowercase>"
	Content string `yaml:"content"` // genesis.json base64-encoded (RFC 4648, no newlines)
}

// ContractsSpec holds the seven deployed contract addresses required by a joining bank.
type ContractsSpec struct {
	RegistryAddress     string `yaml:"registryAddress"`
	ZetoFactoryAddress  string `yaml:"zetoFactoryAddress"`
	PenteFactoryAddress string `yaml:"penteFactoryAddress"`
	ZetoTokenAddress    string `yaml:"zetoTokenAddress"`
	PenteContextGroupID string `yaml:"penteContextGroupId"`
	PenteContextAddress string `yaml:"penteContextAddress"`
	FXAgreementAddress  string `yaml:"fxAgreementAddress"`
	// ParticipantRegistryAddress is the IdentityRegistry.sol participant whitelist
	// (deployed at found onboard, FR-018). Consumed by mode:join to set the
	// FXAgreement constructor's _identityRegistry (US3).
	ParticipantRegistryAddress string `yaml:"participantRegistryAddress,omitempty"`
	// FiatTokenAddress is the FiatCentralBankMoney (fCeBM) ERC-20 (deployed at found
	// by deploy-fiat-token). Consumed by mode:join to wire the bank backend's
	// FIAT_TOKEN_ADDRESS (fiat-balance / mint / burn).
	FiatTokenAddress string `yaml:"fiatTokenAddress,omitempty"`
	// HTLCAddress is the HashTimeLockedContract (deployed at found by deploy-htlc).
	// Consumed by mode:join to wire the bank backend's HTLC_ADDRESS.
	HTLCAddress string `yaml:"htlcAddress,omitempty"`
}

// RelaySpec holds the relay endpoint for the spoke.
// Omitted from the YAML when manifest.spec.relay is nil.
type RelaySpec struct {
	Endpoint string `yaml:"endpoint"`
}

// TrustSpec carries the CA certificate chain needed to verify TLS connections
// within the spoke. Contains only public certificate material — never private keys.
type TrustSpec struct {
	CACertPEM string `yaml:"caCertPEM"`
}

// EnodeProvider abstracts the source of the raw enode string.
// The default implementation calls Besu's admin_nodeInfo JSON-RPC endpoint.
// Implementations must respect ctx (timeout / cancellation).
type EnodeProvider interface {
	NodeInfo(ctx context.Context) (string, error)
}
