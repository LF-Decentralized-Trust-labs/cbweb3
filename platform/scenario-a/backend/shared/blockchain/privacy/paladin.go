// SPDX-License-Identifier: Apache-2.0

package privacy

import "context"

// PaladinBypass is a no-op implementation of PrivacyOperator used until the
// Hyperledger Paladin SDK integration is built for payment-orchestrator and
// liquidity-service. All operations return a zero hash and no error.
//
// Replace with a real PaladinClient when Paladin SDK is available.
type PaladinBypass struct{}

func (PaladinBypass) MintNoto(_ context.Context, _, _ string) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (PaladinBypass) TransferZeto(_ context.Context, _, _, _ string) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (PaladinBypass) CreateNotoHTLC(_ context.Context, _, _, _ string) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (PaladinBypass) ClaimNotoHTLC(_ context.Context, _, _ string) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}
