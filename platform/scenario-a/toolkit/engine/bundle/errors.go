// SPDX-License-Identifier: Apache-2.0

package bundle

import "errors"

var (
	// ErrInvalidMode is returned when manifest.Spec.Mode != "found".
	ErrInvalidMode = errors.New("bundle: EmitBundle requires mode: found")

	// ErrInvalidInput is returned for missing mandatory fields in BundleInput
	// (e.g. nil Manifest, empty DataDir, nil EnodeProvider, or p2p.port == 0).
	ErrInvalidInput = errors.New("bundle: invalid manifest input")

	// ErrGenesisNotFound is returned when <dataDir>/genesis/genesis.json does not exist.
	ErrGenesisNotFound = errors.New("bundle: genesis.json not found in dataDir")

	// ErrCACertNotFound is returned when <dataDir>/tls/central-bank.crt is absent,
	// contains no valid PEM CERTIFICATE block, or contains private key material.
	ErrCACertNotFound = errors.New("bundle: CA cert not found or invalid in dataDir/tls")

	// ErrDeployedAddrsIncomplete is returned when .deployed-addrs.env exists but one
	// or more required addresses are missing or empty. The error message includes the
	// missing key name.
	ErrDeployedAddrsIncomplete = errors.New("bundle: deployed-addrs.env missing required key")

	// ErrEnodeUnavailable is returned when the admin_nodeInfo RPC call fails or the
	// returned enode is malformed.
	ErrEnodeUnavailable = errors.New("bundle: could not obtain enode from Besu")
)
