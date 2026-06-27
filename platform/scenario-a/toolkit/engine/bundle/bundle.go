// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/addrs"
	"gopkg.in/yaml.v3"
)

// EmitBundle reads provisioning artifacts from in.DataDir and writes
// <in.OutputDir>/bundles/<spoke-id>.bundle.yaml atomically.
// Returns the populated JoinBundle on success.
//
// Preconditions (validated before any I/O beyond manifest inspection):
//   - manifest.Spec.Mode == "found"
//   - manifest.Spec.Node.P2P.Port > 0
//   - <dataDir>/genesis/genesis.json exists
//   - <dataDir>/.deployed-addrs.env contains all 7 required addresses
//   - <dataDir>/tls/central-bank.crt contains a valid PEM CERTIFICATE block
//   - in.EnodeProvider.NodeInfo returns a well-formed enode string
func EmitBundle(ctx context.Context, in BundleInput) (*JoinBundle, error) {
	if in.Manifest == nil {
		return nil, fmt.Errorf("%w: Manifest is nil", ErrInvalidInput)
	}
	if in.DataDir == "" {
		return nil, fmt.Errorf("%w: DataDir is empty", ErrInvalidInput)
	}
	if in.EnodeProvider == nil {
		return nil, fmt.Errorf("%w: EnodeProvider is nil", ErrInvalidInput)
	}
	if in.Manifest.Spec.Node.P2P == nil {
		return nil, fmt.Errorf("%w: spec.node.p2p is nil", ErrInvalidInput)
	}
	if in.Manifest.Spec.Node.P2P.Port <= 0 {
		return nil, fmt.Errorf("%w: spec.node.p2p.port must be > 0", ErrInvalidInput)
	}
	if in.Manifest.Spec.Mode != "found" {
		return nil, fmt.Errorf("%w, got: %s", ErrInvalidMode, in.Manifest.Spec.Mode)
	}

	spokeID := in.Manifest.Spec.Spoke.ID
	slog.InfoContext(ctx, "bundle: emitting", "spoke_id", spokeID)

	genesis, err := readGenesis(ctx, in.DataDir)
	if err != nil {
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, err
	}

	trust, err := readCACert(in.DataDir)
	if err != nil {
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, err
	}

	contracts, err := readContracts(in.DataDir)
	if err != nil {
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, err
	}

	rawEnode, err := in.EnodeProvider.NodeInfo(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrEnodeUnavailable, err)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	enode, err := parseAndRewriteEnode(
		rawEnode,
		in.Manifest.Spec.Node.AdvertisedHost,
		in.Manifest.Spec.Node.P2P.Port,
	)
	if err != nil {
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, err
	}

	spec := BundleSpec{
		SpokeID:  spokeID,
		ChainID:  in.Manifest.Spec.Spoke.ChainID,
		Currency: in.Manifest.Spec.Spoke.Currency,
		Bootnode: BootnodeSpec{
			Enode:          enode,
			AdvertisedHost: in.Manifest.Spec.Node.AdvertisedHost,
			P2PPort:        in.Manifest.Spec.Node.P2P.Port,
		},
		Genesis:   genesis,
		Contracts: contracts,
		Trust:     trust,
	}
	if in.Manifest.Spec.Relay != nil {
		spec.Relay = &RelaySpec{Endpoint: in.Manifest.Spec.Relay.Endpoint}
	}

	bundle := &JoinBundle{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: BundleMetadata{
			Name:        spokeID,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Spec: spec,
	}

	if err := writeBundleAtomic(in.OutputDir, spokeID, bundle); err != nil {
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, err
	}

	bundlePath := filepath.Join(in.OutputDir, "bundles", spokeID+".bundle.yaml")
	slog.InfoContext(ctx, "bundle: written", "spoke_id", spokeID, "path", bundlePath)
	return bundle, nil
}

// readGenesis reads genesis/genesis.json from dataDir, computes its SHA-256 hash,
// and base64-encodes its content (RFC 4648, no padding newlines).
func readGenesis(ctx context.Context, dataDir string) (GenesisSpec, error) {
	genesisPath := filepath.Join(dataDir, "genesis", "genesis.json")
	genesisBytes, err := os.ReadFile(genesisPath)
	if err != nil {
		if os.IsNotExist(err) {
			return GenesisSpec{}, ErrGenesisNotFound
		}
		return GenesisSpec{}, fmt.Errorf("read genesis: %w", err)
	}
	if len(genesisBytes) > 1<<20 {
		slog.WarnContext(ctx, "bundle: genesis exceeds 1 MB", "bytes", len(genesisBytes))
	}
	sum := sha256.Sum256(genesisBytes)
	return GenesisSpec{
		Hash:    fmt.Sprintf("sha256:%x", sum),
		Content: base64.StdEncoding.EncodeToString(genesisBytes),
	}, nil
}

// readCACert reads tls/central-bank.crt from dataDir, validates it contains a
// PEM CERTIFICATE block, and rejects any file that contains private key material.
func readCACert(dataDir string) (TrustSpec, error) {
	certPath := filepath.Join(dataDir, "tls", "central-bank.crt")
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return TrustSpec{}, fmt.Errorf("%w: file not found at %s", ErrCACertNotFound, certPath)
		}
		return TrustSpec{}, fmt.Errorf("read CA cert: %w", err)
	}

	content := string(certBytes)
	if strings.Contains(content, "PRIVATE KEY") {
		return TrustSpec{}, fmt.Errorf("%w: private key material detected", ErrCACertNotFound)
	}

	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return TrustSpec{}, fmt.Errorf("%w: no valid PEM CERTIFICATE block found", ErrCACertNotFound)
	}

	return TrustSpec{CACertPEM: content}, nil
}

// readContracts reads .deployed-addrs.env from dataDir and validates all 7 required
// contract addresses are present and non-empty.
func readContracts(dataDir string) (ContractsSpec, error) {
	envPath := filepath.Join(dataDir, ".deployed-addrs.env")
	a, err := addrs.ParseDeployedAddrs(envPath)
	if err != nil {
		return ContractsSpec{}, fmt.Errorf("%w: %v", ErrDeployedAddrsIncomplete, err)
	}

	required := []struct {
		name string
		val  string
	}{
		{"REGISTRY_CONTRACT_ADDRESS", a.RegistryContractAddress},
		{"ZETO_FACTORY_ADDRESS", a.ZetoFactoryAddress},
		{"PENTE_FACTORY_ADDRESS", a.PenteFactoryAddress},
		{"ZETO_TOKEN_ADDRESS", a.ZetoTokenAddress},
		{"PENTE_CONTEXT_GROUP_ID", a.PenteContextGroupID},
		{"PENTE_CONTEXT_ADDRESS", a.PenteContextAddress},
		{"FX_AGREEMENT_DEPLOYED_AT", a.FXAgreementDeployedAt},
	}
	for _, r := range required {
		if r.val == "" {
			return ContractsSpec{}, fmt.Errorf("%w: %s", ErrDeployedAddrsIncomplete, r.name)
		}
	}

	return ContractsSpec{
		RegistryAddress:     a.RegistryContractAddress,
		ZetoFactoryAddress:  a.ZetoFactoryAddress,
		PenteFactoryAddress: a.PenteFactoryAddress,
		ZetoTokenAddress:    a.ZetoTokenAddress,
		PenteContextGroupID: a.PenteContextGroupID,
		PenteContextAddress: a.PenteContextAddress,
		FXAgreementAddress:  a.FXAgreementDeployedAt,
	}, nil
}

// writeBundleAtomic serializes bundle to YAML and writes it atomically via temp+rename.
func writeBundleAtomic(outputDir, spokeID string, bundle *JoinBundle) error {
	bundleDir := filepath.Join(outputDir, "bundles")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return fmt.Errorf("create bundles dir: %w", err)
	}

	data, err := yaml.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}

	tmp, err := os.CreateTemp(bundleDir, ".bundle-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("create temp bundle: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp bundle: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp bundle: %w", err)
	}

	bundlePath := filepath.Join(bundleDir, spokeID+".bundle.yaml")
	if err := os.Rename(tmpName, bundlePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename bundle: %w", err)
	}
	return nil
}
