// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	// ErrBundleNotFound is returned when the join bundle file does not exist.
	ErrBundleNotFound = errors.New("bundle: join bundle file not found")

	// ErrBundleInvalid is returned when a loaded bundle fails mode:join validation
	// (e.g. empty validators list, missing cbEndpoint, or missing genesis content).
	ErrBundleInvalid = errors.New("bundle: invalid join bundle")
)

// LoadBundle reads the join bundle YAML at path and parses it into a *JoinBundle.
// It does not validate field values — use ValidateForJoin for that.
// Returns ErrBundleNotFound (wrapped) if the file does not exist.
func LoadBundle(path string) (*JoinBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrBundleNotFound, path)
		}
		return nil, fmt.Errorf("read join bundle %q: %w", path, err)
	}
	var b JoinBundle
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parse join bundle %q: %w", path, err)
	}
	return &b, nil
}

// ValidateForJoin checks that a bundle carries everything the mode:join engine
// needs. It collects all violations before returning so the operator never needs
// to re-run to discover additional errors. Returns nil if the bundle is valid.
func ValidateForJoin(b *JoinBundle) error {
	if b == nil {
		return fmt.Errorf("%w: nil bundle", ErrBundleInvalid)
	}

	var errs []error

	if b.Spec.SpokeID == "" {
		errs = append(errs, fmt.Errorf("%w: spec.spokeId is empty", ErrBundleInvalid))
	}
	if b.Spec.Bootnode.Enode == "" {
		errs = append(errs, fmt.Errorf("%w: spec.bootnode.enode is empty", ErrBundleInvalid))
	}
	if b.Spec.Genesis.Content == "" {
		errs = append(errs, fmt.Errorf("%w: spec.genesis.content is empty", ErrBundleInvalid))
	}
	if b.Spec.Genesis.Hash == "" {
		errs = append(errs, fmt.Errorf("%w: spec.genesis.hash is empty", ErrBundleInvalid))
	}
	if len(b.Spec.Validators) == 0 {
		errs = append(errs, fmt.Errorf("%w: validators list is empty — cannot execute mode:join", ErrBundleInvalid))
	}
	for i, v := range b.Spec.Validators {
		if v.Address == "" {
			errs = append(errs, fmt.Errorf("%w: validators[%d].address is empty", ErrBundleInvalid, i))
		}
		if v.RPCURL == "" {
			errs = append(errs, fmt.Errorf("%w: validators[%d].rpcUrl is empty", ErrBundleInvalid, i))
		}
	}
	if b.Spec.CBEndpoint == "" {
		errs = append(errs, fmt.Errorf("%w: cbEndpoint is required for mode:join", ErrBundleInvalid))
	}
	if b.Spec.Contracts.RegistryAddress == "" {
		errs = append(errs, fmt.Errorf("%w: spec.contracts.registryAddress is empty", ErrBundleInvalid))
	}

	return errors.Join(errs...)
}
