// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/dockervolume"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"gopkg.in/yaml.v3"
)

// EmitBundle reads provisioning artifacts from in.DataDir and writes
// <in.OutputDir>/bundles/<spoke-id>.bundle.yaml atomically.
// Returns the populated JoinBundle on success.
//
// Preconditions (validated before any I/O beyond manifest inspection):
//   - manifest.Spec.Mode == "found"
//   - manifest.Spec.Node.P2P.Port > 0
//   - genesis.json exists at <dataDir>/genesis/genesis.json, or (production) in
//     the GenesisVolume named Docker volume when GenesisVolume is set
//   - <dataDir>/.deployed-addrs.env contains all 7 required addresses
//   - central-bank.crt (at <dataDir>/tls/central-bank.crt, or in the TLSVolume
//     named Docker volume when TLSVolume is set) contains a valid PEM
//     CERTIFICATE block
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

	genesis, err := readGenesis(ctx, in.DataDir, in.GenesisVolume)
	if err != nil {
		slog.ErrorContext(ctx, "bundle: failed", "spoke_id", spokeID, "error", err)
		return nil, err
	}

	trust, err := readCACert(ctx, in.DataDir, in.TLSVolume)
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
		Genesis:    genesis,
		Contracts:  contracts,
		Trust:      trust,
		Validators: in.Validators,
		CBEndpoint: in.CBEndpoint,
	}
	if in.Manifest.Spec.Relay != nil {
		spec.Relay = &RelaySpec{Endpoint: in.Manifest.Spec.Relay.Endpoint}
	}

	// When no validators are supplied, derive the founding CB validator from its
	// own enode. This is correct by construction for mode:found: the founding
	// central bank creates the spoke and is its sole initial QBFT validator;
	// additional validators are added later via mode:join, not at founding time.
	// Callers that already know the live validator set (e.g. re-emitting a bundle
	// for a grown spoke) should pass in.Validators explicitly.
	// NOTE: rpcURL uses the P2P-advertised host. In the local (DOCKER) profile the
	// advertised host also serves JSON-RPC; a prod deployment that binds RPC to a
	// different host/port must pass in.Validators with the correct rpcUrl.
	if len(spec.Validators) == 0 {
		if vAddr, derr := EnodeToValidatorAddress(enode); derr == nil {
			rpcURL := fmt.Sprintf("http://%s:%d", in.Manifest.Spec.Node.AdvertisedHost, rpcPortOf(in.Manifest))
			spec.Validators = []ValidatorSpec{{Address: vAddr, RPCURL: rpcURL}}
		} else {
			// Leave validators empty; the apply found-path validates the emitted
			// bundle with ValidateForJoin and fails fast if it is unusable, so the
			// fault surfaces at emission rather than at a later join.
			slog.WarnContext(ctx, "bundle: could not derive founding validator from enode", "spoke_id", spokeID, "error", derr)
		}
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

// rpcPortOf returns the manifest's RPC port, or 8545 if unset.
func rpcPortOf(m *manifest.Manifest) int {
	if m.Spec.Node.RPC != nil && m.Spec.Node.RPC.Port > 0 {
		return m.Spec.Node.RPC.Port
	}
	return 8545
}

// readGenesis reads genesis.json, computes its SHA-256 hash, and base64-encodes
// its content (RFC 4648, no padding newlines). If genesisVolume is set, it reads
// from that named Docker volume (engine/dockervolume) instead of
// <dataDir>/genesis/genesis.json — see the GenesisVolume field doc on BundleInput.
func readGenesis(ctx context.Context, dataDir, genesisVolume string) (GenesisSpec, error) {
	var genesisBytes []byte
	if genesisVolume != "" {
		b, err := dockervolume.ReadFile(ctx, genesisVolume, "genesis.json")
		if err != nil {
			if errors.Is(err, dockervolume.ErrNotFound) {
				return GenesisSpec{}, ErrGenesisNotFound
			}
			return GenesisSpec{}, fmt.Errorf("read genesis from volume %s: %w", genesisVolume, err)
		}
		genesisBytes = b
	} else {
		genesisPath := filepath.Join(dataDir, "genesis", "genesis.json")
		b, err := os.ReadFile(genesisPath)
		if err != nil {
			if os.IsNotExist(err) {
				return GenesisSpec{}, ErrGenesisNotFound
			}
			return GenesisSpec{}, fmt.Errorf("read genesis: %w", err)
		}
		genesisBytes = b
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

// readCACert reads central-bank.crt, validates it contains a PEM CERTIFICATE
// block, and rejects any file that contains private key material. If
// tlsVolume is set, it reads from that named Docker volume (engine/dockervolume)
// instead of <dataDir>/tls/central-bank.crt — see the TLSVolume field doc on
// BundleInput.
func readCACert(ctx context.Context, dataDir, tlsVolume string) (TrustSpec, error) {
	var certBytes []byte
	if tlsVolume != "" {
		b, err := dockervolume.ReadFile(ctx, tlsVolume, "central-bank.crt")
		if err != nil {
			if errors.Is(err, dockervolume.ErrNotFound) {
				return TrustSpec{}, fmt.Errorf("%w: not found in volume %s", ErrCACertNotFound, tlsVolume)
			}
			return TrustSpec{}, fmt.Errorf("read CA cert from volume %s: %w", tlsVolume, err)
		}
		certBytes = b
	} else {
		certPath := filepath.Join(dataDir, "tls", "central-bank.crt")
		b, err := os.ReadFile(certPath)
		if err != nil {
			if os.IsNotExist(err) {
				return TrustSpec{}, fmt.Errorf("%w: file not found at %s", ErrCACertNotFound, certPath)
			}
			return TrustSpec{}, fmt.Errorf("read CA cert: %w", err)
		}
		certBytes = b
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

	// found (CB-only) produces only spoke-level contracts. The Pente context and
	// FXAgreement-in-Pente are per-relationship and created at join time
	// (feature 033), so they are NOT required in the bundle.
	required := []struct {
		name string
		val  string
	}{
		{"REGISTRY_CONTRACT_ADDRESS", a.RegistryContractAddress},
		{"ZETO_FACTORY_ADDRESS", a.ZetoFactoryAddress},
		{"PENTE_FACTORY_ADDRESS", a.PenteFactoryAddress},
		{"ZETO_TOKEN_ADDRESS", a.ZetoTokenAddress},
	}
	for _, r := range required {
		if r.val == "" {
			return ContractsSpec{}, fmt.Errorf("%w: %s", ErrDeployedAddrsIncomplete, r.name)
		}
	}

	return ContractsSpec{
		RegistryAddress:            a.RegistryContractAddress,
		ZetoFactoryAddress:         a.ZetoFactoryAddress,
		PenteFactoryAddress:        a.PenteFactoryAddress,
		ZetoTokenAddress:           a.ZetoTokenAddress,
		PenteContextGroupID:        a.PenteContextGroupID,
		PenteContextAddress:        a.PenteContextAddress,
		FXAgreementAddress:         a.FXAgreementDeployedAt,
		ParticipantRegistryAddress: a.ParticipantRegistryAddress,
		// fCeBM + HTLC are deployed by found (deploy-fiat-token / deploy-htlc). Carried
		// so mode:join can wire the bank backend's FIAT_TOKEN_ADDRESS / HTLC_ADDRESS.
		FiatTokenAddress: a.FiatTokenAddress,
		HTLCAddress:      a.HTLCAddress,
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
