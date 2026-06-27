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
	SpokeID   string        `yaml:"spokeId"`
	ChainID   int           `yaml:"chainId"`
	Currency  string        `yaml:"currency"`
	Bootnode  BootnodeSpec  `yaml:"bootnode"`
	Genesis   GenesisSpec   `yaml:"genesis"`
	Contracts ContractsSpec `yaml:"contracts"`
	Relay     *RelaySpec    `yaml:"relay,omitempty"`
	Trust     TrustSpec     `yaml:"trust"`
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
