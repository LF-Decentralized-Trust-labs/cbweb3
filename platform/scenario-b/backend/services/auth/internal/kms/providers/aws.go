// SPDX-License-Identifier: Apache-2.0

package kmsproviders

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
)

// KMSAws is a stub for the AWS KMS provider. Not yet implemented.
type KMSAws struct{}

// NewKMSAws creates a new (stub) AWS KMS provider.
func NewKMSAws() *KMSAws { return &KMSAws{} }

func (a *KMSAws) Name() string { return ProviderAws }

func (a *KMSAws) CreateKey(_ context.Context, _ string) (kms.KeyInfo, error) {
	return kms.KeyInfo{}, kms.ErrNotImplemented
}

func (a *KMSAws) Sign(_ context.Context, _, _ string) (kms.SignResult, error) {
	return kms.SignResult{}, kms.ErrNotImplemented
}

func (a *KMSAws) GetAddress(_ context.Context, _ string) (string, error) {
	return "", kms.ErrNotImplemented
}

func (a *KMSAws) DeleteKey(_ context.Context, _ string) error {
	return kms.ErrNotImplemented
}
