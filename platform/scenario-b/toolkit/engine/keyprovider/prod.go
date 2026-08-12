// SPDX-License-Identifier: Apache-2.0

package keyprovider

import "context"

// prodKeyProvider is the production stub: real KMS integration is a later phase.
// Every operation returns ErrNotImplemented. It deliberately does NOT implement
// LocalKeyExporter — production never exports private key material.
type prodKeyProvider struct{}

func (prodKeyProvider) GenerateKey(context.Context, string) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (prodKeyProvider) Sign(context.Context, string, []byte) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (prodKeyProvider) GetPublicKey(context.Context, string) ([]byte, error) {
	return nil, ErrNotImplemented
}
