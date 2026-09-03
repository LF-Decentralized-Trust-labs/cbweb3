// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// signTxViaKeyProvider signs tx with the key identified by keyID, using the
// KeyProvider (the private key never leaves the provider). The orchestrator holds
// no raw key material — all on-chain signing goes through here.
func signTxViaKeyProvider(ctx context.Context, provider kp.KeyProvider, keyID string, signer types.Signer, tx *types.Transaction) (*types.Transaction, error) {
	h := signer.Hash(tx)
	sig, err := provider.Sign(ctx, keyID, h[:])
	if err != nil {
		return nil, fmt.Errorf("keyprovider sign (%s): %w", keyID, err)
	}
	// go-ethereum expects V at index 64 as 0/1, not 27/28.
	if len(sig) == 65 && sig[64] >= 27 {
		sig[64] -= 27
	}
	return tx.WithSignature(signer, sig)
}

// keyProviderAddress returns the EVM address of the key identified by keyID.
func keyProviderAddress(ctx context.Context, provider kp.KeyProvider, keyID string) (common.Address, error) {
	pub, err := provider.GetPublicKey(ctx, keyID)
	if err != nil {
		return common.Address{}, fmt.Errorf("keyprovider pubkey (%s): %w", keyID, err)
	}
	addr, err := kp.EVMAddress(pub)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(addr), nil
}
