// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// KeySigner is the capability this package needs to sign without holding a private key.
//
// Production custody never exports key material: a KMS or HSM signs a digest and returns the
// signature. So the runtime path cannot keep taking a hex key — there will be none to give it.
//
// The shape deliberately mirrors the toolkit's KeyProvider, which scenario A already signs
// transactions through (scenario-a/toolkit/engine/orchestrator/signtx.go). Two things follow from
// that choice: a real provider written for one side satisfies the other, and the merge the team
// expects finds one interface instead of two designs to reconcile. It is redeclared here rather than
// imported because the toolkit is a separate Go module — the backend cannot import it.
type KeySigner interface {
	// Sign signs a 32-byte digest with the key identified by keyID and returns a 65-byte
	// Ethereum-style signature (r || s || v).
	Sign(ctx context.Context, keyID string, digest []byte) ([]byte, error)
	// GetPublicKey returns the uncompressed 65-byte public key for keyID.
	GetPublicKey(ctx context.Context, keyID string) ([]byte, error)
}

// NewRemoteSigner builds a Signer that signs through provider instead of holding a key.
//
// The address is read from the provider at construction, because with a KMS there is nothing local
// to derive it from — and an address that disagrees with whoever signs is an account that cannot
// sign. Reading it here also fails fast: a provider that cannot answer is refused at wiring rather
// than discovered mid-payment.
func NewRemoteSigner(ctx context.Context, provider KeySigner, keyID string, chainID *big.Int) (*Signer, error) {
	if provider == nil {
		return nil, fmt.Errorf("evm: a key provider is required for a remote signer")
	}
	if keyID == "" {
		return nil, fmt.Errorf("evm: a key id is required for a remote signer")
	}
	if chainID == nil || chainID.Sign() <= 0 {
		return nil, fmt.Errorf("evm: chainID must be a positive integer")
	}
	pub, err := provider.GetPublicKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("evm: read public key for %q: %w", keyID, err)
	}
	pubKey, err := gethcrypto.UnmarshalPubkey(pub)
	if err != nil {
		return nil, fmt.Errorf("evm: unmarshal public key for %q: %w", keyID, err)
	}
	return &Signer{
		remote:  provider,
		keyID:   keyID,
		address: gethcrypto.PubkeyToAddress(*pubKey),
		chainID: new(big.Int).Set(chainID),
	}, nil
}

// signTx signs tx, locally or through the provider.
//
// The remote branch normalizes the recovery id. Some signers return v as 27/28 and go-ethereum
// expects 0/1 at index 64; the difference produces a signature that is well-formed, type-checks, and
// is rejected by the node with an unhelpful error. Scenario A hit this and left a note about it — the
// tests here recover the sender from the signed transaction rather than merely checking that bytes
// came back, so a wrong v fails loudly.
func (s *Signer) signTx(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
	london := types.NewLondonSigner(s.ChainID())
	if s.remote == nil {
		signed, err := types.SignTx(tx, london, s.key)
		if err != nil {
			return nil, fmt.Errorf("sign tx: %w", err)
		}
		return signed, nil
	}
	digest := london.Hash(tx)
	sig, err := s.remote.Sign(ctx, s.keyID, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign tx via key provider (%s): %w", s.keyID, err)
	}
	if len(sig) != 65 {
		return nil, fmt.Errorf("sign tx via key provider (%s): expected a 65-byte signature, got %d", s.keyID, len(sig))
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	signed, err := tx.WithSignature(london, sig)
	if err != nil {
		return nil, fmt.Errorf("attach signature (%s): %w", s.keyID, err)
	}
	return signed, nil
}

// RemoteAddress is a convenience for callers that need an address before building a signer — for
// example to write it into configuration where a private key used to go.
func RemoteAddress(ctx context.Context, provider KeySigner, keyID string) (common.Address, error) {
	if provider == nil {
		return common.Address{}, fmt.Errorf("evm: a key provider is required")
	}
	pub, err := provider.GetPublicKey(ctx, keyID)
	if err != nil {
		return common.Address{}, fmt.Errorf("evm: read public key for %q: %w", keyID, err)
	}
	pubKey, err := gethcrypto.UnmarshalPubkey(pub)
	if err != nil {
		return common.Address{}, fmt.Errorf("evm: unmarshal public key for %q: %w", keyID, err)
	}
	return gethcrypto.PubkeyToAddress(*pubKey), nil
}
