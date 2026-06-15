// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"math/big"
)

// NoopRegistryClient is a no-op implementation used in local/test environments
// where no Besu node is available. All write operations succeed silently;
// reads return safe defaults (canTransact=true, verified status).
type NoopRegistryClient struct{}

func (NoopRegistryClient) RegisterParticipant(_ context.Context, _, _, _ string, _ [32]byte) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (NoopRegistryClient) UpdateStatus(_ context.Context, _ string, _ uint8) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (NoopRegistryClient) SetCertFingerprint(_ context.Context, _ string, _ [32]byte) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (NoopRegistryClient) CanTransact(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (NoopRegistryClient) IsWhitelisted(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (NoopRegistryClient) GetParticipant(_ context.Context, _ string) (OnChainParticipant, error) {
	return OnChainParticipant{
		Role:   RoleNone,
		Status: KycStatusVerified,
		LastUpdate: big.NewInt(0),
	}, nil
}

func (NoopRegistryClient) GetCertFingerprint(_ context.Context, _ string) ([32]byte, error) {
	return [32]byte{}, nil
}
