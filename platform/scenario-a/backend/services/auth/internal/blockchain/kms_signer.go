// SPDX-License-Identifier: Apache-2.0

package blockchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	"github.com/ethereum/go-ethereum/core/types"
)

// Compile-time check: KMSSigner implements registry.TransactionSigner.
var _ registry.TransactionSigner = (*KMSSigner)(nil)

// KMSSigner implements registry.TransactionSigner by delegating signing to a
// kms.Provider. Used by commercial banks where the participant's secp256k1
// private key is stored in the KMS (created during onboarding).
//
// The signer user ID is resolved per-request from the context via
// SignerUserIDFromContext. Callers must set it with WithSignerUserID
// before invoking any blockchain write operation.
//
// Security: KMSSigner must NEVER be used on a Central Bank instance.
// Central Banks sign with StaticKeySigner (CB_PRIVATE_KEY) which does not
// consult the context and cannot be influenced by request data.
type KMSSigner struct {
	KMS kms.Provider
}

func (s *KMSSigner) signerUserID(ctx context.Context) (string, error) {
	uid, ok := SignerUserIDFromContext(ctx)
	if !ok {
		return "", ErrNoSignerUserID
	}
	return uid, nil
}

func (s *KMSSigner) SignerAddress(ctx context.Context) (string, error) {
	userID, err := s.signerUserID(ctx)
	if err != nil {
		return "", err
	}
	addr, err := s.KMS.GetAddress(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("kms signer: resolving address for %q: %w", userID, err)
	}
	return addr, nil
}

func (s *KMSSigner) SignTx(ctx context.Context, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	userID, err := s.signerUserID(ctx)
	if err != nil {
		return nil, err
	}
	ethSigner := types.NewEIP155Signer(chainID)
	txHash := ethSigner.Hash(tx)

	result, err := s.KMS.Sign(ctx, userID, hex.EncodeToString(txHash[:]))
	if err != nil {
		return nil, fmt.Errorf("kms signer: signing tx for %q: %w", userID, err)
	}

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(result.Signature, "0x"))
	if err != nil {
		return nil, fmt.Errorf("kms signer: decoding signature: %w", err)
	}

	return tx.WithSignature(ethSigner, sigBytes)
}
