// SPDX-License-Identifier: Apache-2.0

package keyprovider

import "context"

// prodKeyProvider is a stub for the production KMS provider.
// All methods return ErrNotImplemented until PR-1 (Phase 4) is complete.
type prodKeyProvider struct{}

var _ KeyProvider = (*prodKeyProvider)(nil)

func (p *prodKeyProvider) GenerateKey(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (p *prodKeyProvider) Sign(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (p *prodKeyProvider) GetPublicKey(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrNotImplemented
}
